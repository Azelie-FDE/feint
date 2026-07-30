---
id: EXO-5
title: "EXO-5: the network load balancer"
labels: roadmap, exoscale
milestone: "Wave 6 — load balancing and gateways"
after: EXO-3
size: M
---
Batch 5 of `docs/roadmap-exoscale-iaas.md`; wave 6 of `docs/roadmap.md`.

**Delivers.** The 11 NLB operations, services included. Pure control plane;
every mutation mints an Operation with the correct `reference.command`.

**Where the work lands.** New `internal/providers/exoscale/nlb.go`; contract
extraction in the same change.

**Depends on.** EXO-2 and EXO-3.

**Closed by.**

```bash
mise run conformance
# terraform apply with exoscale_nlb and exoscale_nlb_service; empty second plan
```

**Done means all four** — the "When a batch is done" conditions of `docs/roadmap.md`:

- [ ] `mise run check` passes (gofmt, vet, golangci-lint, `go test -race`)
- [ ] Every new route declares its upstream `Route.Operation`, and `TestEveryRouteDeclaresAnOperation` proves it
- [ ] `mise run conformance` passes, including this batch's own new evidence
- [ ] `tools/drift/gate.sh check` returns 0, this batch's operations served or declined with a reason
