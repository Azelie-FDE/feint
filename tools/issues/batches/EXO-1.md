---
id: EXO-1
title: "EXO-1: the managed-service surface declined by name"
labels: roadmap, exoscale
milestone: "Wave 1 — triage"
after: X-1
size: M
---
Batch 1 of `docs/roadmap-exoscale-iaas.md`; wave 1 of `docs/roadmap.md`.

**Delivers.** ~250 refusals into a `Declined()` that returns `nil` today,
grouped by family with one reason per block: DBaaS (144), SKS (24), AI (21),
KMS (16), DNS (16), IAM (17), SOS and billing (~9). Listed by name, not by
prefix, so an upstream addition under a declined family still shows up — the
pattern the Outscale pack already uses for OKS. The untriaged column drops from
358 (96% of the API) to about 105, and the gate stops being a wall.

**Where the work lands.** `internal/providers/exoscale/pack.go`,
`coverage/exoscale-baseline.json`.

**Depends on.** X-1 — this batch is the argument for it: 250 bare strings would
be unwieldy and mute.

**Closed by.**

```bash
tools/drift/gate.sh check ./feint   # returns 0 after mise run drift:update
```

**Done means all four** — the "When a batch is done" conditions of `docs/roadmap.md`:

- [ ] `mise run check` passes (gofmt, vet, golangci-lint, `go test -race`)
- [ ] Every new route declares its upstream `Route.Operation`, and `TestEveryRouteDeclaresAnOperation` proves it
- [ ] `mise run conformance` passes, including this batch's own new evidence
- [ ] `tools/drift/gate.sh check` returns 0, this batch's operations served or declined with a reason
