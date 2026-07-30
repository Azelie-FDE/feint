package scaleway

import (
	"net/http"
	"net/netip"
	"time"

	"github.com/stephrobert/feint/internal/core/emulator"
	"github.com/stephrobert/feint/internal/core/resource"
)

// IPAM is how a client learns the address of a private NIC, and serving it is
// not optional: instance/v1.PrivateNIC carries no address at all. It carries
// `ipam_ip_ids`, and the client follows them here.
//
// This emulator got that wrong first: it served invented `private_ip` and
// `private_ips` fields on the NIC, and its own conformance suite read them back,
// so the suite proved the emulator against itself rather than against the API.
// That is the one mistake this project cannot afford, since its whole claim is
// that the official client sees no difference.
//
// Shapes come from the SDK (api/ipam/v1/ipam_sdk.go): IP, Source, Resource, and
// ListIPsResponse. Note the address is an scw.IPNet, so it carries its mask;
// serving a bare address makes the SDK fail to decode what it just received.

const kindIPAMIP = "ipam/ip"

// runtimeNICKey links an IPAM address to the private NIC holding it.
const runtimeNICKey = "nic"

// resourceTypePrivateNIC is what the SDK calls a NIC of an Instance server.
const resourceTypePrivateNIC = "instance_private_nic"

func (p *Pack) listIPAMIPs(w http.ResponseWriter, r *http.Request) {
	region, ok := regionOf(w, r)
	if !ok {
		return
	}

	all := p.env.Store.List(kindIPAMIP, p.regionScopeOf(r, region))
	// The filters a client actually sends: the provider asks for the addresses
	// of one NIC, or of one private network.
	q := r.URL.Query()
	if id := q.Get("resource_id"); id != "" {
		all = filterResources(all, func(res *resource.Resource) bool {
			return res.Runtime[runtimeNICKey] == id
		})
	}
	if id := q.Get("private_network_id"); id != "" {
		all = filterResources(all, func(res *resource.Resource) bool {
			return res.Attrs["private_network_id"] == id
		})
	}

	page := parsePage(r)
	start, end := page.slice(len(all))
	ips := make([]map[string]any, 0, end-start)
	for _, res := range all[start:end] {
		ips = append(ips, p.ipamIPView(res))
	}

	emulator.WriteJSON(w, http.StatusOK, map[string]any{
		"ips":         ips,
		"total_count": len(all),
	})
}

func (p *Pack) getIPAMIP(w http.ResponseWriter, r *http.Request) {
	res, ok := p.resourceOf(w, r, kindIPAMIP, "ipID", "ip")
	if !ok {
		return
	}
	emulator.WriteJSON(w, http.StatusOK, p.ipamIPView(res))
}

// newIPAMIP records the address a private NIC received, as the resource a
// client resolves it through.
func (p *Pack) newIPAMIP(region, project string, address netip.Prefix, nic, pn *resource.Resource) *resource.Resource {
	now := p.env.Now()
	return &resource.Resource{
		ID:      p.env.NewID(),
		Kind:    kindIPAMIP,
		Tenant:  resource.Tenant{Provider: Name, Project: project, Zone: region},
		State:   "available",
		Created: now,
		Updated: now,
		Attrs: map[string]any{
			"address":            address.String(),
			"project_id":         project,
			"is_ipv6":            false,
			"tags":               []string{},
			"private_network_id": pn.ID,
			"vpc_id":             pn.Attrs["vpc_id"],
			"subnet_id":          subnetIDOf(pn.ID),
			"mac_address":        nic.Attrs["mac_address"],
			"zone":               nic.Tenant.Zone,
		},
		Runtime: map[string]string{runtimeNICKey: nic.ID},
	}
}

// ipamIPsOf returns the addresses held by a NIC, which is what the NIC view
// publishes as ipam_ip_ids.
func (p *Pack) ipamIPsOf(nicID string) []*resource.Resource {
	all := p.env.Store.List(kindIPAMIP, resource.Tenant{Provider: Name})
	out := make([]*resource.Resource, 0, len(all))
	for _, res := range all {
		if res.Runtime[runtimeNICKey] == nicID {
			out = append(out, res)
		}
	}
	return out
}

// addressOfNIC returns the address a NIC holds, empty when it holds none. The
// pack needs it to drive the machine; a client goes through IPAM instead.
func (p *Pack) addressOfNIC(nicID string) string {
	for _, ip := range p.ipamIPsOf(nicID) {
		if address, _ := ip.Attrs["address"].(string); address != "" {
			if prefix, err := netip.ParsePrefix(address); err == nil {
				return prefix.Addr().String()
			}
		}
	}
	return ""
}

func (p *Pack) ipamIPView(res *resource.Resource) map[string]any {
	out := map[string]any{
		"id":         res.ID,
		"region":     res.Tenant.Zone,
		"created_at": res.Created.Format(time.RFC3339),
		"updated_at": res.Updated.Format(time.RFC3339),
		"reverses":   []any{},
	}
	for _, key := range []string{"address", "project_id", "is_ipv6", "tags", "zone"} {
		out[key] = res.Attrs[key]
	}

	// Source says where the address comes from, Resource says what holds it.
	// Both are objects the SDK decodes; flattening them is how a client ends up
	// with zero values it never notices.
	out["source"] = map[string]any{
		"private_network_id": res.Attrs["private_network_id"],
		"subnet_id":          res.Attrs["subnet_id"],
		"vpc_id":             res.Attrs["vpc_id"],
	}
	out["resource"] = map[string]any{
		"type":        resourceTypePrivateNIC,
		"id":          res.Runtime[runtimeNICKey],
		"mac_address": res.Attrs["mac_address"],
		"name":        nil,
	}
	return out
}

func filterResources(all []*resource.Resource, keep func(*resource.Resource) bool) []*resource.Resource {
	out := all[:0]
	for _, res := range all {
		if keep(res) {
			out = append(out, res)
		}
	}
	return out
}
