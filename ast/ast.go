/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package ast

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"beedance/token"
)

// The base Node interface
type Node interface {
	TokenLiteral() string
	Pos() (int, int) // Returns (line, column)
	String() string
}

// All statement nodes implement this
type Statement interface {
	Node
	statementNode()
}

// All expression nodes implement this
type Expression interface {
	Node
	expressionNode()
}

// A TypeSpecifier represents a data type in the language, e.g., INT, BOOL.
type TypeSpecifier struct {
	Token token.Token // The type token, e.g., token.INT
}

func (ts *TypeSpecifier) expressionNode()      {}
func (ts *TypeSpecifier) Pos() (int, int)      { return ts.Token.Row, ts.Token.Column }
func (ts *TypeSpecifier) TokenLiteral() string { return ts.Token.Literal }
func (ts *TypeSpecifier) String() string       { return ts.Token.Literal }

type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	} else {
		return ""
	}
}
func (p *Program) Pos() (int, int) {
	if len(p.Statements) > 0 {
		return p.Statements[0].Pos()
	}
	return 0, 0 // Default or error position
}

func (p *Program) String() string {
	var out bytes.Buffer

	for _, s := range p.Statements {
		out.WriteString(s.String())
	}

	return out.String()
}

// Statements
type VarDeclStatement struct {
	Token         token.Token // the 'VAR' token
	Name          *Identifier
	Location      *AtDeclaration
	DataType      Expression
	Value         Expression // Initial value
	IsConstant    bool
	IsRetain      bool
	IsNonRetain   bool
	IsRisingEdge  bool
	IsFallingEdge bool
	AccessType    string // "READ_ONLY", "READ_WRITE", or ""
}

func (vds *VarDeclStatement) statementNode()       {}
func (vds *VarDeclStatement) Pos() (int, int)      { return vds.Token.Row, vds.Token.Column }
func (vds *VarDeclStatement) TokenLiteral() string { return vds.Token.Literal }
func (vds *VarDeclStatement) String() string {
	var out bytes.Buffer

	// Only add the block type keyword if it's part of the token,
	// to correctly format struct members and other declarations.
	if vds.Token.Type >= token.VAR && vds.Token.Type <= token.VAR_CONFIG {
		out.WriteString(vds.TokenLiteral() + " ")
	}
	out.WriteString(vds.Name.String())
	if vds.Location != nil {
		out.WriteString(" ")
		out.WriteString(vds.Location.String())
	}
	out.WriteString(" : ")
	if vds.DataType != nil {
		out.WriteString(vds.DataType.String())
	}

	if vds.Value != nil {
		out.WriteString(" := ")
		out.WriteString(vds.Value.String())
	}
	out.WriteString(";")
	return out.String()
}

type AtDeclaration struct {
	Token    token.Token // The 'AT' token
	Location *DirectVariable
}

func (ad *AtDeclaration) statementNode()       {}
func (ad *AtDeclaration) Pos() (int, int)      { return ad.Token.Row, ad.Token.Column }
func (ad *AtDeclaration) TokenLiteral() string { return ad.Token.Literal }
func (ad *AtDeclaration) String() string {
	var out bytes.Buffer
	out.WriteString("AT ")
	out.WriteString(ad.Location.String())
	return out.String()
}

type DirectVariable struct {
	Token   token.Token // The '%' token
	Address string
}

func (dv *DirectVariable) expressionNode()      {}
func (dv *DirectVariable) Pos() (int, int)      { return dv.Token.Row, dv.Token.Column }
func (dv *DirectVariable) TokenLiteral() string { return dv.Token.Literal }
func (dv *DirectVariable) String() string {
	return "%" + dv.Address
}

type ConfigurationDeclaration struct {
	Token      token.Token // The 'CONFIGURATION' token
	Name       *Identifier
	GlobalVars []*GlobalVarDeclaration
	Resources  []*ResourceDeclaration
	AccessVars []*AccessVarDeclaration
	ConfigVars []*ConfigVarDeclaration
}

func (cd *ConfigurationDeclaration) statementNode()       {}
func (cd *ConfigurationDeclaration) Pos() (int, int)      { return cd.Token.Row, cd.Token.Column }
func (cd *ConfigurationDeclaration) TokenLiteral() string { return cd.Token.Literal }
func (cd *ConfigurationDeclaration) String() string {
	var out bytes.Buffer
	out.WriteString("CONFIGURATION " + cd.Name.String() + "\n")
	// ... string representations for children
	out.WriteString("END_CONFIGURATION")
	return out.String()
}

type ResourceDeclaration struct {
	Token        token.Token // The 'RESOURCE' token
	Name         *Identifier
	ResourceType *Identifier
	GlobalVars   []*GlobalVarDeclaration
	Tasks        []*TaskDeclaration
	Programs     []*ProgramConfiguration
}

func (rd *ResourceDeclaration) statementNode()       {}
func (rd *ResourceDeclaration) Pos() (int, int)      { return rd.Token.Row, rd.Token.Column }
func (rd *ResourceDeclaration) TokenLiteral() string { return rd.Token.Literal }
func (rd *ResourceDeclaration) String() string {
	var out bytes.Buffer
	out.WriteString("RESOURCE " + rd.Name.String() + " ON " + rd.ResourceType.String() + "\n")
	// ... string representations for children
	out.WriteString("END_RESOURCE")
	return out.String()
}

type TaskDeclaration struct {
	Token    token.Token // The 'TASK' token
	Name     *Identifier
	Single   Expression
	Interval Expression
	Priority Expression
}

