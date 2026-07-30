---
id: OSC-2
title: "OSC-2: ProductCodes, admin password, tags, root volume — Outscale's first terraform apply"
labels: roadmap, outscale, conformance
milestone: "Wave 2 — first Terraform proofs"
after: OSC-1
size: S
---
Batch 2 of `docs/roadmap-outscale-iaas.md`; wave 2 of `docs/roadmap.md`.
Small, and the highest-value Outscale batch: the pack has no Terraform evidence
at all today.

**Delivers.** What breaks first is not a missing product but a missing field:
the emulated catalogue publishes no `ProductCodes`, so the Terraform provider
calls `ReadAdminPassword` on **every** VM it reads back, Linux included, and
that route does not exist. So: publish a Linux `ProductCodes` on images and VMs;
serve `ReadAdminPassword` (empty password, never an invented one);
`UpdateVolume` and `ReadVolumes` at least for the root volume (the VM resource
itself calls `UpdateVolume`); `CreateTags`, `ReadTags`, `DeleteTags`;
`ReadVmsState`. Plus the missing fixture:
`tools/conformance/outscale/terraform/main.tf`, `terraform.sh` on the Scaleway
model, and a `conformance:terraform:outscale` mise task.

**Where the work lands.** `internal/providers/outscale/catalog.go`, `vms.go`,
new `volumes.go`; new `tools/conformance/outscale/terraform/`; `mise.toml`.

**Depends on.** OSC-1, for legibility of the gate. Contract first: the product
enters `tools/contract/update.sh` extraction before its handlers are written —
`additionalProperties: false` refuses an unextracted product's responses
outright.

**Closed by.**

```bash
mise run conformance
# terraform apply of outscale_keypair + outscale_vm, empty second plan,
# clean destroy, contract on
```

**Done means all four** — the "When a batch is done" conditions of `docs/roadmap.md`:

- [ ] `mise run check` passes (gofmt, vet, golangci-lint, `go test -race`)
- [ ] Every new route declares its upstream `Route.Operation`, and `TestEveryRouteDeclaresAnOperation` proves it
- [ ] `mise run conformance` passes, including this batch's own new evidence
- [ ] `tools/drift/gate.sh check` returns 0, this batch's operations served or declined with a reason
