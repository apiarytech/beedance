/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either license.
 *
 * See the LICENSE files in the project root for full license text.
 */
package tui

import (
	"beedance/evaluator"
	"beedance/lexer"
	"beedance/object"
	"beedance/parser"
	"beedance/token"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	lg "github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	tree "github.com/mariusor/bubbles-tree"
)

// menuItem represents a top-level menu item
type menuItem struct {
	name  string
	items []string // For now, just a list of names for sub-items
}

var menuItems = []menuItem{
	{name: "File", items: []string{"New Project", "Open...", "Close Project", "Save", "Save As...", "Import/Export", "Print", "Exit"}},
	{name: "Build", items: []string{"Lex/Parse/Evaluate", "Compile", "Run/Pause/Stop"}},
	{name: "Edit", items: []string{"Undo/Redo", "Cut/Copy/Paste/Delete", "Select All", "Find/Replace/Browse"}},
	{name: "View", items: []string{"Show/Hide panels: Left Right and Bottom"}},
}

// A simple item for the tree bubble, implementing the tree.Node interface.
type treeNode struct {
	data     string
	children []tree.Node
}

func (n treeNode) Data() interface{}     { return n.data }
func (n treeNode) Children() []tree.Node { return n.children }

// focusablePanel is an enum for the different focusable areas of the TUI.
type focusablePanel int

// promptModeState defines the reason a prompt is being shown.
type promptModeState int

const (
	menuPanel focusablePanel = iota
	projectTreePanel
	editorPanel
	builtinsTreePanel
)
const (
	promptModeNone promptModeState = iota
	promptModeSaveAs
	promptModeConfirmClose
	promptModeConfirmNew
	promptModeOpen
)

// Define styles for the panels
var (
	docStyle = lg.NewStyle().Margin(1, 2)

	// Style for a selected menu item
	selectedMenuItemStyle = lg.NewStyle().
				Background(lg.Color("240")).
				Foreground(lg.Color("0"))

	// Style for a regular menu item
	menuItemStyle = lg.NewStyle()

	topPanelStyle = lg.NewStyle().
			Border(lg.NormalBorder(), false, false, true, false).
			BorderForeground(lg.Color("240"))

	bottomPanelStyle = lg.NewStyle().
				Border(lg.NormalBorder(), true, false, false, false).
				BorderForeground(lg.Color("240"))

	leftPanelStyle = lg.NewStyle().
			Border(lg.NormalBorder(), false, true, false, false).
			BorderForeground(lg.Color("240"))

	focusedLeftPanelStyle = lg.NewStyle().
				Border(lg.NormalBorder(), false, true, false, false).
				BorderForeground(lg.Color("86")) // Green focus border

	focusedRightPanelStyle = lg.NewStyle().
				Border(lg.NormalBorder(), false, false, false, true).
				BorderForeground(lg.Color("86")) // Green focus border

	rightPanelStyle = lg.NewStyle().
			Border(lg.NormalBorder(), false, false, false, true).
			BorderForeground(lg.Color("240"))

	middlePanelStyle = lg.NewStyle().
				Border(lg.NormalBorder()).
				BorderForeground(lg.Color("240"))

	focusedMiddlePanelStyle = lg.NewStyle().
				Border(lg.NormalBorder()).
				BorderForeground(lg.Color("86")) // A bright green for focus

)

type model struct {
	width, height      int
	env                *object.Environment
	err                error
	selectedMenu       int // Index of the selected top-level menu
	tree               tree.Model
	builtinsTree       tree.Model
	log                *log.Logger
	logOutput          *strings.Builder
	logViewport        viewport.Model
	textarea           textarea.Model
	lastTreeSelection  string
	pousContent        map[string]string            // Maps POU identifier to its code content
	styles             map[token.TokenType]lg.Style // For syntax highlighting
	middleViewport     viewport.Model
	editorFocus        bool
	focusIndex         focusablePanel
	menuOpen           bool
	selectedMenuItem   int
	lexerOk            bool
	parserOk           bool
	evaluatorOk        bool
	compileOk          bool
	isDirty            bool
	promptMode         promptModeState
	promptInput        textarea.Model
	currentProjectPath string
}

