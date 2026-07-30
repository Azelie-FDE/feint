package machine

import "sync"

// Serialising the lifecycle of one target is the half of the concurrency story
// the store lock cannot cover.
//
// Launching a container takes tens of seconds, so no pack holds the store lock
// across it: it reads a copy, calls the runtime, and writes the result back with
// Commit. Commit makes that write conditional, so a resource deleted meanwhile
// stays deleted. What it does not do is order two writers on the same resource.
//
// Two concurrent poweron on one server therefore each called the runtime, and
// the loser overwrote the winner's state: the second launch failed on a name the
// first had already taken, so the API ended up describing "stopped" a container
// that was running. A machine the control plane does not describe is a machine
// nobody thinks to stop, which is the exact failure this emulator exists to
// avoid reporting.
//
// The exclusion is named rather than widened. One lock per target, never one
// global lock: a global one would queue every server in the session behind a
// single launch, turning a correctness fix into a throughput bug. attachMu in
// the Incus driver is the same idea one layer down, for interface allocation.

// targetLocks is the set of targets currently held or waited on.
var targetLocks = &keyedMutex{held: make(map[string]*heldLock)}

type heldLock struct {
	mu sync.Mutex
	// refs counts holders and waiters, so the entry outlives the first
	// unlock only while somebody still needs it.
	refs int
}

type keyedMutex struct {
	mu   sync.Mutex
	held map[string]*heldLock
}

// lock blocks until key is free and returns the release.
//
// The entry is dropped once the last waiter is gone. Keeping it would grow the
// map by one mutex for every resource id a session ever touched, and an
// emulator that a client can turn into a memory sink is a defect of the same
// family as the ones this lock closes.
func (k *keyedMutex) lock(key string) func() {
	k.mu.Lock()
	l, ok := k.held[key]
	if !ok {
		l = &heldLock{}
		k.held[key] = l
	}
	l.refs++
	k.mu.Unlock()

	l.mu.Lock()

	// Once, because a caller that both defers the release and calls it on an
	// early return would otherwise unlock a mutex it no longer holds, which
	// panics and takes the server with it.
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Unlock()

			k.mu.Lock()
			l.refs--
			if l.refs == 0 {
				delete(k.held, key)
			}
			k.mu.Unlock()
		})
	}
}

// Serialise excludes every other action on the same target until the returned
// function is called. A pack takes it before it reads the resource and holds it
// past the write-back, because the read, the runtime call and the Commit are one
// operation even though only the last of them touches the store.
//
//	unlock := p.binding().Serialise(id)
//	defer unlock()
//
// It is safe with no runtime configured, and packs take it there too: the window
// is shorter without a driver, not absent, and a lock a pack takes only in one
// mode is a lock somebody removes from the other.
//
// The key carries the provider, so two packs numbering their resources the same
// way do not wait on each other.
func (b Binding) Serialise(id string) func() {
	return targetLocks.lock(b.Provider + "|" + id)
}
