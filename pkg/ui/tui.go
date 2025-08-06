package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"sprout/pkg/git"
)

type model struct {
	input           string
	cursor          int
	submitted       bool
	creating        bool
	done            bool
	success         bool
	errorMsg        string
	result          string
	worktreeManager *git.WorktreeManager
}

func NewTUI() (model, error) {
	wm, err := git.NewWorktreeManager()
	if err != nil {
		return model{}, err
	}

	return model{
		input:           "",
		cursor:          0,
		submitted:       false,
		creating:        false,
		done:            false,
		success:         false,
		errorMsg:        "",
		result:          "",
		worktreeManager: wm,
	}, nil
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.done {
			return m, tea.Quit
		}

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyEnter:
			if strings.TrimSpace(m.input) != "" && !m.submitted {
				m.submitted = true
				m.creating = true
				return m, m.createWorktree()
			}

		case tea.KeyBackspace:
			if m.cursor > 0 && !m.submitted {
				m.input = m.input[:m.cursor-1] + m.input[m.cursor:]
				m.cursor--
			}

		case tea.KeyLeft:
			if m.cursor > 0 && !m.submitted {
				m.cursor--
			}

		case tea.KeyRight:
			if m.cursor < len(m.input) && !m.submitted {
				m.cursor++
			}

		case tea.KeyRunes:
			if !m.submitted {
				m.input = m.input[:m.cursor] + string(msg.Runes) + m.input[m.cursor:]
				m.cursor += len(msg.Runes)
			}
		}

	case worktreeCreatedMsg:
		m.creating = false
		m.done = true
		m.success = true
		m.result = fmt.Sprintf("Worktree created at: %s", msg.path)
		return m, tea.Quit

	case errMsg:
		m.creating = false
		m.done = true
		m.success = false
		m.errorMsg = msg.err.Error()
		return m, tea.Quit
	}

	return m, nil
}

func (m model) createWorktree() tea.Cmd {
	return func() tea.Msg {
		branchName := strings.TrimSpace(m.input)
		worktreePath, err := m.worktreeManager.CreateWorktree(branchName)
		if err != nil {
			return errMsg{err}
		}
		return worktreeCreatedMsg{branchName, worktreePath}
	}
}

type errMsg struct {
	err error
}

type worktreeCreatedMsg struct {
	branch string
	path   string
}

func (m model) View() string {
	if m.done {
		if m.success {
			return fmt.Sprintf("✅ %s\n\nPress any key to exit.\n", m.result)
		} else {
			return fmt.Sprintf("❌ Error: %s\n\nPress any key to exit.\n", m.errorMsg)
		}
	}

	if m.creating {
		return "Creating worktree...\n"
	}

	s := strings.Builder{}
	s.WriteString("🌱 Sprout - Create New Worktree\n\n")
	s.WriteString("Enter branch name: ")
	
	// Display input with cursor
	for i, r := range m.input {
		if i == m.cursor {
			s.WriteString("│")
		}
		s.WriteRune(r)
	}
	if m.cursor == len(m.input) {
		s.WriteString("│")
	}
	
	s.WriteString("\n\n")
	s.WriteString("Press Enter to create, Esc/Ctrl+C to quit")
	
	return s.String()
}

func RunInteractive() error {
	model, err := NewTUI()
	if err != nil {
		return err
	}

	p := tea.NewProgram(model)
	_, err = p.Run()
	return err
}