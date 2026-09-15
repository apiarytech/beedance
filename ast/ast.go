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

// Node is the base interface for all nodes in the abstract syntax tree.
type Node interface {
	TokenLiteral() string
	Pos() (int, int) // Returns (line, column)
	String() string
}

// Statement is an interface for all statement nodes.
type Statement interface {
	Node
	statementNode()
}

// Expression is an interface for all expression nodes.
type Expression interface {
	Node
	expressionNode()
}

// Visitor defines the interface for a visitor pattern that walks the AST.
// The `Visit` method is called for each node encountered.
type Visitor interface {
	Visit(node Node) (w Visitor)
}

// TypeSpecifier represents a data type in the language, e.g., INT, BOOL.
type TypeSpecifier struct {
	Token token.Token // The type token, e.g., token.INT
}

// expressionNode marks TypeSpecifier as an expression node.
func (ts *TypeSpecifier) expressionNode() {}

// Pos returns the position of the type specifier's token.
func (ts *TypeSpecifier) Pos() (int, int) { return ts.Token.Row, ts.Token.Column }

// TokenLiteral returns the literal value of the token.
func (ts *TypeSpecifier) TokenLiteral() string { return ts.Token.Literal }

// String returns the string representation of the type specifier.
func (ts *TypeSpecifier) String() string { return ts.Token.Literal }

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

// String returns the string representation of the entire program.
func (p *Program) String() string {
	var out bytes.Buffer

	for _, s := range p.Statements {
		out.WriteString(s.String())
	}

	return out.String()
}

// VarDeclStatement represents a variable declaration statement.
type VarDeclStatement struct {
	LeadingComments []string
	Token           token.Token // the 'VAR' token
	Name            *Identifier
	Location        *AtDeclaration
	AccessPath      Expression
	DataType        Expression
	Subrange        Expression
	StringLength    Expression
	Value           Expression // Initial value
	IsConstant      bool
	IsRetain        bool
	IsNonRetain     bool
	IsRisingEdge    bool
	IsFallingEdge   bool
	AccessType      string // "READ_ONLY", "READ_WRITE", or ""
}

// statementNode marks VarDeclStatement as a statement node.
func (vds *VarDeclStatement) statementNode() {}

// GetLeadingComments returns the leading comments for the statement.
func (vds *VarDeclStatement) GetLeadingComments() []string { return vds.LeadingComments }

// Pos returns the position of the statement's token.
func (vds *VarDeclStatement) Pos() (int, int) { return vds.Token.Row, vds.Token.Column }

// TokenLiteral returns the literal value of the token.
func (vds *VarDeclStatement) TokenLiteral() string { return vds.Token.Literal }

// String returns the string representation of the variable declaration statement.
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
	if vds.AccessPath != nil {
		out.WriteString(" : ")
		out.WriteString(vds.AccessPath.String())
	}

	if vds.DataType != nil {
		out.WriteString(" : ")
		if vds.IsRisingEdge {
			out.WriteString("R_EDGE ")
		}
		if vds.IsFallingEdge {
			out.WriteString("F_EDGE ")
		}
		out.WriteString(vds.DataType.String())
	}
	if vds.Subrange != nil {
		out.WriteString(" ")
		out.WriteString(vds.Subrange.String())
	}
	if vds.StringLength != nil {
		out.WriteString("(")
		out.WriteString(vds.StringLength.String())
		out.WriteString(")")
	}

	if vds.Value != nil {
		out.WriteString(" := ")
		out.WriteString(vds.Value.String())
	}

	out.WriteString(";")
	return out.String()
}

// AtDeclaration represents an AT clause for direct variable mapping.
type AtDeclaration struct {
	Token    token.Token // The 'AT' token
	Location *DirectVariable
}

// expressionNode marks AtDeclaration as an expression node.
func (ad *AtDeclaration) expressionNode() {}

// Pos returns the position of the AT token.
func (ad *AtDeclaration) Pos() (int, int) { return ad.Token.Row, ad.Token.Column }

// TokenLiteral returns the literal value of the token.
func (ad *AtDeclaration) TokenLiteral() string { return ad.Token.Literal }

// String returns the string representation of the AT declaration.
func (ad *AtDeclaration) String() string {
	var out bytes.Buffer
	out.WriteString("AT ")
	out.WriteString(ad.Location.String())
	return out.String()
}

// DirectVariable represents a directly addressed variable (e.g., %IX0.0).
type DirectVariable struct {
	Token   token.Token // The '%' token
	Address string
}

func (dv *DirectVariable) expressionNode() {}
func (dv *DirectVariable) Pos() (int, int) { return dv.Token.Row, dv.Token.Column }

// TokenLiteral returns the literal value of the token.
func (dv *DirectVariable) TokenLiteral() string { return dv.Token.Literal }

// String returns the string representation of the direct variable.
func (dv *DirectVariable) String() string {
	return "%" + dv.Address
}

// ConfigurationDeclaration represents a CONFIGURATION block.
type ConfigurationDeclaration struct {
	Token           token.Token // The 'CONFIGURATION' token
	Name            *Identifier
	GlobalVars      []*GlobalVarDeclaration
	Resources       []*ResourceDeclaration
	AccessVars      []*AccessVarDeclaration
	VarConfigs      []*ConfigVarDeclaration
	LeadingComments []string
}

// statementNode marks ConfigurationDeclaration as a statement node.
func (cd *ConfigurationDeclaration) statementNode() {}

// Pos returns the position of the configuration token.
func (cd *ConfigurationDeclaration) Pos() (int, int) { return cd.Token.Row, cd.Token.Column }

// GetLeadingComments returns the leading comments for the statement.
func (cd *ConfigurationDeclaration) GetLeadingComments() []string { return cd.LeadingComments }

// TokenLiteral returns the literal value of the token.
func (cd *ConfigurationDeclaration) TokenLiteral() string { return cd.Token.Literal }

// String returns the string representation of the configuration declaration.
func (cd *ConfigurationDeclaration) String() string {
	var out bytes.Buffer
	out.WriteString("CONFIGURATION " + cd.Name.String() + "\n")
	for _, gv := range cd.GlobalVars {
		out.WriteString(gv.String() + "\n")
	}
	for _, res := range cd.Resources {
		out.WriteString(res.String() + "\n")
	}
	for _, acc := range cd.AccessVars {
		out.WriteString(acc.String() + "\n")
	}
	for _, cfg := range cd.VarConfigs {
		out.WriteString(cfg.String() + "\n")
	}
	out.WriteString("END_CONFIGURATION")
	return out.String()
}

// ResourceDeclaration represents a RESOURCE block within a CONFIGURATION.
type ResourceDeclaration struct {
	Token        token.Token // The 'RESOURCE' token
	Name         *Identifier
	ResourceType *Identifier
	GlobalVars   []*GlobalVarDeclaration
	Tasks        []*TaskDeclaration
	Programs     []*ProgramConfiguration
}

// statementNode marks ResourceDeclaration as a statement node.
func (rd *ResourceDeclaration) statementNode() {}

// Pos returns the position of the resource token.
func (rd *ResourceDeclaration) Pos() (int, int) { return rd.Token.Row, rd.Token.Column }

// TokenLiteral returns the literal value of the token.
func (rd *ResourceDeclaration) TokenLiteral() string { return rd.Token.Literal }

// String returns the string representation of the resource declaration.
func (rd *ResourceDeclaration) String() string {
	var out bytes.Buffer
	out.WriteString("RESOURCE " + rd.Name.String() + " ON " + rd.ResourceType.String() + "\n")
	for _, gv := range rd.GlobalVars {
		out.WriteString(gv.String() + "\n")
	}
	for _, task := range rd.Tasks {
		out.WriteString(task.String() + "\n")
	}
	for _, prog := range rd.Programs {
		out.WriteString(prog.String() + "\n")
	}
	out.WriteString("END_RESOURCE")
	return out.String()
}

// TaskDeclaration represents a TASK definition within a RESOURCE.
type TaskDeclaration struct {
	Token    token.Token // The 'TASK' token
	Name     *Identifier
	Single   Expression
	Interval Expression
	Priority Expression
}

// statementNode marks TaskDeclaration as a statement node.
func (td *TaskDeclaration) statementNode() {}

// Pos returns the position of the task token.
func (td *TaskDeclaration) Pos() (int, int) { return td.Token.Row, td.Token.Column }

// TokenLiteral returns the literal value of the token.
func (td *TaskDeclaration) TokenLiteral() string { return td.Token.Literal }

