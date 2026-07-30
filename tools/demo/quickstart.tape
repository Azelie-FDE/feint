# The README's demo, as a script rather than a recording nobody can reproduce.
#
# Regenerate with `mise run demo`. Everything shown here is executed for real:
# the emulator starts, the official Scaleway CLI and the official Terraform
# provider drive it, and the emulator stops. If a command in this tape breaks,
# the GIF breaks, which is the point — a demo that cannot be re-run is a claim,
# not a proof.
#
# Two things the hidden section has to get right. It builds the binary and
# pre-initialises Terraform, because `terraform init` downloads a provider and
# takes longer than anybody will watch; and it *sleeps* after each of those,
# because vhs types the next line without waiting for the previous one to
# finish. Without those sleeps the setup spills into the first frames, which is
# exactly what the first recording did.

Output docs/assets/quickstart.gif

Set Shell bash
Set FontSize 16
Set Width 1100
Set Height 640
Set Padding 12
Set TypingSpeed 40ms
Set Theme "Catppuccin Mocha"

Hide
# REPO is captured rather than written out: vhs runs from the repository root
# (`mise run demo`), and a hardcoded path would publish one machine's layout and
# break on every other.
Type "export REPO=$PWD && export PS1='$ ' && export DEMO=$(mktemp -d)" Enter
Sleep 1s
Type "cd $REPO && go build -o feint ./cmd/feint" Enter
Sleep 12s
Type "./feint stop >/dev/null 2>&1; cp tools/demo/main.tf feint $DEMO/ && cd $DEMO" Enter
Sleep 2s
Type "export PATH=$DEMO:$PATH && terraform init -input=false >/dev/null 2>&1" Enter
Sleep 25s
Type "clear" Enter
Sleep 2s
Show

Type "# One binary. Three European clouds. No account, no bill."
Enter
Sleep 1500ms

Type "feint start"
Enter
Sleep 4s

Type "# Point the official Scaleway CLI at it. Nothing to edit."
Enter
Sleep 1s
Type 'eval "$(feint env scaleway)"'
Enter
Sleep 1500ms

Type "scw instance server create name=demo type=DEV1-S image=ubuntu_jammy zone=fr-par-1 -o json | jq -c '{name,state,commercial_type}'"
Enter
Sleep 5s

Type "# And the real Terraform provider, through api_url."
Enter
Sleep 1s

Type "terraform apply -auto-approve -input=false -no-color | tail -4"
Enter
Sleep 14s

Type "scw instance server list zone=fr-par-1 -o json | jq -c '.[] | {name,state}'"
Enter
Sleep 5s

Type "feint stop"
Enter
Sleep 4s

Hide
Type "cd $REPO" Enter
Show
