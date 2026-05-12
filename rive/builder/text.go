package builder

import (
	"github.com/redriot-io/rive-go/rive"
)

// TextAlign controls horizontal text alignment.
type TextAlign uint64

const (
	AlignLeft   TextAlign = 0
	AlignRight  TextAlign = 1
	AlignCenter TextAlign = 2
)

// TextSizing controls how the text box dimensions are determined.
type TextSizing uint64

const (
	SizingAutoWidth  TextSizing = 0
	SizingAutoHeight TextSizing = 1
	SizingFixed      TextSizing = 2
)

// TextOverflow controls what happens when text exceeds its bounding box.
type TextOverflow uint64

const (
	OverflowVisible  TextOverflow = 0
	OverflowHidden   TextOverflow = 1
	OverflowClipped  TextOverflow = 2
	OverflowEllipsis TextOverflow = 3
	OverflowFit      TextOverflow = 4
)

// axisConfig holds one variable-font axis override.
type axisConfig struct {
	tag   string
	value float64
}

// FontRef is a handle to an embedded font asset added to an artboard.
// Create it via ArtboardBuilder.EmbedFont.
type FontRef struct {
	name     string
	ttfBytes []byte
	idx      uint64 // artboard-relative index of the FontAsset object, set on emit
}

// TextStyleRef is a handle to a TextStyle under a Text object.
// Configure it with Fill, LetterSpacing, LineHeight before using in a Run.
// featureConfig stores one OpenType feature override (e.g. "liga" enabled).
type featureConfig struct {
	tag   string
	value uint64 // 1 = on, 0 = off
}

// modifierRangeConfig stores one TextModifierRange config.
type modifierRangeConfig struct {
	modifyFrom  float64
	modifyTo    float64 // default 1.0
	strength    float64 // default 1.0
	falloffFrom float64
	falloffTo   float64 // default 1.0
	offset      float64
}

// modifierVariationConfig stores one TextVariationModifier config.
type modifierVariationConfig struct {
	tag   string
	value float64
}

// modifierGroupConfig stores one TextModifierGroup config.
type modifierGroupConfig struct {
	name       string
	ranges     []modifierRangeConfig
	variations []modifierVariationConfig
}

// ModifierGroupRef is a handle to a TextModifierGroup under a Text object.
type ModifierGroupRef struct {
	cfg *modifierGroupConfig
}

// Range adds a TextModifierRange to this group. modifyFrom/modifyTo select which
// characters are affected (0.0–1.0 normalized range); strength controls intensity.
func (mg *ModifierGroupRef) Range(modifyFrom, modifyTo, strength float64) *ModifierGroupRef {
	mg.cfg.ranges = append(mg.cfg.ranges, modifierRangeConfig{
		modifyFrom: modifyFrom,
		modifyTo:   modifyTo,
		strength:   strength,
		falloffTo:  1.0,
	})
	return mg
}

// VariationModifier adds a TextVariationModifier to this group that overrides
// the given variable font axis (e.g. "wght") to axisValue.
func (mg *ModifierGroupRef) VariationModifier(tag string, axisValue float64) *ModifierGroupRef {
	mg.cfg.variations = append(mg.cfg.variations, modifierVariationConfig{tag: tag, value: axisValue})
	return mg
}

type TextStyleRef struct {
	font          *FontRef
	fontSize      float64
	fillColor     *uint32
	lineHeight    float64 // 0 = emit -1 (auto); non-zero value used verbatim
	letterSpacing float64
	axes          []axisConfig
	features      []featureConfig

	idx              uint64 // artboard-relative TextStyle index, set on emit
	solidColorIdx    uint64
	hasSolidColorIdx bool
}

// Fill sets the text color as a solid fill (ARGB packed, e.g. 0xFF000000 for black).
func (s *TextStyleRef) Fill(color uint32) *TextStyleRef {
	s.fillColor = &color
	return s
}

// LineHeight sets the line height in pixels. Use 0 (default) for auto.
func (s *TextStyleRef) LineHeight(v float64) *TextStyleRef {
	s.lineHeight = v
	return s
}

// LetterSpacing sets letter spacing in pixels.
func (s *TextStyleRef) LetterSpacing(v float64) *TextStyleRef {
	s.letterSpacing = v
	return s
}

// FontVariation adds a variable font axis override (e.g. tag "wght", value 700).
func (s *TextStyleRef) FontVariation(tag string, value float64) *TextStyleRef {
	s.axes = append(s.axes, axisConfig{tag: tag, value: value})
	return s
}

