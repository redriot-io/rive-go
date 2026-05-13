package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"time"
)

// runToCBisection runs ToC key bisection for all proven types.
// For each type, it tests which ToC keys in the baseline .riv are REQUIRED —
// i.e., removing that key from the ToC causes WASM to fail.
// Returns a map from typeName → sorted list of required key ints.
func runToCBisection(harness string) map[string][]int {
	results := make(map[string][]int)
	for _, typeName := range typeOrder {
		buildFn, ok := buildFuncs[typeName]
		if !ok {
			continue
		}
		required, err := tocBisectType(typeName, harness, buildFn)
		if err != nil {
			fmt.Printf("TOC_BISECT %-20s  ERROR: %v\n", typeName, err)
			results[typeName] = []int{}
			continue
		}
		results[typeName] = required
	}
	return results
}

// tocBisectType discovers which ToC keys in the baseline .riv are required.
// Returns a sorted list of required key ints (may be empty/nil).
func tocBisectType(typeName, harness string, buildFn func() ([]byte, error)) ([]int, error) {
	baseline, err := buildFn()
	if err != nil {
		return nil, fmt.Errorf("build baseline: %w", err)
	}

	tocKeys, err := parseToCKeys(baseline)
	if err != nil {
		return nil, fmt.Errorf("parse ToC: %w", err)
	}

	if len(tocKeys) == 0 {
		fmt.Printf("TOC_BISECT %-20s  no ToC keys\n", typeName)
		return []int{}, nil
	}

	fmt.Printf("TOC_BISECT %-20s  testing %d key(s): %v\n", typeName, len(tocKeys), tocKeys)

	// Verify baseline passes before bisecting.
	baselinePath, err := writeTempRiv(baseline)
	if err != nil {
		return nil, fmt.Errorf("write baseline temp: %w", err)
	}
	defer os.Remove(baselinePath)

	code, _, _ := runHarness(harness, baselinePath)
	if code != 0 {
		fmt.Printf("TOC_BISECT %-20s  WARNING: baseline fails (exit=%d) — skipping\n", typeName, code)
		return []int{}, nil
	}

	// Test each key: remove it from the ToC and see if WASM fails.
	var required []int
	for _, key := range tocKeys {
		patched, err := rebuildWithoutToCKey(baseline, key)
		if err != nil {
			fmt.Printf("TOC_BISECT %-20s  key=%d: patch error: %v\n", typeName, key, err)
			continue
		}
		path, err := writeTempRiv(patched)
		if err != nil {
			fmt.Printf("TOC_BISECT %-20s  key=%d: write temp error: %v\n", typeName, key, err)
			continue
		}
		exitCode, _, _ := runHarness(harness, path)
		os.Remove(path)

		if exitCode != 0 {
			fmt.Printf("TOC_REQUIRED %-20s  key=%d (removing causes exit=%d)\n", typeName, key, exitCode)
			required = append(required, int(key))
		} else {
			fmt.Printf("TOC_OPTIONAL %-20s  key=%d\n", typeName, key)
		}
	}

	sort.Ints(required)
	return required, nil
}

// parseToCKeys reads the property keys listed in the ToC section of a .riv file.
// Format after "RIVE" + 3 varints (major, minor, fileID):
//
//	varuint(key1), varuint(key2), ..., varuint(0)  ← 0 is the terminator
func parseToCKeys(data []byte) ([]uint32, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("data too short (%d bytes)", len(data))
	}
	pos := 4 // skip "RIVE" fingerprint
	// Skip major, minor, fileID varints.
	for i := 0; i < 3; i++ {
		_, pos = tocDecodeVarUint(data, pos)
	}
	var keys []uint32
	for pos < len(data) {
		k, newPos := tocDecodeVarUint(data, pos)
		pos = newPos
		if k == 0 {
			break
		}
		keys = append(keys, uint32(k))
	}
	return keys, nil
}

// rebuildWithoutToCKey returns a copy of the .riv bytes with excludeKey removed from the ToC.
// The object stream is left untouched; only the ToC header section is patched.
func rebuildWithoutToCKey(data []byte, excludeKey uint32) ([]byte, error) {
	return rebuildWithoutToCKeys(data, map[uint32]bool{excludeKey: true})
}

