package commands

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"mycli.sh/cli/internal/auth"
	"mycli.sh/cli/internal/cache"
	"mycli.sh/cli/internal/client"
	"mycli.sh/cli/internal/config"
	"mycli.sh/cli/internal/library"
	"mycli.sh/cli/internal/termui"

	"github.com/spf13/cobra"
)

// --- Views ---

type exploreView int

const (
	viewList exploreView = iota
	viewDetail
)

// --- Model ---

type exploreModel struct {
	view          exploreView
	width, height int

	// List view
	searchInput   string
	searchFocused bool
	libraries     []client.PublicLibrary
	totalResults  int
	cursor        int
	scrollOffset  int
	listLoading   bool
	listError     string
	searchSeq     int

	// Detail view
	selectedLib   *client.PublicLibraryDetail
	releases      []client.LibraryReleaseInfo
	detailTab     int // 0=commands, 1=releases
	detailLoading bool
	detailError   string
	detailScroll  int

	// Install
	installing   bool
	installMsg   string
	installedSet map[string]struct{}

	// Animation
	spinnerFrame  int
	cursorVisible bool
	borderOffset  int

	apiClient *client.Client
}

// --- Messages ---

type librariesLoadedMsg struct {
	seq       int
	libraries []client.PublicLibrary
	total     int
	err       error
}

type libraryDetailMsg struct {
	detail *client.PublicLibraryDetail
	err    error
}

type releasesLoadedMsg struct {
	releases []client.LibraryReleaseInfo
	err      error
}

type installResultMsg struct {
	name string
	err  error
}

type searchDebounceMsg struct {
	seq int
}

type spinnerTickMsg struct{}

// --- Spinner ---

var spinnerFrames = [...]string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func spinnerTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

// --- Colors ---

var (
	violet      = lipgloss.Color("#8B5CF6")
	violetLight = lipgloss.Color("#A78BFA")
	violetDark  = lipgloss.Color("#6D28D9")
	cyanColor   = lipgloss.Color("#22D3EE")
	white       = lipgloss.Color("#F4F4F5")
	subtleGray  = lipgloss.Color("#71717A")
	mutedGray   = lipgloss.Color("#A1A1AA")
	dimGray     = lipgloss.Color("#3F3F46")
	darkGray    = lipgloss.Color("#27272A")
	darkerGray  = lipgloss.Color("#18181B")
	greenColor  = lipgloss.Color("#4ADE80")
	greenDim    = lipgloss.Color("#166534")
	redColor    = lipgloss.Color("#F87171")
)

// --- Styles ---

var (
	// Header
	headerTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(violet)

	// Search box
	searchBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(dimGray).
			Padding(0, 1).
			MarginLeft(1)

	searchBoxFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(violet).
				Padding(0, 1).
				MarginLeft(1)

	searchIconStyle    = lipgloss.NewStyle().Foreground(subtleGray)
	searchCountStyle   = lipgloss.NewStyle().Foreground(subtleGray)
	searchSpinnerStyle = lipgloss.NewStyle().Foreground(violet)

	// Library cards
	cardNormalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(dimGray).
			Padding(0, 1).
			MarginLeft(1)

	// Detail hero card
	detailHeroStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).
			MarginLeft(1)

	// Tabs
	tabActiveStyle   = lipgloss.NewStyle().Bold(true).Foreground(violet)
	tabInactiveStyle = lipgloss.NewStyle().Foreground(subtleGray)
	tabLineStyle     = lipgloss.NewStyle().Foreground(dimGray)

	// Badges
	badgeOfficialStyle    = lipgloss.NewStyle().Foreground(violet)
	badgeVersionStyle     = lipgloss.NewStyle().Foreground(darkerGray).Background(violetLight).Padding(0, 1)
	badgeInstallCntStyle  = lipgloss.NewStyle().Foreground(subtleGray)
	installBtnStyle       = lipgloss.NewStyle().Foreground(white).Background(violet).Padding(0, 2).Bold(true)
	installBtnActiveStyle = lipgloss.NewStyle().Foreground(white).Background(violetDark).Padding(0, 2)
	installBtnDoneStyle   = lipgloss.NewStyle().Foreground(white).Background(greenDim).Padding(0, 2).Bold(true)

	// Help bar
	helpKeyStyle  = lipgloss.NewStyle().Foreground(white).Background(darkGray).Padding(0, 1).Bold(true)
	helpDescStyle = lipgloss.NewStyle().Foreground(subtleGray)
	positionStyle = lipgloss.NewStyle().Foreground(mutedGray).Background(darkGray).Padding(0, 1)

	// Scroll indicators
	scrollIndicatorStyle = lipgloss.NewStyle().Foreground(violet)

	// Content
	libNameStyle  = lipgloss.NewStyle().Bold(true).Foreground(white)
	libOwnerStyle = lipgloss.NewStyle().Foreground(subtleGray)
	libDescStyle  = lipgloss.NewStyle().Foreground(mutedGray)
	errorStyle    = lipgloss.NewStyle().Foreground(redColor)
	successStyle  = lipgloss.NewStyle().Foreground(greenColor)

	// Commands table
	cmdSlugStyle = lipgloss.NewStyle().Foreground(violetLight)
	cmdDescStyle = lipgloss.NewStyle().Foreground(mutedGray)

	// Releases timeline
	timelineDotLatest = lipgloss.NewStyle().Foreground(violet)
	timelineDotOlder  = lipgloss.NewStyle().Foreground(subtleGray)
	timelineConnector = lipgloss.NewStyle().Foreground(dimGray)
	releaseTagStyle   = lipgloss.NewStyle().Foreground(darkerGray).Background(violetLight).Padding(0, 1)
	releaseDateStyle  = lipgloss.NewStyle().Foreground(subtleGray)
	releaseCmdStyle   = lipgloss.NewStyle().Foreground(dimGray)
)

