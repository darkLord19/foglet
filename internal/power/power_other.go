//go:build !darwin

package power

func (ih *Inhibitor) startPlatform() {
	// no-op on non-darwin platforms
}

func (ih *Inhibitor) stopPlatform() {
	// no-op on non-darwin platforms
}
