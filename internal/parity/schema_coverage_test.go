package parity

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unraid/apprise-go/internal/notify"
	"github.com/unraid/apprise-go/internal/testutil"
)

func TestSchemaCoverage(t *testing.T) {
	appriseRoot := testutil.AppriseSourceRoot(t)
	script := filepath.Join(testutil.RepoRoot(t), "internal", "testutil", "scripts", "list_schemas.py")

	stdout, stderr, err := testutil.RunPythonScript(t, script, appriseRoot)
	if err != nil {
		t.Fatalf("list schemas failed: %v (stderr: %s)", err, strings.TrimSpace(stderr))
	}

	var pythonSchemas []string
	if err := json.Unmarshal([]byte(stdout), &pythonSchemas); err != nil {
		t.Fatalf("parse schemas: %v (output: %s)", err, strings.TrimSpace(stdout))
	}

	pythonSet := map[string]struct{}{}
	for _, schema := range pythonSchemas {
		pythonSet[strings.ToLower(schema)] = struct{}{}
	}

	goSchemas := notify.SupportedSchemas()
	goSet := map[string]struct{}{}
	for _, schema := range goSchemas {
		goSet[strings.ToLower(schema)] = struct{}{}
	}

	missing := []string{}
	for schema := range pythonSet {
		if _, ok := goSet[schema]; !ok {
			missing = append(missing, schema)
		}
	}

	if len(missing) > 0 {
		t.Fatalf("missing schemas in go: %s", strings.Join(missing, ", "))
	}
}
