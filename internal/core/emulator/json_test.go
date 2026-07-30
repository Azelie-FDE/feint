package emulator

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// Validating what we answer catches a field we made up. Nothing caught a field
// the client sent and we silently dropped, and that is the same defect seen from
// the other side: CreateVms read a VmCount where Outscale sends MinVmsCount and
// MaxVmsCount, so every request created one machine whatever it asked for, and
// json.Unmarshal was perfectly happy. So was every test.

type filters struct {
	Names []string `json:"names"`
	Sizes []int    `json:"sizes"`
}

type nested struct {
	Key string `json:"key"`
}

type target struct {
	Name    string   `json:"name"`
	Count   *int     `json:"count"`
	Filters filters  `json:"filters"`
	Tags    []nested `json:"tags"`
	Bare    string
	Skipped string `json:"-"`
	hidden  string //nolint:unused // present to prove unexported fields are ignored
}

func unread(t *testing.T, body string) []string {
	t.Helper()
	var v target
	r := httptest.NewRequest("POST", "/x", strings.NewReader(body))
	rep := &requestReport{}
	r = r.WithContext(contextWithReport(r, rep))
	if err := DecodeJSON(r, &v); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return rep.fields()
}

func TestDecodeReportsAFieldNobodyReads(t *testing.T) {
	got := unread(t, `{"name": "a", "vmCount": 3}`)
	if len(got) != 1 || got[0] != "vmCount" {
		t.Fatalf("got %v, want [vmCount]", got)
	}
}

func TestDecodeSaysNothingAboutFieldsItReads(t *testing.T) {
	got := unread(t, `{"name": "a", "count": 2, "filters": {"names": ["x"]}, "tags": [{"key": "k"}]}`)
	if len(got) != 0 {
		t.Fatalf("got %v, want nothing", got)
	}
}

func TestDecodeLooksInsideNestedObjects(t *testing.T) {
	// Outscale puts most arguments under Filters, so a misspelt filter is a
	// request that silently matches everything. Top-level checking alone would
	// miss the ones that matter most.
	got := unread(t, `{"filters": {"names": ["x"], "vmIds": ["i-1"]}}`)
	if len(got) != 1 || got[0] != "filters.vmIds" {
		t.Fatalf("got %v, want [filters.vmIds]", got)
	}
}

func TestDecodeLooksInsideArraysOfObjects(t *testing.T) {
	got := unread(t, `{"tags": [{"key": "k"}, {"value": "v"}]}`)
	if len(got) != 1 || got[0] != "tags[1].value" {
		t.Fatalf("got %v, want [tags[1].value]", got)
	}
}

func TestDecodeFollowsEncodingJSONOnCase(t *testing.T) {
	// encoding/json falls back to a case-insensitive match, so a body using
	// another case IS read. Reporting it would be a false positive, and a report
	// with those in it stops being read at all.
	if got := unread(t, `{"Name": "a"}`); len(got) != 0 {
		t.Fatalf("got %v: the field was read, so it must not be reported", got)
	}
	// A field with no tag is matched on its Go name, same rule.
	if got := unread(t, `{"bare": "b"}`); len(got) != 0 {
		t.Fatalf("got %v: Bare is declared", got)
	}
}

func TestDecodeReportsAFieldTheTypeRefuses(t *testing.T) {
	// json:"-" means encoding/json will never read it, so a client sending it
	// is sending something nobody honours.
	got := unread(t, `{"Skipped": "x"}`)
	if len(got) != 1 || got[0] != "Skipped" {
		t.Fatalf("got %v, want [Skipped]", got)
	}
}

func TestDecodeIsSilentWithoutAnObserver(t *testing.T) {
	// Nothing watching means no second decode and no bookkeeping: the report is
	// a development-time facility, not a tax on every request.
	var v target
	r := httptest.NewRequest("POST", "/x", strings.NewReader(`{"nope": 1}`))
	if err := DecodeJSON(r, &v); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func TestDecodeSaysNothingAboutABodyThatIsNotAnObject(t *testing.T) {
	var v []int
	r := httptest.NewRequest("POST", "/x", strings.NewReader(`[1,2,3]`))
	rep := &requestReport{}
	r = r.WithContext(contextWithReport(r, rep))
	if err := DecodeJSON(r, &v); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := rep.fields(); len(got) != 0 {
		t.Fatalf("got %v: an array body has no field names to check", got)
	}
}

func TestAnEmptyBodyStillDecodes(t *testing.T) {
	// Several provider APIs accept an empty payload on create and their SDKs
	// rely on it.
	var v target
	r := httptest.NewRequest("POST", "/x", strings.NewReader(""))
	if err := DecodeJSON(r, &v); err != nil {
		t.Fatalf("an empty body must decode to the zero value, got %v", err)
	}
}
