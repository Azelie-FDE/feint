package machine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// How Incus reports what it is doing, and what of it is worth repeating. The
// contract is in watch.go.

// incusEvent is the shape `incus monitor --format json` emits, one per line.
type incusEvent struct {
	Type     string `json:"type"`
	Metadata struct {
		Action  string         `json:"action"`
		Name    string         `json:"name"`
		Message string         `json:"message"`
		Level   string         `json:"level"`
		Context map[string]any `json:"context"`
	} `json:"metadata"`
}

// Watch implements Watcher.
//
// The stream is filtered on the emulator's own naming, which is the only thing
// an event carries that ties it back to us: instances are prefixed, and so are
// the networks. Logging events are kept only from warning upwards, because the
// daemon is chatty at info and drowning the emulator's log would defeat the
// purpose.
func (d *Incus) Watch(ctx context.Context) (<-chan Event, error) {
	binary := d.Binary
	if binary == "" {
		binary = "incus"
	}

	// --type is a repeatable flag, not a list: "--type=lifecycle,logging"
	// subscribes to an event type named "lifecycle,logging", which does not
	// exist, and the stream stays silent forever. Measured on incus 7.2, and
	// it is exactly the failure mode a filter should never have.
	//nolint:gosec // fixed binary, fixed arguments, never shell-interpreted
	cmd := exec.CommandContext(ctx, binary, "monitor", "--type=lifecycle", "--type=logging", "--format=json")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("watch runtime: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("watch runtime: %w", err)
	}

	events := make(chan Event, 32)
	go func() {
		defer close(events)
		defer func() { _ = cmd.Wait() }()

		scanner := bufio.NewScanner(stdout)
		// A daemon message can be long; the default 64k limit would split it
		// and produce two undecodable halves.
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			event, ok := decodeIncusEvent(scanner.Bytes())
			if !ok {
				continue
			}
			select {
			case events <- event:
			case <-ctx.Done():
				return
			}
		}

		// A stream that ends while nobody asked it to means the operator has
		// silently lost runtime visibility, typically because the daemon
		// restarted or a line overflowed the scanner. Said in the stream itself,
		// because the closed channel alone reads as a normal shutdown.
		if ctx.Err() == nil {
			message := "the runtime event stream ended; runtime messages are no longer reported"
			if err := scanner.Err(); err != nil {
				message = "the runtime event stream failed: " + err.Error()
			}
			select {
			case events <- Event{Kind: "logging", Level: "warning", Message: message}:
			default:
			}
		}
	}()
	return events, nil
}

// decodeIncusEvent converts one line, reporting false for anything that is not
// ours or not worth reporting.
func decodeIncusEvent(line []byte) (Event, bool) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 || line[0] != '{' {
		return Event{}, false
	}

	var raw incusEvent
	if err := json.Unmarshal(line, &raw); err != nil {
		return Event{}, false
	}

	event := Event{
		Kind:     raw.Type,
		Level:    raw.Metadata.Level,
		Action:   raw.Metadata.Action,
		Resource: raw.Metadata.Name,
		Message:  raw.Metadata.Message,
	}

	switch raw.Type {
	case "lifecycle":
		if !ours(event.Resource) {
			return Event{}, false
		}
	case "logging":
		switch raw.Metadata.Level {
		case "warning", "error", "panic", "fatal":
		default:
			return Event{}, false
		}
		// A logging event names its subject in the context rather than in a
		// field of its own.
		if event.Resource == "" {
			event.Resource = subjectOfContext(raw.Metadata.Context)
		}
		if !ours(event.Resource) && !mentionsOurs(event.Message, raw.Metadata.Context) {
			return Event{}, false
		}
	default:
		return Event{}, false
	}
	return explainStateRace(event, raw.Metadata.Context), true
}

// The daemon's wording, verbatim from Incus (driver_lxc.go), for the two state
// reads that lose a race with an instance's own lifecycle. Matching the exact
// strings is deliberate: the same messages with a different cause must keep
// their alarm.
const (
	netlinkRaceMessage     = "Failed to retrieve network information via netlink"
	storagePoolMessage     = "Error loading storage pool"
	storagePoolRaceMissing = "Instance storage pool not found"
)

// explainStateRace rewrites the daemon errors that are not ones.
//
// While a stop operation holds the instance lock, the daemon keeps answering
// "Running" (statusCode in Incus's driver_lxc.go), so a concurrent `incus list`
// still walks /proc of an init that just died: it logs the netlink failure,
// and once the pid is fully gone the list renders the instance as a bare ERROR
// row until the stop completes. The same list racing a create or a delete finds
// the instance record without its storage volume and logs the pool failure.
// Measured on incus 7.2: ten launch/stop/delete cycles produced both on every
// cycle, and the instance was healthy every time. So they are reported as
// warnings that say what the operator will see, instead of echoing alarms
// about a machine that is merely being set up or torn down.
func explainStateRace(event Event, fields map[string]any) Event {
	if event.Kind != "logging" {
		return event
	}
	errText, _ := fields["err"].(string)
	switch {
	case event.Message == netlinkRaceMessage:
		event.Level = "warning"
		event.Message = "a state read raced the teardown of " + event.Resource +
			"; incus list may show a transient ERROR row until the stop completes (harmless)"
	case event.Message == storagePoolMessage && strings.Contains(errText, storagePoolRaceMissing):
		event.Level = "warning"
		event.Message = "a state read raced the creation or deletion of " + event.Resource +
			" before its storage volume existed (harmless)"
	}
	return event
}

// ourPrefixes are the names the emulator gives what it creates.
//
// It used to list "scw-", "osc-" and "exo-" — the three provider abbreviations,
// written into the neutral core, so a fourth pack would have had to edit
// internal/core for its own events to be reported. The core must never know a
// provider exists, and that is exactly how the rule gets broken quietly.
//
// Two markers now, both the emulator's own: machines are named through
// Binding.Prefix and networks through NetworkPrefix. Which provider owns a
// resource is carried by its label, not by its name.
var ourPrefixes = []string{"feint-", NetworkPrefix + "-"}

func ours(name string) bool {
	for _, prefix := range ourPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func subjectOfContext(fields map[string]any) string {
	for _, key := range []string{"instance", "network", "name", "device"} {
		if value, found := fields[key].(string); found && value != "" {
			return value
		}
	}
	return ""
}

// mentionsOurs catches the messages that name a resource inside their text,
// which is how the daemon reports a bridge whose DHCP service failed to bind.
func mentionsOurs(message string, fields map[string]any) bool {
	haystack := message
	for _, value := range fields {
		if text, ok := value.(string); ok {
			haystack += " " + text
		}
	}
	for _, prefix := range ourPrefixes {
		if strings.Contains(haystack, prefix) {
			return true
		}
	}
	return false
}
