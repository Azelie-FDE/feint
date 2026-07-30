package machine

import "context"

// An emulator that leaves its machines behind is worse than one that starts
// none. The leftovers are not merely untidy: a bridge kept from a killed run
// holds a block the next run wants, and an instance whose network was swept
// underneath it lands in a state the runtime reports as ERROR. Both were
// observed on this project before this existed.
//
// Everything the emulator creates carries a label, so it can find its own work
// again without guessing from names, and without ever touching what an operator
// created. The Incus half is in incus_prune.go.

// LabelKey is the label every resource feint creates carries, so a sweep can
// tell its own work from an operator's.
const LabelKey = "feint.provider"

// Pruned counts what a sweep removed, so the caller can report it rather than
// claim success in silence.
type Pruned struct {
	Machines  int
	Networks  int
	Firewalls int
}

// Total reports whether anything was found at all.
func (p Pruned) Total() int { return p.Machines + p.Networks + p.Firewalls }

// Pruner is the optional half of a Driver that can find and remove everything
// the emulator created through it.
type Pruner interface {
	// Prune removes the machines, networks and rule sets carrying the label,
	// in that order: a network cannot go while a machine sits on it.
	Prune(ctx context.Context) (Pruned, error)
}
