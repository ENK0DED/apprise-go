package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// RepoRoot walks upward from the current working directory to find go.mod.
func RepoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("go.mod not found; repo root unknown")
		}
		dir = parent
	}
}

func VenvBinDir(t *testing.T) string {
	t.Helper()

	root := RepoRoot(t)
	venvBin := filepath.Join(root, ".venv", "bin")
	if _, err := os.Stat(venvBin); err != nil {
		t.Skipf("venv bin dir not found: %s", venvBin)
	}

	return venvBin
}
