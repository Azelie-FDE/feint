---
id: SW-1
title: "SW-1: iam and marketplace under the gate; instance, vpc, ipam triaged"
labels: roadmap, scaleway
milestone: "Wave 1 — triage"
after: X-1
size: S
---
Batch 1 of `docs/roadmap-scaleway-iaas.md`; wave 1 of `docs/roadmap.md`.

**Delivers.** An honest gate. The pack serves iam (5 routes) and marketplace
(1 route) that the drift gate does not scan — served, and unmeasured, the least
defensible state a route can be in. Move `FEINT_PRODUCTS` to
`instance,vpc,ipam,iam,marketplace` and triage the ~85 operations that appear:
decline the bulk of iam (policies, applications, groups, API keys, JWT, logs —
only SSH key management stays; the emulator authenticates nothing) and the
unserved marketplace operations except `GetLocalImage` if the probe needs it.
Finish triaging the instance/vpc/ipam untriaged column. Refresh the baseline
with `mise run drift:update`.

**Where the work lands.** `tools/drift/gate.sh:31` (`FEINT_PRODUCTS`),
`internal/providers/scaleway/pack.go`, `coverage/scaleway-baseline.json`.

**Depends on.** X-1, so the ~80 new refusals are written once, against the
`(operation, reason)` signature.

**Closed by.**

```bash
tools/drift/gate.sh check ./feint   # returns 0 against the new baseline
mise run conformance                # the unchanged suite still passes
```

**Done means all four** — the "When a batch is done" conditions of `docs/roadmap.md`:

- [ ] `mise run check` passes (gofmt, vet, golangci-lint, `go test -race`)
- [ ] Every new route declares its upstream `Route.Operation`, and `TestEveryRouteDeclaresAnOperation` proves it
- [ ] `mise run conformance` passes, including this batch's own new evidence
- [ ] `tools/drift/gate.sh check` returns 0, this batch's operations served or declined with a reason