// rebuildWithoutToCKeys returns a copy of the .riv bytes with all excludeKeys removed from ToC.
//
// .riv binary layout (after fingerprint):
//
//	varuint(major) varuint(minor) varuint(fileID)
//	[varuint(key) ...] varuint(0)          ← ToC key list
//	[uint32 ...]                            ← type-bit words (1 per 4 keys)
//	[object stream]
func rebuildWithoutToCKeys(data []byte, excludeKeys map[uint32]bool) ([]byte, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("data too short (%d bytes)", len(data))
	}
	var out bytes.Buffer

	// Copy "RIVE" fingerprint (4 bytes).
	out.Write(data[:4])
	pos := 4

	// Copy major, minor, fileID varints verbatim.
	for i := 0; i < 3; i++ {
		start := pos
		for pos < len(data) && data[pos]&0x80 != 0 {
			pos++
		}
		pos++ // include final byte of this varint
		out.Write(data[start:pos])
	}

	// Parse the existing ToC key list.
	type tocEntry struct {
		key      uint32
		propType uint8
	}
	var allEntries []tocEntry
	for pos < len(data) {
		k, newPos := tocDecodeVarUint(data, pos)
		pos = newPos
		if k == 0 {
			break
		}
		allEntries = append(allEntries, tocEntry{key: uint32(k)})
	}

	// Decode type bits that follow the key list.
	oldCount := len(allEntries)
	oldWordCount := (oldCount + 3) / 4
	typeBitsStart := pos
	if typeBitsStart+oldWordCount*4 > len(data) {
		return nil, fmt.Errorf("type bits extend past end of data (need %d bytes at %d, have %d)",
			oldWordCount*4, typeBitsStart, len(data)-typeBitsStart)
	}
	for i := range allEntries {
		wordIdx := i / 4
		bitPos := uint(i%4) * 2
		off := typeBitsStart + wordIdx*4
		word := uint32(data[off]) |
			uint32(data[off+1])<<8 |
			uint32(data[off+2])<<16 |
			uint32(data[off+3])<<24
		allEntries[i].propType = uint8((word >> bitPos) & 3)
	}
	typeBitsEnd := typeBitsStart + oldWordCount*4

	// Build filtered entry list (excluding requested keys).
	var filtered []tocEntry
	for _, e := range allEntries {
		if !excludeKeys[e.key] {
			filtered = append(filtered, e)
		}
	}

	// Write new ToC key list.
	for _, e := range filtered {
		tocEncodeVarUint(&out, uint64(e.key))
	}
	tocEncodeVarUint(&out, 0) // terminator

	// Write new type-bit words.
	newWordCount := (len(filtered) + 3) / 4
	newWords := make([]uint32, newWordCount)
	for i, e := range filtered {
		wordIdx := i / 4
		bitPos := uint(i%4) * 2
		newWords[wordIdx] |= uint32(e.propType) << bitPos
	}
	for _, w := range newWords {
		out.WriteByte(byte(w))
		out.WriteByte(byte(w >> 8))
		out.WriteByte(byte(w >> 16))
		out.WriteByte(byte(w >> 24))
	}

	// Append the object stream unchanged.
	out.Write(data[typeBitsEnd:])

	return out.Bytes(), nil
}

// tocDecodeVarUint reads a LEB128 unsigned varint from data[pos].
func tocDecodeVarUint(data []byte, pos int) (uint64, int) {
	var result uint64
	var shift uint
	for pos < len(data) {
		b := data[pos]
		pos++
		result |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
	}
	return result, pos
}

// tocEncodeVarUint appends a LEB128 unsigned varint to buf.
func tocEncodeVarUint(buf *bytes.Buffer, v uint64) {
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		buf.WriteByte(b)
		if v == 0 {
			break
		}
	}
}

