package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/cucumber/godog"
	"github.com/muesli/termenv"
	"sprout/pkg/config"
	"sprout/pkg/git"
	"sprout/pkg/linear"
)

// TUITestContext holds the state for our Gherkin tests
type TUITestContext struct {
	model               model
	testModel           *teatest.TestModel
	fakeClient          *linear.FakeLinearClient
	fakeWorktreeManager *testWorktreeManager
	defaultWorktreeCmd  string
	resumeWorktreeCmd   string
	postCreateRuns      []string
	postResumeRuns      []string
	postCreateRan       bool
	postResumeRan       bool
	pendingMsgs         chan tea.Msg
	t                   *testing.T
	terminalWidth       int
	terminalHeight      int
}

// NewTUITestContext creates a new test context
func NewTUITestContext(t *testing.T) *TUITestContext {
	return &TUITestContext{
		fakeClient:          linear.NewFakeLinearClient(),
		fakeWorktreeManager: &testWorktreeManager{},
		defaultWorktreeCmd:  "",
		resumeWorktreeCmd:   "",
		postCreateRuns:      nil,
		postResumeRuns:      nil,
		postCreateRan:       false,
		postResumeRan:       false,
		pendingMsgs:         make(chan tea.Msg, 64),
		t:                   t,
		terminalWidth:       80, // Default width
		terminalHeight:      24, // Default height
	}
}

// testWorktreeManager is a lightweight implementation for exercising the TUI
type testWorktreeManager struct {
	lastCreatedWorktree string
	lastCreatedBranch   string
	gitCommands         []string
	worktrees           []git.Worktree
	delayCreate         bool
	createUnblock       chan struct{}
}

func (m *testWorktreeManager) CreateWorktree(branchName string) (string, error) {
	if branchName == "" {
		return "", fmt.Errorf("branch name required")
	}
	m.lastCreatedWorktree = branchName
	m.gitCommands = append(m.gitCommands, fmt.Sprintf("git worktree add /mock/worktrees/%s -b %s main", branchName, branchName))
	if m.delayCreate {
		if m.createUnblock == nil {
			m.createUnblock = make(chan struct{})
		}
		<-m.createUnblock
		m.delayCreate = false
		m.createUnblock = nil
	}
	return "/mock/worktrees/" + branchName, nil
}

func (m *testWorktreeManager) CreateBranch(branchName string) error {
	if branchName == "" {
		return fmt.Errorf("branch name required")
	}
	m.lastCreatedBranch = branchName
	m.gitCommands = append(m.gitCommands, fmt.Sprintf("git checkout -b %s", branchName))
	return nil
}

func (m *testWorktreeManager) ListWorktrees() ([]git.Worktree, error) {
	return m.worktrees, nil
}

func (m *testWorktreeManager) ListWorktreesForTUI() ([]git.Worktree, error) {
	return m.worktrees, nil
}

func (m *testWorktreeManager) PruneWorktree(branchName string) error {
	return nil
}

func (m *testWorktreeManager) PruneAllMerged() error {
	return nil
}

func (m *testWorktreeManager) delayWorktreeCreation() {
	m.delayCreate = true
	m.createUnblock = make(chan struct{})
}

func (m *testWorktreeManager) completeDelayedWorktreeCreation() {
	if m.delayCreate && m.createUnblock != nil {
		close(m.createUnblock)
	}
}

// CursorPosition represents the position of the cursor in the terminal
type CursorPosition struct {
	Row int
	Col int
}

// extractCursorPosition finds and extracts cursor position from expected output
// Returns the cleaned output (without █) and cursor position if found
func extractCursorPosition(expected string) (string, *CursorPosition, error) {
	const cursorChar = "█"

	lines := strings.Split(expected, "\n")
	var cursorPos *CursorPosition

	for lineIdx, line := range lines {
		if idx := strings.Index(line, cursorChar); idx != -1 {
			if cursorPos != nil {
				return "", nil, fmt.Errorf("multiple cursor positions found - only one █ character allowed")
			}
			cursorPos = &CursorPosition{
				Row: lineIdx,
				Col: idx,
			}
			// Remove the cursor character from this line
			lines[lineIdx] = strings.Replace(line, cursorChar, "", 1)
		}
	}

	cleanedOutput := strings.Join(lines, "\n")
	return cleanedOutput, cursorPos, nil
}

