package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestYAMLTemplateResolution(t *testing.T) {
	for _, tc := range []struct {
		name, defaults, override, env, want string
		fail                                bool
	}{{"override", "default", "caller", "environment", "caller", false}, {"default", "default", "", "environment", "default", false}, {"environment", "null", "", "environment", "environment", false}, {"missing", "null", "", "", "", true}, {"control", "null", "secret\nvalue", "", "", true}, {"encoding", "null", "a/b&c=1", "", "a%2Fb&c=1", false}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "apprise.yml")
			source := "version: 1\ntemplate:\n  TOKEN: " + tc.defaults + "\nurls:\n  - json://example.com/${token}\n"
			if err := os.WriteFile(file, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("APPRISE_TEMPLATE_TOKEN", tc.env)
			opts := defaultCliOptions()
			opts.templateValues = map[string]string{"token": tc.override}
			got := loadTaggedURLsForOptions(&opts, []string{file})
			if tc.fail {
				if opts.templateErr == nil {
					t.Fatal("expected template error")
				}
				if strings.Contains(opts.templateErr.Error(), "secret") {
					t.Fatal("error disclosed value")
				}
				return
			}
			if opts.templateErr != nil {
				t.Fatal(opts.templateErr)
			}
			if len(got) != 1 || got[0].URL != "json://example.com/"+tc.want {
				t.Fatalf("URLs %+v", got)
			}
		})
	}
}
func TestYAMLTemplateCannotInjectConfiguration(t *testing.T) {
	file := filepath.Join(t.TempDir(), "apprise.yml")
	if err := os.WriteFile(file, []byte("template: [token]\nurls:\n - json://example.com/${token}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	opts := defaultCliOptions()
	opts.templateValues = map[string]string{"token": "value\nurls: [json://unrelated]"}
	if got := loadTaggedURLsForOptions(&opts, []string{file}); len(got) != 0 || opts.templateErr == nil {
		t.Fatalf("URLs %+v err %v", got, opts.templateErr)
	}
}

func TestTemplateURLPositions(t *testing.T) {
	for _, tc := range []struct{ source, value, want string }{
		{"${token}://example.com/path", "json", "${token}://example.com/path"},
		{"json://example.com/${token}?q=${token}", "a/b&c=1", "json://example.com/a%2Fb&c=1?q=a%2Fb%26c%3D1"},
		{"json://${token}/path", "user:p@ss@example.com", "json://user:p%40ss@example.com/path"},
		{"json://${token}@example.com/path", "a@b/c", "json://a%40b%2Fc@example.com/path"},
		{"json://${token}@example.com/path", "user:p@ss", "json://user:p%40ss@example.com/path"},
	} {
		got, err := substituteTemplateText(tc.source, false, map[string]any{"token": nil}, func(string) (string, error) { return tc.value, nil })
		if err != nil || got != tc.want {
			t.Errorf("%q got%q want%q err%v", tc.source, got, tc.want, err)
		}
	}
}
func TestTemplateYAMLSettingNamesStayLiteral(t *testing.T) {
	file := filepath.Join(t.TempDir(), "apprise.yml")
	if err := os.WriteFile(file, []byte("template: {token: urls}\n'${token}': [json://example.com/injected]\nurls: [json://example.com/expected]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	opts := defaultCliOptions()
	got := loadTaggedURLsForOptions(&opts, []string{file})
	if opts.templateErr != nil || len(got) != 1 || got[0].URL != "json://example.com/expected" {
		t.Fatalf("URLs%+v err%v", got, opts.templateErr)
	}
}

func TestTemplateValueAndResolvedURLLimits(t *testing.T) {
	for _, tc := range []struct {
		value any
		want  string
	}{{1, "1"}, {1.0, "1.0"}, {1000000.0, "1000000.0"}, {1e16, "1e+16"}, {true, "yes"}} {
		got, err := templateValue("token", tc.value)
		if err != nil || got != tc.want {
			t.Errorf("%v got%q want%q error%v", tc.value, got, tc.want, err)
		}
	}
	_, err := substituteTemplateText("json://example.com/"+strings.Repeat("${token}", 9), false, map[string]any{"token": nil}, func(string) (string, error) { return strings.Repeat("a", 1024), nil })
	if err == nil {
		t.Fatal("accepted resolved URL exceeding upstream's 8192-character limit")
	}
}
