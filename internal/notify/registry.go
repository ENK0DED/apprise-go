package notify

import (
	"sort"
	"strings"
)

var supportedSchemas = map[string]struct{}{
	"json":  {},
	"jsons": {},
	"form":  {},
	"forms": {},
	"xml":   {},
	"xmls":  {},
}

func SupportedSchemas() []string {
	schemas := make([]string, 0, len(supportedSchemas))
	for schema := range supportedSchemas {
		schemas = append(schemas, schema)
	}
	sort.Strings(schemas)
	return schemas
}

func SupportsSchema(schema string) bool {
	_, ok := supportedSchemas[strings.ToLower(schema)]
	return ok
}
