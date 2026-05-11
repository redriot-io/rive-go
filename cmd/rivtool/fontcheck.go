package main

import (
	"fmt"

	"github.com/redriot-io/rive-go/internal/fontcheck"
	"github.com/redriot-io/rive-go/rive"
)

// verifyFonts checks glyph coverage for every TextValueRun in f.
// It returns pass messages (full coverage) and error messages (zero coverage).
// Partial coverage is reported as a warning in errs (prefix "⚠").
//
// Font-to-bytes pairing: each FontAsset (typeKey=141) is paired with the
// FileAssetContents (typeKey=106) that immediately follows it in the stream.
// TextValueRun.styleId (key=272) is artboard-relative → resolves to a
// TextStylePaint (typeKey=137), which carries fontAssetId (key=279).
func verifyFonts(f *rive.File) (passes, errs []string) {
	objs := f.Objects

	// Collect FontAsset entries paired with their embedded bytes.
	type fontEntry struct {
		name  string
		bytes []byte
	}
	var fonts []fontEntry
	for i := 0; i < len(objs); i++ {
		if objs[i].TypeKey() != 141 { // FontAsset
			continue
		}
		name := ""
		for _, p := range objs[i].Properties() {
			if p.Key == 203 {
				name, _ = p.Value.(string)
			}
		}
		var fb []byte
		if i+1 < len(objs) && objs[i+1].TypeKey() == 106 { // FileAssetContents
			for _, p := range objs[i+1].Properties() {
				if p.Key == 212 {
					fb, _ = p.Value.([]byte)
				}
			}
			i++ // skip the FileAssetContents
		}
		fonts = append(fonts, fontEntry{name: name, bytes: fb})
	}

	if len(fonts) == 0 {
		return // no embedded fonts → no text to check
	}

	// Find artboard (typeKey=1) — needed to resolve artboard-relative styleIds.
	artboardIdx := -1
	for i, o := range objs {
		if o.TypeKey() == 1 {
			artboardIdx = i
			break
		}
	}
	if artboardIdx < 0 {
		return
	}

	// Build map: global object index → fontAssetId for TextStylePaint (typeKey=137).
	styleFont := make(map[int]int)
	for i, o := range objs {
		if o.TypeKey() != 137 {
			continue
		}
		for _, p := range o.Properties() {
			if p.Key == 279 { // fontAssetId
				if v, ok := p.Value.(uint64); ok {
					styleFont[i] = int(v)
				}
			}
		}
	}

	// Parse each font's cmap once, cache the result.
	type cmapEntry struct {
		fn  func(rune) bool
		err error
	}
	cmaps := make([]cmapEntry, len(fonts))
	for i, fe := range fonts {
		fn, err := fontcheck.ParseCmap(fe.bytes)
		cmaps[i] = cmapEntry{fn: fn, err: err}
	}

	// Check each TextValueRun (typeKey=135).
	for i, o := range objs {
		if o.TypeKey() != 135 {
			continue
		}
		var text string
		var styleId uint64
		hasStyle := false
		for _, p := range o.Properties() {
			switch p.Key {
			case 268: // text
				text, _ = p.Value.(string)
			case 272: // styleId (artboard-relative)
				if v, ok := p.Value.(uint64); ok {
					styleId = v
					hasStyle = true
				}
			}
		}
		if text == "" || !hasStyle {
			continue
		}

		styleGlobal := artboardIdx + int(styleId)
		fontIdx, ok := styleFont[styleGlobal]
		if !ok {
			errs = append(errs, fmt.Sprintf("TextValueRun[%d] %q: styleId=%d → global=%d has no fontAssetId", i, clip(text, 30), styleId, styleGlobal))
			continue
		}
		if fontIdx < 0 || fontIdx >= len(fonts) {
			errs = append(errs, fmt.Sprintf("TextValueRun[%d] %q: fontAssetId=%d out of range (have %d font(s))", i, clip(text, 30), fontIdx, len(fonts)))
			continue
		}
		if len(fonts[fontIdx].bytes) == 0 {
			errs = append(errs, fmt.Sprintf("TextValueRun[%d] %q: font %q has no embedded bytes", i, clip(text, 30), fonts[fontIdx].name))
			continue
		}
		ce := cmaps[fontIdx]
		if ce.err != nil {
			errs = append(errs, fmt.Sprintf("⚠ TextValueRun[%d] %q: cannot parse font %q cmap: %v", i, clip(text, 30), fonts[fontIdx].name, ce.err))
			continue
		}

		covered, total := 0, 0
		for _, r := range text {
			if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
				continue // whitespace — no glyph required
			}
			total++
			if ce.fn(r) {
				covered++
			}
		}
		if total == 0 {
			continue
		}

		label := fmt.Sprintf("font %q, text %q", fonts[fontIdx].name, clip(text, 40))
		switch {
		case covered == 0:
			errs = append(errs, fmt.Sprintf("%s: zero glyph coverage (%d non-space chars) — wrong font?", label, total))
		case covered < total:
			errs = append(errs, fmt.Sprintf("⚠ %s: partial coverage (%d/%d chars)", label, covered, total))
		default:
			passes = append(passes, fmt.Sprintf("%s: full coverage (%d/%d chars)", label, covered, total))
		}
	}

	return
}

func clip(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
