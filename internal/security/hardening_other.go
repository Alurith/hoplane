//go:build !linux

package security

func Harden() error { return nil }
