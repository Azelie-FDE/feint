package scaleway

import (
	"net/http"

	"github.com/stephrobert/feint/internal/core/emulator"
)

// A local emulator has no inventory of its own, but it cannot decline the
// catalogue either: `scw instance server create` reads the server types and the
// default image BEFORE it creates anything, and gives up on a 404. Declining
// these endpoints makes the official CLI unusable, which defeats the purpose.
//
// So the catalogue is served from a small, fixed table. It is fiction, and it is
// labelled as such in the docs, but it is the fiction the clients need.

// serverType mirrors instance.ServerType. Only the fields the CLI and the
// Terraform provider actually read are populated.
type serverType struct {
	MonthlyPrice float32  `json:"monthly_price"`
	HourlyPrice  float32  `json:"hourly_price"`
	AltNames     []string `json:"alt_names"`
	Ncpus        uint32   `json:"ncpus"`
	Gpu          uint64   `json:"gpu"`
	RAM          uint64   `json:"ram"`
	Arch         string   `json:"arch"`
	Baremetal    bool     `json:"baremetal"`
	Network      struct {
		Interfaces           []any  `json:"interfaces"`
		SumInternalBandwidth uint64 `json:"sum_internal_bandwidth"`
		SumInternetBandwidth uint64 `json:"sum_internet_bandwidth"`
		IPv6Support          bool   `json:"ipv6_support"`
	} `json:"network"`
	VolumesConstraint struct {
		MinSize uint64 `json:"min_size"`
		MaxSize uint64 `json:"max_size"`
	} `json:"volumes_constraint"`
	Capabilities struct {
		BlockStorage bool     `json:"block_storage"`
		BootTypes    []string `json:"boot_types"`
	} `json:"capabilities"`
}

func newServerType(cpus uint32, ramGiB uint64, hourly float32) *serverType {
	st := &serverType{
		MonthlyPrice: hourly * 24 * 30,
		HourlyPrice:  hourly,
		AltNames:     []string{},
		Ncpus:        cpus,
		RAM:          ramGiB << 30,
		Arch:         "x86_64",
	}
	st.Network.Interfaces = []any{}
	st.Network.SumInternalBandwidth = 100_000_000
	st.Network.SumInternetBandwidth = 100_000_000
	st.Network.IPv6Support = true
	// min_size stays at 0: the CLI sums the LOCAL volumes of a create request and
	// refuses anything below the minimum. Modern Scaleway types boot from block
	// storage and carry no local volume, so a non-zero minimum here would make
	// `scw instance server create` fail with "total local volume size must be
	// between ...". Types that really require local storage are not emulated.
	st.VolumesConstraint.MinSize = 0
	st.VolumesConstraint.MaxSize = 200_000_000_000
	st.Capabilities.BlockStorage = true
	st.Capabilities.BootTypes = []string{"local", "rescue"}
	return st
}

// catalogue is the fixed set of types the emulator accepts. Names are real
// Scaleway commercial types so copied documentation and existing Terraform code
// work unchanged.
var catalogue = map[string]*serverType{
	"PLAY2-PICO": newServerType(1, 2, 0.0086),
	"PLAY2-NANO": newServerType(2, 4, 0.0172),
	"DEV1-S":     newServerType(2, 2, 0.0084),
	"DEV1-M":     newServerType(3, 4, 0.0168),
	"DEV1-L":     newServerType(4, 8, 0.0336),
	"GP1-XS":     newServerType(4, 16, 0.0771),
	"PRO2-XXS":   newServerType(2, 8, 0.0301),
}

// defaultImageLabel is the image the CLI asks for when none is given.
const defaultImageLabel = "ubuntu_jammy"

func (p *Pack) listServerTypes(w http.ResponseWriter, r *http.Request) {
	if _, ok := zoneOf(w, r); !ok {
		return
	}
	emulator.WriteJSON(w, http.StatusOK, map[string]any{
		"servers":     catalogue,
		"total_count": len(catalogue),
	})
}

// listLocalImages answers the marketplace lookup the CLI performs to resolve an
// image label into an ID. Any label is accepted and mapped onto one stable
// image, because refusing an unknown label would only block a workflow the
// emulator has no opinion about.
func (p *Pack) listLocalImages(w http.ResponseWriter, r *http.Request) {
	label := r.URL.Query().Get("image_label")
	if label == "" {
		label = defaultImageLabel
	}
	zone := r.URL.Query().Get("zone")
	if zone == "" {
		zone = "fr-par-1"
	}

	compatible := make([]string, 0, len(catalogue))
	for name := range catalogue {
		compatible = append(compatible, name)
	}

	// A fixed UUID: Terraform stores the image ID in state, and a value that
	// changed between two runs would show up as a permanent diff.
	const imageID = "22222222-2222-4222-8222-222222222222"

	emulator.WriteJSON(w, http.StatusOK, map[string]any{
		"local_images": []map[string]any{{
			"id":                          imageID,
			"compatible_commercial_types": compatible,
			"arch":                        "x86_64",
			"zone":                        zone,
			"label":                       label,
			"type":                        "instance_sbs",
		}},
		"total_count": 1,
	})
}
