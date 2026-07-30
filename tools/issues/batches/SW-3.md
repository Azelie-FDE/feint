---
id: SW-3
title: "SW-3: block/v1 and the sbs_volume root volume"
labels: roadmap, scaleway
milestone: "Wave 3 — the Scaleway golden image"
after: SW-2
size: M
---
Batch 3 of `docs/roadmap-scaleway-iaas.md`; wave 3 of `docs/roadmap.md`.
The highest-value unblock measured: the Terraform provider reads every volume
through `GetUnknownVolume` — `instance.GetVolume` first, then on a typed 404 a
fallback to `block.GetVolume`, which today hits an unmounted route. Any
`sbs_volume` root volume dies there.

**Delivers.** block/v1: `CreateVolume`, `GetVolume`, `ListVolumes`,
`UpdateVolume`, `DeleteVolume`, the snapshot lifecycle (5), and
`ListVolumeTypes` as a small fixed catalogue (declining it would reproduce the
`min_size` trap). Then the `sbs_volume` root volume on the instance side:
`createServer` accepts the type and materialises the volume in block. Declined
with a reason: `block/v1alpha1` wholesale (14, superseded), the two Object
Storage transfer operations (same measured reason as
`instance/v1/API.ExportSnapshot`).

**Where the work lands.** New `internal/providers/scaleway/block.go`,
`catalog.go`, `tools/contract/scaleway-products.txt` (contract extraction in
the same change as the routes, never after).

**Depends on.** SW-2, for attachment consistency. The instance 404 must keep
its exact typed shape — the fallback depends on it. Block's `references` field
follows the stored-on-one-side, computed-on-the-other rule.

**Closed by.**

```bash
mise run conformance
# terraform apply with scaleway_block_volume, scaleway_block_snapshot, and a
# scaleway_instance_server whose root_volume.volume_type = "sbs_volume";
# empty second plan; scw block volume create/list/delete
```

**Done means all four** — the "When a batch is done" conditions of `docs/roadmap.md`:

- [ ] `mise run check` passes (gofmt, vet, golangci-lint, `go test -race`)
- [ ] Every new route declares its upstream `Route.Operation`, and `TestEveryRouteDeclaresAnOperation` proves it
- [ ] `mise run conformance` passes, including this batch's own new evidence
- [ ] `tools/drift/gate.sh check` returns 0, this batch's operations served or declined with a reason
