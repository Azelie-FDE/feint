---
id: EXO-3
title: "EXO-3: private networks and instance attachment"
labels: roadmap, exoscale, machine
milestone: "Wave 4 — networks that route"
after: EXO-2
size: M
---
Batch 3 of `docs/roadmap-exoscale-iaas.md`; wave 4 of `docs/roadmap.md`.

**Delivers.** Private networks and their subnets,
`attach-instance-to-private-network`, `detach-instance-from-private-network`,
`attach-instance-to-subnet`, and the external sources on a security group.
Plus a `network.sh` for Exoscale on the model of
`tools/conformance/scaleway/network.sh`.

**Where the work lands.** New files in `internal/providers/exoscale/`; new
`tools/conformance/exoscale/network.sh`.

**Depends on.** EXO-2. This is where the pack starts driving the machine
layer: the machine-driver-author skill and the ownership question (`mustOwn`)
apply. Isolation is asserted under OVN when the mode declares the capability
(`capabilities.isolation`), skipped elsewhere, and never claimed without
naming the mode.

**Closed by.**

```bash
mise run conformance
# terraform apply with exoscale_private_network and an instance attached
FEINT_VM=incus-ovn mise run conformance
# the isolation assertion passes rather than skips when the mode declares it
```

**Done means all four** — the "When a batch is done" conditions of `docs/roadmap.md`:

- [ ] `mise run check` passes (gofmt, vet, golangci-lint, `go test -race`)
- [ ] Every new route declares its upstream `Route.Operation`, and `TestEveryRouteDeclaresAnOperation` proves it
- [ ] `mise run conformance` passes, including this batch's own new evidence
- [ ] `tools/drift/gate.sh check` returns 0, this batch's operations served or declined with a reason
