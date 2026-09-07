package tui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	plan "github.com/ducva/tofu-diff/internal/plan/domain"
	"github.com/ducva/tofu-diff/internal/presentation/valuefmt"
)

var (
	clrCreate  = lipgloss.Color("10")
	clrUpdate  = lipgloss.Color("214")
	clrDelete  = lipgloss.Color("9")
	clrReplace = lipgloss.Color("141")
	clrMuted   = lipgloss.Color("240")
	clrAccent  = lipgloss.Color("39")
	clrBorder  = lipgloss.Color("238")
	clrSelBg   = lipgloss.Color("237")

	// JSON syntax highlight colours
	clrJSONStr  = lipgloss.Color("117") // light blue — string values
	clrJSONNum  = lipgloss.Color("214") // orange     — numbers
	clrJSONBool = lipgloss.Color("131") // rose       — bool / null

	// Unified diff line background tints (medium intensity)
	bgRemoved = lipgloss.Color("#3d1616") // medium dark red
	bgAdded   = lipgloss.Color("#163d16") // medium dark green
	clrMenuBg = lipgloss.Color("235")

	writeClipboard = clipboard.WriteAll
)

func actionColor(a plan.ActionType) lipgloss.Color {
	switch a {
	case plan.ActionCreate:
		return clrCreate
	case plan.ActionUpdate:
		return clrUpdate
	case plan.ActionDelete:
		return clrDelete
	case plan.ActionReplace:
		return clrReplace
	default:
		return clrMuted
	}
}

func actionSym(a plan.ActionType) string {
	switch a {
	case plan.ActionCreate:
		return "[+]"
	case plan.ActionUpdate:
		return "[~]"
	case plan.ActionDelete:
		return "[-]"
	case plan.ActionReplace:
		return "[±]"
	default:
		return "[?]"
	}
}

func actionName(a plan.ActionType) string {
	switch a {
	case plan.ActionCreate:
		return "create"
	case plan.ActionUpdate:
		return "update"
	case plan.ActionDelete:
		return "destroy"
	case plan.ActionReplace:
		return "replace"
	default:
		return "unknown"
	}
}

type focusPanel int

const (
	focusLeft focusPanel = iota
	focusRight
)

type ActionSummary struct {
	Create  int
	Update  int
	Delete  int
	Replace int
}

type contextMenuItem int

const (
	contextMenuCopyResourceName contextMenuItem = iota
	contextMenuCopyPlanCommand
)

var contextMenuLabels = []string{
	"Copy full resource name",
	"Copy tofu plan command",
}

type leftItemKind int

const (
	itemResource leftItemKind = iota
	itemModuleHeader
)

type leftItem struct {
	kind          leftItemKind
	module        string
	resourceIndex int
}

type Model struct {
	resources []plan.ResourceChange
	filtered  []int // indices into resources matching filter & search

	items            []leftItem
	expanded         map[int]bool    // keyed by resource index
	collapsedModules map[string]bool // keyed by module name
	cursor           int             // index in items
	focus            focusPanel

	searchInput textinput.Model
	searchMode  bool
	filters     map[plan.ActionType]bool

	leftVP  viewport.Model
	rightVP viewport.Model

	width  int
	height int
	ready  bool

	leftPanelWidthOffset int

	summary       ActionSummary
	copyStatus    string
	diffOnly      bool
	groupByModule bool

	contextMenuOpen   bool
	contextMenuCursor int
}

const (
	headerLines = 1
	searchLines = 1
	footerLines = 1
)

func (m *Model) panelHeight() int {
	h := m.height - headerLines - searchLines - footerLines
	if h < 1 {
		return 1
	}
	return h
}

func (m *Model) leftWidth() int {
	w := (m.width * 38 / 100) + m.leftPanelWidthOffset
	if w < 28 {
		w = 28
	}
	if w > m.width-20 {
		w = m.width - 20
	}
	return w
}

func (m *Model) rightWidth() int {
	return m.width - m.leftWidth()
}

func New(pf plan.Plan) Model {
	return NewWithOptions(pf, true, false)
}

func NewWithDiffOnly(pf plan.Plan, diffOnly bool) Model {
	return NewWithOptions(pf, diffOnly, false)
}

func NewWithOptions(pf plan.Plan, diffOnly, groupByModule bool) Model {
	var resources []plan.ResourceChange
	for _, rc := range pf.ResourceChanges {
		if rc.Change.NormalizedAction() != plan.ActionNoOp {
			resources = append(resources, rc)
		}
	}

	var summary ActionSummary
	for _, rc := range resources {
		switch rc.Change.NormalizedAction() {
		case plan.ActionCreate:
			summary.Create++
		case plan.ActionUpdate:
			summary.Update++
		case plan.ActionDelete:
			summary.Delete++
		case plan.ActionReplace:
			summary.Replace++
		}
	}

	ti := textinput.New()
	ti.Placeholder = "search resources..."
	ti.Prompt = ""
	ti.CharLimit = 100

	m := Model{
		resources:        resources,
		expanded:         make(map[int]bool),
		collapsedModules: make(map[string]bool),
		filters:          make(map[plan.ActionType]bool),
		searchInput:      ti,
		cursor:           0,
		summary:          summary,
		diffOnly:         diffOnly,
		groupByModule:    groupByModule,
	}
	m.refilter()
	return m
}

// WithDiffOnly returns a copy of the model with diffOnly set. Useful for
// applying a CLI flag after construction.
func (m Model) WithDiffOnly(v bool) Model {
	m.diffOnly = v
	return m
}

// SetDiffOnly sets diff-only mode on the model pointer.
func (m *Model) SetDiffOnly(v bool) {
	m.diffOnly = v
}

// WithGroupByModule returns a copy of the model with groupByModule set.
func (m Model) WithGroupByModule(v bool) Model {
	m.groupByModule = v
	m.refilter()
	return m
}