// String returns the string representation of the task declaration.
func (td *TaskDeclaration) String() string {
	var out bytes.Buffer
	out.WriteString("TASK " + td.Name.String())
	params := []string{}
	if td.Single != nil {
		params = append(params, "SINGLE := "+td.Single.String())
	}
	if td.Interval != nil {
		params = append(params, "INTERVAL := "+td.Interval.String())
	}
	if td.Priority != nil {
		params = append(params, "PRIORITY := "+td.Priority.String())
	}
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(")")
	return out.String()
}

// FbTaskAssociation represents an association of a function block instance
// to a specific task within a program configuration. e.g., FB1 WITH MyTask
type FbTaskAssociation struct {
	Token    token.Token // The fb_name token
	FbName   *Identifier
	TaskName *Identifier
}

func (fta *FbTaskAssociation) expressionNode()      {}
func (fta *FbTaskAssociation) Pos() (int, int)      { return fta.Token.Row, fta.Token.Column }
func (fta *FbTaskAssociation) TokenLiteral() string { return fta.Token.Literal }
func (fta *FbTaskAssociation) String() string {
	var out bytes.Buffer
	out.WriteString(fta.FbName.String())
	out.WriteString(" WITH ")
	out.WriteString(fta.TaskName.String())
	return out.String()
}

// ProgramConfiguration represents a program instance within a RESOURCE.
type ProgramConfiguration struct {
	Token        token.Token // The 'PROGRAM' token
	IsRetain     bool        // cspell:disable-line
	IsNonRetain  bool        // cspell:disable-line
	InstanceName *Identifier
	TaskName     *Identifier // Optional: from WITH clause
	TypeName     *Identifier
	Parameters   []Expression         // Holds prog_cnxn elements
	FbTasks      []*FbTaskAssociation // Holds fb_task elements
}

// statementNode marks ProgramConfiguration as a statement node.
func (pc *ProgramConfiguration) statementNode() {}

// Pos returns the position of the program token.
func (pc *ProgramConfiguration) Pos() (int, int) { return pc.Token.Row, pc.Token.Column }

// TokenLiteral returns the literal value of the token.
func (pc *ProgramConfiguration) TokenLiteral() string { return pc.Token.Literal }

// String returns the string representation of the program configuration.
func (pc *ProgramConfiguration) String() string {
	var out bytes.Buffer
	out.WriteString("PROGRAM ")
	if pc.IsRetain {
		out.WriteString("RETAIN ")
	}
	if pc.IsNonRetain {
		out.WriteString("NON_RETAIN ")
	}
	out.WriteString(pc.InstanceName.String())
	if pc.TaskName != nil {
		out.WriteString(" WITH ")
		out.WriteString(pc.TaskName.String())
	}
	out.WriteString(" : ")
	out.WriteString(pc.TypeName.String())

	// Combine FbTasks and Parameters for printing
	allParams := []string{}
	for _, fbTask := range pc.FbTasks {
		allParams = append(allParams, fbTask.String())
	}
	for _, param := range pc.Parameters {
		allParams = append(allParams, param.String())
	}

	if len(allParams) > 0 {
		out.WriteString("(" + strings.Join(allParams, ", ") + ")")
	}
	out.WriteString(";")
	return out.String()
}

// ExpressionStatement represents a statement that consists of a single expression.
type ExpressionStatement struct {
	Token           token.Token // the first token of the expression
	Expression      Expression
	LeadingComments []string
}

// statementNode marks ExpressionStatement as a statement node.
func (es *ExpressionStatement) statementNode() {}

// GetLeadingComments returns the leading comments for the statement.
func (es *ExpressionStatement) GetLeadingComments() []string { return es.LeadingComments }

// Pos returns the position of the statement's token.
func (es *ExpressionStatement) Pos() (int, int) { return es.Token.Row, es.Token.Column }

// TokenLiteral returns the literal value of the token.
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }

// String returns the string representation of the expression statement.
func (es *ExpressionStatement) String() string {
	if es.Expression != nil {
		return es.Expression.String() + ";"
	}
	return ""
}

// AssignmentStatement represents an assignment statement (:=).
type AssignmentStatement struct {
	Token           token.Token // The ':=' token
	Left            Expression
	Value           Expression
	LeadingComments []string
}

// statementNode marks AssignmentStatement as a statement node.
func (as *AssignmentStatement) statementNode() {}

// GetLeadingComments returns the leading comments for the statement.
func (as *AssignmentStatement) GetLeadingComments() []string { return as.LeadingComments }

// Pos returns the position of the assignment token.
func (as *AssignmentStatement) Pos() (int, int) { return as.Token.Row, as.Token.Column }

// TokenLiteral returns the literal value of the token.
func (as *AssignmentStatement) TokenLiteral() string { return as.Token.Literal }

// String returns the string representation of the assignment statement.
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

// ReturnStatement represents a RETURN statement.
type ReturnStatement struct {
	Token       token.Token // the 'return' token
	ReturnValue Expression
}

// statementNode marks ReturnStatement as a statement node.
func (rs *ReturnStatement) statementNode() {}

// Pos returns the position of the return token.
func (rs *ReturnStatement) Pos() (int, int) { return rs.Token.Row, rs.Token.Column }

// TokenLiteral returns the literal value of the token.
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }

// String returns the string representation of the return statement.
func (rs *ReturnStatement) String() string {
	var out bytes.Buffer

	out.WriteString(rs.TokenLiteral() + " ")

	if rs.ReturnValue != nil {
		out.WriteString(rs.ReturnValue.String())
	}

	out.WriteString(";")

	return out.String()
}

// ExitStatement represents an EXIT statement used to terminate a loop.
type ExitStatement struct {
	Token token.Token // the 'EXIT' token
}

// statementNode marks ExitStatement as a statement node.
func (es *ExitStatement) statementNode() {}

// Pos returns the position of the exit token.
func (es *ExitStatement) Pos() (int, int) { return es.Token.Row, es.Token.Column }

// TokenLiteral returns the literal value of the token.
func (es *ExitStatement) TokenLiteral() string { return es.Token.Literal }

// String returns the string representation of the exit statement.
func (es *ExitStatement) String() string {
	var out bytes.Buffer
	out.WriteString(es.TokenLiteral() + ";")
	return out.String()
}

// BlockStatement represents a block of statements.
type BlockStatement struct {
	Token      token.Token // the { token
	Statements []Statement
}

// statementNode marks BlockStatement as a statement node.
func (bs *BlockStatement) statementNode() {}

// expressionNode allows blocks to be treated as expressions (e.g., in IL).
func (bs *BlockStatement) expressionNode() {} // Allow blocks to be treated as expressions (e.g., in IL)
// Pos returns the position of the block's starting token.
func (bs *BlockStatement) Pos() (int, int) { return bs.Token.Row, bs.Token.Column }

// TokenLiteral returns the literal value of the token.
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) String() string {
	var out bytes.Buffer

	for _, s := range bs.Statements {
		out.WriteString(s.String())
	}

	return out.String()
}

// Identifier represents an identifier in the code.
type Identifier struct {
	Token token.Token // the token.IDENT token
	Value string
}

// expressionNode marks Identifier as an expression node.
func (i *Identifier) expressionNode() {}

// Pos returns the position of the identifier's token.
func (i *Identifier) Pos() (int, int) { return i.Token.Row, i.Token.Column }

// TokenLiteral returns the literal value of the token.
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }

// String returns the value of the identifier.
func (i *Identifier) String() string { return i.Value }

// Boolean represents a boolean literal (TRUE or FALSE).
type Boolean struct {
	Token token.Token
	Value bool
}

// expressionNode marks Boolean as an expression node.
func (b *Boolean) expressionNode()      {}
func (b *Boolean) Pos() (int, int)      { return b.Token.Row, b.Token.Column }
func (b *Boolean) TokenLiteral() string { return b.Token.Literal }
func (b *Boolean) String() string       { return b.Token.Literal }

type IntegerLiteral struct {
	Token token.Token
	Value int64
	Type  token.TokenType
}

// expressionNode marks IntegerLiteral as an expression node.
func (il *IntegerLiteral) expressionNode() {}

// Pos returns the position of the integer literal's token.
func (il *IntegerLiteral) Pos() (int, int) { return il.Token.Row, il.Token.Column }

// TokenLiteral returns the literal value of the token.
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }

// String returns the string representation of the integer literal.
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

// expressionNode marks UnsignedIntegerLiteral as an expression node.
func (ul *UnsignedIntegerLiteral) expressionNode() {}

// Pos returns the position of the unsigned integer literal's token.
func (ul *UnsignedIntegerLiteral) Pos() (int, int) { return ul.Token.Row, ul.Token.Column }

