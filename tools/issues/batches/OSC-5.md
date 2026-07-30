---
id: OSC-5
title: "OSC-5: load balancing"
labels: roadmap, outscale
milestone: "Wave 6 — load balancing and gateways"
after: OSC-3
size: L
---
Batch 5 of `docs/roadmap-outscale-iaas.md`; wave 6 of `docs/roadmap.md`.

**Delivers.** The 23 operations of the LoadBalancer family,
`ServerCertificate` included. Pure control plane, immediate states.

**Where the work lands.** New `internal/providers/outscale/lb.go`; contract
extraction in the same change.

**Depends on.** OSC-2 and OSC-3. Main risk: size, and a strict contract
(`additionalProperties: false`) over rich responses — which is the intended
behaviour and will fail the first attempts.

**Closed by.**

```bash
mise run conformance
# terraform apply with outscale_load_balancer,
# outscale_load_balancer_listener_rule and outscale_load_balancer_vms;
# empty second plan
```

**Done means all four** — the "When a batch is done" conditions of `docs/roadmap.md`:

- [ ] `mise run check` passes (gofmt, vet, golangci-lint, `go test -race`)
- [ ] Every new route declares its upstream `Route.Operation`, and `TestEveryRouteDeclaresAnOperation` proves it
- [ ] `mise run conformance` passes, including this batch's own new evidence
- [ ] `tools/drift/gate.sh check` returns 0, this batch's operations served or declined with a reason