// StripANSI removes ANSI escape sequences from text
func StripANSI(text string) string {
	// Remove common ANSI sequences
	replacements := []string{
		"\x1b[?25l",   // Hide cursor
		"\x1b[?25h",   // Show cursor
		"\x1b[?2004h", // Enable bracketed paste
		"\x1b[?2004l", // Disable bracketed paste
		"\x1b[?1002l", // Disable mouse tracking
		"\x1b[?1003l", // Disable mouse tracking
		"\x1b[?1006l", // Disable mouse tracking
		"\x1b[K",      // Clear to end of line
		"\x1b[2K",     // Clear entire line
	}

	result := text
	for _, seq := range replacements {
		result = strings.ReplaceAll(result, seq, "")
	}

	// Remove cursor positioning sequences like [80D
	// This regex-like approach handles variable numbers
	for i := 0; i < len(result); i++ {
		if i+1 < len(result) && result[i] == '\x1b' && result[i+1] == '[' {
			// Find the end of the sequence
			j := i + 2
			for j < len(result) && result[j] >= '0' && result[j] <= '9' {
				j++
			}
			if j < len(result) && (result[j] == 'D' || result[j] == 'A' || result[j] == 'B' || result[j] == 'C') {
				// Remove the entire sequence
				result = result[:i] + result[j+1:]
				i-- // Recheck this position
			}
		}
	}

	return result
}

// Step definitions

func (tc *TUITestContext) theFollowingLinearIssuesExist(issueTable *godog.Table) error {
	// Clear any existing data
	tc.fakeClient = linear.NewFakeLinearClient()

	// Parse table and populate fake client
	for i, row := range issueTable.Rows {
		if i == 0 { // Skip header row
			continue
		}

		identifier := row.Cells[0].Value
		title := row.Cells[1].Value
		parentID := row.Cells[2].Value

		// Default status if not provided in table
		var status linear.State
		if len(row.Cells) > 3 && row.Cells[3].Value != "" {
			stateType := strings.ToLower(strings.ReplaceAll(row.Cells[3].Value, " ", "_"))
			if stateType == "done" {
				stateType = "completed"
			}
			status = linear.State{
				ID:   identifier + "-state",
				Name: row.Cells[3].Value,
				Type: stateType,
			}
		} else {
			// Default status for tests
			status = linear.State{
				ID:   identifier + "-state",
				Name: "Todo",
				Type: "todo",
			}
		}

		var updatedAt time.Time
		if len(row.Cells) > 4 && row.Cells[4].Value != "" {
			updatedAt, _ = time.Parse(time.RFC3339, strings.TrimSpace(row.Cells[4].Value))
		}

		// Create issue with identifier as ID for simplicity in tests
		issue := linear.Issue{
			ID:          identifier,
			Identifier:  identifier,
			Title:       title,
			State:       status,
			HasChildren: false, // Will be set by FakeLinearClient
			Expanded:    false,
			Depth:       0,                // Will be set by UI based on hierarchy
			Children:    []linear.Issue{}, // Not used in fake client
			UpdatedAt:   updatedAt,
		}

		// Add to fake client (it handles parent-child relationships)
		tc.fakeClient.AddIssue(issue, parentID)
	}

	return nil
}

func (tc *TUITestContext) theFollowingWorktreesExist(worktreeTable *godog.Table) error {
	var worktrees []git.Worktree
	for i, row := range worktreeTable.Rows {
		if i == 0 {
			continue
		}
		branch := strings.TrimSpace(row.Cells[0].Value)
		path := strings.TrimSpace(row.Cells[1].Value)
		updatedAt, _ := time.Parse(time.RFC3339, strings.TrimSpace(row.Cells[2].Value))
		merged := false
		if len(row.Cells) > 3 {
			merged = strings.EqualFold(strings.TrimSpace(row.Cells[3].Value), "true")
		}
		prStatus := ""
		if merged {
			prStatus = "Merged"
		}
		worktrees = append(worktrees, git.Worktree{
			Branch:    branch,
			Path:      path,
			UpdatedAt: updatedAt,
			Merged:    merged,
			PRStatus:  prStatus,
		})
	}
	tc.fakeWorktreeManager.worktrees = worktrees
	return nil
}