// tocUnion returns the sorted union of two int slices (no duplicates).
func tocUnion(a, b []int) []int {
	seen := make(map[int]bool, len(a)+len(b))
	for _, v := range a {
		seen[v] = true
	}
	for _, v := range b {
		seen[v] = true
	}
	if len(seen) == 0 {
		return []int{}
	}
	out := make([]int, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}

// tocCoerce converts nil to an empty slice so JSON emits [] instead of null.
func tocCoerce(s []int) []int {
	if s == nil {
		return []int{}
	}
	return s
}

// ── ToC Ordering Detection ────────────────────────────────────────────────────

// OrderingConstraint records a must-precede relationship between two ToC keys.
// "Before" key must appear earlier in the ToC than "After" key.
type OrderingConstraint struct {
	Before int `json:"before"`
	After  int `json:"after"`
}

// runToCOrderDetection detects ToC key ordering constraints for proven types.
// Only runs for types that have ≥2 required ToC keys (single-key types have no ordering).
// Returns a map from typeName → slice of ordering constraints (may be empty).
func runToCOrderDetection(harness string, requiredMap map[string][]int, buildMap map[string]func() ([]byte, error)) map[string][]OrderingConstraint {
	results := make(map[string][]OrderingConstraint)
	for _, typeName := range typeOrder {
		required := requiredMap[typeName]
		if len(required) < 2 {
			fmt.Printf("TOC_ORDER  %-20s  skip (<%d required keys)\n", typeName, 2)
			results[typeName] = []OrderingConstraint{}
			continue
		}
		buildFn, ok := buildMap[typeName]
		if !ok {
			results[typeName] = []OrderingConstraint{}
			continue
		}

		keys := make([]uint32, len(required))
		for i, k := range required {
			keys[i] = uint32(k)
		}

		constraints, err := tocOrderDetectType(typeName, harness, buildFn, keys)
		if err != nil {
			fmt.Printf("TOC_ORDER  %-20s  ERROR: %v\n", typeName, err)
			results[typeName] = []OrderingConstraint{}
			continue
		}
		results[typeName] = constraints
	}
	return results
}

// tocOrderDetectType tests ordering constraints among the given required ToC keys.
// For ≤8 keys: tests all n! permutations.
// For >8 keys: tests 50 random permutations then infers must-precede pairs.
// Per-type timeout: 60 seconds.
func tocOrderDetectType(typeName, harness string, buildFn func() ([]byte, error), requiredKeys []uint32) ([]OrderingConstraint, error) {
	baseline, err := buildFn()
	if err != nil {
		return nil, fmt.Errorf("build baseline: %w", err)
	}

	n := len(requiredKeys)
	deadline := time.Now().Add(60 * time.Second)

	var perms [][]int
	if n <= 8 {
		perms = allPermutations(n)
		fmt.Printf("TOC_ORDER  %-20s  testing %d! = %d permutations of keys %v\n",
			typeName, n, len(perms), requiredKeys)
	} else {
		perms = randomSamplePermutations(n, 50)
		fmt.Printf("TOC_ORDER  %-20s  >8 keys — sampling 50 random permutations\n", typeName)
	}

	var passingPerms, failingPerms [][]int
	timedOut := false

	for _, perm := range perms {
		if time.Now().After(deadline) {
			fmt.Printf("TOC_ORDER  %-20s  WARNING: 60s timeout — stopping early\n", typeName)
			timedOut = true
			break
		}

		// Build the permuted key list (required keys in perm order; preserve non-required).
		permKeys := applyPermToToCKeys(baseline, requiredKeys, perm)
		patched, err := rebuildWithToCKeyList(baseline, permKeys)
		if err != nil {
			continue
		}
		path, err := writeTempRiv(patched)
		if err != nil {
			continue
		}
		code, _, _ := runHarness(harness, path)
		os.Remove(path)

		if code == 0 {
			passingPerms = append(passingPerms, perm)
		} else {
			failingPerms = append(failingPerms, perm)
		}
	}

	if len(failingPerms) == 0 {
		msg := "all orderings pass — no constraints"
		if timedOut {
			msg = "partial sample: all tested orderings pass"
		}
		fmt.Printf("TOC_ORDER  %-20s  %s\n", typeName, msg)
		return []OrderingConstraint{}, nil
	}

	constraints := inferOrderConstraints(requiredKeys, passingPerms, failingPerms)
	for _, c := range constraints {
		fmt.Printf("TOC_ORDER  %-20s  constraint: key %d must precede key %d\n",
			typeName, c.Before, c.After)
	}
	if len(constraints) == 0 {
		fmt.Printf("TOC_ORDER  %-20s  %d failing perms but no consistent must-precede pairs\n",
			typeName, len(failingPerms))
	}
	return constraints, nil
}

// applyPermToToCKeys returns the ToC key list from the baseline .riv with
// the required keys replaced by perm[i]→requiredKeys[perm[i]] in order.
// Non-required keys keep their original relative positions.
func applyPermToToCKeys(baseline []byte, requiredKeys []uint32, perm []int) []uint32 {
	origKeys, _ := parseToCKeys(baseline)
	requiredSet := make(map[uint32]bool, len(requiredKeys))
	for _, k := range requiredKeys {
		requiredSet[k] = true
	}

	// Collect positions of required keys in the original order.
	var reqPositions []int
	for i, k := range origKeys {
		if requiredSet[k] {
			reqPositions = append(reqPositions, i)
		}
	}

	result := make([]uint32, len(origKeys))
	copy(result, origKeys)

	// Replace required-key slots with the permuted values.
	for slotIdx, origPos := range reqPositions {
		result[origPos] = requiredKeys[perm[slotIdx]]
	}
	return result
}

// rebuildWithToCKeyList rebuilds .riv with the ToC key list replaced by newKeys.
// propTypes for each key are taken from the original ToC.
// Keys not in the original ToC default to PropertyType uint (0).
func rebuildWithToCKeyList(data []byte, newKeys []uint32) ([]byte, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("data too short")
	}

	// Parse original ToC to get propType per key.
	type origEntry struct {
		key      uint32
		propType uint8
	}
	var origEntries []origEntry

	pos := 4
	for i := 0; i < 3; i++ {
		_, pos = tocDecodeVarUint(data, pos)
	}
	headerEnd := pos // start of header varints (major/minor/fileID already skipped above)
	_ = headerEnd

	// Re-parse from the start including header bytes.
	pos = 4
	for i := 0; i < 3; i++ {
		_, pos = tocDecodeVarUint(data, pos)
	}
	tocStart := pos
	for pos < len(data) {
		k, newPos := tocDecodeVarUint(data, pos)
		pos = newPos
		if k == 0 {
			break
		}
		origEntries = append(origEntries, origEntry{key: uint32(k)})
	}

	oldWordCount := (len(origEntries) + 3) / 4
	typeBitsStart := pos
	if typeBitsStart+oldWordCount*4 > len(data) {
		return nil, fmt.Errorf("type bits extend past end of data")
	}
	for i := range origEntries {
		wi := i / 4
		bp := uint(i%4) * 2
		off := typeBitsStart + wi*4
		w := uint32(data[off]) | uint32(data[off+1])<<8 |
			uint32(data[off+2])<<16 | uint32(data[off+3])<<24
		origEntries[i].propType = uint8((w >> bp) & 3)
	}
	typeBitsEnd := typeBitsStart + oldWordCount*4

	// Build key→propType map from original.
	keyToType := make(map[uint32]uint8, len(origEntries))
	for _, e := range origEntries {
		keyToType[e.key] = e.propType
	}

	// Write rebuilt bytes.
	var out bytes.Buffer
	// Fingerprint + header varints verbatim.
	headerPos := 4
	for i := 0; i < 3; i++ {
		start := headerPos
		for headerPos < len(data) && data[headerPos]&0x80 != 0 {
			headerPos++
		}
		headerPos++
		_ = start // we write the whole header block below
	}
	out.Write(data[:tocStart])

	// New ToC keys.
	for _, k := range newKeys {
		tocEncodeVarUint(&out, uint64(k))
	}
	tocEncodeVarUint(&out, 0)

	// New type-bit words.
	newWordCount := (len(newKeys) + 3) / 4
	newWords := make([]uint32, newWordCount)
	for i, k := range newKeys {
		pt := keyToType[k] // 0 (uint) if unknown
		newWords[i/4] |= uint32(pt) << (uint(i%4) * 2)
	}
	for _, w := range newWords {
		out.WriteByte(byte(w))
		out.WriteByte(byte(w >> 8))
		out.WriteByte(byte(w >> 16))
		out.WriteByte(byte(w >> 24))
	}

	// Object stream unchanged.
	out.Write(data[typeBitsEnd:])
	return out.Bytes(), nil
}

