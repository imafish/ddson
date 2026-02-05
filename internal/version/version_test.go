package version

import "testing"

func TestVersionString(t *testing.T) {
	v := Version{Major: 1, Minor: 2, Patch: 3, Suffix: "dev"}
	if got := v.String(); got != "1.2.3-dev" {
		t.Fatalf("unexpected version string: %s", got)
	}
}

func TestVersionCompatible(t *testing.T) {
	base := &Version{Major: 1, Minor: 2}
	if !VersionCompatible(base, &Version{Major: 1, Minor: 2, Patch: 9}) {
		t.Fatalf("expected compatible versions")
	}
	if VersionCompatible(base, &Version{Major: 2, Minor: 2}) {
		t.Fatalf("expected incompatible major")
	}
	if VersionCompatible(base, &Version{Major: 1, Minor: 3}) {
		t.Fatalf("expected incompatible minor")
	}
}

func TestVersionFromString(t *testing.T) {
	v, err := VersionFromString("1.2.3-dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Major != 1 || v.Minor != 2 || v.Patch != 3 || v.Suffix != "dev" {
		t.Fatalf("unexpected version: %+v", v)
	}

	if _, err := VersionFromString("bad"); err == nil {
		t.Fatalf("expected error for invalid format")
	}
}

func TestCurrentVersion(t *testing.T) {
	v := CurrentVersion()
	if v == nil {
		t.Fatalf("expected non-nil version")
	}
}
