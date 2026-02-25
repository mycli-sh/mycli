package commands

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	installing bool
	installMsg string

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

// --- Styles ---

var (
	violet      = lipgloss.Color("#8B5CF6")
	subtleGray  = lipgloss.Color("#71717A")
	mutedGray   = lipgloss.Color("#A1A1AA")
	dimGray     = lipgloss.Color("#3F3F46")
	greenColor  = lipgloss.Color("#4ADE80")
	yellowColor = lipgloss.Color("#FACC15")
	whiteColor  = lipgloss.Color("#E4E4E7")

	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(whiteColor)
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(violet)

	searchBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(dimGray).
			Padding(0, 1).
			MarginLeft(2)

	searchBoxFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(violet).
				Padding(0, 1).
				MarginLeft(2)

	cursorStyle      = lipgloss.NewStyle().Foreground(violet).Bold(true)
	libNameStyle     = lipgloss.NewStyle().Bold(true).Foreground(whiteColor)
	libOwnerStyle    = lipgloss.NewStyle().Foreground(subtleGray)
	officialStyle    = lipgloss.NewStyle().Foreground(violet)
	libDescStyle     = lipgloss.NewStyle().Foreground(mutedGray)
	installsStyle    = lipgloss.NewStyle().Foreground(subtleGray)
	activeTabStyle   = lipgloss.NewStyle().Bold(true).Foreground(violet)
	inactiveTabStyle = lipgloss.NewStyle().Foreground(subtleGray)
	helpStyle        = lipgloss.NewStyle().Foreground(subtleGray)
	errorStyle       = lipgloss.NewStyle().Foreground(yellowColor)
	successStyle     = lipgloss.NewStyle().Foreground(greenColor)
	countStyle       = lipgloss.NewStyle().Foreground(subtleGray)
	detailCardStyle  = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(violet).
				Padding(0, 1).
				MarginLeft(2)
)

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
	return tea.Batch(tea.EnterAltScreen, fetchLibraries(m.apiClient, "", 0))
}

func (m exploreModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

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
			m.installMsg = "Error: " + msg.err.Error()
		} else {
			m.installMsg = "Installed " + msg.name
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

func (m exploreModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global quit
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}

	if m.view == viewList {
		return m.handleListKey(msg)
	}
	return m.handleDetailKey(msg)
}

func (m exploreModel) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.searchFocused {
		return m.handleSearchKey(msg)
	}

	switch msg.Type {
	case tea.KeyEsc:
		return m, tea.Quit
	case tea.KeyUp:
		return m.moveCursor(-1), nil
	case tea.KeyDown:
		return m.moveCursor(1), nil
	case tea.KeyEnter:
		return m.openDetail()
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "q":
			return m, tea.Quit
		case "/":
			m.searchFocused = true
			return m, nil
		case "k":
			return m.moveCursor(-1), nil
		case "j":
			return m.moveCursor(1), nil
		}
	}
	return m, nil
}

func (m exploreModel) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		if m.searchInput == "" {
			return m, tea.Quit
		}
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
	case tea.KeyRunes:
		m.searchInput += string(msg.Runes)
		m.searchSeq++
		return m, debounceSearch(m.searchSeq)
	}
	return m, nil
}

func (m exploreModel) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyLeft:
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
		m.detailScroll++
		return m, nil
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "q":
			return m, tea.Quit
		case "k":
			if m.detailScroll > 0 {
				m.detailScroll--
			}
			return m, nil
		case "j":
			m.detailScroll++
			return m, nil
		case "i":
			if !m.installing && m.selectedLib != nil {
				m.installing = true
				m.installMsg = "Installing..."
				return m, installLibraryCmd(m.apiClient, m.selectedLib.Owner, m.selectedLib.Library.Slug)
			}
			return m, nil
		}
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
	)
}

func (m exploreModel) visibleListRows() int {
	// header(3) + search(3) + help(2) + padding = ~10 lines overhead
	rows := (m.height - 10) / 3 // each library row = ~3 lines
	if rows < 1 {
		rows = 1
	}
	return rows
}

// --- View ---

func (m exploreModel) View() string {
	if m.width == 0 {
		return ""
	}

	var b strings.Builder

	// Header
	logo := termui.Violet(">") + " " + termui.Bold("my") + termui.Violet("cli")
	title := headerStyle.Render("Explore Libraries")
	headerLine := "  " + logo
	padding := m.width - lipgloss.Width(logo) - lipgloss.Width(title) - 4
	if padding > 0 {
		headerLine += strings.Repeat(" ", padding) + title
	}
	b.WriteString(headerLine + "\n\n")

	if m.view == viewList {
		b.WriteString(m.viewList())
	} else {
		b.WriteString(m.viewDetail())
	}

	return b.String()
}