// inferOrderConstraints analyzes pass/fail permutation results to find must-precede pairs.
// A pair (a, b) is a constraint if ALL passing perms have a before b AND
// at least one failing perm has b before a (i.e., swapping causes failure).
func inferOrderConstraints(keys []uint32, passingPerms, failingPerms [][]int) []OrderingConstraint {
	if len(failingPerms) == 0 {
		return nil
	}
	n := len(keys)
	var constraints []OrderingConstraint

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			// Does every passing perm have keys[i] before keys[j]?
			allPassHaveIBeforeJ := true
			for _, perm := range passingPerms {
				posI, posJ := indexIn(perm, i), indexIn(perm, j)
				if posI > posJ {
					allPassHaveIBeforeJ = false
					break
				}
			}
			if !allPassHaveIBeforeJ {
				continue
			}
			// Does any failing perm have keys[j] before keys[i]?
			someFailHasJBeforeI := false
			for _, perm := range failingPerms {
				if indexIn(perm, j) < indexIn(perm, i) {
					someFailHasJBeforeI = true
					break
				}
			}
			if someFailHasJBeforeI {
				constraints = append(constraints, OrderingConstraint{
					Before: int(keys[i]),
					After:  int(keys[j]),
				})
			}
		}
	}
	return constraints
}

