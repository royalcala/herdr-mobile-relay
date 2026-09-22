// Package machineid holds the shared machine-identity constants and helpers so
// both the wire protocol and the coordinator can agree on them without importing
// each other.
package machineid

// Local names the session the relay talks to directly. An empty machine ID means
// the same.
const Local = "local"

// IsRemote reports whether a machine ID names a saved SSH machine rather than the
// local session.
func IsRemote(machineID string) bool {
	return machineID != "" && machineID != Local
}

// ScopedPaneID namespaces a per-server pane ID by its machine so a frame the
// phone keys by pane_id resolves to that machine's agent. Two saved machines can
// both host `w1:p1`, so the phone stores a remote agent's frames under
// `<relay>::<machine>::<raw>` but rebuilds a frame's key as
// `<relay>::<frame.pane_id>`. A remote frame therefore has to carry
// `<machine>::<raw>`; the local machine keeps the bare pane ID.
func ScopedPaneID(machineID, paneID string) string {
	if paneID == "" || !IsRemote(machineID) {
		return paneID
	}
	return machineID + "::" + paneID
}
