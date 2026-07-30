package machine

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The property is exclusion on one target, and nothing measured it. The audit of
// 2026-07-29 found two concurrent poweron each calling the runtime, and the
// project's own instructions had already named the fix — a lock per target name,
// never a global one — which is how a documented fix survives months without
// existing.
func TestSerialiseExcludesTwoActionsOnOneTarget(t *testing.T) {
	b := Binding{Provider: "scaleway"}

	var inside atomic.Int32
	var overlap atomic.Bool
	var wg sync.WaitGroup

	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := b.Serialise("srv-1")
			defer unlock()

			if inside.Add(1) > 1 {
				overlap.Store(true)
			}
			// Long enough that an unserialised caller is inside at the same
			// time rather than merely likely to be.
			time.Sleep(2 * time.Millisecond)
			inside.Add(-1)
		}()
	}
	wg.Wait()

	if overlap.Load() {
		t.Fatal("two actions ran on one target at the same time")
	}
}

// The accepting half, and it is not decoration: a single global lock passes the
// test above and puts every server of the session in a queue behind one launch,
// which takes tens of seconds. Terraform creates ten resources at a time by
// default, so that is the ordinary case, not the edge one.
func TestSerialiseDoesNotQueueDifferentTargets(t *testing.T) {
	b := Binding{Provider: "scaleway"}

	const targets = 4
	var arrived sync.WaitGroup
	arrived.Add(targets)
	release := make(chan struct{})
	done := make(chan struct{})

	for i := range targets {
		go func() {
			unlock := b.Serialise(string(rune('a' + i)))
			defer unlock()
			arrived.Done()
			<-release
		}()
	}

	go func() {
		arrived.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("four different targets could not be held at once: the lock is global, not per target")
	}
	close(release)
}

// Two packs may number their resources the same way, and one waiting on the
// other would be a coupling nothing in the neutral core is allowed to create.
func TestSerialiseSeparatesProviders(t *testing.T) {
	scw := Binding{Provider: "scaleway"}
	osc := Binding{Provider: "outscale"}

	unlock := scw.Serialise("1")
	defer unlock()

	done := make(chan struct{})
	go func() {
		release := osc.Serialise("1")
		release()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("outscale waited on a scaleway target of the same id")
	}
}

// A map that keeps one mutex per id a session ever touched is the memory sink an
// emulator must not become. The refcount is what stops it, and it is exactly the
// kind of bookkeeping that reads as correct and leaks.
func TestSerialiseForgetsATargetOnceNobodyHoldsIt(t *testing.T) {
	b := Binding{Provider: "scaleway"}

	for i := range 100 {
		unlock := b.Serialise(string(rune('a' + i%26)))
		unlock()
	}

	targetLocks.mu.Lock()
	held := len(targetLocks.held)
	targetLocks.mu.Unlock()

	if held != 0 {
		t.Fatalf("%d target locks are still held after every caller left", held)
	}
}

// A release called twice unlocks a mutex the caller no longer holds, which
// panics. Ordinary Go style produces it: a defer plus an explicit call on an
// early return.
func TestReleaseIsIdempotent(t *testing.T) {
	b := Binding{Provider: "scaleway"}

	unlock := b.Serialise("srv-1")
	unlock()
	unlock()

	// And the target is still usable afterwards: a double release must not
	// leave the lock held by a ghost.
	again := b.Serialise("srv-1")
	again()
}
