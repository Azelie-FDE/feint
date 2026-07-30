---
id: EXO-2
title: "EXO-2: instance lifecycle, security groups, elastic IPs — the preview label comes off"
labels: roadmap, exoscale, conformance
milestone: "Wave 2 — first Terraform proofs"
after: EXO-1
size: M
---
Batch 2 of `docs/roadmap-exoscale-iaas.md`; wave 2 of `docs/roadmap.md`.

**Delivers.** The pack serves create, read and delete, and nothing else:
`exo compute instance stop` fails against the emulator today. Serve
`start-instance`, `stop-instance`, `reboot-instance`, `reset-instance`,
`scale-instance`, `resize-instance-disk`, instance protection add/remove
(`get-console-proxy-url` served or declined with a reason — there is no console
to proxy); security groups and their rules; anti-affinity groups; elastic IPs
with instance attachment. Plus the missing fixture:
`tools/conformance/exoscale/terraform/main.tf`, `terraform.sh`, and a
`conformance:terraform:exoscale` task. Every mutation mints an Operation with
the correct `reference.command` — the provider calls `client.Wait` 89 times and
reads it.

**Where the work lands.** `internal/providers/exoscale/machines.go` and new
files; new `tools/conformance/exoscale/terraform/`; `mise.toml`.

**Depends on.** EXO-1, for legibility only. Every new lifecycle path takes
`machine.Binding.Serialise` and proves it with a concurrency test.

**Closed by.**

```bash
mise run conformance
# exo compute instance stop, then start; terraform apply of exoscale_ssh_key,
# exoscale_security_group(+rule), exoscale_anti_affinity_group,
# exoscale_elastic_ip and exoscale_compute_instance; empty second plan; destroy
```

**This is the batch that removes the README's *preview* label**, in the same
commit, on the condition `docs/roadmap.md` sets.

**Done means all four** — the "When a batch is done" conditions of `docs/roadmap.md`:

- [ ] `mise run check` passes (gofmt, vet, golangci-lint, `go test -race`)
- [ ] Every new route declares its upstream `Route.Operation`, and `TestEveryRouteDeclaresAnOperation` proves it
- [ ] `mise run conformance` passes, including this batch's own new evidence
- [ ] `tools/drift/gate.sh check` returns 0, this batch's operations served or declined with a reason