// --- Helpers ---

func installedKey(owner, slug string) string {
	return owner + "/" + slug
}

func (m exploreModel) isInstalled(owner, slug string) bool {
	_, ok := m.installedSet[installedKey(owner, slug)]
	return ok
}

func formatCount(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// --- Commands (async) ---

func fetchLibraries(c *client.Client, query string, seq int) tea.Cmd {
	return func() tea.Msg {
		resp, err := c.SearchPublicLibraries(query, 50, 0)
		if err != nil {
			return librariesLoadedMsg{seq: seq, err: err}
		}
		return librariesLoadedMsg{
			seq:       seq,
			libraries: resp.Libraries,
			total:     resp.Total,
		}
	}
}

func fetchLibraryDetail(c *client.Client, owner, slug string) tea.Cmd {
	return func() tea.Msg {
		detail, err := c.GetPublicLibrary(owner, slug)
		return libraryDetailMsg{detail: detail, err: err}
	}
}

func fetchReleases(c *client.Client, owner, slug string) tea.Cmd {
	return func() tea.Msg {
		releases, err := c.ListReleases(owner, slug)
		if err != nil {
			return releasesLoadedMsg{err: err}
		}
		return releasesLoadedMsg{releases: releases}
	}
}

func installLibraryCmd(c *client.Client, owner, slug string) tea.Cmd {
	return func() tea.Msg {
		// Verify library exists
		detail, err := c.GetPublicLibrary(owner, slug)
		if err != nil {
			return installResultMsg{name: slug, err: fmt.Errorf("library not found: %w", err)}
		}

		// Install via API if logged in
		if auth.IsLoggedIn() {
			if err := c.InstallLibrary(owner, slug); err != nil {
				return installResultMsg{name: slug, err: fmt.Errorf("API install failed: %w", err)}
			}
			// Sync
			_, _ = cache.Sync(c, false)
		}

		// Register locally
		reg, err := library.LoadRegistry()
		if err != nil {
			return installResultMsg{name: slug, err: err}
		}

		displayName := slug
		if library.FindByName(reg, displayName) != nil {
			return installResultMsg{name: displayName, err: fmt.Errorf("already installed")}
		}

		reg.Sources = append(reg.Sources, library.SourceEntry{
			Name:        displayName,
			Owner:       detail.Owner,
			Slug:        detail.Library.Slug,
			Kind:        "registry",
			AddedAt:     time.Now(),
			LastUpdated: time.Now(),
		})
		if err := library.SaveRegistry(reg); err != nil {
			return installResultMsg{name: displayName, err: err}
		}

		return installResultMsg{name: displayName}
	}
}

func debounceSearch(seq int) tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
		return searchDebounceMsg{seq: seq}
	})
}

// --- Init / Update / View ---

func (m exploreModel) Init() tea.Cmd {
	return tea.Batch(fetchLibraries(m.apiClient, "", 0), spinnerTick())
}

