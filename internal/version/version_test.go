package version

import (
	"strings"
	"testing"
)

func TestGet_FieldsPopulated(t *testing.T) {
	info := Get()
	if info.Name != "bookdb" {
		t.Fatalf("unexpected name %q", info.Name)
	}
	if info.Version == "" {
		t.Fatal("version must be non-empty")
	}
	if !strings.HasPrefix(info.Go, "go") {
		t.Fatalf("unexpected go runtime %q", info.Go)
	}
	if !strings.Contains(info.Platform, "/") {
		t.Fatalf("platform should be os/arch: %q", info.Platform)
	}
}
