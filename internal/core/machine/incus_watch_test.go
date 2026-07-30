package machine

import (
	"strings"
	"testing"
)

// The decoder is the filter between the daemon's firehose and the emulator's
// log: every line here is a real event captured from `incus monitor` on incus
// 7.2, so a format drift upstream fails a test instead of silencing the watch.
func TestDecodeIncusEvent(t *testing.T) {
	tests := []struct {
		name         string
		line         string
		want         bool
		wantKind     string
		wantLevel    string
		wantResource string
	}{
		{
			name: "our lifecycle event is kept",
			line: `{"location":"none","metadata":{"action":"instance-started","name":"feint-scw-1234",` +
				`"project":"default","source":"/1.0/instances/feint-scw-1234"},"type":"lifecycle"}`,
			want:         true,
			wantKind:     "lifecycle",
			wantResource: "feint-scw-1234",
		},
		{
			name: "someone else's lifecycle event is dropped",
			line: `{"metadata":{"action":"instance-started","name":"operator-vm"},"type":"lifecycle"}`,
			want: false,
		},
		{
			name: "a debug logging event is dropped even when it is ours",
			line: `{"metadata":{"context":{"instance":"feint-scw-1234"},"level":"debug",` +
				`"message":"Handling API request"},"type":"logging"}`,
			want: false,
		},
		{
			name: "a daemon error naming our instance in its context is kept",
			line: `{"metadata":{"context":{"device":"eth0","instance":"feint-scw-1234"},"level":"error",` +
				`"message":"Failed to start device"},"type":"logging"}`,
			want:         true,
			wantKind:     "logging",
			wantLevel:    "error",
			wantResource: "feint-scw-1234",
		},
		{
			name: "a warning naming our network only in its text is kept",
			line: `{"metadata":{"context":{"err":"close udp 10.76.154.1:41050->10.76.154.1:67"},"level":"warning",` +
				`"message":"Failed to bind DHCP on fnt-44650e94dae"},"type":"logging"}`,
			want:      true,
			wantKind:  "logging",
			wantLevel: "warning",
		},
		{
			name: "garbage is dropped, not decoded into a ghost event",
			line: `not json at all`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, ok := decodeIncusEvent([]byte(tt.line))
			if ok != tt.want {
				t.Fatalf("decodeIncusEvent kept=%v, want %v (event %+v)", ok, tt.want, event)
			}
			if !ok {
				return
			}
			if event.Kind != tt.wantKind {
				t.Errorf("Kind = %q, want %q", event.Kind, tt.wantKind)
			}
			if tt.wantLevel != "" && event.Level != tt.wantLevel {
				t.Errorf("Level = %q, want %q", event.Level, tt.wantLevel)
			}
			if tt.wantResource != "" && event.Resource != tt.wantResource {
				t.Errorf("Resource = %q, want %q", event.Resource, tt.wantResource)
			}
		})
	}
}

// The lifecycle races are the daemon errors that are expected: verbatim
// captures from incus 7.2 while a list raced a `stop --force` and a launch.
// They must come out as warnings that explain what the operator will see, not
// as echoed alarms — and the same wording with a different cause must keep its
// alarm, because that one could be a pool actually broken.
func TestDecodeIncusEventExplainsLifecycleRaces(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		wantLevel   string
		wantExplain string
	}{
		{
			name: "a netlink read of a dead init explains the transient ERROR row",
			line: `{"location":"none","metadata":{"context":{"err":"open /proc/508019/ns/net: no such file or directory",` +
				`"instance":"feint-scw-repro","instanceType":"container","pid":"508019","project":"default"},` +
				`"level":"error","message":"Failed to retrieve network information via netlink"},"type":"logging"}`,
			wantLevel:   "warning",
			wantExplain: "ERROR",
		},
		{
			name: "a pool read before the volume exists explains the create/delete race",
			line: `{"metadata":{"context":{"err":"Failed getting instance pool: Instance storage pool not found",` +
				`"instance":"feint-scw-repro","instanceType":"container","project":"default"},` +
				`"level":"error","message":"Error loading storage pool"},"type":"logging"}`,
			wantLevel:   "warning",
			wantExplain: "storage volume",
		},
		{
			name: "the same pool message with another cause keeps its alarm",
			line: `{"metadata":{"context":{"err":"Required tool 'zpool' is missing",` +
				`"instance":"feint-scw-repro","instanceType":"container","project":"default"},` +
				`"level":"error","message":"Error loading storage pool"},"type":"logging"}`,
			wantLevel:   "error",
			wantExplain: "Error loading storage pool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, ok := decodeIncusEvent([]byte(tt.line))
			if !ok {
				t.Fatal("the event was dropped; it must be reported")
			}
			if event.Level != tt.wantLevel {
				t.Errorf("Level = %q, want %q", event.Level, tt.wantLevel)
			}
			if event.Resource != "feint-scw-repro" {
				t.Errorf("Resource = %q, want %q", event.Resource, "feint-scw-repro")
			}
			if !strings.Contains(event.Message, tt.wantExplain) {
				t.Errorf("Message = %q, want it to contain %q", event.Message, tt.wantExplain)
			}
		})
	}
}
