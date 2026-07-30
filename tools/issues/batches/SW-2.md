---
id: SW-2
title: "SW-2: snapshots, images, placement groups, volume attach"
labels: roadmap, scaleway
milestone: "Wave 3 — the Scaleway golden image"
after: SW-1
size: M
---
Batch 2 of `docs/roadmap-scaleway-iaas.md`; wave 3 of `docs/roadmap.md` — the
first half of the golden-image scenario, the most expensive known gap for the
provider with the most users.

**Delivers.** The 27 untriaged instance/v1 operations, served or declined:
snapshots (`CreateSnapshot`, `GetSnapshot`, `ListSnapshots`, `UpdateSnapshot`,
`DeleteSnapshot`), images (`CreateImage`, `ListImages`, `UpdateImage`,
`DeleteImage`), placement groups (9 operations), volume attachment
(`AttachVolume`, `DetachVolume`, `AttachServerVolume`, `DetachServerVolume`),
`ListServerActions`, `UpdatePrivateNIC`, `ReleaseIPToIpam`. Declined with a
reason: `AttachServerFileSystem`, `DetachServerFileSystem` (follows the fate of
file/v1). The instance untriaged column empties by decision.

**Where the work lands.** New files beside
`internal/providers/scaleway/volumes.go`;
`tools/conformance/scaleway/terraform/main.tf`.

**Depends on.** SW-1. First genuinely bidirectional relation (an attached
volume changes the server's `volumes`; the root volume refuses detachment):
stored on one side, computed on the other. Every new lifecycle path takes
`machine.Binding.Serialise`, proven by a concurrency test on the model of
`TestConcurrentPowerOnStartsTheMachineOnce`.

**Closed by.**

```bash
mise run conformance
# scw instance snapshot create, then an image from that snapshot; terraform
# apply carrying scaleway_instance_placement_group and an extra attached
# volume; empty second plan; destroy
```

**Done means all four** — the "When a batch is done" conditions of `docs/roadmap.md`:

- [ ] `mise run check` passes (gofmt, vet, golangci-lint, `go test -race`)
- [ ] Every new route declares its upstream `Route.Operation`, and `TestEveryRouteDeclaresAnOperation` proves it
- [ ] `mise run conformance` passes, including this batch's own new evidence
- [ ] `tools/drift/gate.sh check` returns 0, this batch's operations served or declined with a reason
