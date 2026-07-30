# The machines the install guide is proven on, declared rather than launched.
#
# Terraform owns the lifecycle and Ansible owns the configuration, which is the
# division that makes both idempotent: `terraform apply` converges the fleet,
# `ansible-playbook` converges what runs inside it, and neither has to know what
# the other did. The inventory is read from this state by the
# cloud.terraform.terraform_provider plugin, so a machine added here appears in
# Ansible with no second list to keep in step — which is the whole reason this
# layer exists rather than a `for` loop over `incus launch`.
#
#   terraform init && terraform apply
#   ansible-playbook -i ../ansible/inventory.yml ../ansible/site.yml
#
# Virtual machines, not containers, and that is not a preference: Incus inside a
# container needs nesting, OVN needs kernel modules the host would have to lend,
# and a guide verified under those conditions is not the guide a reader follows.

terraform {
  required_version = ">= 1.6"

  required_providers {
    incus = {
      source  = "lxc/incus"
      version = "~> 1.1"
    }
    # Not a provider that creates anything: ansible_host resources exist only in
    # state, and cloud.terraform.terraform_provider reads them to build the
    # inventory. That is how the two tools are chained without a second list of
    # machines — the first attempt pointed the plugin at the incus_instance
    # resources directly and got an empty inventory, because those are not what
    # it reads.
    ansible = {
      source  = "ansible/ansible"
      version = "~> 1.5"
    }
  }
}

provider "incus" {}

# The two most recent releases of each supported distribution. Adding one is a
# line here and nothing anywhere else.
variable "targets" {
  description = "The releases the install guide is proven on, keyed by machine name."
  type = map(object({
    image  = string
    family = string
  }))

  default = {
    "feint-debian-12"   = { image = "images:debian/12/cloud", family = "debian" }
    "feint-debian-13"   = { image = "images:debian/13/cloud", family = "debian" }
    "feint-ubuntu-2404" = { image = "images:ubuntu/24.04/cloud", family = "debian" }
    "feint-ubuntu-2604" = { image = "images:ubuntu/26.04/cloud", family = "debian" }
    "feint-rocky-9"     = { image = "images:rockylinux/9/cloud", family = "redhat" }
    "feint-rocky-10"    = { image = "images:rockylinux/10/cloud", family = "redhat" }
    "feint-fedora-43"   = { image = "images:fedora/43/cloud", family = "redhat" }
    "feint-fedora-44"   = { image = "images:fedora/44/cloud", family = "redhat" }
  }
}

variable "memory" {
  description = "Memory per machine. OVN's databases plus a Go build do not fit in the default, and the failure is a linker OOM that names nothing."
  type        = string
  default     = "4GiB"
}

variable "disk" {
  description = "Root disk per machine."
  type        = string
  default     = "20GiB"
}

resource "incus_instance" "target" {
  for_each = var.targets

  name  = each.key
  image = each.value.image
  type  = "virtual-machine"

  config = {
    "limits.memory" = var.memory
    # The emulator inside starts its own containers, which is the thing being
    # verified. Without nesting it reports a runtime it cannot use.
    "security.nesting" = "true"
    # Read by the Ansible inventory plugin, so the family a machine belongs to
    # travels with the machine rather than being re-derived from its name.
    "user.feint-family" = each.value.family
    "user.feint-image"  = each.value.image
  }

  # The agent configuration drive. Debian and Ubuntu cloud images carry the
  # Incus agent already; the Rocky and Fedora ones refuse to start without this
  # disk, with "This virtual machine image requires an agent:config disk be
  # added" — measured on the first apply, and the reason it is declared for every
  # machine rather than for the family that needs it: a device that is redundant
  # on four images is cheaper than a conditional nobody remembers.
  device {
    name = "agent"
    type = "disk"
    properties = {
      source = "agent:config"
    }
  }

  device {
    name = "root"
    type = "disk"
    properties = {
      pool = "default"
      path = "/"
      size = var.disk
    }
  }

  # The guest agent answers a good while after the machine reports RUNNING.
  # Waiting for it here rather than in Ansible means a `terraform apply` that
  # returns has produced machines something can actually talk to — measured: the
  # first run connected too early and failed with "VM agent isn't currently
  # running", which reads like a broken image rather than an impatient caller.
  wait_for {
    type  = "agent"
    delay = "10s"
  }
}

# One inventory entry per machine, derived from the same map. A release added to
# var.targets appears in Terraform and in Ansible at once; nothing else to edit,
# which is the whole reason this indirection exists.
resource "ansible_host" "target" {
  for_each = var.targets

  name   = each.key
  groups = ["${each.value.family}_family"]

  variables = {
    # The connection Ansible will use. Incus rather than ssh: no key to place, no
    # sshd to wait for, and no ~/.ssh/config of the operator's to interfere —
    # this project has already lost an hour to a ProxyJump on a broad Host
    # pattern swallowing the runtime's private range.
    ansible_connection         = "community.general.incus"
    ansible_incus_remote       = "local"
    ansible_python_interpreter = "/usr/bin/python3"
    feint_image                = each.value.image
  }

  # Declared after the machine, so an inventory read between apply and boot
  # cannot name a host that does not exist yet.
  depends_on = [incus_instance.target]
}

output "machines" {
  description = "What Ansible will configure."
  value = {
    for name, instance in incus_instance.target : name => {
      family = var.targets[name].family
      image  = var.targets[name].image
    }
  }
}