func (td *TaskDeclaration) statementNode()       {}
func (td *TaskDeclaration) Pos() (int, int)      { return td.Token.Row, td.Token.Column }
func (td *TaskDeclaration) TokenLiteral() string { return td.Token.Literal }
func (td *TaskDeclaration) String() string {
	var out bytes.Buffer
	out.WriteString("TASK " + td.Name.String())
	out.WriteString("(")
	if td.Single != nil {
		out.WriteString("SINGLE := ")
		out.WriteString(td.Single.String())
	}
	if td.Interval != nil {
		out.WriteString("INTERVAL := ")
		out.WriteString(td.Interval.String())
	}
	out.WriteString("PRIORITY := ")
	out.WriteString(td.Priority.String())
	out.WriteString(")")
	return out.String()
}

type ProgramConfiguration struct {
	Token        token.Token // The 'PROGRAM' token
	InstanceName *Identifier
	TaskName     *Identifier // Optional: from WITH clause
	TypeName     *Identifier
	Parameters   []Expression // <-- Add this line
}

func (pc *ProgramConfiguration) statementNode()       {}
func (pc *ProgramConfiguration) Pos() (int, int)      { return pc.Token.Row, pc.Token.Column }
func (pc *ProgramConfiguration) TokenLiteral() string { return pc.Token.Literal }
func (pc *ProgramConfiguration) String() string {
	var out bytes.Buffer
	out.WriteString("PROGRAM ")
	out.WriteString(pc.InstanceName.String())
	if pc.TaskName != nil {
		out.WriteString(" WITH ")
		out.WriteString(pc.TaskName.String())
	}
	out.WriteString(" : ")
	out.WriteString(pc.TypeName.String())
	out.WriteString(";")
	return out.String()
}

type ExpressionStatement struct {
	Token      token.Token // the first token of the expression
	Expression Expression
}

func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) Pos() (int, int)      { return es.Token.Row, es.Token.Column }
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExpressionStatement) String() string {
	if es.Expression != nil {
		return es.Expression.String() + ";"
	}
	return ""
}

type AssignmentStatement struct {
	Token token.Token // The ':=' token
	Left  Expression
	Value Expression
}

func (as *AssignmentStatement) statementNode()       {}
func (as *AssignmentStatement) Pos() (int, int)      { return as.Token.Row, as.Token.Column }
func (as *AssignmentStatement) TokenLiteral() string { return as.Token.Literal }
func (as *AssignmentStatement) String() string {
	var out bytes.Buffer
	out.WriteString(as.Left.String())
	out.WriteString(" := ")
	if as.Value != nil {
		out.WriteString(as.Value.String())
	}
	out.WriteString(";")
	return out.String()
}

type ReturnStatement struct {
	Token       token.Token // the 'return' token
	ReturnValue Expression
}

func (rs *ReturnStatement) statementNode()       {}
func (rs *ReturnStatement) Pos() (int, int)      { return rs.Token.Row, rs.Token.Column }
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *ReturnStatement) String() string {
	var out bytes.Buffer

	out.WriteString(rs.TokenLiteral() + " ")

	if rs.ReturnValue != nil {
		out.WriteString(rs.ReturnValue.String())
	}

	out.WriteString(";")

	return out.String()
}

type ExitStatement struct {
	Token token.Token // the 'EXIT' token
}

func (es *ExitStatement) statementNode()       {}
func (es *ExitStatement) Pos() (int, int)      { return es.Token.Row, es.Token.Column }
func (es *ExitStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExitStatement) String() string {
	var out bytes.Buffer
	out.WriteString(es.TokenLiteral() + ";")
	return out.String()
}

type BlockStatement struct {
	Token      token.Token // the { token
	Statements []Statement
}

func (bs *BlockStatement) statementNode()       {}
func (bs *BlockStatement) expressionNode()      {} // Allow blocks to be treated as expressions (e.g., in IL)
func (bs *BlockStatement) Pos() (int, int)      { return bs.Token.Row, bs.Token.Column }
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) String() string {
	var out bytes.Buffer

	for _, s := range bs.Statements {
		out.WriteString(s.String())
	}

	return out.String()
}

// Expressions
type Identifier struct {
	Token token.Token // the token.IDENT token
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) Pos() (int, int)      { return i.Token.Row, i.Token.Column }
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) String() string       { return i.Value }

type Boolean struct {
	Token token.Token
	Value bool
}

func (b *Boolean) expressionNode()      {}
func (b *Boolean) Pos() (int, int)      { return b.Token.Row, b.Token.Column }
func (b *Boolean) TokenLiteral() string { return b.Token.Literal }
func (b *Boolean) String() string       { return b.Token.Literal }

type IntegerLiteral struct {
	Token token.Token
	Value int64
	Type  token.TokenType
}

func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) Pos() (int, int)      { return il.Token.Row, il.Token.Column }
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntegerLiteral) String() string {
	if il.Token.Literal != "" {
		return il.Token.Literal
	}
	return fmt.Sprintf("%d", il.Value)
}

type UnsignedIntegerLiteral struct {
	Token token.Token
	Type  token.TokenType
	Value uint64
}

func (ul *UnsignedIntegerLiteral) expressionNode()      {}
func (ul *UnsignedIntegerLiteral) Pos() (int, int)      { return ul.Token.Row, ul.Token.Column }
func (ul *UnsignedIntegerLiteral) TokenLiteral() string { return ul.Token.Literal }
func (ul *UnsignedIntegerLiteral) String() string {
	return ul.Token.Literal
}

type RealLiteral struct {
	Token     token.Token
	Value     float64
	Precision int // 32 for REAL, 64 for LREAL
}

