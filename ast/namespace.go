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
	"beedance/token"
	"bytes"
)

// NamespaceDeclaration represents a NAMESPACE ... END_NAMESPACE block.
type NamespaceDeclaration struct {
	Token           token.Token // The 'NAMESPACE' token
	Name            Expression
	Statements      []Statement
	LeadingComments []string
}

func (nsd *NamespaceDeclaration) statementNode() {}

// Pos returns the position of the namespace token.
func (nsd *NamespaceDeclaration) Pos() (int, int)      { return nsd.Token.Row, nsd.Token.Column }
func (nsd *NamespaceDeclaration) TokenLiteral() string { return nsd.Token.Literal }
func (nsd *NamespaceDeclaration) String() string {
	var out bytes.Buffer
	out.WriteString("NAMESPACE ")
	out.WriteString(nsd.Name.String())
	out.WriteString("\n")
	for _, s := range nsd.Statements {
		out.WriteString(s.String())
	}
	out.WriteString("\nEND_NAMESPACE")
	return out.String()
}
func (nsd *NamespaceDeclaration) GetLeadingComments() []string { return nsd.LeadingComments }
