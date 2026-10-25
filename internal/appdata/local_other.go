//go:build !windows

package appdata

func platformLocalData() (string, error) { return "", nil }
