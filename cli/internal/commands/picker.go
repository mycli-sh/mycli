package commands

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"mycli.sh/cli/internal/cache"
	"mycli.sh/cli/internal/client"
	"mycli.sh/cli/internal/history"
	"mycli.sh/cli/internal/library"
)

// --- Picker data types ---

type pickerSource int

const (
	sourceLocal    pickerSource = iota // command.yaml in CWD
	sourcePersonal                     // API cache, top-level (Library == "")
	sourceLibrary                      // API cache, library command
	sourceGit                          // git-backed library
)

type pickerItem struct {
	Slug        string
	Name        string
	Description string
	Source      pickerSource
	SourceLabel string              // "local", "personal", "kubernetes", etc.
	FilePath    string              // for sourceLocal/sourceGit
	CatalogItem *client.CatalogItem // for sourcePersonal/sourceLibrary
	LibraryKey  string              // for sourceLibrary (cache lookup key)
	LastUsed    time.Time
	UseCount    int
	matchScore  int
}

func (s pickerSource) String() string {
	switch s {
	case sourceLocal:
		return "local"
	case sourcePersonal:
		return "personal"
	case sourceLibrary:
		return "library"
	case sourceGit:
		return "git"
	default:
		return "unknown"
	}
}

// --- Load items from all sources ---

func loadPickerItems() []pickerItem {
	// 1. Build history stats
	historyStats := make(map[string]struct {
		lastUsed time.Time
		count    int
	})
	if entries, err := history.List(0); err == nil {
		for _, e := range entries {
			s := historyStats[e.Slug]
			s.count++
			if e.Timestamp.After(s.lastUsed) {
				s.lastUsed = e.Timestamp
			}
			historyStats[e.Slug] = s
		}
	}

	var items []pickerItem
	seen := make(map[string]bool) // dedup key: "source:slug"

	// 2. API personal commands
	if catalog, err := cache.GetCatalog(); err == nil && catalog != nil {
		for _, ci := range catalog.Items {
			if ci.Library == "" {
				key := "personal:" + ci.Slug
				if seen[key] {
					continue
				}
				seen[key] = true
				item := pickerItem{
					Slug:        ci.Slug,
					Name:        ci.Name,
					Description: ci.Description,
					Source:      sourcePersonal,
					SourceLabel: "personal",
					CatalogItem: func() *client.CatalogItem { c := ci; return &c }(),
				}
				if hs, ok := historyStats[ci.Slug]; ok {
					item.LastUsed = hs.lastUsed
					item.UseCount = hs.count
				}
				items = append(items, item)
			}
		}

		// 3. API library commands
		for _, ci := range catalog.Items {
			if ci.Library != "" {
				displaySlug := ci.Library + "/" + ci.Slug
				key := "library:" + displaySlug
				if seen[key] {
					continue
				}
				seen[key] = true
				libKey := libraryKey(ci.LibraryOwner, ci.Library)
				item := pickerItem{
					Slug:        displaySlug,
					Name:        ci.Name,
					Description: ci.Description,
					Source:      sourceLibrary,
					SourceLabel: ci.Library,
					CatalogItem: func() *client.CatalogItem { c := ci; return &c }(),
					LibraryKey:  libKey,
				}
				if hs, ok := historyStats[displaySlug]; ok {
					item.LastUsed = hs.lastUsed
					item.UseCount = hs.count
				}
				items = append(items, item)
			}
		}
	}

	// 4. Git library commands (API takes precedence)
	if reg, err := library.LoadRegistry(); err == nil {
		for _, entry := range reg.Sources {
			if entry.Kind != "git" || entry.LocalPath == "" {
				continue
			}
			manifest, err := library.LoadManifest(entry.LocalPath)
			if err != nil {
				continue
			}
			for libSlug, libDef := range manifest.Libraries {
				specs, err := library.DiscoverSpecs(entry.LocalPath, libSlug, libDef)
				if err != nil {
					continue
				}
				for _, spec := range specs {
					displaySlug := libSlug + "/" + spec.Slug
					key := "library:" + displaySlug
					if seen[key] {
						continue // API version takes precedence
					}
					seen[key] = true
					item := pickerItem{
						Slug:        displaySlug,
						Name:        spec.Name,
						Description: spec.Description,
						Source:      sourceGit,
						SourceLabel: libSlug,
						FilePath:    spec.SpecPath,
					}
					if hs, ok := historyStats[displaySlug]; ok {
						item.LastUsed = hs.lastUsed
						item.UseCount = hs.count
					}
					items = append(items, item)
				}
			}
		}
	}

	// 5. Sort by: UseCount desc → LastUsed desc → Slug asc
	sort.Slice(items, func(i, j int) bool {
		if items[i].UseCount != items[j].UseCount {
			return items[i].UseCount > items[j].UseCount
		}
		if !items[i].LastUsed.Equal(items[j].LastUsed) {
			return items[i].LastUsed.After(items[j].LastUsed)
		}
		return items[i].Slug < items[j].Slug
	})

	return items
}