func (rl *RealLiteral) expressionNode()      {}
func (rl *RealLiteral) Pos() (int, int)      { return rl.Token.Row, rl.Token.Column }
func (rl *RealLiteral) TokenLiteral() string { return rl.Token.Literal }
func (rl *RealLiteral) String() string {
	return rl.Token.Literal
}

// LRealLiteral represents a literal LREAL value.
type LRealLiteral struct {
	Token token.Token // The 'LREAL' token
	Value float64
}

func (lrl *LRealLiteral) expressionNode()      {}
func (lrl *LRealLiteral) Pos() (int, int)      { return lrl.Token.Row, lrl.Token.Column }
func (lrl *LRealLiteral) TokenLiteral() string { return lrl.Token.Literal }
func (lrl *LRealLiteral) String() string       { return lrl.Token.Literal }

// WStringLiteral represents a literal WSTRING value.
type WStringLiteral struct {
	Token token.Token // The 'WSTRING' token
	Value string
}

func (wsl *WStringLiteral) expressionNode()      {}
func (wsl *WStringLiteral) Pos() (int, int)      { return wsl.Token.Row, wsl.Token.Column }
func (wsl *WStringLiteral) TokenLiteral() string { return wsl.Token.Literal }
func (wsl *WStringLiteral) String() string       { return wsl.Token.Literal }

// EnumeratedValueLiteral represents a qualified enumerated value, e.g., COLOR#RED.
type EnumeratedValueLiteral struct {
	Token    token.Token // The token for the type name, e.g., 'COLOR'
	TypeName *Identifier
	Value    *Identifier
}

func (evl *EnumeratedValueLiteral) expressionNode()      {}
func (evl *EnumeratedValueLiteral) Pos() (int, int)      { return evl.Token.Row, evl.Token.Column }
func (evl *EnumeratedValueLiteral) TokenLiteral() string { return evl.Token.Literal }
func (evl *EnumeratedValueLiteral) String() string {
	return evl.TypeName.String() + "#" + evl.Value.String()
}

type BitStringLiteral struct {
	Token token.Token // The token for the literal (e.g., BYTE, WORD, DWORD, LWORD)
	Value uint64
	Width int // 8, 16, 32, 64
}

func (bsl *BitStringLiteral) expressionNode()      {}
func (bsl *BitStringLiteral) Pos() (int, int)      { return bsl.Token.Row, bsl.Token.Column }
func (bsl *BitStringLiteral) TokenLiteral() string { return bsl.Token.Literal }
func (bsl *BitStringLiteral) String() string {
	switch bsl.Width {
	case 8:
		return fmt.Sprintf("BYTE#16#%X", bsl.Value)
	case 16:
		return fmt.Sprintf("WORD#16#%X", bsl.Value)
	case 32:
		return fmt.Sprintf("DWORD#16#%X", bsl.Value)
	case 64:
		return fmt.Sprintf("LWORD#16#%X", bsl.Value)
	default:
		return fmt.Sprintf("BITSTRING#%d#%X", bsl.Width, bsl.Value) // Fallback for unknown width
	}
}

type PrefixExpression struct {
	Token    token.Token // The prefix token, e.g. !
	Operator string
	Right    Expression
}

func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) Pos() (int, int)      { return pe.Token.Row, pe.Token.Column }
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PrefixExpression) String() string {
	var out bytes.Buffer

	out.WriteString("(")
	out.WriteString(pe.Operator)
	out.WriteString(pe.Right.String())
	out.WriteString(")")

	return out.String()
}

type InfixExpression struct {
	Token    token.Token // The operator token, e.g. +
	Left     Expression
	Operator string
	Right    Expression
}

func (ie *InfixExpression) expressionNode()      {}
func (ie *InfixExpression) Pos() (int, int)      { return ie.Token.Row, ie.Token.Column }
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *InfixExpression) String() string {
	var out bytes.Buffer

	out.WriteString("(")
	out.WriteString(ie.Left.String())
	out.WriteString(" " + ie.Operator + " ")
	out.WriteString(ie.Right.String())
	out.WriteString(")")

	return out.String()
}

type MemberAccessExpression struct {
	Token  token.Token // The '.' token
	Struct Expression  // The expression on the left of the dot
	Member *Identifier // The identifier on the right of the dot
}

func (mae *MemberAccessExpression) expressionNode()      {}
func (mae *MemberAccessExpression) Pos() (int, int)      { return mae.Token.Row, mae.Token.Column }
func (mae *MemberAccessExpression) TokenLiteral() string { return mae.Token.Literal }
func (mae *MemberAccessExpression) String() string {
	var out bytes.Buffer

	out.WriteString("(")
	out.WriteString(mae.Struct.String())
	out.WriteString(".")
	out.WriteString(mae.Member.String())
	out.WriteString(")")

	return out.String()
}

type IfStatement struct {
	Token       token.Token // The 'if' token
	Condition   Expression
	Consequence *BlockStatement
	Alternative Statement // Can be *IfStatement (for ELSIF) or *BlockStatement (for ELSE)
}

