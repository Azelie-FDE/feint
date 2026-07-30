---
id: EXO-4
title: "EXO-4: block storage"
labels: roadmap, exoscale
milestone: "Wave 5 — storage on the two starters"
after: EXO-2
size: M
---
Batch 4 of `docs/roadmap-exoscale-iaas.md`; wave 5 of `docs/roadmap.md`.
Not speculative: the Terraform provider already declares
`exoscale_block_storage_volume` and `exoscale_block_storage_volume_snapshot`,
so a user writing an ordinary configuration reaches this product.

**Delivers.** The 13 block-storage operations, attach/detach and resize
included. Control plane; real backing only ever arrives as a declared
capability.

**Where the work lands.** New `internal/providers/exoscale/block.go`; the
contract extraction in the same change.

**Depends on.** EXO-2. Same bidirectional relation as everywhere else — the
computed-fact model applies as is. Watch `trimOperations` (bound at 512): an
apply creating volumes and snapshots in a loop is the first thing likely to
test it.

**Closed by.**

```bash
mise run conformance
# terraform apply with exoscale_block_storage_volume,
# exoscale_block_storage_volume_snapshot and a volume attached to an
# instance; empty second plan
```

**Done means all four** — the "When a batch is done" conditions of `docs/roadmap.md`:

- [ ] `mise run check` passes (gofmt, vet, golangci-lint, `go test -race`)
- [ ] Every new route declares its upstream `Route.Operation`, and `TestEveryRouteDeclaresAnOperation` proves it
- [ ] `mise run conformance` passes, including this batch's own new evidence
- [ ] `tools/drift/gate.sh check` returns 0, this batch's operations served or declined with a reason
