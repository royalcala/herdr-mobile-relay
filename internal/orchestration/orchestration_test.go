package orchestration

import (
	"path/filepath"
	"testing"
)

// testdata/orchestration.json is a verbatim copy of the registry on the machine.
func TestLoadReadsTheRealRegistry(t *testing.T) {
	registry, err := Load(filepath.Join("testdata", "orchestration.json"))
	if err != nil {
		t.Fatal(err)
	}
	if registry.Defaults.Manager != "manager-v3" {
		t.Fatalf("manager = %q, want manager-v3", registry.Defaults.Manager)
	}
	if len(registry.Defaults.Panels) != 5 {
		t.Fatalf("panels = %v, want the five the registry names", registry.Defaults.Panels)
	}
	entry, ok := registry.Sessions["default"]
	if !ok || entry.Label != "1us · plataforma" {
		t.Fatalf("default session = %#v, want the labelled entry", entry)
	}
	if registry.Agents["manager-v3"] != "manager" {
		t.Fatalf("agents = %#v, want manager-v3 as manager", registry.Agents)
	}
}

func TestResolveAppliesTheDefaultsToAnUnregisteredSession(t *testing.T) {
	registry, err := Load(filepath.Join("testdata", "orchestration.json"))
	if err != nil {
		t.Fatal(err)
	}
	// The golden rule: a session that was just born is already described.
	fresh := registry.Resolve("prueba")
	if fresh.Registered {
		t.Fatal("a session without an entry must not claim to be registered")
	}
	if fresh.Label != "prueba" {
		t.Fatalf("label = %q, want the session's own name when nothing names it", fresh.Label)
	}
	if fresh.Manager != "manager-v3" {
		t.Fatalf("manager = %q, want the default", fresh.Manager)
	}
	if len(fresh.Scopes) != 1 || fresh.Scopes[0] != "local" {
		t.Fatalf("scopes = %v, want the default scope", fresh.Scopes)
	}
	if len(fresh.Panels) != 5 {
		t.Fatalf("panels = %v, want the default panels", fresh.Panels)
	}

	registered := registry.Resolve("default")
	if !registered.Registered || registered.Label != "1us · plataforma" {
		t.Fatalf("default = %#v, want its own label", registered)
	}
	if len(registered.Scopes) != 3 {
		t.Fatalf("scopes = %v, want the three the session declares", registered.Scopes)
	}
}

// A registry that cannot be read falls back to the embedded defaults, loudly:
// the caller gets the error and a usable registry.
func TestLoadFallsBackToEmbeddedDefaults(t *testing.T) {
	registry, err := Load(filepath.Join(t.TempDir(), "orchestration.json"))
	if err == nil {
		t.Fatal("a missing registry must be reported so the caller can say so")
	}
	if registry.Defaults.Manager != Embedded().Defaults.Manager {
		t.Fatalf("defaults = %#v, want the embedded ones", registry.Defaults)
	}
	if resolved := registry.Resolve("prueba"); resolved.Label != "prueba" || len(resolved.Panels) != 5 {
		t.Fatalf("resolve without a registry = %#v, want the embedded defaults", resolved)
	}
}
