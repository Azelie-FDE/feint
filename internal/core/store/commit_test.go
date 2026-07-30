package store_test

import (
	"testing"
	"time"

	"github.com/stephrobert/feint/internal/core/resource"
	"github.com/stephrobert/feint/internal/core/store"
)

// The resurrection this test pins was real, and it was found twice: once on
// Outscale, where a delete racing a boot brought the VM back, and once by an
// audit that found the same shape still alive in the two other packs.
//
// The sequence is the one every pack runs. A handler takes a copy, spends tens
// of seconds asking a runtime to start a machine, then writes the result back.
// If the write-back inserts unconditionally, the delete that landed in between
// is undone.
func TestCommitDoesNotResurrectADeletedResource(t *testing.T) {
	s := store.New()
	now := time.Now()

	s.Put(&resource.Resource{
		ID:     "srv-1",
		Kind:   "server",
		Tenant: resource.Tenant{Provider: "scaleway"},
		State:  "stopped",
		Attrs:  map[string]any{"name": "demo"},
	})

	// What a handler holds while it waits for the runtime.
	held, found := s.Get("scaleway", "server", "srv-1")
	if !found {
		t.Fatal("the resource was not stored")
	}
	held.State = "running"

	// The client deletes it in the meantime.
	if !s.Delete("scaleway", "server", "srv-1") {
		t.Fatal("the delete found nothing to remove")
	}

	if s.Commit(held, now) {
		t.Fatal("Commit reported success on a deleted resource: the caller will answer with a server that no longer exists")
	}
	if _, back := s.Get("scaleway", "server", "srv-1"); back {
		t.Fatal("the deleted resource came back: this is the resurrection Commit exists to prevent")
	}
}

// The ordinary path still has to work, or the fix above would be a way of
// dropping every update.
func TestCommitWritesBackWhatTheRuntimeChanged(t *testing.T) {
	s := store.New()
	now := time.Now().Add(time.Hour)

	s.Put(&resource.Resource{
		ID:     "srv-2",
		Kind:   "server",
		Tenant: resource.Tenant{Provider: "scaleway"},
		State:  "stopped",
		Attrs:  map[string]any{"name": "demo"},
	})

	held, _ := s.Get("scaleway", "server", "srv-2")
	held.State = "running"
	held.Runtime = map[string]string{"address": "10.0.0.2"}
	held.Attrs["name"] = "renamed"

	if !s.Commit(held, now) {
		t.Fatal("Commit failed on a resource that still exists")
	}

	stored, _ := s.Get("scaleway", "server", "srv-2")
	switch {
	case stored.State != "running":
		t.Fatalf("state is %q, want running", stored.State)
	case stored.Runtime["address"] != "10.0.0.2":
		t.Fatalf("address is %q, want 10.0.0.2", stored.Runtime["address"])
	case stored.Attrs["name"] != "renamed":
		t.Fatalf("name is %v, want renamed", stored.Attrs["name"])
	case !stored.Updated.Equal(now):
		t.Fatalf("Updated is %v, want %v", stored.Updated, now)
	}
}

// Commit must not hand out a pointer into the store, or a later mutation of the
// caller's copy would change stored state without passing the lock.
func TestCommitStoresACopy(t *testing.T) {
	s := store.New()

	s.Put(&resource.Resource{
		ID:     "srv-3",
		Kind:   "server",
		Tenant: resource.Tenant{Provider: "scaleway"},
		State:  "stopped",
	})

	held, _ := s.Get("scaleway", "server", "srv-3")
	held.State = "running"
	if !s.Commit(held, time.Now()) {
		t.Fatal("Commit failed")
	}

	held.State = "mutated after the commit"
	stored, _ := s.Get("scaleway", "server", "srv-3")
	if stored.State != "running" {
		t.Fatalf("the store followed a mutation made after Commit: state is %q", stored.State)
	}
}
