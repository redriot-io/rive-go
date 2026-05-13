package main

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestBareStructs(t *testing.T) {
	dir := t.TempDir()
	typesFile := filepath.Join(dir, "types.txt")
	if err := os.WriteFile(typesFile, []byte("Image\nArtboard\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Force reload on next run() call.
	generatedTypes = nil
	typesFilePath = typesFile

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "example")
}
