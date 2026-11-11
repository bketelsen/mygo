package editor

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestLineStylesGeometryCaretHitAndEdits(t *testing.T) {
	ct := newEditorTest(t, "", "one\nheading\nlast")
	ct.st.SetLineStyles([]LineStyle{{Line: 1, Font: ui.Font{Size: 28}, LineHeight: 1.5, Strike: true}})
	ct.settle()
	ed := &ct.st.ed
	v := &ed.view
	tall := v.line(1)
	if tall.rowHeight != 42 || !tall.strike {
		t.Fatalf("metrics: %+v", tall)
	}
	if got, want := ed.rowTop(2), float64(v.lineHeight)+42; got != want {
		t.Fatalf("last at %v, want %v", got, want)
	}
	for _, line := range []int{0, 1, 2} {
		at := v.text.LineStart(line) + 1
		r, ok := ed.caretRect(at)
		if !ok {
			t.Fatal("no caret")
		}
		got := ed.offsetAt(r.X, float64(r.Y+r.H/2))
		if got != at {
			t.Fatalf("line %d hit %d, want %d", line, got, at)
		}
	}
	ct.st.Buffer.Edit(Edit{Start: 0, End: 0, Text: "new\n"})
	ct.settle()
	if got := ct.st.LineStyles(); len(got) != 1 || got[0].Line != 2 {
		t.Fatalf("moved styles: %v", got)
	}
	ct.st.SetLineStyles(nil)
	ct.settle()
	if v.rowHeight(2) != v.lineHeight {
		t.Fatal("style removal retained height")
	}
}

func TestStyledWrappingReplacementsAndBlocks(t *testing.T) {
	ct := newEditorTest(t, "", "first\n"+strings.Repeat("heading ", 30)+"\nlast")
	ct.st.SoftWrap = true
	ct.st.SetLineStyles([]LineStyle{{Line: 1, Font: ui.Font{Size: 30}}})
	ct.st.SetReplacements([]Replacement{{Start: 6, End: 13, Content: func(c *ui.Context, e ui.Element, _ Replacement) { e.Children(func() { ui.Text(c, "H") }) }}})
	ct.st.SetBlocks([]Block{tall(1, false, 30, "Below")})
	ct.settle()
	v := &ct.st.ed.view
	l := v.line(1)
	if len(l.rows) < 2 {
		t.Fatal("heading did not wrap")
	}
	if g := v.heights.geometry(1); g.below-g.rows != float64(len(l.rows))*float64(l.rowHeight) {
		t.Fatalf("wrapped geometry: %+v", g)
	}
	placed := ct.st.ed.placeReplacements(0, 10000, nil)
	if len(placed) != 1 || placed[0].rect.H != l.rowHeight || placed[0].rect.W != l.rowHeight {
		t.Fatalf("replacement geometry: %+v", placed)
	}
}

func TestLineStyleHeightsAgreeWithHitTesting(t *testing.T) {
	r := rand.New(rand.NewPCG(4, 7))
	h := heights{lh: 20, n: 100, dirty: true}
	for i := 0; i < 100; i++ {
		if r.IntN(3) == 0 {
			h.styles = append(h.styles, heightEntry{i, float32(10 + r.IntN(40))})
		}
	}
	h.setHidden([]lineSpan{{10, 15}, {40, 43}})
	h.setBlocks([]block{{line: 20, height: 70}, {line: 60, height: 30}})
	for i := range 100 {
		if h.isHidden(i) {
			continue
		}
		g := h.geometry(i)
		if got := h.lineAt((g.rows + g.below) / 2); got != i {
			t.Fatalf("hit %d at line %d", got, i)
		}
		if g.bottom != h.top(i+1) {
			t.Fatalf("gap after %d: %v != %v", i, g.bottom, h.top(i+1))
		}
	}
}

func TestLineStyleChangesKeepUnchangedLayouts(t *testing.T) {
	ct := newEditorTest(t, "", "one\ntwo\nthree")
	styles := []LineStyle{{Line: 0, Font: ui.Font{Size: 20}}, {Line: 1, Font: ui.Font{Size: 25}}}
	ct.st.SetLineStyles(styles)
	ct.settle()
	before := ct.st.ed.view.line(0)
	styles[1].Font.Size = 30
	ct.st.SetLineStyles(styles)
	ct.settle()
	if ct.st.ed.view.line(0) != before {
		t.Fatal("unchanged styled line laid out again")
	}
	ct.st.SetLineStyles(styles)
	ct.settle()
	if ct.st.ed.view.line(0) != before {
		t.Fatal("identical styles laid out again")
	}
}

func TestFoldedStyledLineTakesNoHeight(t *testing.T) {
	ct := newEditorTest(t, "go", "func a() {\n\txx()\n}\nz")
	ct.st.SetLineStyles([]LineStyle{{Line: 1, Font: ui.Font{Size: 50}}})
	ct.settle()
	ct.st.Fold(0)
	ct.settle()
	if got, want := ct.st.ed.view.heights.total(), 3*float64(ct.st.ed.view.lineHeight); got != want {
		t.Fatalf("folded height %v, want %v", got, want)
	}
}

// A styled line keeps its syntax colors, and a block under it goes below
// its taller rows.
func TestLineStylesTokensAndBlocks(t *testing.T) {
	ct := newEditorTest(t, "go", "package a\n// a comment\nvar x = 1\n")
	ct.st.SetLineStyles([]LineStyle{{Line: 1, Font: ui.Font{Size: 26}}})
	ct.st.SetBlocks([]Block{tall(1, false, 40, "Under")})
	ct.settle()
	ed := &ct.st.ed
	l := ed.view.line(1)
	if len(l.toks) == 0 {
		t.Fatal("the styled line has no tokens")
	}
	under, ok := ct.tt.Find("Under")
	if !ok {
		t.Fatal("the block isn't built")
	}
	g := ed.view.heights.geometry(1)
	if want := ed.r.Y + float32(g.rows+float64(len(l.rows))*float64(l.rowHeight)); under.Y != want {
		t.Fatalf("the block is at %v, want %v under the styled line", under.Y, want)
	}
	if got, want := ed.rowTop(2), g.rows+float64(l.rowHeight)+40; got != want {
		t.Fatalf("the line after is at %v, want %v", got, want)
	}
}