func InitialModel() model {
	// Create the tree structure based on the IEC 61131-3 software model.
	rootNode := treeNode{
		data: "CONFIGURATION CELL_1",
		children: []tree.Node{
			treeNode{
				data: "RESOURCE STATION_1",
				children: []tree.Node{
					treeNode{data: "TASK SLOW_1", children: []tree.Node{treeNode{data: "PROGRAM P1"}}},
					treeNode{data: "TASK FAST_1", children: []tree.Node{treeNode{data: "PROGRAM P2"}}},
				},
			},
			treeNode{
				data: "RESOURCE STATION_2",
				children: []tree.Node{
					treeNode{data: "TASK PER_2", children: []tree.Node{treeNode{data: "PROGRAM P1"}}},
					treeNode{data: "TASK INT_2", children: []tree.Node{treeNode{data: "PROGRAM P4"}}},
				},
			},
		},
	}

	t := tree.New(rootNode)

	// Create the built-ins tree structure.
	builtinsRoot := treeNode{
		data: "Snippets",
		children: []tree.Node{
			treeNode{
				data: "BUILTINS",
				children: []tree.Node{
					treeNode{data: "ADD"},
					treeNode{data: "SUB"},
					treeNode{data: "TON"},
					treeNode{data: "CTU"},
					treeNode{data: "R_TRIG"},
				},
			},
			treeNode{
				data: "MATH",
				children: []tree.Node{
					treeNode{data: "SIN"},
					treeNode{data: "COS"},
					treeNode{data: "TAN"},
					treeNode{data: "SQRT"},
				},
			},
		},
	}

	bt := tree.New(builtinsRoot)

	// --- Textarea setup ---
	ta := textarea.New()
	ta.Placeholder = "Select a POU from the tree to start editing..."
	ta.ShowLineNumbers = true
	// ta.Focus() // We will focus it manually

	prompt := textarea.New()
	prompt.Placeholder = "Enter filename and press Enter..."
	prompt.SetHeight(1)
	prompt.SetWidth(50)
	prompt.Blur()

	// --- Syntax Highlighting Styles ---
	styles := map[token.TokenType]lg.Style{
		// Keywords for control flow
		token.IF:         lg.NewStyle().Foreground(lg.Color("205")), // Magenta
		token.THEN:       lg.NewStyle().Foreground(lg.Color("205")),
		token.ELSE:       lg.NewStyle().Foreground(lg.Color("205")),
		token.ELSIF:      lg.NewStyle().Foreground(lg.Color("205")),
		token.END_IF:     lg.NewStyle().Foreground(lg.Color("205")),
		token.CASE:       lg.NewStyle().Foreground(lg.Color("205")),
		token.OF:         lg.NewStyle().Foreground(lg.Color("205")),
		token.END_CASE:   lg.NewStyle().Foreground(lg.Color("205")),
		token.FOR:        lg.NewStyle().Foreground(lg.Color("205")),
		token.TO:         lg.NewStyle().Foreground(lg.Color("205")),
		token.BY:         lg.NewStyle().Foreground(lg.Color("205")),
		token.DO:         lg.NewStyle().Foreground(lg.Color("205")),
		token.END_FOR:    lg.NewStyle().Foreground(lg.Color("205")),
		token.WHILE:      lg.NewStyle().Foreground(lg.Color("205")),
		token.END_WHILE:  lg.NewStyle().Foreground(lg.Color("205")),
		token.REPEAT:     lg.NewStyle().Foreground(lg.Color("205")),
		token.UNTIL:      lg.NewStyle().Foreground(lg.Color("205")),
		token.END_REPEAT: lg.NewStyle().Foreground(lg.Color("205")),
		token.RETURN:     lg.NewStyle().Foreground(lg.Color("205")),
		token.EXIT:       lg.NewStyle().Foreground(lg.Color("205")),

		// Keywords for program structure (POUs, VAR blocks, etc.)
		token.PROGRAM:            lg.NewStyle().Foreground(lg.Color("81")), // Blue
		token.END_PROGRAM:        lg.NewStyle().Foreground(lg.Color("81")),
		token.FUNCTION:           lg.NewStyle().Foreground(lg.Color("81")),
		token.END_FUNCTION:       lg.NewStyle().Foreground(lg.Color("81")),
		token.FUNCTION_BLOCK:     lg.NewStyle().Foreground(lg.Color("81")),
		token.END_FUNCTION_BLOCK: lg.NewStyle().Foreground(lg.Color("81")),
		token.VAR:                lg.NewStyle().Foreground(lg.Color("81")),
		token.END_VAR:            lg.NewStyle().Foreground(lg.Color("81")),
		token.VAR_INPUT:          lg.NewStyle().Foreground(lg.Color("81")),
		token.VAR_OUTPUT:         lg.NewStyle().Foreground(lg.Color("81")),
		token.VAR_IN_OUT:         lg.NewStyle().Foreground(lg.Color("81")),
		token.VAR_TEMP:           lg.NewStyle().Foreground(lg.Color("81")),
		token.VAR_GLOBAL:         lg.NewStyle().Foreground(lg.Color("81")),
		token.VAR_EXTERNAL:       lg.NewStyle().Foreground(lg.Color("81")),
		token.TYPE:               lg.NewStyle().Foreground(lg.Color("81")),
		token.END_TYPE:           lg.NewStyle().Foreground(lg.Color("81")),
		token.STRUCT:             lg.NewStyle().Foreground(lg.Color("81")),
		token.END_STRUCT:         lg.NewStyle().Foreground(lg.Color("81")),
		token.CONFIGURATION:      lg.NewStyle().Foreground(lg.Color("81")),
		token.END_CONFIGURATION:  lg.NewStyle().Foreground(lg.Color("81")),
		token.RESOURCE:           lg.NewStyle().Foreground(lg.Color("81")),
		token.END_RESOURCE:       lg.NewStyle().Foreground(lg.Color("81")),
		token.ON:                 lg.NewStyle().Foreground(lg.Color("81")),
		token.ACTION:             lg.NewStyle().Foreground(lg.Color("81")),
		token.END_ACTION:         lg.NewStyle().Foreground(lg.Color("81")),
		token.STEP:               lg.NewStyle().Foreground(lg.Color("81")),
		token.END_STEP:           lg.NewStyle().Foreground(lg.Color("81")),
		token.INITIAL_STEP:       lg.NewStyle().Foreground(lg.Color("81")),
		token.TRANSITION:         lg.NewStyle().Foreground(lg.Color("81")),
		token.END_TRANSITION:     lg.NewStyle().Foreground(lg.Color("81")),
		token.FROM:               lg.NewStyle().Foreground(lg.Color("81")),

		// Keywords for variable attributes
		token.RETAIN:     lg.NewStyle().Foreground(lg.Color("214")), // Gold
		token.NON_RETAIN: lg.NewStyle().Foreground(lg.Color("214")),
		token.CONSTANT:   lg.NewStyle().Foreground(lg.Color("214")),
		token.AT:         lg.NewStyle().Foreground(lg.Color("214")),

		// Data Types
		token.BOOL:          lg.NewStyle().Foreground(lg.Color("172")), // Orange
		token.SINT:          lg.NewStyle().Foreground(lg.Color("172")),
		token.INT:           lg.NewStyle().Foreground(lg.Color("172")),
		token.DINT:          lg.NewStyle().Foreground(lg.Color("172")),
		token.LINT:          lg.NewStyle().Foreground(lg.Color("172")),
		token.USINT:         lg.NewStyle().Foreground(lg.Color("172")),
		token.UINT:          lg.NewStyle().Foreground(lg.Color("172")),
		token.UDINT:         lg.NewStyle().Foreground(lg.Color("172")),
		token.ULINT:         lg.NewStyle().Foreground(lg.Color("172")),
		token.REAL:          lg.NewStyle().Foreground(lg.Color("172")),
		token.LREAL:         lg.NewStyle().Foreground(lg.Color("172")),
		token.TIME:          lg.NewStyle().Foreground(lg.Color("172")),
		token.DATE:          lg.NewStyle().Foreground(lg.Color("172")),
		token.TIME_OF_DAY:   lg.NewStyle().Foreground(lg.Color("172")),
		token.DATE_AND_TIME: lg.NewStyle().Foreground(lg.Color("172")),
		token.STRING:        lg.NewStyle().Foreground(lg.Color("172")),
		token.WSTRING:       lg.NewStyle().Foreground(lg.Color("172")),
		token.BYTE:          lg.NewStyle().Foreground(lg.Color("172")),
		token.WORD:          lg.NewStyle().Foreground(lg.Color("172")),
		token.DWORD:         lg.NewStyle().Foreground(lg.Color("172")),
		token.LWORD:         lg.NewStyle().Foreground(lg.Color("172")),
		token.ARRAY:         lg.NewStyle().Foreground(lg.Color("172")),

		// Literals
		token.TRUE:            lg.NewStyle().Foreground(lg.Color("135")), // Purple
		token.FALSE:           lg.NewStyle().Foreground(lg.Color("135")),
		token.STRING_LITERAL:  lg.NewStyle().Foreground(lg.Color("114")), // Green
		token.WSTRING_LITERAL: lg.NewStyle().Foreground(lg.Color("114")),
		token.INTEGER_LITERAL: lg.NewStyle().Foreground(lg.Color("135")),
		token.REAL_LITERAL:    lg.NewStyle().Foreground(lg.Color("135")),

		// Identifiers and others
		token.IDENT:   lg.NewStyle().Foreground(lg.Color("229")), // Light Yellow
		token.ILLEGAL: lg.NewStyle().Foreground(lg.Color("196")).Background(lg.Color("235")),
	}

	// --- Log setup ---
	logOutput := &strings.Builder{}
	logger := log.New(logOutput)
	logger.SetLevel(log.DebugLevel)
	logger.SetTimeFormat("15:04:05")

	vp := viewport.New(0, 0) // Will be sized in View()
	middleVp := viewport.New(0, 0)
	middleVp.SetContent("Select a POU from the tree to view its code. Press Enter to edit.")

	m := model{
		env:                object.NewEnvironment(),
		err:                nil,
		selectedMenu:       0, // Start with "File" selected
		tree:               t,
		builtinsTree:       bt,
		log:                logger,
		logOutput:          logOutput,
		logViewport:        vp, // cspell:disable-line
		textarea:           ta,
		lastTreeSelection:  rootNode.Data().(string),
		pousContent:        make(map[string]string), // cspell:disable-line
		styles:             styles,
		middleViewport:     middleVp,
		editorFocus:        false,
		focusIndex:         menuPanel, // Start with the menu focused
		menuOpen:           false,
		selectedMenuItem:   0,
		lexerOk:            true,
		parserOk:           true,
		evaluatorOk:        true,
		compileOk:          true,
		promptMode:         promptModeNone,
		promptInput:        prompt,
		currentProjectPath: "beedance_project.json",
	}

	m.log.Info("Beedance TUI Initialized. Press 'm' for message, 'w' for warning, 'e' for error.")
	initialContent := fmt.Sprintf("Selected: %s", m.lastTreeSelection)
	m.middleViewport.SetContent(m.highlightWithLineNumbers(initialContent))
	m.textarea.SetValue(initialContent)                 // Keep textarea in sync
	m.pousContent[m.lastTreeSelection] = initialContent // Store initial content

	return m
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.promptMode != promptModeNone {
			switch m.promptMode {
			case promptModeSaveAs, promptModeOpen:
				switch msg.Type {
				case tea.KeyEsc:
					m.promptMode = promptModeNone
					m.promptInput.Blur()
				case tea.KeyEnter:
					if m.promptMode == promptModeSaveAs {
						m.currentProjectPath = m.promptInput.Value()
						m.saveProject()
					} else if m.promptMode == promptModeOpen {
						m.currentProjectPath = m.promptInput.Value()
						m.loadProject()
					}
					m.promptMode = promptModeNone
					m.promptInput.Blur()
				default:
					m.promptInput, cmd = m.promptInput.Update(msg)
					cmds = append(cmds, cmd)
				}
			case promptModeConfirmNew, promptModeConfirmClose:
				switch strings.ToLower(msg.String()) {
				case "y":
					m.saveProject()
					m.newProject()
					m.promptMode = promptModeNone
				case "n":
					m.newProject() // "Closing" is the same as creating a new, blank project.
					m.promptMode = promptModeNone
				case "esc":
					m.promptMode = promptModeNone
				}
			}
			// If it was a text input prompt, we need to batch the command.
			if m.promptMode == promptModeSaveAs || m.promptMode == promptModeOpen {
				return m, tea.Batch(cmds...)
			}
			return m, tea.Batch(cmds...)
		} else { // No prompt is active, handle normal TUI interaction
			// Global keybindings first
			if msg.Type == tea.KeyCtrlC {
				return m, tea.Quit
			}

			// Handle F5 for evaluation
			if msg.Type == tea.KeyF5 {
				m.evaluateCode()
				return m, nil
			}

			// Handle focus switching with Tab
			switch msg.Type {
			case tea.KeyTab:
				m.focusIndex = (m.focusIndex + 1) % 4 // Cycle through 4 panels
				m.editorFocus = (m.focusIndex == editorPanel)
				if m.editorFocus {
					return m, m.textarea.Focus()
				} else {
					m.textarea.Blur()
				}
				return m, nil
			case tea.KeyShiftTab:
				m.focusIndex = (m.focusIndex - 1 + 4) % 4
				m.editorFocus = (m.focusIndex == editorPanel)
				if m.editorFocus {
					return m, m.textarea.Focus()
				} else {
					m.textarea.Blur()
				}
				return m, nil
			}

			// Route key messages to the focused panel
			switch m.focusIndex {
			case menuPanel:
				if m.menuOpen {
					switch msg.Type {
					case tea.KeyUp:
						if m.selectedMenuItem > 0 {
							m.selectedMenuItem--
						}
					case tea.KeyDown:
						if m.selectedMenuItem < len(menuItems[m.selectedMenu].items)-1 {
							m.selectedMenuItem++
						}
					case tea.KeyEnter:
						selectedMenuName := menuItems[m.selectedMenu].name
						selectedItemName := menuItems[m.selectedMenu].items[m.selectedMenuItem]

						if selectedMenuName == "File" {
							switch selectedItemName {
							case "Save":
								m.saveProject()
							case "Save As...":
								m.promptMode = promptModeSaveAs
								m.promptInput.SetValue(m.currentProjectPath)
								m.menuOpen = false
								return m, m.promptInput.Focus()
							case "Open...":
								m.promptMode = promptModeOpen
								m.promptInput.SetValue(m.currentProjectPath)
								m.menuOpen = false
								return m, m.promptInput.Focus()
							case "Exit":
								return m, tea.Quit
							default:
								m.log.Infof("Menu item selected: %s -> %s", selectedMenuName, selectedItemName)
							}
						} else if selectedMenuName == "Build" && selectedItemName == "Lex/Parse/Evaluate" {
							m.evaluateCode()
						} else {
							m.log.Infof("Menu item selected: %s -> %s", selectedMenuName, selectedItemName)
						}

						m.menuOpen = false
					case tea.KeyEsc:
						m.menuOpen = false
					}
				} else {
					switch msg.Type {
					case tea.KeyLeft:
						m.selectedMenu = (m.selectedMenu - 1 + len(menuItems)) % len(menuItems)
					case tea.KeyRight:
						m.selectedMenu = (m.selectedMenu + 1) % len(menuItems)
					case tea.KeyEnter:
						m.menuOpen = true
						m.selectedMenuItem = 0
					case tea.KeyEsc:
						// If menu is not open, Esc does nothing here, but could be used to unfocus
					}
				}
				return m, nil

			case projectTreePanel:
				m.tree, cmd = m.tree.Update(msg)
				cmds = append(cmds, cmd)

			case editorPanel:
				oldValue := m.textarea.Value()
				m.textarea, cmd = m.textarea.Update(msg)
				if m.textarea.Value() != oldValue {
					m.isDirty = true
				}
				cmds = append(cmds, cmd)

			case builtinsTreePanel:
				if msg.Type == tea.KeyEnter {
					selected := m.builtinsTree.GetSelected()
					if selected != nil && len(selected.Children()) == 0 {
						m.textarea.InsertString(selected.Data().(string))
						m.isDirty = true
						// Switch focus back to the editor
						m.focusIndex = editorPanel
						m.editorFocus = true
						return m, m.textarea.Focus()
					}
				} else {
					m.builtinsTree, cmd = m.builtinsTree.Update(msg)
					cmds = append(cmds, cmd)
				}
			}
		}

	case tea.MouseMsg:
		// If the prompt is active, don't process other mouse events.
		if m.promptMode != promptModeNone {
			return m, nil
		}
		return m.handleMouse(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Force a redraw
		return m, tea.ClearScreen
	}

	// Check if tree selection changed and update the textarea content.
	if selected := m.tree.GetSelected(); selected != nil && m.focusIndex == projectTreePanel {
		selectedID := selected.Data().(string)
		if selectedID != m.lastTreeSelection {
			m.updatePousContentFromEditor()
			m.lastTreeSelection = selectedID
			m.loadPousContentToEditor()
			m.updateMiddleViewport()
		}
	}

	// Also update the log viewport to handle scrolling etc.
	m.logViewport, cmd = m.logViewport.Update(msg) // cspell:disable-line
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// evaluateCode triggers the parsing and evaluation of the code in the editor.
func (m *model) evaluateCode() {
	code := m.textarea.Value()
	m.log.Info("--- Evaluating Code ---")
	m.lexerOk = true // Lexer is robust and doesn't have a fail state in this implementation
	m.parserOk = true
	m.evaluatorOk = true

	l := lexer.New(code)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 0 {
		m.parserOk = false
		for _, err := range p.Errors() {
			m.log.Error("Parser Error: " + err)
		}
	} else {
		m.parserOk = true
		evaluated := evaluator.Eval(program, m.env)
		if evaluated != nil {
			if err, ok := evaluated.(*object.Error); ok {
				m.log.Error("Evaluator Error: " + err.Message)
				m.evaluatorOk = false
			} else if evaluated.Type() != object.NULL_OBJ {
				m.log.Info("Result: " + evaluated.Inspect())
			}
		}
	}
	m.log.Info("--- Evaluation Finished ---")
}

func (m *model) saveProject() {
	// Ensure the current editor content is saved to the map before serializing
	m.updatePousContentFromEditor()

	data, err := json.MarshalIndent(m.pousContent, "", "  ")
	if err != nil {
		m.log.Errorf("Failed to marshal project data: %v", err)
		return
	}

	err = os.WriteFile(m.currentProjectPath, data, 0644)
	if err != nil {
		m.log.Errorf("Failed to save project to %s: %v", m.currentProjectPath, err)
		return
	}

	m.log.Infof("Project saved to %s", m.currentProjectPath)
	m.isDirty = false
}

func (m *model) loadProject() {
	data, err := os.ReadFile(m.currentProjectPath)
	if err != nil {
		m.log.Errorf("Failed to read project file %s: %v", m.currentProjectPath, err)
		return
	}

	if err := json.Unmarshal(data, &m.pousContent); err != nil {
		m.log.Errorf("Failed to unmarshal project data: %v", err)
		return
	}

	m.log.Infof("Project loaded from %s", m.currentProjectPath)
	m.loadPousContentToEditor()
	m.updateMiddleViewport()
	m.isDirty = false
}

func (m *model) newProject() {
	m.log.Info("Creating new project...")
	// Reset project data
	m.pousContent = make(map[string]string)
	m.currentProjectPath = "untitled.json"
	m.isDirty = false

	// Reset UI components to initial state
	rootNode := treeNode{data: "CONFIGURATION CELL_1", children: []tree.Node{treeNode{data: "RESOURCE STATION_1", children: []tree.Node{treeNode{data: "TASK SLOW_1", children: []tree.Node{treeNode{data: "PROGRAM P1"}}}, treeNode{data: "TASK FAST_1", children: []tree.Node{treeNode{data: "PROGRAM P2"}}}}}, treeNode{data: "RESOURCE STATION_2", children: []tree.Node{treeNode{data: "TASK PER_2", children: []tree.Node{treeNode{data: "PROGRAM P1"}}}, treeNode{data: "TASK INT_2", children: []tree.Node{treeNode{data: "PROGRAM P4"}}}}}}}
	newTree := tree.New(rootNode) // cspell:disable-line
	m.tree = newTree
	m.lastTreeSelection = rootNode.Data().(string)

	// Reset editor
	initialContent := fmt.Sprintf("Selected: %s", m.lastTreeSelection)
	m.textarea.SetValue(initialContent)
	m.pousContent[m.lastTreeSelection] = initialContent
	m.updateMiddleViewport()

	m.log.Info("New project created.")
}

func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Type != tea.MouseRelease {
		return m, nil
	}

	// --- Calculate panel dimensions and positions ---
	docWidth := m.width - docStyle.GetHorizontalFrameSize()
	docHeight := m.height - docStyle.GetVerticalFrameSize()
	docTop, docLeft := docStyle.GetMargin()

	topPanelHeight := 1
	statusBarHeight := 1
	dropdownHeight := 0
	if m.menuOpen {
		dropdownHeight = len(menuItems[m.selectedMenu].items) + 2
	}
	bottomPanelHeight := docHeight / 5
	mainContentHeight := docHeight - topPanelHeight - bottomPanelHeight - statusBarHeight - dropdownHeight

	leftPanelWidth := docWidth / 5
	rightPanelWidth := docWidth / 5
	middlePanelWidth := docWidth - leftPanelWidth - rightPanelWidth

	// Panel Y positions
	topPanelY := docTop
	dropdownY := topPanelY + topPanelHeight
	mainContentY := dropdownY + dropdownHeight

	// Panel X positions
	leftPanelX := docLeft
	middlePanelX := leftPanelX + leftPanelWidth
	rightPanelX := middlePanelX + middlePanelWidth

	// --- Check Menu Bar ---
	if msg.Y == topPanelY {
		x := msg.X - docLeft
		currentX := 0
		for i, mi := range menuItems {
			itemWidth := lg.Width(" " + mi.name + " ")
			if x >= currentX && x < currentX+itemWidth {
				if m.focusIndex == menuPanel && m.selectedMenu == i {
					m.menuOpen = !m.menuOpen
				} else {
					m.selectedMenu = i
					m.menuOpen = true
				}
				m.focusIndex = menuPanel
				m.selectedMenuItem = 0
				return m, nil
			}
			currentX += itemWidth
		}
	}

	// --- Check Dropdown Menu ---
	if m.menuOpen && msg.Y >= dropdownY+1 && msg.Y < dropdownY+dropdownHeight-1 {
		relativeY := msg.Y - (dropdownY + 1) // +1 for top border
		if relativeY >= 0 && relativeY < len(menuItems[m.selectedMenu].items) {
			m.selectedMenuItem = relativeY
			// Simulate Enter key press to execute the item
			return m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		}
	}

	// --- Check Left Panel (Project Tree) ---
	if msg.X >= leftPanelX && msg.X < leftPanelX+leftPanelWidth && msg.Y >= mainContentY && msg.Y < mainContentY+mainContentHeight {
		m.focusIndex = projectTreePanel
		m.menuOpen = false // Close menu if clicking elsewhere
		var cmd tea.Cmd
		m.tree, cmd = m.tree.Update(msg)
		return m, cmd
	}

	// --- Check Right Panel (Built-ins Tree) ---
	if msg.X >= rightPanelX && msg.X < rightPanelX+rightPanelWidth && msg.Y >= mainContentY && msg.Y < mainContentY+mainContentHeight {
		m.focusIndex = builtinsTreePanel
		m.menuOpen = false
		var cmd tea.Cmd
		m.builtinsTree, cmd = m.builtinsTree.Update(msg)
		// After the tree updates from the click, the item is selected.
		// Now we can simulate the 'Enter' press to insert it.
		m, enterCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		return m, tea.Batch(cmd, enterCmd)
	}

	// --- Check Middle Panel (Editor) ---
	if msg.X >= middlePanelX && msg.X < middlePanelX+middlePanelWidth && msg.Y >= mainContentY && msg.Y < mainContentY+mainContentHeight {
		m.focusIndex = editorPanel
		m.menuOpen = false
		return m, m.textarea.Focus()
	}

	// If click is anywhere else, just close the menu
	if m.menuOpen {
		m.menuOpen = false
	}

	return m, nil
}

