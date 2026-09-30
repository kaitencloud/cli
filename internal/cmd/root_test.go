package cmd

import "testing"

func TestRootCommandRegistersExpectedCommands(t *testing.T) {
	root := NewRootCommand()

	want := map[string]bool{
		"version":            false,
		"config":             false,
		"doctor":             false,
		"components":         false,
		"customers":          false,
		"deployment-zones":   false,
		"entitlement-groups": false,
		"entitlements":       false,
		"feature-flags":      false,
		"instances":          false,
		"licenses":           false,
		"releases":           false,
		"service-accounts":   false,
	}

	for _, cmd := range root.Commands() {
		if _, ok := want[cmd.Name()]; ok {
			want[cmd.Name()] = true
		}
	}

	for name, found := range want {
		if !found {
			t.Fatalf("root command missing %q", name)
		}
	}
}
