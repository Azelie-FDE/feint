package machine

import "context"

// Two emulated subnets are two real subnets, and upstream they do not reach each
// other. Runtimes disagree on which way round that is: some join networks by
// default and must be separated by rules, others keep them apart and must be
// joined on request. The two interfaces below are that fork, and a pack asks
// which one it is talking to rather than assuming.
//
// The Incus halves of both are in incus_isolate.go, with the measurements that
// made the OVN mode necessary.

// Isolator is the optional half of a Driver whose networks are born joined, and
// that can keep them apart with rules.
type Isolator interface {
	// IsolateNetwork rejects traffic from the network towards each foreign
	// block, and leaves everything else alone. Called again with a different
	// list, it replaces the previous one: blocks appear and disappear as
	// subnets are created and deleted.
	IsolateNetwork(ctx context.Context, network string, foreign []string) error
}

// Peerer is the optional half of a Driver whose networks are born separate
// and joined on request — the exact inverse of Isolator. A pack asks
// NativeIsolation to know which of the two it is talking to: with a Peerer that
// answers true, reject rules against foreign blocks are dead weight, and
// reachability is granted by peering instead.
type Peerer interface {
	// NativeIsolation reports whether two networks of this driver are
	// unreachable from each other unless peered.
	NativeIsolation() bool
	// PeerNetworks makes the network reach exactly the given peers: missing
	// peerings are created, in both their halves, and stale ones removed.
	PeerNetworks(ctx context.Context, network string, peers []string) error
}