// TokenLiteral returns the literal value of the token.
func (ul *UnsignedIntegerLiteral) TokenLiteral() string { return ul.Token.Literal }

// String returns the string representation of the unsigned integer literal.
func (ul *UnsignedIntegerLiteral) String() string {
	return ul.Token.Literal
}

type RealLiteral struct {
	Token     token.Token
	Value     float64
	Precision int // 32 for REAL, 64 for LREAL
}

// expressionNode marks RealLiteral as an expression node.
func (rl *RealLiteral) expressionNode() {}

// Pos returns the position of the real literal's token.
func (rl *RealLiteral) Pos() (int, int) { return rl.Token.Row, rl.Token.Column }

// TokenLiteral returns the literal value of the token.
func (rl *RealLiteral) TokenLiteral() string { return rl.Token.Literal }

// String returns the string representation of the real literal.
func (rl *RealLiteral) String() string {
	return rl.Token.Literal
}

// LRealLiteral represents a literal LREAL value.
type LRealLiteral struct {
	Token token.Token // The 'LREAL' token
	Value float64
}

// expressionNode marks LRealLiteral as an expression node.
func (lrl *LRealLiteral) expressionNode() {}

// Pos returns the position of the LREAL literal's token.
func (lrl *LRealLiteral) Pos() (int, int) { return lrl.Token.Row, lrl.Token.Column }

// TokenLiteral returns the literal value of the token.
func (lrl *LRealLiteral) TokenLiteral() string { return lrl.Token.Literal }

// String returns the string representation of the LREAL literal.
func (lrl *LRealLiteral) String() string { return lrl.Token.Literal }

// WStringLiteral represents a literal WSTRING value.
type WStringLiteral struct {
	Token token.Token // The 'WSTRING' token
	Value string
}

// expressionNode marks WStringLiteral as an expression node.
func (wsl *WStringLiteral) expressionNode() {}

// Pos returns the position of the WSTRING literal's token.
func (wsl *WStringLiteral) Pos() (int, int) { return wsl.Token.Row, wsl.Token.Column }

// TokenLiteral returns the literal value of the token.
func (wsl *WStringLiteral) TokenLiteral() string { return wsl.Token.Literal }

// String returns the string representation of the WSTRING literal.
func (wsl *WStringLiteral) String() string { return wsl.Token.Literal }

// EnumeratedValueLiteral represents a qualified enumerated value, e.g., COLOR#RED.
type EnumeratedValueLiteral struct {
	Token    token.Token // The token for the type name, e.g., 'COLOR'
	TypeName *Identifier
	Value    *Identifier
}

// expressionNode marks EnumeratedValueLiteral as an expression node.
func (evl *EnumeratedValueLiteral) expressionNode() {}

// Pos returns the position of the enumerated value's token.
func (evl *EnumeratedValueLiteral) Pos() (int, int) { return evl.Token.Row, evl.Token.Column }

// TokenLiteral returns the literal value of the token.
func (evl *EnumeratedValueLiteral) TokenLiteral() string { return evl.Token.Literal }

// String returns the string representation of the enumerated value literal.
func (evl *EnumeratedValueLiteral) String() string {
	return evl.TypeName.String() + "#" + evl.Value.String()
}

// BitStringLiteral represents a bit-string literal (e.g., BYTE, WORD).
type BitStringLiteral struct {
	Token token.Token // The token for the literal (e.g., BYTE, WORD, DWORD, LWORD)
	Value uint64
	Width int // 8, 16, 32, 64
}

func (bsl *BitStringLiteral) expressionNode() {}

// Pos returns the position of the bit-string literal's token.
func (bsl *BitStringLiteral) Pos() (int, int) { return bsl.Token.Row, bsl.Token.Column }

// TokenLiteral returns the literal value of the token.
func (bsl *BitStringLiteral) TokenLiteral() string { return bsl.Token.Literal }

// String returns the string representation of the bit-string literal.
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

// TypedLiteral represents a literal with an explicit type, e.g., INT#10.
type TypedLiteral struct {
	Token    token.Token // The type token, e.g., the 'INT' token
	TypeName string
	Value    Expression
}

// expressionNode marks TypedLiteral as an expression node.
func (tl *TypedLiteral) expressionNode() {}

// TokenLiteral returns the literal value of the token.
func (tl *TypedLiteral) TokenLiteral() string { return tl.Token.Literal }

// String returns the string representation of the typed literal.
func (tl *TypedLiteral) String() string {
	var out bytes.Buffer
	out.WriteString(tl.TypeName)
	out.WriteString("#")
	out.WriteString(tl.Value.String())
	return out.String()
}

// Pos returns the position of the typed literal's token.
func (tl *TypedLiteral) Pos() (int, int) {
	return tl.Token.Row, tl.Token.Column
}

// PrefixExpression represents a prefix operator expression (e.g., -5, NOT flag).
type PrefixExpression struct {
	Token    token.Token // The prefix token, e.g. !
	Operator string
	Right    Expression
}

// expressionNode marks PrefixExpression as an expression node.
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

// InfixExpression represents an infix operator expression (e.g., 5 + 5).
type InfixExpression struct {
	Token    token.Token // The operator token, e.g. +
	Left     Expression
	Operator string
	Right    Expression
}

// expressionNode marks InfixExpression as an expression node.
func (ie *InfixExpression) expressionNode() {}

// Pos returns the position of the infix operator's token.
func (ie *InfixExpression) Pos() (int, int) { return ie.Token.Row, ie.Token.Column }

// TokenLiteral returns the literal value of the token.
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }

// String returns the string representation of the infix expression.
func (ie *InfixExpression) String() string {
	var out bytes.Buffer

	out.WriteString("(")
	out.WriteString(ie.Left.String())
	out.WriteString(" " + ie.Operator + " ")
	out.WriteString(ie.Right.String())
	out.WriteString(")")

	return out.String()
}

// MemberAccessExpression represents accessing a member of a struct (e.g., myStruct.field).
type MemberAccessExpression struct {
	Token  token.Token // The '.' token
	Struct Expression  // The expression on the left of the dot
	Member *Identifier // The identifier on the right of the dot
}

func (mae *MemberAccessExpression) expressionNode() {}

// Pos returns the position of the '.' token.
func (mae *MemberAccessExpression) Pos() (int, int) { return mae.Token.Row, mae.Token.Column }

// TokenLiteral returns the literal value of the token.
func (mae *MemberAccessExpression) TokenLiteral() string { return mae.Token.Literal }

// String returns the string representation of the member access expression.
func (mae *MemberAccessExpression) String() string {
	return mae.Struct.String() + "." + mae.Member.String()
}

// IfStatement represents an IF...THEN...ELSIF...ELSE...END_IF statement.
type IfStatement struct {
	Token           token.Token // The 'if' token
	Condition       Expression
	Consequence     *BlockStatement
	Alternative     Statement // Can be *IfStatement (for ELSIF) or *BlockStatement (for ELSE)
	LeadingComments []string
}

// statementNode marks IfStatement as a statement node.
func (is *IfStatement) statementNode() {}

// GetLeadingComments returns the leading comments for the statement.
func (is *IfStatement) GetLeadingComments() []string { return is.LeadingComments }

// Pos returns the position of the IF token.
func (is *IfStatement) Pos() (int, int) { return is.Token.Row, is.Token.Column }

// TokenLiteral returns the literal value of the token.
func (is *IfStatement) TokenLiteral() string { return is.Token.Literal }

// String returns the string representation of the IF statement.
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

// ForLoopStatement represents a FOR loop.
type ForLoopStatement struct {
	Token           token.Token // The 'FOR' token
	ControlVar      *AssignmentStatement
	EndValue        Expression // The value to iterate TO
	StepValue       Expression // Can be nil for default step of 1
	Body            *BlockStatement
	LeadingComments []string
}

// statementNode marks ForLoopStatement as a statement node.
func (fls *ForLoopStatement) statementNode() {}

// GetLeadingComments returns the leading comments for the statement.
func (fls *ForLoopStatement) GetLeadingComments() []string { return fls.LeadingComments }

// Pos returns the position of the FOR token.
func (fls *ForLoopStatement) Pos() (int, int) { return fls.Token.Row, fls.Token.Column }

// TokenLiteral returns the literal value of the token.
func (fls *ForLoopStatement) TokenLiteral() string { return fls.Token.Literal }

// String returns the string representation of the FOR loop.
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

// WhileStatement represents a WHILE loop.
type WhileStatement struct {
	Token           token.Token // The 'WHILE' token
	Condition       Expression
	Body            *BlockStatement
	LeadingComments []string
}