// SetGroupByModule sets group-by-module mode on the model pointer.
func (m *Model) SetGroupByModule(v bool) {
	m.groupByModule = v
	m.refilter()
}

func (m *Model) refilter() {
	query := strings.ToLower(m.searchInput.Value())
	anyFilter := false
	for _, v := range m.filters {
		if v {
			anyFilter = true
			break
		}
	}

	m.filtered = nil
	for i, rc := range m.resources {
		action := rc.Change.NormalizedAction()
		if anyFilter && !m.filters[action] {
			continue
		}
		if query != "" &&
			!strings.Contains(strings.ToLower(rc.Address), query) &&
			!strings.Contains(strings.ToLower(rc.Type), query) &&
			!strings.Contains(strings.ToLower(rc.ModuleName()), query) {
			continue
		}
		m.filtered = append(m.filtered, i)
	}

	m.rebuildItems()
}

func (m *Model) rebuildItems() {
	if !m.groupByModule {
		m.items = make([]leftItem, len(m.filtered))
		for i, ri := range m.filtered {
			m.items[i] = leftItem{
				kind:          itemResource,
				module:        m.resources[ri].ModuleName(),
				resourceIndex: ri,
			}
		}
	} else {
		modulesMap := make(map[string][]int)
		var moduleNames []string
		for _, ri := range m.filtered {
			mod := m.resources[ri].ModuleName()
			if _, exists := modulesMap[mod]; !exists {
				moduleNames = append(moduleNames, mod)
			}
			modulesMap[mod] = append(modulesMap[mod], ri)
		}

		sort.SliceStable(moduleNames, func(i, j int) bool {
			if moduleNames[i] == "(root)" {
				return true
			}
			if moduleNames[j] == "(root)" {
				return false
			}
			return moduleNames[i] < moduleNames[j]
		})

		m.items = nil
		for _, mod := range moduleNames {
			m.items = append(m.items, leftItem{
				kind:   itemModuleHeader,
				module: mod,
			})
			if !m.collapsedModules[mod] {
				for _, ri := range modulesMap[mod] {
					m.items = append(m.items, leftItem{
						kind:          itemResource,
						module:        mod,
						resourceIndex: ri,
					})
				}
			}
		}
	}

	if m.cursor >= len(m.items) {
		if len(m.items) > 0 {
			m.cursor = len(m.items) - 1
		} else {
			m.cursor = 0
		}
	}
}

func (m *Model) selectedIndex() int {
	if len(m.items) == 0 || m.cursor >= len(m.items) {
		return -1
	}
	item := m.items[m.cursor]
	if item.kind != itemResource {
		return -1
	}
	return item.resourceIndex
}

func (m *Model) selectedItem() *leftItem {
	if len(m.items) == 0 || m.cursor >= len(m.items) {
		return nil
	}
	return &m.items[m.cursor]
}

func (m *Model) moduleResourceCount(mod string) int {
	count := 0
	for _, ri := range m.filtered {
		if m.resources[ri].ModuleName() == mod {
			count++
		}
	}
	return count
}

func (m *Model) refreshViewports() {
	m.leftVP.SetContent(m.buildLeftContent())
	m.rightVP.SetContent(m.buildRightContent())
}

// scrollLeftToCursor adjusts leftVP.YOffset so the cursor row is visible.
func (m *Model) scrollLeftToCursor() {
	line := 0
	for vi := 0; vi < m.cursor && vi < len(m.items); vi++ {
		item := m.items[vi]
		line++
		if item.kind == itemResource && m.expanded[item.resourceIndex] {
			diffs := plan.DiffAttributes(m.resources[item.resourceIndex].Change)
			if len(diffs) == 0 {
				line++
			} else {
				line += len(diffs)
			}
		}
	}
	if line < m.leftVP.YOffset {
		m.leftVP.SetYOffset(line)
	} else if line >= m.leftVP.YOffset+m.leftVP.Height {
		m.leftVP.SetYOffset(line - m.leftVP.Height + 1)
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		ph := m.panelHeight()
		lw := m.leftWidth()
		rw := m.rightWidth()
		vh := max(1, ph-2)
		leftVW := max(1, lw-2)
		rightVW := max(1, rw-2)
		m.searchInput.Width = m.width - 10
		if !m.ready {
			m.leftVP = viewport.New(leftVW, vh)
			m.rightVP = viewport.New(rightVW, vh)
			m.ready = true
		} else {
			m.leftVP.Width = leftVW
			m.leftVP.Height = vh
			m.rightVP.Width = rightVW
			m.rightVP.Height = vh
		}
		m.refreshViewports()
		return m, nil

	case tea.KeyMsg:
		if !m.ready {
			return m, nil
		}
		if msg.String() != "y" {
			m.copyStatus = ""
		}
		if m.searchMode {
			return m.handleSearchKey(msg)
		}
		return m.handleNormalKey(msg)
	}
	return m, nil
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		m.searchMode = false
		m.searchInput.Blur()
		m.refreshViewports()
		return m, nil
	default:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.refilter()
		m.rightVP.GotoTop()
		m.refreshViewports()
		return m, cmd
	}
}

