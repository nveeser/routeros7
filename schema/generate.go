package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	if err := initConfig(); err != nil {
		log.Fatalf("Failed initializing config: %v", err)
	}

	//--url="https://10.10.0.100" -user=robot --pass=robots

	endpoint := viper.GetString("url")
	user := viper.GetString("user")
	pass := viper.GetString("pass")
	insecure := viper.GetBool("insecure")
	pkg := viper.GetString("pkg")
	outFile := viper.GetString("out")
	readOnly := viper.GetBool("readonly")
	menus := viper.GetString("menus")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ins := NewInspector(endpoint, user, pass, insecure)

	var specs []*ResourceSpec
	for _, m := range strings.Split(menus, ";") {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		log.Printf("Inspecting /%s ...", strings.ReplaceAll(m, ",", "/"))
		spec, err := ins.InspectMenu(ctx, m)
		if err != nil {
			log.Fatalf("Failed inspecting %q: %v", m, err)
		}
		specs = append(specs, spec)
	}

	code, err := renderCode(pkg, specs, readOnly)
	if err != nil {
		log.Fatalf("Failed formatting generated code: %v", err)
	}

	if err := os.WriteFile(outFile, code, 0644); err != nil {
		log.Fatalf("Failed writing %s: %v", outFile, err)
	}

	log.Printf("Generated %d structs into %s", len(specs), outFile)
}

// InspectRequest matches POST /rest/console/inspect payload.
type InspectRequest struct {
	Request string `json:"request"`
	Path    string `json:"path"`
}

// InspectNode represents one entry returned by /rest/console/inspect.
type InspectNode struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	NodeType string `json:"node-type"`
	Text     string `json:"text"`
	Symbol   string `json:"symbol"`
}

type FieldSpec struct {
	RosName  string
	GoName   string
	GoType   string
	ReadOnly bool
	Syntax   string
}

type ResourceSpec struct {
	MenuPath     string // e.g. "ip,address"
	RestPath     string // e.g. "/rest/ip/address"
	StructName   string // e.g. "IPAddressConfig"
	IsCollection bool
	Fields       []FieldSpec
}

type Inspector struct {
	baseURL    string
	authHeader string
	client     *http.Client
}

func NewInspector(endpoint, user, pass string, insecure bool) *Inspector {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if insecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		panic(err)
	}
	req.SetBasicAuth(user, pass)

	return &Inspector{
		baseURL:    strings.TrimRight(endpoint, "/"),
		authHeader: req.Header.Get("Authorization"),
		client:     &http.Client{Transport: tr, Timeout: 15 * time.Second},
	}
}