func (is *IfStatement) statementNode()       {}
func (is *IfStatement) Pos() (int, int)      { return is.Token.Row, is.Token.Column }
func (is *IfStatement) TokenLiteral() string { return is.Token.Literal }
func (is *IfStatement) String() string {
	var out bytes.Buffer

	out.WriteString("IF ")
	out.WriteString(is.Condition.String())
	out.WriteString(" THEN ")
	if is.Consequence != nil {
		out.WriteString("\n\t" + is.Consequence.String() + "\n")
	}

	// Iterate through the ELSIF/ELSE chain
	currentAlt := is.Alternative
	for currentAlt != nil {
		switch alt := currentAlt.(type) {
		case *IfStatement: // This is an ELSIF clause
			out.WriteString(" ELSIF ")
			out.WriteString(alt.Condition.String())
			out.WriteString(" THEN ")
			if alt.Consequence != nil {
				out.WriteString("\n\t" + alt.Consequence.String() + "\n")
			}
			currentAlt = alt.Alternative // Continue to the next alternative in the chain
		case *BlockStatement: // This is an ELSE clause
			out.WriteString("ELSE\n\t")
			out.WriteString(alt.String() + "\n")
			currentAlt = nil // End of the alternative chain
		default:
			currentAlt = nil // Unknown alternative type, stop processing
		}
	}
	out.WriteString("END_IF")

	return out.String()
}

type ForLoopStatement struct {
	Token      token.Token // The 'FOR' token
	ControlVar *AssignmentStatement
	EndValue   Expression // The value to iterate TO
	StepValue  Expression // Can be nil for default step of 1
	Body       *BlockStatement
}

func (fls *ForLoopStatement) statementNode()       {}
func (fls *ForLoopStatement) Pos() (int, int)      { return fls.Token.Row, fls.Token.Column }
func (fls *ForLoopStatement) TokenLiteral() string { return fls.Token.Literal }
func (fls *ForLoopStatement) String() string {
	// String representation for debugging
	var out bytes.Buffer
	out.WriteString("FOR ")
	out.WriteString(fls.ControlVar.String())
	out.WriteString(" TO ")
	out.WriteString(fls.EndValue.String())
	if fls.StepValue != nil {
		out.WriteString(" BY ")
		out.WriteString(fls.StepValue.String())
	}
	out.WriteString(" DO ")
	if fls.Body != nil {
		out.WriteString(fls.Body.String())
	}
	out.WriteString(" END_FOR")
	return out.String()
}

type WhileStatement struct {
	Token     token.Token // The 'WHILE' token
	Condition Expression
	Body      *BlockStatement
}

func (ws *WhileStatement) statementNode()       {}
func (ws *WhileStatement) Pos() (int, int)      { return ws.Token.Row, ws.Token.Column }
func (ws *WhileStatement) TokenLiteral() string { return ws.Token.Literal }
func (ws *WhileStatement) String() string {
	var out bytes.Buffer
	out.WriteString("WHILE ")
	out.WriteString(ws.Condition.String())
	out.WriteString(" DO ")
	out.WriteString(ws.Body.String())
	out.WriteString(" END_WHILE")
	return out.String()
}

type RepeatStatement struct {
	Token     token.Token // The 'REPEAT' token
	Body      *BlockStatement
	Condition Expression
}

func (rs *RepeatStatement) statementNode()       {}
func (rs *RepeatStatement) Pos() (int, int)      { return rs.Token.Row, rs.Token.Column }
func (rs *RepeatStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *RepeatStatement) String() string {
	var out bytes.Buffer
	out.WriteString("REPEAT ")
	out.WriteString(rs.Body.String())
	out.WriteString(" UNTIL ")
	out.WriteString(rs.Condition.String())
	out.WriteString(" END_REPEAT")
	return out.String()
}

type CaseBranch struct {
	Token       token.Token // The first token of the value list
	Values      []Expression
	Consequence Statement
}

func (cb *CaseBranch) statementNode()       {}
func (cb *CaseBranch) Pos() (int, int)      { return cb.Token.Row, cb.Token.Column }
func (cb *CaseBranch) TokenLiteral() string { return cb.Token.Literal }
func (cb *CaseBranch) String() string {
	var out bytes.Buffer
	vals := []string{}
	for _, v := range cb.Values {
		vals = append(vals, v.String())
	}
	out.WriteString(strings.Join(vals, ", "))
	out.WriteString(": ")
	out.WriteString(cb.Consequence.String())
	return out.String()
}

type CaseStatement struct {
	Token       token.Token // The 'CASE' token
	Expression  Expression
	Cases       []*CaseBranch
	Alternative *BlockStatement // The 'ELSE' block
}

func (cs *CaseStatement) statementNode()       {}
func (cs *CaseStatement) Pos() (int, int)      { return cs.Token.Row, cs.Token.Column }
func (cs *CaseStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *CaseStatement) String() string {
	// String representation for debugging
	var out bytes.Buffer
	out.WriteString("CASE ")
	out.WriteString(cs.Expression.String())
	return out.String()
}

type FunctionLiteral struct {
	Token      token.Token // The 'fn' token
	Parameters []*Identifier
	Body       *BlockStatement
}

func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) Pos() (int, int)      { return fl.Token.Row, fl.Token.Column }
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FunctionLiteral) String() string {
	var out bytes.Buffer

	params := []string{}
	for _, p := range fl.Parameters {
		params = append(params, p.String())
	}

	out.WriteString(fl.TokenLiteral())
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") ")
	out.WriteString(fl.Body.String())

	return out.String()
}

type CallExpression struct {
	Token     token.Token // The '(' token
	Function  Expression  // Identifier or FunctionLiteral
	Arguments []Expression
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) Pos() (int, int)      { return ce.Token.Row, ce.Token.Column }
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) String() string {
	var out bytes.Buffer

	args := []string{}
	for _, a := range ce.Arguments {
		args = append(args, a.String())
	}

	out.WriteString(ce.Function.String())
	out.WriteString("(")
	out.WriteString(strings.Join(args, ", "))
	out.WriteString(")")

	return out.String()
}

type NamedArgument struct {
	Token token.Token // The identifier token for the argument name
	Name  *Identifier
	Value Expression
}

