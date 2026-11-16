package inspect

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"log"
	"net/http"
	"os"
	"slices"
	"sort"
	"strings"
)

type ResourceSpec struct {
	MenuPath     []string // e.g., ["ip", "address"]
	StructName   string   // e.g., "IPAddressConfig"
	RestPath     string   // e.g., "/rest/ip/address"
	IsCollection bool
	Children     map[string]*ResourceSpec // Sub-containers (e.g., "firewall" under "ip")
	Fields       []FieldSpec
	Resources    []ResourceSpec
}

func (r *ResourceSpec) Path() string {
	if len(r.MenuPath) == 0 {
		return "<root>"
	}
	return strings.Join(r.MenuPath, ",")
}
func (r *ResourceSpec) Filename() string {
	if len(r.MenuPath) == 0 {
		return "model"
	}
	return r.MenuPath[0]
}

func (r *ResourceSpec) IsConfig() bool {
	return len(r.Resources) > 0
}

func (r *ResourceSpec) Walk() iter.Seq2[int, *ResourceSpec] { return r.walk(0) }

func (r *ResourceSpec) walk(depth int) iter.Seq2[int, *ResourceSpec] {
	return func(yield func(int, *ResourceSpec) bool) {
		if !yield(depth, r) {
			return
		}
		for _, s := range r.Children {
			for d, x := range s.walk(depth + 1) {
				if !yield(d, x) {
					return
				}
			}
		}
	}
}

type FieldSpec struct {
	GoName       string
	GoType       string
	JSONKey      string // JSON key
	YAMLKey      string // e.g. "address", "dns", "bridge_port"
	ReadOnly     bool
	IsCollection bool
	IsConfig     bool
}

func GenerateURL(ctx context.Context, url string) (*ResourceSpec, error) {
	rawJSON, err := loadSpec(ctx, url)
	if err != nil {
		log.Fatalf("Load spec failed: %v", err)
	}
	return GenerateJSON(rawJSON)
}

func GenerateJSON(rawJSON []byte) (*ResourceSpec, error) {
	var root map[string]any
	if err := json.Unmarshal(rawJSON, &root); err != nil {
		log.Fatalf("Unmarshal inspect.json: %v", err)
	}
	return Generate(root), nil
}

func Generate(root map[string]any) *ResourceSpec {
	w := &walker{wanted: nil}
	w.walkInspectTree(root, nil)

	sort.Slice(w.specs, func(i, j int) bool {
		return w.specs[i].RestPath < w.specs[j].RestPath
	})
	return buildAggregateTree(w.specs)
}

func loadSpec(ctx context.Context, src string) ([]byte, error) {
	if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
		return os.ReadFile(src)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, src)
	}
	return io.ReadAll(resp.Body)
}

// buildAggregateTree groups flat ResourceSpecs into hierarchical aggregate structs.
func buildAggregateTree(specs []*ResourceSpec) *ResourceSpec {
	root := &ResourceSpec{
		StructName: "RouterConfig",
		Children:   make(map[string]*ResourceSpec),
	}

	// Index specs by MenuPath so we can detect hybrid nodes (menus that are both resource + parent)
	specByPath := make(map[string]*ResourceSpec, len(specs))
	for _, s := range specs {
		specByPath[s.Path()] = s
	}

	// Ensure pure directory nodes exist for multi-segment paths
	for _, spec := range specs {
		fmt.Printf("(%s): START\n", spec.Path())
		parent := findParent(root, spec.MenuPath, specByPath)
		fmt.Printf("\t%s PARENT\n", parent.Path())
		addChildResource(parent, spec)
	}
	return root
}

func findParent(root *ResourceSpec, path []string, specByPath map[string]*ResourceSpec) *ResourceSpec {
	curr := root
	// Walk ancestor segments, stopping before the leaf resource itself.
	// If an ancestor is itself a ResourceSpec (hybrid menu like "interface,bridge"),
	// keep attaching descendants to the nearest pure directory ("interface").
	for nextPath, step := range iterateMenuPath(path) {

		next, exists := curr.Children[step]
		if !exists {
			next = &ResourceSpec{
				MenuPath:   slices.Clone(nextPath), // append([]string{}, nextPath...),
				StructName: pathToStructName(nextPath) + "Config",
				Children:   make(map[string]*ResourceSpec),
			}
			curr.Children[step] = next
			fmt.Printf("\t%s ADD\n", next.Path())
			field := FieldSpec{
				GoName:   next.StructName,
				GoType:   next.StructName,
				JSONKey:  step,
				YAMLKey:  strings.ReplaceAll(step, "-", "_"),
				ReadOnly: false,
				IsConfig: true,
			}
			fmt.Printf("\tADD FIELD %s.%s\n", curr.StructName, field.GoName)
			curr.Fields = append(curr.Fields, field)
		}
		curr = next
	}
	return curr
}

func addChildResource(parent *ResourceSpec, s *ResourceSpec) {
	// Relative segments from the container down to this leaf resource
	relName := strings.Join(s.MenuPath[len(parent.MenuPath):], "-")

	goType := s.StructName
	if s.IsCollection {
		goType = "[]" + s.StructName
	}
	field := FieldSpec{
		GoName:       kebabToPascal(relName),
		GoType:       goType,
		YAMLKey:      strings.ReplaceAll(relName, "-", "_"),
		JSONKey:      relName,
		IsCollection: s.IsCollection,
		IsConfig:     s.IsConfig(),
	}
	parent.Fields = append(parent.Fields, field)
	parent.Resources = append(parent.Resources, *s)

	sort.Slice(parent.Fields, func(i, j int) bool {
		return parent.Fields[i].GoName < parent.Fields[j].GoName
	})
}

func iterateMenuPath(menuPath []string) iter.Seq2[[]string, string] {
	return func(yield func([]string, string) bool) {
		for i, step := range menuPath {
			if i == len(menuPath)-1 {
				return
			}
			if !yield(menuPath[:i+1], step) {
				return
			}
		}
	}
}
