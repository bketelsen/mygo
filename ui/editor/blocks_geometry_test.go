package editor

import (
	"math"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Deleting a block's line attaches it to the surviving line at that
// position, without changing whether it goes above or below that line.
func TestBlocksDeletedLinePlacement(t *testing.T) {
	for _, tc := range []struct {
		name, source, survivor string
		line, want             int
	}{
		{"first", "delete\nsurvivor\ntail", "survivor", 0, 0},
		{"middle", "head\ndelete\nsurvivor\ntail", "survivor", 1, 1},
		{"last", "survivor\ndelete", "survivor", 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ct := newEditorTest(t, "", tc.source)
			ct.st.SetBlocks([]Block{tall(tc.line, true, 20, "Above"), tall(tc.line, false, 30, "Below")})
			ct.settle()
			ed := &ct.st.ed
			ids := [2]int{ed.view.blocks[0].id, ed.view.blocks[1].id}
			start := ct.st.Buffer.Offset(Pos{Line: tc.line})
			end := len(tc.source)
			if tc.line+1 < ct.st.Buffer.LineCount() {
				end = ct.st.Buffer.Offset(Pos{Line: tc.line + 1})
			} else {
				start-- // Delete the preceding newline with the final line.
			}
			ct.st.Buffer.Edit(Edit{Start: start, End: end})
			ct.settle()
			if got := ct.st.Buffer.Line(tc.want); got != tc.survivor {
				t.Fatalf("surviving line is %q, want %q", got, tc.survivor)
			}
			for i, b := range ct.st.Blocks() {
				if b.Line != tc.want || b.Above != (i == 0) || ed.view.blocks[i].id != ids[i] {
					t.Fatalf("block %d changed placement or identity: %+v", i, b)
				}
			}
			for _, label := range []string{"Above", "Below"} {
				room, ok := ct.tt.Find(label)
				want := float32(ed.rowTop(tc.want)) + ed.view.lineHeight
				if label == "Above" {
					want = float32(ed.rowTop(tc.want)) - 20
				}
				if !ok || room.Y != ed.r.Y+want {
					t.Fatalf("%s is at %+v (%v), want y %v", label, room, ok, ed.r.Y+want)
				}
			}
		})
	}
}

// Blocks use the same effective text width as layout, including the wrap
// column and minimum width, even with wrapping off and after a resize.
func TestBlocksTextWidth(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		ct := newEditorTest(t, "", "one\ntwo")
		ct.st.SoftWrap = wrap
		ct.st.SetBlocks([]Block{tall(0, false, 40, "Width")})
		for _, column := range []int{0, 20, 40, 1, 0} {
			ct.st.WrapColumn = column
			for _, width := range []int{600, 180, 900} {
				ct.tt.SetSize(width, 400)
				ct.settle()
				ed := &ct.st.ed
				want := float32(width) - ed.textX - ed.trackWidth() - 4
				if column > 0 {
					want = min(want, float32(column)*ed.view.charWidth)
				}
				want = float32(math.Floor(float64(max(want, 8*ed.view.charWidth))))
				room, ok := ct.tt.Find("Width")
				if !ok || room.W != want || wrap && ed.view.wrapWidth != want {
					t.Fatalf("wrap %v, column %d, width %d: block %+v (%v), want %v; text wraps at %v", wrap, column, width, room, ok, want, ed.view.wrapWidth)
				}
			}
		}
	}
}

// Replacement byte offsets and block indices name different things, even
// when elements of both kinds share the same overlay.
func TestPlacedElementIdentities(t *testing.T) {
	ct := newEditorTest(t, "", "head\nobject\ntail")
	r := Replacement{Start: ct.st.Buffer.Offset(Pos{Line: 1}), End: ct.st.Buffer.Offset(Pos{Line: 2}), Content: func(c *ui.Context, e ui.Element, r Replacement) {
		e.Children(func() { ui.Box(c).Fill() })
	}}
	ct.st.SetReplacements([]Replacement{r})
	ct.st.SetBlocks([]Block{tall(0, false, 20, "First"), tall(2, true, 30, "Second")})
	ct.settle()
	ps := ct.st.ed.placed
	if len(ps) != 3 {
		t.Fatalf("placed %d elements, want 1 replacement and 2 blocks", len(ps))
	}
	if ps[0].block || ps[0].start != r.Start || ps[0].blockIndex != 0 {
		t.Fatalf("the replacement lost its byte offset: %+v", ps[0])
	}
	for i, p := range ps[1:] {
		if !p.block || p.blockIndex != i || p.start != 0 {
			t.Fatalf("block %d overloads a text offset: %+v", i, p)
		}
	}
}

