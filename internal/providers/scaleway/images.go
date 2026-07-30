package scaleway

import (
	"net/http"

	"github.com/stephrobert/feint/internal/core/emulator"
)

// imageID is the single image the emulator knows about, matching the one the
// marketplace lookup returns. Fixed on purpose: Terraform keeps the image ID in
// state, and a value that changed between runs would show as a permanent diff.
const imageID = "22222222-2222-4222-8222-222222222222"

// getImage answers the lookup the CLI performs after resolving a label. Any ID
// is served as the same image: the emulator has no image catalogue and refusing
// unknown IDs would only break scripts that hardcode a real one.
func (p *Pack) getImage(w http.ResponseWriter, r *http.Request) {
	zone, ok := zoneOf(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		id = imageID
	}
	emulator.WriteJSON(w, http.StatusOK, map[string]any{"image": p.imageView(zone, id, defaultImageLabel)})
}

// imageView is the shape the SDK decodes into instance.Image, shared by the
// image endpoint and by the image a server carries.
//
// A server must carry the object, never null. The Terraform provider reads the
// image of the server it just created without checking, so a null there does not
// produce a diff: it crashes the plugin, which surfaces as "Plugin did not
// respond" with nothing in the emulator's log.
// imageEpoch is when the emulated images were "built". A fixed instant, because
// the catalogue is fixed.
const imageEpoch = "2025-01-01T00:00:00Z"

func (p *Pack) imageView(zone, id, label string) map[string]any {
	// Fixed, not the wall clock. The catalogue is stable across runs by design,
	// and a real image's dates do not move: stamping each read with Now() gave a
	// client a modification_date that changed every time it looked, which is a
	// permanent diff for anything that compares.
	stamp := imageEpoch
	return map[string]any{
		"id":                id,
		"name":              label,
		"arch":              "x86_64",
		"creation_date":     stamp,
		"modification_date": stamp,
		"organization":      defaultOrganization,
		"project":           defaultProject,
		"public":            true,
		"state":             "available",
		"tags":              []string{},
		"zone":              zone,
		"extra_volumes":     map[string]any{},
		"from_server":       nil,
		"root_volume": map[string]any{
			"id":          "33333333-3333-4333-8333-333333333333",
			"name":        label + "-root",
			"size":        20_000_000_000,
			"volume_type": "b_ssd",
		},
	}
}

// resolveImage maps what a create request put in `image` onto the pair the
// emulator needs: the ID to publish, and the label the machine driver turns into
// a base image.
//
// Clients send either form. The Terraform provider resolves the label through
// the marketplace first and sends a UUID; the CLI can send the label itself.
// Telling them apart is what lets `image = "debian_bookworm"` boot a Debian
// rather than the default.
func resolveImage(requested string) (id, label string) {
	switch {
	case requested == "":
		return imageID, defaultImageLabel
	case looksLikeUUID(requested):
		return requested, defaultImageLabel
	default:
		return imageID, requested
	}
}

func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}
