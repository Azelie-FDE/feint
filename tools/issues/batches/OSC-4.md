---
id: OSC-4
title: "OSC-4: volumes, snapshots, images"
labels: roadmap, outscale
milestone: "Wave 5 — storage on the two starters"
after: OSC-2
size: M
---
Batch 4 of `docs/roadmap-outscale-iaas.md`; wave 5 of `docs/roadmap.md`.

**Delivers.** `Volume` (6 operations), `Snapshot` (4), `Image` (3, alongside
the already-served `ReadImages`). The export tasks stay declined — the
measurement is in `docs/limits.md` and it is about S3, not Outscale.

**Where the work lands.** `internal/providers/outscale/`.

**Depends on.** OSC-2, for attachment consistency. The bidirectional volume/VM
relation is the same as everywhere else: stored on one side, computed on the
other; what a delete refuses is a pack invariant the fixture's destroy
exercises for free.

**Closed by.**

```bash
mise run conformance
# terraform apply with outscale_volume, outscale_volumes_link,
# outscale_snapshot and an image created from a snapshot; empty second plan;
# oapi-cli over the same path
```

**Done means all four** — the "When a batch is done" conditions of `docs/roadmap.md`:

- [ ] `mise run check` passes (gofmt, vet, golangci-lint, `go test -race`)
- [ ] Every new route declares its upstream `Route.Operation`, and `TestEveryRouteDeclaresAnOperation` proves it
- [ ] `mise run conformance` passes, including this batch's own new evidence
- [ ] `tools/drift/gate.sh check` returns 0, this batch's operations served or declined with a reason