// statementNode marks WhileStatement as a statement node.
func (ws *WhileStatement) statementNode() {}

// GetLeadingComments returns the leading comments for the statement.
func (ws *WhileStatement) GetLeadingComments() []string { return ws.LeadingComments }

// Pos returns the position of the WHILE token.
func (ws *WhileStatement) Pos() (int, int) { return ws.Token.Row, ws.Token.Column }

// TokenLiteral returns the literal value of the token.
func (ws *WhileStatement) TokenLiteral() string { return ws.Token.Literal }

// String returns the string representation of the WHILE loop.
func (ws *WhileStatement) String() string {
	var out bytes.Buffer
	out.WriteString("WHILE ")
	out.WriteString(ws.Condition.String())
	out.WriteString(" DO ")
	out.WriteString(ws.Body.String())
	out.WriteString(" END_WHILE")
	return out.String()
}

// RepeatStatement represents a REPEAT...UNTIL loop.
type RepeatStatement struct {
	Token           token.Token // The 'REPEAT' token
	Body            *BlockStatement
	Condition       Expression
	LeadingComments []string
}

// statementNode marks RepeatStatement as a statement node.
func (rs *RepeatStatement) statementNode() {}

// GetLeadingComments returns the leading comments for the statement.
func (rs *RepeatStatement) GetLeadingComments() []string { return rs.LeadingComments }

// Pos returns the position of the REPEAT token.
func (rs *RepeatStatement) Pos() (int, int) { return rs.Token.Row, rs.Token.Column }

// TokenLiteral returns the literal value of the token.
func (rs *RepeatStatement) TokenLiteral() string { return rs.Token.Literal }

// String returns the string representation of the REPEAT loop.
func (rs *RepeatStatement) String() string {
	var out bytes.Buffer
	out.WriteString("REPEAT ")
	out.WriteString(rs.Body.String())
	out.WriteString(" UNTIL ")
	out.WriteString(rs.Condition.String())
	out.WriteString(" END_REPEAT")
	return out.String()
}

// CaseBranch represents a single branch within a CASE statement.
type CaseBranch struct {
	Token       token.Token // The first token of the value list
	Values      []Expression
	Consequence *BlockStatement
}

// statementNode marks CaseBranch as a statement node.
func (cb *CaseBranch) statementNode() {}

// Pos returns the position of the case branch's token.
func (cb *CaseBranch) Pos() (int, int) { return cb.Token.Row, cb.Token.Column }

// TokenLiteral returns the literal value of the token.
func (cb *CaseBranch) TokenLiteral() string { return cb.Token.Literal }

// String returns the string representation of the case branch.
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

// CaseStatement represents a CASE statement.
type CaseStatement struct {
	Token           token.Token // The 'CASE' token
	Expression      Expression
	Cases           []*CaseBranch
	Alternative     *BlockStatement // The 'ELSE' block
	LeadingComments []string
}

// statementNode marks CaseStatement as a statement node.
func (cs *CaseStatement) statementNode() {}

// GetLeadingComments returns the leading comments for the statement.
func (cs *CaseStatement) GetLeadingComments() []string { return cs.LeadingComments }

// Pos returns the position of the CASE token.
func (cs *CaseStatement) Pos() (int, int) { return cs.Token.Row, cs.Token.Column }

// TokenLiteral returns the literal value of the token.
func (cs *CaseStatement) TokenLiteral() string { return cs.Token.Literal }

// String returns the string representation of the CASE statement.
func (cs *CaseStatement) String() string {
	// String representation for debugging
	var out bytes.Buffer
	out.WriteString("CASE ")
	out.WriteString(cs.Expression.String())
	out.WriteString(" OF\n")
	for _, c := range cs.Cases {
		out.WriteString("\t" + c.String() + "\n")
	}
	if cs.Alternative != nil {
		out.WriteString("ELSE\n\t")
		out.WriteString(cs.Alternative.String())
		out.WriteString("\n")
	}
	out.WriteString("END_CASE")
	return out.String()
}

// FunctionParameter represents a typed parameter in a function literal.
type FunctionParameter struct {
	Name     *Identifier
	DataType Expression // Can be TypeSpecifier or Identifier for user-defined types
}

// expressionNode marks FunctionParameter as an expression node (for consistency, though not strictly an expression).
func (fp *FunctionParameter) expressionNode() {}

// Pos returns the position of the parameter's name token.
func (fp *FunctionParameter) Pos() (int, int) { return fp.Name.Pos() }

// TokenLiteral returns the literal value of the parameter's name token.
func (fp *FunctionParameter) TokenLiteral() string { return fp.Name.TokenLiteral() }

// String returns the string representation of the function parameter.
func (fp *FunctionParameter) String() string {
	return fmt.Sprintf("%s : %s", fp.Name.String(), fp.DataType.String())
}

// FunctionLiteral represents an anonymous function expression.
type FunctionLiteral struct {
	Token      token.Token          // The 'fn' token
	Parameters []*FunctionParameter // Changed from []*Identifier
	ReturnType Expression           // New field for return type
	VarInputs  []*VarDeclStatement
	Body       *BlockStatement
	Name       string
}

// expressionNode marks FunctionLiteral as an expression node.
func (fl *FunctionLiteral) expressionNode() {}

// Pos returns the position of the function literal's token.
func (fl *FunctionLiteral) Pos() (int, int) { return fl.Token.Row, fl.Token.Column }

// TokenLiteral returns the literal value of the token.
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }

// String returns the string representation of the function literal.
func (fl *FunctionLiteral) String() string {
	var out bytes.Buffer

	params := []string{}
	if fl.VarInputs != nil {
		for _, p := range fl.VarInputs {
			params = append(params, p.String())
		}
	} else { // Fallback for older AST structures if needed
		for _, p := range fl.Parameters {
			params = append(params, p.String())
		}
	}

	out.WriteString(fl.TokenLiteral())
	if fl.Name != "" {
		out.WriteString(fmt.Sprintf("<%s>", fl.Name))
	}
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(")")
	if fl.ReturnType != nil {
		out.WriteString(" : " + fl.ReturnType.String())
	}
	out.WriteString(" ")
	out.WriteString(fl.Body.String())

	return out.String()
}

// CallExpression represents a function or function block call.
type CallExpression struct {
	Token     token.Token // The '(' token
	Function  Expression  // Identifier or FunctionLiteral
	Arguments []Expression
}

// expressionNode marks CallExpression as an expression node.
func (ce *CallExpression) expressionNode() {}

// Pos returns the position of the opening parenthesis token.
func (ce *CallExpression) Pos() (int, int) { return ce.Token.Row, ce.Token.Column }

// TokenLiteral returns the literal value of the token.
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }

// String returns the string representation of the call expression.
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

// NamedArgument represents a named input argument in a function call (e.g., In1 := 10).
type NamedArgument struct {
	Token token.Token // The identifier token for the argument name
	Name  *Identifier
	Value Expression
}

// expressionNode marks NamedArgument as an expression node.
func (na *NamedArgument) expressionNode() {}

// Pos returns the position of the argument's name token.
func (na *NamedArgument) Pos() (int, int) { return na.Token.Row, na.Token.Column }

// TokenLiteral returns the literal value of the token.
func (na *NamedArgument) TokenLiteral() string { return na.Token.Literal }

// String returns the string representation of the named argument.
func (na *NamedArgument) String() string {
	var out bytes.Buffer
	out.WriteString(na.Name.String())
	out.WriteString(" := ")
	out.WriteString(na.Value.String())
	return out.String()
}

// OutputArgument represents an output argument mapping in a function call (e.g., Out1 => Res1).
type OutputArgument struct {
	Token  token.Token // The '=>' token
	Source *Identifier
	Target Expression // Should be a variable
}

// expressionNode marks OutputArgument as an expression node.
func (oa *OutputArgument) expressionNode() {}

// Pos returns the position of the '=>' token.
func (oa *OutputArgument) Pos() (int, int) { return oa.Token.Row, oa.Token.Column }

// TokenLiteral returns the literal value of the token.
func (oa *OutputArgument) TokenLiteral() string { return oa.Token.Literal }

// String returns the string representation of the output argument.
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

// StringLiteral represents a single-byte string literal.
type StringLiteral struct {
	Token token.Token
	Value string
}

// expressionNode marks StringLiteral as an expression node.
func (sl *StringLiteral) expressionNode() {}

// Pos returns the position of the string literal's token.
func (sl *StringLiteral) Pos() (int, int) { return sl.Token.Row, sl.Token.Column }

// TokenLiteral returns the literal value of the token.
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }

