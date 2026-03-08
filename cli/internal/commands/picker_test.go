package commands

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// --- fuzzyMatch tests ---

func TestFuzzyMatchEmptyQuery(t *testing.T) {
	score := fuzzyMatch("", "anything")
	if score != 0 {
		t.Errorf("expected 0 for empty query, got %d", score)
	}
}

func TestFuzzyMatchExactSubstring(t *testing.T) {
	score := fuzzyMatch("deploy", "deploy-service")
	if score < 0 {
		t.Fatalf("expected positive score for exact substring, got %d", score)
	}
	// Score = 1000 - len("deploy-service") - index(0)
	expected := 1000 - len("deploy-service") - 0
	if score != expected {
		t.Errorf("expected score %d, got %d", expected, score)
	}
}

func TestFuzzyMatchPrefixHigherThanMidString(t *testing.T) {
	prefixScore := fuzzyMatch("dep", "deploy")
	midScore := fuzzyMatch("dep", "undeploy")

	if prefixScore <= midScore {
		t.Errorf("prefix match (%d) should score higher than mid-string match (%d)", prefixScore, midScore)
	}
}

func TestFuzzyMatchSequentialChars(t *testing.T) {
	// "dpl" matches d-e-p-l-o-y sequentially: d(0), p(2), l(4)
	score := fuzzyMatch("dpl", "deploy")
	if score < 0 {
		t.Fatalf("expected positive score for sequential match, got %d", score)
	}
	// Should be in the 500 range (sequential match baseline)
	if score > 500 || score < 400 {
		t.Errorf("expected score in 400-500 range for sequential match, got %d", score)
	}
}

func TestFuzzyMatchGapsReduceScore(t *testing.T) {
	// "dy" in "deploy" has more gaps than "de" in "deploy"
	closeScore := fuzzyMatch("de", "deploy")
	gappyScore := fuzzyMatch("dy", "deploy")

	if closeScore <= gappyScore {
		t.Errorf("close chars (%d) should score higher than gappy chars (%d)", closeScore, gappyScore)
	}
}

func TestFuzzyMatchNoMatch(t *testing.T) {
	score := fuzzyMatch("xyz", "deploy")
	if score != -1 {
		t.Errorf("expected -1 for no match, got %d", score)
	}
}

func TestFuzzyMatchCaseInsensitive(t *testing.T) {
	score1 := fuzzyMatch("Deploy", "deploy-service")
	score2 := fuzzyMatch("deploy", "Deploy-Service")

	if score1 < 0 {
		t.Error("expected positive score for case-insensitive match (upper query)")
	}
	if score2 < 0 {
		t.Error("expected positive score for case-insensitive match (upper target)")
	}
}

// --- filterAndRank tests ---

func testItems() []pickerItem {
	return []pickerItem{
		{Slug: "deploy", Name: "Deploy Service", Description: "Deploy to production"},
		{Slug: "build", Name: "Build", Description: "Build the project"},
		{Slug: "test-runner", Name: "Test Runner", Description: "Run all tests"},
		{Slug: "cleanup", Name: "Cleanup", Description: "Remove temporary files"},
	}
}

func TestFilterAndRankEmptyQuery(t *testing.T) {
	items := testItems()
	result := filterAndRank(items, "")

	if len(result) != len(items) {
		t.Errorf("expected all %d items returned for empty query, got %d", len(items), len(result))
	}
}

func TestFilterAndRankFiltersNonMatching(t *testing.T) {
	items := testItems()
	result := filterAndRank(items, "deploy")

	// Should match "deploy" (slug) and "Deploy Service" (name) and "Deploy to production" (desc)
	if len(result) == 0 {
		t.Fatal("expected at least one match for 'deploy'")
	}
	for _, item := range result {
		if item.matchScore < 0 {
			t.Errorf("item %q has negative match score", item.Slug)
		}
	}

	// "xyz" should match nothing
	noResult := filterAndRank(items, "xyz")
	if len(noResult) != 0 {
		t.Errorf("expected 0 results for 'xyz', got %d", len(noResult))
	}
}

func TestFilterAndRankSortsByScore(t *testing.T) {
	items := testItems()
	result := filterAndRank(items, "build")

	if len(result) == 0 {
		t.Fatal("expected at least one match")
	}
	// First result should be "build" (exact slug match)
	if result[0].Slug != "build" {
		t.Errorf("expected 'build' as top result, got %q", result[0].Slug)
	}
}

func TestFilterAndRankSlugMatchHigherThanDescription(t *testing.T) {
	items := []pickerItem{
		{Slug: "other", Name: "Other", Description: "deploy related"},
		{Slug: "deploy", Name: "Deploy", Description: "something"},
	}
	result := filterAndRank(items, "deploy")

	if len(result) < 2 {
		t.Fatalf("expected 2 matches, got %d", len(result))
	}
	if result[0].Slug != "deploy" {
		t.Errorf("expected slug match 'deploy' ranked first, got %q", result[0].Slug)
	}
}

// --- moveCursor tests ---