func (na *NamedArgument) expressionNode()      {}
func (na *NamedArgument) Pos() (int, int)      { return na.Token.Row, na.Token.Column }
func (na *NamedArgument) TokenLiteral() string { return na.Token.Literal }
func (na *NamedArgument) String() string {
	var out bytes.Buffer
	out.WriteString(na.Name.String())
	out.WriteString(" := ")
	out.WriteString(na.Value.String())
	return out.String()
}

type OutputArgument struct {
	Token  token.Token // The '=>' token
	Source *Identifier
	Target Expression // Should be a variable
}

func (oa *OutputArgument) expressionNode()      {}
func (oa *OutputArgument) Pos() (int, int)      { return oa.Token.Row, oa.Token.Column }
func (oa *OutputArgument) TokenLiteral() string { return oa.Token.Literal }
func (oa *OutputArgument) String() string {
	var out bytes.Buffer
	if oa.Source != nil {
		out.WriteString(oa.Source.String())
	}
	out.WriteString(" => ")
	if oa.Target != nil {
		out.WriteString(oa.Target.String())
	}
	return out.String()
}

type StringLiteral struct {
	Token token.Token
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) Pos() (int, int)      { return sl.Token.Row, sl.Token.Column }
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) String() string       { return sl.Token.Literal }

type TimeLiteral struct {
	Token token.Token // The token.TIME token
	Value string      // The raw string value, e.g., "T#5s"
}

func (tl *TimeLiteral) expressionNode()      {}
func (tl *TimeLiteral) Pos() (int, int)      { return tl.Token.Row, tl.Token.Column }
func (tl *TimeLiteral) TokenLiteral() string { return tl.Token.Literal }
func (tl *TimeLiteral) String() string {
	return tl.Token.Literal
}

type DateLiteral struct {
	Token token.Token // The token.DATE token
	Value string
}

func (dl *DateLiteral) expressionNode()      {}
func (dl *DateLiteral) Pos() (int, int)      { return dl.Token.Row, dl.Token.Column }
func (dl *DateLiteral) TokenLiteral() string { return dl.Token.Literal }
func (dl *DateLiteral) String() string {
	return dl.Token.Literal
}

type TimeOfDayLiteral struct {
	Token token.Token // The token.TIME_OF_DAY token
	Value string
}

func (todl *TimeOfDayLiteral) expressionNode()      {}
func (todl *TimeOfDayLiteral) Pos() (int, int)      { return todl.Token.Row, todl.Token.Column }
func (todl *TimeOfDayLiteral) TokenLiteral() string { return todl.Token.Literal }
func (todl *TimeOfDayLiteral) String() string {
	return todl.Token.Literal
}

type DateAndTimeLiteral struct {
	Token token.Token // The token.DATE_AND_TIME token
	Value string
}

func (dtl *DateAndTimeLiteral) expressionNode()      {}
func (dtl *DateAndTimeLiteral) Pos() (int, int)      { return dtl.Token.Row, dtl.Token.Column }
func (dtl *DateAndTimeLiteral) TokenLiteral() string { return dtl.Token.Literal }
func (dtl *DateAndTimeLiteral) String() string {
	return dtl.Token.Literal
}

type ArrayLiteral struct {
	Token    token.Token // the '[' token
	Elements []Expression
}

func (al *ArrayLiteral) expressionNode()      {}
func (na *ArrayLiteral) Pos() (int, int)      { return na.Token.Row, na.Token.Column }
func (al *ArrayLiteral) TokenLiteral() string { return al.Token.Literal }
func (al *ArrayLiteral) String() string {
	var out bytes.Buffer

	elements := []string{}
	for _, el := range al.Elements {
		elements = append(elements, el.String())
	}

	out.WriteString("[")
	out.WriteString(strings.Join(elements, ", "))
	out.WriteString("]")

	return out.String()
}

type IndexExpression struct {
	Token token.Token // The [ token
	Left  Expression
	Index Expression
}

func (ie *IndexExpression) expressionNode()      {}
func (ie *IndexExpression) Pos() (int, int)      { return ie.Token.Row, ie.Token.Column }
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IndexExpression) String() string {
	var out bytes.Buffer

	out.WriteString("(")
	out.WriteString(ie.Left.String())
	out.WriteString("[")
	out.WriteString(ie.Index.String())
	out.WriteString("])")

	return out.String()
}

type HashLiteral struct {
	Token token.Token // the '{' token
	Pairs map[Expression]Expression
}

func (hl *HashLiteral) expressionNode()      {}
func (hl *HashLiteral) Pos() (int, int)      { return hl.Token.Row, hl.Token.Column }
func (hl *HashLiteral) TokenLiteral() string { return hl.Token.Literal }
func (hl *HashLiteral) String() string {
	var out bytes.Buffer

	// Create a slice of keys from the map to allow sorting.
	keys := make([]Expression, 0, len(hl.Pairs))
	for key := range hl.Pairs {
		keys = append(keys, key)
	}

	// Sort the keys based on their string representation for deterministic output.
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].String() < keys[j].String()
	})

	pairs := []string{}
	for _, key := range keys {
		pairs = append(pairs, key.String()+": "+hl.Pairs[key].String())
	}

	out.WriteString("{")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("}")

	return out.String()
}

type MacroLiteral struct {
	Token      token.Token // The 'macro' token
	Parameters []*Identifier
	Body       *BlockStatement
}

func (ml *MacroLiteral) expressionNode()      {}
func (ml *MacroLiteral) Pos() (int, int)      { return ml.Token.Row, ml.Token.Column }
func (ml *MacroLiteral) TokenLiteral() string { return ml.Token.Literal }
func (ml *MacroLiteral) String() string {
	var out bytes.Buffer

	params := []string{}
	for _, p := range ml.Parameters {
		params = append(params, p.String())
	}

	out.WriteString(ml.TokenLiteral())
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") ")
	out.WriteString(ml.Body.String())

	return out.String()
}