func (m exploreModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case spinnerTickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
		m.cursorVisible = !m.cursorVisible
		m.borderOffset++
		// Keep ticking while there's animation to show
		if m.listLoading || m.detailLoading || m.installing || m.searchFocused {
			return m, spinnerTick()
		}
		return m, nil

	case librariesLoadedMsg:
		if msg.seq != m.searchSeq {
			return m, nil // stale
		}
		m.listLoading = false
		if msg.err != nil {
			m.listError = msg.err.Error()
			return m, nil
		}
		m.listError = ""
		m.libraries = msg.libraries
		m.totalResults = msg.total
		if m.cursor >= len(m.libraries) {
			m.cursor = max(0, len(m.libraries)-1)
		}
		m.scrollOffset = 0
		return m, nil

	case libraryDetailMsg:
		m.detailLoading = false
		if msg.err != nil {
			m.detailError = msg.err.Error()
			return m, nil
		}
		m.detailError = ""
		m.selectedLib = msg.detail
		return m, nil

	case releasesLoadedMsg:
		if msg.err == nil {
			m.releases = msg.releases
		}
		return m, nil

	case installResultMsg:
		m.installing = false
		if msg.err != nil {
			m.installMsg = "error:" + msg.err.Error()
		} else {
			m.installMsg = "ok:" + msg.name
			if m.selectedLib != nil {
				m.installedSet[installedKey(m.selectedLib.Owner, m.selectedLib.Library.Slug)] = struct{}{}
			}
		}
		return m, nil

	case searchDebounceMsg:
		if msg.seq != m.searchSeq {
			return m, nil
		}
		m.listLoading = true
		m.listError = ""
		return m, fetchLibraries(m.apiClient, m.searchInput, m.searchSeq)
	}

	return m, nil
}

func (m exploreModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Global quit
	if msg.Mod.Contains(tea.ModCtrl) && msg.Code == 'c' {
		return m, tea.Quit
	}

	if m.view == viewList {
		return m.handleListKey(msg)
	}
	return m.handleDetailKey(msg)
}

func (m exploreModel) handleListKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.searchFocused {
		return m.handleSearchKey(msg)
	}

	switch msg.Code {
	case tea.KeyEscape:
		return m, tea.Quit
	case tea.KeyUp:
		return m.moveCursor(-1), nil
	case tea.KeyDown:
		return m.moveCursor(1), nil
	case tea.KeyEnter:
		return m.openDetail()
	}

	switch msg.Text {
	case "q":
		return m, tea.Quit
	case "/":
		m.searchFocused = true
		return m, spinnerTick()
	case "k":
		return m.moveCursor(-1), nil
	case "j":
		return m.moveCursor(1), nil
	}

	return m, nil
}

func (m exploreModel) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.Code {
	case tea.KeyEscape:
		m.searchFocused = false
		return m, nil
	case tea.KeyEnter:
		m.searchFocused = false
		return m, nil
	case tea.KeyBackspace:
		if len(m.searchInput) > 0 {
			m.searchInput = m.searchInput[:len(m.searchInput)-1]
			m.searchSeq++
			return m, debounceSearch(m.searchSeq)
		}
		return m, nil
	}

	if msg.Text != "" {
		m.searchInput += msg.Text
		m.searchSeq++
		return m, debounceSearch(m.searchSeq)
	}

	return m, nil
}

func (m exploreModel) handleDetailKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.Code {
	case tea.KeyEscape:
		m.view = viewList
		m.selectedLib = nil
		m.releases = nil
		m.detailTab = 0
		m.detailScroll = 0
		m.installMsg = ""
		return m, nil
	case tea.KeyTab:
		m.detailTab = (m.detailTab + 1) % 2
		m.detailScroll = 0
		return m, nil
	case tea.KeyUp:
		if m.detailScroll > 0 {
			m.detailScroll--
		}
		return m, nil
	case tea.KeyDown:
		if m.detailScroll < m.maxDetailScroll() {
			m.detailScroll++
		}
		return m, nil
	}

	switch msg.Text {
	case "q":
		return m, tea.Quit
	case "k":
		if m.detailScroll > 0 {
			m.detailScroll--
		}
		return m, nil
	case "j":
		if m.detailScroll < m.maxDetailScroll() {
			m.detailScroll++
		}
		return m, nil
	case "i":
		if !m.installing && m.selectedLib != nil && !m.isInstalled(m.selectedLib.Owner, m.selectedLib.Library.Slug) {
			m.installing = true
			m.installMsg = ""
			return m, tea.Batch(
				installLibraryCmd(m.apiClient, m.selectedLib.Owner, m.selectedLib.Library.Slug),
				spinnerTick(),
			)
		}
		return m, nil
	}

	return m, nil
}