func (tc *TUITestContext) fetchingChildrenForFails(identifier string) error {
	tc.fakeClient.FailChildFetch(identifier, fmt.Errorf("failed to fetch children for %s", identifier))
	return nil
}

func (tc *TUITestContext) iStartTheSproutTUI() error {
	// Set consistent color profile for testing
	lipgloss.SetColorProfile(termenv.Ascii)

	// Create test model with fake client and worktree manager stub
	var err error
	tc.model, err = NewTUIWithDependenciesAndConfig(tc.fakeWorktreeManager, tc.fakeClient, &config.Config{
		DefaultCommand: tc.defaultWorktreeCmd,
		ResumeCommand:  tc.resumeWorktreeCmd,
	})
	if err != nil {
		return err
	}

	// Manually execute the initialization to trigger loading
	tc.executeInitialization()

	if tc.t != nil {
		tc.testModel = teatest.NewTestModel(tc.t, tc.model, teatest.WithInitialTermSize(tc.terminalWidth, tc.terminalHeight))

		// Send window size message to the model to set up responsive layout
		windowSizeMsg := tea.WindowSizeMsg{Width: tc.terminalWidth, Height: tc.terminalHeight}
		updatedModel, _ := tc.model.Update(windowSizeMsg)
		tc.model = updatedModel.(model)
	}

	return nil
}

// executeInitialization simulates the full TUI initialization process including async loading
func (tc *TUITestContext) executeInitialization() {
	// Manually trigger the linear loading since we can't easily execute tea.Batch in tests
	if tc.model.LinearClient != nil && tc.model.LinearLoading {
		// Simulate the fetchLinearIssues command
		issues, err := tc.model.LinearClient.GetAssignedIssues()

		var msg tea.Msg
		if err != nil {
			msg = linearErrorMsg{err}
		} else {
			msg = linearIssuesLoadedMsg{issues}
		}

		// Update the model with the loading result
		updatedModel, _ := tc.model.Update(msg)
		tc.model = updatedModel.(model)
	}
	if tc.model.WorktreeManager != nil && tc.model.WorktreesLoading {
		worktrees, err := tc.model.WorktreeManager.ListWorktreesForTUI()
		var msg tea.Msg
		if err != nil {
			msg = worktreesErrorMsg{err}
		} else {
			msg = worktreesLoadedMsg{worktrees}
		}
		updatedModel, _ := tc.model.Update(msg)
		tc.model = updatedModel.(model)
	}
}

func (tc *TUITestContext) iPress(key string) error {
	var keyMsg tea.KeyMsg

	switch key {
	case "down":
		keyMsg = tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		keyMsg = tea.KeyMsg{Type: tea.KeyUp}
	case "right":
		keyMsg = tea.KeyMsg{Type: tea.KeyRight}
	case "left":
		keyMsg = tea.KeyMsg{Type: tea.KeyLeft}
	case "enter":
		keyMsg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		keyMsg = tea.KeyMsg{Type: tea.KeyEsc}
	case "escape":
		keyMsg = tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		keyMsg = tea.KeyMsg{Type: tea.KeyTab}
	case "/":
		keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	case "backspace":
		keyMsg = tea.KeyMsg{Type: tea.KeyBackspace}
	case "alt+enter":
		keyMsg = tea.KeyMsg{Type: tea.KeyEnter, Alt: true}
	case "shift+enter":
		// Bubble Tea doesn't expose a dedicated KeyShiftEnter, so tests map this to ctrl+j.
		keyMsg = tea.KeyMsg{Type: tea.KeyCtrlJ}
	case "ctrl+s":
		keyMsg = tea.KeyMsg{Type: tea.KeyCtrlS}
	case "u":
		keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}}
	case "d":
		keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
	case "z":
		keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}}
	case "a":
		keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	default:
		return fmt.Errorf("unknown key: %s", key)
	}

	// Update our local model reference and execute any returned commands
	updatedModel, cmd := tc.model.Update(keyMsg)
	tc.model = updatedModel.(model)

	tc.processCmd(cmd)
	tc.drainWithTimeout(10 * time.Millisecond)

	return nil
}

func (tc *TUITestContext) iType(text string) error {
	// Send each character as a separate key event
	for _, char := range text {
		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{char}}

		// Update our local model reference and execute any returned commands
		updatedModel, cmd := tc.model.Update(keyMsg)
		tc.model = updatedModel.(model)

		tc.processCmd(cmd)
		tc.drainWithTimeout(10 * time.Millisecond)
	}

	return nil
}

