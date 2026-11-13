package xadmin

import (
	"errors"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTUIMenuNavigation(t *testing.T) {
	m := newTUIModel()
	m.width, m.height = 100, 30

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = model.(*tuiModel)
	if m.menuCursor != 1 {
		t.Fatalf("menuCursor = %d, want 1", m.menuCursor)
	}

	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = model.(*tuiModel)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = model.(*tuiModel)
	if m.menuCursor != 1 {
		t.Fatalf("menuCursor = %d, want 1", m.menuCursor)
	}
}

func TestTUIAPIFormAuthToggle(t *testing.T) {
	m := newTUIModel()
	m.width, m.height = 100, 30
	m.openAPIForm()

	if m.screen != screenAPIForm {
		t.Fatalf("screen = %v, want API form", m.screen)
	}
	if len(m.inputs) != 5 {
		t.Fatalf("fields = %d, want 5", len(m.inputs))
	}
	if got := m.inputs[1].Value(); got != "v1" {
		t.Fatalf("default version = %q, want v1", got)
	}
	if m.auth {
		t.Fatal("auth should default to false")
	}

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = model.(*tuiModel)
	if !m.auth {
		t.Fatal("auth should be enabled after ctrl+t")
	}

	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = model.(*tuiModel)
	if m.formFocus != 1 {
		t.Fatalf("formFocus = %d, want 1", m.formFocus)
	}
}

func TestTUIViewDoesNotPanic(t *testing.T) {
	m := newTUIModel()
	for _, size := range [][2]int{{0, 0}, {80, 24}, {120, 40}} {
		m.width, m.height = size[0], size[1]
		m.resize()
		for _, screen := range []tuiScreen{screenMenu, screenAPIForm, screenBusy, screenOutput} {
			m.screen = screen
			if view := m.View(); view == "" {
				t.Fatalf("screen %v rendered empty view at %dx%d", screen, size[0], size[1])
			}
		}
	}
}

func TestTUIOutputShowsError(t *testing.T) {
	m := newTUIModel()
	m.width, m.height = 100, 30
	m.resize()
	m.showText(tuiTextMsg{title: "测试", err: os.ErrNotExist})
	if !strings.Contains(m.viewport.View(), "✖") {
		t.Fatalf("output missing error marker:\n%s", m.viewport.View())
	}
}

func TestTUIFormErrorStaysOnForm(t *testing.T) {
	m := newTUIModel()
	m.width, m.height = 100, 30
	m.openAPIForm()
	m.showText(tuiTextMsg{title: "创建 API 面", err: errors.New("bad name"), form: screenAPIForm})
	if m.screen != screenAPIForm {
		t.Fatalf("screen = %v, want API form", m.screen)
	}
	if !m.statusErr || !strings.Contains(m.status, "bad name") {
		t.Fatalf("status = %q, statusErr = %v", m.status, m.statusErr)
	}
}

func TestShouldUseTUIRespectsEnvironment(t *testing.T) {
	t.Setenv("XADMIN_NO_TUI", "1")
	if shouldUseTUI() {
		t.Fatal("XADMIN_NO_TUI should disable TUI")
	}
	t.Setenv("XADMIN_NO_TUI", "")
	t.Setenv("CI", "true")
	if shouldUseTUI() {
		t.Fatal("CI should disable TUI")
	}
}

func TestIsTerminalOnNonTTY(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if isTerminal(f) {
		t.Fatal("/dev/null should not be treated as a terminal")
	}
}
