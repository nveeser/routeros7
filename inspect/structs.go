package inspect

import (
	"fmt"
	"go/token"
	"sort"
	"strings"
	"unicode"
)

var acronyms = map[string]string{
	"ip":    "IP",
	"ipv6":  "IPv6",
	"id":    "ID",
	"ids":   "IDs",
	"dns":   "DNS",
	"ntp":   "",
	"mtu":   "",
	"vlan":  "",
	"vlans": "VLANs",
	"pvid":  "",
	"bgp":   "",
	"ospf":  "",
	"dhcp":  "",
	"nat":   "",
	"ttl":   "",
	"hw":    "",
	"mac":   "",
	"tcp":   "",
	"udp":   "",
}

// extractFieldsFromCommand handles both map-style and slice-style parameter representations in inspect.json.
func extractFieldsFromCommand(cmdNode any) []FieldSpec {
	fieldMap := make(map[string]FieldSpec)
	addParam := func(name, syntax string) {
		if name == "" || strings.HasPrefix(name, ".") {
			return
		}
		switch name {
		case "numbers", "place-before", "copy-from", "find":
			return
		}
		fieldMap[name] = FieldSpec{
			JSONKey: name,
			GoName:  kebabToPascal(name),
			GoType:  inferGoType(name, syntax),
			YAMLKey: strings.ReplaceAll(strings.ReplaceAll(name, "-", "_"), ".", "_"),
		}
	}

	switch typed := cmdNode.(type) {
	case map[string]any:
		// Format A: {"address": {"type": "arg", "text": "..."}, ...}
		for paramName, meta := range typed {
			if strings.HasPrefix(paramName, "_") || strings.HasPrefix(paramName, ".") {
				continue
			}
			syntax := ""
			if metaMap, ok := meta.(map[string]any); ok {
				if t, ok := metaMap["text"].(string); ok {
					syntax = t
				}
			}
			addParam(paramName, syntax)
		}

	case []any:
		// Format B: [{"type": "arg", "name": "address", "text": "..."}, ...]
		for _, item := range typed {
			if m, ok := item.(map[string]any); ok {
				nodeType, _ := m["type"].(string)
				name, _ := m["name"].(string)
				text, _ := m["text"].(string)
				if nodeType == "arg" || nodeType == "syntax" {
					addParam(name, text)
				}
			}
		}
	}

	var out []FieldSpec
	for _, f := range fieldMap {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].JSONKey < out[j].JSONKey
	})
	return out
}

func pathToStructName(parts []string) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(kebabToPascal(p))
	}
	//b.WriteString("Config")
	return b.String()
}

func kebabToPascal(s string) string {
	tokens := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || unicode.IsSpace(r)
	})
	var out strings.Builder
	for _, tok := range tokens {
		lower := strings.ToLower(tok)
		if acr, ok := acronyms[lower]; ok {
			if acr != "" {
				out.WriteString(acr)
			} else {
				out.WriteString(strings.ToUpper(lower))
			}
			continue
		}
		runes := []rune(lower)
		runes[0] = unicode.ToUpper(runes[0])
		out.WriteString(string(runes))
	}
	goName := out.String()
	if !token.IsIdentifier(goName) {
		return fmt.Sprintf("Field_%s", goName)
	}
	return goName
}

func inferGoType(name, syntax string) string {
	s := strings.ToLower(syntax)
	n := strings.ToLower(name)

	if strings.Contains(s, "yes | no") || strings.Contains(s, "true | false") ||
		n == "disabled" || n == "dynamic" || n == "running" || n == "invalid" || n == "enabled" {
		return "MBool"
	}

	if n == "servers" || n == "vlan-ids" || n == "tagged" || n == "untagged" {
		return "[]string"
	}

	if (strings.Contains(s, "..") && !strings.Contains(s, "ip") && !strings.Contains(s, ":")) ||
		n == "pvid" || n == "mtu" || n == "distance" || n == "vlan-id" || n == "priority" {
		return "MInt"
	}

	return "string"
}