func (m exploreModel) moveCursor(delta int) exploreModel {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.libraries) {
		m.cursor = max(0, len(m.libraries)-1)
	}

	// Scroll viewport
	visibleRows := m.visibleListRows()
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+visibleRows {
		m.scrollOffset = m.cursor - visibleRows + 1
	}
	return m
}

func (m exploreModel) openDetail() (tea.Model, tea.Cmd) {
	if len(m.libraries) == 0 {
		return m, nil
	}
	lib := m.libraries[m.cursor]
	m.view = viewDetail
	m.detailLoading = true
	m.detailError = ""
	m.detailTab = 0
	m.detailScroll = 0
	m.installMsg = ""
	return m, tea.Batch(
		fetchLibraryDetail(m.apiClient, lib.Owner, lib.Slug),
		fetchReleases(m.apiClient, lib.Owner, lib.Slug),
		spinnerTick(),
	)
}

func (m exploreModel) maxDetailScroll() int {
	var itemCount int
	if m.detailTab == 0 {
		if m.selectedLib != nil {
			itemCount = len(m.selectedLib.Commands)
		}
	} else {
		itemCount = len(m.releases)
	}

	availableRows := m.height - 18
	if availableRows < 1 {
		availableRows = 1
	}

	// For releases, each item takes 2 visual lines (item + connector) except the last.
	// So N items fit if (maxRows+1)/2 >= N, meaning max visible = (availableRows+1)/2.
	var visibleItems int
	if m.detailTab == 1 {
		visibleItems = (availableRows + 1) / 2
	} else {
		visibleItems = availableRows
	}

	maxScroll := itemCount - visibleItems
	if maxScroll < 0 {
		maxScroll = 0
	}
	return maxScroll
}

func (m exploreModel) visibleListRows() int {
	// header(2) + search(5) + scroll indicators(2) + blank(1) + help(1) = 11 lines overhead
	// each card = 5 lines (top border + 2 content + bottom border + newline separator)
	rows := (m.height - 11) / 5
	if rows < 1 {
		rows = 1
	}
	return rows
}

// --- View ---

func (m exploreModel) View() tea.View {
	if m.width == 0 {
		return tea.View{Content: "", AltScreen: true}
	}

	var b strings.Builder

	b.WriteString(m.renderHeader())

	if m.view == viewList {
		b.WriteString(m.viewList())
	} else {
		b.WriteString(m.viewDetail())
	}

	return tea.View{Content: b.String(), AltScreen: true}
}

// --- Header ---

func (m exploreModel) renderHeader() string {
	logo := termui.Violet(">") + " " + termui.Bold("my") + termui.Violet("cli")
	title := headerTitleStyle.Render("Explore Libraries")
	headerLine := "  " + logo
	padding := m.width - lipgloss.Width(logo) - lipgloss.Width(title) - 4
	if padding > 0 {
		headerLine += strings.Repeat(" ", padding) + title
	}
	return headerLine + "\n\n"
}

// --- Search Box ---

func (m exploreModel) renderSearchBox() string {
	innerWidth := m.width - 6 // 2 margin + 2 border + 2 padding
	if innerWidth < 18 {
		innerWidth = 18
	}

	icon := searchIconStyle.Render("🔍 ")
	boxStyle := searchBoxStyle

	var textPart string
	if m.searchFocused {
		boxStyle = searchBoxFocusedStyle
		if m.searchInput == "" {
			cursor := " "
			if m.cursorVisible {
				cursor = lipgloss.NewStyle().Foreground(violet).Bold(true).Render("█")
			}
			textPart = lipgloss.NewStyle().Foreground(subtleGray).Render("Search libraries...") + cursor
		} else {
			cursor := " "
			if m.cursorVisible {
				cursor = lipgloss.NewStyle().Foreground(violet).Bold(true).Render("█")
			}
			textPart = lipgloss.NewStyle().Foreground(white).Render(m.searchInput) + cursor
		}
	} else if m.searchInput == "" {
		textPart = lipgloss.NewStyle().Foreground(subtleGray).Render("Press / to search...")
	} else {
		textPart = lipgloss.NewStyle().Foreground(white).Render(m.searchInput)
	}

	// Right-side indicator: spinner while loading, result count otherwise
	var rightPart string
	if m.listLoading {
		rightPart = searchSpinnerStyle.Render(spinnerFrames[m.spinnerFrame])
	} else if len(m.libraries) > 0 && m.searchInput != "" {
		rightPart = searchCountStyle.Render(fmt.Sprintf("%d results", m.totalResults))
	}

	content := icon + textPart
	rightWidth := lipgloss.Width(rightPart)
	contentWidth := lipgloss.Width(content)

	// Pad to fill inner width (icon is already included in content)
	gap := innerWidth - contentWidth - rightWidth
	if gap < 0 {
		gap = 0
	}
	padded := content + strings.Repeat(" ", gap) + rightPart

	return boxStyle.Render(padded) + "\n\n"
}