// Feature adds an OpenType feature override (e.g. tag "liga", value 1 to enable).
// tag must be a 4-character OpenType feature tag; value 1 = on, 0 = off.
func (s *TextStyleRef) Feature(tag string, value uint64) *TextStyleRef {
	s.features = append(s.features, featureConfig{tag: tag, value: value})
	return s
}

// packTag encodes a 4-character OpenType axis tag to a packed uint64.
// e.g. "wght" → 0x77676874 (2003265652).
func packTag(tag string) uint64 {
	b := [4]byte{}
	for i := 0; i < 4 && i < len(tag); i++ {
		b[i] = tag[i]
	}
	return uint64(b[0])<<24 | uint64(b[1])<<16 | uint64(b[2])<<8 | uint64(b[3])
}

// animIdx implements AnimTarget — returns the TextStyle's artboard-relative index.
func (s *TextStyleRef) animIdx() uint64 { return s.idx }

// animColorIdx implements AnimTarget — returns the SolidColor child's index.
func (s *TextStyleRef) animColorIdx() (uint64, bool) {
	return s.solidColorIdx, s.hasSolidColorIdx
}

// runConfig holds one text run (text content + style reference).
type runConfig struct {
	text  string
	style *TextStyleRef
}

// TextRef is a handle to a Text object in an artboard.
// Create it via ArtboardBuilder.Text.
type TextRef struct {
	name   string
	x, y   float64
	align  TextAlign
	sizing TextSizing
	width  float64
	height float64

	overflow TextOverflow

	styles         []*TextStyleRef
	runs           []runConfig
	modifierGroups []*modifierGroupConfig

	idx uint64 // artboard-relative Text index, set on emit
}

// ModifierGroup adds a TextModifierGroup to this text object and returns a ref
// for further configuration (add ranges and variation modifiers).
func (t *TextRef) ModifierGroup(name string) *ModifierGroupRef {
	cfg := &modifierGroupConfig{name: name}
	t.modifierGroups = append(t.modifierGroups, cfg)
	return &ModifierGroupRef{cfg: cfg}
}

// Position sets the text object's x, y coordinates.
func (t *TextRef) Position(x, y float64) *TextRef {
	t.x = x
	t.y = y
	return t
}

// Align sets horizontal text alignment.
func (t *TextRef) Align(a TextAlign) *TextRef {
	t.align = a
	return t
}

// Sizing sets the text sizing mode.
func (t *TextRef) Sizing(s TextSizing) *TextRef {
	t.sizing = s
	return t
}

// Overflow sets what happens when text content exceeds its bounding box.
func (t *TextRef) Overflow(o TextOverflow) *TextRef {
	t.overflow = o
	return t
}

// Size sets the fixed text box width and height (for SizingFixed mode).
func (t *TextRef) Size(w, h float64) *TextRef {
	t.width = w
	t.height = h
	return t
}

// Style adds a TextStyle to this text and returns the style reference.
// font and fontSize are required. Decorate the result with Fill / LetterSpacing / LineHeight.
func (t *TextRef) Style(font *FontRef, fontSize float64) *TextStyleRef {
	s := &TextStyleRef{
		font:     font,
		fontSize: fontSize,
	}
	t.styles = append(t.styles, s)
	return s
}

// Run appends a text span with the given style.
func (t *TextRef) Run(text string, style *TextStyleRef) *TextRef {
	t.runs = append(t.runs, runConfig{text: text, style: style})
	return t
}

// FirstStyle returns the first TextStyleRef added to this Text, or nil if none.
// Consumed by fromjson to resolve "name.style.X" animation dot-paths.
func (t *TextRef) FirstStyle() *TextStyleRef {
	if len(t.styles) == 0 {
		return nil
	}
	return t.styles[0]
}

// animIdx implements AnimTarget — returns the Text object's artboard-relative index.
func (t *TextRef) animIdx() uint64 { return t.idx }

// animColorIdx implements AnimTarget — returns the first style's SolidColor index.
func (t *TextRef) animColorIdx() (uint64, bool) {
	for _, s := range t.styles {
		if s.hasSolidColorIdx {
			return s.solidColorIdx, true
		}
	}
	return 0, false
}

