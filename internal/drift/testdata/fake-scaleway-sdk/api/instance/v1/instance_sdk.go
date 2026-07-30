// This file was automatically generated. DO NOT EDIT.
// Fixture mimicking the layout of a real Scaleway SDK product, including the
// traps a regex-based scanner would fall for. Generated methods build their own
// scw.ScalewayRequest, which is what makes them surface.

package instance

// API is the zone-scoped client.
type API struct{}

// ListServers is a real operation.
func (s *API) ListServers(req *ListServersRequest) (*ListServersResponse, error) {
	scwReq := &scw.ScalewayRequest{Method: "GET", Path: "/instance/v1/servers"}
	return nil, s.client.Do(scwReq, nil)
}

// CreateServer is a real operation.
func (s *API) CreateServer(req *CreateServerRequest) (*CreateServerResponse, error) {
	scwReq := &scw.ScalewayRequest{Method: "POST", Path: "/instance/v1/servers"}
	return nil, s.client.Do(scwReq, nil)
}

// GetServer is a real operation, and the one the pollers below lean on.
func (s *API) GetServer(req *GetServerRequest) (*GetServerResponse, error) {
	scwReq := &scw.ScalewayRequest{Method: "GET", Path: "/instance/v1/servers/x"}
	return nil, s.client.Do(scwReq, nil)
}

// ServerAction is a real operation.
func (s *API) ServerAction(req *ServerActionRequest) error {
	scwReq := &scw.ScalewayRequest{Method: "POST", Path: "/instance/v1/servers/x/action"}
	return s.client.Do(scwReq, nil)
}

// Trap 1: a comment that looks like a declaration.
// func (s *API) GhostFromAComment(req *Request) error

// Trap 2: the same text inside a string literal.
const decoy = "func (s *API) GhostFromAString(req *Request) error"

// Trap 3: an unexported method is not part of the public surface.
func (s *API) internalHelper() {}

// Trap 4: a method on a non-API receiver.
type helper struct{}

func (h *helper) NotAnOperation() {}

// Trap 5: a constant accessor, which reaches nothing at all. Zones and Regions
// are the real ones, and there are sixty-one of them upstream.
func (s *API) Zones() []string { return []string{"fr-par-1"} }

// A second receiver type ending in API counts, as ZonedAPI does upstream.
type ZonedAPI struct{}

// ListVolumes is served by the zoned client.
func (s *ZonedAPI) ListVolumes(req *ListVolumesRequest) (*ListVolumesResponse, error) {
	scwReq := &scw.ScalewayRequest{Method: "GET", Path: "/instance/v1/volumes"}
	return nil, s.client.Do(scwReq, nil)
}

type (
	ListServersRequest   struct{}
	ListServersResponse  struct{}
	CreateServerRequest  struct{}
	CreateServerResponse struct{}
	GetServerRequest     struct{}
	GetServerResponse    struct{}
	ServerActionRequest  struct{}
	ListVolumesRequest   struct{}
	ListVolumesResponse  struct{}
)