// --- Library Card ---

func (m exploreModel) renderLibraryCard(lib client.PublicLibrary, selected bool) string {
	innerWidth := m.width - 6 // margin + border + padding
	if innerWidth < 20 {
		innerWidth = 20
	}

	// Line 1: name + badge + install count
	name := libNameStyle.Render(lib.Slug)
	var badge string
	if isSystemOwner(lib.Owner) {
		badge = badgeOfficialStyle.Render("✓ official")
	} else {
		badge = libOwnerStyle.Render("by " + lib.Owner)
	}
	if m.isInstalled(lib.Owner, lib.Slug) {
		badge += "  " + successStyle.Render("✓ installed")
	}
	installs := badgeInstallCntStyle.Render("⬇ " + formatCount(lib.InstallCount))

	line1 := name + "  " + badge
	rightSide := installs
	gap := innerWidth - lipgloss.Width(line1) - lipgloss.Width(rightSide)
	if gap > 0 {
		line1 += strings.Repeat(" ", gap) + rightSide
	} else {
		line1 += "  " + rightSide
	}

	// Line 2: description
	desc := lib.Description
	maxDesc := innerWidth
	if len(desc) > maxDesc && maxDesc > 3 {
		desc = desc[:maxDesc-3] + "..."
	}
	line2 := libDescStyle.Render(desc)
	if pad := innerWidth - lipgloss.Width(line2); pad > 0 {
		line2 += strings.Repeat(" ", pad)
	}

	// Pad line1 too
	if pad := innerWidth - lipgloss.Width(line1); pad > 0 {
		line1 += strings.Repeat(" ", pad)
	}

	cardContent := line1 + "\n" + line2

	if selected {
		style := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1).
			MarginLeft(1).
			Background(darkGray).
			BorderForegroundBlend(violetDark, violet, cyanColor, violet, violetDark).
			BorderForegroundBlendOffset(m.borderOffset)
		return style.Render(cardContent)
	}

	return cardNormalStyle.Render(cardContent)
}

// --- Scroll Indicators ---

func (m exploreModel) renderScrollUp(above int) string {
	if above <= 0 {
		return ""
	}
	return "  " + scrollIndicatorStyle.Render(fmt.Sprintf("▲ %d more above", above)) + "\n"
}

func (m exploreModel) renderScrollDown(below int) string {
	if below <= 0 {
		return ""
	}
	return "  " + scrollIndicatorStyle.Render(fmt.Sprintf("▼ %d more below", below)) + "\n"
}

// --- Help Bar ---

func (m exploreModel) renderListHelpBar() string {
	type helpEntry struct {
		key  string
		desc string
	}
	entries := []helpEntry{
		{"↑↓", "navigate"},
		{"⏎", "open"},
		{"/", "search"},
		{"q", "quit"},
	}

	var parts []string
	for _, e := range entries {
		parts = append(parts, helpKeyStyle.Render(e.key)+" "+helpDescStyle.Render(e.desc))
	}

	left := "  " + strings.Join(parts, "  ")

	var right string
	if m.listLoading {
		right = searchSpinnerStyle.Render(spinnerFrames[m.spinnerFrame]+" searching...")
	} else if len(m.libraries) > 0 {
		right = positionStyle.Render(fmt.Sprintf("%d of %d", m.cursor+1, m.totalResults))
	}

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap > 0 {
		return left + strings.Repeat(" ", gap) + right + "\n"
	}
	return left + "  " + right + "\n"
}

func (m exploreModel) renderDetailHelpBar() string {
	type helpEntry struct {
		key  string
		desc string
	}
	entries := []helpEntry{
		{"esc", "back"},
		{"tab", "switch"},
		{"i", "install"},
		{"q", "quit"},
	}

	var parts []string
	for _, e := range entries {
		parts = append(parts, helpKeyStyle.Render(e.key)+" "+helpDescStyle.Render(e.desc))
	}

	return "  " + strings.Join(parts, "  ") + "\n"
}

