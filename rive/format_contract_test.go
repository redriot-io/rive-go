package rive_test

import (
	"encoding/json"
	"os"
	"testing"
)

// contractJSON mirrors just the fields we want to validate.
type contractJSON struct {
	EmissionOrder struct {
		GlobalPrefix []uint32 `json:"global_prefix"`
	} `json:"emission_order"`
	TocRules struct {
		IncludeKeys map[string]struct {
			FieldIndex int    `json:"field_index"`
			Note       string `json:"note"`
		} `json:"include_keys"`
		BytesProxyKeys []uint32 `json:"bytes_proxy_keys"`
	} `json:"toc_rules"`
	Defaults map[string]struct {
		Name        string      `json:"name"`
		Type        string      `json:"type"`
		RiveDefault interface{} `json:"rive_default"`
		Mismatch    bool        `json:"mismatch"`
	} `json:"defaults"`
	ParentIdRules map[string]struct {
		HasParentId bool `json:"has_parent_id"`
	} `json:"parent_id_rules"`
	TypeRegistry map[string]string `json:"type_registry"`
}

// TestAnalyze_ContractValid validates the generated format_contract.json
// against known ground-truth facts from the official Rive test assets.
// If format_contract.json does not exist, the test is skipped (it is only
// generated when running 'rivtool analyze').
func TestAnalyze_ContractValid(t *testing.T) {
	data, err := os.ReadFile("format_contract.json")
	if err != nil {
		t.Skip("format_contract.json not found — run: rivtool analyze --assets rive/testdata/official/ --defs <defs-dir> -o rive/format_contract.json")
	}

	var c contractJSON
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("unmarshal format_contract.json: %v", err)
	}

	// ── 1. key 212 in toc_rules.include_keys with field_index=1 ──────────────
	entry212, ok := c.TocRules.IncludeKeys["212"]
	if !ok {
		t.Error("toc_rules.include_keys: key 212 (FileAssetContents.bytes) is missing")
	} else {
		if entry212.FieldIndex != 1 {
			t.Errorf("toc_rules.include_keys[212].field_index = %d, want 1 (bytes proxied as string)", entry212.FieldIndex)
		}
		if entry212.Note != "bytes_proxy" {
			t.Errorf("toc_rules.include_keys[212].note = %q, want bytes_proxy", entry212.Note)
		}
	}

	// ── 2. key 212 in bytes_proxy_keys ───────────────────────────────────────
	has212Proxy := false
	for _, k := range c.TocRules.BytesProxyKeys {
		if k == 212 {
			has212Proxy = true
		}
	}
	if !has212Proxy {
		t.Error("toc_rules.bytes_proxy_keys: key 212 is missing")
	}

	// ── 3. opacity has rive_default=1.0 and mismatch=true ────────────────────
	// Property key 18 is opacity (from drawable/transform_component defs).
	opacityEntry, opacityFound := c.Defaults["18"]
	if !opacityFound {
		t.Error("defaults[18] (opacity) is missing — was dev/defs provided?")
	} else {
		if !opacityEntry.Mismatch {
			t.Errorf("defaults[18].mismatch = false, want true (rive default=1.0, Go zero=0.0)")
		}
		if rv, ok := opacityEntry.RiveDefault.(float64); !ok || rv != 1.0 {
			t.Errorf("defaults[18].rive_default = %v, want 1.0", opacityEntry.RiveDefault)
		}
	}

	// ── 4. Backboard (typeKey 23) has_parent_id=false ─────────────────────────
	bb, ok := c.ParentIdRules["23"]
	if !ok {
		t.Error("parent_id_rules[23] (Backboard) is missing")
	} else if bb.HasParentId {
		t.Error("parent_id_rules[23].has_parent_id = true, want false (Backboard has no parent)")
	}

	// ── 5. FontAsset (typeKey 141) has_parent_id=false ────────────────────────
	fa, ok := c.ParentIdRules["141"]
	if !ok {
		t.Error("parent_id_rules[141] (FontAsset) is missing")
	} else if fa.HasParentId {
		t.Error("parent_id_rules[141].has_parent_id = true, want false (FontAsset is a global asset, no parent)")
	}

	// ── 6. global_prefix starts with 23 (Backboard) ──────────────────────────
	gp := c.EmissionOrder.GlobalPrefix
	if len(gp) == 0 {
		t.Error("emission_order.global_prefix is empty")
	} else if gp[0] != 23 {
		t.Errorf("emission_order.global_prefix[0] = %d, want 23 (Backboard)", gp[0])
	}

	// ── 7. type_registry sanity checks ───────────────────────────────────────
	type typeCheck struct {
		key  string
		want string
	}
	checks := []typeCheck{
		{"1", "Artboard"},
		{"23", "Backboard"},
		{"134", "Text"},
		{"137", "TextStylePaint"},
		{"141", "FontAsset"},
	}
	for _, tc := range checks {
		got, ok := c.TypeRegistry[tc.key]
		if !ok {
			t.Errorf("type_registry[%s]: missing, want %q", tc.key, tc.want)
		} else if got != tc.want {
			t.Errorf("type_registry[%s] = %q, want %q", tc.key, got, tc.want)
		}
	}

	t.Logf("format_contract.json ok: %d ToC keys, %d defaults, %d type registry entries",
		len(c.TocRules.IncludeKeys), len(c.Defaults), len(c.TypeRegistry))
}