// String returns the string representation of the string literal.
func (sl *StringLiteral) String() string { return sl.Token.Literal }

// TimeLiteral represents a TIME literal (e.g., T#5s).
type TimeLiteral struct {
	Token token.Token // The token.TIME token
	Value string      // The raw string value, e.g., "T#5s"
}

// expressionNode marks TimeLiteral as an expression node.
func (tl *TimeLiteral) expressionNode() {}

// Pos returns the position of the TIME literal's token.
func (tl *TimeLiteral) Pos() (int, int) { return tl.Token.Row, tl.Token.Column }

// TokenLiteral returns the literal value of the token.
func (tl *TimeLiteral) TokenLiteral() string { return tl.Token.Literal }

// String returns the string representation of the TIME literal.
func (tl *TimeLiteral) String() string {
	return tl.Value
}

// DateLiteral represents a DATE literal (e.g., D#2026-05-21).
type DateLiteral struct {
	Token token.Token // The token.DATE token
	Value string
}

// expressionNode marks DateLiteral as an expression node.
func (dl *DateLiteral) expressionNode() {}

// Pos returns the position of the DATE literal's token.
func (dl *DateLiteral) Pos() (int, int) { return dl.Token.Row, dl.Token.Column }

// TokenLiteral returns the literal value of the token.
func (dl *DateLiteral) TokenLiteral() string { return dl.Token.Literal }

// String returns the string representation of the DATE literal.
func (dl *DateLiteral) String() string {
	return dl.Value
}

// TimeOfDayLiteral represents a TIME_OF_DAY literal (e.g., TOD#14:30:00).
type TimeOfDayLiteral struct {
	Token token.Token // The token.TIME_OF_DAY token
	Value string
}

// expressionNode marks TimeOfDayLiteral as an expression node.
func (todl *TimeOfDayLiteral) expressionNode() {}

// Pos returns the position of the TIME_OF_DAY literal's token.
func (todl *TimeOfDayLiteral) Pos() (int, int) { return todl.Token.Row, todl.Token.Column }

// TokenLiteral returns the literal value of the token.
func (todl *TimeOfDayLiteral) TokenLiteral() string { return todl.Token.Literal }
func (todl *TimeOfDayLiteral) String() string {
	return todl.Value
}

type DateAndTimeLiteral struct {
	Token token.Token // The token.DATE_AND_TIME token
	Value string
}

// expressionNode marks DateAndTimeLiteral as an expression node.
func (dtl *DateAndTimeLiteral) expressionNode() {}

// Pos returns the position of the DATE_AND_TIME literal's token.
func (dtl *DateAndTimeLiteral) Pos() (int, int) { return dtl.Token.Row, dtl.Token.Column }

// TokenLiteral returns the literal value of the token.
func (dtl *DateAndTimeLiteral) TokenLiteral() string { return dtl.Token.Literal }

// String returns the string representation of the DATE_AND_TIME literal.
func (dtl *DateAndTimeLiteral) String() string {
	return dtl.Value
}

// ArrayLiteral represents an array literal expression (e.g., [1, 2, 3]).
type ArrayLiteral struct {
	Token    token.Token // the '[' token
	Elements []Expression
}

// expressionNode marks ArrayLiteral as an expression node.
func (al *ArrayLiteral) expressionNode() {}

// Pos returns the position of the opening bracket token.
func (na *ArrayLiteral) Pos() (int, int) { return na.Token.Row, na.Token.Column }

// TokenLiteral returns the literal value of the token.
func (al *ArrayLiteral) TokenLiteral() string { return al.Token.Literal }

// String returns the string representation of the array literal.
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

// ArrayRepetition represents a repetition factor in an array literal, e.g., 3(0) or 2(1,2,3).
type ArrayRepetition struct {
	Token    token.Token  // The token for the repetition factor (e.g., the '3' in 3(0))
	Factor   Expression   // The repetition factor (e.g., IntegerLiteral for 3)
	Elements []Expression // The elements to repeat (e.g., [0] for 3(0))
}

// expressionNode marks ArrayRepetition as an expression node.
func (ar *ArrayRepetition) expressionNode() {}

// Pos returns the position of the repetition factor's token.
func (ar *ArrayRepetition) Pos() (int, int) { return ar.Token.Row, ar.Token.Column }

// TokenLiteral returns the literal value of the token.
func (ar *ArrayRepetition) TokenLiteral() string { return ar.Token.Literal }

// String returns the string representation of the array repetition.
func (ar *ArrayRepetition) String() string {
	var out bytes.Buffer
	out.WriteString(ar.Factor.String())
	out.WriteString("(")
	elements := []string{}
	for _, el := range ar.Elements {
		elements = append(elements, el.String())
	}
	out.WriteString(strings.Join(elements, ", "))
	out.WriteString(")")
	return out.String()
}

// IndexExpression represents an array or string indexing expression (e.g., myArray[i]).
type IndexExpression struct {
	Token token.Token // The [ token
	Left  Expression
	Index Expression
}

// expressionNode marks IndexExpression as an expression node.
func (ie *IndexExpression) expressionNode() {}

// Pos returns the position of the opening bracket token.
func (ie *IndexExpression) Pos() (int, int) { return ie.Token.Row, ie.Token.Column }

// TokenLiteral returns the literal value of the token.
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }

// String returns the string representation of the index expression.
func (ie *IndexExpression) String() string {
	var out bytes.Buffer

	out.WriteString("(")
	out.WriteString(ie.Left.String())
	out.WriteString("[")
	out.WriteString(ie.Index.String())
	out.WriteString("])")

	return out.String()
}

// HashLiteral represents a hash or map literal (e.g., { 'key': 'value' }).
type HashLiteral struct {
	Token token.Token // the '{' token
	Pairs map[Expression]Expression
}

// expressionNode marks HashLiteral as an expression node.
func (hl *HashLiteral) expressionNode() {}

// Pos returns the position of the opening brace token.
func (hl *HashLiteral) Pos() (int, int) { return hl.Token.Row, hl.Token.Column }

// TokenLiteral returns the literal value of the token.
func (hl *HashLiteral) TokenLiteral() string { return hl.Token.Literal }

// String returns the string representation of the hash literal.
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

// MacroLiteral represents a macro definition.
type MacroLiteral struct {
	Token      token.Token // The 'macro' token
	Parameters []*Identifier
	Body       *BlockStatement
}

// expressionNode marks MacroLiteral as an expression node.
func (ml *MacroLiteral) expressionNode() {}

// Pos returns the position of the macro token.
func (ml *MacroLiteral) Pos() (int, int) { return ml.Token.Row, ml.Token.Column }

// TokenLiteral returns the literal value of the token.
func (ml *MacroLiteral) TokenLiteral() string { return ml.Token.Literal }

// String returns the string representation of the macro literal.
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

// FunctionBlockDeclaration represents a FUNCTION_BLOCK declaration.
type FunctionBlockDeclaration struct {
	Token           token.Token // The 'FUNCTION_BLOCK' token
	Name            *Identifier
	Extends         *Identifier // For FB inheritance
	Implements      []*Identifier
	VarInputs       []*VarDeclStatement
	VarOutputs      []*VarDeclStatement
	VarInOuts       []*VarDeclStatement
	VarExternal     []*ExternalVarDeclaration
	Vars            []*VarDeclStatement
	VarTemp         []*TempVarDeclaration
	Body            Statement
	LeadingComments []string
}

// statementNode marks FunctionBlockDeclaration as a statement node.
func (fbd *FunctionBlockDeclaration) statementNode() {}

// GetLeadingComments returns the leading comments for the statement.
func (fbd *FunctionBlockDeclaration) GetLeadingComments() []string { return fbd.LeadingComments }

// Pos returns the position of the FUNCTION_BLOCK token.
func (fbd *FunctionBlockDeclaration) Pos() (int, int) { return fbd.Token.Row, fbd.Token.Column }

// TokenLiteral returns the literal value of the token.
func (fbd *FunctionBlockDeclaration) TokenLiteral() string { return fbd.Token.Literal }

// String returns the string representation of the function block declaration.
func (fbd *FunctionBlockDeclaration) String() string {
	var out bytes.Buffer
	out.WriteString("FUNCTION_BLOCK ")
	if fbd.Name != nil {
		out.WriteString(fbd.Name.String())
	}
	if fbd.Extends != nil {
		out.WriteString(" EXTENDS " + fbd.Extends.String())
	}
	if len(fbd.Implements) > 0 {
		out.WriteString(" IMPLEMENTS ")
		impls := []string{}
		for _, i := range fbd.Implements {
			impls = append(impls, i.String())
		}
		out.WriteString(strings.Join(impls, ", "))
	}
	out.WriteString("\n")
	// Simplified string representation for now
	if fbd.Body != nil {
		out.WriteString(fbd.Body.String())
	}
	out.WriteString("\nEND_FUNCTION_BLOCK")
	return out.String()
}

