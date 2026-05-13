// Contract Prover — generates minimal .riv fixtures per object type and validates
// them against the Rive WASM runtime. Reads format_contract_proposed.json, emits
// format_contract_proven.json with proven:true for every type that passes WASM load.
// On failure, runs property bisection to identify which required_defaults are missing.
//
// Usage:
//
//	go run ./cmd/contract-prover/ \
//	    --proposed format_contract_proposed.json \
//	    --out format_contract_proven.json \
//	    --harness tools/wasm-harness/validate.js
//
// Demo (bisection):
//
//	go run ./cmd/contract-prover/ \
//	    --proposed format_contract_proposed.json \
//	    --out format_contract_proven.json \
//	    --harness tools/wasm-harness/validate.js \
//	    --force-fail Image
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	ToCRequiredKeys  []int                  `json:"toc_required_keys"`
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
	forceFail    := flag.String("force-fail", "", "comma-separated type names to force into broken/bisect mode (demo)")
	flag.Parse()

	// Parse --force-fail list into a set for O(1) lookup.
	forceFailSet := map[string]bool{}
	if *forceFail != "" {
		for _, t := range strings.Split(*forceFail, ",") {
			forceFailSet[strings.TrimSpace(t)] = true
		}
	}

	// ── Read proposed contract ────────────────────────────────────────────────
	raw, err := os.ReadFile(*proposedPath)
	if err != nil {
		log.Fatalf("read %s: %v", *proposedPath, err)
	}
	var proposed ProposedContract
	if err := json.Unmarshal(raw, &proposed); err != nil {
		log.Fatalf("parse %s: %v", *proposedPath, err)
	}

	// Build index: type name → proposed entry.
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
	// Write test PNG asset alongside fixtures for reference.
	pngPath := filepath.Join(*fixtureDir, "assets", "test.png")
	if err := os.WriteFile(pngPath, minimalPNG, 0o644); err != nil {
		log.Fatalf("write test.png: %v", err)
	}

	// ── Process types in dependency order ─────────────────────────────────────
	now := time.Now().UTC().Format(time.RFC3339)
	var proven []ProvenEntry
	passCount, failCount := 0, 0

	for _, typeName := range typeOrder {
		proposedEntry, hasProposed := proposedByType[typeName]
		if !hasProposed {
			log.Printf("SKIP  %-20s  (not in proposed contract)", typeName)
			continue
		}

		fixturePath := filepath.Join(*fixtureDir, typeName+"_minimal.riv")
		forcedBroken := forceFailSet[typeName]

		// ── Build the fixture .riv ─────────────────────────────────────────
		var rivBytes []byte
		var buildErr error

		if forcedBroken {
			// Use the broken build function (for bisection demo).
			bisectFn, hasBisect := bisectFuncs[typeName]
			if !hasBisect {
				log.Printf("SKIP  %-20s  (--force-fail set but no bisect function)", typeName)
				continue
			}
			rivBytes, buildErr = bisectFn(nil) // nil = broken state
		} else {
			buildFn, hasBuild := buildFuncs[typeName]
			if !hasBuild {
				log.Printf("SKIP  %-20s  (no build function)", typeName)
				continue
			}
			rivBytes, buildErr = buildFn()
		}

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

		// ── Run WASM harness ───────────────────────────────────────────────
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
				ToCRequiredKeys:  tocCoerce(proposedEntry.ToCRequiredKeys),
				Notes:            proposedEntry.Notes,
			})
			continue
		}

		// ── FAIL path: run bisection ───────────────────────────────────────
		reason := strings.TrimSpace(stderr)
		if reason == "" {
			reason = strings.TrimSpace(stdout)
		}
		if reason == "" {
			reason = fmt.Sprintf("exit code %d", exitCode)
		}
		log.Printf("FAIL  %-20s  exit=%d reason=%q", typeName, exitCode, reason)
		failCount++

		var suggestions []Suggestion
		bisectFn, hasBisect := bisectFuncs[typeName]
		if hasBisect && len(proposedEntry.RequiredDefaults) > 0 {
			candidates := defaultsToCandidates(proposedEntry.RequiredDefaults)
			suggestions = bisect(typeName, *harness, candidates, bisectFn)
		} else if !hasBisect {
			fmt.Printf("BISECT %-20s  skip (no bisect function registered)\n", typeName)
		}

		proven = append(proven, ProvenEntry{
			Type:          typeName,
			Proven:        false,
			FixturePath:   fixturePath,
			VerifiedAt:    now,
			ParentChain:   parentChains[typeName],
			FailureReason: reason,
			Suggestions:   suggestions,
		})
	}


	// ── ToC Key Bisection pass ───────────────────────────────────────────────────
	fmt.Printf("\n── ToC Key Bisection ──────────────────────────────────────\n")
	tocResults := runToCBisection(*harness)
	for i, entry := range proven {
		if !entry.Proven {
			continue
		}
		bisected := tocResults[entry.Type]
		// Union bisected keys with seeds from the proposed contract.
		// Seeds represent known-required keys that the WASM load test may not catch.
		proven[i].ToCRequiredKeys = tocUnion(entry.ToCRequiredKeys, bisected)
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

// defaultsToCandidates converts a required_defaults map to a sorted CandidateProp slice.
// Sorted by name for deterministic bisection ordering.
func defaultsToCandidates(defaults map[string]interface{}) []CandidateProp {
	keys := make([]string, 0, len(defaults))
	for k := range defaults {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]CandidateProp, 0, len(keys))
	for _, k := range keys {
		out = append(out, CandidateProp{Name: k, Value: defaults[k]})
	}
	return out
}
