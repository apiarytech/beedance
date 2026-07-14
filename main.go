/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"beedance/evaluator"
	"beedance/lexer"
	"beedance/object"
	"beedance/parser"

	"beedance/token"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	lg "github.com/charmbracelet/lipgloss"
)

const version = "0.1.0"

type model struct {
	viewport    viewport.Model
	textarea    textarea.Model
	history     []string
	env         *object.Environment
	senderStyle lg.Style
	err         error
	// For syntax highlighting using our own lexer
	styles map[token.TokenType]lg.Style
}

func initialModel() model {
	ta := textarea.New()
	ta.Placeholder = "Enter IEC 61131-3 code here..."
	ta.Focus()

	ta.Prompt = "┃ "
	ta.CharLimit = 0 // No limit

	ta.SetWidth(50)
	ta.SetHeight(3)

	// Remove cursor line styling
	ta.FocusedStyle.CursorLine = lg.NewStyle()
	ta.ShowLineNumbers = false

	vp := viewport.New(50, 10)
	vp.SetContent(`Welcome to Beedance (IEC 61131) v` + version + `!
Type your code below and press Enter. Type 'exit' or press Ctrl+C to quit.`)

	// --- Syntax Highlighting Styles using our own lexer ---
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

		// Data Types (also colors number literals due to lexer design)
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

		// Operators
		token.AND:      lg.NewStyle().Foreground(lg.Color("51")), // Cyan
		token.OR:       lg.NewStyle().Foreground(lg.Color("51")),
		token.XOR:      lg.NewStyle().Foreground(lg.Color("51")),
		token.NOT:      lg.NewStyle().Foreground(lg.Color("51")),
		token.MOD:      lg.NewStyle().Foreground(lg.Color("51")),
		token.PLUS:     lg.NewStyle().Foreground(lg.Color("51")),
		token.MINUS:    lg.NewStyle().Foreground(lg.Color("51")),
		token.ASTERISK: lg.NewStyle().Foreground(lg.Color("51")),
		token.SLASH:    lg.NewStyle().Foreground(lg.Color("51")),
		token.GT:       lg.NewStyle().Foreground(lg.Color("51")),
		token.LT:       lg.NewStyle().Foreground(lg.Color("51")),
		token.GE:       lg.NewStyle().Foreground(lg.Color("51")),
		token.LE:       lg.NewStyle().Foreground(lg.Color("51")),
		token.EQ:       lg.NewStyle().Foreground(lg.Color("51")),
		token.NEQ:      lg.NewStyle().Foreground(lg.Color("51")),
		token.ASSIGN:   lg.NewStyle().Foreground(lg.Color("51")),
		token.ARROW:    lg.NewStyle().Foreground(lg.Color("51")),
		token.EXPONENT: lg.NewStyle().Foreground(lg.Color("51")),

		// Punctuation
		token.LPAREN:    lg.NewStyle().Foreground(lg.Color("244")), // Dim gray
		token.RPAREN:    lg.NewStyle().Foreground(lg.Color("244")),
		token.LBRACKET:  lg.NewStyle().Foreground(lg.Color("244")),
		token.RBRACKET:  lg.NewStyle().Foreground(lg.Color("244")),
		token.COMMA:     lg.NewStyle().Foreground(lg.Color("244")),
		token.SEMICOLON: lg.NewStyle().Foreground(lg.Color("244")),
		token.COLON:     lg.NewStyle().Foreground(lg.Color("244")),
		token.DOT:       lg.NewStyle().Foreground(lg.Color("244")),
		token.RANGE:     lg.NewStyle().Foreground(lg.Color("244")),

		// Identifiers and others
		token.IDENT:   lg.NewStyle().Foreground(lg.Color("229")), // Light Yellow
		token.ILLEGAL: lg.NewStyle().Foreground(lg.Color("196")).Background(lg.Color("235")),
	}

	return model{
		textarea:    ta,
		viewport:    vp,
		history:     []string{},
		env:         object.NewEnvironment(),
		senderStyle: lg.NewStyle().Foreground(lg.Color("5")), // A nice purple
		err:         nil,
		styles:      styles,
	}
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m *model) highlight(code string) string {
	var sb strings.Builder
	l := lexer.New(code)

	for {
		tok := l.NextToken()
		if tok.Type == token.EOF {
			break
		}

		if style, ok := m.styles[tok.Type]; ok {
			sb.WriteString(style.Render(tok.Literal))
		} else {
			sb.WriteString(tok.Literal)
		}
	}
	return sb.String()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			input := m.textarea.Value()
			if strings.TrimSpace(input) == "" {
				return m, nil
			}
			if strings.ToLower(strings.TrimSpace(input)) == "exit" {
				return m, tea.Quit
			}

			// Add command to history with syntax highlighting
			m.history = append(m.history, m.senderStyle.Render("▶ ")+m.highlight(input))

			// Evaluate the code
			l := lexer.New(input)
			p := parser.New(l)
			program := p.ParseProgram()

			if len(p.Errors()) != 0 {
				for _, err := range p.Errors() {
					m.history = append(m.history, "Parser Error: "+err)
				}
			} else {
				evaluated := evaluator.Eval(program, m.env)
				if evaluated != nil && evaluated.Type() != object.NULL_OBJ {
					m.history = append(m.history, evaluated.Inspect())
				}
			}

			m.viewport.SetContent(strings.Join(m.history, "\n"))
			m.textarea.Reset()
			m.viewport.GotoBottom()
		}

	case tea.WindowSizeMsg:
		headerHeight := 3
		footerHeight := 5
		verticalMargin := headerHeight + footerHeight

		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - verticalMargin
		m.textarea.SetWidth(msg.Width)
		// Force a redraw
		return m, tea.ClearScreen
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m model) View() string {
	header := "🐝 Beedance IEC 61131-3 Interpreter\n" + m.senderStyle.Render("Ctrl+C to exit") + "\n"
	return fmt.Sprintf(
		"%s\n%s\n\n%s",
		header,
		m.viewport.View(),
		m.textarea.View(),
	)
}

func main() {
	trace := flag.Bool("trace", false, "Enable parser tracing")
	versionFlag := flag.Bool("version", false, "Print the application version")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("beedance version %s\n", version)
		os.Exit(0)
	}

	if *trace {
		// Redirect trace output to a file to keep the TUI clean.
		logFile, _ := os.OpenFile("trace.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		os.Stderr = logFile
		defer logFile.Close()
		parser.SetTracing(true)
	}

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