// ProgramDeclaration represents a PROGRAM declaration.
type ProgramDeclaration struct {
	Token           token.Token // The 'PROGRAM' token
	Name            *Identifier
	VarInputs       []*VarDeclStatement
	VarOutputs      []*VarDeclStatement
	VarInOuts       []*VarDeclStatement
	Vars            []*VarDeclStatement
	VarExternal     []*ExternalVarDeclaration
	VarGlobal       []*GlobalVarDeclaration
	VarAccess       []*AccessVarDeclaration
	VarTemp         []*TempVarDeclaration
	Body            Statement
	LeadingComments []string
}

// statementNode marks ProgramDeclaration as a statement node.
func (pd *ProgramDeclaration) statementNode() {}

// GetLeadingComments returns the leading comments for the statement.
func (pd *ProgramDeclaration) GetLeadingComments() []string { return pd.LeadingComments }

// Pos returns the position of the PROGRAM token.
func (pd *ProgramDeclaration) Pos() (int, int) { return pd.Token.Row, pd.Token.Column }

// TokenLiteral returns the literal value of the token.
func (pd *ProgramDeclaration) TokenLiteral() string { return pd.Token.Literal }

// String returns the string representation of the program declaration.
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

// ExternalVarDeclaration represents a VAR_EXTERNAL block.
type ExternalVarDeclaration struct {
	Token token.Token // The 'VAR_EXTERNAL' token
	Vars  []*VarDeclStatement
}

// statementNode marks ExternalVarDeclaration as a statement node.
func (evd *ExternalVarDeclaration) statementNode() {}

// Pos returns the position of the VAR_EXTERNAL token.
func (evd *ExternalVarDeclaration) Pos() (int, int) { return evd.Token.Row, evd.Token.Column }

// TokenLiteral returns the literal value of the token.
func (evd *ExternalVarDeclaration) TokenLiteral() string { return evd.Token.Literal }

// String returns the string representation of the external variable declaration block.
func (evd *ExternalVarDeclaration) String() string {
	var out bytes.Buffer
	out.WriteString("VAR_EXTERNAL\n")
	for _, v := range evd.Vars {
		out.WriteString("\t" + v.String() + "\n")
	}
	out.WriteString("END_VAR")
	return out.String()
}

// ConfigVarDeclaration represents a VAR_CONFIG block.
type ConfigVarDeclaration struct {
	Token               token.Token // The 'VAR_CONFIG' token
	ProgramInstanceName *Identifier
	Declarations        []*VarDeclStatement
}

// statementNode marks ConfigVarDeclaration as a statement node.
func (cvd *ConfigVarDeclaration) statementNode() {}

// Pos returns the position of the VAR_CONFIG token.
func (cvd *ConfigVarDeclaration) Pos() (int, int) { return cvd.Token.Row, cvd.Token.Column }

// TokenLiteral returns the literal value of the token.
func (cvd *ConfigVarDeclaration) TokenLiteral() string { return cvd.Token.Literal }

// String returns the string representation of the config variable declaration block.
func (cvd *ConfigVarDeclaration) String() string {
	var out bytes.Buffer
	cvd.Pos() // Ensure Pos() is called
	out.WriteString("VAR_CONFIG")
	if cvd.ProgramInstanceName != nil {
		out.WriteString(" " + cvd.ProgramInstanceName.String())
	}
	out.WriteString("\n")
	for _, v := range cvd.Declarations {
		out.WriteString("\t" + v.String() + "\n")
	}
	out.WriteString("END_VAR")
	return out.String()
}

// TempVarDeclaration represents a VAR_TEMP block.
type TempVarDeclaration struct {
	Token token.Token // The 'VAR_TEMP' token
	Vars  []*VarDeclStatement
}

// statementNode marks TempVarDeclaration as a statement node.
func (tvd *TempVarDeclaration) statementNode() {}

// Pos returns the position of the VAR_TEMP token.
func (tvd *TempVarDeclaration) Pos() (int, int) { return tvd.Token.Row, tvd.Token.Column }

// TokenLiteral returns the literal value of the token.
func (tvd *TempVarDeclaration) TokenLiteral() string { return tvd.Token.Literal }

// String returns the string representation of the temporary variable declaration block.
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

// AccessVarDeclaration represents a VAR_ACCESS block.
type AccessVarDeclaration struct {
	Token token.Token // The 'VAR_ACCESS' token
	Vars  []*VarDeclStatement
}

// statementNode marks AccessVarDeclaration as a statement node.
func (avd *AccessVarDeclaration) statementNode() {}

// Pos returns the position of the VAR_ACCESS token.
func (avd *AccessVarDeclaration) Pos() (int, int) { return avd.Token.Row, avd.Token.Column }

// TokenLiteral returns the literal value of the token.
func (avd *AccessVarDeclaration) TokenLiteral() string { return avd.Token.Literal }

// String returns the string representation of the access variable declaration block.
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

// GlobalVarDeclaration represents a VAR_GLOBAL block.
type GlobalVarDeclaration struct {
	Token token.Token // The 'VAR_GLOBAL' token
	Vars  []*VarDeclStatement
}

// statementNode marks GlobalVarDeclaration as a statement node.
func (gvd *GlobalVarDeclaration) statementNode() {}

// Pos returns the position of the VAR_GLOBAL token.
func (gvd *GlobalVarDeclaration) Pos() (int, int) { return gvd.Token.Row, gvd.Token.Column }

// TokenLiteral returns the literal value of the token.
func (gvd *GlobalVarDeclaration) TokenLiteral() string { return gvd.Token.Literal }

// String returns the string representation of the global variable declaration block.
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

// VarBlockDeclaration represents a VAR block.
type VarBlockDeclaration struct {
	Token           token.Token // The 'VAR' token
	Declarations    []*VarDeclStatement
	LeadingComments []string
}

// statementNode marks VarBlockDeclaration as a statement node.
func (vbd *VarBlockDeclaration) statementNode() {}

// GetLeadingComments returns the leading comments for the statement.
func (vbd *VarBlockDeclaration) GetLeadingComments() []string { return vbd.LeadingComments }

// Pos returns the position of the VAR token.
func (vbd *VarBlockDeclaration) Pos() (int, int) { return vbd.Token.Row, vbd.Token.Column }

// TokenLiteral returns the literal value of the token.
func (vbd *VarBlockDeclaration) TokenLiteral() string { return vbd.Token.Literal }

// String returns the string representation of the variable declaration block.
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

// TypeDeclaration represents a single declaration within a TYPE block.
type TypeDeclaration struct {
	LeadingComments []string
	Token           token.Token // The identifier token (the name of the new type)
	Name            *Identifier
	DataType        Expression
	Subrange        Expression // For subrange types, e.g., (0..100)
	StringLength    Expression // For string length, e.g., (10)
	InitialValue    Expression // For initialized types, e.g., := 10
}

// statementNode marks TypeDeclaration as a statement node.
func (td *TypeDeclaration) statementNode() {}

// GetLeadingComments returns the leading comments for the statement.
func (td *TypeDeclaration) GetLeadingComments() []string { return td.LeadingComments }

// Pos returns the position of the type declaration's token.
func (td *TypeDeclaration) Pos() (int, int) { return td.Token.Row, td.Token.Column }

// TokenLiteral returns the literal value of the token.
func (td *TypeDeclaration) TokenLiteral() string { return td.Token.Literal }

// String returns the string representation of the type declaration.
func (td *TypeDeclaration) String() string {
	var out bytes.Buffer
	out.WriteString(td.Name.String())
	out.WriteString(" : ")
	out.WriteString(td.DataType.String())
	if td.Subrange != nil {
		out.WriteString(" ")
		out.WriteString(td.Subrange.String())
	}
	if td.StringLength != nil {
		out.WriteString("(")
		out.WriteString(td.StringLength.String())
		out.WriteString(")")
	}
	if td.InitialValue != nil {
		out.WriteString(" := ")
		out.WriteString(td.InitialValue.String())
	}
	out.WriteString(";")
	return out.String()
}

// TypeBlockDeclaration represents a TYPE...END_TYPE block.
type TypeBlockDeclaration struct {
	Token           token.Token // The 'TYPE' token
	Declarations    []*TypeDeclaration
	LeadingComments []string
}

