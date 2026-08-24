//go:build linux

package security

import "golang.org/x/sys/unix"

// Harden disables core dumps and same-user ptrace access for Hoplane. This does
// not protect against root or already-compromised code running as the user, but
// reduces accidental disclosure of secrets held in process memory.
func Harden() error {
	limit := unix.Rlimit{Cur: 0, Max: 0}
	if err := unix.Setrlimit(unix.RLIMIT_CORE, &limit); err != nil {
		return err
	}
	return unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0)
}
