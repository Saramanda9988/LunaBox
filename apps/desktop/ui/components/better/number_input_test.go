package better

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/theme"
)

type numberHarness struct {
	value   int
	changes []int
	tt      *ui.Tester
}

func newNumberHarness(value int, p NumberInputProps) *numberHarness {
	h := &numberHarness{value: value}
	th := theme.New(true, 1.5)
	p.Label = "数量" // i18n:keep：测试数据
	p.OnChange = func(v int) {
		h.value = v
		h.changes = append(h.changes, v)
	}
	h.tt = ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Padding(20).Children(func() { NumberInput(c, th, "n", h.value, p) })
	}, 400, 200)
	h.tt.Frame()
	return h
}

func (h *numberHarness) focus(t *testing.T) {
	t.Helper()
	if err := h.tt.Click("数量"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
}

// 输入时只提交范围内的数字；按 Enter 时把草稿收回到范围内。
func TestNumberInputTypingAndClamp(t *testing.T) {
	h := newNumberHarness(5, NumberInputProps{Min: Bound(1), Max: Bound(100)})
	h.focus(t)
	h.tt.Command("selectAll")
	h.tt.Type("42")
	h.tt.Frame()
	if h.value != 42 {
		t.Fatalf("value %d after typing 42", h.value)
	}
	h.tt.Command("selectAll")
	h.tt.Type("500")
	h.tt.Frame()
	if h.value != 42 {
		t.Fatalf("out of range draft committed: %d", h.value)
	}
	h.tt.Key(0, ui.KeyEnter)
	h.tt.Frame()
	h.tt.Frame()
	if h.value != 100 {
		t.Errorf("Enter clamps to max: %d", h.value)
	}
}

// ↑ / ↓ 与步进按钮按 Step 增减并夹在范围内。
func TestNumberInputStep(t *testing.T) {
	h := newNumberHarness(10, NumberInputProps{Min: Bound(0), Max: Bound(20), Step: 5})
	h.focus(t)
	h.tt.Key(0, ui.KeyUp)
	h.tt.Frame()
	h.tt.Frame()
	if h.value != 15 {
		t.Fatalf("up: %d", h.value)
	}
	h.tt.Key(0, ui.KeyUp)
	h.tt.Frame()
	h.tt.Key(0, ui.KeyUp)
	h.tt.Frame()
	h.tt.Frame()
	if h.value != 20 {
		t.Fatalf("up clamps to max: %d", h.value)
	}
	h.tt.Key(0, ui.KeyDown)
	h.tt.Frame()
	h.tt.Frame()
	if h.value != 15 {
		t.Errorf("down: %d", h.value)
	}
}

func TestNumberInputDisabled(t *testing.T) {
	h := newNumberHarness(3, NumberInputProps{Disabled: true})
	r, ok := h.tt.Find("数量")
	if !ok {
		t.Fatalf("input not found: %q", h.tt.Texts())
	}
	// 右侧的步进按钮（框宽 144，按钮宽 36）
	h.tt.ClickAt(r.X+r.W+18, r.Y+r.H/4)
	h.tt.Frame()
	h.tt.Key(0, ui.KeyUp)
	h.tt.Frame()
	h.tt.Frame()
	if len(h.changes) != 0 {
		t.Errorf("disabled input changed: %v", h.changes)
	}
}

func TestParseDraft(t *testing.T) {
	for in, want := range map[string]int{"12": 12, "-3": -3, "7abc": 7, " 8": 8} {
		if got, ok := parseDraft(in); !ok || got != want {
			t.Errorf("parseDraft(%q) = %d, %v", in, got, ok)
		}
	}
	for _, in := range []string{"", "-", "abc"} {
		if _, ok := parseDraft(in); ok {
			t.Errorf("parseDraft(%q) ok", in)
		}
	}
}
