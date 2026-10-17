/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under:
 * - GPL v2.0
 * - Commercial
 *
 * You may choose to use this software under the terms of either license.
 * See the LICENSE files in the project root for full license text.
 */

package ast

import "github.com/apiarytech/beedance/token"

// Jumps in Structured Text (IEC-68): `JMP label;` and `label:`, as IL has
// them, by the owner's decision that a jump is allowed in every language
// (IEC 61131-3 Ed.3 lists none among ST's statements). A label names the
// statement after it in a POU body; a jump goes on from there. A jump may
// leave an IF, CASE, FOR, WHILE or REPEAT but not enter one.

// LabelStatement is `name:` before a statement in an ST body.
type LabelStatement struct {
	Token token.Token // the label's identifier
	Name  *Identifier
}

func (ls *LabelStatement) statementNode() {}

// Pos returns the position of the label.
func (ls *LabelStatement) Pos() (int, int) { return ls.Token.Row, ls.Token.Column }

// TokenLiteral returns the label's name.
func (ls *LabelStatement) TokenLiteral() string { return ls.Token.Literal }

// String prints the label as written: `name:`.
func (ls *LabelStatement) String() string { return ls.Name.Value + ":" }

// JumpStatement is `JMP label;` in an ST body.
type JumpStatement struct {
	Token token.Token // the 'JMP' token
	Label *Identifier
}

func (js *JumpStatement) statementNode() {}

// Pos returns the position of the JMP keyword.
func (js *JumpStatement) Pos() (int, int) { return js.Token.Row, js.Token.Column }

// TokenLiteral returns "JMP".
func (js *JumpStatement) TokenLiteral() string { return js.Token.Literal }

// String prints the jump as written: `JMP label;`.
func (js *JumpStatement) String() string { return "JMP " + js.Label.Value + ";" }
