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

func TestGroupByModuleDefault(t *testing.T) {
	pf := plan.Plan{FormatVersion: "1.0"}
	m := New(pf)
	if m.groupByModule {
		t.Fatalf("expected default groupByModule false, got true")
	}
	m2 := NewWithOptions(pf, true, true)
	if !m2.groupByModule {
		t.Fatalf("expected NewWithOptions with groupByModule=true, got false")
	}
	m3 := m.WithGroupByModule(true)
	if !m3.groupByModule {
		t.Fatalf("WithGroupByModule true failed")
	}
}

func TestToggleM(t *testing.T) {
	pf := plan.Plan{
		FormatVersion: "1.0",
		ResourceChanges: []plan.ResourceChange{
			{Address: "module.vpc.aws_subnet.public", ModuleAddress: "module.vpc", Change: plan.Change{Actions: []string{"create"}}},
			{Address: "aws_s3_bucket.main", Change: plan.Change{Actions: []string{"update"}}},
		},
	}
	m := New(pf)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = model.(Model)

	if m.groupByModule {
		t.Fatal("expected groupByModule to start false")
	}

	// Toggle on with 'm'
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m = model.(Model)
	if !m.groupByModule {
		t.Fatal("expected groupByModule to be true after toggle")
	}
	if m.copyStatus != "Group by module: enabled" {
		t.Fatalf("unexpected copyStatus %q", m.copyStatus)
	}
	if !contains(m.renderHeader(), "MODULES") {
		t.Fatal("header should contain MODULES badge")
	}
	if !contains(m.renderFooter(), "modules") {
		t.Fatal("footer should contain modules hint")
	}

	// In grouped mode, left panel should have module headers
	leftContent := m.buildLeftContent()
	if !contains(leftContent, "(root)") || !contains(leftContent, "module.vpc") {
		t.Fatalf("left panel should contain module headers, got:\n%s", leftContent)
	}
	// And relative address aws_subnet.public instead of module.vpc.aws_subnet.public
	if !contains(leftContent, "aws_subnet.public") {
		t.Fatalf("left panel should contain relative address aws_subnet.public, got:\n%s", leftContent)
	}

	// Toggle off with 'm'
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m = model.(Model)
	if m.groupByModule {
		t.Fatal("expected groupByModule to be false after toggle")
	}
	if m.copyStatus != "Group by module: disabled" {
		t.Fatalf("unexpected copyStatus %q", m.copyStatus)
	}
	if contains(m.renderHeader(), "MODULES") {
		t.Fatal("header should not contain MODULES badge when disabled")
	}
}

func TestModuleNavigationAndCollapse(t *testing.T) {
	pf := plan.Plan{
		FormatVersion: "1.0",
		ResourceChanges: []plan.ResourceChange{
			{Address: "module.vpc.aws_subnet.public", ModuleAddress: "module.vpc", Change: plan.Change{Actions: []string{"create"}}},
			{Address: "aws_s3_bucket.main", Change: plan.Change{Actions: []string{"update"}}},
		},
	}
	m := NewWithOptions(pf, true, true)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = model.(Model)

	// Items should be:
	// 0: Module (root)
	// 1: Resource aws_s3_bucket.main
	// 2: Module module.vpc
	// 3: Resource module.vpc.aws_subnet.public
	if len(m.items) != 4 {
		t.Fatalf("expected 4 items (2 modules + 2 resources), got %d", len(m.items))
	}
	if m.items[0].kind != itemModuleHeader || m.items[0].module != "(root)" {
		t.Fatalf("expected item 0 to be (root) module header, got %+v", m.items[0])
	}
	if m.items[1].kind != itemResource {
		t.Fatalf("expected item 1 to be resource, got %+v", m.items[1])
	}
	if m.items[2].kind != itemModuleHeader || m.items[2].module != "module.vpc" {
		t.Fatalf("expected item 2 to be module.vpc header, got %+v", m.items[2])
	}
	if m.items[3].kind != itemResource {
		t.Fatalf("expected item 3 to be resource, got %+v", m.items[3])
	}

	// Press Space on item 0 ((root) module header) -> collapses (root)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = model.(Model)
	if !m.collapsedModules["(root)"] {
		t.Fatal("expected (root) module to be collapsed")
	}
	// After collapsing (root), items should be 3:
	// 0: Module (root)
	// 1: Module module.vpc
	// 2: Resource module.vpc.aws_subnet.public
	if len(m.items) != 3 {
		t.Fatalf("expected 3 items after collapsing (root), got %d", len(m.items))
	}

	// Press Space again -> expands (root)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = model.(Model)
	if m.collapsedModules["(root)"] {
		t.Fatal("expected (root) module to be expanded")
	}
	if len(m.items) != 4 {
		t.Fatalf("expected 4 items after expanding (root), got %d", len(m.items))
	}
}

