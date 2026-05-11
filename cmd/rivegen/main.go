// rivegen generates Go types and registry from Rive dev/defs JSON schemas.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// ── JSON schema types ────────────────────────────────────────────────────────

type Schema struct {
	Name       string                 `json:"name"`
	Key        *SchemaKey             `json:"key"`
	Extends    string                 `json:"extends"`
	Abstract   bool                   `json:"abstract"`
	EditorOnly bool                   `json:"editorOnly"`
	Mixins     []string               `json:"mixins"`
	Properties map[string]*SchemaProp `json:"properties"`
}

func (s *Schema) isAbstract() bool {
	return s.Abstract || s.Key == nil || s.Key.Int == 0
}

type SchemaKey struct {
	Int    int    `json:"int"`
	String string `json:"string"`
}

type SchemaProp struct {
	Type                string  `json:"type"`
	TypeRuntime         string  `json:"typeRuntime"`
	InitialValue        string  `json:"initialValue"`
	InitialValueRuntime string  `json:"initialValueRuntime"`
	Key                 PropKey `json:"key"`
	Runtime             *bool   `json:"runtime"` // nil → true
	Passthrough         bool    `json:"passthrough"`
	Animates            bool    `json:"animates"`
}

func (p *SchemaProp) isRuntime() bool {
	if p.Runtime != nil && !*p.Runtime {
		return false
	}
	if p.Passthrough {
		return false
	}
	eff := p.effectiveType()
	return eff != "callback" && eff != "FractionalIndex"
}

func (p *SchemaProp) effectiveType() string {
	if p.TypeRuntime != "" {
		return p.TypeRuntime
	}
	return p.Type
}

type PropKey struct {
	Int        int       `json:"int"`
	String     string    `json:"string"`
	Alternates []PropKey `json:"alternates"`
}

// ── Loaded entry ──────────────────────────────────────────────────────────────

type Entry struct {
	Schema   *Schema
	Filename string // relative to defs root, e.g., "shapes/rectangle.json"
	Category string // "root", "animation", "shapes", …
}

// ── Go type helpers ───────────────────────────────────────────────────────────

func goType(p *SchemaProp) string {
	switch p.effectiveType() {
	case "double":
		return "float64"
	case "uint", "Id":
		return "uint64"
	case "bool":
		return "bool"
	case "String":
		return "string"
	case "Color":
		return "uint32"
	case "Bytes", "List<Id>":
		return "[]byte"
	default:
		return "uint64"
	}
}

func propTypeConst(p *SchemaProp) string {
	switch p.effectiveType() {
	case "double":
		return "PropertyTypeFloat"
	case "String":
		return "PropertyTypeString"
	case "Color":
		return "PropertyTypeColor"
	case "Bytes", "List<Id>":
		return "PropertyTypeBytes"
	default:
		return "PropertyTypeUint" // uint, Id, bool
	}
}

// propTypeCode returns the PropertyType numeric value for the global table.
func propTypeCode(p *SchemaProp) int {
	switch p.effectiveType() {
	case "double":
		return 2 // PropertyTypeFloat
	case "String":
		return 1 // PropertyTypeString
	case "Color":
		return 3 // PropertyTypeColor
	case "Bytes", "List<Id>":
		return 4 // PropertyTypeBytes
	default:
		return 0 // PropertyTypeUint (uint, Id, bool)
	}
}

// defaultCondition returns a boolean expression that is true when the field
// differs from its default (i.e., should be emitted in Properties()).
func defaultCondition(field string, p *SchemaProp) string {
	initVal := p.InitialValueRuntime
	if initVal == "" {
		initVal = p.InitialValue
	}
	switch p.effectiveType() {
	case "double":
		v, err := strconv.ParseFloat(initVal, 64)
		if err != nil || v == 0 {
			return fmt.Sprintf("o.%s != 0", field)
		}
		return fmt.Sprintf("o.%s != %v", field, v)
	case "uint", "Id":
		switch initVal {
		case "", "Core.missingId":
			return fmt.Sprintf("o.%s != 0", field)
		case "-1":
			return fmt.Sprintf("o.%s != ^uint64(0)", field)
		default:
			v, err := strconv.ParseUint(initVal, 10, 64)
			if err != nil || v == 0 {
				return fmt.Sprintf("o.%s != 0", field)
			}
			return fmt.Sprintf("o.%s != %d", field, v)
		}
	case "bool":
		if initVal == "true" {
			return fmt.Sprintf("!o.%s", field)
		}
		return fmt.Sprintf("o.%s", field)
	case "String":
		return fmt.Sprintf(`o.%s != ""`, field)
	case "Color":
		return fmt.Sprintf("o.%s != 0", field)
	case "Bytes", "List<Id>":
		return fmt.Sprintf("len(o.%s) > 0", field)
	default:
		return fmt.Sprintf("o.%s != 0", field)
	}
}

// propValue returns the expression to pass as Property.Value.
func propValue(field string, p *SchemaProp) string {
	switch p.effectiveType() {
	case "bool":
		return fmt.Sprintf("boolToUint64(o.%s)", field)
	case "Color":
		return fmt.Sprintf("uint64(o.%s)", field)
	default:
		return fmt.Sprintf("o.%s", field)
	}
}