func (ins *Inspector) inspect(ctx context.Context, requestType, commaPath string) ([]InspectNode, error) {
	payload, err := json.Marshal(InspectRequest{
		Request: requestType,
		Path:    commaPath,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ins.baseURL+"/rest/console/inspect", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", ins.authHeader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := ins.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("inspect (%s %s) HTTP %d: %s", requestType, commaPath, resp.StatusCode, string(body))
	}

	var nodes []InspectNode
	if err := json.Unmarshal(body, &nodes); err != nil {
		return nil, fmt.Errorf("unmarshal inspect response: %w", err)
	}
	return nodes, nil
}

// InspectMenu introspects a RouterOS menu (e.g., "ip,address" or "system,clock")
// and extracts its writable and read-only schema fields.
func (ins *Inspector) InspectMenu(ctx context.Context, menuPath string) (*ResourceSpec, error) {
	children, err := ins.inspect(ctx, "child", menuPath)
	if err != nil {
		return nil, err
	}

	hasAdd := false
	hasSet := false
	for _, ch := range children {
		switch ch.Name {
		case "add":
			hasAdd = true
		case "set":
			hasSet = true
		}
	}

	if !hasAdd && !hasSet {
		return nil, fmt.Errorf("menu %q has neither 'add' nor 'set' commands", menuPath)
	}

	spec := &ResourceSpec{
		MenuPath:     menuPath,
		RestPath:     "/rest/" + strings.ReplaceAll(menuPath, ",", "/"),
		StructName:   menuToStructName(menuPath),
		IsCollection: hasAdd,
	}

	// 1. Inspect writable parameters via "add" (for collections) or "set" (for singletons)
	writeCmd := "set"
	if hasAdd {
		writeCmd = "add"
	}
	writeNodes, err := ins.inspect(ctx, "syntax", menuPath+","+writeCmd)
	if err != nil {
		return nil, err
	}

	fieldMap := make(map[string]FieldSpec)
	for _, n := range writeNodes {
		if !isValidParamNode(n) {
			continue
		}
		fieldMap[n.Name] = FieldSpec{
			RosName:  n.Name,
			GoName:   kebabToPascal(n.Name),
			GoType:   inferGoType(n.Name, n.Text),
			ReadOnly: false,
			Syntax:   strings.TrimSpace(n.Text),
		}
	}

	// 2. Optionally inspect "get" syntax to discover read-only runtime properties
	if getNodes, err := ins.inspect(ctx, "syntax", menuPath+",get"); err == nil {
		for _, n := range getNodes {
			if n.Name == "value-name" && n.Text != "" {
				// RouterOS encodes get value-name choices as "(prop1 | prop2 | ...)"
				for _, prop := range parsePipeEnum(n.Text) {
					if _, exists := fieldMap[prop]; !exists && prop != ".id" && prop != ".nextid" {
						fieldMap[prop] = FieldSpec{
							RosName:  prop,
							GoName:   kebabToPascal(prop),
							GoType:   inferGoType(prop, ""),
							ReadOnly: true,
							Syntax:   "read-only",
						}
					}
				}
			}
		}
	}

	for _, f := range fieldMap {
		spec.Fields = append(spec.Fields, f)
	}
	sort.Slice(spec.Fields, func(i, j int) bool {
		// Sort writable fields first, then alphabetically
		if spec.Fields[i].ReadOnly != spec.Fields[j].ReadOnly {
			return !spec.Fields[i].ReadOnly
		}
		return spec.Fields[i].RosName < spec.Fields[j].RosName
	})

	return spec, nil
}

func isValidParamNode(n InspectNode) bool {
	if n.Name == "" || strings.HasPrefix(n.Name, ".") {
		return false
	}
	// Skip positional/control flags that aren't resource attributes
	switch n.Name {
	case "numbers", "place-before", "copy-from", "find":
		return false
	}
	return n.Type == "arg" || n.NodeType == "arg" || n.Type == "syntax"
}

func parsePipeEnum(text string) []string {
	cleaned := strings.TrimSpace(text)
	cleaned = strings.TrimPrefix(cleaned, "(")
	cleaned = strings.TrimSuffix(cleaned, ")")
	parts := strings.Split(cleaned, "|")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// inferGoType maps RouterOS syntax hints and field names to Go types.
func inferGoType(name, syntax string) string {
	s := strings.ToLower(syntax)
	n := strings.ToLower(name)

	// Boolean detection
	if strings.Contains(s, "yes | no") || strings.Contains(s, "true | false") ||
		n == "disabled" || n == "dynamic" || n == "running" || n == "invalid" || n == "enabled" {
		return "RosBool"
	}

	// Slice / comma-delimited list detection
	if strings.HasSuffix(n, "s") && (n == "servers" || n == "vlan-ids" || n == "tagged" || n == "untagged") {
		return "[]string"
	}

	// Integer range detection (e.g., "0..65535", "1..4094", "integer")
	if strings.Contains(s, "..") && !strings.Contains(s, "ip") && !strings.Contains(s, ":") {
		return "RosInt"
	}
	if n == "pvid" || n == "mtu" || n == "distance" || n == "vlan-id" || n == "priority" {
		return "RosInt"
	}

	return "string"
}

func menuToStructName(menuPath string) string {
	parts := strings.Split(menuPath, ",")
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(kebabToPascal(p))
	}
	b.WriteString("Config")
	return b.String()
}

func kebabToPascal(s string) string {
	acronyms := map[string]string{
		"ip": "IP", "ipv6": "IPv6", "id": "ID", "ids": "IDs",
		"dns": "DNS", "ntp": "NTP", "mtu": "MTU", "vlan": "VLAN",
		"vlans": "VLANs", "pvid": "PVID", "bgp": "BGP", "ospf": "OSPF",
		"nat": "NAT", "ttl": "TTL", "hw": "HW", "mac": "MAC", "tcp": "TCP", "udp": "UDP",
	}

	tokens := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || unicode.IsSpace(r)
	})

	var out strings.Builder
	for _, tok := range tokens {
		lower := strings.ToLower(tok)
		if acr, ok := acronyms[lower]; ok {
			out.WriteString(acr)
			continue
		}
		runes := []rune(lower)
		runes[0] = unicode.ToUpper(runes[0])
		out.WriteString(string(runes))
	}
	return out.String()
}