type FunctionBlockDeclaration struct {
	Token      token.Token // The 'FUNCTION_BLOCK' token
	Name       *Identifier
	VarInputs  []*VarDeclStatement
	VarOutputs []*VarDeclStatement
	VarInOuts  []*VarDeclStatement
	Vars       []*VarDeclStatement
	Body       Statement
}

func (fbd *FunctionBlockDeclaration) statementNode()       {}
func (fbd *FunctionBlockDeclaration) Pos() (int, int)      { return fbd.Token.Row, fbd.Token.Column }
func (fbd *FunctionBlockDeclaration) TokenLiteral() string { return fbd.Token.Literal }
func (fbd *FunctionBlockDeclaration) String() string {
	var out bytes.Buffer
	out.WriteString("FUNCTION_BLOCK ")
	if fbd.Name != nil {
		out.WriteString(fbd.Name.String())
	}
	out.WriteString("\n")
	// Simplified string representation for now
	if fbd.Body != nil {
		out.WriteString(fbd.Body.String())
	}
	out.WriteString("\nEND_FUNCTION_BLOCK")
	return out.String()
}

type ProgramDeclaration struct {
	Token      token.Token // The 'PROGRAM' token
	Name       *Identifier
	VarInputs  []*VarDeclStatement
	VarOutputs []*VarDeclStatement
	VarInOuts  []*VarDeclStatement
	Vars       []*VarDeclStatement
	Body       Statement
}

func (pd *ProgramDeclaration) statementNode()       {}
func (pd *ProgramDeclaration) Pos() (int, int)      { return pd.Token.Row, pd.Token.Column }
func (pd *ProgramDeclaration) TokenLiteral() string { return pd.Token.Literal }
func (pd *ProgramDeclaration) String() string {
	var out bytes.Buffer
	out.WriteString("PROGRAM ")
	if pd.Name != nil {
		out.WriteString(pd.Name.String())
	}
	out.WriteString("\n")
	if pd.Body != nil {
		out.WriteString(pd.Body.String())
	}
	out.WriteString("\nEND_PROGRAM")
	return out.String()
}

type ExternalVarDeclaration struct {
	Token token.Token // The 'VAR_EXTERNAL' token
	Vars  []*VarDeclStatement
}

func (evd *ExternalVarDeclaration) statementNode()       {}
func (evd *ExternalVarDeclaration) Pos() (int, int)      { return evd.Token.Row, evd.Token.Column }
func (evd *ExternalVarDeclaration) TokenLiteral() string { return evd.Token.Literal }
func (evd *ExternalVarDeclaration) String() string {
	var out bytes.Buffer
	out.WriteString("VAR_EXTERNAL\n")
	for _, v := range evd.Vars {
		out.WriteString("\t" + v.String() + "\n")
	}
	out.WriteString("END_VAR")
	return out.String()
}

type ConfigVarDeclaration struct {
	Token token.Token // The 'VAR_CONFIG' token
	Vars  []*VarDeclStatement
}

func (cvd *ConfigVarDeclaration) statementNode()       {}
func (cvd *ConfigVarDeclaration) Pos() (int, int)      { return cvd.Token.Row, cvd.Token.Column }
func (cvd *ConfigVarDeclaration) TokenLiteral() string { return cvd.Token.Literal }
func (cvd *ConfigVarDeclaration) String() string {
	var out bytes.Buffer
	cvd.Pos() // Ensure Pos() is called
	out.WriteString("VAR_CONFIG\n")
	for _, v := range cvd.Vars {
		out.WriteString("\t" + v.String() + "\n")
	}
	out.WriteString("END_VAR")
	return out.String()
}

type TempVarDeclaration struct {
	Token token.Token // The 'VAR_TEMP' token
	Vars  []*VarDeclStatement
}

func (tvd *TempVarDeclaration) statementNode()       {}
func (tvd *TempVarDeclaration) Pos() (int, int)      { return tvd.Token.Row, tvd.Token.Column }
func (tvd *TempVarDeclaration) TokenLiteral() string { return tvd.Token.Literal }
func (tvd *TempVarDeclaration) String() string {
	var out bytes.Buffer
	tvd.Pos() // Ensure Pos() is called
	out.WriteString("VAR_TEMP\n")
	for _, v := range tvd.Vars {
		out.WriteString("\t" + v.String() + "\n")
	}
	out.WriteString("END_VAR")
	return out.String()
}

type AccessVarDeclaration struct {
	Token token.Token // The 'VAR_ACCESS' token
	Vars  []*VarDeclStatement
}

func (avd *AccessVarDeclaration) statementNode()       {}
func (avd *AccessVarDeclaration) Pos() (int, int)      { return avd.Token.Row, avd.Token.Column }
func (avd *AccessVarDeclaration) TokenLiteral() string { return avd.Token.Literal }
func (avd *AccessVarDeclaration) String() string {
	var out bytes.Buffer
	avd.Pos() // Ensure Pos() is called
	out.WriteString("VAR_ACCESS\n")
	for _, v := range avd.Vars {
		out.WriteString("\t" + v.String() + "\n")
	}
	out.WriteString("END_VAR")
	return out.String()
}

type GlobalVarDeclaration struct {
	Token token.Token // The 'VAR_GLOBAL' token
	Vars  []*VarDeclStatement
}

