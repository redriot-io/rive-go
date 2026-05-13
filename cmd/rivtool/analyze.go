package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redriot-io/rive-go/rive"
)

// execCommand is a thin wrapper around exec.Command for testability.
var execCommand = exec.Command

// ── Output schema ─────────────────────────────────────────────────────────────

type FormatContract struct {
	GeneratedAt   string                  `json:"generated_at"`
	Sources       ContractSources         `json:"sources"`
	EmissionOrder EmissionOrderSection    `json:"emission_order"`
	TocRules      TocRulesSection         `json:"toc_rules"`
	Defaults      map[string]DefaultEntry `json:"defaults,omitempty"`
	ParentIdRules map[string]ParentIdRule `json:"parent_id_rules"`
	TypeRegistry  map[string]string       `json:"type_registry"`
}

type ContractSources struct {
	AssetFiles []string `json:"asset_files"`
	DefsDir    string   `json:"defs_dir,omitempty"`
}

type EmissionOrderSection struct {
	GlobalPrefix      []uint32                  `json:"global_prefix"`
	ArtboardPhase     []uint32                  `json:"artboard_phase"`
	TypePhasePriority map[string]TypePhaseEntry `json:"type_phase_priority"`
	Notes             []string                  `json:"notes,omitempty"`
}

type TypePhaseEntry struct {
	Phase    string `json:"phase"`
	Priority int    `json:"priority"`
}

type TocRulesSection struct {
	IncludeKeys    map[string]TocKeyEntry `json:"include_keys"`
	BytesProxyKeys []uint32               `json:"bytes_proxy_keys"`
}

type TocKeyEntry struct {
	FieldIndex int      `json:"field_index"`
	SeenIn     []string `json:"seen_in"`
	Note       string   `json:"note,omitempty"`
}

type DefaultEntry struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	RiveDefault interface{} `json:"rive_default"`
	GoZero      interface{} `json:"go_zero"`
	Mismatch    bool        `json:"mismatch"`
}

type ParentIdRule struct {
	HasParentId bool   `json:"has_parent_id"`
	RelativeTo  string `json:"relative_to,omitempty"`
}

// ── dev/defs parsing types ────────────────────────────────────────────────────

type defFile struct {
	Name       string                 `json:"name"`
	Key        *defKeyObj             `json:"key"`
	Properties map[string]defProperty `json:"properties"`
}

type defKeyObj struct {
	Int int `json:"int"`
}

type defProperty struct {
	Type                string     `json:"type"`
	TypeRuntime         string     `json:"typeRuntime"`
	InitialValue        string     `json:"initialValue"`
	InitialValueRuntime string     `json:"initialValueRuntime"`
	KeyData             defPropKey `json:"key"`
	Runtime             *bool      `json:"runtime"` // nil = runtime property; false = editor-only
}

type defPropKey struct {
	Int int `json:"int"`
}

// ── File observation ──────────────────────────────────────────────────────────

type rivObs struct {
	name    string
	objects []rive.Object
	toc     map[uint32]rive.PropertyType
}

// ── Well-known phase classification ──────────────────────────────────────────
// Used to give semantic names to post-artboard typeKeys.

var (
	paintTypeKeys = map[uint32]bool{
		18: true, // SolidColor
		20: true, // Fill
		21: true, // LinearGradient
		22: true, // RadialGradient
		81: true, 82: true, 83: true,
	}
	animationTypeKeys = map[uint32]bool{
		31: true, // LinearAnimation
		95: true, 96: true, 97: true, 113: true, 114: true, 115: true, 116: true,
	}
	stateMachineTypeKeys = map[uint32]bool{
		53: true, // StateMachine
		57: true, // StateMachineLayer
		61: true, 62: true, 63: true, 64: true, 65: true,
		66: true, 67: true, 68: true, 69: true, 70: true,
		71: true, 72: true, 73: true, 74: true,
		168: true, 169: true, 170: true,
	}
	// bytesTypeKeys are property keys known to have bytes wire encoding.
	// These appear in the ToC with field_index=1 (proxied as string) because
	// PropertyTypeBytes=4 cannot be represented in the 2-bit ToC.
	bytesTypeKeys = map[uint32]bool{
		212: true, // FileAssetContents.bytes
		223: true, // Mesh.triangleIndexBytes
		359: true, // FileAsset.cdnUuid
		588: true, // DataBind.sourcePathIds
		890: true, // Backboard.hydrogenPanes
		911: true, // FileAssetContents.signature
		920: true, // DataBind.path
		963: true, // ViewModelInstanceListItem.viewModelPathIds
	}
)