func (m exploreModel) viewList() string {
	var b strings.Builder
	contentWidth := m.width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Search bar — manually pad content so border auto-sizes correctly
	innerWidth := m.width - 6 // 2 indent + 2 border + 2 padding
	if innerWidth < 18 {
		innerWidth = 18
	}

	searchText := m.searchInput
	colorStyle := lipgloss.NewStyle().Foreground(whiteColor)
	boxStyle := searchBoxStyle
	if m.searchFocused {
		boxStyle = searchBoxFocusedStyle
		if searchText == "" {
			colorStyle = lipgloss.NewStyle().Foreground(subtleGray)
			searchText = "Search libraries..._"
		} else {
			searchText += "_"
		}
	} else if searchText == "" {
		colorStyle = lipgloss.NewStyle().Foreground(subtleGray)
		searchText = "Search libraries..."
	}

	// Pre-color and pad to exact inner width so border auto-sizes correctly
	colored := colorStyle.Render(searchText)
	if pad := innerWidth - lipgloss.Width(colored); pad > 0 {
		colored += strings.Repeat(" ", pad)
	}

	b.WriteString(boxStyle.Render(colored) + "\n\n")

	// Loading / error
	if m.listLoading && len(m.libraries) == 0 {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(subtleGray).Render("Loading...") + "\n")
		return b.String()
	}
	if m.listError != "" {
		b.WriteString("  " + errorStyle.Render("Error: "+m.listError) + "\n")
		return b.String()
	}
	if len(m.libraries) == 0 {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(subtleGray).Render("No libraries found.") + "\n")
		b.WriteString(m.listHelp())
		return b.String()
	}

	// Library rows
	visRows := m.visibleListRows()
	endIdx := m.scrollOffset + visRows
	if endIdx > len(m.libraries) {
		endIdx = len(m.libraries)
	}

	for i := m.scrollOffset; i < endIdx; i++ {
		lib := m.libraries[i]
		prefix := "  "
		if i == m.cursor {
			prefix = cursorStyle.Render("▸ ")
		}

		// First line: name + owner + installs
		name := libNameStyle.Render(lib.Slug)
		installs := installsStyle.Render(fmt.Sprintf("⬇ %d", lib.InstallCount))

		firstLine := prefix + name
		if !isSystemOwner(lib.Owner) {
			firstLine += "  " + libOwnerStyle.Render("by "+lib.Owner)
		} else {
			firstLine += "  " + officialStyle.Render("✓ official")
		}
		rightSide := installs
		gap := contentWidth - lipgloss.Width(firstLine) - lipgloss.Width(rightSide) + 2
		if gap > 0 {
			firstLine += strings.Repeat(" ", gap) + rightSide
		} else {
			firstLine += "  " + rightSide
		}

		b.WriteString(firstLine + "\n")

		// Second line: description
		desc := lib.Description
		maxDesc := contentWidth - 2
		if len(desc) > maxDesc && maxDesc > 3 {
			desc = desc[:maxDesc-3] + "..."
		}
		b.WriteString("    " + libDescStyle.Render(desc) + "\n")

		// Blank line separator
		b.WriteString("\n")
	}

	b.WriteString(m.listHelp())
	return b.String()
}

func (m exploreModel) listHelp() string {
	var b strings.Builder

	// Help + count
	help := helpStyle.Render("↑/↓ navigate  ⏎ details  / search  q quit")
	count := ""
	if len(m.libraries) > 0 {
		count = countStyle.Render(fmt.Sprintf("%d of %d", m.cursor+1, m.totalResults))
	}
	if m.listLoading {
		count = countStyle.Render("searching...")
	}

	helpLine := "  " + help
	gap := m.width - lipgloss.Width(helpLine) - lipgloss.Width(count) - 4
	if gap > 0 {
		helpLine += strings.Repeat(" ", gap) + count
	}
	b.WriteString(helpLine + "\n")
	return b.String()
}