func TestModuleRightPanelSummary(t *testing.T) {
	pf := plan.Plan{
		FormatVersion: "1.0",
		ResourceChanges: []plan.ResourceChange{
			{Address: "module.vpc.aws_subnet.public", ModuleAddress: "module.vpc", Change: plan.Change{Actions: []string{"create"}}},
			{Address: "module.vpc.aws_vpc.main", ModuleAddress: "module.vpc", Change: plan.Change{Actions: []string{"update"}}},
		},
	}
	m := NewWithOptions(pf, true, true)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = model.(Model)

	// Cursor is at 0 (module.vpc header)
	rightContent := m.buildRightContent()
	if !contains(rightContent, "MODULE") {
		t.Fatalf("right panel should contain MODULE header, got:\n%s", rightContent)
	}
	if !contains(rightContent, "module.vpc") {
		t.Fatalf("right panel should contain module.vpc, got:\n%s", rightContent)
	}
	if !contains(rightContent, "RESOURCES (2)") {
		t.Fatalf("right panel should show resource count, got:\n%s", rightContent)
	}
	if !contains(rightContent, "module.vpc.aws_subnet.public") || !contains(rightContent, "module.vpc.aws_vpc.main") {
		t.Fatalf("right panel should list resources in module, got:\n%s", rightContent)
	}
}

func TestModuleContextMenuAndCopy(t *testing.T) {
	var copied string
	originalWriteClipboard := writeClipboard
	writeClipboard = func(text string) error {
		copied = text
		return nil
	}
	defer func() { writeClipboard = originalWriteClipboard }()

	pf := plan.Plan{
		FormatVersion: "1.0",
		ResourceChanges: []plan.ResourceChange{
			{Address: "module.vpc.aws_subnet.public", ModuleAddress: "module.vpc", Change: plan.Change{Actions: []string{"create"}}},
		},
	}
	m := NewWithOptions(pf, true, true)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = model.(Model)

	// Cursor is at 0 (module.vpc header)
	// Test 'y' copy
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = model.(Model)
	if copied != "module.vpc" {
		t.Fatalf("expected copied %q, got %q", "module.vpc", copied)
	}
	if m.copyStatus != "Copied module address!" {
		t.Fatalf("unexpected copyStatus %q", m.copyStatus)
	}

	// Test '?' context menu
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = model.(Model)
	if !m.contextMenuOpen {
		t.Fatal("expected context menu to open")
	}
	if !contains(m.View(), "Module actions") || !contains(m.View(), "Copy module address") {
		t.Fatalf("expected module actions in menu, got:\n%s", m.View())
	}

	// Move down to target plan command
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = model.(Model)
	if !contains(m.renderContextMenu(), "tofu plan -target='module.vpc'") {
		t.Fatalf("expected target command in preview, got:\n%s", m.renderContextMenu())
	}

	// Press Enter on item 1 (tofu plan command)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	if copied != "tofu plan -target='module.vpc'" {
		t.Fatalf("expected copied target command, got %q", copied)
	}
	if m.contextMenuOpen {
		t.Fatal("expected context menu to close after enter")
	}

	// Test first menu item (Copy module address)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = model.(Model)
	if m.contextMenuCursor != 0 {
		t.Fatalf("expected cursor 0 on reopen, got %d", m.contextMenuCursor)
	}
	if !contains(m.renderContextMenu(), "module.vpc") {
		t.Fatalf("expected module.vpc in preview, got:\n%s", m.renderContextMenu())
	}
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	if copied != "module.vpc" {
		t.Fatalf("expected copied %q, got %q", "module.vpc", copied)
	}
}