// statementNode marks TypeBlockDeclaration as a statement node.
func (tbd *TypeBlockDeclaration) statementNode() {}

// GetLeadingComments returns the leading comments for the statement.
func (tbd *TypeBlockDeclaration) GetLeadingComments() []string { return tbd.LeadingComments }

// Pos returns the position of the TYPE token.
func (tbd *TypeBlockDeclaration) Pos() (int, int) { return tbd.Token.Row, tbd.Token.Column }

// TokenLiteral returns the literal value of the token.
func (tbd *TypeBlockDeclaration) TokenLiteral() string { return tbd.Token.Literal }

// String returns the string representation of the type declaration block.
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

// StructDefinition represents a STRUCT definition.
type StructDefinition struct {
	Token   token.Token // The 'STRUCT' token
	Members []*VarDeclStatement
}

// expressionNode marks StructDefinition as an expression node.
func (sd *StructDefinition) expressionNode() {}

// Pos returns the position of the STRUCT token.
func (sd *StructDefinition) Pos() (int, int) { return sd.Token.Row, sd.Token.Column }

// TokenLiteral returns the literal value of the token.
func (sd *StructDefinition) TokenLiteral() string { return sd.Token.Literal }

// String returns the string representation of the struct definition.
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

// GetMemberType finds a member by name and returns its data type name as a string.
// This is a helper for analysis tools like the transpiler. It handles simple types
// and drills down through arrays to find the base type name.
func (sd *StructDefinition) GetMemberType(memberName string) string {
	for _, member := range sd.Members {
		if member.Name.Value == memberName {
			if member.DataType != nil {
				currentType := member.DataType
				// For arrays, we need to get the base element type.
				for {
					if arrayDef, ok := currentType.(*ArrayDefinition); ok {
						currentType = arrayDef.DataType
					} else {
						break
					}
				}
				return currentType.String()
			}
		}
	}
	return "" // Member not found or has no type
}

// EnumDefinition represents an enumerated type definition.
type EnumDefinition struct {
	Token  token.Token // The '(' token
	Values []*Identifier
}

// expressionNode marks EnumDefinition as an expression node.
func (ed *EnumDefinition) expressionNode() {}

// Pos returns the position of the opening parenthesis token.
func (ed *EnumDefinition) Pos() (int, int) { return ed.Token.Row, ed.Token.Column }

// TokenLiteral returns the literal value of the token.
func (ed *EnumDefinition) TokenLiteral() string { return ed.Token.Literal }

// String returns the string representation of the enum definition.
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

// ArrayDefinition represents an ARRAY type definition.
type ArrayDefinition struct {
	Token    token.Token // The 'ARRAY' token
	Ranges   []Expression
	DataType *TypeSpecifier
}

// expressionNode marks ArrayDefinition as an expression node.
func (ad *ArrayDefinition) expressionNode() {}

// Pos returns the position of the ARRAY token.
func (ad *ArrayDefinition) Pos() (int, int) { return ad.Token.Row, ad.Token.Column }

// TokenLiteral returns the literal value of the token.
func (ad *ArrayDefinition) TokenLiteral() string { return ad.Token.Literal }

// String returns the string representation of the array definition.
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

// ActionStatement represents an ACTION block in an SFC.
type ActionStatement struct {
	Token           token.Token // The 'ACTION' token
	LeadingComments []string
	Name            *Identifier
	Body            Statement
}

// statementNode marks ActionStatement as a statement node.
func (as *ActionStatement) statementNode() {}

// Pos returns the position of the ACTION token.
func (as *ActionStatement) Pos() (int, int) { return as.Token.Row, as.Token.Column }

// GetLeadingComments returns the leading comments for the statement.
func (as *ActionStatement) GetLeadingComments() []string { return as.LeadingComments }

// TokenLiteral returns the literal value of the token.
func (as *ActionStatement) TokenLiteral() string { return as.Token.Literal }

// String returns the string representation of the action statement.
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

// FunctionDeclaration represents a FUNCTION declaration.
type FunctionDeclaration struct {
	Token           token.Token // The 'FUNCTION' token
	Name            *Identifier
	ReturnType      *TypeSpecifier
	VarInputs       []*VarDeclStatement
	VarOutputs      []*VarDeclStatement
	VarInOuts       []*VarDeclStatement
	Vars            []*VarDeclStatement
	Body            Statement
	LeadingComments []string
}

// statementNode marks FunctionDeclaration as a statement node.
func (fd *FunctionDeclaration) statementNode() {}

// GetLeadingComments returns the leading comments for the statement.
func (fd *FunctionDeclaration) GetLeadingComments() []string { return fd.LeadingComments }

// Pos returns the position of the FUNCTION token.
func (fd *FunctionDeclaration) Pos() (int, int) { return fd.Token.Row, fd.Token.Column }

// TokenLiteral returns the literal value of the token.
func (fd *FunctionDeclaration) TokenLiteral() string { return fd.Token.Literal }

// String returns the string representation of the function declaration.
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

// statementNode marks IlInstructionStatement as a statement node.
func (ils *IlInstructionStatement) statementNode() {}

// Pos returns the position of the instruction's token.
func (ils *IlInstructionStatement) Pos() (int, int) { return ils.Token.Row, ils.Token.Column }

// TokenLiteral returns the literal value of the token.
func (ils *IlInstructionStatement) TokenLiteral() string { return ils.Token.Literal }

// String returns the string representation of the IL instruction.
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

// StepStatement represents a STEP in an SFC.
type StepStatement struct {
	Token       token.Token // The 'STEP' or 'INITIAL_STEP' token
	Name        *Identifier
	IsInitial   bool
	Actions     []*ActionBlockStatement
	Transitions []*TransitionStatement
	Body        *BlockStatement // For textual step bodies
}

// statementNode marks StepStatement as a statement node.
func (ss *StepStatement) statementNode() {}

// Pos returns the position of the STEP token.
func (ss *StepStatement) Pos() (int, int) { return ss.Token.Row, ss.Token.Column }

// TokenLiteral returns the literal value of the token.
func (ss *StepStatement) TokenLiteral() string { return ss.Token.Literal }

// String returns the string representation of the step statement.
func (ss *StepStatement) String() string {
	var out bytes.Buffer
	if ss.IsInitial {
		out.WriteString("INITIAL_STEP ")
	} else {
		out.WriteString("STEP ")
	}
	out.WriteString(ss.Name.String())
	out.WriteString(":\n")
	for _, a := range ss.Actions {
		out.WriteString("\t" + a.String() + "\n")
	}
	if ss.Body != nil {
		bodyStr := ss.Body.String()
		if bodyStr != "" {
			if len(ss.Actions) > 0 {
				out.WriteString("\n")
			}
			out.WriteString(bodyStr)
			out.WriteString("\n")
		}
	}
	out.WriteString("END_STEP")
	return out.String()
}

// TransitionStatement represents a TRANSITION in an SFC.
type TransitionStatement struct {
	Token     token.Token // The 'TRANSITION' token
	From      []*Identifier
	To        []*Identifier
	Condition Expression
}

// statementNode marks TransitionStatement as a statement node.
func (ts *TransitionStatement) statementNode() {}

// Pos returns the position of the TRANSITION token.
func (ts *TransitionStatement) Pos() (int, int) { return ts.Token.Row, ts.Token.Column }

// TokenLiteral returns the literal value of the token.
func (ts *TransitionStatement) TokenLiteral() string { return ts.Token.Literal }

// String returns the string representation of the transition statement.
func (ts *TransitionStatement) String() string {
	var out bytes.Buffer
	out.WriteString("TRANSITION")
	if len(ts.From) > 0 {
		out.WriteString(" FROM ")
		froms := []string{}
		for _, f := range ts.From {
			froms = append(froms, f.String())
		}
		out.WriteString(strings.Join(froms, ", "))
	}
	if len(ts.To) > 0 {
		out.WriteString(" TO ")
		tos := []string{}
		for _, t := range ts.To {
			tos = append(tos, t.String())
		}
		out.WriteString(strings.Join(tos, ", "))
	}
	out.WriteString(" := ")
	if ts.Condition != nil {
		out.WriteString(ts.Condition.String())
	}
	out.WriteString(";\nEND_TRANSITION")
	return out.String()
}

// SFCProgram represents a Sequential Function Chart program.
type SFCProgram struct {
	Token    token.Token // The 'SFC' token or first element's token
	Elements []Statement
}

// statementNode marks SFCProgram as a statement node.
func (sp *SFCProgram) statementNode() {}

