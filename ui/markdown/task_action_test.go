package markdown

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestTaskActionReportsItsSourceLineAndNextState(t *testing.T) {
	var line int
	var next bool
	o := Options{OnTask: func(n int, checked bool) { line, next = n, checked }}
	tt := ui.NewTester(func(c *ui.Context) { View(c, "Intro\n\n- [ ] water\n", &o) }, 400, 200)
	if err := tt.Click("Mark task done"); err != nil {
		t.Fatal(err)
	}
	if line != 3 || !next {
		t.Fatalf("task action: line %d, checked %v", line, next)
	}
}
func TestCodeBlockReportsItsOpeningSourceLine(t *testing.T) {
	line := 0
	o := Options{Code: map[string]func(*ui.Context, *CodeBlockView){"custom": func(c *ui.Context, b *CodeBlockView) { line = b.SourceLine(); ui.Text(c, b.Code()) }}}
	ui.NewTester(func(c *ui.Context) { View(c, "Intro\n\n```custom\nbody\n```\n", &o) }, 400, 200)
	if line != 3 {
		t.Fatalf("fence line %d", line)
	}
}
