package store

import (
	"bytes"
	"testing"
	"time"

	"github.com/stephrobert/feint/internal/core/resource"
)

func res(id, kind, zone string, created time.Time) *resource.Resource {
	return &resource.Resource{
		ID:      id,
		Kind:    kind,
		Tenant:  resource.Tenant{Provider: "scaleway", Project: "proj", Zone: zone},
		State:   "stopped",
		Created: created,
		Updated: created,
		Attrs:   map[string]any{"name": id},
	}
}

func TestPutGetDelete(t *testing.T) {
	s := New()
	now := time.Unix(1700000000, 0).UTC()
	s.Put(res("a", "server", "fr-par-1", now))

	got, ok := s.Get("scaleway", "server", "a")
	if !ok {
		t.Fatal("expected the resource to exist")
	}
	if got.Attrs["name"] != "a" {
		t.Fatalf("unexpected attrs: %v", got.Attrs)
	}
	if _, ok := s.Get("scaleway", "server", "missing"); ok {
		t.Fatal("expected a miss for an unknown ID")
	}
	if !s.Delete("scaleway", "server", "a") {
		t.Fatal("expected Delete to report the resource existed")
	}
	if s.Delete("scaleway", "server", "a") {
		t.Fatal("expected the second Delete to report a miss")
	}
}

func TestGetReturnsACopy(t *testing.T) {
	s := New()
	now := time.Unix(1700000000, 0).UTC()
	s.Put(res("a", "server", "fr-par-1", now))

	got, _ := s.Get("scaleway", "server", "a")
	got.Attrs["name"] = "mutated"

	again, _ := s.Get("scaleway", "server", "a")
	if again.Attrs["name"] != "a" {
		t.Fatalf("mutating a returned resource leaked into the store: %v", again.Attrs)
	}
}

func TestListIsFilteredAndOrdered(t *testing.T) {
	s := New()
	base := time.Unix(1700000000, 0).UTC()
	s.Put(res("c", "server", "fr-par-1", base.Add(2*time.Second)))
	s.Put(res("a", "server", "fr-par-1", base))
	s.Put(res("b", "server", "nl-ams-1", base.Add(time.Second)))
	s.Put(res("v", "volume", "fr-par-1", base))

	all := s.List("server", resource.Tenant{Provider: "scaleway"})
	if len(all) != 3 {
		t.Fatalf("expected 3 servers, got %d", len(all))
	}
	if all[0].ID != "a" || all[1].ID != "b" || all[2].ID != "c" {
		t.Fatalf("expected creation order a,b,c, got %s,%s,%s", all[0].ID, all[1].ID, all[2].ID)
	}

	zoned := s.List("server", resource.Tenant{Provider: "scaleway", Zone: "fr-par-1"})
	if len(zoned) != 2 {
		t.Fatalf("expected 2 servers in fr-par-1, got %d", len(zoned))
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	s := New()
	now := time.Unix(1700000000, 0).UTC()
	s.Put(res("a", "server", "fr-par-1", now))
	s.Put(res("b", "server", "fr-par-1", now.Add(time.Second)))

	var buf bytes.Buffer
	if err := s.Snapshot(&buf); err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	restored := New()
	if err := restored.Restore(&buf); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restored.Len() != 2 {
		t.Fatalf("expected 2 resources after restore, got %d", restored.Len())
	}
	got, ok := restored.Get("scaleway", "server", "b")
	if !ok || !got.Created.Equal(now.Add(time.Second)) {
		t.Fatalf("restored resource lost its timestamps: %+v", got)
	}
}
