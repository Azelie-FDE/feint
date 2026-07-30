---
id: EXO-6
title: "EXO-6: VPC and routes"
labels: roadmap, exoscale
milestone: "Wave 6 — load balancing and gateways"
after: EXO-3
size: M
---
Batch 6 of `docs/roadmap-exoscale-iaas.md`; wave 6 of `docs/roadmap.md`.

**Delivers.** The 9 VPC operations and their routes. The pack models nothing
here today, so this is a new resource kind rather than an extension.

**Where the work lands.** New `internal/providers/exoscale/vpc.go`; contract
extraction in the same change.

**Depends on.** EXO-3. Under OVN mode, measure what routing can honestly be
backed; declare degraded rather than claim — an undeclared capability counts
as absent.

**Closed by.**

```bash
mise run conformance
# terraform apply on the VPC resources the provider exposes; backing declared
# or declared degraded
```

**Done means all four** — the "When a batch is done" conditions of `docs/roadmap.md`:

- [ ] `mise run check` passes (gofmt, vet, golangci-lint, `go test -race`)
- [ ] Every new route declares its upstream `Route.Operation`, and `TestEveryRouteDeclaresAnOperation` proves it
- [ ] `mise run conformance` passes, including this batch's own new evidence
- [ ] `tools/drift/gate.sh check` returns 0, this batch's operations served or declined with a reason
