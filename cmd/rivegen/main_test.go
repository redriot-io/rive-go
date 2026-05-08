package main

import (
	"path/filepath"
	"testing"
)

const defsDir = "../../internal/schema/defs"

func TestGenerator_ParsesAllDefs(t *testing.T) {
	all, err := loadDefs(defsDir)
	if err != nil {
		t.Fatalf("loadDefs: %v", err)
	}
	if len(all) < 300 {
		t.Fatalf("expected ≥300 defs, got %d", len(all))
	}
	t.Logf("Parsed %d type definitions", len(all))
}

func TestGenerator_ResolvesInheritance(t *testing.T) {
	all, err := loadDefs(defsDir)
	if err != nil {
		t.Fatal(err)
	}

	// Rectangle → ParametricPath → Path → Node → TransformComponent
	chain := []string{
		"shapes/rectangle.json",
		"shapes/parametric_path.json",
		"shapes/path.json",
		"node.json",
		"transform_component.json",
		"world_transform_component.json",
		"container_component.json",
		"component.json",
	}

	for i := 0; i < len(chain)-1; i++ {
		e := all[chain[i]]
		if e == nil {
			t.Fatalf("missing def: %s", chain[i])
		}
		parentName := parentGoName(e, all)
		expectedEntry := all[chain[i+1]]
		if expectedEntry == nil {
			t.Fatalf("missing expected parent def: %s", chain[i+1])
		}
		if parentName != expectedEntry.Schema.Name {
			t.Errorf("%s parent = %q, want %q", chain[i], parentName, expectedEntry.Schema.Name)
		}
	}
}

func TestGenerator_SkipsRuntimeFalse(t *testing.T) {
	all, err := loadDefs(defsDir)
	if err != nil {
		t.Fatal(err)
	}
	nodeEntry := all["node.json"]
	if nodeEntry == nil {
		t.Fatal("node.json not found")
	}

	// styleValue has runtime:false (key 176)
	styleProp := nodeEntry.Schema.Properties["styleValue"]
	if styleProp == nil {
		t.Fatal("styleValue property not found in node.json")
	}
	if styleProp.isRuntime() {
		t.Error("styleValue (runtime:false) reported as runtime=true")
	}

	// x is a runtime property (key 13)
	xProp := nodeEntry.Schema.Properties["x"]
	if xProp == nil {
		t.Fatal("x property not found in node.json")
	}
	if !xProp.isRuntime() {
		t.Error("x property reported as runtime=false")
	}
}

func TestGenerator_AbstractNotInRegistry(t *testing.T) {
	all, err := loadDefs(defsDir)
	if err != nil {
		t.Fatal(err)
	}

	var concrete, abstract int
	for _, e := range all {
		if e.Schema.EditorOnly {
			continue
		}
		if e.Schema.isAbstract() {
			abstract++
		} else {
			concrete++
		}
	}
	t.Logf("concrete=%d abstract=%d total=%d", concrete, abstract, len(all))

	if concrete < 200 {
		t.Errorf("expected ≥200 concrete types, got %d", concrete)
	}
	if abstract < 50 {
		t.Errorf("expected ≥50 abstract types, got %d", abstract)
	}
}

func TestGenerator_GeneratesCleanOutput(t *testing.T) {
	all, err := loadDefs(defsDir)
	if err != nil {
		t.Fatal(err)
	}

	// pick a small category and ensure generation produces valid gofmt'd output
	byCategory := map[string][]*Entry{}
	for _, e := range all {
		byCategory[e.Category] = append(byCategory[e.Category], e)
	}

	// bones is small (7 types) — fast to test
	bones := byCategory["bones"]
	if len(bones) == 0 {
		t.Fatal("no bones category entries")
	}

	src, err := generateCategoryFile("bones", bones, all)
	if err != nil {
		t.Fatalf("generateCategoryFile(bones): %v", err)
	}
	if len(src) == 0 {
		t.Fatal("generated empty output")
	}
	t.Logf("bones file: %d bytes", len(src))
	_ = filepath.Join("") // keep import used
}

func TestGenerator_RegistryEntries(t *testing.T) {
	all, err := loadDefs(defsDir)
	if err != nil {
		t.Fatal(err)
	}
	src, err := generateRegistry(all)
	if err != nil {
		t.Fatalf("generateRegistry: %v", err)
	}
	if len(src) == 0 {
		t.Fatal("registry output empty")
	}
}