func (m Model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.contextMenuOpen {
		return m.handleContextMenuKey(msg)
	}

	switch msg.String() {
	case "[":
		targetW := m.leftWidth() - 2
		if targetW < 28 {
			targetW = 28
		}
		m.leftPanelWidthOffset = targetW - (m.width * 38 / 100)
		m.leftVP.Width = max(1, targetW-2)
		m.rightVP.Width = max(1, (m.width-targetW)-2)
		m.refreshViewports()
		return m, nil

	case "]":
		targetW := m.leftWidth() + 2
		maxW := m.width - 20
		if targetW > maxW {
			targetW = maxW
		}
		if targetW < 28 {
			targetW = 28
		}
		m.leftPanelWidthOffset = targetW - (m.width * 38 / 100)
		m.leftVP.Width = max(1, targetW-2)
		m.rightVP.Width = max(1, (m.width-targetW)-2)
		m.refreshViewports()
		return m, nil

	case "/":
		m.searchMode = true
		m.searchInput.Focus()

	case "?":
		if m.selectedItem() != nil {
			m.contextMenuOpen = true
			m.contextMenuCursor = 0
			m.refreshViewports()
		}

	case "y":
		if m.focus == focusLeft {
			if sel := m.selectedItem(); sel != nil {
				var text string
				var successMsg string
				if sel.kind == itemModuleHeader {
					if sel.module == "(root)" {
						m.copyStatus = "(root) module has no address"
						m.refreshViewports()
						return m, nil
					}
					text = sel.module
					successMsg = "Copied module address!"
				} else {
					rc := m.resources[sel.resourceIndex]
					text = rc.Address
					successMsg = "Copied address!"
				}
				if err := writeClipboard(text); err != nil {
					m.copyStatus = "Copy failed: " + err.Error()
				} else {
					m.copyStatus = successMsg
				}
				m.refreshViewports()
			}
		}

	case "up", "k":
		if m.focus == focusLeft {
			if m.cursor > 0 {
				m.cursor--
				m.scrollLeftToCursor()
				m.rightVP.GotoTop()
				m.refreshViewports()
			}
		} else {
			m.rightVP.ScrollUp(1)
		}

	case "down", "j":
		if m.focus == focusLeft {
			if m.cursor < len(m.items)-1 {
				m.cursor++
				m.scrollLeftToCursor()
				m.rightVP.GotoTop()
				m.refreshViewports()
			}
		} else {
			m.rightVP.ScrollDown(1)
		}

	case "enter":
		if m.focus == focusLeft {
			if sel := m.selectedItem(); sel != nil && sel.kind == itemModuleHeader {
				m.collapsedModules[sel.module] = !m.collapsedModules[sel.module]
				m.rebuildItems()
				m.refreshViewports()
			}
		}

	case " ":
		if sel := m.selectedItem(); sel != nil {
			if sel.kind == itemModuleHeader {
				m.collapsedModules[sel.module] = !m.collapsedModules[sel.module]
				m.rebuildItems()
			} else {
				ri := sel.resourceIndex
				m.expanded[ri] = !m.expanded[ri]
			}
			m.refreshViewports()
		}

	case "tab":
		if m.focus == focusLeft {
			m.focus = focusRight
		} else {
			m.focus = focusLeft
		}
		m.refreshViewports()

	case "E":
		m.collapsedModules = make(map[string]bool)
		for _, ri := range m.filtered {
			m.expanded[ri] = true
		}
		m.rebuildItems()
		m.refreshViewports()

	case "C":
		if len(m.expanded) > 0 {
			m.expanded = make(map[int]bool)
		} else if m.groupByModule {
			for _, item := range m.items {
				if item.kind == itemModuleHeader {
					m.collapsedModules[item.module] = true
				}
			}
			m.rebuildItems()
		}
		m.refreshViewports()

	case "1":
		m.filters[plan.ActionCreate] = !m.filters[plan.ActionCreate]
		m.refilter()
		m.rightVP.GotoTop()
		m.refreshViewports()

	case "2":
		m.filters[plan.ActionUpdate] = !m.filters[plan.ActionUpdate]
		m.refilter()
		m.rightVP.GotoTop()
		m.refreshViewports()

	case "3":
		m.filters[plan.ActionDelete] = !m.filters[plan.ActionDelete]
		m.refilter()
		m.rightVP.GotoTop()
		m.refreshViewports()

	case "4":
		m.filters[plan.ActionReplace] = !m.filters[plan.ActionReplace]
		m.refilter()
		m.rightVP.GotoTop()
		m.refreshViewports()

	case "o", "O":
		m.diffOnly = !m.diffOnly
		if m.diffOnly {
			m.copyStatus = "Diff-only: showing changed lines only"
		} else {
			m.copyStatus = "Diff: showing full context"
		}
		m.rightVP.GotoTop()
		m.refreshViewports()

	case "m", "M":
		m.groupByModule = !m.groupByModule
		if m.groupByModule {
			m.copyStatus = "Group by module: enabled"
		} else {
			m.copyStatus = "Group by module: disabled"
		}
		m.rebuildItems()
		m.rightVP.GotoTop()
		m.refreshViewports()

	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleContextMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "?":
		m.contextMenuOpen = false
		m.refreshViewports()
	case "up", "k":
		if m.contextMenuCursor > 0 {
			m.contextMenuCursor--
		}
	case "down", "j":
		if m.contextMenuCursor < len(contextMenuLabels)-1 {
			m.contextMenuCursor++
		}
	case "enter":
		m.executeContextMenuItem(contextMenuItem(m.contextMenuCursor))
	}
	return m, nil
}

func (m *Model) executeContextMenuItem(item contextMenuItem) {
	sel := m.selectedItem()
	if sel == nil {
		m.contextMenuOpen = false
		return
	}

	var text string
	var status string

	if sel.kind == itemModuleHeader {
		if sel.module == "(root)" {
			m.copyStatus = "(root) module has no address"
			m.contextMenuOpen = false
			m.refreshViewports()
			return
		}
		text = sel.module
		status = "Copied module address!"
		if item == contextMenuCopyPlanCommand {
			text = tofuPlanTargetCommand(sel.module)
			status = "Copied tofu plan command!"
		}
	} else {
		address := m.resources[sel.resourceIndex].Address
		text = address
		status = "Copied resource name!"
		if item == contextMenuCopyPlanCommand {
			text = tofuPlanTargetCommand(address)
			status = "Copied tofu plan command!"
		}
	}

	if err := writeClipboard(text); err != nil {
		m.copyStatus = "Copy failed: " + err.Error()
	} else {
		m.copyStatus = status
	}
	m.contextMenuOpen = false
	m.refreshViewports()
}

