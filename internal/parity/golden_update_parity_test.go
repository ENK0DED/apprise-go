package parity

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/unraid/apprise-go/internal/testutil"
)

func TestGoldenUpdateCheck(t *testing.T) {
	appriseRoot := testutil.AppriseSourceRoot(t)
	t.Setenv("PYTHONPATH", appriseRoot)
	t.Setenv("APPRISE_SOURCE_ROOT", appriseRoot)

	script := filepath.Join(testutil.RepoRoot(t), "internal", "testutil", "scripts", "update_golden.py")
	_, stderr, err := testutil.RunPythonScript(t, script, "--check")
	if err != nil {
		t.Fatalf("update_golden --check failed: %v (stderr: %s)", err, strings.TrimSpace(stderr))
	}
	t.Run("matrix-fresh-crypto", func(t *testing.T) {
		t.Setenv("APPRISE_CAPTURE_CACHE", "0")
		_, stderr, err := testutil.RunPythonScript(t, script, "--check", "matrix")
		if err != nil {
			t.Fatalf("fresh Matrix golden check failed: %v (stderr: %s)", err, strings.TrimSpace(stderr))
		}
	})
}