// highlightWithLineNumbers applies syntax highlighting to a code string and adds line numbers.
func (m *model) highlightWithLineNumbers(code string) string {
	var sb strings.Builder
	l := lexer.New(code)

	lineCount := 1
	// Calculate padding for line numbers for alignment.
	maxDigits := len(strconv.Itoa(strings.Count(code, "\n") + 1))
	if maxDigits < 3 {
		maxDigits = 3 // Use a minimum padding for aesthetics.
	}
	lineNumFormat := fmt.Sprintf("%%%dd │ ", maxDigits)

	sb.WriteString(fmt.Sprintf(lineNumFormat, lineCount))
	lineCount++

	for {
		tok := l.NextToken()
		if tok.Type == token.EOF {
			break
		}

		parts := strings.Split(tok.Literal, "\n")
		for i, part := range parts {
			if style, ok := m.styles[tok.Type]; ok {
				sb.WriteString(style.Render(part))
			} else {
				sb.WriteString(part)
			}

			if i < len(parts)-1 {
				sb.WriteString("\n")
				sb.WriteString(fmt.Sprintf(lineNumFormat, lineCount))
				lineCount++
			}
		}
	}
	return sb.String()
}

// updatePousContentFromEditor saves the current textarea value to the pousContent map.
func (m *model) updatePousContentFromEditor() {
	if m.lastTreeSelection != "" {
		m.pousContent[m.lastTreeSelection] = m.textarea.Value()
	}
}

