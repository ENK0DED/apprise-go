// Command schema_gen refreshes existing provider metadata from the pinned Python reference.
// It never registers a provider: delivery support must exist before metadata can be refreshed.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/unraid/apprise-go/internal/version"
)

type replacement struct {
	start, end int
	text       string
}

func literal(value any, depth int) string {
	pad, inner := strings.Repeat("\t", depth), strings.Repeat("\t", depth+1)
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var out strings.Builder
		out.WriteString("map[string]any{\n")
		for _, k := range keys {
			fmt.Fprintf(&out, "%s%s: %s,\n", inner, strconv.Quote(k), literal(v[k], depth+1))
		}
		out.WriteString(pad + "}")
		return out.String()
	case []any:
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = literal(item, depth+1)
		}
		return "[]any{" + strings.Join(parts, ", ") + "}"
	case json.Number:
		if integer, err := v.Int64(); err == nil {
			if integer < -2147483648 || integer > 2147483647 {
				return "int64(" + v.String() + ")"
			}
		}
		return v.String()
	case string:
		return strconv.Quote(v)
	case nil:
		return "nil"
	default:
		data, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		return string(data)
	}
}

func run() error {
	python := filepath.Join(".venv", "bin", "python")
	cmd := exec.Command(python, "-c", "import apprise; print(apprise.__version__)")
	installed, err := cmd.Output()
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(installed)) != version.UpstreamVersion {
		return fmt.Errorf("python reference does not match upstream pin %s", version.UpstreamVersion)
	}
	cmd = exec.Command(python, filepath.Join("internal", "testutil", "scripts", "schema_metadata.py"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("read upstream metadata: %w: %s", err, stderr.String())
	}
	metadata := map[string]any{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err = decoder.Decode(&metadata); err != nil {
		return err
	}
	files, err := filepath.Glob("internal/notify/*.go")
	if err != nil {
		return err
	}
	updated := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		set := token.NewFileSet()
		file, err := parser.ParseFile(set, path, source, 0)
		if err != nil {
			return err
		}
		var changes []replacement
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, ok := call.Fun.(*ast.Ident)
			if !ok || name.Name != "RegisterSchemaEntryOrdered" || len(call.Args) != 2 {
				return true
			}
			entry, ok := call.Args[1].(*ast.CompositeLit)
			if !ok {
				return true
			}
			var details *ast.KeyValueExpr
			var schemas []string
			for _, elt := range entry.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := kv.Key.(*ast.BasicLit)
				if !ok {
					continue
				}
				field, _ := strconv.Unquote(key.Value)
				if field == "details" {
					details = kv
				}
				if field != "protocols" && field != "secure_protocols" {
					continue
				}
				values, ok := kv.Value.(*ast.CompositeLit)
				if !ok {
					continue
				}
				for _, item := range values.Elts {
					if value, ok := item.(*ast.BasicLit); ok {
						schema, err := strconv.Unquote(value.Value)
						if err == nil {
							schemas = append(schemas, schema)
						}
					}
				}
			}
			if details == nil {
				return true
			}
			for _, schema := range schemas {
				if fresh, ok := metadata[schema]; ok {
					changes = append(changes, replacement{set.Position(details.Value.Pos()).Offset, set.Position(details.Value.End()).Offset, literal(fresh, 2)})
					break
				}
			}
			return false
		})
		if len(changes) == 0 {
			continue
		}
		sort.Slice(changes, func(i, j int) bool { return changes[i].start > changes[j].start })
		for _, change := range changes {
			source = append(append(append([]byte{}, source[:change.start]...), []byte(change.text)...), source[change.end:]...)
		}
		formatted, err := format.Source(source)
		if err != nil {
			return fmt.Errorf("format %s: %w", path, err)
		}
		original, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(original, formatted) {
			if err := os.WriteFile(path, formatted, 0o644); err != nil {
				return err
			}
			updated++
		}
	}
	fmt.Printf("Refreshed metadata in %d existing provider files for Apprise %s\n", updated, version.UpstreamVersion)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
