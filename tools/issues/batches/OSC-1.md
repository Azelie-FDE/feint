---
id: OSC-1
title: "OSC-1: the non-IaaS half of the Outscale surface declined by name"
labels: roadmap, outscale
milestone: "Wave 1 — triage"
after: X-1
size: S
---
Batch 1 of `docs/roadmap-outscale-iaas.md`; wave 1 of `docs/roadmap.md`.

**Delivers.** 102 refusals, written by name with a reason per block, the way
`declined.go` already does for OKS: IAM (56 — the emulator authenticates
nothing), carrier connectivity (21 — DirectLink, VPN, gateways: nothing to back,
nothing to prove), hardware and sundry (25 — FlexibleGpu, DedicatedGroup,
VmTemplate, VmGroup and the price catalogues). Add the measured comment that
`ReadQuotas` and `ReadAccounts` are only read by data sources, so their refusal
breaks no `apply` — a refusal carrying its measurement is a refusal that can be
revisited. The untriaged column drops from 199 to 97 and becomes a work list.

**Where the work lands.** `internal/providers/outscale/declined.go`,
`coverage/outscale-baseline.json`.

**Depends on.** X-1: 102 refusals written against the old `[]string` signature
would be rewritten a week later.

**Closed by.**

```bash
tools/drift/gate.sh check ./feint   # returns 0 after mise run drift:update
```

**Done means all four** — the "When a batch is done" conditions of `docs/roadmap.md`:

- [ ] `mise run check` passes (gofmt, vet, golangci-lint, `go test -race`)
- [ ] Every new route declares its upstream `Route.Operation`, and `TestEveryRouteDeclaresAnOperation` proves it
- [ ] `mise run conformance` passes, including this batch's own new evidence
- [ ] `tools/drift/gate.sh check` returns 0, this batch's operations served or declined with a reason