func indexIn(perm []int, v int) int {
	for i, x := range perm {
		if x == v {
			return i
		}
	}
	return -1
}

// allPermutations returns all permutations of [0..n-1] using Heap's algorithm.
func allPermutations(n int) [][]int {
	arr := make([]int, n)
	for i := range arr {
		arr[i] = i
	}
	var result [][]int
	var generate func(k int)
	generate = func(k int) {
		if k == 1 {
			p := make([]int, n)
			copy(p, arr)
			result = append(result, p)
			return
		}
		for i := 0; i < k; i++ {
			generate(k - 1)
			if k%2 == 0 {
				arr[i], arr[k-1] = arr[k-1], arr[i]
			} else {
				arr[0], arr[k-1] = arr[k-1], arr[0]
			}
		}
	}
	generate(n)
	return result
}

// randomSamplePermutations generates count random permutations of [0..n-1].
func randomSamplePermutations(n, count int) [][]int {
	src := randSource()
	result := make([][]int, count)
	for i := range result {
		p := make([]int, n)
		for j := range p {
			p[j] = j
		}
		// Fisher-Yates shuffle using our simple LCG source.
		for j := n - 1; j > 0; j-- {
			k := src.Intn(j + 1)
			p[j], p[k] = p[k], p[j]
		}
		result[i] = p
	}
	return result
}

// generateImageOrderingFixture tests all permutations of Image's full ToC key list
// and saves a wrong-order fixture if any permutation fails.
func generateImageOrderingFixture(harness, fixtureDir string) {
	fmt.Printf("\n── Image ToC Ordering Regression ──────────────────────────────────────────\n")

	baseline, err := buildImage()
	if err != nil {
		fmt.Printf("IMAGE_ORDER  build error: %v\n", err)
		return
	}

	tocKeys, err := parseToCKeys(baseline)
	if err != nil || len(tocKeys) < 2 {
		fmt.Printf("IMAGE_ORDER  no ToC keys to permute\n")
		return
	}

	fmt.Printf("IMAGE_ORDER  baseline ToC: %v — testing all %d! = %d permutations\n",
		tocKeys, len(tocKeys), factorial(len(tocKeys)))

	perms := allPermutations(len(tocKeys))
	var wrongOrderBytes []byte
	var wrongOrderPerm []int

	for _, perm := range perms {
		permKeys := make([]uint32, len(tocKeys))
		for i, idx := range perm {
			permKeys[i] = tocKeys[idx]
		}
		patched, err := rebuildWithToCKeyList(baseline, permKeys)
		if err != nil {
			continue
		}
		path, err := writeTempRiv(patched)
		if err != nil {
			continue
		}
		code, _, _ := runHarness(harness, path)
		os.Remove(path)

		if code != 0 {
			fmt.Printf("IMAGE_ORDER  FAIL perm %v keys %v (exit=%d) — ordering matters!\n",
				perm, permKeys, code)
			if wrongOrderBytes == nil {
				wrongOrderBytes = patched
				wrongOrderPerm = perm
			}
		}
	}

	if wrongOrderBytes == nil {
		fmt.Printf("IMAGE_ORDER  all %d permutations PASS — ToC ordering is not sensitive for Image\n",
			len(perms))
		fmt.Printf("IMAGE_ORDER  regression fixture NOT generated (no ordering constraint found)\n")
		return
	}

	fixturePath := fixtureDir + "/Image_wrong_toc_order.riv"
	if err := os.WriteFile(fixturePath, wrongOrderBytes, 0o644); err != nil {
		fmt.Printf("IMAGE_ORDER  failed to write fixture: %v\n", err)
		return
	}
	fmt.Printf("IMAGE_ORDER  regression fixture saved: %s (perm %v)\n", fixturePath, wrongOrderPerm)
}

func factorial(n int) int {
	f := 1
	for i := 2; i <= n; i++ {
		f *= i
	}
	return f
}

// randSource returns a minimal pseudo-random source (LCG) seeded from time.
// Avoids importing math/rand to keep dependencies minimal.
type lcgRand struct{ state uint64 }

func randSource() *lcgRand {
	return &lcgRand{state: uint64(time.Now().UnixNano())}
}

func (r *lcgRand) Intn(n int) int {
	// LCG constants from Numerical Recipes.
	r.state = r.state*6364136223846793005 + 1442695040888963407
	return int((r.state >> 33) % uint64(n))
}