func tofuPlanTargetCommand(address string) string {
	return "tofu plan -target=" + shellQuote(address)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// padTo pads s to exactly width display cells (ANSI-aware).
func padTo(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func (m *Model) buildLeftContent() string {
	lw := max(1, m.leftWidth()-2)

	if len(m.items) == 0 {
		msg := "  No resources match."
		if len(m.resources) == 0 {
			msg = "  No changes."
		}
		return lipgloss.NewStyle().Foreground(clrMuted).Render(msg)
	}

	var sb strings.Builder
	for vi, item := range m.items {
		selected := vi == m.cursor

		if item.kind == itemModuleHeader {
			collapsed := m.collapsedModules[item.module]
			toggle := lipgloss.NewStyle().Foreground(clrAccent).Render(map[bool]string{true: "▶", false: "▼"}[collapsed])
			count := m.moduleResourceCount(item.module)
			badge := lipgloss.NewStyle().Foreground(clrMuted).Render(fmt.Sprintf("(%d)", count))

			modTitle := item.module
			const prefixW = 6
			badgeW := lipgloss.Width(badge) + 1
			maxTitleW := lw - prefixW - badgeW
			if maxTitleW > 3 && len(modTitle) > maxTitleW {
				modTitle = modTitle[:maxTitleW-3] + "..."
			}

			modStyle := lipgloss.NewStyle().Foreground(clrAccent).Bold(true)
			line := fmt.Sprintf(" %s %s %s", toggle, modStyle.Render(modTitle), badge)
			if selected {
				style := lipgloss.NewStyle().Background(clrSelBg)
				if m.focus == focusLeft {
					style = style.Bold(true)
				}
				line = style.Render(padTo(line, lw))
			} else {
				line = padTo(line, lw)
			}
			sb.WriteString(line + "\n")
			continue
		}

		ri := item.resourceIndex
		rc := m.resources[ri]
		action := rc.Change.NormalizedAction()
		expanded := m.expanded[ri]

		toggle := lipgloss.NewStyle().Foreground(clrMuted).Render(map[bool]string{true: "▼", false: "▶"}[expanded])
		sym := lipgloss.NewStyle().Foreground(actionColor(action)).Bold(true).Render(actionSym(action))

		addr := rc.Address
		indent := "  "
		if m.groupByModule {
			addr = rc.RelativeAddress()
			indent = "    "
		}

		prefixWidth := len(indent) + 7
		addrMax := lw - prefixWidth
		if len(addr) > addrMax && addrMax > 3 {
			addr = addr[:addrMax-3] + "..."
		}

		line := fmt.Sprintf("%s%s %s %s", indent, toggle, sym, addr)
		if selected {
			style := lipgloss.NewStyle().Background(clrSelBg)
			if m.focus == focusLeft {
				style = style.Bold(true)
			}
			line = style.Render(padTo(line, lw))
		} else {
			line = padTo(line, lw)
		}
		sb.WriteString(line + "\n")

		if expanded {
			attrIndent := "    "
			if m.groupByModule {
				attrIndent = "      "
			}
			diffs := plan.DiffAttributes(rc.Change)
			if len(diffs) == 0 {
				detail := padTo(lipgloss.NewStyle().Foreground(clrMuted).Italic(true).Render(attrIndent+"(no attribute changes)"), lw)
				sb.WriteString(detail + "\n")
			} else {
				for _, d := range diffs {
					key := lipgloss.NewStyle().Foreground(clrAccent).Render(d.Key)
					beforeDisplay := valuefmt.Format(d.BeforeRaw, d.BeforeSensitive)
					before := lipgloss.NewStyle().Foreground(clrDelete).Render(beforeDisplay)
					arrow := lipgloss.NewStyle().Foreground(clrMuted).Render(" → ")
					var after string
					if d.IsUnknownAfter {
						after = lipgloss.NewStyle().Foreground(clrReplace).Italic(true).Render("(known after apply)")
					} else {
						afterDisplay := valuefmt.Format(d.AfterRaw, d.AfterSensitive)
						after = lipgloss.NewStyle().Foreground(clrCreate).Render(afterDisplay)
					}
					detail := padTo(attrIndent+key+"  "+before+arrow+after, lw)
					sb.WriteString(detail + "\n")
				}
			}
		}
	}
	return sb.String()
}

func (m *Model) buildRightContent() string {
	rw := max(1, m.rightWidth()-2)
	sel := m.selectedItem()

	if sel == nil {
		return lipgloss.NewStyle().Foreground(clrMuted).Render("  Select a resource or module to inspect")
	}

	if sel.kind == itemModuleHeader {
		return m.buildModuleRightContent(sel.module)
	}

	ri := sel.resourceIndex
	rc := m.resources[ri]
	action := rc.Change.NormalizedAction()

	hr := lipgloss.NewStyle().Foreground(clrBorder).Render(" " + strings.Repeat("─", max(0, rw-2)))
	contentW := rw - 3 // reserve 3 chars for the gutter (" − " / " + " / "   ")

	var sb strings.Builder

	// Metadata
	sb.WriteString(lipgloss.NewStyle().Foreground(clrMuted).Bold(true).Render(" RESOURCE") + "\n")
	sb.WriteString(hr + "\n")
	for _, row := range [][2]string{
		{" address", rc.Address},
		{" type   ", rc.Type},
		{" name   ", rc.Name},
		{" action ", lipgloss.NewStyle().Foreground(actionColor(action)).Bold(true).Render(actionName(action))},
	} {
		k := lipgloss.NewStyle().Foreground(clrMuted).Render(row[0] + " ")
		sb.WriteString(k + row[1] + "\n")
	}
	sb.WriteString("\n")

	// Diffs
	diffs := plan.DiffAttributes(rc.Change)
	sb.WriteString(lipgloss.NewStyle().Foreground(clrMuted).Bold(true).Render(fmt.Sprintf(" CHANGES (%d)", len(diffs))) + "\n")
	sb.WriteString(hr + "\n")

	if len(diffs) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(clrMuted).Italic(true).Render("  (no attribute changes)") + "\n")
		return sb.String()
	}

	for _, d := range diffs {
		bPlain, bRich := valueToLinesRaw(d.BeforeRaw, d.BeforeSensitive, contentW, true)
		var aPlain, aRich []string
		if d.IsUnknownAfter {
			aPlain = []string{"(known after apply)"}
			aRich = []string{lipgloss.NewStyle().Foreground(clrReplace).Italic(true).Render("(known after apply)")}
		} else {
			aPlain, aRich = valueToLinesRaw(d.AfterRaw, d.AfterSensitive, contentW, false)
		}

		ulines := lcsUnify(bPlain, bRich, aPlain, aRich)
		if m.diffOnly {
			filtered := filterToDiffOnly(ulines)
			if len(filtered) == 0 {
				continue
			}
			ulines = filtered
		}

		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(clrAccent).Bold(true).Render(" "+d.Key) + "\n")

		for _, ul := range ulines {
			sb.WriteString(renderULine(ul, rw) + "\n")
		}
	}

	return sb.String()
}

