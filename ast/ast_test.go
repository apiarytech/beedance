package ast

import (
	"testing"

	"beedance/token"
)

func TestProgramString(t *testing.T) {
	program := &Program{
		Statements: []Statement{
			&VarBlockDeclaration{
				Token: token.Token{Type: token.VAR, Literal: "VAR"},
				Declarations: []*VarDeclStatement{
					{
						Token:    token.Token{Type: token.IDENT, Literal: "myVar"},
						Name:     &Identifier{Token: token.Token{Type: token.IDENT, Literal: "myVar"}, Value: "myVar"},
						DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}},
						Value:    &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "5"}, Value: 5},
					},
				},
			},
		},
	}
	expected := "VAR\n\tmyVar : INT := 5;\nEND_VAR"
	if program.String() != expected {
		t.Errorf("program.String() wrong.\nwant=%q\ngot=%q", expected, program.String())
	}
}

func TestStringMethods(t *testing.T) {
	tests := []struct {
		node     Node
		expected string
	}{
		// Literals
		{&Identifier{Token: token.Token{Type: token.IDENT, Literal: "foobar"}, Value: "foobar"}, "foobar"},
		{&IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "5"}, Value: 5}, "5"},
		{&RealLiteral{Token: token.Token{Type: token.REAL, Literal: "3.14"}, Value: 3.14}, "3.14"},
		{&Boolean{Token: token.Token{Type: token.TRUE, Literal: "TRUE"}, Value: true}, "TRUE"},
		{&StringLiteral{Token: token.Token{Type: token.STRING_LITERAL, Literal: `"hello"`}, Value: "hello"}, `"hello"`},
		{&TimeLiteral{Token: token.Token{Type: token.TIME, Literal: "T#5s"}, Value: "T#5s"}, "T#5s"},
		{&DateLiteral{Token: token.Token{Type: token.DATE, Literal: "D#2026-01-01"}, Value: "D#2026-01-01"}, "D#2026-01-01"},
		{&TimeOfDayLiteral{Token: token.Token{Type: token.TIME_OF_DAY, Literal: "TOD#12:30:00"}, Value: "TOD#12:30:00"}, "TOD#12:30:00"},
		{&DateAndTimeLiteral{Token: token.Token{Type: token.DATE_AND_TIME, Literal: "DT#2026-01-01-12:30:00"}, Value: "DT#2026-01-01-12:30:00"}, "DT#2026-01-01-12:30:00"},
		{&LRealLiteral{Token: token.Token{Type: token.LREAL, Literal: "LREAL#1.23"}, Value: 1.23}, "LREAL#1.23"},
		{&WStringLiteral{Token: token.Token{Type: token.WSTRING_LITERAL, Literal: `"wide"`}, Value: "wide"}, `"wide"`},
		{&BitStringLiteral{Token: token.Token{Type: token.BYTE, Literal: "BYTE#16#FF"}, Value: 255, Width: 8}, "BYTE#16#FF"},
		{&UnsignedIntegerLiteral{Token: token.Token{Type: token.UINT, Literal: "UINT#65535"}, Value: 65535}, "UINT#65535"},
		{&EnumeratedValueLiteral{TypeName: &Identifier{Value: "MyEnum"}, Value: &Identifier{Value: "EnumValue"}}, "MyEnum#EnumValue"},
		{&TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}, "INT"},
		{&DirectVariable{Token: token.Token{Type: token.DIRECT_VAR, Literal: "%IX0.0"}, Address: "IX0.0"}, "%IX0.0"},
		{&AtDeclaration{Location: &DirectVariable{Address: "QX1.2"}}, "AT %QX1.2"},

		// Expressions
		{&PrefixExpression{Token: token.Token{Type: token.MINUS, Literal: "-"}, Operator: "-", Right: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "5"}, Value: 5}}, "(-5)"},
		{&InfixExpression{Left: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "5"}, Value: 5}, Operator: "+", Right: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "10"}, Value: 10}}, "(5 + 10)"},
		{&MemberAccessExpression{Struct: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "myStruct"}, Value: "myStruct"}, Member: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "field"}, Value: "field"}}, "(myStruct.field)"},
		{&CallExpression{Function: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "MyFunc"}, Value: "MyFunc"}, Arguments: []Expression{&IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "1"}, Value: 1}, &NamedArgument{Name: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "In1"}, Value: "In1"}, Value: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "2"}, Value: 2}}}}, "MyFunc(1, In1 := 2)"},
		{&NamedArgument{Name: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "Param"}, Value: "Param"}, Value: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "10"}, Value: 10}}, "Param := 10"},
		{&OutputArgument{Source: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "Out"}, Value: "Out"}, Target: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "Result"}, Value: "Result"}}, "Out => Result"},
		{&ArrayLiteral{Elements: []Expression{&IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "1"}, Value: 1}, &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "2"}, Value: 2}}}, "[1, 2]"},
		{&IndexExpression{Left: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "myArr"}, Value: "myArr"}, Index: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "0"}, Value: 0}}, "(myArr[0])"},
		{&HashLiteral{Pairs: map[Expression]Expression{&StringLiteral{Token: token.Token{Type: token.STRING_LITERAL, Literal: `"key"`}, Value: "key"}: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "1"}, Value: 1}}}, `{"key": 1}`},
		{&FunctionLiteral{Token: token.Token{Type: token.FUNCTION, Literal: "FUNCTION"}, Parameters: []*Identifier{{Value: "x"}}, Body: &BlockStatement{}}, "FUNCTION(x) "},
		{&MacroLiteral{Token: token.Token{Type: token.MACRO, Literal: "macro"}, Parameters: []*Identifier{{Token: token.Token{Type: token.IDENT, Literal: "x"}, Value: "x"}}, Body: &BlockStatement{Statements: []Statement{}}}, "macro(x) "}, // Corrected in previous turn

		// Statements
		{&AssignmentStatement{Token: token.Token{Type: token.IDENT, Literal: "x"}, Left: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "x"}, Value: "x"}, Value: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "10"}, Value: 10}}, "x := 10;"},
		{&ReturnStatement{Token: token.Token{Type: token.RETURN, Literal: "RETURN"}, ReturnValue: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "5"}, Value: 5}}, "RETURN 5;"},
		{&ExitStatement{Token: token.Token{Type: token.EXIT, Literal: "EXIT"}}, "EXIT;"}, // Corrected in previous turn
		{&BlockStatement{Statements: []Statement{&ExpressionStatement{Token: token.Token{Type: token.IDENT, Literal: "x"}, Expression: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "x"}, Value: "x"}}}}, "x;"}, // Corrected in previous turn
		{&IfStatement{Token: token.Token{Type: token.IF, Literal: "IF"}, Condition: &Boolean{Token: token.Token{Type: token.TRUE, Literal: "TRUE"}, Value: true}, Consequence: &BlockStatement{Statements: []Statement{}}}, "IF TRUE THEN \n\t\nEND_IF"},
		{&IfStatement{Token: token.Token{Type: token.IF, Literal: "IF"}, Condition: &Boolean{Token: token.Token{Type: token.TRUE, Literal: "TRUE"}, Value: true}, Consequence: &BlockStatement{Statements: []Statement{}}, Alternative: &BlockStatement{Token: token.Token{Type: token.ELSE, Literal: "ELSE"}, Statements: []Statement{}}}, "IF TRUE THEN \n\t\nELSE\n\t\nEND_IF"},
		{&IfStatement{Token: token.Token{Type: token.IF, Literal: "IF"}, Condition: &Boolean{Token: token.Token{Type: token.TRUE, Literal: "TRUE"}, Value: true}, Consequence: &BlockStatement{Statements: []Statement{}}, Alternative: &IfStatement{Token: token.Token{Type: token.ELSIF, Literal: "ELSIF"}, Condition: &Boolean{Token: token.Token{Type: token.FALSE, Literal: "FALSE"}, Value: false}, Consequence: &BlockStatement{Statements: []Statement{}}, Alternative: &BlockStatement{Token: token.Token{Type: token.ELSE, Literal: "ELSE"}, Statements: []Statement{}}}}, "IF TRUE THEN \n\t\n ELSIF FALSE THEN \n\t\nELSE\n\t\nEND_IF"}, // Corrected in previous turn
		{&ForLoopStatement{Token: token.Token{Type: token.FOR, Literal: "FOR"}, ControlVar: &AssignmentStatement{Left: &Identifier{Value: "i"}, Value: &IntegerLiteral{Value: 1}}, EndValue: &IntegerLiteral{Value: 10}, Body: &BlockStatement{}}, "FOR i := 1; TO 10 DO  END_FOR"},
		{&WhileStatement{Token: token.Token{Type: token.WHILE, Literal: "WHILE"}, Condition: &Boolean{Token: token.Token{Type: token.TRUE, Literal: "TRUE"}, Value: true}, Body: &BlockStatement{Statements: []Statement{}}}, "WHILE TRUE DO  END_WHILE"},         // Corrected in previous turn
		{&RepeatStatement{Token: token.Token{Type: token.REPEAT, Literal: "REPEAT"}, Body: &BlockStatement{Statements: []Statement{}}, Condition: &Boolean{Token: token.Token{Type: token.TRUE, Literal: "TRUE"}, Value: true}}, "REPEAT  UNTIL TRUE END_REPEAT"}, // Corrected in previous turn
		{&CaseStatement{Expression: &Identifier{Value: "myVar"}}, "CASE myVar"}, // Simplified
		{&ActionStatement{Name: &Identifier{Value: "MyAction"}, Body: &BlockStatement{Statements: []Statement{}}}, "ACTION MyAction\n\nEND_ACTION"},
		{&StepStatement{Name: &Identifier{Value: "MyStep"}}, "STEP MyStep"}, // Simplified
		{&TransitionStatement{}, "TRANSITION"},                              // Simplified
		{&IlInstructionStatement{Label: &Identifier{Value: "Loop"}, Operator: "LD", Operand: &Identifier{Value: "myVar"}}, "Loop: LD myVar"},
		{&ActionBlockStatement{ActionName: &Identifier{Value: "Action1"}, Qualifier: &Identifier{Value: "N"}}, "Action1(N);"},

		// Declarations
		{&VarDeclStatement{Token: token.Token{Type: token.IDENT, Literal: "myVar"}, Name: &Identifier{Value: "myVar"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}, Value: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "5"}, Value: 5}}, "myVar : INT := 5;"},
		{&VarDeclStatement{Token: token.Token{Type: token.VAR, Literal: "VAR"}, Name: &Identifier{Value: "myVar"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}}, "VAR myVar : INT;"},
		{&VarDeclStatement{Token: token.Token{Type: token.IDENT, Literal: "myVar"}, Name: &Identifier{Value: "myVar"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.BOOL, Literal: "BOOL"}}, Location: &AtDeclaration{Location: &DirectVariable{Address: "IX0.0"}}}, "myVar AT %IX0.0 : BOOL;"},
		{&VarBlockDeclaration{Declarations: []*VarDeclStatement{{Name: &Identifier{Value: "x"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}}}}, "VAR\n\tx : INT;\nEND_VAR"},
		{&TypeDeclaration{Name: &Identifier{Value: "MyType"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}}, "MyType : INT;"},
		{&TypeBlockDeclaration{Declarations: []*TypeDeclaration{{Name: &Identifier{Value: "MyType"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}}}}, "TYPE\n\tMyType : INT;\nEND_TYPE"},
		{&StructDefinition{Members: []*VarDeclStatement{{Name: &Identifier{Value: "Field1"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}}}}, "STRUCT\n\tField1 : INT;\nEND_STRUCT"},
		{&EnumDefinition{Values: []*Identifier{{Value: "RED"}, {Value: "GREEN"}}}, "(RED, GREEN)"},
		{&ArrayDefinition{Token: token.Token{Type: token.ARRAY, Literal: "ARRAY"}, Ranges: []Expression{&InfixExpression{Left: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "1"}, Value: 1}, Operator: "..", Right: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "10"}, Value: 10}}}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}}, "ARRAY [1 .. 10] OF INT"},
		{&FunctionDeclaration{Name: &Identifier{Value: "MyFunc"}, ReturnType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}, Body: &BlockStatement{Statements: []Statement{}}}, "FUNCTION MyFunc : INT\n\nEND_FUNCTION"},
		{&FunctionBlockDeclaration{Name: &Identifier{Value: "MyFB"}, Body: &BlockStatement{Statements: []Statement{}}}, "FUNCTION_BLOCK MyFB\n\nEND_FUNCTION_BLOCK"},
		{&ProgramDeclaration{Name: &Identifier{Value: "MyProg"}, Body: &BlockStatement{Statements: []Statement{}}}, "PROGRAM MyProg\n\nEND_PROGRAM"},
		{&ExternalVarDeclaration{Vars: []*VarDeclStatement{{Name: &Identifier{Value: "ExtVar"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}}}}, "VAR_EXTERNAL\n\tExtVar : INT;\nEND_VAR"},
		{&ConfigVarDeclaration{Vars: []*VarDeclStatement{{Name: &Identifier{Value: "CfgVar"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.BOOL, Literal: "BOOL"}}}}}, "VAR_CONFIG\n\tCfgVar : BOOL;\nEND_VAR"},
		{&TempVarDeclaration{Vars: []*VarDeclStatement{{Name: &Identifier{Value: "TmpVar"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.REAL, Literal: "REAL"}}}}}, "VAR_TEMP\n\tTmpVar : REAL;\nEND_VAR"},
		{&AccessVarDeclaration{Vars: []*VarDeclStatement{{Name: &Identifier{Value: "AccVar"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.IDENT, Literal: "MyFB"}}}}}, "VAR_ACCESS\n\tAccVar : MyFB;\nEND_VAR"},
		{&GlobalVarDeclaration{Vars: []*VarDeclStatement{{Name: &Identifier{Value: "GlbVar"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}}}}, "VAR_GLOBAL\n\tGlbVar : INT;\nEND_VAR"},
		{&ConfigurationDeclaration{Token: token.Token{Type: token.CONFIGURATION, Literal: "CONFIGURATION"}, Name: &Identifier{Value: "MyConfig"}}, "CONFIGURATION MyConfig\nEND_CONFIGURATION"},                  // Simplified
		{&ResourceDeclaration{Token: token.Token{Type: token.RESOURCE, Literal: "RESOURCE"}, Name: &Identifier{Value: "Res1"}, ResourceType: &Identifier{Value: "PLC1"}}, "RESOURCE Res1 ON PLC1\nEND_RESOURCE"}, // Simplified
		{&TaskDeclaration{Token: token.Token{Type: token.TASK, Literal: "TASK"}, Name: &Identifier{Value: "Task1"}, Priority: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "1"}, Value: 1}}, "TASK Task1(PRIORITY := 1)"},
		{&ProgramConfiguration{InstanceName: &Identifier{Value: "ProgInst"}, TypeName: &Identifier{Value: "ProgType"}}, "PROGRAM ProgInst : ProgType;"},
		{&SFCProgram{Elements: []Statement{&StepStatement{Name: &Identifier{Value: "Step1"}}}}, "STEP Step1"}, // Simplified
	}

	for _, tt := range tests {
		t.Run(tt.node.String(), func(t *testing.T) {
			actual := tt.node.String()
			if actual != tt.expected {
				t.Errorf("node.String() wrong.\nwant=%q\ngot=%q", tt.expected, actual)
			}
		})
	}
}
