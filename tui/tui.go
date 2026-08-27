package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ducva/tofu-diff/plan"
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

type Model struct {
	resources []plan.ResourceChange
	filtered  []int // indices into resources

	expanded map[int]bool // keyed by resource index
	cursor   int          // index in filtered
	focus    focusPanel

	searchInput textinput.Model
	searchMode  bool
	filters     map[plan.ActionType]bool

	leftVP  viewport.Model
	rightVP viewport.Model

	width  int
	height int
	ready  bool

	leftPanelWidthOffset int

	summary ActionSummary
	copyStatus string
	diffOnly bool
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

func New(pf plan.PlanFile) Model {
	return NewWithDiffOnly(pf, true)
}

func NewWithDiffOnly(pf plan.PlanFile, diffOnly bool) Model {
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
		resources:   resources,
		expanded:    make(map[int]bool),
		filters:     make(map[plan.ActionType]bool),
		searchInput: ti,
		cursor:      0,
		summary:     summary,
		diffOnly:    diffOnly,
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
			!strings.Contains(strings.ToLower(rc.Type), query) {
			continue
		}
		m.filtered = append(m.filtered, i)
	}

	if m.cursor >= len(m.filtered) {
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
		} else {
			m.cursor = 0
		}
	}
}

func (m *Model) selectedIndex() int {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return -1
	}
	return m.filtered[m.cursor]
}

func (m *Model) refreshViewports() {
	m.leftVP.SetContent(m.buildLeftContent())
	m.rightVP.SetContent(m.buildRightContent())
}

// scrollLeftToCursor adjusts leftVP.YOffset so the cursor row is visible.
func (m *Model) scrollLeftToCursor() {
	line := 0
	for vi, ri := range m.filtered {
		if vi == m.cursor {
			break
		}
		line++
		if m.expanded[ri] {
			diffs := plan.DiffAttributes(m.resources[ri].Change)
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

	case "y":
		if m.focus == focusLeft {
			if ri := m.selectedIndex(); ri >= 0 {
				rc := m.resources[ri]
				if err := clipboard.WriteAll(rc.Address); err != nil {
					m.copyStatus = "Copy failed: " + err.Error()
				} else {
					m.copyStatus = "Copied address!"
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
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				m.scrollLeftToCursor()
				m.rightVP.GotoTop()
				m.refreshViewports()
			}
		} else {
			m.rightVP.ScrollDown(1)
		}

	case " ":
		if ri := m.selectedIndex(); ri >= 0 {
			m.expanded[ri] = !m.expanded[ri]
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
		for _, ri := range m.filtered {
			m.expanded[ri] = true
		}
		m.refreshViewports()

	case "C":
		for k := range m.expanded {
			delete(m.expanded, k)
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

	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
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

	if len(m.filtered) == 0 {
		msg := "  No resources match."
		if len(m.resources) == 0 {
			msg = "  No changes."
		}
		return lipgloss.NewStyle().Foreground(clrMuted).Render(msg)
	}

	var sb strings.Builder
	for vi, ri := range m.filtered {
		rc := m.resources[ri]
		action := rc.Change.NormalizedAction()
		expanded := m.expanded[ri]
		selected := vi == m.cursor

		toggle := lipgloss.NewStyle().Foreground(clrMuted).Render(map[bool]string{true: "▼", false: "▶"}[expanded])
		sym := lipgloss.NewStyle().Foreground(actionColor(action)).Bold(true).Render(actionSym(action))

		// prefix: "  ▶ [~] " = 2+1+1+1+3+1 = 9 display chars
		const prefixWidth = 9
		addrMax := lw - prefixWidth
		addr := rc.Address
		if len(addr) > addrMax {
			addr = addr[:addrMax-3] + "..."
		}

		line := fmt.Sprintf("  %s %s %s", toggle, sym, addr)
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
			diffs := plan.DiffAttributes(rc.Change)
			if len(diffs) == 0 {
				detail := padTo(lipgloss.NewStyle().Foreground(clrMuted).Italic(true).Render("    (no attribute changes)"), lw)
				sb.WriteString(detail + "\n")
			} else {
				for _, d := range diffs {
					key := lipgloss.NewStyle().Foreground(clrAccent).Render(d.Key)
					before := lipgloss.NewStyle().Foreground(clrDelete).Render(d.BeforeDisplay)
					arrow := lipgloss.NewStyle().Foreground(clrMuted).Render(" → ")
					var after string
					if d.IsUnknownAfter {
						after = lipgloss.NewStyle().Foreground(clrReplace).Italic(true).Render("(known after apply)")
					} else {
						after = lipgloss.NewStyle().Foreground(clrCreate).Render(d.AfterDisplay)
					}
					detail := padTo("    "+key+"  "+before+arrow+after, lw)
					sb.WriteString(detail + "\n")
				}
			}
		}
	}
	return sb.String()
}

func (m *Model) buildRightContent() string {
	rw := max(1, m.rightWidth()-2)
	ri := m.selectedIndex()

	if ri < 0 {
		return lipgloss.NewStyle().Foreground(clrMuted).Render("  Select a resource to inspect its diff")
	}

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
// bypassing the 120-char truncation applied by plan.FormatValue. Used by the
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

	return m.renderHeader() + "\n" +
		m.renderSearchBar() + "\n" +
		panels + "\n" +
		m.renderFooter()
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
		{"[/]", "resize"},
		{"Tab", "switch panel"},
		{"E/C", "expand/collapse all"},
		{"1-4", "filter by action"},
		{"/", "search"},
		{"o", "diff-only"},
		{"q", "quit"},
	}
	var parts []string
	for _, h := range hints {
		kStyle := lipgloss.NewStyle().Bold(true)
		if h.key == "o" && m.diffOnly {
			kStyle = kStyle.Reverse(true).Foreground(clrAccent)
		}
		k := kStyle.Render(h.key)
		dStyle := lipgloss.NewStyle().Foreground(clrMuted)
		if h.key == "o" && m.diffOnly {
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