func (m *Model) buildModuleRightContent(mod string) string {
	rw := max(1, m.rightWidth()-2)
	hr := lipgloss.NewStyle().Foreground(clrBorder).Render(" " + strings.Repeat("─", max(0, rw-2)))

	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Foreground(clrMuted).Bold(true).Render(" MODULE") + "\n")
	sb.WriteString(hr + "\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(clrMuted).Render(" module  ") + mod + "\n")

	var modResources []plan.ResourceChange
	var s ActionSummary
	for _, rc := range m.resources {
		if rc.ModuleName() == mod {
			modResources = append(modResources, rc)
			switch rc.Change.NormalizedAction() {
			case plan.ActionCreate:
				s.Create++
			case plan.ActionUpdate:
				s.Update++
			case plan.ActionDelete:
				s.Delete++
			case plan.ActionReplace:
				s.Replace++
			}
		}
	}

	var summaryParts []string
	if s.Create > 0 {
		summaryParts = append(summaryParts, lipgloss.NewStyle().Foreground(clrCreate).Render(fmt.Sprintf("%d to create", s.Create)))
	}
	if s.Update > 0 {
		summaryParts = append(summaryParts, lipgloss.NewStyle().Foreground(clrUpdate).Render(fmt.Sprintf("%d to update", s.Update)))
	}
	if s.Delete > 0 {
		summaryParts = append(summaryParts, lipgloss.NewStyle().Foreground(clrDelete).Render(fmt.Sprintf("%d to destroy", s.Delete)))
	}
	if s.Replace > 0 {
		summaryParts = append(summaryParts, lipgloss.NewStyle().Foreground(clrReplace).Render(fmt.Sprintf("%d to replace", s.Replace)))
	}
	changesStr := strings.Join(summaryParts, ", ")
	if changesStr == "" {
		changesStr = "no changes"
	}
	sb.WriteString(lipgloss.NewStyle().Foreground(clrMuted).Render(" changes ") + fmt.Sprintf("%d (%s)\n\n", len(modResources), changesStr))

	sb.WriteString(lipgloss.NewStyle().Foreground(clrMuted).Bold(true).Render(fmt.Sprintf(" RESOURCES (%d)", len(modResources))) + "\n")
	sb.WriteString(hr + "\n")

	for _, rc := range modResources {
		act := rc.Change.NormalizedAction()
		sym := lipgloss.NewStyle().Foreground(actionColor(act)).Bold(true).Render(actionSym(act))
		sb.WriteString(fmt.Sprintf(" %s %s\n", sym, rc.Address))
	}

	return sb.String()
}

// lineKind classifies a line in a unified diff.
type lineKind uint8

const (
	lineKindSame    lineKind = iota
	lineKindRemoved          // present in before, not after
	lineKindAdded            // present in after, not before
)

// uLine is one line in a unified diff output.
type uLine struct {
	kind    lineKind
	content string // plain text for same lines, ANSI-highlighted for removed/added
}

// valueToLinesRaw is like valueToLines but operates on the full json.RawMessage,
// bypassing the compact truncation used in resource summaries. Used by the
// right panel to show complete attribute values.
func valueToLinesRaw(raw json.RawMessage, sensitive bool, maxW int, isBefore bool) (plain, rich []string) {
	scalarClr := clrCreate
	if isBefore {
		scalarClr = clrDelete
	}
	if sensitive {
		r := lipgloss.NewStyle().Foreground(clrMuted).Italic(true).Render("(sensitive)")
		return []string{"(sensitive)"}, []string{r}
	}
	if raw == nil || string(raw) == "null" {
		return []string{"null"}, []string{lipgloss.NewStyle().Foreground(clrMuted).Render("null")}
	}

	// JSON string: unmarshal to get the unquoted value, then check if it is
	// itself a JSON object/array (some providers encode nested JSON as strings).
	if len(raw) > 0 && raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			trimmed := strings.TrimSpace(s)
			if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
				var v interface{}
				if err2 := json.Unmarshal([]byte(trimmed), &v); err2 == nil {
					return prettyJSONLines(v, maxW)
				}
			}
			plainLines := wrapTextSoft(s, maxW)
			richLines := make([]string, len(plainLines))
			for idx, pl := range plainLines {
				richLines[idx] = lipgloss.NewStyle().Foreground(scalarClr).Render(pl)
			}
			return plainLines, richLines
		}
	}

	// JSON object or array.
	if len(raw) > 0 && (raw[0] == '{' || raw[0] == '[') {
		var v interface{}
		if err := json.Unmarshal(raw, &v); err == nil {
			return prettyJSONLines(v, maxW)
		}
	}

	s := string(raw)
	plainLines := wrapTextSoft(s, maxW)
	richLines := make([]string, len(plainLines))
	for idx, pl := range plainLines {
		richLines[idx] = lipgloss.NewStyle().Foreground(scalarClr).Render(pl)
	}
	return plainLines, richLines
}