// IME anchors and painted stand-in carets share one full-height cell.
// Blocks stay fixed horizontally while the source scrolls sideways.
func TestBlocksStandInCaretGeometryAndShape(t *testing.T) {
	for _, shape := range []CursorShape{CursorBar, CursorBlock, CursorUnderline} {
		for _, focused := range []bool{true, false} {
			ct := newEditorTest(t, "", strings.Repeat("x", 200)+"\ntable\nsource\nend")
			ct.st.Theme, ct.st.NoActiveLine, ct.st.CursorShape = redTheme, true, shape
			r := hide(ct.st.Buffer.Offset(Pos{Line: 1}), ct.st.Buffer.Offset(Pos{Line: 3}))
			ct.st.SetReplacements([]Replacement{r})
			ct.st.SetBlocks([]Block{tall(1, false, 30, "First"), tall(2, true, 50, "Second")})
			ct.st.Select(r.Start, r.Start)
			ct.settle()
			ed := &ct.st.ed
			ed.scrollX = 37
			ed.reveal = revealFit
			ct.tt.SetFocused(focused)
			ct.settle()
			if ed.scrollX != 37 {
				t.Fatalf("shape %v, focused %v: revealing a fixed block changed horizontal scroll to %v", shape, focused, ed.scrollX)
			}
			want := ui.Rect{X: ed.scrollX - 3, Y: ed.view.lineHeight, W: 2, H: 80}
			if shape != CursorBar {
				want.W = ed.view.charWidth
			}
			cr, ok := ed.caretRect(r.Start)
			if !ok || cr != want {
				t.Fatalf("shape %v, focused %v: caret %+v (%v), want %+v", shape, focused, cr, ok, want)
			}
			offset := ed.view.text.UTF16(r.Start)
			bounds, _, ok := ed.BoundsForRange(ui.TextInputRange{Start: offset, End: offset})
			if !ok || bounds != (ui.Rect{X: ed.textX - 3, Y: cr.Y - float32(ed.scrollY), W: 1, H: cr.H}) {
				t.Fatalf("IME bounds %+v (%v) disagree with the stand-in cell %+v", bounds, ok, cr)
			}
			x, y := ed.r.X+ed.textX-3, ed.r.Y+cr.Y-float32(ed.scrollY)
			red := func(px, py float32) bool {
				r, g, _ := ct.pixel(px, py)
				return r > 200 && g < 230
			}
			middle := red(x+want.W/2, y+40)
			top, bottom := red(x+want.W/2, y+0.5), red(x+want.W/2, y+79.5)
			switch {
			case !focused && shape != CursorBlock:
				if middle || top || bottom {
					t.Fatalf("an unfocused %v caret is visible", shape)
				}
			case shape == CursorBar:
				if !middle || !top || !bottom || red(x+4, y+40) {
					t.Fatal("the bar does not cover the full stand-in at its insertion point")
				}
			case shape == CursorBlock && focused:
				r, g, b := ct.pixel(x+want.W/2, y+40)
				if !middle || !top || !bottom || r < 250 || g < 60 || g != b {
					t.Fatalf("the focused block is not full-height translucent red: %d,%d,%d", r, g, b)
				}
			case shape == CursorBlock:
				if middle || !top || !bottom {
					t.Fatal("the unfocused block is not outlined over the full stand-in")
				}
			case shape == CursorUnderline:
				if middle || top || !bottom {
					t.Fatal("the underline is not at the bottom of the whole stand-in")
				}
			}
			ct.tt.Advance(blinkInterval)
			ct.tt.Frame()
			if focused {
				if red(x+want.W/2, y+40) || red(x+want.W/2, y+79.5) {
					t.Fatalf("the %v caret did not blink off", shape)
				}
			} else if shape == CursorBlock && !red(x+want.W/2, y+79.5) {
				t.Fatal("the unfocused block outline blinked off")
			}
		}
	}
}

// Revealing and centering a stand-in use the whole block, not a source
// row that is hidden behind it.
func TestBlocksStandInCaretReveal(t *testing.T) {
	ct := newEditorTest(t, "", strings.Repeat("top\n", 20)+"table\nsource\n"+strings.Repeat("end\n", 100))
	ct.st.ScrollMargin = -1
	r := hide(ct.st.Buffer.Offset(Pos{Line: 20}), ct.st.Buffer.Offset(Pos{Line: 22}))
	ct.st.SetReplacements([]Replacement{r})
	ct.st.SetBlocks([]Block{tall(20, false, 120, "Table")})
	ct.settle()
	ed := &ct.st.ed
	if b := ed.view.blocks[0]; !b.measured || b.height != 120 {
		t.Fatalf("the stand-in is not measured before reveal: %+v", b)
	}
	ct.st.Select(r.Start, r.Start)
	ct.settle()
	top := float64(20 * ed.view.lineHeight)
	if want := top + 120 - float64(ed.viewH); ed.scrollY != want {
		t.Fatalf("revealed at %v, want %v with the whole stand-in visible", ed.scrollY, want)
	}
	want := top - float64(ed.viewH-120)/2
	ct.run(ScrollCursorCenter)
	if ed.scrollY != want {
		t.Fatalf("center command scrolled to %v, want %v", ed.scrollY, want)
	}
	ct.st.SetScrollY(0)
	ed.reveal = revealCenter
	ct.settle()
	if ed.scrollY != want {
		t.Fatalf("center reveal scrolled to %v, want %v", ed.scrollY, want)
	}
}
