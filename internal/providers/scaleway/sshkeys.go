package scaleway

import (
	"crypto/md5" //nolint:gosec // SSH fingerprints are MD5 by protocol, not a security choice here
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/stephrobert/feint/internal/core/emulator"
	"github.com/stephrobert/feint/internal/core/resource"
)

// SSH keys are what turn an emulated server into a machine you can log into.
//
// Scaleway attaches them to the PROJECT, not to the server: there is no
// key_name field on a create request, unlike EC2. Every key of the project is
// injected into every server the project boots. Emulating that faithfully means
// serving the IAM product, and it is why the machine driver can hand a working
// cloud-init to Incus.

// kindSSHKey is the store kind for IAM SSH keys.
const kindSSHKey = "iam/ssh-key"

type createSSHKeyRequest struct {
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
	ProjectID string `json:"project_id"`
}

type updateSSHKeyRequest struct {
	Name     *string `json:"name"`
	Disabled *bool   `json:"disabled"`
}

func (p *Pack) listSSHKeys(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project_id")
	all := p.env.Store.List(kindSSHKey, resource.Tenant{Provider: Name, Project: project})

	page := parsePage(r)
	start, end := page.slice(len(all))
	keys := make([]map[string]any, 0, end-start)
	for _, res := range all[start:end] {
		keys = append(keys, sshKeyView(res))
	}

	emulator.WriteJSON(w, http.StatusOK, map[string]any{
		"ssh_keys":    keys,
		"total_count": len(all),
	})
}

func (p *Pack) createSSHKey(w http.ResponseWriter, r *http.Request) {
	var req createSSHKeyRequest
	if err := emulator.DecodeJSON(r, &req); err != nil {
		writeInvalidArguments(w, ArgumentError{ArgumentName: "body", Reason: "format", HelpMessage: err.Error()})
		return
	}
	if strings.TrimSpace(req.PublicKey) == "" {
		writeInvalidArguments(w, ArgumentError{ArgumentName: "public_key", Reason: "required"})
		return
	}
	// The API rejects anything that is not an SSH public key, and so must the
	// emulator: a malformed key silently breaks every later login attempt.
	if !looksLikePublicKey(req.PublicKey) {
		writeInvalidArguments(w, ArgumentError{
			ArgumentName: "public_key",
			Reason:       "constraint",
			HelpMessage:  "not an OpenSSH public key",
		})
		return
	}

	// IAM keys belong to a project, and the organization above it is the
	// account, never the same identifier.
	project, organization := projectOf(req.ProjectID)
	now := p.env.Now()

	res := &resource.Resource{
		ID:      p.env.NewID(),
		Kind:    kindSSHKey,
		Tenant:  resource.Tenant{Provider: Name, Project: project},
		State:   "enabled",
		Created: now,
		Updated: now,
		Attrs: map[string]any{
			"name":            orDefault(req.Name, "key"),
			"public_key":      strings.TrimSpace(req.PublicKey),
			"fingerprint":     fingerprint(req.PublicKey),
			"project_id":      project,
			"organization_id": organization,
			"disabled":        false,
		},
	}
	p.env.Store.Put(res)

	emulator.WriteJSON(w, http.StatusCreated, sshKeyView(res))
}

func (p *Pack) getSSHKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	res, ok := p.env.Store.Get(Name, kindSSHKey, id)
	if !ok {
		writeNotFound(w, "ssh_key", id)
		return
	}
	emulator.WriteJSON(w, http.StatusOK, sshKeyView(res))
}

func (p *Pack) updateSSHKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	res, ok := p.env.Store.Get(Name, kindSSHKey, id)
	if !ok {
		writeNotFound(w, "ssh_key", id)
		return
	}

	var req updateSSHKeyRequest
	if err := emulator.DecodeJSON(r, &req); err != nil {
		writeInvalidArguments(w, ArgumentError{ArgumentName: "body", Reason: "format", HelpMessage: err.Error()})
		return
	}
	if req.Name != nil {
		res.Attrs["name"] = *req.Name
	}
	if req.Disabled != nil {
		res.Attrs["disabled"] = *req.Disabled
	}
	if !p.env.Store.Commit(res, p.env.Now()) {
		writeNotFound(w, "ssh key", res.ID)
		return
	}

	emulator.WriteJSON(w, http.StatusOK, sshKeyView(res))
}

func (p *Pack) deleteSSHKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !p.env.Store.Delete(Name, kindSSHKey, id) {
		writeNotFound(w, "ssh_key", id)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// authorizedKeys returns the enabled public keys of a project, which is what
// gets injected into every machine that project boots.
func (p *Pack) authorizedKeys(project string) []string {
	var keys []string
	for _, res := range p.env.Store.List(kindSSHKey, resource.Tenant{Provider: Name, Project: project}) {
		if disabled, _ := res.Attrs["disabled"].(bool); disabled {
			continue
		}
		if pub, _ := res.Attrs["public_key"].(string); pub != "" {
			keys = append(keys, pub)
		}
	}
	return keys
}

func sshKeyView(res *resource.Resource) map[string]any {
	out := make(map[string]any, len(res.Attrs)+3)
	for k, v := range res.Attrs {
		out[k] = v
	}
	out["id"] = res.ID
	out["created_at"] = res.Created.Format(time.RFC3339)
	out["updated_at"] = res.Updated.Format(time.RFC3339)
	return out
}

// looksLikePublicKey accepts the OpenSSH one-line format. It validates the
// shape, not the cryptography: the emulator has no reason to reject a
// well-formed key it cannot parse.
//
// "One-line" is enforced rather than assumed, and that is a security check, not
// tidiness. The stored value is rendered into a cloud-config document by
// text/template, which concatenates without escaping, so a key carrying a newline
// opens a top-level YAML key of the attacker's choosing: an audit obtained
// runcmd, bootcmd and write_files that way. strings.Fields below splits on
// newlines just like spaces, which is exactly why the check has to come first.
//
// The contamination is why this matters more than it looks: an authorized key
// applies to every machine the project starts afterwards, including one a
// terraform apply creates without ever supplying user data.
func looksLikePublicKey(key string) bool {
	if strings.ContainsAny(key, "\n\r\x00") {
		return false
	}
	fields := strings.Fields(strings.TrimSpace(key))
	if len(fields) < 2 {
		return false
	}
	switch fields[0] {
	case "ssh-rsa", "ssh-ed25519", "ssh-dss",
		"ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521",
		"sk-ssh-ed25519@openssh.com", "sk-ecdsa-sha2-nistp256@openssh.com":
	default:
		return false
	}
	_, err := base64.StdEncoding.DecodeString(fields[1])
	return err == nil
}

// fingerprint mirrors what the API returns: Scaleway exposes the MD5 form, the
// colon-separated one ssh-keygen printed by default for years.
func fingerprint(key string) string {
	fields := strings.Fields(strings.TrimSpace(key))
	if len(fields) < 2 {
		return ""
	}
	blob, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		return ""
	}

	sum := md5.Sum(blob) //nolint:gosec // the SSH fingerprint format is MD5, not a security decision
	hexed := hex.EncodeToString(sum[:])
	parts := make([]string, 0, len(hexed)/2)
	for i := 0; i+2 <= len(hexed); i += 2 {
		parts = append(parts, hexed[i:i+2])
	}
	return strings.Join(parts, ":")
}
