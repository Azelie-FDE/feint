package scaleway_test

import (
	"net/http"
	"sync"
	"testing"
)

// Terraform creates with a parallelism of ten by default, so two servers joining
// the same private network within one apply is the nominal case, not a corner.
// Rebuilding the allocator per request is correct on its own and useless without
// a lock: both requests start from the same state and receive the same address.
func TestConcurrentAttachmentsGetDistinctAddresses(t *testing.T) {
	ts := newTestServer(t)
	pnID, _ := privateNetwork(t, ts, `{"name":"race","subnets":["10.150.0.0/24"]}`)

	const workers = 8
	servers := make([]string, workers)
	for i := range servers {
		servers[i], _ = serverWith(t, ts, `{"name":"race","commercial_type":"DEV1-S"}`)
	}

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		addresses = make(map[string]string, workers)
	)
	for _, serverID := range servers {
		wg.Add(1)
		go func(serverID string) {
			defer wg.Done()
			status, created := do(t, ts, "POST",
				zoneURL+"/servers/"+serverID+"/private_nics", `{"private_network_id":"`+pnID+`"}`)
			if status != http.StatusCreated {
				t.Errorf("attach: expected 201, got %d (%v)", status, created)
				return
			}
			nic, _ := created["private_nic"].(map[string]any)
			address := nicAddress(t, ts, nic)

			mu.Lock()
			defer mu.Unlock()
			if other, taken := addresses[address]; taken {
				t.Errorf("address %s handed to both %s and %s", address, other, serverID)
			}
			addresses[address] = serverID
		}(serverID)
	}
	wg.Wait()

	if len(addresses) != workers {
		t.Errorf("got %d distinct addresses for %d attachments", len(addresses), workers)
	}
}

// The flexible address used to be (Store.Len() %% 250) + 2, which counts every
// resource of every kind: create an IP, create and delete anything, create
// another, and the same public address is handed out twice.
func TestFlexibleAddressesAreNeverReused(t *testing.T) {
	ts := newTestServer(t)

	seen := map[string]bool{}
	for i := 0; i < 6; i++ {
		status, created := do(t, ts, "POST", zoneURL+"/ips", `{}`)
		if status != http.StatusCreated {
			t.Fatalf("create ip: expected 201, got %d (%v)", status, created)
		}
		ip, _ := created["ip"].(map[string]any)
		address, _ := ip["address"].(string)
		if address == "" {
			t.Fatalf("no address in %v", created)
		}
		if seen[address] {
			t.Fatalf("address %s handed out twice", address)
		}
		seen[address] = true

		// Churn of another kind, which the old counter mistook for a released
		// address.
		serverID, _ := serverWith(t, ts, `{"name":"churn","commercial_type":"DEV1-S"}`)
		do(t, ts, "DELETE", zoneURL+"/servers/"+serverID, "")
	}
}

// Concurrent creates must not collide either.
func TestConcurrentFlexibleAddressesAreDistinct(t *testing.T) {
	ts := newTestServer(t)

	const workers = 10
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		seen = make(map[string]bool, workers)
	)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, created := do(t, ts, "POST", zoneURL+"/ips", `{}`)
			if status != http.StatusCreated {
				t.Errorf("create ip: got %d (%v)", status, created)
				return
			}
			ip, _ := created["ip"].(map[string]any)
			address, _ := ip["address"].(string)

			mu.Lock()
			defer mu.Unlock()
			if seen[address] {
				t.Errorf("address %s handed out twice", address)
			}
			seen[address] = true
		}()
	}
	wg.Wait()

	if len(seen) != workers {
		t.Errorf("got %d distinct addresses for %d creates", len(seen), workers)
	}
}