func (gvd *GlobalVarDeclaration) statementNode()       {}
func (gvd *GlobalVarDeclaration) Pos() (int, int)      { return gvd.Token.Row, gvd.Token.Column }
func (gvd *GlobalVarDeclaration) TokenLiteral() string { return gvd.Token.Literal }
func (gvd *GlobalVarDeclaration) String() string {
	var out bytes.Buffer
	gvd.Pos() // Ensure Pos() is called
	out.WriteString("VAR_GLOBAL\n")
	for _, v := range gvd.Vars {
		out.WriteString("\t" + v.String() + "\n")
	}
	out.WriteString("END_VAR")
	return out.String()
}

type VarBlockDeclaration struct {
	Token        token.Token // The 'VAR' token
	Declarations []*VarDeclStatement
}

func (vbd *VarBlockDeclaration) statementNode()       {}
func (vbd *VarBlockDeclaration) Pos() (int, int)      { return vbd.Token.Row, vbd.Token.Column }
func (vbd *VarBlockDeclaration) TokenLiteral() string { return vbd.Token.Literal }
func (vbd *VarBlockDeclaration) String() string {
	var out bytes.Buffer
	vbd.Pos() // Ensure Pos() is called
	out.WriteString("VAR\n")
	for _, d := range vbd.Declarations {
		out.WriteString("\t" + d.String() + "\n")
	}
	out.WriteString("END_VAR")
	return out.String()
}

type TypeDeclaration struct {
	Token        token.Token // The identifier token (the name of the new type)
	Name         *Identifier
	DataType     Expression
	Subrange     Expression // For subrange types, e.g., (0..100)
	InitialValue Expression // For initialized types, e.g., := 10
}

func (td *TypeDeclaration) statementNode()       {}
func (td *TypeDeclaration) Pos() (int, int)      { return td.Token.Row, td.Token.Column }
func (td *TypeDeclaration) TokenLiteral() string { return td.Token.Literal }
func (td *TypeDeclaration) String() string {
	var out bytes.Buffer
	out.WriteString(td.Name.String())
	out.WriteString(" : ")
	out.WriteString(td.DataType.String())
	if td.Subrange != nil {
		out.WriteString(" ")
		out.WriteString(td.Subrange.String())
	}
	if td.InitialValue != nil {
		out.WriteString(" := ")
		out.WriteString(td.InitialValue.String())
	}
	out.WriteString(";")
	return out.String()
}

type TypeBlockDeclaration struct {
	Token        token.Token // The 'TYPE' token
	Declarations []*TypeDeclaration
}

func (tbd *TypeBlockDeclaration) statementNode()       {}
func (tbd *TypeBlockDeclaration) Pos() (int, int)      { return tbd.Token.Row, tbd.Token.Column }
func (tbd *TypeBlockDeclaration) TokenLiteral() string { return tbd.Token.Literal }
func (tbd *TypeBlockDeclaration) String() string {
	var out bytes.Buffer
	tbd.Pos() // Ensure Pos() is called
	out.WriteString("TYPE\n")
	for _, d := range tbd.Declarations {
		out.WriteString("\t" + d.String() + "\n")
	}
	out.WriteString("END_TYPE")
	return out.String()
}

type StructDefinition struct {
	Token   token.Token // The 'STRUCT' token
	Members []*VarDeclStatement
}

func (sd *StructDefinition) expressionNode()      {}
func (sd *StructDefinition) Pos() (int, int)      { return sd.Token.Row, sd.Token.Column }
func (sd *StructDefinition) TokenLiteral() string { return sd.Token.Literal }
func (sd *StructDefinition) String() string {
	var out bytes.Buffer
	sd.Pos() // Ensure Pos() is called
	out.WriteString("STRUCT\n")
	for _, m := range sd.Members {
		out.WriteString("\t" + m.String() + "\n")
	}
	out.WriteString("END_STRUCT")
	return out.String()
}

type EnumDefinition struct {
	Token  token.Token // The '(' token
	Values []*Identifier
}

func (ed *EnumDefinition) expressionNode()      {}
func (ed *EnumDefinition) Pos() (int, int)      { return ed.Token.Row, ed.Token.Column }
func (ed *EnumDefinition) TokenLiteral() string { return ed.Token.Literal }
func (ed *EnumDefinition) String() string {
	var out bytes.Buffer
	ed.Pos() // Ensure Pos() is called
	vals := []string{}
	for _, v := range ed.Values {
		vals = append(vals, v.String())
	}
	out.WriteString("(")
	out.WriteString(strings.Join(vals, ", "))
	out.WriteString(")")
	return out.String()
}

type ArrayDefinition struct {
	Token    token.Token // The 'ARRAY' token
	Ranges   []Expression
	DataType *TypeSpecifier
}

func (ad *ArrayDefinition) expressionNode()      {}
func (ad *ArrayDefinition) Pos() (int, int)      { return ad.Token.Row, ad.Token.Column }
func (ad *ArrayDefinition) TokenLiteral() string { return ad.Token.Literal }
func (ad *ArrayDefinition) String() string {
	var out bytes.Buffer
	ad.Pos() // Ensure Pos() is called
	out.WriteString("ARRAY [")
	ranges := []string{}
	for _, r := range ad.Ranges {
		if infixExp, ok := r.(*InfixExpression); ok {
			// Format InfixExpression for range without outer parentheses
			ranges = append(ranges, fmt.Sprintf("%s %s %s", infixExp.Left.String(), infixExp.Operator, infixExp.Right.String()))
		} else {
			ranges = append(ranges, r.String())
		}
	}
	out.WriteString(strings.Join(ranges, ", "))
	out.WriteString("] OF ")
	if ad.DataType != nil {
		out.WriteString(ad.DataType.String())
	}

	return out.String()
}

