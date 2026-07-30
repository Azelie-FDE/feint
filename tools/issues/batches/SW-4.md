---
id: SW-4
title: "SW-4: IPAM lifecycle and the rest of vpc"
labels: roadmap, scaleway, machine
milestone: "Wave 4 — networks that route"
after: SW-1
size: M
---
Batch 4 of `docs/roadmap-scaleway-iaas.md`; wave 4 of `docs/roadmap.md`.
Any `terraform apply` carrying `scaleway_ipam_ip` fails today: the provider
calls BookIP, GetIP, UpdateIP and ReleaseIP, and only List and Get are served.

**Delivers.** ipam/v1: `BookIP`, `ReleaseIP`, `ReleaseIPSet`, `UpdateIP`,
`AttachIP`, `DetachIP`, `MoveIP` — the pack's address allocator already exists;
BookIP makes it client-drivable, and it is the pivot lb and vpcgw wait on.
vpc/v2: `ListSubnets`, `ListSubnetOverlaps`, `GetACL`, `SetACL`, routes (4 plus
`ListRoutesWithNexthop`), `EnableRouting`, `EnableDHCP`,
`EnableCustomRoutesPropagation`, ingress rules (5). Declined with a reason:
`CreateVPCConnector` and its four siblings, until OVN mode has measured
peering.

**Where the work lands.** `internal/providers/scaleway/ipam.go`, `vpc.go`,
`internal/core/machine/incus_ovn.go`.

**Depends on.** SW-1 only. SetACL touches the machine layer: the
machine-driver-author skill and the ownership question (`mustOwn`) both apply.
Never claim isolation without naming the mode.

**Closed by.**

```bash
mise run conformance
# terraform apply with a scaleway_ipam_ip booked and then carried by a private
# NIC; scw ipam ip list
FEINT_VM=incus-ovn mise run conformance
# under OVN, the ACL set by SetACL actually filters (network.sh extension)
```

**Done means all four** — the "When a batch is done" conditions of `docs/roadmap.md`:

- [ ] `mise run check` passes (gofmt, vet, golangci-lint, `go test -race`)
- [ ] Every new route declares its upstream `Route.Operation`, and `TestEveryRouteDeclaresAnOperation` proves it
- [ ] `mise run conformance` passes, including this batch's own new evidence
- [ ] `tools/drift/gate.sh check` returns 0, this batch's operations served or declined with a reason