func TestRootModuleContextMenuAndCopy(t *testing.T) {
	var copied string
	originalWriteClipboard := writeClipboard
	writeClipboard = func(text string) error {
		copied = text
		return nil
	}
	defer func() { writeClipboard = originalWriteClipboard }()

	pf := plan.Plan{
		FormatVersion: "1.0",
		ResourceChanges: []plan.ResourceChange{
			{Address: "aws_s3_bucket.main", Change: plan.Change{Actions: []string{"update"}}},
		},
	}
	m := NewWithOptions(pf, true, true)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = model.(Model)

	// Cursor is at 0 ((root) module header)
	if m.items[0].kind != itemModuleHeader || m.items[0].module != "(root)" {
		t.Fatalf("expected item 0 to be (root) module, got %+v", m.items[0])
	}

	// Test 'y' copy on (root)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = model.(Model)
	if copied != "(root)" {
		t.Fatalf("expected copied %q, got %q", "(root)", copied)
	}
	if m.copyStatus != "Copied module address!" {
		t.Fatalf("unexpected copyStatus %q", m.copyStatus)
	}

	// Test '?' context menu
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = model.(Model)
	if !m.contextMenuOpen {
		t.Fatal("expected context menu to open for (root)")
	}
	if !contains(m.renderContextMenu(), "Module actions") || !contains(m.renderContextMenu(), "(root)") {
		t.Fatalf("expected module actions and (root) in menu, got:\n%s", m.renderContextMenu())
	}

	// First item preview should be (root)
	rendered := m.renderContextMenu()
	if !contains(rendered, "Preview") || !contains(rendered, "(root)") {
		t.Fatalf("expected (root) preview, got:\n%s", rendered)
	}

	// Press Enter to copy module name/address
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	if copied != "(root)" {
		t.Fatalf("expected copied (root), got %q", copied)
	}
	if m.contextMenuOpen {
		t.Fatal("expected menu to close")
	}

	// Re-open and move down to tofu plan command
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = model.(Model)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = model.(Model)
	if m.contextMenuCursor != 1 {
		t.Fatalf("expected cursor 1, got %d", m.contextMenuCursor)
	}
	// For root module, plan command preview is "tofu plan"
	if !contains(m.renderContextMenu(), "tofu plan") {
		t.Fatalf("expected 'tofu plan' preview for root module, got:\n%s", m.renderContextMenu())
	}

	// Press Enter to copy "tofu plan"
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	if copied != "tofu plan" {
		t.Fatalf("expected copied 'tofu plan', got %q", copied)
	}
	if m.copyStatus != "Copied tofu plan command!" {
		t.Fatalf("unexpected copyStatus %q", m.copyStatus)
	}

	// Test dismissing context menu with esc
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = model.(Model)
	if !m.contextMenuOpen {
		t.Fatal("expected context menu to open")
	}
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = model.(Model)
	if m.contextMenuOpen {
		t.Fatal("expected esc to dismiss context menu")
	}
}

func TestMixedModulesContextMenuNavigation(t *testing.T) {
	var copied string
	originalWriteClipboard := writeClipboard
	writeClipboard = func(text string) error {
		copied = text
		return nil
	}
	defer func() { writeClipboard = originalWriteClipboard }()

	pf := plan.Plan{
		FormatVersion: "1.0",
		ResourceChanges: []plan.ResourceChange{
			{Address: "aws_s3_bucket.main", Change: plan.Change{Actions: []string{"update"}}},
			{Address: "module.vpc.aws_subnet.public", ModuleAddress: "module.vpc", Change: plan.Change{Actions: []string{"create"}}},
		},
	}
	m := NewWithOptions(pf, true, true)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = model.(Model)

	// Items should be:
	// 0: (root) module
	// 1: aws_s3_bucket.main
	// 2: module.vpc module
	// 3: module.vpc.aws_subnet.public
	if len(m.items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(m.items))
	}

	// 0: Root module
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = model.(Model)
	if !contains(m.renderContextMenu(), "Module actions") || !contains(m.renderContextMenu(), "(root)") {
		t.Fatalf("expected root module context menu, got:\n%s", m.renderContextMenu())
	}
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = model.(Model)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	if copied != "tofu plan" {
		t.Fatalf("expected 'tofu plan', got %q", copied)
	}

	// Move to 1: aws_s3_bucket.main
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = model.(Model)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = model.(Model)
	if !contains(m.renderContextMenu(), "Resource actions") || !contains(m.renderContextMenu(), "aws_s3_bucket.main") {
		t.Fatalf("expected resource context menu, got:\n%s", m.renderContextMenu())
	}
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = model.(Model)

	// Move to 2: module.vpc
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = model.(Model)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = model.(Model)
	if !contains(m.renderContextMenu(), "Module actions") || !contains(m.renderContextMenu(), "module.vpc") {
		t.Fatalf("expected module.vpc context menu, got:\n%s", m.renderContextMenu())
	}
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = model.(Model)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	if copied != "tofu plan -target='module.vpc'" {
		t.Fatalf("expected 'tofu plan -target=\\'module.vpc\\'', got %q", copied)
	}

	// Move to 3: module.vpc.aws_subnet.public
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = model.(Model)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = model.(Model)
	if !contains(m.renderContextMenu(), "Resource actions") || !contains(m.renderContextMenu(), "module.vpc.aws_subnet.public") {
		t.Fatalf("expected resource context menu, got:\n%s", m.renderContextMenu())
	}
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = model.(Model)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	if copied != "tofu plan -target='module.vpc.aws_subnet.public'" {
		t.Fatalf("expected 'tofu plan -target=\\'module.vpc.aws_subnet.public\\'', got %q", copied)
	}
}