// --- Fuzzy matching ---

func fuzzyMatch(query, target string) int {
	if query == "" {
		return 0
	}
	query = strings.ToLower(query)
	target = strings.ToLower(target)

	// Exact substring match → high score
	if idx := strings.Index(target, query); idx >= 0 {
		// Prefer shorter targets and earlier matches
		return 1000 - len(target) - idx
	}

	// Sequential character match
	qi := 0
	gaps := 0
	matched := false
	for ti := 0; ti < len(target) && qi < len(query); ti++ {
		if target[ti] == query[qi] {
			qi++
			matched = true
		} else if matched {
			gaps++
		}
	}
	if qi < len(query) {
		return -1 // not all query chars matched
	}
	return 500 - gaps*10 - len(target)
}

func filterAndRank(items []pickerItem, query string) []pickerItem {
	if query == "" {
		return items
	}

	var result []pickerItem
	for _, item := range items {
		bestScore := -1
		for _, field := range []string{item.Slug, item.Name, item.Description} {
			if score := fuzzyMatch(query, field); score > bestScore {
				bestScore = score
			}
		}
		if bestScore >= 0 {
			item.matchScore = bestScore
			result = append(result, item)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].matchScore > result[j].matchScore
	})
	return result
}

// --- Bubble Tea model ---

type pickerModel struct {
	width, height int
	allItems      []pickerItem
	filtered      []pickerItem
	query         string
	cursor        int
	scrollOffset  int
	selected      *pickerItem // set on Enter
	cancelled     bool        // set on Esc/Ctrl+C
}

func (m pickerModel) Init() tea.Cmd {
	return nil
}

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m pickerModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Ctrl+C always quits
	if msg.Mod.Contains(tea.ModCtrl) && msg.Code == 'c' {
		m.cancelled = true
		return m, tea.Quit
	}

	switch msg.Code {
	case tea.KeyEscape:
		m.cancelled = true
		return m, tea.Quit
	case tea.KeyEnter:
		if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
			item := m.filtered[m.cursor]
			m.selected = &item
		}
		return m, tea.Quit
	case tea.KeyUp:
		return m.moveCursor(-1), nil
	case tea.KeyDown:
		return m.moveCursor(1), nil
	case tea.KeyBackspace:
		if len(m.query) > 0 {
			_, size := utf8.DecodeLastRuneInString(m.query)
			m.query = m.query[:len(m.query)-size]
			m.refilter()
		}
		return m, nil
	}

	// Printable text
	if msg.Text != "" {
		m.query += msg.Text
		m.refilter()
		return m, nil
	}

	return m, nil
}

func (m *pickerModel) refilter() {
	m.filtered = filterAndRank(m.allItems, m.query)
	m.cursor = 0
	m.scrollOffset = 0
}

func (m pickerModel) moveCursor(delta int) pickerModel {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}

	visibleRows := m.visibleRows()
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+visibleRows {
		m.scrollOffset = m.cursor - visibleRows + 1
	}
	return m
}

func (m pickerModel) visibleRows() int {
	// header(1) + search(1) + blank(1) + help(1) + blank(1) = 5 lines overhead
	rows := m.height - 5
	if rows < 1 {
		rows = 1
	}
	return rows
}

// --- Picker styles ---