// ── Main command ──────────────────────────────────────────────────────────────

func cmdAnalyze(args []string) {
	assetsDir := ""
	defsDir := ""
	outputPath := "format_contract.json"
	prove := false
	proposedPath := "format_contract_proposed.json"
	provenPath := "format_contract_proven.json"
	harnessPath := "tools/wasm-harness/validate.js"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--assets":
			if i+1 < len(args) {
				i++
				assetsDir = args[i]
			}
		case "--defs":
			if i+1 < len(args) {
				i++
				defsDir = args[i]
			}
		case "-o", "--output":
			if i+1 < len(args) {
				i++
				outputPath = args[i]
			}
		case "--prove":
			prove = true
		case "--proposed":
			if i+1 < len(args) {
				i++
				proposedPath = args[i]
			}
		case "--proven":
			if i+1 < len(args) {
				i++
				provenPath = args[i]
			}
		case "--harness":
			if i+1 < len(args) {
				i++
				harnessPath = args[i]
			}
		case "--help", "-h":
			fmt.Println(`rivtool analyze — extract format_contract.json from .riv assets

Usage:
  rivtool analyze --assets <dir> [--defs <dir>] [-o <file.json>]
  rivtool analyze --assets <dir> --prove [--proposed <file>] [--proven <file>] [--harness <file>]

Flags:
  --assets <dir>       Directory of .riv files to analyze (required)
  --defs   <dir>       Path to dev/defs JSON schema directory (optional)
  -o       <file>      Output path for format_contract.json (default: format_contract.json)
  --prove              After static analysis, run Contract Prover to validate all types
                       via the Rive WASM runtime and emit format_contract_proven.json
  --proposed <file>    Proposed contract input for --prove (default: format_contract_proposed.json)
  --proven   <file>    Proven contract output for --prove (default: format_contract_proven.json)
  --harness  <file>    WASM validate.js path for --prove (default: tools/wasm-harness/validate.js)`)
			return
		}
	}

	if assetsDir == "" {
		fmt.Fprintln(os.Stderr, "analyze: --assets <dir> is required")
		fmt.Fprintln(os.Stderr, "Run 'rivtool analyze --help' for usage.")
		os.Exit(1)
	}

	contract, err := buildContract(assetsDir, defsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "analyze: %v\n", err)
		os.Exit(1)
	}

	data, err := json.MarshalIndent(contract, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "analyze: marshal: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "analyze: write %s: %v\n", outputPath, err)
		os.Exit(1)
	}
	fmt.Printf("✓ wrote %s (%d bytes, %d types, %d defaults)\n",
		outputPath, len(data), len(contract.TypeRegistry), len(contract.Defaults))

	if prove {
		runContractProver(proposedPath, provenPath, harnessPath)
	}
}

// runContractProver invokes cmd/contract-prover via go run.
// It streams stdout/stderr so the user sees progress in real time.
func runContractProver(proposedPath, provenPath, harnessPath string) {
	fmt.Println("\n── Contract Prover ──────────────────────────────────────────")
	fmt.Printf("  proposed: %s\n  proven:   %s\n  harness:  %s\n\n", proposedPath, provenPath, harnessPath)

	cmd := execCommand("go", "run", "./cmd/contract-prover/",
		"--proposed", proposedPath,
		"--out", provenPath,
		"--harness", harnessPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "analyze --prove: prover failed: %v\n", err)
		os.Exit(1)
	}
}

// ── Contract builder ──────────────────────────────────────────────────────────

