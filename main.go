package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	green        = lipgloss.Color("083")
	appStyle     = lipgloss.NewStyle().Padding(1, 2)
	titleStyle   = lipgloss.NewStyle().Background(green).Foreground(lipgloss.Color("000")).Padding(0, 1)
	focusedStyle = lipgloss.NewStyle().Foreground(green)
	blurredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle  = focusedStyle.Copy()
	noStyle      = lipgloss.NewStyle()
	helpStyle    = blurredStyle.Copy()
	banner       = lipgloss.NewStyle().Foreground(green).Bold(true).Render(`
         ,_---~~~~~----._
  _,,_,*^____      _____  *g*\"*,      ____      _        ____             _
 / __/ /'     ^.  /      \ ^@q   f    / ___|___ (_)_ __  / ___| ___  _ __ | |__   ___ _ __
[  @f | @))    |  | @))   l  0 _/    | |   / _ \| | '_ \| |  _ / _ \| '_ \| '_ \ / _ \ '__|
 \ /   \~____ / __ \_____/    \      | |__| (_) | | | | | |_| | (_) | |_) | | | |  __/ |
  |           _l__l_           I      \____\___/|_|_| |_|\____|\___/| .__/|_| |_|\___|_|
  }          [______]           I                                   |_|
  ]            | | |            |
  ]             ~ ~             |
  |                            |
   |                           |
`)

	focusedButton = focusedStyle.Copy().Render("[ Submit ]")
	blurredButton = fmt.Sprintf("[ %s ]", blurredStyle.Render("Submit"))
)

type Kind bool

const (
	Credit Kind = true
	Debit  Kind = false
)

type FocusArea bool

const (
	List FocusArea = true
	Form FocusArea = false
)

type txn struct {
	note  string
	value float64
	kind  Kind
}

func (t txn) Title() string { return fmt.Sprintf("[%s]", t.note) }
func (t txn) Description() string {
	switch t.kind {
	case Credit:
		return fmt.Sprintf("+ %f", t.value)
	case Debit:
		return fmt.Sprintf("- %f", t.value)
	}

	return ""
}
func (t txn) FilterValue() string { return t.note }

type model struct {
	txns       []txn
	txnsList   list.Model
	focusIndex int
	inputs     []textinput.Model
	kind       Kind
	cursorMode cursor.Mode
	focusArea  FocusArea
}

func initialModel() model {

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle.Foreground(green).BorderForeground(green)
	delegate.Styles.SelectedDesc.Foreground(green).BorderForeground(green)

	list := list.New(make([]list.Item, 0), delegate, 0, 0)
	list.SetShowTitle(false)

	m := model{
		txns:      make([]txn, 0),
		txnsList:  list,
		inputs:    make([]textinput.Model, 2),
		focusArea: List,
		kind:      Credit,
	}

	var t textinput.Model
	for i := range m.inputs {
		t = textinput.New()
		t.Cursor.Style = cursorStyle
		t.CharLimit = 32

		switch i {
		case 0:
			t.Placeholder = "Transaction Note"
			t.Focus()
			t.PromptStyle = focusedStyle
			t.TextStyle = focusedStyle
			t.CharLimit = 64
		case 1:
			t.Placeholder = "Transaction Value"
		}

		m.inputs[i] = t
	}

	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tea.EnterAltScreen)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := appStyle.GetFrameSize()
		m.txnsList.SetSize(msg.Width-h, msg.Height-v-20)
	case tea.KeyMsg:
		switch msg.String() {
		case "+":
			m.focusArea = Form
			return m, nil

		case "-":
			m.focusArea = List
			return m, nil

		case "ctrl+c":
			return m, tea.Quit

		// Set focus to next input
		case "tab", "shift+tab", "enter", "up", "down":
			if m.focusArea == List {
				return m, nil
			}

			s := msg.String()

			if s == "enter" && m.focusIndex == 2 {
				m.kind = Credit
				return m, nil
			}
			if s == "enter" && m.focusIndex == 3 {
				m.kind = Debit
				return m, nil
			}

			// Did the user press enter while the submit button was focused?
			if s == "enter" && m.focusIndex == len(m.inputs)+2 {
				note := m.inputs[0].Value()
				valueStr := m.inputs[1].Value()

				value, err := strconv.ParseFloat(valueStr, 64)
				if err != nil {
					panic(err)
				}

				new_txn := txn{
					note:  note,
					value: value,
					kind:  m.kind,
				}

				m.txns = append(m.txns, new_txn)

				for i := range m.inputs {
					m.inputs[i].Reset()
				}

				items := make([]list.Item, len(m.txns))
				for i, t := range m.txns {
					items[i] = t
				}

				m.focusIndex = -1
				m.focusArea = List

				cmd = tea.Batch(cmd, m.txnsList.SetItems(items))
			}

			// Cycle indexes
			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}

			if m.focusIndex > len(m.inputs)+2 {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs) + 2
			}

			cmds := make([]tea.Cmd, len(m.inputs))
			for i := 0; i <= len(m.inputs)-1; i++ {
				if i == m.focusIndex {
					// Set focused state
					cmds[i] = m.inputs[i].Focus()
					m.inputs[i].PromptStyle = focusedStyle
					m.inputs[i].TextStyle = focusedStyle
					continue
				}
				// Remove focused state
				m.inputs[i].Blur()
				m.inputs[i].PromptStyle = noStyle
				m.inputs[i].TextStyle = noStyle
			}

			return m, tea.Batch(cmds...)
		}
	}

	if m.focusArea == List {
		var listCmd tea.Cmd
		m.txnsList, listCmd = m.txnsList.Update(msg)
		return m, tea.Batch(cmd, listCmd)
	} else {
		// Handle character input and blinking
		cmd = tea.Batch(cmd, m.updateInputs(msg))
		return m, cmd
	}

}

