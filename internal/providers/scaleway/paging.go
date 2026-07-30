package scaleway

import (
	"net/http"
	"strconv"
)

// Scaleway paginates with page (1-based) and a page size, and returns the full
// count in total_count. 330 of the 336 List*Response structs in the Go SDK carry
// that field, so this helper covers virtually every list endpoint and any future
// product that follows the house convention.
//
// The size is spelled twice, and that is the whole difficulty. instance/v1 sends
// `per_page` (`parameter.AddToQuery(query, "per_page", req.PerPage)` throughout
// its generated client); vpc/v2 and ipam/v1, which are newer, send `page_size`.
// This helper read only the second, so every list route of instance/v1 served
// fifty items whatever the client asked for — `scw instance server list
// per-page=2` returned five. Nothing caught it: the conformance suite never
// varies the page size, the probe drives routes rather than parameters, and
// query strings are not part of the contract check.
//
// Both are read, first one present wins. No client sends both, and a helper that
// refused the spelling its own SDK uses would be wrong in the only way that
// matters here.

const (
	defaultPageSize = 50
	maxPageSize     = 100
)

type pageRequest struct {
	page int
	size int
}

// parsePage reads the pagination query parameters, ignoring malformed values the
// way the API does rather than rejecting the request.
func parsePage(r *http.Request) pageRequest {
	q := r.URL.Query()
	p := pageRequest{page: 1, size: defaultPageSize}
	if v, err := strconv.Atoi(q.Get("page")); err == nil && v > 0 {
		p.page = v
	}
	for _, key := range []string{"per_page", "page_size"} {
		if v, err := strconv.Atoi(q.Get(key)); err == nil && v > 0 {
			p.size = min(v, maxPageSize)
			break
		}
	}
	return p
}

// slice returns the window of items for the request. An out-of-range page yields
// an empty slice, never an error: that is what the real API does and what the
// SDK pagination loop relies on to terminate.
func (p pageRequest) slice(total int) (start, end int) {
	// Bounded before the multiplication rather than after it. page comes from
	// the query string and is only checked for being positive, so (page-1)*size
	// overflows for a large one: page=922337203685477581 with size=100 gave
	// start = -80, and all[start:end] panicked the handler. net/http swallows
	// the panic, so the client got no response at all, on every list route this
	// package serves.
	if p.page > 1 && p.page-1 > total/p.size {
		return total, total
	}
	start = (p.page - 1) * p.size
	if start > total {
		start = total
	}
	end = min(start+p.size, total)
	return start, end
}
