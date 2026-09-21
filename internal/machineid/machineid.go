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
