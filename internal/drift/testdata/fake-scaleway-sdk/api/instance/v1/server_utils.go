// Hand-written helpers, as Scaleway ships alongside the generated file: some
// are the public entry point of a generated call, others only compose calls
// already counted. Only the first kind is surface.

package instance

// UpdateServer is the public entry point callers actually use. It builds no
// request of its own and must still count, because it is the only name for the
// endpoint the generated updateServer serves.
func (s *API) UpdateServer(req *UpdateServerRequest) (*UpdateServerResponse, error) {
	return s.updateServer(req)
}

// updateServer is the generated call; it is not part of the public surface.
func (s *API) updateServer(req *UpdateServerRequest) (*UpdateServerResponse, error) {
	scwReq := &scw.ScalewayRequest{Method: "PATCH", Path: "/instance/v1/servers/x"}
	return nil, s.client.Do(scwReq, nil)
}

// WaitForServer polls until the server settles. Client-side convenience, not an
// API operation, so it must not inflate the surface.
func (s *API) WaitForServer(req *WaitForServerRequest) error {
	for {
		if _, err := s.GetServer(&GetServerRequest{}); err != nil {
			return err
		}
	}
}

// ServerActionAndWait composes two operations already counted, which is the
// shape that used to be mistaken for an endpoint of its own.
func (s *API) ServerActionAndWait(req *ServerActionRequest) error {
	if err := s.ServerAction(req); err != nil {
		return err
	}
	return s.WaitForServer(&WaitForServerRequest{})
}

type (
	UpdateServerRequest  struct{}
	UpdateServerResponse struct{}
	WaitForServerRequest struct{}
)
