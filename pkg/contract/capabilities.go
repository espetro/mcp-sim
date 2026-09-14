package contract

import "strings"

// Capability is a single bit in a CapabilitySet.
type Capability uint32

// CapabilitySet is a bitmask of the operations a Platform supports. Pre-1.0
// contract: every Platform must implement Capabilities() and keep it in sync
// with what its methods actually do.
type CapabilitySet uint32

const (
	CapList       Capability = 1 << iota // List
	CapStart                             // Start
	CapStop                              // Stop
	CapState                             // State
	CapAwaitReady                        // AwaitReady
	CapWipe                              // Wipe
	CapOpenURL                           // OpenURL
	CapInstall                           // InstallApp (requires AppInstaller)
	CapLaunch                            // LaunchApp (requires AppLauncher)
	CapOptimize                          // Optimize/Restore/OptimizeStatus (requires Optimizer)
	CapMeasure                           // Measure (requires Optimizer)

	// Reserved for future capabilities (do not reorder existing bits):
	//   CapStream   Capability = 1 << 9
	//   CapSnapshot Capability = 1 << 10
)

// CapAll is the set of base lifecycle capabilities every in-tree platform has.
const CapAll CapabilitySet = CapabilitySet(CapList | CapStart | CapStop | CapState | CapAwaitReady | CapWipe | CapOpenURL)

// Has reports whether the set contains c.
func (s CapabilitySet) Has(c Capability) bool { return s&CapabilitySet(c) != 0 }

// Enable returns a new set with c enabled (value receiver; immutable).
func (s CapabilitySet) Enable(c Capability) CapabilitySet { return s | CapabilitySet(c) }

// capNames maps capabilities to display names for String.
var capNames = []struct {
	c Capability
	n string
}{
	{CapList, "list"},
	{CapStart, "start"},
	{CapStop, "stop"},
	{CapState, "state"},
	{CapAwaitReady, "await_ready"},
	{CapWipe, "wipe"},
	{CapOpenURL, "open_url"},
	{CapInstall, "install_app"},
	{CapLaunch, "launch_app"},
	{CapOptimize, "optimize"},
	{CapMeasure, "measure"},
}

// String returns a human-readable listing, e.g. "list|start|optimize".
func (s CapabilitySet) String() string {
	if s == 0 {
		return "none"
	}
	var parts []string
	for _, cn := range capNames {
		if s.Has(cn.c) {
			parts = append(parts, cn.n)
		}
	}
	return strings.Join(parts, "|")
}

// CapabilitiesFor derives the capability set for a Platform: base lifecycle
// caps plus CapOptimize/CapMeasure when p also implements Optimizer, and
// CapInstall/CapLaunch when p implements AppInstaller/AppLauncher.
func CapabilitiesFor(p Platform) CapabilitySet {
	s := CapAll
	if _, ok := p.(Optimizer); ok {
		s |= CapabilitySet(CapOptimize) | CapabilitySet(CapMeasure)
	}
	if _, ok := p.(AppInstaller); ok {
		s |= CapabilitySet(CapInstall)
	}
	if _, ok := p.(AppLauncher); ok {
		s |= CapabilitySet(CapLaunch)
	}
	return s
}

// As is the gocloud-style escape hatch: it type-asserts p to an arbitrary
// concrete type or extra interface (e.g. contract.As[Optimizer](p)) so callers
// can reach platform-specific behavior without widening Platform.
func As[T any](p Platform) (T, bool) {
	v, ok := p.(T)
	return v, ok
}
