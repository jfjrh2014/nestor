package platform

import (
	"testing"
)

func TestDetectOS(t *testing.T) {
	os := detectOS()
	switch os {
	case OSMacOS, OSLinux, OSWSL:
		// valid
	default:
		t.Errorf("detectOS returned invalid value: %q", os)
	}
}

func TestDetectPackageManager(t *testing.T) {
	// on a CI box we should always find one of the supported managers
	pm, err := detectPackageManager(detectOS())
	if err != nil {
		t.Skipf("no package manager on this host: %v", err)
	}
	switch pm {
	case PMBrew, PMAPT, PMDNF, PMPacman, PMSnap:
		// valid
	default:
		t.Errorf("unexpected package manager: %q", pm)
	}
}

func TestCommandExists(t *testing.T) {
	// sh is everywhere
	if !commandExists("sh") {
		t.Error("expected sh to exist on PATH")
	}
	if commandExists("definitely-not-a-real-command-xyz123") {
		t.Error("expected non-existent command to return false")
	}
}

func TestDetect(t *testing.T) {
	info, err := Detect()
	if err != nil {
		t.Skipf("platform not fully detectable here: %v", err)
	}
	if info.OS == "" {
		t.Error("OS should not be empty")
	}
	if info.Arch == "" {
		t.Error("Arch should not be empty")
	}
	if info.PackageManager == "" {
		t.Error("PackageManager should not be empty")
	}
	t.Logf("detected: %+v", info)
}
