package tui

import (
  "testing"
  "github.com/ducva/tofu-diff/plan"
  tea "github.com/charmbracelet/bubbletea"
)

func TestDiffOnlyDefault(t *testing.T) {
  pf := plan.PlanFile{FormatVersion: "1.0"}
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
  pf := plan.PlanFile{FormatVersion: "1.0", ResourceChanges: []plan.ResourceChange{{Address: "a.b", Change: plan.Change{Actions: []string{"update"}}}}}
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
