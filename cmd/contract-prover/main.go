// Contract Prover — generates minimal .riv fixtures per object type and validates
// them against the Rive WASM runtime. Reads format_contract_proposed.json, emits
// format_contract_proven.json with proven:true for every type that passes WASM load.
//
// Usage:
//
//	go run ./cmd/contract-prover/ \
//	    --proposed format_contract_proposed.json \
//	    --out format_contract_proven.json \
//	    --harness tools/wasm-harness/validate.js
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ── JSON schema types ────────────────────────────────────────────────────────

type ProposedEntry struct {
	Type             string                 `json:"type"`
	ParentChain      []string               `json:"parent_chain"`
	RequiredDefaults map[string]interface{} `json:"required_defaults"`
	ToCRequiredKeys  []int                  `json:"toc_required_keys"`
	Notes            string                 `json:"notes,omitempty"`
}

type ProposedContract struct {
	ContractVersion string          `json:"contract_version"`
	ProposedAt      string          `json:"proposed_at"`
	WasmVersion     string          `json:"wasm_version"`
	Notes           string          `json:"notes,omitempty"`
	Entries         []ProposedEntry `json:"entries"`
}

type Suggestion struct {
	Property  string      `json:"property"`
	Value     interface{} `json:"value"`
	Rationale string      `json:"rationale"`
}

type ProvenEntry struct {
	Type             string                 `json:"type"`
	Proven           bool                   `json:"proven"`
	FixturePath      string                 `json:"fixture_path"`
	VerifiedAt       string                 `json:"verified_at"`
	ParentChain      []string               `json:"parent_chain,omitempty"`
	RequiredDefaults map[string]interface{} `json:"required_defaults,omitempty"`
	ToCRequiredKeys  []int                  `json:"toc_required_keys,omitempty"`
	Notes            string                 `json:"notes,omitempty"`
	FailureReason    string                 `json:"failure_reason,omitempty"`
	Suggestions      []Suggestion           `json:"suggestions,omitempty"`
}

type ProvenContract struct {
	ContractVersion string        `json:"contract_version"`
	ProvenAt        string        `json:"proven_at"`
	WasmVersion     string        `json:"wasm_version"`
	Entries         []ProvenEntry `json:"entries"`
}

// ── Main ─────────────────────────────────────────────────────────────────────

