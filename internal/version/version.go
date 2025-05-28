package version

import "fmt"

const VersionString = "0.0.1-dev"

type Version struct {
	Major  int
	Minor  int
	Patch  int
	Suffix string
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d-%s", v.Major, v.Minor, v.Patch, v.Suffix)
}

func VersionCompatible(this, incoming *Version) bool {
	if this.Major != incoming.Major || this.Minor != incoming.Minor {
		return false
	}
	return true
}

func VersionFromString(version string) (*Version, error) {
	var major, minor, patch int
	var suffix string
	n, err := fmt.Sscanf(version, "%d.%d.%d-%s", &major, &minor, &patch, &suffix)
	if err != nil || n < 3 {
		return nil, fmt.Errorf("invalid version format: %s", version)
	}
	return &Version{Major: major, Minor: minor, Patch: patch, Suffix: suffix}, nil
}

func CurrentVersion() *Version {
	v, err := VersionFromString(VersionString)
	if err != nil {
		panic(fmt.Sprintf("invalid version string: %s", VersionString))
	}
	return v
}