// loadPousContentToEditor loads content from the map into the textarea.
func (m *model) loadPousContentToEditor() {
	content, found := m.pousContent[m.lastTreeSelection]
	if found {
		m.textarea.SetValue(content)
	} else {
		// Content not found, generate placeholder and save it.
		var newContent string
		if strings.HasPrefix(m.lastTreeSelection, "PROGRAM") {
			programName := strings.TrimPrefix(m.lastTreeSelection, "PROGRAM ")
			newContent = fmt.Sprintf(
				`PROGRAM %s
	VAR
		myVar : INT := 0;
	END_VAR

	(* Main program body *)
	myVar := myVar + 1;
END_PROGRAM`, programName)
		} else {
			newContent = fmt.Sprintf("Selected: %s", m.lastTreeSelection)
		}
		m.textarea.SetValue(newContent)
		m.pousContent[m.lastTreeSelection] = newContent // Save the new placeholder content
	}
}

// updateMiddleViewport highlights the current textarea content and sets it in the viewport.
func (m *model) updateMiddleViewport() {
	m.middleViewport.SetContent(m.highlightWithLineNumbers(m.textarea.Value()))
}

// renderTopPanel renders the menu bar.
func (m model) renderTopPanel(width int) string {
	styleToUse := topPanelStyle
	if m.focusIndex == menuPanel {
		styleToUse = topPanelStyle.Copy().BorderForeground(lg.Color("86"))
	}

	var menuStrings []string
	for i, mi := range menuItems {
		if i == m.selectedMenu {
			menuStrings = append(menuStrings, selectedMenuItemStyle.Render(" "+mi.name+" "))
		} else {
			menuStrings = append(menuStrings, menuItemStyle.Render(" "+mi.name+" "))
		}
	}
	return styleToUse.
		Width(width).
		Render(lg.JoinHorizontal(lg.Left, menuStrings...))
}