func buildContract(assetsDir, defsDir string) (*FormatContract, error) {
	// 1. Collect .riv files
	rivFiles, err := collectRivFiles(assetsDir)
	if err != nil {
		return nil, fmt.Errorf("assets: %w", err)
	}
	if len(rivFiles) == 0 {
		return nil, fmt.Errorf("no .riv files found in %s", assetsDir)
	}

	// 2. Parse each file
	var obs []rivObs
	for _, path := range rivFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		f, err := rive.ReadBytes(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		obs = append(obs, rivObs{
			name:    filepath.Base(path),
			objects: f.Objects,
			toc:     f.TocEntries(),
		})
	}

	// 3. Derive each section
	emOrder, notes := deriveEmissionOrder(obs)
	tocRules := deriveTocRules(obs)
	parentIdRules := deriveParentIdRules(obs)

	var defaults map[string]DefaultEntry
	var typeReg map[string]string
	if defsDir != "" {
		defs, err := parseAllDefs(defsDir)
		if err != nil {
			return nil, fmt.Errorf("defs: %w", err)
		}
		defaults = buildDefaults(defs)
		typeReg = buildTypeRegistry(defs)
	} else {
		typeReg = buildTypeRegistryFromObs(obs)
	}

	// 4. Assemble
	assetNames := make([]string, len(obs))
	for i, o := range obs {
		assetNames[i] = o.name
	}

	return &FormatContract{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Sources: ContractSources{
			AssetFiles: assetNames,
			DefsDir:    defsDir,
		},
		EmissionOrder: EmissionOrderSection{
			GlobalPrefix:      emOrder.globalPrefix,
			ArtboardPhase:     []uint32{1},
			TypePhasePriority: emOrder.phasePriority,
			Notes:             notes,
		},
		TocRules:      tocRules,
		Defaults:      defaults,
		ParentIdRules: parentIdRules,
		TypeRegistry:  typeReg,
	}, nil
}

// ── .riv file collection ──────────────────────────────────────────────────────

func collectRivFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".riv") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

// ── Emission order ────────────────────────────────────────────────────────────

type emOrderResult struct {
	globalPrefix  []uint32
	phasePriority map[string]TypePhaseEntry
}

func deriveEmissionOrder(obs []rivObs) (emOrderResult, []string) {
	typeBeforeArtboard := map[uint32]bool{}
	typeAfterArtboard := map[uint32]bool{}
	typeFirstSeen := map[uint32]int{}
	counter := 0

	for _, o := range obs {
		// Find first Artboard(1) index in this file
		artIdx := -1
		for i, obj := range o.objects {
			if obj.TypeKey() == 1 {
				artIdx = i
				break
			}
		}

		for i, obj := range o.objects {
			tk := obj.TypeKey()
			if _, seen := typeFirstSeen[tk]; !seen {
				typeFirstSeen[tk] = counter
				counter++
			}
			if artIdx >= 0 {
				if i < artIdx {
					typeBeforeArtboard[tk] = true
				} else if i > artIdx {
					typeAfterArtboard[tk] = true
				}
			}
		}
	}

	// Global: appears before artboard and NEVER after
	globalOnly := map[uint32]bool{}
	for tk := range typeBeforeArtboard {
		if !typeAfterArtboard[tk] {
			globalOnly[tk] = true
		}
	}

	// Build sorted global prefix (by first-seen order)
	var globalPrefix []uint32
	for tk := range globalOnly {
		globalPrefix = append(globalPrefix, tk)
	}
	sort.Slice(globalPrefix, func(i, j int) bool {
		return typeFirstSeen[globalPrefix[i]] < typeFirstSeen[globalPrefix[j]]
	})

	// Collect all post-artboard typeKeys (sorted by first-seen)
	var postArtboard []uint32
	for tk := range typeAfterArtboard {
		if !globalOnly[tk] && tk != 1 {
			postArtboard = append(postArtboard, tk)
		}
	}
	sort.Slice(postArtboard, func(i, j int) bool {
		ai, aj := typeFirstSeen[postArtboard[i]], typeFirstSeen[postArtboard[j]]
		if ai != aj {
			return ai < aj
		}
		return postArtboard[i] < postArtboard[j]
	})

	// Assign phase + priority
	phasePriority := map[string]TypePhaseEntry{}
	for i, tk := range globalPrefix {
		phasePriority[uintKey(tk)] = TypePhaseEntry{Phase: "global", Priority: i}
	}
	phasePriority["1"] = TypePhaseEntry{Phase: "artboard", Priority: 0}

	phaseCounters := map[string]int{}
	for _, tk := range postArtboard {
		phase := classifyPhase(tk)
		phasePriority[uintKey(tk)] = TypePhaseEntry{Phase: phase, Priority: phaseCounters[phase]}
		phaseCounters[phase]++
	}

	notes := []string{
		"global_prefix: types observed before first Artboard(1) in all files where they appear",
		"paint ordering: SolidColor(18) appears before its parent Fill(20) in official files — forward-reference parentId pattern",
		"fontAssetId (key 279): 0-based index into file-level FontAsset list, not artboard-relative",
		"bytes in ToC proxied as field_index=1 (PropertyTypeString) — wire encoding identical, but ToC 2-bit field cannot represent PropertyTypeBytes=4",
	}

	return emOrderResult{globalPrefix: globalPrefix, phasePriority: phasePriority}, notes
}