func (m *model) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	// Only text inputs with Focus() set will respond, so it's safe to simply
	// update all of them here without any further logic.
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return tea.Batch(cmds...)
}

func (m model) View() string {
	var b strings.Builder

	b.WriteString(banner)
	b.WriteString("\n\n")

	var balance float64
	for _, txn := range m.txns {
		switch txn.kind {
		case Credit:
			balance += txn.value
		case Debit:
			balance -= txn.value
		}
	}

	var listArea strings.Builder
	listArea.WriteString(titleStyle.Render("Balance"))
	listArea.WriteString(focusedStyle.Render(fmt.Sprintf(" %f", balance)))
	listArea.WriteString("\n\n")

	listArea.WriteString(m.txnsList.View())
	listArea.WriteString("\n\n")

	listArea.WriteString(helpStyle.Render("Use + to add a new transaction"))

	if m.focusArea == List {
		b.WriteString(listArea.String())
	}

	var formArea strings.Builder
	formArea.WriteString(titleStyle.Render("Add New Transaction"))
	formArea.WriteString("\n\n")
	for i := range m.inputs {
		formArea.WriteString(m.inputs[i].View())
		if i < len(m.inputs)-1 {
			formArea.WriteRune('\n')
		}
	}

	formArea.WriteString("\n\n")

	switch m.kind {
	case Credit:
		if m.focusIndex == 2 {
			formArea.WriteString(focusedStyle.Render("[x] Credit"))
		} else {
			formArea.WriteString(blurredStyle.Render("[x] Credit"))
		}

		formArea.WriteRune('\n')

		if m.focusIndex == 3 {
			formArea.WriteString(focusedStyle.Render("[ ] Debit"))
		} else {
			formArea.WriteString(blurredStyle.Render("[ ] Debit"))

		}
	case Debit:
		if m.focusIndex == 2 {
			formArea.WriteString(focusedStyle.Render("[ ] Credit"))
		} else {
			formArea.WriteString(blurredStyle.Render("[ ] Credit"))
		}

		formArea.WriteRune('\n')

		if m.focusIndex == 3 {
			formArea.WriteString(focusedStyle.Render("[x] Debit"))
		} else {
			formArea.WriteString(blurredStyle.Render("[x] Debit"))

		}
	}

	button := &blurredButton
	if m.focusIndex == len(m.inputs)+2 {
		button = &focusedButton
	}
	fmt.Fprintf(&formArea, "\n\n%s\n\n", *button)
	formArea.WriteString(helpStyle.Render("Use - to return to transaction list"))

	if m.focusArea == Form {
		b.WriteString(formArea.String())
	}

	return appStyle.Render(b.String())
}

func main() {
	if _, err := tea.NewProgram(initialModel()).Run(); err != nil {
		fmt.Printf("could not start program: %s\n", err)
		os.Exit(1)
	}
}