func (m model) renderDropdownView() string {
	if !m.menuOpen {
		return ""
	}

	var menuContent strings.Builder
	items := menuItems[m.selectedMenu].items
	for i, item := range items {
		if i == m.selectedMenuItem {
			menuContent.WriteString(selectedMenuItemStyle.Render("> " + item))
		} else {
			menuContent.WriteString(menuItemStyle.Render("  " + item))
		}
		if i < len(items)-1 {
			menuContent.WriteString("\n")
		}
	}

	// Calculate offset to position the dropdown under the correct menu item
	offset := 0
	for i := 0; i < m.selectedMenu; i++ {
		offset += lg.Width(" " + menuItems[i].name + " ")
	}

	dropdownStyle := lg.NewStyle().
		Border(lg.NormalBorder()).
		BorderForeground(lg.Color("240")).
		Padding(0, 1).
		MarginLeft(offset)

	return dropdownStyle.Render(menuContent.String())
}

func (m model) renderStatusBar(width int) string {
	// Focus status
	var focusStr string
	switch m.focusIndex {
	case menuPanel:
		focusStr = "Menu"
	case projectTreePanel:
		focusStr = "Project Tree"
	case editorPanel:
		focusStr = "Editor"
	case builtinsTreePanel:
		focusStr = "Built-ins"
	}
	focus := "FOCUS: " + focusStr

	// File status
	fileStr := "FILE: " + m.lastTreeSelection

	// OK/ERR statuses
	lexerStatus := "Lexer: OK"
	if !m.lexerOk {
		lexerStatus = statusErrStyle.Render("Lexer: ERR")
	}
	parserStatus := "Parser: OK"
	if !m.parserOk {
		parserStatus = statusErrStyle.Render("Parser: ERR")
	}
	evaluatorStatus := "Eval: OK"
	if !m.evaluatorOk {
		evaluatorStatus = statusErrStyle.Render("Eval: ERR")
	}
	compileStatus := "Compile: OK"
	if !m.compileOk {
		compileStatus = statusErrStyle.Render("Compile: ERR")
	}

	statusGroup := lg.JoinHorizontal(lg.Top, lexerStatus, " | ", parserStatus, " | ", evaluatorStatus, " | ", compileStatus)

	leftPart := lg.JoinHorizontal(lg.Top, focus, " | ", fileStr)

	remainingWidth := width - lg.Width(leftPart) - lg.Width(statusGroup) - statusBarStyle.GetHorizontalPadding()
	if remainingWidth < 0 {
		remainingWidth = 0
	}
	spring := strings.Repeat(" ", remainingWidth)

	return statusBarStyle.Render(lg.JoinHorizontal(lg.Top, leftPart, spring, statusGroup))
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// Define panel dimensions
	docWidth := m.width - docStyle.GetHorizontalFrameSize()
	docHeight := m.height - docStyle.GetVerticalFrameSize()

	topPanelHeight := 1
	statusBarHeight := 1
	dropdownHeight := 0
	if m.menuOpen {
		dropdownHeight = len(menuItems[m.selectedMenu].items) + 2 // +2 for top/bottom border
	}
	bottomPanelHeight := docHeight / 5 // 20% for logs
	mainContentHeight := docHeight - topPanelHeight - bottomPanelHeight - statusBarHeight - dropdownHeight

	leftPanelWidth := docWidth / 5  // 20% for TreeView
	rightPanelWidth := docWidth / 5 // 20% for Builtins
	middlePanelWidth := docWidth - leftPanelWidth - rightPanelWidth

	// Create panel content
	topPanel := m.renderTopPanel(docWidth)

	dropdownView := m.renderDropdownView()

	// Set component sizes before rendering
	m.textarea.SetWidth(middlePanelWidth)
	m.textarea.SetHeight(mainContentHeight - middlePanelStyle.GetVerticalFrameSize()) // Account for border

	m.middleViewport.Width = middlePanelWidth
	m.middleViewport.Height = mainContentHeight - middlePanelStyle.GetVerticalFrameSize()

	// Set viewport size and content before rendering
	m.logViewport.Width = docWidth
	m.logViewport.Height = bottomPanelHeight - bottomPanelStyle.GetVerticalFrameSize() // Account for border
	m.logViewport.SetContent(m.logOutput.String())
	m.logViewport.GotoBottom()

	bottomPanel := bottomPanelStyle.
		Width(docWidth).
		Height(bottomPanelHeight).
		Render(m.logViewport.View())

	statusBar := m.renderStatusBar(docWidth)

	leftPanelStyleToUse := leftPanelStyle
	if m.focusIndex == projectTreePanel {
		leftPanelStyleToUse = focusedLeftPanelStyle
	}
	leftPanel := leftPanelStyleToUse.
		Width(leftPanelWidth).
		Height(mainContentHeight).
		Render(m.tree.View())

	rightPanelStyleToUse := rightPanelStyle
	if m.focusIndex == builtinsTreePanel {
		rightPanelStyleToUse = focusedRightPanelStyle
	}
	rightPanel := rightPanelStyleToUse.
		Width(rightPanelWidth).
		Height(mainContentHeight).
		Render(m.builtinsTree.View())

	// Render the middle panel with either the viewport or textarea.
	var middlePanel string
	// The editor panel is "focused" if its index is selected.
	// The `editorFocus` bool now strictly means "in edit mode".
	if m.focusIndex == editorPanel {
		middlePanel = focusedMiddlePanelStyle.
			Width(middlePanelWidth).
			Height(mainContentHeight).
			Render(m.textarea.View())
	} else {
		middlePanel = middlePanelStyle.
			Width(middlePanelWidth).
			Height(mainContentHeight).
			Render(m.middleViewport.View())
	}

	// Join panels
	mainContent := lg.JoinHorizontal(lg.Top,
		leftPanel,
		middlePanel,
		rightPanel,
	)

	fullLayout := lg.JoinVertical(lg.Left,
		topPanel,
		dropdownView,
		mainContent,
		bottomPanel,
		statusBar,
	)

	// If the prompt is active, render it on top of the main layout.
	if m.promptMode != promptModeNone {
		var promptTitle string
		var promptContent string

		switch m.promptMode {
		case promptModeSaveAs:
			promptTitle = "Save Project As"
			promptContent = m.promptInput.View() + "\n\n(Enter to confirm, Esc to cancel)"
		case promptModeOpen:
			promptTitle = "Open Project"
			promptContent = m.promptInput.View() + "\n\n(Enter to confirm, Esc to cancel)"
		case promptModeConfirmNew:
			promptTitle = "Unsaved Changes"
			promptContent = "Do you want to save the changes to your project before creating a new one?\n\n(Y)es / (N)o / (Esc)ancel"
		case promptModeConfirmClose:
			promptTitle = "Unsaved Changes"
			promptContent = "Do you want to save the changes to your project before closing?\n\n(Y)es / (N)o / (Esc)ancel"
		}

		promptBox := lg.NewStyle().
			Border(lg.NormalBorder(), true).
			BorderForeground(lg.Color("240")).
			Padding(1, 2).
			Render(promptTitle + "\n\n" + promptContent)

		return lg.Place(m.width, m.height, lg.Center, lg.Center, promptBox, lg.WithOverlay(docStyle.Render(fullLayout)))
	}

	return docStyle.Render(fullLayout)
}

// StartTUI is the entry point for the TUI application.
func StartTUI() {
	p := tea.NewProgram(InitialModel(), tea.WithAltScreen(), tea.WithMouseAllMotion())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