// toPascalCase uppercases the first rune of a camelCase identifier.
func toPascalCase(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// ── Load ──────────────────────────────────────────────────────────────────────

func loadDefs(defsDir string) (map[string]*Entry, error) {
	entries := map[string]*Entry{}
	err := filepath.Walk(defsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		rel, _ := filepath.Rel(defsDir, path)
		rel = filepath.ToSlash(rel)

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var s Schema
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}

		cat := "root"
		if idx := strings.Index(rel, "/"); idx >= 0 {
			cat = rel[:idx]
		}

		entries[rel] = &Entry{Schema: &s, Filename: rel, Category: cat}
		return nil
	})
	return entries, err
}

// parentGoName resolves the Go struct name of the parent type.
func parentGoName(e *Entry, all map[string]*Entry) string {
	if e.Schema.Extends == "" {
		return ""
	}
	pe := all[e.Schema.Extends]
	if pe == nil {
		return ""
	}
	return pe.Schema.Name
}

// sortedRuntimeProps returns runtime properties in key.int order.
func sortedRuntimeProps(s *Schema) []*namedProp {
	var out []*namedProp
	for pname, p := range s.Properties {
		if p.isRuntime() {
			out = append(out, &namedProp{name: pname, prop: p})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].prop.Key.Int < out[j].prop.Key.Int
	})
	return out
}

type namedProp struct {
	name string
	prop *SchemaProp
}

// ── Code generation ───────────────────────────────────────────────────────────

func generateCategoryFile(cat string, entries []*Entry, all map[string]*Entry) ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated by rivegen. DO NOT EDIT.\npackage rive\n\n")

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Schema.Name < entries[j].Schema.Name
	})

	for _, e := range entries {
		s := e.Schema
		if s.EditorOnly {
			continue
		}

		parent := parentGoName(e, all)
		ownProps := sortedRuntimeProps(s)

		abstract := ""
		if s.isAbstract() {
			abstract = " [abstract]"
		}
		keyInt := 0
		if s.Key != nil {
			keyInt = s.Key.Int
		}
		fmt.Fprintf(&b, "// %s (typeKey: %d)%s\n", s.Name, keyInt, abstract)
		fmt.Fprintf(&b, "type %s struct {\n", s.Name)
		if parent != "" {
			fmt.Fprintf(&b, "\t%s\n", parent)
		}
		for _, np := range ownProps {
			field := toPascalCase(np.name)
			tag := fmt.Sprintf("`rive:\"%d,%s\"`", np.prop.Key.Int, propTagType(np.prop))
			fmt.Fprintf(&b, "\t%s %s %s\n", field, goType(np.prop), tag)
		}
		fmt.Fprintf(&b, "}\n\n")

		if !s.isAbstract() {
			fmt.Fprintf(&b, "func (o *%s) TypeKey() uint32 { return %d }\n\n", s.Name, s.Key.Int)
		}

		fmt.Fprintf(&b, "func (o *%s) Properties() []Property {\n", s.Name)
		if parent == "" && len(ownProps) == 0 {
			fmt.Fprintf(&b, "\treturn nil\n")
		} else if parent != "" && len(ownProps) == 0 {
			fmt.Fprintf(&b, "\treturn o.%s.Properties()\n", parent)
		} else {
			if parent != "" {
				fmt.Fprintf(&b, "\tprops := o.%s.Properties()\n", parent)
			} else {
				fmt.Fprintf(&b, "\tvar props []Property\n")
			}
			for _, np := range ownProps {
				field := toPascalCase(np.name)
				cond := defaultCondition(field, np.prop)
				ptype := propTypeConst(np.prop)
				val := propValue(field, np.prop)
				fmt.Fprintf(&b, "\tif %s {\n", cond)
				fmt.Fprintf(&b, "\t\tprops = append(props, Property{Key: %d, Type: %s, Value: %s})\n",
					np.prop.Key.Int, ptype, val)
				fmt.Fprintf(&b, "\t}\n")
			}
			fmt.Fprintf(&b, "\treturn props\n")
		}
		fmt.Fprintf(&b, "}\n\n")
	}

	return format.Source(b.Bytes())
}

func propTagType(p *SchemaProp) string {
	switch p.effectiveType() {
	case "double":
		return "float"
	case "uint", "Id":
		return "uint"
	case "bool":
		return "bool"
	case "String":
		return "string"
	case "Color":
		return "color"
	case "Bytes", "List<Id>":
		return "bytes"
	default:
		return "uint"
	}
}

func generateRegistry(all map[string]*Entry) ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated by rivegen. DO NOT EDIT.\npackage rive\n\n")
	fmt.Fprintf(&b, "// Registry maps typeKey → constructor for all concrete Rive object types.\n")
	fmt.Fprintf(&b, "var Registry = map[uint32]func() Object{\n")

	type regEntry struct {
		key  int
		name string
	}
	var entries []regEntry
	for _, e := range all {
		s := e.Schema
		if s.EditorOnly || s.isAbstract() {
			continue
		}
		entries = append(entries, regEntry{s.Key.Int, s.Name})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].key < entries[j].key })

	for _, re := range entries {
		fmt.Fprintf(&b, "\t%d: func() Object { return &%s{} },\n", re.key, re.name)
	}
	fmt.Fprintf(&b, "}\n")
	return format.Source(b.Bytes())
}