var (
	pickerSearchStyle  = lipgloss.NewStyle().Foreground(violet).Bold(true)
	pickerPromptStyle  = lipgloss.NewStyle().Foreground(violet)
	pickerCursorStyle  = lipgloss.NewStyle().Foreground(violet).Bold(true)
	pickerNormalRow    = lipgloss.NewStyle().Foreground(subtleGray)
	pickerSourceStyle  = lipgloss.NewStyle().Foreground(dimGray)
	pickerSlugStyle    = lipgloss.NewStyle().Foreground(violetLight)
	pickerDescDimStyle = lipgloss.NewStyle().Foreground(dimGray)
	pickerHelpKey      = lipgloss.NewStyle().Foreground(white).Bold(true)
	pickerHelpDesc     = lipgloss.NewStyle().Foreground(subtleGray)
	pickerEmptyStyle   = lipgloss.NewStyle().Foreground(subtleGray)
)

func (m pickerModel) View() tea.View {
	if m.width == 0 {
		return tea.View{Content: ""}
	}

	var b strings.Builder

	// Search input line
	prompt := pickerPromptStyle.Render("> ")
	queryDisplay := pickerSearchStyle.Render(m.query)
	cursor := pickerCursorStyle.Render("█")
	b.WriteString(prompt + queryDisplay + cursor + "\n")

	// Filtered results
	if len(m.filtered) == 0 {
		if m.query != "" {
			b.WriteString(pickerEmptyStyle.Render("  No matching commands") + "\n")
		} else {
			b.WriteString(pickerEmptyStyle.Render("  No commands available") + "\n")
		}
	} else {
		visRows := m.visibleRows()
		endIdx := m.scrollOffset + visRows
		if endIdx > len(m.filtered) {
			endIdx = len(m.filtered)
		}

		for i := m.scrollOffset; i < endIdx; i++ {
			item := m.filtered[i]
			isCurrent := i == m.cursor

			var line string
			if isCurrent {
				line = pickerCursorStyle.Render("▸ ")
			} else {
				line = "  "
			}

			slug := item.Slug
			if isCurrent {
				slug = pickerSlugStyle.Render(slug)
			} else {
				slug = pickerNormalRow.Render(slug)
			}

			src := pickerSourceStyle.Render("[" + item.SourceLabel + "]")
			line += slug + " " + src

			if item.Description != "" {
				desc := item.Description
				maxDesc := m.width - lipgloss.Width(line) - 3
				if maxDesc > 3 && len(desc) > maxDesc {
					desc = desc[:maxDesc-3] + "..."
				}
				if maxDesc > 3 {
					line += " " + pickerDescDimStyle.Render(desc)
				}
			}

			b.WriteString(line + "\n")
		}
	}

	// Help bar
	b.WriteString("\n")
	help := pickerHelpKey.Render("↑↓") + " " + pickerHelpDesc.Render("navigate") +
		"  " + pickerHelpKey.Render("⏎") + " " + pickerHelpDesc.Render("select") +
		"  " + pickerHelpKey.Render("esc") + " " + pickerHelpDesc.Render("cancel") +
		"  " + pickerHelpDesc.Render(fmt.Sprintf("%d commands", len(m.filtered)))
	b.WriteString(help)

	return tea.View{Content: b.String()}
}

// --- Entry point ---

func runPicker(localFile string) (*pickerItem, error) {
	items := loadPickerItems()

	// Prepend local file if provided
	if localFile != "" {
		local := pickerItem{
			Slug:        localFile,
			Name:        "Run local spec",
			Description: "Run " + localFile + " directly",
			Source:      sourceLocal,
			SourceLabel: "local",
			FilePath:    localFile,
		}
		items = append([]pickerItem{local}, items...)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no commands available. Push a command with 'my cli push' or install a library")
	}

	model := pickerModel{
		allItems: items,
		filtered: items,
		width:    80, // default, will be updated by WindowSizeMsg
		height:   24,
	}

	p := tea.NewProgram(model, tea.WithoutSignalHandler())
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("picker: %w", err)
	}

	result := finalModel.(pickerModel)
	if result.cancelled || result.selected == nil {
		return nil, nil
	}
	return result.selected, nil
}
