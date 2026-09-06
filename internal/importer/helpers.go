package importer

import (
	"os"
	"os/exec"

	"github.com/jfjrh2014/nestor/internal/pathutil"
)

// execLookPath is a wrapper around exec.LookPath so it can be mocked in tests.
var execLookPath = exec.LookPath

// osUserHomeDir is a wrapper around os.UserHomeDir for mocking in tests.
var osUserHomeDir = os.UserHomeDir

// runCommand runs a shell command and returns its stdout as a string.
// It is a var so tests can stub command execution (same seam as the
// cmdRunner in packages and cmdOutput in secrets).
var runCommand = func(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return string(out), err
}

// expandHome replaces a leading ~ or ~/... with the home directory.
// The "~user/..." form is deliberately NOT expanded; see pathutil.ExpandHome.
func expandHome(path string) string {
	home, err := osUserHomeDir()
	if err != nil {
		home = ""
	}
	return pathutil.ExpandHome(path, home)
}