func classifyPhase(tk uint32) string {
	switch {
	case stateMachineTypeKeys[tk]:
		return "state_machine"
	case animationTypeKeys[tk]:
		return "animation"
	case paintTypeKeys[tk]:
		return "paint"
	default:
		return "children"
	}
}

// ── ToC rules ─────────────────────────────────────────────────────────────────

func deriveTocRules(obs []rivObs) TocRulesSection {
	includeKeys := map[string]TocKeyEntry{}
	proxySet := map[uint32]bool{}

	for _, o := range obs {
		for k, fi := range o.toc {
			ks := uintKey(k)
			entry := includeKeys[ks]
			if entry.SeenIn == nil {
				entry.FieldIndex = int(fi)
			}
			// Append filename if not already present
			found := false
			for _, fn := range entry.SeenIn {
				if fn == o.name {
					found = true
					break
				}
			}
			if !found {
				entry.SeenIn = append(entry.SeenIn, o.name)
				sort.Strings(entry.SeenIn)
			}
			if fi == 1 && bytesTypeKeys[k] {
				entry.Note = "bytes_proxy"
				proxySet[k] = true
			}
			includeKeys[ks] = entry
		}
	}

	var proxyKeys []uint32
	for k := range proxySet {
		proxyKeys = append(proxyKeys, k)
	}
	sort.Slice(proxyKeys, func(i, j int) bool { return proxyKeys[i] < proxyKeys[j] })

	return TocRulesSection{IncludeKeys: includeKeys, BytesProxyKeys: proxyKeys}
}

// ── ParentId rules ────────────────────────────────────────────────────────────

func deriveParentIdRules(obs []rivObs) map[string]ParentIdRule {
	hasParent := map[uint32]bool{}
	seenType := map[uint32]bool{}

	for _, o := range obs {
		for _, obj := range o.objects {
			tk := obj.TypeKey()
			seenType[tk] = true
			for _, p := range obj.Properties() {
				if p.Key == 5 { // parentId
					hasParent[tk] = true
					break
				}
			}
		}
	}

	var typeKeys []uint32
	for tk := range seenType {
		typeKeys = append(typeKeys, tk)
	}
	sort.Slice(typeKeys, func(i, j int) bool { return typeKeys[i] < typeKeys[j] })

	rules := map[string]ParentIdRule{}
	for _, tk := range typeKeys {
		if hasParent[tk] {
			rules[uintKey(tk)] = ParentIdRule{HasParentId: true, RelativeTo: "artboard"}
		} else {
			rules[uintKey(tk)] = ParentIdRule{HasParentId: false}
		}
	}
	return rules
}

// ── dev/defs parsing ──────────────────────────────────────────────────────────

func parseAllDefs(defsDir string) ([]defFile, error) {
	var defs []defFile
	err := filepath.WalkDir(defsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		var df defFile
		if jsonErr := json.Unmarshal(data, &df); jsonErr != nil {
			return nil // skip malformed files
		}
		if df.Name != "" {
			defs = append(defs, df)
		}
		return nil
	})
	return defs, err
}

func buildTypeRegistry(defs []defFile) map[string]string {
	reg := map[string]string{}
	for _, df := range defs {
		if df.Key != nil && df.Key.Int > 0 {
			reg[strconv.Itoa(df.Key.Int)] = df.Name
		}
	}
	return reg
}

func buildTypeRegistryFromObs(obs []rivObs) map[string]string {
	reg := map[string]string{}
	for _, o := range obs {
		for _, obj := range o.objects {
			tk := obj.TypeKey()
			k := uintKey(tk)
			if _, exists := reg[k]; !exists {
				reg[k] = dumpTypeKeyName(tk)
			}
		}
	}
	return reg
}