// --- Empty State ---

func (m exploreModel) renderEmptyState() string {
	var b strings.Builder
	if m.searchInput != "" {
		b.WriteString("\n")
		b.WriteString("  " + lipgloss.NewStyle().Foreground(subtleGray).Render(fmt.Sprintf("No results for '%s'", m.searchInput)) + "\n")
		b.WriteString("  " + lipgloss.NewStyle().Foreground(dimGray).Render("Try a different search term") + "\n")
	} else {
		b.WriteString("\n")
		b.WriteString("  " + lipgloss.NewStyle().Foreground(subtleGray).Render("No libraries found.") + "\n")
	}
	return b.String()
}

// --- List View ---

func (m exploreModel) viewList() string {
	var b strings.Builder

	b.WriteString(m.renderSearchBox())

	// Loading state
	if m.listLoading && len(m.libraries) == 0 {
		b.WriteString("\n")
		b.WriteString("  " + searchSpinnerStyle.Render(spinnerFrames[m.spinnerFrame]) + " " + lipgloss.NewStyle().Foreground(subtleGray).Render("Loading libraries...") + "\n")
		return b.String()
	}

	// Error state
	if m.listError != "" {
		b.WriteString("\n")
		b.WriteString("  " + errorStyle.Render("✗ "+m.listError) + "\n")
		return b.String()
	}

	// Empty state
	if len(m.libraries) == 0 {
		b.WriteString(m.renderEmptyState())
		b.WriteString("\n")
		b.WriteString(m.renderListHelpBar())
		return b.String()
	}

	// Card list
	visRows := m.visibleListRows()
	endIdx := m.scrollOffset + visRows
	if endIdx > len(m.libraries) {
		endIdx = len(m.libraries)
	}

	// Scroll-up indicator
	b.WriteString(m.renderScrollUp(m.scrollOffset))

	for i := m.scrollOffset; i < endIdx; i++ {
		lib := m.libraries[i]
		b.WriteString(m.renderLibraryCard(lib, i == m.cursor) + "\n")
	}

	// Scroll-down indicator
	below := len(m.libraries) - endIdx
	b.WriteString(m.renderScrollDown(below))

	b.WriteString("\n")
	b.WriteString(m.renderListHelpBar())
	return b.String()
}

// --- Detail View ---

func (m exploreModel) viewDetail() string {
	var b strings.Builder
	contentWidth := m.width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Back navigation
	b.WriteString("  " + helpKeyStyle.Render("esc") + " " + helpDescStyle.Render("back") + "\n\n")

	// Loading
	if m.detailLoading {
		b.WriteString("  " + searchSpinnerStyle.Render(spinnerFrames[m.spinnerFrame]) + " " + lipgloss.NewStyle().Foreground(subtleGray).Render("Loading...") + "\n")
		return b.String()
	}

	// Error
	if m.detailError != "" {
		b.WriteString("  " + errorStyle.Render("✗ "+m.detailError) + "\n")
		return b.String()
	}

	if m.selectedLib == nil {
		return b.String()
	}

	lib := m.selectedLib

	// --- Hero card ---
	b.WriteString(m.renderHeroCard(lib))

	// --- Tab bar ---
	b.WriteString(m.renderTabBar(lib))

	// --- Tab content ---
	availableRows := m.height - 18
	if availableRows < 1 {
		availableRows = 1
	}

	if m.detailTab == 0 {
		b.WriteString(m.renderCommands(lib.Commands, contentWidth, availableRows))
	} else {
		b.WriteString(m.renderReleases(m.releases, contentWidth, availableRows))
	}

	// Install message
	if m.installMsg != "" {
		b.WriteString("\n")
		if strings.HasPrefix(m.installMsg, "error:") {
			b.WriteString("  " + errorStyle.Render("✗ "+m.installMsg[6:]) + "\n")
		} else if strings.HasPrefix(m.installMsg, "ok:") {
			b.WriteString("  " + successStyle.Render("✓ Installed "+m.installMsg[3:]) + "\n")
		}
	}

	// Help
	b.WriteString("\n")
	b.WriteString(m.renderDetailHelpBar())

	return b.String()
}

// --- Hero Card ---

