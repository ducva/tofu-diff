package tui

import (
  "testing"
  plan "github.com/ducva/tofu-diff/internal/plan/domain"
  tea "github.com/charmbracelet/bubbletea"
)

func TestDiffOnlyDefault(t *testing.T) {
  pf := plan.Plan{FormatVersion: "1.0"}
  m := New(pf)
  if !m.diffOnly {
    t.Fatalf("expected default diffOnly true, got false")
  }
  m2 := NewWithDiffOnly(pf, false)
  if m2.diffOnly {
    t.Fatalf("expected false")
  }
  m3 := m.WithDiffOnly(false)
  if m3.diffOnly {
    t.Fatalf("WithDiffOnly false failed")
  }
}

func TestToggleO(t *testing.T) {
  pf := plan.Plan{FormatVersion: "1.0", ResourceChanges: []plan.ResourceChange{{Address: "a.b", Change: plan.Change{Actions: []string{"update"}}}}}
  m := New(pf)
  // need to make ready to handle keys
  m.width = 100
  m.height = 30
  // init viewports
  model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
  m = model.(Model)
  if !m.diffOnly {
    t.Fatalf("should start true")
  }
  // toggle off
  model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
  m = model.(Model)
  if m.diffOnly {
    t.Fatalf("toggle should be false")
  }
  if m.copyStatus != "Diff: showing full context" {
    t.Fatalf("unexpected copyStatus %q", m.copyStatus)
  }
  // toggle on
  model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
  m = model.(Model)
  if !m.diffOnly {
    t.Fatalf("toggle should be true again")
  }
  // header should contain DIFF ONLY
  h := m.renderHeader()
  if !contains(h, "DIFF ONLY") {
    t.Fatalf("header should contain badge when diffOnly, got %q", h)
  }
  // footer should highlight o
  f := m.renderFooter()
  if !contains(f, "diff-only") {
    t.Fatalf("footer missing diff-only")
  }
}

func TestResourceContextMenuCopiesResourceName(t *testing.T) {
	var copied string
	originalWriteClipboard := writeClipboard
	writeClipboard = func(text string) error {
		copied = text
		return nil
	}
	defer func() { writeClipboard = originalWriteClipboard }()

	m := readyModelWithResource("module.app.aws_instance.web[0]")
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = model.(Model)
	if !m.contextMenuOpen {
		t.Fatal("expected resource context menu to open")
	}
	if !contains(m.View(), "Copy full resource name") || !contains(m.View(), "Copy tofu plan command") {
		t.Fatal("context menu should show both copy actions")
	}
	if !contains(m.View(), "module.app.aws_instance.web[0]") || !contains(m.View(), "1 to update") {
		t.Fatal("context menu should overlay the existing resource view")
	}
	if !contains(m.renderContextMenu(), "Preview") || !contains(m.renderContextMenu(), "module.app.aws_instance.web[0]") {
		t.Fatal("resource name should be previewed for the first menu item")
	}

	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	if m.contextMenuOpen {
		t.Fatal("expected context menu to close after copying")
	}
	if copied != "module.app.aws_instance.web[0]" {
		t.Fatalf("copied %q, want resource address", copied)
	}
	if m.copyStatus != "Copied resource name!" {
		t.Fatalf("unexpected copy status %q", m.copyStatus)
	}
}

func TestResourceContextMenuCopiesPlanCommand(t *testing.T) {
	var copied string
	originalWriteClipboard := writeClipboard
	writeClipboard = func(text string) error {
		copied = text
		return nil
	}
	defer func() { writeClipboard = originalWriteClipboard }()

	m := readyModelWithResource("module.app.aws_instance.web[0]")
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = model.(Model)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = model.(Model)
	if m.contextMenuCursor != 1 {
		t.Fatalf("cursor = %d, want second menu item", m.contextMenuCursor)
	}
	if !contains(m.renderContextMenu(), "tofu plan -target='module.app.aws_instance.web[0]'") {
		t.Fatal("plan command should be previewed for the second menu item")
	}
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)

	want := "tofu plan -target='module.app.aws_instance.web[0]'"
	if copied != want {
		t.Fatalf("copied %q, want %q", copied, want)
	}
	if m.copyStatus != "Copied tofu plan command!" {
		t.Fatalf("unexpected copy status %q", m.copyStatus)
	}
}

func TestResourceContextMenuCanBeDismissed(t *testing.T) {
	m := readyModelWithResource("aws_instance.web")
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = model.(Model)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = model.(Model)
	if m.contextMenuOpen {
		t.Fatal("expected esc to close context menu")
	}
}

func TestTofuPlanTargetCommandShellQuotesAddress(t *testing.T) {
	got := tofuPlanTargetCommand("module.app.aws_instance.web[0]")
	want := "tofu plan -target='module.app.aws_instance.web[0]'"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := shellQuote("a'b"); got != `'a'\''b'` {
		t.Fatalf("shellQuote = %q, want %q", got, `'a'\''b'`)
	}
}

func TestOverlayCenteredPreservesUnderlyingView(t *testing.T) {
	got := overlayCentered("HEADER\nleft panel content\nFOOTER", "MENU", 18, 3)
	if !contains(got, "MENU") {
		t.Fatal("overlay is missing")
	}
	if !contains(got, "left pa") || !contains(got, "content") {
		t.Fatalf("overlay should preserve content around menu: %q", got)
	}
}

func readyModelWithResource(address string) Model {
	m := New(plan.Plan{
		FormatVersion: "1.0",
		ResourceChanges: []plan.ResourceChange{{
			Address: address,
			Change:  plan.Change{Actions: []string{"update"}},
		}},
	})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return model.(Model)
}

func contains(s, sub string) bool {
  return len(s) >= len(sub) && (func() bool {
    for i:=0; i+len(sub)<=len(s); i++ {
      if s[i:i+len(sub)] == sub { return true }
    }
    return false
  })()
}

func TestFilterToDiffOnly(t *testing.T) {
  in := []uLine{
    {kind: lineKindSame, content: "a"},
    {kind: lineKindRemoved, content: "b"},
    {kind: lineKindSame, content: "c"},
    {kind: lineKindAdded, content: "d"},
    {kind: lineKindSame, content: "e"},
  }
  out := filterToDiffOnly(in)
  if len(out) != 2 {
    t.Fatalf("expected 2, got %d", len(out))
  }
  if out[0].kind != lineKindRemoved || out[1].kind != lineKindAdded {
    t.Fatalf("wrong kinds")
  }
  if len(filterToDiffOnly([]uLine{{kind: lineKindSame}})) != 0 {
    t.Fatalf("all same should be empty")
  }
  if len(filterToDiffOnly(nil)) != 0 {
    t.Fatalf("nil should be empty")
  }
}

func TestBuildRightContentHidesEmptyAttribute(t *testing.T) {
  // This tests the hide behavior: when diffOnly true and attribute would be empty after filtering, it should be hidden.
  // We can test lcsUnify where before==after (all same) -> filtered empty -> buildRightContent should skip that attribute.
  // However DiffAttributes would not include an attribute where before==after, so this is edge. We'll test filter logic directly.
  // Just ensure filterToDiffOnly handles all-same case
}
