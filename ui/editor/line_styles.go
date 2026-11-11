package editor

import (
	"slices"
	"sort"

	"github.com/egoist/mygo/ui"
)

// LineStyle draws a line in another font. Zero font fields inherit the editor's.
// Styles move with edits, like blocks; diff views take no styles.
type LineStyle struct {
	Line int
	Font ui.Font
	// LineHeight is a multiple of the font size; zero inherits the editor's.
	LineHeight float32
	// Strike draws a rule through the text, including wrapped rows.
	Strike bool
}

type appLineStyle struct {
	LineStyle
	at int
}
type lineStyleKey struct {
	font   ui.Font
	height float32
}

// SetLineStyles replaces the styles. Missing lines are ignored; the last style
// of a line wins. Only changed lines need laying out again.
func (st *State) SetLineStyles(styles []LineStyle) {
	st.ensure()
	v := &st.ed.view
	next := make([]appLineStyle, 0, len(styles))
	for _, s := range styles {
		if s.Line >= 0 && s.Line < v.text.LineCount() {
			next = append(next, appLineStyle{s, v.text.LineStart(s.Line)})
		}
	}
	slices.SortStableFunc(next, func(a, b appLineStyle) int { return a.Line - b.Line })
	out := next[:0]
	for _, s := range next {
		if len(out) > 0 && out[len(out)-1].Line == s.Line {
			out[len(out)-1] = s
		} else {
			out = append(out, s)
		}
	}
	if slices.Equal(v.lineStyles, out) {
		return
	}
	for i, j := 0, 0; i < len(v.lineStyles) || j < len(out); {
		switch {
		case j >= len(out) || i < len(v.lineStyles) && v.lineStyles[i].Line < out[j].Line:
			v.forgetLines(v.lineStyles[i].at, v.lineStyles[i].at)
			i++
		case i >= len(v.lineStyles) || out[j].Line < v.lineStyles[i].Line:
			v.forgetLines(out[j].at, out[j].at)
			j++
		default:
			if v.lineStyles[i].LineStyle != out[j].LineStyle {
				v.forgetLines(out[j].at, out[j].at)
			}
			i++
			j++
		}
	}
	v.lineStyles = out
	v.stylesDirty = true
	v.heights.dirty = true
}

// LineStyles returns a copy of the current styles, after following edits.
func (st *State) LineStyles() []LineStyle {
	st.ensure()
	out := make([]LineStyle, len(st.ed.view.lineStyles))
	for i, s := range st.ed.view.lineStyles {
		out[i] = s.LineStyle
	}
	return out
}

func (v *view) lineStyle(line int) *appLineStyle {
	i := sort.Search(len(v.lineStyles), func(i int) bool { return v.lineStyles[i].Line >= line })
	if i < len(v.lineStyles) && v.lineStyles[i].Line == line {
		return &v.lineStyles[i]
	}
	return nil
}

func (v *view) styledView(s LineStyle) *view {
	f := s.Font
	if f.Family == "" {
		f.Family = v.font.Family
	}
	if f.Size <= 0 {
		f.Size = v.font.Size
	}
	if f.Weight == 0 {
		f.Weight = v.font.Weight
	}
	if !f.Italic {
		f.Italic = v.font.Italic
	}
	if f.Features == "" {
		f.Features = v.font.Features
	}
	if f.Lang == "" {
		f.Lang = v.font.Lang
	}
	if !f.Thicken {
		f.Thicken = v.font.Thicken
	}
	height := s.LineHeight
	if height <= 0 {
		height = v.lineHeight / v.font.Size
	}
	key := lineStyleKey{f, height}
	if v.styleViews == nil {
		v.styleViews = map[lineStyleKey]*view{}
	}
	child := v.styleViews[key]
	if child == nil {
		child = &view{}
		v.styleViews[key] = child
	}
	if child.version != v.version || child.buf != v.buf || child.hl != v.hl {
		child.lines.m = nil
	}
	// The buffer's highlighter colors its lines, as the editor's own.
	child.hl = v.hl
	child.buf, child.text, child.version = v.buf, v.text, v.version
	child.repls, child.inlays = v.repls, v.inlays
	child.setStyle(f, height, v.scale, v.tabSize)
	child.setTheme(v.theme)
	child.setWrap(v.wrap, v.wrapWidth)
	return child
}

func (v *view) refreshLineHeights() {
	if !v.stylesDirty {
		return
	}
	if len(v.lineStyles) == 0 && len(v.heights.styles) == 0 {
		return
	}
	hs := v.heights.styles[:0]
	for _, s := range v.lineStyles {
		child := v.styledView(s.LineStyle)
		hs = append(hs, heightEntry{s.Line, child.lineHeight})
	}
	v.heights.styles = hs
	v.heights.dirty = true
	v.stylesDirty = false
}

func (v *view) moveLineStyles(cs []change) {
	for i := range v.lineStyles {
		v.lineStyles[i].at = mapOffset(v.lineStyles[i].at, cs, biasRight)
	}
	v.stylesDirty = true
}
func (v *view) settleLineStyles() {
	for i := range v.lineStyles {
		s := &v.lineStyles[i]
		s.Line = v.text.LineOf(s.at)
		s.at = v.text.LineStart(s.Line)
	}
	v.lineStyles = slices.CompactFunc(v.lineStyles, func(a, b appLineStyle) bool { return a.Line == b.Line })
}
func (v *view) rowHeight(line int) float32 { return v.heights.rowHeight(line) }
func (h *heights) rowHeight(line int) float32 {
	i := sort.Search(len(h.styles), func(i int) bool { return h.styles[i].line >= line })
	if i < len(h.styles) && h.styles[i].line == line {
		return h.styles[i].extra
	}
	return h.lh
}

func (v *view) invalidateStyledLines() {
	for _, child := range v.styleViews {
		child.lines.m = nil
	}
}