func (tc *TUITestContext) iTypeTheFollowingText(text *godog.DocString) error {
	return tc.iType(text.Content)
}

// processCmd executes a command and handles any resulting messages (including batches)
func (tc *TUITestContext) processCmd(cmd tea.Cmd) {
	if cmd == nil {
		return
	}

	go func() {
		tc.pendingMsgs <- cmd()
	}()

	tc.drainReadyMessages()
}

func (tc *TUITestContext) drainReadyMessages() {
	for {
		select {
		case msg := <-tc.pendingMsgs:
			tc.processMsg(msg)
		default:
			return
		}
	}
}

func (tc *TUITestContext) drainWithTimeout(timeout time.Duration) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case msg := <-tc.pendingMsgs:
			tc.processMsg(msg)
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(timeout)
		case <-timer.C:
			return
		}
	}
}

func (tc *TUITestContext) waitForOneAsyncMessage(timeout time.Duration) error {
	select {
	case msg := <-tc.pendingMsgs:
		tc.processMsg(msg)
		tc.drainReadyMessages()
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timed out waiting for async command completion")
	}
}

// processMsg updates the model with a message and processes any follow-up commands
func (tc *TUITestContext) processMsg(msg tea.Msg) {
	if msg == nil {
		return
	}

	switch m := msg.(type) {
	case tea.BatchMsg:
		for _, subMsg := range m {
			tc.processMsg(subMsg)
		}
	case tea.Cmd:
		tc.processCmd(m)
		return
	}

	updatedModel, _ := tc.model.Update(msg)
	tc.model = updatedModel.(model)
	tc.maybeRunPostCreateCommand()
	tc.maybeRunPostResumeCommand()
}

func (tc *TUITestContext) maybeRunPostCreateCommand() {
	if tc.postCreateRan {
		return
	}
	if tc.defaultWorktreeCmd == "" {
		return
	}
	if !tc.model.Done || !tc.model.Success || tc.model.WorktreePath == "" {
		return
	}

	cfg := &config.Config{DefaultCommand: tc.defaultWorktreeCmd}
	resolved := config.ResolveDefaultCommand(cfg.GetDefaultCommand(), tc.model.CapturedPrompt)
	if len(resolved) == 0 {
		return
	}

	tc.postCreateRuns = append(tc.postCreateRuns, fmt.Sprintf("cd %s && %s", tc.model.WorktreePath, formatCommandArgs(resolved)))
	tc.postCreateRan = true
}

func (tc *TUITestContext) maybeRunPostResumeCommand() {
	if tc.postResumeRan {
		return
	}
	if !tc.model.Done || !tc.model.Success || !tc.model.Resumed || tc.model.WorktreePath == "" {
		return
	}

	cfg := &config.Config{
		DefaultCommand: tc.defaultWorktreeCmd,
		ResumeCommand:  tc.resumeWorktreeCmd,
	}
	resolved := config.ResolveResumeCommand(cfg.GetResumeCommand(), cfg.GetDefaultCommand(), config.ResumeContext{
		WorktreePath: tc.model.WorktreePath,
		BranchName:   tc.model.ResumeBranch,
		RepoName:     "sprout",
	})
	if len(resolved) > 0 {
		tc.postResumeRuns = append(tc.postResumeRuns, fmt.Sprintf("cd %s && %s", tc.model.WorktreePath, formatCommandArgs(resolved)))
	}
	tc.postResumeRan = true
}

func formatCommandArgs(args []string) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		if strings.Contains(arg, "\n") {
			parts[i] = `"` + arg + `"`
		} else if arg == "" || strings.ContainsAny(arg, " \t\"'") {
			parts[i] = strconv.Quote(arg)
		} else {
			parts[i] = arg
		}
	}
	return strings.Join(parts, " ")
}

