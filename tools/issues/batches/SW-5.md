---
id: SW-5
title: "SW-5: lb/v1 ZonedAPI"
labels: roadmap, scaleway
milestone: "Wave 6 — load balancing and gateways"
after: SW-4
size: L
---
Batch 5 of `docs/roadmap-scaleway-iaas.md`; wave 6 of `docs/roadmap.md`.
The largest single batch of the roadmap.

**Delivers.** The 54 operations of `lb/v1/ZonedAPI`: LBs, IPs, backends,
frontends, ACLs, certificates, routes, private-network attachment, and
`ListLBTypes` as a small fixed catalogue. Control plane first: `ready` states
immediately — the provider's `WaitForLb` loops on GetLB until `status=ready`,
and every transient state that does not converge is a twenty-minute timeout.
Declined with a reason: the 53 operations of `lb/v1/API`, the deprecated
regional API the provider never calls (measured under `services/lb/`).

**Where the work lands.** New `internal/providers/scaleway/lb.go`; contract
extraction in `tools/contract/scaleway-products.txt` (the portal slug is
`load-balancer/zoned`, the file documents it).

**Depends on.** SW-4: attachment to a Private Network passes `ipam_ip_ids`, so
lb depends on BookIP and vpc. Main risk: size, and the state machine the
provider's waiters observe — every object (LB, backend, certificate) has its
own status and the provider reads all of them. The probe refuses to invent
identifiers, so the frontend→backend→LB dependency chains will test its
planning; if it cannot keep up, the batch says so and the probe improves, not
the other way round.

**Closed by.**

```bash
mise run conformance
# full terraform apply: scaleway_lb_ip, scaleway_lb, scaleway_lb_backend,
# scaleway_lb_frontend, scaleway_lb_acl, attachment to a Private Network via
# ipam_ip_ids; waiters converge; empty second plan; destroy;
# scw lb lb create/get/delete
```

**Done means all four** — the "When a batch is done" conditions of `docs/roadmap.md`:

- [ ] `mise run check` passes (gofmt, vet, golangci-lint, `go test -race`)
- [ ] Every new route declares its upstream `Route.Operation`, and `TestEveryRouteDeclaresAnOperation` proves it
- [ ] `mise run conformance` passes, including this batch's own new evidence
- [ ] `tools/drift/gate.sh check` returns 0, this batch's operations served or declined with a reason
