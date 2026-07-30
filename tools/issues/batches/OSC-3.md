---
id: OSC-3
title: "OSC-3: routable networking — the provider's own examples/net_vm applies"
labels: roadmap, outscale
milestone: "Wave 4 — networks that route"
after: OSC-2
size: M
---
Batch 3 of `docs/roadmap-outscale-iaas.md`; wave 4 of `docs/roadmap.md`.

**Delivers.** What makes a Net routable: `SecurityGroup` and
`SecurityGroupRule` (5), `PublicIp` and its link (5), `Nic` and its links
(6+2), `RouteTable`, `Route` and the links (~10), `InternetService` and its
link (5), `NatService` (3). Fix `tools/conformance/outscale/oapi-cli.sh:268` in
the same batch: all five of its still-unserved 404 candidates become served,
and the script says so itself.

**Where the work lands.** New files in `internal/providers/outscale/`;
`tools/conformance/outscale/oapi-cli.sh:268`;
`tools/conformance/outscale/terraform/`.

**Depends on.** OSC-2. The security group must genuinely filter under
`FEINT_VM=incus`, or `network.sh` contradicts what the API publishes — the
shared layer already does this for Scaleway, so it is wiring, not a rewrite.
One architecture question from `docs/roadmap.md` gates the enforcement claim: a
rule sourced by *group* rather than by CIDR needs an OVN selector, and that
must be answered before this batch promises enforcement.

**Closed by.**

```bash
mise run conformance
# terraform apply of the provider's own examples/net_vm (13 resources),
# adapted to the emulated catalogue's identifiers; empty second plan; destroy
```

**Done means all four** — the "When a batch is done" conditions of `docs/roadmap.md`:

- [ ] `mise run check` passes (gofmt, vet, golangci-lint, `go test -race`)
- [ ] Every new route declares its upstream `Route.Operation`, and `TestEveryRouteDeclaresAnOperation` proves it
- [ ] `mise run conformance` passes, including this batch's own new evidence
- [ ] `tools/drift/gate.sh check` returns 0, this batch's operations served or declined with a reason