type ActionStatement struct {
	Token token.Token // The 'ACTION' token
	Name  *Identifier
	Body  Statement
}

func (as *ActionStatement) statementNode()       {}
func (as *ActionStatement) Pos() (int, int)      { return as.Token.Row, as.Token.Column }
func (as *ActionStatement) TokenLiteral() string { return as.Token.Literal }
func (as *ActionStatement) String() string {
	var out bytes.Buffer
	as.Pos() // Ensure Pos() is called
	out.WriteString("ACTION ")
	out.WriteString(as.Name.String())
	out.WriteString("\n")
	out.WriteString(as.Body.String())
	out.WriteString("\nEND_ACTION")
	return out.String()
}

type FunctionDeclaration struct {
	Token      token.Token // The 'FUNCTION' token
	Name       *Identifier
	ReturnType *TypeSpecifier
	VarInputs  []*VarDeclStatement
	VarOutputs []*VarDeclStatement
	VarInOuts  []*VarDeclStatement
	Vars       []*VarDeclStatement
	Body       Statement
}

func (fd *FunctionDeclaration) statementNode()       {}
func (fd *FunctionDeclaration) Pos() (int, int)      { return fd.Token.Row, fd.Token.Column }
func (fd *FunctionDeclaration) TokenLiteral() string { return fd.Token.Literal }
func (fd *FunctionDeclaration) String() string {
	var out bytes.Buffer
	out.WriteString("FUNCTION ")
	if fd.Name != nil {
		out.WriteString(fd.Name.String())
	}
	out.WriteString(" : ")
	if fd.ReturnType != nil {
		out.WriteString(fd.ReturnType.String())
	}
	out.WriteString("\n")
	// Simplified string representation for now
	if fd.Body != nil {
		out.WriteString(fd.Body.String())
	}
	out.WriteString("\nEND_FUNCTION")
	return out.String()
}

// IlInstructionStatement represents a single instruction in an Instruction List program.
// It implements the Statement interface.
type IlInstructionStatement struct {
	Token    token.Token // The first token of the instruction (label or operator)
	Label    *Identifier // Optional label for the instruction (e.g., "MyLabel:")
	Operator string      // The instruction operator (e.g., "LD", "ST", "ADD")
	Operand  Expression  // The operand for the instruction, which can be any expression
	Modifier string      // Optional modifier (e.g., "N", "C")
}

func (ils *IlInstructionStatement) statementNode()       {}
func (ils *IlInstructionStatement) Pos() (int, int)      { return ils.Token.Row, ils.Token.Column }
func (ils *IlInstructionStatement) TokenLiteral() string { return ils.Token.Literal }
func (ils *IlInstructionStatement) String() string {
	var out bytes.Buffer

	if ils.Label != nil {
		out.WriteString(ils.Label.String() + ": ")
	}
	out.WriteString(ils.Operator)
	if ils.Operand != nil {
		out.WriteString(" " + ils.Operand.String())
	}

	return out.String()
}

type StepStatement struct {
	Token       token.Token // The 'STEP' or 'INITIAL_STEP' token
	Name        *Identifier
	IsInitial   bool
	Actions     []*ActionBlockStatement
	Transitions []*TransitionStatement
}

func (ss *StepStatement) statementNode()       {}
func (ss *StepStatement) Pos() (int, int)      { return ss.Token.Row, ss.Token.Column }
func (ss *StepStatement) TokenLiteral() string { return ss.Token.Literal }
func (ss *StepStatement) String() string       { return "STEP " + ss.Name.String() }

type TransitionStatement struct {
	Token     token.Token // The 'TRANSITION' token
	From      []*Identifier
	To        []*Identifier
	Condition Expression
}

func (ts *TransitionStatement) statementNode()       {}
func (ts *TransitionStatement) Pos() (int, int)      { return ts.Token.Row, ts.Token.Column }
func (ts *TransitionStatement) TokenLiteral() string { return ts.Token.Literal }
func (ts *TransitionStatement) String() string       { return "TRANSITION" }

// SFCProgram represents a Sequential Function Chart program.
type SFCProgram struct {
	Token    token.Token // The 'SFC' token or first element's token
	Elements []Statement
}

func (sp *SFCProgram) statementNode()       {}
func (sp *SFCProgram) TokenLiteral() string { return sp.Token.Literal }
func (sp *SFCProgram) String() string {
	var out bytes.Buffer
	for _, s := range sp.Elements {
		out.WriteString(s.String())
	}
	return out.String()
}
func (sp *SFCProgram) Pos() (int, int) {
	if len(sp.Elements) > 0 {
		return sp.Elements[0].Pos()
	}
	return sp.Token.Row, sp.Token.Column // Default or error position if no elements
}

// ActionBlockStatement represents an action associated with an SFC step.
type ActionBlockStatement struct {
	Token      token.Token // The action name (an IDENT token)
	ActionName *Identifier
	Qualifier  *Identifier
	Duration   Expression // Optional duration for time-limited qualifiers (L, D, etc.)
	Body       *BlockStatement
}

func (abs *ActionBlockStatement) statementNode()       {}
func (abs *ActionBlockStatement) Pos() (int, int)      { return abs.Token.Row, abs.Token.Column }
func (abs *ActionBlockStatement) TokenLiteral() string { return abs.Token.Literal }
func (abs *ActionBlockStatement) String() string {
	var out bytes.Buffer
	out.WriteString(abs.ActionName.String())
	out.WriteString("(")
	if abs.Qualifier != nil {
		out.WriteString(abs.Qualifier.String())
	}
	out.WriteString(");")
	return out.String()
}
