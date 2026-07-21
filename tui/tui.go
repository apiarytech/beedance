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

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	lg "charm.land/lipgloss/v2"
	"charm.land/log/v2"
)

// menuItem represents a top-level menu item
type menuItem struct {
	name  string
	items []string // For now, just a list of names for sub-items
}

// --- Project Model Structs ---

// Program represents a single program POU.
type Program struct {
	Name string
}

// Task represents a task that contains programs.
type Task struct {
	Name     string
	Programs []Program
}

// Resource represents a resource that contains tasks.
type Resource struct {
	Name  string
	Tasks []Task
}

// Configuration represents the top-level configuration containing resources.
type Configuration struct {
	Name      string
	Resources []Resource
}

// AddResource appends a new resource to the configuration.
func (c *Configuration) AddResource(resource Resource) {
	c.Resources = append(c.Resources, resource)
}

// AddTask appends a new task to the resource.
func (r *Resource) AddTask(task Task) {
	r.Tasks = append(r.Tasks, task)
}

// --- TUI Tree Node ---

// treeNode is the data structure used by the rendering component.
type treeNode struct {
	data     string
	children []treeNode
}

var menuItems = []menuItem{
	{name: "File", items: []string{"New Project", "Open...", "Close Project", "Save", "Save As...", "Import/Export", "Print", "Exit"}},
	{name: "Build", items: []string{"Lex/Parse/Evaluate", "Compile", "Run", "Pause", "Stop"}}, // cspell:disable-line
	{name: "Edit", items: []string{"Undo", "Redo", "Cut", "Copy", "Paste", "Delete", "Select All", "Find", "Replace", "Browse"}},
	{name: "View", items: []string{"Show/Hide panels: Left Right and Bottom"}},
}

// toTreeNode converts the project model into a treeNode structure for rendering.
func (c *Configuration) toTreeNode() treeNode {
	root := treeNode{data: "CONFIGURATION " + c.Name}
	for _, res := range c.Resources {
		resNode := treeNode{data: "RESOURCE " + res.Name}
		for _, task := range res.Tasks {
			taskNode := treeNode{data: "TASK " + task.Name}
			for _, prog := range task.Programs {
				progNode := treeNode{data: "PROGRAM " + prog.Name}
				taskNode.children = append(taskNode.children, progNode)
			}
			resNode.children = append(resNode.children, taskNode)
		}
		root.children = append(root.children, resNode)
	}
	return root
}

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
	promptModeConfirmExit
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

	statusBarStyle = lg.NewStyle().
			Background(lg.Color("235")).
			Foreground(lg.Color("248"))

	statusErrStyle = lg.NewStyle().
			Inherit(statusBarStyle).
			Foreground(lg.Color("196")) // Red
)

