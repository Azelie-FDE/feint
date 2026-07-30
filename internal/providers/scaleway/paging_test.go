package scaleway

import (
	"math"
	"testing"
)

// A page number large enough to overflow (page-1)*size made start negative, and
// the handler panicked on the slice. net/http swallows a panic, so the client
// got no response at all — no status, no body — on every list route this
// package serves. Reachable by anyone who can reach the emulator, with one
// query parameter.
func TestSliceSurvivesAnOverflowingPage(t *testing.T) {
	cases := []struct{ page, size, total int }{
		{922337203685477581, 100, 5},
		{1 << 62, 1, 3},
		{math.MaxInt, 100, 0},
		{math.MaxInt, 1, 1},
	}
	for _, tc := range cases {
		start, end := pageRequest{page: tc.page, size: tc.size}.slice(tc.total)
		if start < 0 || end < 0 || start > tc.total || end > tc.total || start > end {
			t.Errorf("page=%d size=%d total=%d gave [%d:%d], which is not a window",
				tc.page, tc.size, tc.total, start, end)
		}
	}
}

// The ordinary windows still have to be right: an out-of-range page yields an
// empty slice rather than an error, which is what the SDK's pagination loop
// relies on to terminate.
func TestSliceWindows(t *testing.T) {
	cases := []struct {
		page, size, total  int
		wantStart, wantEnd int
	}{
		{1, 50, 0, 0, 0},
		{1, 2, 5, 0, 2},
		{2, 2, 5, 2, 4},
		{3, 2, 5, 4, 5},
		{4, 2, 5, 5, 5},
		{99, 2, 5, 5, 5},
	}
	for _, tc := range cases {
		start, end := pageRequest{page: tc.page, size: tc.size}.slice(tc.total)
		if start != tc.wantStart || end != tc.wantEnd {
			t.Errorf("page=%d size=%d total=%d gave [%d:%d], want [%d:%d]",
				tc.page, tc.size, tc.total, start, end, tc.wantStart, tc.wantEnd)
		}
	}
}
