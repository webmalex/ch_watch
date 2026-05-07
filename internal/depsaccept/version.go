package depsaccept

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type semver struct {
	major int
	minor int
	patch int
}

func readVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(data))
	if _, err := parseSemver(version); err != nil {
		return "", err
	}
	return version, nil
}

func writeVersion(path string, version string) error {
	if _, err := parseSemver(version); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(version+"\n"), 0o644)
}

func bumpPatch(version string) (string, error) {
	parsed, err := parseSemver(version)
	if err != nil {
		return "", err
	}
	parsed.patch++
	return parsed.String(), nil
}

func parseSemver(version string) (semver, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(version, "v"))
	parts := strings.Split(trimmed, ".")
	if len(parts) != 3 {
		return semver{}, fmt.Errorf("version %q must have MAJOR.MINOR.PATCH format", version)
	}

	major, err := parseVersionPart(parts[0], "major", version)
	if err != nil {
		return semver{}, err
	}
	minor, err := parseVersionPart(parts[1], "minor", version)
	if err != nil {
		return semver{}, err
	}
	patch, err := parseVersionPart(parts[2], "patch", version)
	if err != nil {
		return semver{}, err
	}
	return semver{major: major, minor: minor, patch: patch}, nil
}

func parseVersionPart(part string, label string, original string) (int, error) {
	if part == "" {
		return 0, fmt.Errorf("version %q has empty %s part", original, label)
	}
	value, err := strconv.Atoi(part)
	if err != nil {
		return 0, fmt.Errorf("version %q has invalid %s part %q", original, label, part)
	}
	if value < 0 {
		return 0, fmt.Errorf("version %q has negative %s part", original, label)
	}
	return value, nil
}

func (v semver) String() string {
	return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
}