type model struct {
	width, height      int
	env                *object.Environment
	project            Configuration
	err                error
	selectedMenu       int // Index of the selected top-level menu
	projectTree        treeNode
	builtinsTree       treeNode
	projectTreeCursor  []int
	builtinsTreeCursor []int
	log                *log.Logger
	logOutput          *strings.Builder // cspell:disable-line
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
	// Create the dynamic project structure.
	project := Configuration{
		Name: "CELL_1",
		Resources: []Resource{
			{
				Name: "STATION_1",
				Tasks: []Task{
					{Name: "SLOW_1", Programs: []Program{{Name: "P1"}}},
					{Name: "FAST_1", Programs: []Program{{Name: "P2"}}},
				},
			},
			{
				Name: "STATION_2",
				Tasks: []Task{
					{Name: "PER_2", Programs: []Program{{Name: "P1"}}},
					{Name: "INT_2", Programs: []Program{{Name: "P4"}}},
				},
			},
		},
	}
	rootNode := project.toTreeNode()

	// Create the built-ins tree structure.
	builtinsRoot := treeNode{
		data: "Snippets",
		children: []treeNode{
			{
				data: "BUILTINS",
				children: []treeNode{
					{data: "ADD"},
					{data: "SUB"},
					{data: "TON"},
					{data: "CTU"},
					{data: "R_TRIG"},
				},
			},
			{
				data: "MATH",
				children: []treeNode{
					{data: "SIN"},
					{data: "COS"},
					{data: "TAN"},
					{data: "SQRT"},
				},
			},
		},
	}

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

		// Identifiers and others
		token.IDENT:   lg.NewStyle().Foreground(lg.Color("229")), // Light Yellow
		token.ILLEGAL: lg.NewStyle().Foreground(lg.Color("196")).Background(lg.Color("235")),
	}

	// --- Log setup ---
	logOutput := &strings.Builder{}
	logger := log.NewWithOptions(logOutput, log.Options{})
	logger.SetLevel(log.DebugLevel)
	logger.SetTimeFormat("15:04:05")

	vp := viewport.New() // Will be sized in View()
	middleVp := viewport.New()
	middleVp.SetContent("Select a POU from the tree to view its code. Press Enter to edit.")

	m := model{
		env:                object.NewEnvironment(),
		project:            project,
		err:                nil,
		selectedMenu:       0, // Start with "File" selected
		projectTree:        rootNode,
		builtinsTree:       builtinsRoot,
		projectTreeCursor:  []int{0},
		builtinsTreeCursor: []int{0},
		log:                logger,
		logOutput:          logOutput,
		logViewport:        vp, // cspell:disable-line
		textarea:           ta,
		lastTreeSelection:  rootNode.data,
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
			case promptModeSaveAs, promptModeOpen: // cspell:disable-line
				switch msg.String() {
				case "esc":
					m.promptMode = promptModeNone
					m.promptInput.Blur()
				case "enter":
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
			case promptModeConfirmExit:
				switch strings.ToLower(msg.String()) {
				case "y":
					m.saveProject()
					return m, tea.Quit
				case "n":
					return m, tea.Quit
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
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}

			// Handle F5 for evaluation
			if msg.String() == "f5" {
				m.evaluateCode()
				return m, nil
			}

			// Handle focus switching with Tab
			switch msg.String() {
			case "tab":
				m.focusIndex = (m.focusIndex + 1) % 4 // Cycle through 4 panels
				m.editorFocus = (m.focusIndex == editorPanel)
				if m.editorFocus {
					return m, m.textarea.Focus()
				} else {
					m.textarea.Blur()
				}
				return m, nil
			case "shift+tab":
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
					switch msg.String() {
					case "up":
						if m.selectedMenuItem > 0 {
							m.selectedMenuItem--
						}
					case "down":
						if m.selectedMenuItem < len(menuItems[m.selectedMenu].items)-1 {
							m.selectedMenuItem++
						}
					case "enter":
						selectedMenuName := menuItems[m.selectedMenu].name
						selectedItemName := menuItems[m.selectedMenu].items[m.selectedMenuItem]

						if selectedMenuName == "File" {
							switch selectedItemName {
							case "New Project":
								if m.isDirty {
									m.promptMode = promptModeConfirmNew
								} else {
									m.newProject()
								}
								m.menuOpen = false
								return m, nil
							case "Close Project":
								if m.isDirty {
									m.promptMode = promptModeConfirmClose
								} else {
									m.newProject()
								}
								m.menuOpen = false
								return m, nil
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
								if m.isDirty {
									m.promptMode = promptModeConfirmExit
								} else {
									return m, tea.Quit
								}
								m.menuOpen = false
							default:
								m.log.Infof("Menu item selected: %s -> %s", selectedMenuName, selectedItemName)
							}
						} else if selectedMenuName == "Build" && selectedItemName == "Lex/Parse/Evaluate" {
							m.evaluateCode()
						} else {
							m.log.Infof("Menu item selected: %s -> %s", selectedMenuName, selectedItemName)
						}

						if selectedMenuName == "Edit" {
							switch selectedItemName {
							// case "Add Resource":
							// 	m.addResource()
							// case "Add Task":
							// 	m.addTask()
							default:
								m.log.Infof("Menu item selected: %s -> %s", selectedMenuName, selectedItemName)
							}
						} else {
							m.log.Infof("Menu item selected: %s -> %s", selectedMenuName, selectedItemName)

						}

						m.menuOpen = false
					case "esc":
						m.menuOpen = false
					}
				} else {
					switch msg.String() {
					case "left":
						m.selectedMenu = (m.selectedMenu - 1 + len(menuItems)) % len(menuItems)
					case "right":
						m.selectedMenu = (m.selectedMenu + 1) % len(menuItems)
					case "enter":
						m.menuOpen = true
						m.selectedMenuItem = 0
					case "esc":
						// If menu is not open, Esc does nothing here, but could be used to unfocus
					}
				}
				return m, nil

			case projectTreePanel:
				switch msg.String() {
				case "up":
					m.moveCursor(&m.projectTree, &m.projectTreeCursor, -1)
				case "down":
					m.moveCursor(&m.projectTree, &m.projectTreeCursor, 1)
				case "enter":
					// On enter, if the node has children, toggle expansion (not implemented yet).
					// If it's a leaf, we could switch focus to the editor.
					selectedNode := m.getSelectedNode(&m.projectTree, m.projectTreeCursor)
					if selectedNode != nil && len(selectedNode.children) == 0 {
						m.focusIndex = editorPanel
						m.editorFocus = true
						return m, m.textarea.Focus()
					}
				}
				// After moving cursor, update the selected item
				m.updateSelectionFromCursor()
				return m, nil

			case editorPanel:
				oldValue := m.textarea.Value()
				m.textarea, cmd = m.textarea.Update(msg)
				if m.textarea.Value() != oldValue {
					m.isDirty = true
				}
				cmds = append(cmds, cmd)

			case builtinsTreePanel:
				switch msg.String() {
				case "up":
					m.moveCursor(&m.builtinsTree, &m.builtinsTreeCursor, -1)
				case "down":
					m.moveCursor(&m.builtinsTree, &m.builtinsTreeCursor, 1)
				case "enter":
					selectedNode := m.getSelectedNode(&m.builtinsTree, m.builtinsTreeCursor)
					if selectedNode != nil && len(selectedNode.children) == 0 {
						m.textarea.InsertString(selectedNode.data)
						m.isDirty = true
						// Switch focus back to the editor
						m.focusIndex = editorPanel
						m.editorFocus = true
						return m, m.textarea.Focus()
					}
				}
				return m, nil
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
	// This is now handled by the cursor movement logic in the update case.
	// The `updateSelectionFromCursor` function will take care of this.

	// Update the middle viewport if not in editor mode
	m.updateMiddleViewport()

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
	m.projectTree = treeNode{
		data: "CONFIGURATION CELL_1",
		children: []treeNode{
			{ // cspell:disable-line
				data: "RESOURCE STATION_1",
				children: []treeNode{
					{data: "TASK SLOW_1", children: []treeNode{{data: "PROGRAM P1"}}},
					{data: "TASK FAST_1", children: []treeNode{{data: "PROGRAM P2"}}},
				},
			},
			{
				data: "RESOURCE STATION_2",
				children: []treeNode{
					{data: "TASK PER_2", children: []treeNode{{data: "PROGRAM P1"}}},
					{data: "TASK INT_2", children: []treeNode{{data: "PROGRAM P4"}}},
				},
			},
		},
	}

	// Reset editor
	m.lastTreeSelection = m.projectTree.data
	initialContent := fmt.Sprintf("Selected: %s", m.lastTreeSelection)
	m.textarea.SetValue(initialContent)
	m.pousContent[m.lastTreeSelection] = initialContent
	m.updateMiddleViewport()

	m.log.Info("New project created.")
}

func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	mouse := msg.Mouse()
	if mouse.Button != tea.MouseLeft {
		return m, nil
	}

	// --- Calculate panel dimensions and positions ---
	docWidth := m.width - docStyle.GetHorizontalFrameSize() // cspell:disable-line
	docHeight := m.height - docStyle.GetVerticalFrameSize()
	docTop, _, _, docLeft := docStyle.GetMargin()

	topPanelHeight := 1
	statusBarHeight := 1
	dropdownHeight := 0
	if m.menuOpen { // cspell:disable-line
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
	if mouse.Y == topPanelY {
		x := mouse.X - docLeft
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
	if m.menuOpen && mouse.Y >= dropdownY+1 && mouse.Y < dropdownY+dropdownHeight-1 {
		relativeY := mouse.Y - (dropdownY + 1) // +1 for top border
		if relativeY >= 0 && relativeY < len(menuItems[m.selectedMenu].items) {
			m.selectedMenuItem = relativeY
			// Simulate Enter key press to execute the item
			return m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
		}
	}

	// --- Check Left Panel (Project Tree) ---
	if mouse.X >= leftPanelX && mouse.X < leftPanelX+leftPanelWidth && mouse.Y >= mainContentY && mouse.Y < mainContentY+mainContentHeight {
		m.focusIndex = projectTreePanel
		m.menuOpen = false // Close menu if clicking elsewhere
		// Mouse click on tree is not implemented for selection in this version.
		return m, nil
	}

	// --- Check Right Panel (Built-ins Tree) ---
	if mouse.X >= rightPanelX && mouse.X < rightPanelX+rightPanelWidth && mouse.Y >= mainContentY && mouse.Y < mainContentY+mainContentHeight {
		m.focusIndex = builtinsTreePanel
		m.menuOpen = false // Close menu if clicking elsewhere
		var cmd tea.Cmd
		// Mouse click on tree is not implemented for selection in this version.
		return m, cmd
	}

	// --- Check Middle Panel (Editor) ---
	if mouse.X >= middlePanelX && mouse.X < middlePanelX+middlePanelWidth && mouse.Y >= mainContentY && mouse.Y < mainContentY+mainContentHeight {
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

// updateSelectionFromCursor updates the `lastTreeSelection` based on the current cursor position.
func (m *model) updateSelectionFromCursor() {
	if m.focusIndex == projectTreePanel {
		selectedNode := m.getSelectedNode(&m.projectTree, m.projectTreeCursor)
		if selectedNode != nil {
			if selectedNode.data != m.lastTreeSelection {
				m.updatePousContentFromEditor()
				m.lastTreeSelection = selectedNode.data
				m.loadPousContentToEditor()
			}
		}
	}
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
			menuStrings = append(menuStrings, selectedMenuItemStyle.Render(" "+mi.name+" ")) // cspell:disable-line
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
		focusStr = "Menu" // cspell:disable-line
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

	leftPart := lg.JoinHorizontal(lg.Top, focus, " | ", fileStr) // cspell:disable-line

	remainingWidth := width - lg.Width(leftPart) - lg.Width(statusGroup) - statusBarStyle.GetHorizontalPadding() // cspell:disable-line
	if remainingWidth < 0 {
		remainingWidth = 0 // cspell:disable-line
	}
	spring := strings.Repeat(" ", remainingWidth)

	return statusBarStyle.Render(lg.JoinHorizontal(lg.Top, leftPart, spring, statusGroup))
}

func (m model) View() tea.View {
	v := tea.NewView("Beedance TUI")
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion

	if m.width == 0 || m.height == 0 {
		v.SetContent("Initializing...")
		return v
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
	m.textarea.SetWidth(middlePanelWidth - middlePanelStyle.GetHorizontalFrameSize())
	m.textarea.SetHeight(mainContentHeight - middlePanelStyle.GetVerticalFrameSize())

	middleViewport := m.middleViewport
	middleViewport.SetWidth(middlePanelWidth - middlePanelStyle.GetHorizontalFrameSize())
	middleViewport.SetHeight(mainContentHeight - middlePanelStyle.GetVerticalFrameSize())

	// Set viewport size and content before rendering
	logViewport := m.logViewport
	logViewport.SetWidth(docWidth - bottomPanelStyle.GetHorizontalFrameSize())
	logViewport.SetHeight(bottomPanelHeight - bottomPanelStyle.GetVerticalFrameSize())
	logViewport.SetContent(m.logOutput.String())
	logViewport.GotoBottom()

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
		Render(m.renderTree(&m.projectTree, m.projectTreeCursor, m.focusIndex == projectTreePanel))

	rightPanelStyleToUse := rightPanelStyle
	if m.focusIndex == builtinsTreePanel {
		rightPanelStyleToUse = focusedRightPanelStyle
	}
	rightPanel := rightPanelStyleToUse.
		Width(rightPanelWidth).
		Height(mainContentHeight).
		Render(m.renderTree(&m.builtinsTree, m.builtinsTreeCursor, m.focusIndex == builtinsTreePanel))

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
			Render(middleViewport.View())
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
			promptContent = "Do you want to save the changes to your project before closing it?\n\n(Y)es / (N)o / (Esc)ancel"
		case promptModeConfirmExit:
			promptTitle = "Unsaved Changes"
			promptContent = "Do you want to save the changes to your project before exiting?\n\n(Y)es / (N)o / (Esc)ancel"
		}

		dialogBox := lg.Place(m.width, m.height, lg.Center, lg.Center,
			lg.NewStyle().Border(lg.NormalBorder()).BorderForeground(lg.Color("62")).Padding(1, 2).Render(promptTitle+"\n\n"+promptContent),
		)
		v.SetContent(lg.JoinVertical(lg.Left, fullLayout, dialogBox))
		return v
	}
	v.SetContent(fullLayout)
	return v
}

func (m *model) addResource() {
	// Implementation for adding a resource
}

func (m *model) addTask() {
	// Implementation for adding a task
}

func (m *model) moveCursor(tree *treeNode, cursor *[]int, direction int) {
	if len(*cursor) == 0 {
		*cursor = []int{0} // Start at the first node if cursor is invalid
		return
	}

	if direction == 1 { // Move down
		// 1. Try to move to the first child
		node := m.getSelectedNode(tree, *cursor)
		if node != nil && len(node.children) > 0 {
			*cursor = append(*cursor, 0)
			return
		}

		// 2. Try to move to the next sibling
		tempCursor := *cursor
		for len(tempCursor) > 0 {
			parentPath := tempCursor[:len(tempCursor)-1]
			parent := m.getSelectedNode(tree, parentPath)
			if parent == nil { // Should not happen with valid logic
				return
			}
			nextSiblingIndex := tempCursor[len(tempCursor)-1] + 1
			if nextSiblingIndex < len(parent.children) {
				*cursor = append(parentPath, nextSiblingIndex)
				return
			}
			// 3. No more siblings, move up and try again
			tempCursor = parentPath
		}
	} else if direction == -1 { // Move up
		// 1. Try to move to the previous sibling's last descendant
		if (*cursor)[len(*cursor)-1] > 0 {
			prevSiblingIndex := (*cursor)[len(*cursor)-1] - 1
			newPath := append((*cursor)[:len(*cursor)-1], prevSiblingIndex)
			node := m.getSelectedNode(tree, newPath)
			// Traverse to the deepest, last child
			for len(node.children) > 0 {
				lastChildIndex := len(node.children) - 1
				newPath = append(newPath, lastChildIndex)
				node = &node.children[lastChildIndex]
			}
			*cursor = newPath
			return
		}

		// 2. No previous sibling, move up to the parent
		if len(*cursor) > 1 {
			*cursor = (*cursor)[:len(*cursor)-1]
		}
	}
}

func (m *model) getSelectedNode(tree *treeNode, cursor []int) *treeNode {
	node := tree
	for _, index := range cursor {
		if index < 0 || index >= len(node.children) {
			return nil // Invalid cursor path
		}
		node = &node.children[index]
	}
	return node
}

func (m *model) renderTree(node *treeNode, cursor []int, isFocused bool) string {
	var sb strings.Builder
	m.renderNode(&sb, node, cursor, []int{}, isFocused)
	return sb.String()
}

func (m *model) renderNode(sb *strings.Builder, node *treeNode, cursor []int, path []int, isFocused bool) {
	prefix := ""
	if len(path) > 0 {
		for i := 0; i < len(path)-1; i++ {
			prefix += " │  "
		}
		prefix += " ├─ "
	}

	isCursorOnNode := len(path) == len(cursor)
	for i := range path {
		if path[i] != cursor[i] {
			isCursorOnNode = false
			break
		}
	}

	line := prefix + node.data
	if isFocused && isCursorOnNode {
		// Apply a style to highlight the selected node when its panel is focused.
		sb.WriteString(selectedMenuItemStyle.Render(line) + "\n")
	} else {
		sb.WriteString(line + "\n")
	}
}

// StartTUI is the entry point for the Bubble Tea application.
func StartTUI() {
	// tea.WithAltScreen() provides a clean exit, restoring the terminal.
	// tea.WithMouseAllMotion() enables mouse support for clicking and scrolling.
	p := tea.NewProgram(InitialModel())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
