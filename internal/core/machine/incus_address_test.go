package machine

import "testing"

// The route lookup is over an exact comma-separated entry: a substring match
// would see 203.0.113.2/32 inside 203.0.113.22/32 and undo the wrong address.
func TestRouteListContains(t *testing.T) {
	tests := []struct {
		routes string
		route  string
		want   bool
	}{
		{"203.0.113.2/32", "203.0.113.2/32", true},
		{"10.0.0.0/24, 203.0.113.2/32", "203.0.113.2/32", true},
		{"203.0.113.22/32", "203.0.113.2/32", false},
		{"", "203.0.113.2/32", false},
	}
	for _, tt := range tests {
		if got := routeListContains(tt.routes, tt.route); got != tt.want {
			t.Errorf("routeListContains(%q, %q) = %v, want %v", tt.routes, tt.route, got, tt.want)
		}
	}
}
