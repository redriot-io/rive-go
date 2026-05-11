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

// FontRef is a handle to an embedded font asset added to an artboard.
// Create it via ArtboardBuilder.EmbedFont.
type FontRef struct {
	name     string
	ttfBytes []byte
	idx      uint64 // artboard-relative index of the FontAsset object, set on emit
}

// TextStyleRef is a handle to a TextStyle under a Text object.
// Configure it with Fill, LetterSpacing, LineHeight before using in a Run.
type TextStyleRef struct {
	font          *FontRef
	fontSize      float64
	fillColor     *uint32
	lineHeight    float64 // 0 = emit -1 (auto); non-zero value used verbatim
	letterSpacing float64

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

	styles []*TextStyleRef
	runs   []runConfig

	idx uint64 // artboard-relative Text index, set on emit
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

		ts := &rive.TextStyle{}
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
			fillIdx := uint64(len(*objects)) - artboardOffset
			fill := &rive.Fill{}
			fill.ParentId = style.idx
			fill.IsVisible = true
			fill.BlendModeValue = 127
			*objects = append(*objects, fill)

			style.solidColorIdx = uint64(len(*objects)) - artboardOffset
			style.hasSolidColorIdx = true
			sc := &rive.SolidColor{}
			sc.ColorValue = *style.fillColor
			sc.ParentId = fillIdx
			*objects = append(*objects, sc)
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
}
