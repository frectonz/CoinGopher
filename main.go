package main

import (
	"encoding/json"
	"errors"
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
	Note  string
	Value float64
	Kind  Kind
}

func (t txn) Title() string { return fmt.Sprintf("[%s]", t.Note) }

var kindString = map[Kind]string{
	Credit: "+",
	Debit:  "-",
}

func (t txn) Description() string {
	return fmt.Sprintf("%s %f", kindString[t.Kind], t.Value)
}

func (t txn) FilterValue() string { return t.Note }

type model struct {
	filename   string
	txns       []txn
	txnsList   list.Model
	focusIndex int
	inputs     []textinput.Model
	kind       Kind
	cursorMode cursor.Mode
	focusArea  FocusArea
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func initialModel(filename string) model {
	var txns []txn

	data, err := os.ReadFile(filename)

	if errors.Is(err, os.ErrNotExist) {
		file, err := os.Create(filename)
		check(err)

		txns = make([]txn, 0)
		data, err := json.Marshal(txns)
		check(err)

		_, err = file.Write(data)
		check(err)
	} else {
		check(err)

		err = json.Unmarshal(data, &txns)
		check(err)
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle.Foreground(green).BorderForeground(green)
	delegate.Styles.SelectedDesc.Foreground(green).BorderForeground(green)

	items := make([]list.Item, len(txns))
	for i, txn := range txns {
		items[i] = txn
	}

	list := list.New(items, delegate, 0, 0)
	list.SetShowTitle(false)

	m := model{
		filename:  filename,
		txns:      txns,
		txnsList:  list,
		inputs:    make([]textinput.Model, 2),
		focusArea: List,
		kind:      Credit,
	}

	placeholders := []string{"Transaction Note", "Transaction Value"}
	for i := range m.inputs {
		t := textinput.New()
		t.Cursor.Style = cursorStyle
		t.CharLimit = 32
		t.Placeholder = placeholders[i]

		m.inputs[i] = t
	}
	m.inputs[0].Focus()
	m.inputs[0].PromptStyle = focusedStyle
	m.inputs[0].TextStyle = focusedStyle

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
				check(err)

				new_txn := txn{
					Note:  note,
					Value: value,
					Kind:  m.kind,
				}

				m.txns = append(m.txns, new_txn)

				data, err := json.Marshal(m.txns)
				check(err)
				err = os.WriteFile(m.filename, data, 0644)
				check(err)

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
		switch txn.Kind {
		case Credit:
			balance += txn.Value
		case Debit:
			balance -= txn.Value
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
	if len(os.Args) != 2 {
		fmt.Println("Provide a file to store the transactions")
		os.Exit(1)
	}

	if _, err := tea.NewProgram(initialModel(os.Args[1])).Run(); err != nil {
		fmt.Printf("could not start program: %s\n", err)
		os.Exit(1)
	}
}