func (tc *TUITestContext) theUIShouldDisplay(expected *godog.DocString) error {
	if tc.testModel == nil {
		return fmt.Errorf("test model not initialized")
	}
	tc.drainWithTimeout(20 * time.Millisecond)

	// Extract cursor position from expected output if present
	expectedContent, expectedCursorPos, err := extractCursorPosition(expected.Content)
	if err != nil {
		return fmt.Errorf("cursor position extraction error: %v", err)
	}

	// Get current view from our model state instead of teatest output
	actual := tc.model.View()

	// Strip ANSI codes for comparison
	actual = StripANSI(actual)

	// Normalize whitespace more aggressively - strip leading and trailing whitespace from each line
	actualLines := strings.Split(actual, "\n")
	expectedLines := strings.Split(expectedContent, "\n")

	for i := range actualLines {
		actualLines[i] = strings.TrimSpace(actualLines[i])
	}
	for i := range expectedLines {
		expectedLines[i] = strings.TrimSpace(expectedLines[i])
	}

	// Remove empty lines at the beginning and end
	actualLines = trimEmptyLines(actualLines)
	expectedLines = trimEmptyLines(expectedLines)

	actualNormalized := strings.Join(actualLines, "\n")
	expectedNormalized := strings.Join(expectedLines, "\n")

	if actualNormalized != expectedNormalized {
		return fmt.Errorf("UI output mismatch:\nExpected:\n%s\n\nActual:\n%s", expectedNormalized, actualNormalized)
	}

	// Validate cursor position if specified
	if expectedCursorPos != nil {
		// Get actual cursor position from the text input model
		// The cursor position depends on which input is focused and active
		var actualCursorRow, actualCursorCol int

		if tc.model.InputMode && tc.model.TextInput.Focused() {
			// Cursor is in the main text input - find its row dynamically
			inputLine := strings.TrimSpace(tc.model.TextInput.View())
			actualCursorRow = -1
			for idx, line := range actualLines {
				if line == inputLine {
					actualCursorRow = idx
					break
				}
			}
			if actualCursorRow == -1 {
				// Fallback to 0 if we can't find the line
				actualCursorRow = 0
			}
			actualCursorCol = len(tc.model.TextInput.Prompt) + tc.model.TextInput.Position()
		} else if tc.model.SubtaskInputMode && tc.model.SubtaskInput.Focused() {
			// Cursor is in subtask input - need to find its position in the tree
			// For now, we'll implement a basic version
			actualCursorRow = 2 // This would need more complex logic for subtask inputs
			actualCursorCol = tc.model.SubtaskInput.Position()
		} else {
			// Cursor is on the selected item in the tree (non-input mode)
			// For tree navigation, the cursor is typically not visible
			// We'll skip cursor validation for non-input modes for now
			return nil
		}

		// Calculate the expected cursor position relative to the normalized output
		// We need to account for the trimming we did above
		originalLines := strings.Split(expected.Content, "\n")
		normalizedLines := strings.Split(expectedNormalized, "\n")

		// Find how many lines were trimmed from the top
		trimmedFromTop := 0
		for i, line := range originalLines {
			if strings.TrimSpace(line) != "" {
				break
			}
			if i < len(originalLines)-1 {
				trimmedFromTop++
			}
		}

		// Adjust expected cursor position for trimmed lines
		adjustedExpectedRow := expectedCursorPos.Row - trimmedFromTop

		// For column position, we need to account for leading whitespace that was trimmed
		var adjustedExpectedCol int
		if adjustedExpectedRow >= 0 && adjustedExpectedRow < len(normalizedLines) {
			originalLine := originalLines[expectedCursorPos.Row]
			normalizedLine := normalizedLines[adjustedExpectedRow]

			// Calculate how much leading whitespace was trimmed
			leadingSpacesTrimmed := len(originalLine) - len(strings.TrimLeft(originalLine, " \t"))
			normalizedLeadingSpaces := len(normalizedLine) - len(strings.TrimLeft(normalizedLine, " \t"))

			adjustedExpectedCol = expectedCursorPos.Col - leadingSpacesTrimmed + normalizedLeadingSpaces
		} else {
			adjustedExpectedCol = expectedCursorPos.Col
		}

		if actualCursorRow != adjustedExpectedRow || actualCursorCol != adjustedExpectedCol {
			return fmt.Errorf("cursor position mismatch:\nExpected: row=%d, col=%d\nActual: row=%d, col=%d",
				adjustedExpectedRow, adjustedExpectedCol, actualCursorRow, actualCursorCol)
		}
	}

	return nil
}

