package inspect

import (
	"strings"
)

var skipMenus = map[string]bool{
	"tool":                   true,
	"log":                    true,
	"file":                   true,
	"console":                true,
	"system/resource":        true,
	"system/history":         true,
	"system/package":         true,
	"system/routerboard":     true,
	"system/health":          true,
	"system/license":         true,
	"ip/arp":                 true,
	"ip/accounting":          true,
	"ip/firewall/connection": true,
	"routing/route":          true,
}

// Recurse into child sub-menus (ignore CLI command keys)
var recurseCmdKeys = map[string]bool{
	"add":     true,
	"set":     true,
	"get":     true,
	"print":   true,
	"remove":  true,
	"enable":  true,
	"disable": true,
	"export":  true,
	"find":    true,
	"edit":    true,
	"comment": true,
	"move":    true,
	"reset":   true,
}

type walker struct {
	specs  []*ResourceSpec
	wanted map[string]bool
}

// walkInspectTree traverses the nested inspect.json tree to extract configurable menus.
func (w *walker) walkInspectTree(node map[string]any, path []string) {
	slashPath := strings.Join(path, "/")
	if skipMenus[slashPath] {
		return
	}

	addNode, hasAdd := node["add"]
	setNode, hasSet := node["set"]

	if (hasAdd || hasSet) && len(path) > 0 {
		if len(w.wanted) == 0 || w.wanted[slashPath] {
			writeTarget := setNode
			if hasAdd {
				writeTarget = addNode
			}

			fields := extractFieldsFromCommand(writeTarget)
			if len(fields) > 0 {
				w.specs = append(w.specs, &ResourceSpec{
					MenuPath:     append([]string(nil), path...),
					RestPath:     "/rest/" + slashPath,
					StructName:   pathToStructName(path),
					IsCollection: hasAdd,
					Fields:       fields,
				})
			}
		}
	}

	for key, val := range node {
		if recurseCmdKeys[key] || strings.HasPrefix(key, ".") || strings.HasPrefix(key, "_") {
			continue
		}
		if childMap, ok := val.(map[string]any); ok {
			w.walkInspectTree(childMap, append(path, key))
		}
	}
}
