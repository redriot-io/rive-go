package main

import (
	"fmt"
	"os"
	"time"
)

const bisectTimeout = 30 * time.Second

// CandidateProp is a property name+value pair that might fix a failing type when added.
type CandidateProp struct {
	Name  string
	Value interface{}
}

// BisectFunc builds a .riv with the given candidate properties applied on top of
// the broken base state (all required defaults missing / zeroed).
// Called with nil or empty candidates → pure broken state.
// Called with one or more candidates → those properties are applied back.
type BisectFunc func(candidates []CandidateProp) ([]byte, error)

// bisect runs property bisection for a failing type.
// candidates is the universe of properties to search (from required_defaults).
// fn is the BisectFunc that builds the broken .riv with a candidate subset applied.
// Returns Suggestions (properties whose addition causes WASM to pass).
func bisect(typeName, harness string, candidates []CandidateProp, fn BisectFunc) []Suggestion {
	if len(candidates) == 0 {
		fmt.Printf("BISECT %-20s  no candidates — skip\n", typeName)
		return nil
	}
	fmt.Printf("BISECT %-20s  isolating from %d candidate(s)...\n", typeName, len(candidates))
	deadline := time.Now().Add(bisectTimeout)
	return bisectRecurse(typeName, harness, candidates, fn, deadline)
}

func bisectRecurse(typeName, harness string, candidates []CandidateProp, fn BisectFunc, deadline time.Time) []Suggestion {
	if time.Now().After(deadline) {
		fmt.Printf("BISECT %-20s  timeout → sequential fallback\n", typeName)
		return bisectSequential(typeName, harness, candidates, fn)
	}
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 {
		if testWith(harness, candidates, fn) {
			fmt.Printf("SUGGEST %-20s  add property=%s value=%v\n", typeName, candidates[0].Name, candidates[0].Value)
			return []Suggestion{{
				Property:  candidates[0].Name,
				Value:     candidates[0].Value,
				Rationale: "bisection: adding this property caused PASS",
			}}
		}
		return nil
	}

	mid := len(candidates) / 2
	left := candidates[:mid]
	right := candidates[mid:]

	// Test left half
	if testWith(harness, left, fn) {
		fmt.Printf("BISECT %-20s  left half passes → narrowing (n=%d)\n", typeName, len(left))
		return bisectRecurse(typeName, harness, left, fn, deadline)
	}

	// Test right half
	if testWith(harness, right, fn) {
		fmt.Printf("BISECT %-20s  right half passes → narrowing (n=%d)\n", typeName, len(right))
		return bisectRecurse(typeName, harness, right, fn, deadline)
	}

	// Neither half alone works — check if all candidates together pass
	if testWith(harness, candidates, fn) {
		// Multiple required properties — find each independently via sequential search
		fmt.Printf("BISECT %-20s  multiple props required → sequential (n=%d)\n", typeName, len(candidates))
		return bisectSequential(typeName, harness, candidates, fn)
	}

	// Candidates don't fix the failure at all
	fmt.Printf("BISECT %-20s  candidates do not fix the failure\n", typeName)
	return nil
}

// bisectSequential tests each candidate individually. O(n).
// Used when multiple properties are required, or on timeout fallback.
func bisectSequential(typeName, harness string, candidates []CandidateProp, fn BisectFunc) []Suggestion {
	var out []Suggestion
	for _, c := range candidates {
		if testWith(harness, []CandidateProp{c}, fn) {
			fmt.Printf("SUGGEST %-20s  add property=%s value=%v (sequential)\n", typeName, c.Name, c.Value)
			out = append(out, Suggestion{
				Property:  c.Name,
				Value:     c.Value,
				Rationale: "sequential search: adding this property caused PASS",
			})
		}
	}
	return out
}

// testWith builds a .riv with the given candidates and runs the WASM harness.
// Returns true if the harness exits with code 0.
func testWith(harness string, candidates []CandidateProp, fn BisectFunc) bool {
	data, err := fn(candidates)
	if err != nil {
		return false
	}
	path, err := writeTempRiv(data)
	if err != nil {
		return false
	}
	defer os.Remove(path)
	code, _, _ := runHarness(harness, path)
	return code == 0
}

// writeTempRiv writes bytes to a temp file and returns its path.
func writeTempRiv(data []byte) (string, error) {
	f, err := os.CreateTemp("", "prover-bisect-*.riv")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return "", err
	}
	return f.Name(), nil
}