func generateProperties(all map[string]*Entry) ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated by rivegen. DO NOT EDIT.\npackage rive\n\n")
	fmt.Fprintf(&b, "// Property key constants — PropKey<N> = N, unique across all Rive types.\nconst (\n")

	type keyEntry struct {
		key  int
		name string
	}
	seen := map[int]bool{}
	var keys []keyEntry
	for _, e := range all {
		for pname, p := range e.Schema.Properties {
			k := p.Key.Int
			if seen[k] {
				continue
			}
			seen[k] = true
			keys = append(keys, keyEntry{k, pname})
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].key < keys[j].key })
	for _, ke := range keys {
		fmt.Fprintf(&b, "\tPropKey%d = %d // %s\n", ke.key, ke.key, ke.name)
	}
	fmt.Fprintf(&b, ")\n")
	return format.Source(b.Bytes())
}

// generatePropTypeTable produces a global map from every known property key to
// its wire PropertyType. Used by the reader as a fallback when a property key
// is not present in the file's ToC.
func generatePropTypeTable(all map[string]*Entry) ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated by rivegen. DO NOT EDIT.\npackage rive\n\n")
	fmt.Fprintf(&b, "// globalPropTypes maps every known Rive property key to its wire PropertyType.\n")
	fmt.Fprintf(&b, "// Used by the reader when a property key is absent from the file's ToC.\n")
	fmt.Fprintf(&b, "var globalPropTypes = map[uint32]PropertyType{\n")

	type entry struct {
		key      int
		typecode int
		name     string
	}
	seen := map[int]bool{}
	var entries []entry
	addKey := func(k int, tc int, name string) {
		if seen[k] {
			return
		}
		seen[k] = true
		entries = append(entries, entry{k, tc, name})
	}
	for _, e := range all {
		for pname, p := range e.Schema.Properties {
			tc := propTypeCode(p)
			addKey(p.Key.Int, tc, pname)
			for _, alt := range p.Key.Alternates {
				addKey(alt.Int, tc, alt.String)
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].key < entries[j].key })
	for _, en := range entries {
		fmt.Fprintf(&b, "\t%d: %d, // %s\n", en.key, en.typecode, en.name)
	}
	fmt.Fprintf(&b, "}\n")
	return format.Source(b.Bytes())
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	defsDir := flag.String("defs", "internal/schema/defs", "path to dev/defs directory")
	outDir := flag.String("out", "rive", "output directory for generated Go files")
	contractPath := flag.String("contract", "", "path to format_contract.json (generates gen_format_rules.go)")
	flag.Parse()

	// -contract mode: generate gen_format_rules.go from format_contract.json
	if *contractPath != "" {
		if err := generateFormatRules(*contractPath, *outDir); err != nil {
			log.Fatalf("generate format rules: %v", err)
		}
		log.Printf("wrote %s/gen_format_rules.go", *outDir)
		return
	}

	all, err := loadDefs(*defsDir)
	if err != nil {
		log.Fatalf("load defs: %v", err)
	}
	log.Printf("Loaded %d type definitions", len(all))

	if err := os.MkdirAll(*outDir, 0755); err != nil {
		log.Fatal(err)
	}

	byCategory := map[string][]*Entry{}
	for _, e := range all {
		byCategory[e.Category] = append(byCategory[e.Category], e)
	}

	cats := make([]string, 0, len(byCategory))
	for c := range byCategory {
		cats = append(cats, c)
	}
	sort.Strings(cats)

	generated := 0
	for _, cat := range cats {
		entries := byCategory[cat]
		src, err := generateCategoryFile(cat, entries, all)
		if err != nil {
			log.Fatalf("generate %s: %v", cat, err)
		}
		fname := filepath.Join(*outDir, "gen_"+cat+".go")
		if err := os.WriteFile(fname, src, 0644); err != nil {
			log.Fatal(err)
		}
		log.Printf("  wrote %s (%d types)", fname, len(entries))
		generated += len(entries)
	}

	for _, pair := range []struct {
		name string
		fn   func(map[string]*Entry) ([]byte, error)
	}{
		{"gen_registry.go", generateRegistry},
		{"gen_properties.go", generateProperties},
		{"gen_prop_type_table.go", generatePropTypeTable},
	} {
		src, err := pair.fn(all)
		if err != nil {
			log.Fatalf("generate %s: %v", pair.name, err)
		}
		if err := os.WriteFile(filepath.Join(*outDir, pair.name), src, 0644); err != nil {
			log.Fatal(err)
		}
		log.Printf("  wrote %s/%s", *outDir, pair.name)
	}

	log.Printf("Done: %d types across %d categories", generated, len(cats))
}