// TokenLiteral returns the literal value of the token.
func (sp *SFCProgram) TokenLiteral() string {
	if len(sp.Elements) > 0 {
		return sp.Elements[0].TokenLiteral()
	}
	return sp.Token.Literal
}

// String returns the string representation of the SFC program.
func (sp *SFCProgram) String() string {
	var out bytes.Buffer
	for _, s := range sp.Elements {
		out.WriteString(s.String() + "\n")
	}
	return out.String()
}

// Pos returns the position of the first element in the SFC program.
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

// statementNode marks ActionBlockStatement as a statement node.
func (abs *ActionBlockStatement) statementNode() {}

// Pos returns the position of the action block's token.
func (abs *ActionBlockStatement) Pos() (int, int) { return abs.Token.Row, abs.Token.Column }

// TokenLiteral returns the literal value of the token.
func (abs *ActionBlockStatement) TokenLiteral() string { return abs.Token.Literal }

// String returns the string representation of the action block statement.
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

// ReferenceType represents a REFERENCE TO <data_type> specifier.
type ReferenceType struct {
	Token    token.Token // The 'REFERENCE' token
	BaseType Expression  // The data type being referenced
}

// expressionNode marks ReferenceType as an expression node.
func (rt *ReferenceType) expressionNode() {}

// Pos returns the position of the REFERENCE token.
func (rt *ReferenceType) Pos() (int, int) { return rt.Token.Row, rt.Token.Column }

// TokenLiteral returns the literal value of the token.
func (rt *ReferenceType) TokenLiteral() string { return rt.Token.Literal }

// String returns the string representation of the reference type.
func (rt *ReferenceType) String() string {
	var out bytes.Buffer
	out.WriteString("REFERENCE TO " + rt.BaseType.String())
	return out.String()
}

// MethodDeclaration represents a method signature within an INTERFACE.
type MethodDeclaration struct {
	Token      token.Token // The 'METHOD' token
	Name       *Identifier
	ReturnType *TypeSpecifier
	VarInputs  []*VarDeclStatement
	VarOutputs []*VarDeclStatement
	VarInOuts  []*VarDeclStatement
}

// statementNode marks MethodDeclaration as a statement node.
func (md *MethodDeclaration) statementNode() {}

// Pos returns the position of the METHOD token.
func (md *MethodDeclaration) Pos() (int, int) { return md.Token.Row, md.Token.Column }

// TokenLiteral returns the literal value of the token.
func (md *MethodDeclaration) TokenLiteral() string { return md.Token.Literal }

// String returns the string representation of the method declaration.
func (md *MethodDeclaration) String() string {
	var out bytes.Buffer
	out.WriteString("METHOD ")
	if md.Name != nil {
		out.WriteString(md.Name.String())
	}
	if md.ReturnType != nil {
		out.WriteString(" : ")
		out.WriteString(md.ReturnType.String())
	}
	out.WriteString("\n")
	for _, v := range md.VarInputs {
		out.WriteString("\t" + v.String() + "\n")
	}
	for _, v := range md.VarOutputs {
		out.WriteString("\t" + v.String() + "\n")
	}
	for _, v := range md.VarInOuts {
		out.WriteString("\t" + v.String() + "\n")
	}
	out.WriteString("END_METHOD")
	return out.String()
}

// InterfaceDeclaration represents an INTERFACE ... END_INTERFACE block.
type InterfaceDeclaration struct {
	Token           token.Token // The 'INTERFACE' token
	Name            *Identifier
	Methods         []*MethodDeclaration
	LeadingComments []string
}

// statementNode marks InterfaceDeclaration as a statement node.
func (id *InterfaceDeclaration) statementNode() {}

// GetLeadingComments returns the leading comments for the statement.
func (id *InterfaceDeclaration) GetLeadingComments() []string { return id.LeadingComments }

// Pos returns the position of the INTERFACE token.
func (id *InterfaceDeclaration) Pos() (int, int) { return id.Token.Row, id.Token.Column }

// TokenLiteral returns the literal value of the token.
func (id *InterfaceDeclaration) TokenLiteral() string { return id.Token.Literal }

// String returns the string representation of the interface declaration.
func (id *InterfaceDeclaration) String() string {
	var out bytes.Buffer
	out.WriteString("INTERFACE ")
	if id.Name != nil {
		out.WriteString(id.Name.String())
	}
	out.WriteString("\n")
	for _, method := range id.Methods {
		out.WriteString("\t" + method.String() + "\n")
	}
	out.WriteString("END_INTERFACE")
	return out.String()
}

// MethodImplementation represents a method implementation within a FUNCTION_BLOCK.
type MethodImplementation struct {
	Token      token.Token // The 'METHOD' token
	Name       *Identifier
	ReturnType *TypeSpecifier
	VarInputs  []*VarDeclStatement
	VarOutputs []*VarDeclStatement
	VarInOuts  []*VarDeclStatement
	Vars       []*VarDeclStatement
	Body       *BlockStatement
}

// statementNode marks MethodImplementation as a statement node.
func (mi *MethodImplementation) statementNode() {}

// Pos returns the position of the METHOD token.
func (mi *MethodImplementation) Pos() (int, int) { return mi.Token.Row, mi.Token.Column }

// TokenLiteral returns the literal value of the token.
func (mi *MethodImplementation) TokenLiteral() string { return mi.Token.Literal }

// String returns the string representation of the method implementation.
func (mi *MethodImplementation) String() string {
	var out bytes.Buffer
	out.WriteString("METHOD ")
	if mi.Name != nil {
		out.WriteString(mi.Name.String())
	}
	if mi.ReturnType != nil {
		out.WriteString(" : ")
		out.WriteString(mi.ReturnType.String())
	}
	out.WriteString("\n")
	for _, v := range mi.VarInputs {
		out.WriteString("\t" + v.String() + "\n")
	}
	for _, v := range mi.VarOutputs {
		out.WriteString("\t" + v.String() + "\n")
	}
	for _, v := range mi.VarInOuts {
		out.WriteString("\t" + v.String() + "\n")
	}
	for _, v := range mi.Vars {
		out.WriteString("\t" + v.String() + "\n")
	}
	if mi.Body != nil {
		out.WriteString(mi.Body.String())
	}
	out.WriteString("\nEND_METHOD")
	return out.String()
}

// ThisExpression represents the 'THIS' keyword, a pointer to the current FB instance.
type ThisExpression struct {
	Token token.Token // The 'THIS' token
}

// expressionNode marks ThisExpression as an expression node.
func (te *ThisExpression) expressionNode() {}

// Pos returns the position of the THIS token.
func (te *ThisExpression) Pos() (int, int) { return te.Token.Row, te.Token.Column }

// TokenLiteral returns the literal value of the token.
func (te *ThisExpression) TokenLiteral() string { return te.Token.Literal }

// String returns the string representation of the THIS expression.
func (te *ThisExpression) String() string { return "THIS" }

// SuperExpression represents the 'SUPER' keyword for calling parent methods.
type SuperExpression struct {
	Token token.Token // The 'SUPER' token
}

func (se *SuperExpression) expressionNode()      {}
func (se *SuperExpression) Pos() (int, int)      { return se.Token.Row, se.Token.Column }
func (se *SuperExpression) TokenLiteral() string { return se.Token.Literal }
func (se *SuperExpression) String() string       { return "SUPER" }

// DereferenceExpression represents dereferencing a pointer (e.g., MyPointer^).
type DereferenceExpression struct {
	Token   token.Token // The '^' token
	Pointer Expression  // The expression being dereferenced (e.g., THIS, SUPER, a REFERENCE TO variable)
}

func (de *DereferenceExpression) expressionNode()      {}
func (de *DereferenceExpression) Pos() (int, int)      { return de.Token.Row, de.Token.Column }
func (de *DereferenceExpression) TokenLiteral() string { return de.Token.Literal }
func (de *DereferenceExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(de.Pointer.String())
	out.WriteString("^)")
	return out.String()
}

// StructLiteral represents a struct initialization, e.g., (Field1 := 1, Field2 := TRUE).
type StructLiteral struct {
	Token        token.Token  // The '(' token
	Initializers []Expression // Should be []*NamedArgument
}

func (sl *StructLiteral) expressionNode()      {}
func (sl *StructLiteral) Pos() (int, int)      { return sl.Token.Row, sl.Token.Column }
func (sl *StructLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StructLiteral) String() string {
	var out bytes.Buffer
	inits := []string{}
	for _, i := range sl.Initializers {
		inits = append(inits, i.String())
	}
	out.WriteString("(")
	out.WriteString(strings.Join(inits, ", "))
	out.WriteString(")")
	return out.String()
}