// emitObjects writes Text → TextStyle(s) → Fill → SolidColor(s) → TextValueRun(s)
// into the object list. FontAssets must already be emitted (fonRef.idx set).
func (t *TextRef) emitObjects(objects *[]rive.Object, parentIdx uint64, artboardOffset uint64) {
	// --- Text ---
	t.idx = uint64(len(*objects)) - artboardOffset

	txt := &rive.Text{}
	txt.Name = t.name
	txt.ParentId = parentIdx
	txt.X = t.x
	txt.Y = t.y
	txt.Opacity = 1.0
	txt.ScaleX = 1.0
	txt.ScaleY = 1.0
	txt.BlendModeValue = 3
	txt.AlignValue = uint64(t.align)
	txt.SizingValue = uint64(t.sizing)
	txt.OverflowValue = uint64(t.overflow)
	txt.FitFromBaseline = true           // Rive default; suppresses k703 emission
	txt.TextRunListSource = ^uint64(0)   // Rive sentinel; suppresses k932 emission
	if t.width > 0 {
		txt.Width = t.width
	}
	if t.height > 0 {
		txt.Height = t.height
	}
	*objects = append(*objects, txt)

	// --- TextStyle children ---
	for _, style := range t.styles {
		style.idx = uint64(len(*objects)) - artboardOffset

		ts := &rive.TextStylePaint{}
		ts.ParentId = t.idx
		ts.FontSize = style.fontSize
		ts.FontAssetId = style.font.idx
		// lineHeight 0 in our API → Rive's -1 (auto); non-zero → use directly
		if style.lineHeight != 0 {
			ts.LineHeight = style.lineHeight
		} else {
			ts.LineHeight = -1.0
		}
		ts.LetterSpacing = style.letterSpacing
		*objects = append(*objects, ts)

		// --- Fill paint under TextStyle ---
		if style.fillColor != nil {
			// SolidColor emitted BEFORE Fill (official encoder forward-reference pattern).
			// SolidColor.parentId points forward to Fill (the very next slot).
			style.solidColorIdx = uint64(len(*objects)) - artboardOffset
			style.hasSolidColorIdx = true
			fillFwdRef := style.solidColorIdx + 1
			sc := &rive.SolidColor{}
			sc.ColorValue = *style.fillColor
			sc.ParentId = fillFwdRef
			*objects = append(*objects, sc)

			fill := &rive.Fill{}
			fill.ParentId = style.idx
			fill.IsVisible = true
			fill.BlendModeValue = 127
			*objects = append(*objects, fill)
		}

		// --- TextStyleAxis children (variable font axes) ---
		for _, ax := range style.axes {
			tsa := &rive.TextStyleAxis{}
			tsa.ParentId = style.idx
			tsa.Tag = packTag(ax.tag)
			tsa.AxisValue = ax.value
			*objects = append(*objects, tsa)
		}

		// --- TextStyleFeature children (OpenType feature overrides) ---
		for _, ft := range style.features {
			tsf := &rive.TextStyleFeature{}
			tsf.ParentId = style.idx
			tsf.Tag = packTag(ft.tag)
			tsf.FeatureValue = ft.value
			*objects = append(*objects, tsf)
		}
	}

	// --- TextValueRun children ---
	for _, run := range t.runs {
		tvr := &rive.TextValueRun{}
		tvr.ParentId = t.idx
		tvr.Text = run.text
		tvr.StyleId = run.style.idx
		*objects = append(*objects, tvr)
	}

	// --- TextModifierGroup children ---
	for _, mgCfg := range t.modifierGroups {
		groupIdx := uint64(len(*objects)) - artboardOffset
		tmg := &rive.TextModifierGroup{}
		tmg.Name = mgCfg.name
		tmg.ParentId = t.idx
		tmg.ScaleX = 1.0
		tmg.ScaleY = 1.0
		tmg.Opacity = 1.0
		*objects = append(*objects, tmg)

		for _, rc := range mgCfg.ranges {
			tmr := &rive.TextModifierRange{}
			tmr.ParentId = groupIdx
			tmr.ModifyFrom = rc.modifyFrom
			tmr.ModifyTo = rc.modifyTo
			tmr.Strength = rc.strength
			tmr.FalloffTo = rc.falloffTo
			tmr.FalloffFrom = rc.falloffFrom
			tmr.Offset = rc.offset
			*objects = append(*objects, tmr)
		}

		for _, vc := range mgCfg.variations {
			tvm := &rive.TextVariationModifier{}
			tvm.ParentId = groupIdx
			tvm.AxisTag = packTag(vc.tag)
			tvm.AxisValue = vc.value
			*objects = append(*objects, tvm)
		}
	}
}