func renderCode(pkgName string, specs []*ResourceSpec, includeReadOnly bool) ([]byte, error) {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "// Code generated by rosgen from RouterOS /console/inspect; DO NOT EDIT.\n")
	fmt.Fprintf(&buf, "package %s\n\n", pkgName)

	for _, spec := range specs {
		fmt.Fprintf(&buf, "// %s maps %s (%s)\n", spec.StructName, spec.RestPath, map[bool]string{true: "collection", false: "singleton"}[spec.IsCollection])
		fmt.Fprintf(&buf, "type %s struct {\n", spec.StructName)

		if spec.IsCollection {
			fmt.Fprintf(&buf, "\tResourceMeta `yaml:\",inline\"`\n")
		}

		for _, f := range spec.Fields {
			if f.ReadOnly && !includeReadOnly {
				continue
			}
			yamlKey := strings.ReplaceAll(f.RosName, "-", "_")
			yamlKey = strings.ReplaceAll(yamlKey, ".", "_")

			if f.ReadOnly {
				// Exclude read-only operational fields from YAML export
				fmt.Fprintf(&buf, "\t%s %s `json:\"%s,omitempty\" yaml:\"-\"` // read-only\n",
					f.GoName, f.GoType, f.RosName)
			} else {
				fmt.Fprintf(&buf, "\t%s %s `json:\"%s,omitempty\" yaml:\"%s,omitempty\"`\n",
					f.GoName, f.GoType, f.RosName, yamlKey)
			}
		}

		fmt.Fprintf(&buf, "}\n\n")
		fmt.Fprintf(&buf, "func (%s) RestPath() string { return %q }\n\n", spec.StructName, spec.RestPath)
	}

	return format.Source(buf.Bytes())
}

func initConfig() error {
	pflag.String("env-file", ".env", "Path to .env file")
	pflag.String("url", "https://192.168.88.1", "RouterOS base URL")
	pflag.String("user", "admin", "RouterOS username")
	pflag.String("pass", "", "RouterOS password")
	pflag.Bool("insecure", true, "Skip TLS certificate verification")
	pflag.String("pkg", "config", "Generated Go package name")
	pflag.String("out", "ros_models_gen.go", "Output Go file path")
	pflag.Bool("readonly", false, "Include read-only status fields (tagged yaml:\"-\")")
	pflag.String("menus", "system,clock;system,ntp,client;interface,bridge;interface,bridge,port;interface,bridge,vlan;interface,vlan;ip,address;ip,dns;ip,route", "Semicolon-separated comma paths to inspect")
	pflag.Parse()

	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		return fmt.Errorf("bind flags: %w", err)
	}

	viper.SetEnvPrefix("ROUTEROS")
	viper.AutomaticEnv()

	_ = viper.BindEnv("url", "ROUTEROS_URL", "ROS_URL")
	_ = viper.BindEnv("user", "ROUTEROS_USER", "ROUTEROS_USERNAME", "ROS_USER", "ROS_USERNAME")
	_ = viper.BindEnv("pass", "ROUTEROS_PASS", "ROUTEROS_PASSWORD", "ROS_PASS", "ROS_PASSWORD")
	_ = viper.BindEnv("insecure", "ROUTEROS_INSECURE", "ROS_INSECURE")
	_ = viper.BindEnv("pkg", "ROUTEROS_PKG", "ROS_PKG")
	_ = viper.BindEnv("out", "ROUTEROS_OUT", "ROS_OUT")
	_ = viper.BindEnv("readonly", "ROUTEROS_READONLY", "ROS_READONLY")
	_ = viper.BindEnv("menus", "ROUTEROS_MENUS", "ROS_MENUS")

	envFile := viper.GetString("env-file")
	if !pflag.CommandLine.Changed("env-file") {
		if _, err := os.Stat(envFile); errors.Is(err, os.ErrNotExist) {
			if _, err := os.Stat("../.env"); err == nil {
				envFile = "../.env"
			}
		}
	}
	viper.SetConfigFile(envFile)
	viper.SetConfigType("env")

	if err := viper.ReadInConfig(); err != nil {
		var cfgNotFound viper.ConfigFileNotFoundError
		if pflag.CommandLine.Changed("env-file") || (!errors.Is(err, os.ErrNotExist) && !errors.As(err, &cfgNotFound)) {
			return fmt.Errorf("read env file %s: %w", envFile, err)
		}
	}

	// Register aliases so .env files work with either prefixed or unprefixed keys.
	for _, a := range []struct{ alias, key string }{
		{"routeros_url", "url"},
		{"ros_url", "url"},
		{"routeros_user", "user"},
		{"routeros_username", "user"},
		{"ros_user", "user"},
		{"ros_username", "user"},
		{"username", "user"},
		{"routeros_pass", "pass"},
		{"routeros_password", "pass"},
		{"ros_pass", "pass"},
		{"ros_password", "pass"},
		{"password", "pass"},
		{"routeros_insecure", "insecure"},
		{"ros_insecure", "insecure"},
		{"routeros_pkg", "pkg"},
		{"ros_pkg", "pkg"},
		{"routeros_out", "out"},
		{"ros_out", "out"},
		{"routeros_readonly", "readonly"},
		{"ros_readonly", "readonly"},
		{"routeros_menus", "menus"},
		{"ros_menus", "menus"},
	} {
		viper.RegisterAlias(a.alias, a.key)
	}

	return nil
}
