package machine

import "testing"

func TestPrunedTotal(t *testing.T) {
	if (Pruned{}).Total() != 0 {
		t.Error("an empty sweep totals zero")
	}
	if got := (Pruned{Machines: 2, Networks: 1, Firewalls: 3}).Total(); got != 6 {
		t.Errorf("Total() = %d, want 6", got)
	}
}
