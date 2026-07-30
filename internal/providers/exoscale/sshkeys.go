package exoscale

import (
	"crypto/md5" //nolint:gosec // the fingerprint format is MD5 by definition, not a security choice
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/stephrobert/feint/internal/core/emulator"
	"github.com/stephrobert/feint/internal/core/resource"
)

// SSH keys, which are on the critical path of a create and were not obvious.
//
// `exo compute instance create` registers a key of its own before it posts the
// instance — it generates a pair, calls POST /ssh-key, and fails the whole
// command on a 404. Nothing in the API description says the create depends on
// it; it was found by putting a logging proxy between the CLI and the emulator
// and reading what actually went past.
//
// That is the third time this pack shape has appeared: Scaleway resolves its
// catalogue before creating, Outscale reads its types and images, Exoscale
// registers a key. The inventory a client walks before it commits is never the
// route it says it is calling.

const kindSSHKey = "ssh-key"

type registerSSHKeyRequest struct {
	Name      string `json:"name"`
	PublicKey string `json:"public-key"`
}

func (p *Pack) registerSSHKey(w http.ResponseWriter, r *http.Request) {
	var req registerSSHKeyRequest
	if err := emulator.DecodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	switch {
	case req.Name == "":
		writeError(w, http.StatusBadRequest, "name is required")
		return
	case req.PublicKey == "":
		writeError(w, http.StatusBadRequest, "public-key is required")
		return
	}
	if _, exists := p.env.Store.Get(Name, kindSSHKey, req.Name); exists {
		writeError(w, http.StatusConflict, "an SSH key named "+req.Name+" already exists")
		return
	}

	now := p.env.Now()
	res := &resource.Resource{
		// The name is the identifier: every route addresses a key by
		// /ssh-key/{name}, so a generated id would be a second identity nobody
		// can reach.
		ID:      req.Name,
		Kind:    kindSSHKey,
		Tenant:  resource.Tenant{Provider: Name},
		State:   "registered",
		Created: now,
		Updated: now,
		Attrs: map[string]any{
			"name":        req.Name,
			"fingerprint": fingerprintOf(req.PublicKey),
		},
	}
	p.env.Store.Put(res)

	emulator.WriteJSON(w, http.StatusOK, p.operationFor(res.ID, "register-ssh-key"))
}

func (p *Pack) listSSHKeys(w http.ResponseWriter, _ *http.Request) {
	list := p.env.Store.List(kindSSHKey, resource.Tenant{Provider: Name})
	keys := make([]map[string]any, 0, len(list))
	for _, res := range list {
		keys = append(keys, sshKeyView(res))
	}
	emulator.WriteJSON(w, http.StatusOK, map[string]any{"ssh-keys": keys})
}

func (p *Pack) getSSHKey(w http.ResponseWriter, r *http.Request) {
	res, ok := p.env.Store.Get(Name, kindSSHKey, r.PathValue("name"))
	if !ok {
		writeError(w, http.StatusNotFound, "no SSH key named "+r.PathValue("name"))
		return
	}
	emulator.WriteJSON(w, http.StatusOK, sshKeyView(res))
}

func (p *Pack) deleteSSHKey(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, ok := p.env.Store.Get(Name, kindSSHKey, name); !ok {
		writeError(w, http.StatusNotFound, "no SSH key named "+name)
		return
	}
	p.env.Store.Delete(Name, kindSSHKey, name)
	emulator.WriteJSON(w, http.StatusOK, p.operationFor(name, "delete-ssh-key"))
}

// sshKeyView is the whole of what the API publishes: a name and a fingerprint.
// The public key is deliberately absent, because their schema does not declare
// it — ssh-key carries two properties and nothing else, and the contract check
// fails a response that adds one.
func sshKeyView(res *resource.Resource) map[string]any {
	return map[string]any{
		"name":        res.Attrs["name"],
		"fingerprint": res.Attrs["fingerprint"],
	}
}

// fingerprintOf computes the colon-separated MD5 digest of the key blob, which
// is the format every SSH tool prints and every client expects to be able to
// compare against `ssh-keygen -l -E md5`.
//
// MD5 is the format, not a security decision: it is what the fingerprint of a
// registered key is, on Exoscale as on AWS. Nothing here authenticates anything.
//
// A key whose blob does not decode gets a digest of the whole string rather than
// an error. The emulator is not a key validator, and refusing at this point
// would fail a create over a field no downstream behaviour depends on.
func fingerprintOf(publicKey string) string {
	blob := publicKey
	if fields := strings.Fields(publicKey); len(fields) >= 2 {
		if raw, err := base64.StdEncoding.DecodeString(fields[1]); err == nil {
			sum := md5.Sum(raw) //nolint:gosec // see above: this is a display format
			return colonHex(sum[:])
		}
	}
	sum := md5.Sum([]byte(blob)) //nolint:gosec // see above
	return colonHex(sum[:])
}

func colonHex(sum []byte) string {
	out := make([]string, 0, len(sum))
	for _, b := range sum {
		out = append(out, hex.EncodeToString([]byte{b}))
	}
	return strings.Join(out, ":")
}