// ── Defaults ──────────────────────────────────────────────────────────────────

func buildDefaults(defs []defFile) map[string]DefaultEntry {
	defaults := map[string]DefaultEntry{}

	for _, df := range defs {
		for propName, prop := range df.Properties {
			// Skip editor-only properties (runtime: false)
			if prop.Runtime != nil && !*prop.Runtime {
				continue
			}
			if prop.KeyData.Int <= 0 {
				continue
			}

			k := strconv.Itoa(prop.KeyData.Int)

			// Prefer runtime-specific type/value
			rawType := prop.TypeRuntime
			if rawType == "" {
				rawType = prop.Type
			}
			rawDefault := prop.InitialValueRuntime
			if rawDefault == "" {
				rawDefault = prop.InitialValue
			}

			normType := normalizeType(rawType)
			if normType == "" {
				continue
			}

			riveDefault, goZero, err := parseDefault(normType, rawDefault)
			if err != nil {
				continue
			}

			mismatch := !valuesEqual(riveDefault, goZero)

			if _, exists := defaults[k]; !exists {
				defaults[k] = DefaultEntry{
					Name:        propName,
					Type:        normType,
					RiveDefault: riveDefault,
					GoZero:      goZero,
					Mismatch:    mismatch,
				}
			}
		}
	}
	return defaults
}

// ── Type normalization ────────────────────────────────────────────────────────

func normalizeType(t string) string {
	switch strings.ToLower(t) {
	case "double", "float":
		return "float"
	case "uint":
		return "uint"
	case "string":
		return "string"
	case "color":
		return "color"
	case "bool":
		return "bool"
	case "bytes":
		return "bytes"
	case "id":
		return "uint"
	default:
		return "" // List<Id>, etc. — skip
	}
}

func parseDefault(normType, raw string) (riveDefault, goZero interface{}, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "0"
	}

	switch normType {
	case "float":
		v, e := strconv.ParseFloat(raw, 64)
		if e != nil {
			return nil, nil, e
		}
		return v, float64(0), nil

	case "uint":
		if raw == "Core.missingId" || raw == "-1" {
			return uint64(^uint64(0)), uint64(0), nil
		}
		if strings.HasPrefix(raw, "-") {
			iv, e := strconv.ParseInt(raw, 10, 64)
			if e != nil {
				return nil, nil, e
			}
			return uint64(iv), uint64(0), nil
		}
		v, e := strconv.ParseUint(raw, 10, 64)
		if e != nil {
			// Try as float (e.g. "1.0")
			fv, fe := strconv.ParseFloat(raw, 64)
			if fe != nil {
				return nil, nil, e
			}
			return uint64(fv), uint64(0), nil
		}
		return v, uint64(0), nil

	case "bool":
		v, e := strconv.ParseBool(raw)
		if e != nil {
			return nil, nil, e
		}
		return v, false, nil

	case "string":
		r := raw
		if len(r) >= 2 &&
			((r[0] == '\'' && r[len(r)-1] == '\'') || (r[0] == '"' && r[len(r)-1] == '"')) {
			r = r[1 : len(r)-1]
		}
		return r, "", nil

	case "color":
		r := raw
		if strings.HasPrefix(r, "0x") || strings.HasPrefix(r, "0X") {
			v, e := strconv.ParseUint(r[2:], 16, 64)
			if e != nil {
				return nil, nil, e
			}
			return v, uint64(0), nil
		}
		v, e := strconv.ParseUint(r, 10, 64)
		if e != nil {
			return nil, nil, e
		}
		return v, uint64(0), nil

	case "bytes":
		return []byte(nil), []byte(nil), nil
	}
	return nil, nil, fmt.Errorf("unhandled type %s", normType)
}

func valuesEqual(a, b interface{}) bool {
	switch av := a.(type) {
	case float64:
		if bv, ok := b.(float64); ok {
			return av == bv
		}
	case uint64:
		if bv, ok := b.(uint64); ok {
			return av == bv
		}
	case bool:
		if bv, ok := b.(bool); ok {
			return av == bv
		}
	case string:
		if bv, ok := b.(string); ok {
			return av == bv
		}
	}
	return false
}

// ── Utilities ─────────────────────────────────────────────────────────────────

func uintKey(v uint32) string {
	return strconv.FormatUint(uint64(v), 10)
}