func (tc *TUITestContext) theUIShouldContain(text string) error {
	if tc.testModel == nil {
		return fmt.Errorf("test model not initialized")
	}

	tc.drainWithTimeout(20 * time.Millisecond)
	actual := StripANSI(tc.model.View())
	if !strings.Contains(actual, text) {
		return fmt.Errorf("expected UI to contain %q\nActual UI:\n%s", text, actual)
	}

	return nil
}

// trimEmptyLines removes empty lines from the beginning and end of a slice
func trimEmptyLines(lines []string) []string {
	// Find first non-empty line
	start := 0
	for start < len(lines) && lines[start] == "" {
		start++
	}

	// Find last non-empty line
	end := len(lines)
	for end > start && lines[end-1] == "" {
		end--
	}

	if start >= end {
		return []string{}
	}

	return lines[start:end]
}

func (tc *TUITestContext) myTerminalWidthIsCharacters(width int) error {
	tc.terminalWidth = width
	return nil
}

func (tc *TUITestContext) theUIShouldDisplayTitlesTruncatedToFitTheAvailableWidth() error {
	if tc.testModel == nil {
		return fmt.Errorf("test model not initialized")
	}
	tc.drainWithTimeout(20 * time.Millisecond)

	// Get current view
	actual := tc.model.View()
	actual = StripANSI(actual)

	// Check that long titles are truncated appropriately for narrow terminal
	// For a 60-character terminal, we expect titles to be truncated
	lines := strings.Split(actual, "\n")
	for _, line := range lines {
		// Check that no line exceeds the terminal width
		if len(line) > tc.terminalWidth {
			return fmt.Errorf("line exceeds terminal width of %d characters: %s (length: %d)", tc.terminalWidth, line, len(line))
		}

		// Check that long titles contain "..." indicating truncation
		if strings.Contains(line, "SPR-124") && tc.terminalWidth < 100 {
			if !strings.Contains(line, "...") {
				return fmt.Errorf("expected long title to be truncated with '...' in narrow terminal, but got: %s", line)
			}
		}
	}

	return nil
}

func (tc *TUITestContext) parseExpectedCommands(commandsTable *godog.Table) ([]string, error) {
	var expected []string
	for i, row := range commandsTable.Rows {
		if i == 0 { // Skip header row
			continue
		}
		if len(row.Cells) == 0 {
			return nil, fmt.Errorf("expected command row %d to have at least one cell", i)
		}
		expected = append(expected, strings.TrimSpace(row.Cells[0].Value))
	}

	return expected, nil
}

func (tc *TUITestContext) allRecordedCommands() []string {
	commands := make([]string, 0, len(tc.fakeWorktreeManager.gitCommands)+len(tc.postCreateRuns)+len(tc.postResumeRuns))
	commands = append(commands, tc.fakeWorktreeManager.gitCommands...)
	commands = append(commands, tc.postCreateRuns...)
	commands = append(commands, tc.postResumeRuns...)
	return commands
}

func (tc *TUITestContext) theFollowingCommandsShouldBeRun(commandsTable *godog.Table) error {
	if tc.fakeWorktreeManager == nil {
		return fmt.Errorf("worktree manager not initialized")
	}
	tc.drainWithTimeout(20 * time.Millisecond)

	expected, err := tc.parseExpectedCommands(commandsTable)
	if err != nil {
		return err
	}

	actual := tc.allRecordedCommands()
	if len(actual) != len(expected) {
		return fmt.Errorf("command count mismatch:\nExpected: %d\nActual: %d\nExpected commands: %v\nActual commands: %v", len(expected), len(actual), expected, actual)
	}

	for i := range expected {
		if actual[i] != expected[i] {
			return fmt.Errorf("command mismatch at index %d:\nExpected: %s\nActual: %s", i, expected[i], actual[i])
		}
	}

	return nil
}

