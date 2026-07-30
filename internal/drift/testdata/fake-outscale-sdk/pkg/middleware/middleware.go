// Plumbing, hand-written, with no client.gen.go. Most of the SDK's packages
// look like this and none of them describe API surface.

package middleware

type Chain struct{}

// Do looks like an operation and is not one: the package ships no generated
// client, so the scan never opens this file.
func (c *Chain) Do() error { return nil }
