# The configuration the README's demo applies.
#
# Deliberately the smallest thing that proves the claim: the official Scaleway
# provider, pointed at the emulator through api_url, creating a real resource and
# reading it back. Everything else (security groups, private networks, volumes)
# is exercised by the conformance suite in tools/conformance/scaleway/terraform,
# which is where breadth belongs. A demo that scrolls is a demo nobody watches.

terraform {
  required_version = ">= 1.7.0"

  required_providers {
    scaleway = {
      source  = "scaleway/scaleway"
      version = "~> 2.79"
    }
  }
}

provider "scaleway" {
  api_url         = "http://127.0.0.1:4599"
  access_key      = "SCWXXXXXXXXXXXXXXXXX"
  secret_key      = "11111111-1111-1111-1111-111111111111"
  project_id      = "11111111-1111-1111-1111-111111111111"
  # A different UUID from the project on purpose: infrastructure belongs to a
  # project, IAM and billing to the organization above it. Reusing one value
  # for both is how a client reads one where it expected the other and
  # notices nothing until it talks to the real API.
  organization_id = "99999999-9999-4999-8999-999999999999"
  region          = "fr-par"
  zone            = "fr-par-1"
}

resource "scaleway_instance_server" "demo" {
  name  = "from-terraform"
  type  = "DEV1-S"
  zone  = "fr-par-1"
  image = "ubuntu_jammy"
  state = "started"
  tags  = ["demo"]
}

output "server" {
  value = "${scaleway_instance_server.demo.name} is ${scaleway_instance_server.demo.state}"
}