func (tc *TUITestContext) theDefaultWorktreeCommandIs(command string) error {
	tc.defaultWorktreeCmd = strings.ReplaceAll(strings.TrimSpace(command), `\`, "")
	return nil
}

func (tc *TUITestContext) aConfigWith(configTable *godog.Table) error {
	for i, row := range configTable.Rows {
		if i == 0 {
			continue
		}
		key := strings.TrimSpace(row.Cells[0].Value)
		value := strings.TrimSpace(row.Cells[1].Value)
		switch key {
		case "defaultCommand", "default_command":
			tc.defaultWorktreeCmd = value
		case "resumeCommand", "resume_command":
			tc.resumeWorktreeCmd = value
		}
	}
	return nil
}

func (tc *TUITestContext) theTUIShouldResumeWorktree(path string) error {
	tc.drainWithTimeout(20 * time.Millisecond)
	if !tc.model.Resumed {
		return fmt.Errorf("expected TUI to resume a worktree")
	}
	if tc.model.WorktreePath != path {
		return fmt.Errorf("expected resumed path %q, got %q", path, tc.model.WorktreePath)
	}
	return nil
}

func (tc *TUITestContext) noNewWorktreeShouldBeCreated() error {
	if tc.fakeWorktreeManager.lastCreatedWorktree != "" {
		return fmt.Errorf("expected no new worktree, got %q", tc.fakeWorktreeManager.lastCreatedWorktree)
	}
	return nil
}

func (tc *TUITestContext) aWorktreeShouldBeCreatedForBranch(branch string) error {
	tc.drainWithTimeout(20 * time.Millisecond)
	if tc.fakeWorktreeManager.lastCreatedWorktree != branch {
		return fmt.Errorf("expected created worktree %q, got %q", branch, tc.fakeWorktreeManager.lastCreatedWorktree)
	}
	return nil
}

func (tc *TUITestContext) theUIShouldNotDisplay(text string) error {
	tc.drainWithTimeout(20 * time.Millisecond)
	actual := StripANSI(tc.model.View())
	if strings.Contains(actual, text) {
		return fmt.Errorf("expected UI not to contain %q\nActual UI:\n%s", text, actual)
	}
	return nil
}

func (tc *TUITestContext) theUIShouldDisplayText(text string) error {
	return tc.theUIShouldContain(text)
}

func (tc *TUITestContext) activeWorkQueueRowsExist(count int) error {
	var worktrees []git.Worktree
	baseTime := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	for i := 0; i < count; i++ {
		branch := fmt.Sprintf("feature-%02d", i+1)
		worktrees = append(worktrees, git.Worktree{
			Branch:    branch,
			Path:      "/mock/worktrees/" + branch,
			UpdatedAt: baseTime.Add(-time.Duration(i) * time.Minute),
		})
	}
	tc.fakeWorktreeManager.worktrees = worktrees
	return nil
}

func (tc *TUITestContext) theUIShouldShowWorkQueueRows(count int) error {
	tc.drainWithTimeout(20 * time.Millisecond)
	actual := StripANSI(tc.model.View())
	rows := 0
	for _, line := range strings.Split(actual, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "├──") || strings.HasPrefix(line, "└──") {
			rows++
		}
	}
	if rows != count {
		return fmt.Errorf("expected %d work queue rows, got %d\nActual UI:\n%s", count, rows, actual)
	}
	return nil
}

func (tc *TUITestContext) postResumeCommandShouldBe(command string) error {
	tc.drainWithTimeout(20 * time.Millisecond)
	if len(tc.postResumeRuns) != 1 {
		return fmt.Errorf("expected one post-resume command, got %d: %v", len(tc.postResumeRuns), tc.postResumeRuns)
	}
	if tc.postResumeRuns[0] != command {
		return fmt.Errorf("expected post-resume command %q, got %q", command, tc.postResumeRuns[0])
	}
	return nil
}

func (tc *TUITestContext) noPostResumeCommandShouldRun() error {
	tc.drainWithTimeout(20 * time.Millisecond)
	if len(tc.postResumeRuns) != 0 {
		return fmt.Errorf("expected no post-resume command, got %v", tc.postResumeRuns)
	}
	return nil
}

func (tc *TUITestContext) worktreeCreationIsDelayed() error {
	tc.fakeWorktreeManager.delayWorktreeCreation()
	return nil
}

func (tc *TUITestContext) worktreeCreationCompletes() error {
	tc.fakeWorktreeManager.completeDelayedWorktreeCreation()
	return tc.waitForOneAsyncMessage(2 * time.Second)
}

// InitializeScenario initializes godog with our step definitions
func InitializeScenario(ctx *godog.ScenarioContext, t *testing.T) {
	tc := NewTUITestContext(t)

	// Setup a test context for each scenario
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		tc.fakeClient = linear.NewFakeLinearClient()
		tc.fakeWorktreeManager = &testWorktreeManager{}
		tc.defaultWorktreeCmd = ""
		tc.resumeWorktreeCmd = ""
		tc.postCreateRuns = nil
		tc.postResumeRuns = nil
		tc.postCreateRan = false
		tc.postResumeRan = false
		tc.pendingMsgs = make(chan tea.Msg, 64)
		tc.t = t              // Ensure t is set for each scenario
		tc.terminalWidth = 80 // Reset to default
		tc.terminalHeight = 24
		return ctx, nil
	})

	// Step definitions
	ctx.Step(`^the following Linear issues exist:$`, tc.theFollowingLinearIssuesExist)
	ctx.Step(`^the following worktrees exist:$`, tc.theFollowingWorktreesExist)
	ctx.Step(`^fetching children for "([^"]*)" fails$`, tc.fetchingChildrenForFails)
	ctx.Step(`^a config with:$`, tc.aConfigWith)
	ctx.Step(`^my terminal width is (\d+) characters$`, tc.myTerminalWidthIsCharacters)
	ctx.Step(`^I start the Sprout TUI$`, tc.iStartTheSproutTUI)
	ctx.Step(`^I press "([^"]*)"$`, tc.iPress)
	ctx.Step(`^I type "([^"]*)"$`, tc.iType)
	ctx.Step(`^I type the following text:$`, tc.iTypeTheFollowingText)
	ctx.Step(`^the UI should display:$`, tc.theUIShouldDisplay)
	ctx.Step(`^the UI should display "([^"]*)"$`, tc.theUIShouldDisplayText)
	ctx.Step(`^the UI should not display "([^"]*)"$`, tc.theUIShouldNotDisplay)
	ctx.Step(`^the UI should contain "([^"]*)"$`, tc.theUIShouldContain)
	ctx.Step(`^the following commands should be run:$`, tc.theFollowingCommandsShouldBeRun)
	ctx.Step(`^the TUI should resume worktree "([^"]*)"$`, tc.theTUIShouldResumeWorktree)
	ctx.Step(`^no new worktree should be created$`, tc.noNewWorktreeShouldBeCreated)
	ctx.Step(`^a worktree should be created for branch "([^"]*)"$`, tc.aWorktreeShouldBeCreatedForBranch)
	ctx.Step(`^(\d+) active work queue rows exist$`, tc.activeWorkQueueRowsExist)
	ctx.Step(`^the UI should show (\d+) work queue rows$`, tc.theUIShouldShowWorkQueueRows)
	ctx.Step(`^the post-resume command should be "([^"]*)"$`, tc.postResumeCommandShouldBe)
	ctx.Step(`^no post-resume command should run$`, tc.noPostResumeCommandShouldRun)
	ctx.Step(`^the default worktree command is "([^"]*)"$`, tc.theDefaultWorktreeCommandIs)
	ctx.Step(`^the default worktree command is "([^"]*)"\$PROMPT\\"([^"]*)"$`, func(prefix, suffix string) error {
		return tc.theDefaultWorktreeCommandIs(prefix + "$PROMPT" + suffix)
	})
	ctx.Step(`^the default worktree command is "([^"]*)"\$PROMPT\\"([^"]*)"\$PROMPT\\"([^"]*)"$`, func(prefix, middle, suffix string) error {
		return tc.theDefaultWorktreeCommandIs(prefix + "$PROMPT" + middle + "$PROMPT" + suffix)
	})
	ctx.Step(`^worktree creation is delayed$`, tc.worktreeCreationIsDelayed)
	ctx.Step(`^worktree creation completes$`, tc.worktreeCreationCompletes)
	ctx.Step(`^the UI should display titles truncated to fit the available width$`, tc.theUIShouldDisplayTitlesTruncatedToFitTheAvailableWidth)
}

// TestFeatures runs the Gherkin tests
func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			InitializeScenario(ctx, t)
		},
		Options: &godog.Options{
			Format: "pretty",
			Paths: []string{
				"../../features/async_prompt.feature",
				"../../features/duplicate_handling.feature",
				"../../features/expansion.feature",
				"../../features/interaction.feature",
				"../../features/navigation.feature",
				"../../features/resume_command.feature",
				"../../features/resume_work_queue.feature",
				"../../features/search.feature",
				"../../features/window_width.feature",
			},
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}