func (m exploreModel) renderHeroCard(lib *client.PublicLibraryDetail) string {
	innerWidth := m.width - 8 // margin + border + padding(2 each side)
	if innerWidth < 20 {
		innerWidth = 20
	}

	// Title
	titleLine := libNameStyle.Bold(true).Render(lib.Library.Name)

	// Meta line: badge · installs · version
	var metaParts []string
	if isSystemOwner(lib.Owner) {
		metaParts = append(metaParts, badgeOfficialStyle.Render("✓ official"))
	} else {
		metaParts = append(metaParts, libOwnerStyle.Render("by "+lib.Owner))
	}
	metaParts = append(metaParts, badgeInstallCntStyle.Render(formatCount(lib.Library.InstallCount)+" installs"))
	if len(m.releases) > 0 {
		metaParts = append(metaParts, badgeVersionStyle.Render(m.releases[0].Tag))
	}
	metaLine := strings.Join(metaParts, "  ·  ")

	// Description
	descLine := ""
	if lib.Library.Description != "" {
		descLine = libDescStyle.Render(lib.Library.Description)
	}

	// Install button
	var btnLine string
	if m.installing {
		btnLine = installBtnActiveStyle.Render(spinnerFrames[m.spinnerFrame] + " Installing...")
	} else if strings.HasPrefix(m.installMsg, "ok:") || m.isInstalled(lib.Owner, lib.Library.Slug) {
		btnLine = installBtnDoneStyle.Render("✓ Installed")
	} else {
		btnLine = installBtnStyle.Render("⬇ Install")
	}

	// Build card content
	var lines []string
	lines = append(lines, titleLine)
	lines = append(lines, metaLine)
	if descLine != "" {
		lines = append(lines, "")
		lines = append(lines, descLine)
	}
	lines = append(lines, "")
	lines = append(lines, btnLine)

	// Pad lines to fill width
	for i, line := range lines {
		if pad := innerWidth - lipgloss.Width(line); pad > 0 {
			lines[i] = line + strings.Repeat(" ", pad)
		}
	}
	cardContent := strings.Join(lines, "\n")

	style := detailHeroStyle.
		BorderForegroundBlend(violetDark, violet, cyanColor, violet, violetDark).
		BorderForegroundBlendOffset(m.borderOffset)

	return style.Render(cardContent) + "\n\n"
}

// --- Tab Bar ---

func (m exploreModel) renderTabBar(lib *client.PublicLibraryDetail) string {
	cmdCount := len(lib.Commands)
	relCount := len(m.releases)

	var cmdTab, relTab string
	if m.detailTab == 0 {
		cmdTab = tabActiveStyle.Render("Commands")
		relTab = tabInactiveStyle.Render("Releases")
	} else {
		cmdTab = tabInactiveStyle.Render("Commands")
		relTab = tabActiveStyle.Render("Releases")
	}

	// Underlines
	cmdWidth := lipgloss.Width(cmdTab)
	relWidth := lipgloss.Width(relTab)

	var cmdUnder, relUnder string
	if m.detailTab == 0 {
		cmdUnder = lipgloss.NewStyle().Foreground(violet).Render(strings.Repeat("━", cmdWidth))
		relUnder = tabLineStyle.Render(strings.Repeat("─", relWidth))
	} else {
		cmdUnder = tabLineStyle.Render(strings.Repeat("─", cmdWidth))
		relUnder = lipgloss.NewStyle().Foreground(violet).Render(strings.Repeat("━", relWidth))
	}

	gap := "     "

	// Right-side count
	var countText string
	if m.detailTab == 0 {
		countText = lipgloss.NewStyle().Foreground(subtleGray).Render(fmt.Sprintf("%d commands", cmdCount))
	} else {
		countText = lipgloss.NewStyle().Foreground(subtleGray).Render(fmt.Sprintf("%d releases", relCount))
	}

	// Fill remaining with dim line
	tabLine := "   " + cmdTab + gap + relTab
	underLine := "   " + cmdUnder + tabLineStyle.Render(strings.Repeat("─", len(gap))) + relUnder

	fillWidth := m.width - lipgloss.Width(underLine) - lipgloss.Width(countText) - 4
	if fillWidth > 0 {
		underLine += tabLineStyle.Render(strings.Repeat("─", fillWidth)) + countText
	}

	return tabLine + "\n" + underLine + "\n\n"
}

// --- Commands Table ---