func TestMoveCursorBounds(t *testing.T) {
	m := pickerModel{
		filtered: make([]pickerItem, 5),
		cursor:   0,
		height:   20,
	}

	// Moving up from 0 should stay at 0
	result := m.moveCursor(-1)
	if result.cursor != 0 {
		t.Errorf("cursor should stay at 0, got %d", result.cursor)
	}

	// Moving down from 0
	result = m.moveCursor(1)
	if result.cursor != 1 {
		t.Errorf("cursor should be 1, got %d", result.cursor)
	}

	// Moving past end should clamp
	m.cursor = 4
	result = m.moveCursor(1)
	if result.cursor != 4 {
		t.Errorf("cursor should stay at 4, got %d", result.cursor)
	}
}

func TestMoveCursorScrollOffset(t *testing.T) {
	m := pickerModel{
		filtered:     make([]pickerItem, 20),
		cursor:       0,
		scrollOffset: 0,
		height:       10, // visibleRows = 10 - 5 = 5
	}

	// Move cursor to bottom of visible area
	for i := 0; i < 6; i++ {
		m = m.moveCursor(1)
	}
	// Cursor should be 6, scroll offset should have adjusted
	if m.cursor != 6 {
		t.Errorf("expected cursor 6, got %d", m.cursor)
	}
	if m.scrollOffset <= 0 {
		t.Errorf("expected scroll offset > 0, got %d", m.scrollOffset)
	}
}

func TestMoveCursorEmptyList(t *testing.T) {
	m := pickerModel{
		filtered: []pickerItem{},
		cursor:   0,
		height:   20,
	}

	result := m.moveCursor(1)
	if result.cursor != 0 {
		t.Errorf("cursor should be 0 for empty list, got %d", result.cursor)
	}
}

// --- handleKey tests ---

func TestHandleKeyEscape(t *testing.T) {
	m := pickerModel{
		filtered: testItems(),
		height:   20,
	}

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}
	result, cmd := m.handleKey(msg)
	model := result.(pickerModel)

	if !model.cancelled {
		t.Error("expected cancelled=true on Escape")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestHandleKeyEnterWithItems(t *testing.T) {
	items := testItems()
	m := pickerModel{
		filtered: items,
		cursor:   1,
		height:   20,
	}

	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	result, cmd := m.handleKey(msg)
	model := result.(pickerModel)

	if model.selected == nil {
		t.Fatal("expected selected item on Enter")
	}
	if model.selected.Slug != items[1].Slug {
		t.Errorf("expected selected slug %q, got %q", items[1].Slug, model.selected.Slug)
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestHandleKeyEnterEmptyList(t *testing.T) {
	m := pickerModel{
		filtered: []pickerItem{},
		cursor:   0,
		height:   20,
	}

	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	result, _ := m.handleKey(msg)
	model := result.(pickerModel)

	if model.selected != nil {
		t.Error("expected nil selected when list is empty")
	}
}

func TestHandleKeyUpDown(t *testing.T) {
	m := pickerModel{
		filtered: testItems(),
		cursor:   0,
		height:   20,
	}

	// Down
	result, _ := m.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	model := result.(pickerModel)
	if model.cursor != 1 {
		t.Errorf("expected cursor 1 after Down, got %d", model.cursor)
	}

	// Up
	result, _ = model.handleKey(tea.KeyPressMsg{Code: tea.KeyUp})
	model = result.(pickerModel)
	if model.cursor != 0 {
		t.Errorf("expected cursor 0 after Up, got %d", model.cursor)
	}
}

func TestHandleKeyTextInput(t *testing.T) {
	items := testItems()
	m := pickerModel{
		allItems: items,
		filtered: items,
		cursor:   0,
		height:   20,
	}

	// Type "d"
	result, _ := m.handleKey(tea.KeyPressMsg{Code: 'd', Text: "d"})
	model := result.(pickerModel)

	if model.query != "d" {
		t.Errorf("expected query 'd', got %q", model.query)
	}
	// Refiltering should have happened (verified by cursor reset below)
	if model.cursor != 0 {
		t.Errorf("expected cursor reset to 0 after refilter, got %d", model.cursor)
	}
}

func TestHandleKeyBackspace(t *testing.T) {
	items := testItems()
	m := pickerModel{
		allItems: items,
		filtered: items,
		query:    "dep",
		cursor:   0,
		height:   20,
	}

	result, _ := m.handleKey(tea.KeyPressMsg{Code: tea.KeyBackspace})
	model := result.(pickerModel)

	if model.query != "de" {
		t.Errorf("expected query 'de' after backspace, got %q", model.query)
	}
}

func TestHandleKeyBackspaceEmpty(t *testing.T) {
	m := pickerModel{
		allItems: testItems(),
		filtered: testItems(),
		query:    "",
		height:   20,
	}

	result, _ := m.handleKey(tea.KeyPressMsg{Code: tea.KeyBackspace})
	model := result.(pickerModel)

	if model.query != "" {
		t.Errorf("expected empty query unchanged, got %q", model.query)
	}
}

func TestHandleKeyCtrlC(t *testing.T) {
	m := pickerModel{
		filtered: testItems(),
		height:   20,
	}

	msg := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	result, cmd := m.handleKey(msg)
	model := result.(pickerModel)

	if !model.cancelled {
		t.Error("expected cancelled=true on Ctrl+C")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}
