// This file was automatically generated. DO NOT EDIT.

package rdb

// API is the regional client.
type API struct{}

// ListInstances is a real operation.
func (s *API) ListInstances(req *ListInstancesRequest) (*ListInstancesResponse, error) {
	scwReq := &scw.ScalewayRequest{Method: "GET", Path: "/rdb/v1/instances"}
	return nil, s.client.Do(scwReq, nil)
}

type (
	ListInstancesRequest  struct{}
	ListInstancesResponse struct{}
)