func (m exploreModel) renderCommands(cmds []client.LibraryCommand, width, maxRows int) string {
	if len(cmds) == 0 {
		return "   " + lipgloss.NewStyle().Foreground(subtleGray).Render("No commands.") + "\n"
	}

	var b strings.Builder
	scroll := m.detailScroll
	if scroll > len(cmds)-1 {
		scroll = max(0, len(cmds)-1)
	}

	end := scroll + maxRows
	if end > len(cmds) {
		end = len(cmds)
	}

	// Scroll-up indicator
	if scroll > 0 {
		b.WriteString("   " + scrollIndicatorStyle.Render(fmt.Sprintf("▲ %d more above", scroll)) + "\n")
	}

	// Find max slug width for alignment
	maxSlug := 0
	for _, cmd := range cmds[scroll:end] {
		if len(cmd.Slug) > maxSlug {
			maxSlug = len(cmd.Slug)
		}
	}

	for idx, cmd := range cmds[scroll:end] {
		slug := cmdSlugStyle.Render(fmt.Sprintf("%-*s", maxSlug, cmd.Slug))
		desc := cmd.Description
		maxDesc := width - maxSlug - 8
		if len(desc) > maxDesc && maxDesc > 3 {
			desc = desc[:maxDesc-3] + "..."
		}
		renderedDesc := cmdDescStyle.Render(desc)

		// Alternating row backgrounds
		row := "   " + slug + "    " + renderedDesc
		if idx%2 == 1 {
			bgStyle := lipgloss.NewStyle().Background(darkerGray)
			rowWidth := lipgloss.Width(row)
			if pad := width - rowWidth + 2; pad > 0 {
				row += strings.Repeat(" ", pad)
			}
			row = bgStyle.Render(row)
		}
		b.WriteString(row + "\n")
	}

	// Scroll-down indicator
	if below := len(cmds) - end; below > 0 {
		b.WriteString("   " + scrollIndicatorStyle.Render(fmt.Sprintf("▼ %d more", below)) + "\n")
	}

	return b.String()
}

// --- Releases Timeline ---

func (m exploreModel) renderReleases(releases []client.LibraryReleaseInfo, width, maxRows int) string {
	if len(releases) == 0 {
		return "   " + lipgloss.NewStyle().Foreground(subtleGray).Render("No releases.") + "\n"
	}

	var b strings.Builder
	scroll := m.detailScroll
	if scroll > len(releases)-1 {
		scroll = max(0, len(releases)-1)
	}

	// Account for scroll-up indicator stealing a row
	effectiveRows := maxRows
	if scroll > 0 {
		effectiveRows--
	}
	if effectiveRows < 1 {
		effectiveRows = 1
	}

	// N releases take 2N-1 visual lines (item + connector between each pair).
	// Solve: 2*items - 1 <= effectiveRows → items <= (effectiveRows+1)/2
	end := scroll + (effectiveRows+1)/2
	if end > len(releases) {
		end = len(releases)
	}

	// Scroll-up indicator
	if scroll > 0 {
		b.WriteString("   " + scrollIndicatorStyle.Render(fmt.Sprintf("▲ %d more above", scroll)) + "\n")
	}

	for i, rel := range releases[scroll:end] {
		globalIdx := scroll + i

		// Timeline dot
		var dot string
		if globalIdx == 0 {
			dot = timelineDotLatest.Render("●")
		} else {
			dot = timelineDotOlder.Render("○")
		}

		// Version badge
		tag := releaseTagStyle.Render(rel.Tag)

		// Date
		date := releaseDateStyle.Render(rel.ReleasedAt)

		// Command count
		cmds := releaseCmdStyle.Render(fmt.Sprintf("%d commands", rel.CommandCount))

		b.WriteString("   " + dot + "  " + tag + "  " + date + "  " + cmds + "\n")

		// Connector to next item
		if i < len(releases[scroll:end])-1 {
			b.WriteString("   " + timelineConnector.Render("│") + "\n")
		}
	}

	// Scroll-down indicator
	if below := len(releases) - end; below > 0 {
		b.WriteString("   " + scrollIndicatorStyle.Render(fmt.Sprintf("▼ %d more", below)) + "\n")
	}

	return b.String()
}

// --- Cobra command ---

func newLibraryExploreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "explore",
		Short: "Interactively browse and install public libraries",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			c := client.New(resolveAPIURL(cfg))
			defer c.Close()

			installed := make(map[string]struct{})
			if reg, err := library.LoadRegistry(); err == nil {
				for _, s := range reg.Sources {
					installed[installedKey(s.Owner, s.Slug)] = struct{}{}
				}
			}

			m := exploreModel{
				apiClient:     c,
				listLoading:   true,
				searchFocused: false,
				cursorVisible: true,
				installedSet:  installed,
			}

			p := tea.NewProgram(m)
			_, err = p.Run()
			return err
		},
	}
}
