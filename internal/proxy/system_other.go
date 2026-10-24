//go:build !windows

package proxy

// readSystem: elsewhere the desktop's proxy reaches apps as *_PROXY
// variables, which Func reads before it gets here.
func readSystem() System { return System{} }
