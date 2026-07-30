---
id: SW-6
title: "SW-6: vpcgw/v2"
labels: roadmap, scaleway
milestone: "Wave 6 — load balancing and gateways"
after: SW-4
size: M
---
Batch 6 of `docs/roadmap-scaleway-iaas.md`; wave 6 of `docs/roadmap.md`.

**Delivers.** The 27 operations of `vpcgw/v2`: gateways, gateway networks,
IPs, PAT rules, bastion, and `ListGatewayTypes` as a small fixed catalogue.
Declined with a reason: `vpcgw/v1` wholesale (37 operations, superseded; the
provider imports v2).

**Where the work lands.** New `internal/providers/scaleway/vpcgw.go`; contract
extraction in the same change.

**Depends on.** SW-4 (IPAM and vpc). The one candidate where OVN backing may
be cheap — the logical router already exists in incus-ovn mode: measure
whether egress NAT can genuinely be backed, and otherwise declare it degraded,
never claim it. The capability rule settles the temptation: an undeclared
capability counts as absent.

**Closed by.**

```bash
mise run conformance
# terraform apply with scaleway_vpc_public_gateway,
# scaleway_vpc_gateway_network and a PAT rule; empty second plan; destroy
```

**Done means all four** — the "When a batch is done" conditions of `docs/roadmap.md`:

- [ ] `mise run check` passes (gofmt, vet, golangci-lint, `go test -race`)
- [ ] Every new route declares its upstream `Route.Operation`, and `TestEveryRouteDeclaresAnOperation` proves it
- [ ] `mise run conformance` passes, including this batch's own new evidence
- [ ] `tools/drift/gate.sh check` returns 0, this batch's operations served or declined with a reason
