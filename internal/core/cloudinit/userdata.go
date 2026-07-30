package cloudinit

import "encoding/base64"

// Decode returns the cloud-init a client sent, whatever encoding its API
// declares for the field.
//
// Outscale documents UserData as "Base64-encoded and limited to 500 kibibytes";
// Exoscale documents user-data as "(base64 encoded)". Scaleway's is a raw body.
// Handing the encoded form straight to cloud-init, which is what both packs did,
// makes it read base64 as a cloud-config, find no header it understands, and
// drop it without a word — so no user script ever ran, on either provider, and
// nothing said so.
//
// Not strict about it: a client that sends plain text where the API asks for
// base64 gets its script honoured rather than a machine that boots without it.
// The emulator is not the place to enforce an encoding, and refusing would turn
// a client's mistake into a failure that looks like ours.
func Decode(userData string) string {
	if userData == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(userData)
	if err != nil {
		return userData
	}
	return string(decoded)
}

// MaxUserData is the size both APIs cap user data at, 500 kibibytes. A larger
// one is refused by them, and accepting it here would let a script through that
// the real API would reject.
const MaxUserData = 500 << 10