// prettyJSONLines marshals v as indented JSON, wraps long lines, and returns
// parallel plain/rich line slices.
func prettyJSONLines(v interface{}, maxW int) (plain, rich []string) {
	plainBytes, _ := json.MarshalIndent(v, "", "  ")
	wrapped := wrapPlainLines(strings.Split(string(plainBytes), "\n"), maxW)
	richLines := make([]string, len(wrapped))
	for i, l := range wrapped {
		richLines[i] = highlightJSONLine(l)
	}
	return wrapped, richLines
}

// valueToLines converts a raw diff display string into parallel slices of
// plain text and ANSI-highlighted text, wrapping long lines to maxW chars.
func valueToLines(raw string, maxW int, isBefore bool) (plain, rich []string) {
	scalarClr := clrCreate
	if isBefore {
		scalarClr = clrDelete
	}

	switch raw {
	case "null":
		return []string{"null"}, []string{lipgloss.NewStyle().Foreground(clrMuted).Render("null")}
	case "(sensitive)":
		r := lipgloss.NewStyle().Foreground(clrMuted).Italic(true).Render("(sensitive)")
		return []string{"(sensitive)"}, []string{r}
	case "(known after apply)":
		r := lipgloss.NewStyle().Foreground(clrReplace).Italic(true).Render("(known after apply)")
		return []string{"(known after apply)"}, []string{r}
	}

	trimmed := strings.TrimSpace(raw)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		var v interface{}
		if err := json.Unmarshal([]byte(trimmed), &v); err == nil {
			plainBytes, _ := json.MarshalIndent(v, "", "  ")
			wrapped := wrapPlainLines(strings.Split(string(plainBytes), "\n"), maxW)
			richLines := make([]string, len(wrapped))
			for i, l := range wrapped {
				richLines[i] = highlightJSONLine(l)
			}
			return wrapped, richLines
		}
	}

	plainLines := wrapTextSoft(raw, maxW)
	richLines := make([]string, len(plainLines))
	for idx, pl := range plainLines {
		richLines[idx] = lipgloss.NewStyle().Foreground(scalarClr).Render(pl)
	}
	return plainLines, richLines
}

