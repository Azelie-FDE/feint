package scaleway_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// serverWith creates a server and returns its id and its decoded body.
func serverWith(t *testing.T, ts *httptest.Server, body string) (string, map[string]any) {
	t.Helper()
	status, created := do(t, ts, "POST", zoneURL+"/servers", body)
	if status != http.StatusCreated {
		t.Fatalf("create server: expected 201, got %d (%v)", status, created)
	}
	server, _ := created["server"].(map[string]any)
	id, _ := server["id"].(string)
	if id == "" {
		t.Fatalf("create server: no id in %v", created)
	}
	return id, server
}

// A server always carries a root volume under key "0". The Terraform provider
// reads it there and sizes the rest with len(volumes)-1, so an empty map is not
// a missing field: it panics the plugin.
func TestServerCarriesARootVolume(t *testing.T) {
	ts := newTestServer(t)

	_, server := serverWith(t, ts, `{"name":"vol","commercial_type":"DEV1-S"}`)
	volumes, _ := server["volumes"].(map[string]any)
	if len(volumes) != 1 {
		t.Fatalf("expected exactly the root volume, got %v", server["volumes"])
	}
	root, ok := volumes["0"].(map[string]any)
	if !ok {
		t.Fatalf(`the root volume is not keyed "0": %v`, volumes)
	}

	volumeID, _ := root["id"].(string)
	if volumeID == "" {
		t.Fatalf("the root volume has no id: %v", root)
	}
	// A local volume would make the CLI refuse the creation it just asked for,
	// because it sums local volumes against the catalogue constraint.
	if root["volume_type"] != "b_ssd" {
		t.Errorf("root volume type is %v, want b_ssd", root["volume_type"])
	}

	// Readable through the volumes endpoint: the provider fetches it by id right
	// after the create, and a 404 there fails the apply.
	status, got := do(t, ts, "GET", zoneURL+"/volumes/"+volumeID, "")
	if status != http.StatusOK {
		t.Fatalf("get volume: expected 200, got %d (%v)", status, got)
	}
	vol, _ := got["volume"].(map[string]any)
	attached, _ := vol["server"].(map[string]any)
	if attached == nil || attached["name"] != "vol" {
		t.Errorf("the volume does not name its server: %v", vol)
	}
}

// Deleting a server detaches its volumes and keeps them: on Scaleway the disk
// outlives the machine, and the CLI polls each volume after the server is gone.
func TestDeletingAServerKeepsItsVolume(t *testing.T) {
	ts := newTestServer(t)

	id, server := serverWith(t, ts, `{"name":"keep","commercial_type":"DEV1-S"}`)
	volumes, _ := server["volumes"].(map[string]any)
	root, _ := volumes["0"].(map[string]any)
	volumeID, _ := root["id"].(string)

	if status, _ := do(t, ts, "DELETE", zoneURL+"/servers/"+id, ""); status != http.StatusNoContent {
		t.Fatalf("delete server: expected 204, got %d", status)
	}

	status, got := do(t, ts, "GET", zoneURL+"/volumes/"+volumeID, "")
	if status != http.StatusOK {
		t.Fatalf("the volume vanished with its server: get returned %d (%v)", status, got)
	}
	vol, _ := got["volume"].(map[string]any)
	if vol["server"] != nil {
		t.Errorf("the volume is still attached to a deleted server: %v", vol["server"])
	}

	// Detached, it can now be deleted, which is what the CLI does next.
	if status, _ := do(t, ts, "DELETE", zoneURL+"/volumes/"+volumeID, ""); status != http.StatusNoContent {
		t.Errorf("delete a detached volume: expected 204, got %d", status)
	}
}

// An attached volume cannot be deleted, and a client that destroys in the wrong
// order depends on that error to retry.
func TestAttachedVolumeRefusesDeletion(t *testing.T) {
	ts := newTestServer(t)

	_, server := serverWith(t, ts, `{"name":"busy","commercial_type":"DEV1-S"}`)
	volumes, _ := server["volumes"].(map[string]any)
	root, _ := volumes["0"].(map[string]any)
	volumeID, _ := root["id"].(string)

	status, denied := do(t, ts, "DELETE", zoneURL+"/volumes/"+volumeID, "")
	if status != http.StatusBadRequest {
		t.Fatalf("delete an attached volume: expected 400, got %d", status)
	}
	if denied["type"] != "precondition_failed" {
		t.Errorf("got error type %v, want precondition_failed", denied["type"])
	}
}

// The image a client asks for must come back as an object, never null: the
// provider reads server.Image without checking and crashes on a null.
func TestServerCarriesItsImage(t *testing.T) {
	ts := newTestServer(t)

	_, server := serverWith(t, ts, `{"name":"img","commercial_type":"DEV1-S","image":"debian_bookworm"}`)
	image, ok := server["image"].(map[string]any)
	if !ok {
		t.Fatalf("the server carries no image object: %v", server["image"])
	}
	// A label is echoed back as the image name, so a client that asked for
	// Debian does not read Ubuntu.
	if image["name"] != "debian_bookworm" {
		t.Errorf("image name is %v, want debian_bookworm", image["name"])
	}
	if image["id"] == nil || image["id"] == "" {
		t.Errorf("the image has no id: %v", image)
	}
	if root, _ := image["root_volume"].(map[string]any); root == nil {
		t.Errorf("the image has no root volume: %v", image)
	}
}