func (m exploreModel) viewDetail() string {
	var b strings.Builder
	contentWidth := m.width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Back
	b.WriteString("  " + helpStyle.Render("← Back") + "\n\n")

	if m.detailLoading {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(subtleGray).Render("Loading...") + "\n")
		return b.String()
	}
	if m.detailError != "" {
		b.WriteString("  " + errorStyle.Render("Error: "+m.detailError) + "\n")
		return b.String()
	}
	if m.selectedLib == nil {
		return b.String()
	}

	lib := m.selectedLib

	// Detail card
	cardLines := []string{
		titleStyle.Render(lib.Library.Name),
		m.detailMetaLine(lib),
	}
	if lib.Library.Description != "" {
		cardLines = append(cardLines, libDescStyle.Render(lib.Library.Description))
	}
	cardContent := strings.Join(cardLines, "\n")

	// Pad each line to inner width so border auto-sizes correctly
	innerWidth := m.width - 6 // 2 indent + 2 border + 2 padding
	if innerWidth < 18 {
		innerWidth = 18
	}
	lines := strings.Split(cardContent, "\n")
	for i, line := range lines {
		if pad := innerWidth - lipgloss.Width(line); pad > 0 {
			lines[i] = line + strings.Repeat(" ", pad)
		}
	}
	b.WriteString(detailCardStyle.Render(strings.Join(lines, "\n")) + "\n\n")

	// Tabs
	var cmdTab, relTab string
	cmdCount := len(lib.Commands)
	relCount := len(m.releases)

	if m.detailTab == 0 {
		cmdTab = activeTabStyle.Render("[Commands]")
		relTab = inactiveTabStyle.Render(" Releases")
	} else {
		cmdTab = inactiveTabStyle.Render(" Commands ")
		relTab = activeTabStyle.Render("[Releases]")
	}

	tabRight := ""
	if m.detailTab == 0 {
		tabRight = countStyle.Render(fmt.Sprintf("%d commands", cmdCount))
	} else {
		tabRight = countStyle.Render(fmt.Sprintf("%d releases", relCount))
	}

	tabLine := "  " + cmdTab + relTab
	gap := m.width - lipgloss.Width(tabLine) - lipgloss.Width(tabRight) - 4
	if gap > 0 {
		tabLine += strings.Repeat(" ", gap) + tabRight
	}
	b.WriteString(tabLine + "\n\n")

	// Tab content
	availableRows := m.height - 14
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
		if strings.HasPrefix(m.installMsg, "Error:") || strings.HasPrefix(m.installMsg, "Installing") {
			if strings.HasPrefix(m.installMsg, "Error:") {
				b.WriteString("  " + errorStyle.Render(m.installMsg) + "\n")
			} else {
				b.WriteString("  " + lipgloss.NewStyle().Foreground(subtleGray).Render(m.installMsg) + "\n")
			}
		} else {
			b.WriteString("  " + successStyle.Render(m.installMsg) + "\n")
		}
	}

	// Help
	b.WriteString("\n")
	b.WriteString("  " + helpStyle.Render("esc back  tab switch  i install  q quit") + "\n")

	return b.String()
}

func (m exploreModel) detailMetaLine(lib *client.PublicLibraryDetail) string {
	var parts []string
	if isSystemOwner(lib.Owner) {
		parts = append(parts, officialStyle.Render("✓ official"))
	} else {
		parts = append(parts, "by "+lib.Owner)
	}
	grayParts := []string{fmt.Sprintf("%d installs", lib.Library.InstallCount)}
	if len(m.releases) > 0 {
		grayParts = append(grayParts, m.releases[0].Tag)
	}
	parts = append(parts, libOwnerStyle.Render(strings.Join(grayParts, " · ")))
	return strings.Join(parts, " · ")
}

func (m exploreModel) renderCommands(cmds []client.LibraryCommand, width, maxRows int) string {
	if len(cmds) == 0 {
		return "  " + lipgloss.NewStyle().Foreground(subtleGray).Render("No commands.") + "\n"
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

	// Find max slug width for alignment
	maxSlug := 0
	for _, cmd := range cmds[scroll:end] {
		if len(cmd.Slug) > maxSlug {
			maxSlug = len(cmd.Slug)
		}
	}

	for _, cmd := range cmds[scroll:end] {
		slug := libNameStyle.Render(fmt.Sprintf("%-*s", maxSlug, cmd.Slug))
		desc := libDescStyle.Render(cmd.Description)
		maxDesc := width - maxSlug - 6
		if len(cmd.Description) > maxDesc && maxDesc > 3 {
			desc = libDescStyle.Render(cmd.Description[:maxDesc-3] + "...")
		}
		b.WriteString("  " + slug + "  " + desc + "\n")
	}

	if end < len(cmds) {
		b.WriteString("  " + countStyle.Render(fmt.Sprintf("... %d more", len(cmds)-end)) + "\n")
	}

	return b.String()
}

func (m exploreModel) renderReleases(releases []client.LibraryReleaseInfo, width, maxRows int) string {
	if len(releases) == 0 {
		return "  " + lipgloss.NewStyle().Foreground(subtleGray).Render("No releases.") + "\n"
	}

	var b strings.Builder
	scroll := m.detailScroll
	if scroll > len(releases)-1 {
		scroll = max(0, len(releases)-1)
	}

	end := scroll + maxRows
	if end > len(releases) {
		end = len(releases)
	}

	for _, rel := range releases[scroll:end] {
		tag := libNameStyle.Render(fmt.Sprintf("%-12s", rel.Tag))
		date := libOwnerStyle.Render(rel.ReleasedAt)
		cmds := countStyle.Render(fmt.Sprintf("%d commands", rel.CommandCount))
		b.WriteString("  " + tag + "  " + date + "  " + cmds + "\n")
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

			m := exploreModel{
				apiClient:     c,
				listLoading:   true,
				searchFocused: false,
			}

			p := tea.NewProgram(m)
			_, err = p.Run()
			return err
		},
	}
}