// lcsUnify computes a unified diff between before and after using LCS.
// Same lines store plain text; removed/added lines store ANSI-highlighted text.
func lcsUnify(bPlain, bRich, aPlain, aRich []string) []uLine {
	m, n := len(bPlain), len(aPlain)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if bPlain[i] == aPlain[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	var out []uLine
	i, j := 0, 0
	for i < m && j < n {
		if bPlain[i] == aPlain[j] {
			out = append(out, uLine{lineKindSame, bPlain[i]})
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			out = append(out, uLine{lineKindRemoved, bRich[i]})
			i++
		} else {
			out = append(out, uLine{lineKindAdded, aRich[j]})
			j++
		}
	}
	for ; i < m; i++ {
		out = append(out, uLine{lineKindRemoved, bRich[i]})
	}
	for ; j < n; j++ {
		out = append(out, uLine{lineKindAdded, aRich[j]})
	}
	return out
}

// filterToDiffOnly returns only added/removed lines, dropping context (same) lines.
func filterToDiffOnly(in []uLine) []uLine {
	out := make([]uLine, 0, len(in))
	for _, ul := range in {
		if ul.kind == lineKindRemoved || ul.kind == lineKindAdded {
			out = append(out, ul)
		}
	}
	return out
}

// renderULine renders one unified diff line at the given display width.
func renderULine(ul uLine, width int) string {
	switch ul.kind {
	case lineKindRemoved:
		g := lipgloss.NewStyle().Foreground(clrDelete).Bold(true).Render(" − ")
		return lipgloss.NewStyle().Background(bgRemoved).Render(padTo(g+ul.content, width))
	case lineKindAdded:
		g := lipgloss.NewStyle().Foreground(clrCreate).Bold(true).Render(" + ")
		return lipgloss.NewStyle().Background(bgAdded).Render(padTo(g+ul.content, width))
	default: // same — dimmed plain text, no background
		g := lipgloss.NewStyle().Foreground(clrMuted).Render("   ")
		return lipgloss.NewStyle().Foreground(clrMuted).Render(padTo(g+ul.content, width))
	}
}

// wrapPlainLines hard-wraps plain-text lines to maxW characters.
// Continuation lines carry the same leading indent plus two extra spaces.
func wrapPlainLines(lines []string, maxW int) []string {
	if maxW <= 4 {
		return lines
	}
	var out []string
	for _, line := range lines {
		if len(line) <= maxW {
			out = append(out, line)
			continue
		}
		stripped := strings.TrimLeft(line, " ")
		indentN := len(line) - len(stripped)
		cont := strings.Repeat(" ", indentN+2)
		for len(line) > maxW {
			out = append(out, line[:maxW])
			line = cont + strings.TrimLeft(line[maxW:], " ")
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func wrapLineSoftPreserveSpace(line string, maxW int) []string {
	if maxW <= 0 {
		return []string{line}
	}
	if len(line) <= maxW {
		return []string{line}
	}

	var lines []string
	runes := []rune(line)
	start := 0

	for start < len(runes) {
		end := start + maxW
		if end >= len(runes) {
			lines = append(lines, string(runes[start:]))
			break
		}

		// Look for a space or tab character backwards from the end
		splitAt := -1
		for i := end; i > start; i-- {
			if runes[i] == ' ' || runes[i] == '\t' {
				splitAt = i
				break
			}
		}

		if splitAt != -1 {
			lines = append(lines, string(runes[start:splitAt]))
			start = splitAt + 1
		} else {
			// No space found; hard wrap
			lines = append(lines, string(runes[start:end]))
			start = end
		}
	}
	return lines
}

func wrapTextSoft(text string, maxW int) []string {
	if maxW <= 0 {
		return []string{text}
	}
	var lines []string
	rawLines := strings.Split(text, "\n")
	for _, l := range rawLines {
		wrapped := wrapLineSoftPreserveSpace(l, maxW)
		if len(wrapped) == 0 {
			lines = append(lines, "")
		} else {
			lines = append(lines, wrapped...)
		}
	}
	return lines
}

// highlightJSONLine applies ANSI colours to a single line of json.MarshalIndent output.
func highlightJSONLine(line string) string {
	braceS := lipgloss.NewStyle().Foreground(clrBorder)
	keyS := lipgloss.NewStyle().Foreground(clrAccent)
	strS := lipgloss.NewStyle().Foreground(clrJSONStr)
	numS := lipgloss.NewStyle().Foreground(clrJSONNum)
	boolS := lipgloss.NewStyle().Foreground(clrJSONBool)

	stripped := strings.TrimLeft(line, " ")
	indent := line[:len(line)-len(stripped)]

	// Strip trailing comma for classification, restore it at the end.
	trailingComma := ""
	val := stripped
	if strings.HasSuffix(val, ",") {
		val = val[:len(val)-1]
		trailingComma = braceS.Render(",")
	}

	// Pure structural characters.
	switch val {
	case "{", "}", "[", "]", "{}", "[]":
		return indent + braceS.Render(val) + trailingComma
	}

	// Object entry: starts with a quoted key followed by ": ".
	if len(val) > 0 && val[0] == '"' {
		// Find closing quote of the key (simple scan; keys from MarshalIndent are safe).
		keyEnd := strings.Index(val[1:], `"`) + 1
		if keyEnd > 0 && keyEnd+2 <= len(val) && val[keyEnd+1] == ':' {
			key := val[:keyEnd+1]
			rest := strings.TrimPrefix(val[keyEnd+1:], ": ")
			return indent + keyS.Render(key) + braceS.Render(": ") + colorJSONToken(rest, strS, numS, boolS, braceS) + trailingComma
		}
		// Array string element.
		return indent + strS.Render(val) + trailingComma
	}

	return indent + colorJSONToken(val, strS, numS, boolS, braceS) + trailingComma
}

// colorJSONToken colours a standalone JSON value token (no key prefix).
func colorJSONToken(s string, strS, numS, boolS, braceS lipgloss.Style) string {
	switch s {
	case "true", "false", "null":
		return boolS.Render(s)
	case "{", "}", "[", "]", "{}", "[]":
		return braceS.Render(s)
	}
	if len(s) == 0 {
		return s
	}
	switch s[0] {
	case '"':
		return strS.Render(s)
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return numS.Render(s)
	}
	return s
}


func (m Model) View() string {
	if !m.ready {
		return ""
	}

	leftBorderColor := clrBorder
	rightBorderColor := clrBorder
	if m.focus == focusLeft {
		leftBorderColor = clrAccent
	} else {
		rightBorderColor = clrAccent
	}

	leftStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(leftBorderColor)
	rightStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(rightBorderColor)

	leftBox := leftStyle.Render(m.leftVP.View())
	rightBox := rightStyle.Render(m.rightVP.View())

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)

	view := m.renderHeader() + "\n" +
		m.renderSearchBar() + "\n" +
		panels + "\n" +
		m.renderFooter()
	if m.contextMenuOpen {
		return overlayCentered(view, m.renderContextMenu(), m.width, m.height)
	}
	return view
}

func overlayCentered(base, overlay string, width, height int) string {
	overlayWidth := lipgloss.Width(overlay)
	overlayHeight := lipgloss.Height(overlay)
	left := max(0, (width-overlayWidth)/2)
	top := max(0, (height-overlayHeight)/2)

	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")
	for i, overlayLine := range overlayLines {
		row := top + i
		if row < 0 || row >= len(baseLines) {
			continue
		}

		baseLine := baseLines[row]
		baseLines[row] = cutANSI(baseLine, 0, left) +
			padMenuLine(overlayLine, overlayWidth) +
			cutANSI(baseLine, left+overlayWidth, width)
	}
	return strings.Join(baseLines, "\n")
}

func padMenuLine(line string, width int) string {
	padding := width - lipgloss.Width(line)
	if padding <= 0 {
		return line
	}
	return line + lipgloss.NewStyle().Background(clrMenuBg).Render(strings.Repeat(" ", padding))
}

// cutANSI returns the cell range [left, right) without breaking ANSI escape
// sequences. It is used to replace a rectangular section of the rendered TUI.
func cutANSI(s string, left, right int) string {
	if right <= left {
		return ""
	}

	var out strings.Builder
	cell := 0
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			start := i
			i++
			if i < len(s) && s[i] == '[' {
				i++
				for i < len(s) {
					b := s[i]
					i++
					if b >= '@' && b <= '~' {
						break
					}
				}
			} else if i < len(s) {
				i++
			}
			if cell >= left && cell < right {
				out.WriteString(s[start:i])
			}
			continue
		}

		_, size := utf8.DecodeRuneInString(s[i:])
		if size == 0 {
			break
		}
		char := s[i : i+size]
		charWidth := lipgloss.Width(char)
		if cell >= left && cell+charWidth <= right {
			out.WriteString(char)
		}
		cell += charWidth
		i += size
	}
	return out.String()
}

func (m *Model) renderContextMenu() string {
	title := "Resource actions"
	target := "(no resource selected)"
	label0 := "Copy full resource name"
	label1 := "Copy tofu plan command"

	if sel := m.selectedItem(); sel != nil {
		if sel.kind == itemModuleHeader {
			title = "Module actions"
			target = sel.module
			label0 = "Copy module address"
		} else {
			target = m.resources[sel.resourceIndex].Address
		}
	}

	labels := []string{label0, label1}

	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render(title))
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(clrMuted).Render(target))
	sb.WriteString("\n\n")
	for i, label := range labels {
		line := "  " + label
		if i == m.contextMenuCursor {
			line = lipgloss.NewStyle().Background(clrSelBg).Bold(true).Render("> " + label)
		}
		sb.WriteString(line)
		if i < len(labels)-1 {
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(clrMuted).Render("↑↓/jk select · enter copy · esc close"))
	sb.WriteString("\n\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(clrMuted).Bold(true).Render("Preview"))
	sb.WriteString("\n")
	previewStyle := lipgloss.NewStyle().Foreground(clrMuted)
	for _, line := range wrapTextSoft(m.contextMenuPreview(), 56) {
		sb.WriteString(previewStyle.Render("  " + line))
		sb.WriteString("\n")
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(clrAccent).
		Background(clrMenuBg).
		Padding(1, 2).
		Render(sb.String())
}

func (m *Model) contextMenuPreview() string {
	sel := m.selectedItem()
	if sel == nil {
		return ""
	}

	if sel.kind == itemModuleHeader {
		if sel.module == "(root)" {
			return "(root) module has no target address"
		}
		if contextMenuItem(m.contextMenuCursor) == contextMenuCopyPlanCommand {
			return tofuPlanTargetCommand(sel.module)
		}
		return sel.module
	}

	address := m.resources[sel.resourceIndex].Address
	if contextMenuItem(m.contextMenuCursor) == contextMenuCopyPlanCommand {
		return tofuPlanTargetCommand(address)
	}
	return address
}

func (m *Model) renderHeader() string {
	s := m.summary
	var parts []string
	if s.Create > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(clrCreate).Render(fmt.Sprintf("  %d to create", s.Create)))
	}
	if s.Update > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(clrUpdate).Render(fmt.Sprintf("  %d to update", s.Update)))
	}
	if s.Delete > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(clrDelete).Render(fmt.Sprintf("  %d to destroy", s.Delete)))
	}
	if s.Replace > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(clrReplace).Render(fmt.Sprintf("  %d to replace", s.Replace)))
	}

	var base string
	if len(parts) == 0 {
		base = lipgloss.NewStyle().Foreground(clrMuted).Render("  No changes. Infrastructure is up-to-date.")
	} else {
		total, all := len(m.filtered), len(m.resources)
		if total < all {
			parts = append(parts, lipgloss.NewStyle().Foreground(clrMuted).Render(fmt.Sprintf("  (%d/%d shown)", total, all)))
		}
		base = strings.Join(parts, "")
	}

	if m.diffOnly {
		badge := lipgloss.NewStyle().Foreground(clrAccent).Bold(true).Reverse(true).Render(" DIFF ONLY ")
		base = base + "  " + badge
	}
	if m.groupByModule {
		badge := lipgloss.NewStyle().Foreground(clrAccent).Bold(true).Reverse(true).Render(" MODULES ")
		base = base + "  " + badge
	}

	if m.copyStatus != "" {
		style := lipgloss.NewStyle().Foreground(clrAccent).Bold(true)
		if strings.Contains(strings.ToLower(m.copyStatus), "failed") {
			style = lipgloss.NewStyle().Foreground(clrDelete).Bold(true)
		}
		return base + "  " + style.Render(m.copyStatus)
	}
	return base
}

