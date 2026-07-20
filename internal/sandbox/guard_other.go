//go:build !darwin

package sandbox

// Wrap runs the command unchanged.
//
// Linux enforcement is intended to use Landlock, which is in-process and needs
// no setuid helper — a better fit for a Go daemon than bubblewrap. It is not
// implemented yet, so Applied stays false and callers must not mistake this for
// enforcement.
func (g Guard) Wrap(name string, args []string) (Wrapped, error) {
	return passthrough(name, args), nil
}