func main() {
	proposedPath := flag.String("proposed", "format_contract_proposed.json", "path to hand-authored proposed contract")
	outPath      := flag.String("out", "format_contract_proven.json", "output path for proven contract")
	harness      := flag.String("harness", "tools/wasm-harness/validate.js", "path to validate.js")
	fixtureDir   := flag.String("fixtures", "testdata/prover", "directory for generated .riv fixtures")
	flag.Parse()

	// ── Read proposed contract ────────────────────────────────────────────────
	raw, err := os.ReadFile(*proposedPath)
	if err != nil {
		log.Fatalf("read %s: %v", *proposedPath, err)
	}
	var proposed ProposedContract
	if err := json.Unmarshal(raw, &proposed); err != nil {
		log.Fatalf("parse %s: %v", *proposedPath, err)
	}

	// Build index: type name → proposed entry
	proposedByType := map[string]ProposedEntry{}
	for _, e := range proposed.Entries {
		proposedByType[e.Type] = e
	}

	// ── Prepare fixture directory ─────────────────────────────────────────────
	if err := os.MkdirAll(*fixtureDir, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", *fixtureDir, err)
	}
	if err := os.MkdirAll(filepath.Join(*fixtureDir, "assets"), 0o755); err != nil {
		log.Fatalf("mkdir assets: %v", err)
	}
	// Write test PNG asset for Image fixtures
	pngPath := filepath.Join(*fixtureDir, "assets", "test.png")
	if err := os.WriteFile(pngPath, minimalPNG, 0o644); err != nil {
		log.Fatalf("write test.png: %v", err)
	}

	// ── Process types in dependency order ─────────────────────────────────────
	now := time.Now().UTC().Format(time.RFC3339)
	var proven []ProvenEntry
	passCount, failCount := 0, 0

	for _, typeName := range typeOrder {
		buildFn, hasBuild := buildFuncs[typeName]
		if !hasBuild {
			log.Printf("SKIP  %-20s  (no build function)", typeName)
			continue
		}

		proposedEntry, hasProposed := proposedByType[typeName]
		if !hasProposed {
			log.Printf("SKIP  %-20s  (not in proposed contract)", typeName)
			continue
		}

		// Generate minimal .riv
		rivBytes, buildErr := buildFn()
		fixturePath := filepath.Join(*fixtureDir, typeName+"_minimal.riv")
		if buildErr != nil {
			log.Printf("FAIL  %-20s  (build error: %v)", typeName, buildErr)
			proven = append(proven, ProvenEntry{
				Type:          typeName,
				Proven:        false,
				FixturePath:   fixturePath,
				VerifiedAt:    now,
				ParentChain:   parentChains[typeName],
				FailureReason: fmt.Sprintf("build error: %v", buildErr),
			})
			failCount++
			continue
		}
		if err := os.WriteFile(fixturePath, rivBytes, 0o644); err != nil {
			log.Fatalf("write fixture %s: %v", fixturePath, err)
		}

		// Run WASM harness
		exitCode, stdout, stderr := runHarness(*harness, fixturePath)

		if exitCode == 0 {
			log.Printf("PASS  %-20s  (%s)", typeName, strings.TrimSpace(stdout))
			passCount++
			proven = append(proven, ProvenEntry{
				Type:             typeName,
				Proven:           true,
				FixturePath:      fixturePath,
				VerifiedAt:       now,
				ParentChain:      parentChains[typeName],
				RequiredDefaults: proposedEntry.RequiredDefaults,
				ToCRequiredKeys:  proposedEntry.ToCRequiredKeys,
				Notes:            proposedEntry.Notes,
			})
		} else {
			reason := strings.TrimSpace(stderr)
			if reason == "" {
				reason = strings.TrimSpace(stdout)
			}
			if reason == "" {
				reason = fmt.Sprintf("exit code %d", exitCode)
			}
			log.Printf("FAIL  %-20s  exit=%d reason=%q", typeName, exitCode, reason)
			failCount++
			proven = append(proven, ProvenEntry{
				Type:          typeName,
				Proven:        false,
				FixturePath:   fixturePath,
				VerifiedAt:    now,
				ParentChain:   parentChains[typeName],
				FailureReason: reason,
			})
		}
	}

	// ── Write proven contract ─────────────────────────────────────────────────
	contract := ProvenContract{
		ContractVersion: proposed.ContractVersion,
		ProvenAt:        now,
		WasmVersion:     proposed.WasmVersion,
		Entries:         proven,
	}
	out, err := json.MarshalIndent(contract, "", "  ")
	if err != nil {
		log.Fatalf("marshal proven contract: %v", err)
	}
	if err := os.WriteFile(*outPath, out, 0o644); err != nil {
		log.Fatalf("write %s: %v", *outPath, err)
	}

	fmt.Printf("\n── Contract Prover complete ──\n")
	fmt.Printf("  PASS: %d\n", passCount)
	fmt.Printf("  FAIL: %d\n", failCount)
	fmt.Printf("  Output: %s\n", *outPath)

	if passCount == 0 {
		os.Exit(1)
	}
}

// runHarness invokes `node <harness> <rivPath>` and returns exit code, stdout, stderr.
func runHarness(harness, rivPath string) (int, string, string) {
	cmd := exec.Command("node", harness, rivPath)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			return 3, "", fmt.Sprintf("exec error: %v", err)
		}
	}
	return code, outBuf.String(), errBuf.String()
}