func (m *Model) renderSearchBar() string {
	filterDefs := []struct {
		a plan.ActionType
		n string
	}{
		{plan.ActionCreate, "[+]"},
		{plan.ActionUpdate, "[~]"},
		{plan.ActionDelete, "[-]"},
		{plan.ActionReplace, "[±]"},
	}

	var filterParts []string
	for _, f := range filterDefs {
		var s string
		if m.filters[f.a] {
			s = lipgloss.NewStyle().Foreground(actionColor(f.a)).Bold(true).Reverse(true).Render(f.n)
		} else {
			s = lipgloss.NewStyle().Foreground(clrMuted).Render(f.n)
		}
		filterParts = append(filterParts, s)
	}
	filters := "  " + strings.Join(filterParts, " ")

	var searchStr string
	if m.searchMode {
		searchStr = lipgloss.NewStyle().Foreground(clrAccent).Render("/") + " " + m.searchInput.View()
	} else if q := m.searchInput.Value(); q != "" {
		searchStr = lipgloss.NewStyle().Foreground(clrAccent).Render("/ "+q)
	} else {
		searchStr = lipgloss.NewStyle().Foreground(clrMuted).Render("/ press / to search")
	}

	return filters + "   " + searchStr
}

func (m *Model) renderFooter() string {
	type hint struct{ key, desc string }
	hints := []hint{
		{"↑↓/jk", "navigate"},
		{"Space", "expand"},
		{"y", "copy"},
		{"?", "menu"},
		{"[/]", "resize"},
		{"Tab", "switch panel"},
		{"E/C", "expand/collapse all"},
		{"1-4", "filter by action"},
		{"/", "search"},
		{"o", "diff-only"},
		{"m", "modules"},
		{"q", "quit"},
	}
	var parts []string
	for _, h := range hints {
		kStyle := lipgloss.NewStyle().Bold(true)
		if (h.key == "o" && m.diffOnly) || (h.key == "m" && m.groupByModule) {
			kStyle = kStyle.Reverse(true).Foreground(clrAccent)
		}
		k := kStyle.Render(h.key)
		dStyle := lipgloss.NewStyle().Foreground(clrMuted)
		if (h.key == "o" && m.diffOnly) || (h.key == "m" && m.groupByModule) {
			dStyle = dStyle.Foreground(clrAccent)
		}
		d := dStyle.Render(" " + h.desc)
		parts = append(parts, k+d)
	}
	sep := lipgloss.NewStyle().Foreground(clrBorder).Render("  ·  ")
	return "  " + strings.Join(parts, sep)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
