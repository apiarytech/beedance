package ast

import (
	"github.com/apiarytech/beedance/token"
	"reflect"
	"testing"
)

func TestProgramString(t *testing.T) {
	program := &Program{
		Statements: []Statement{
			&VarBlockDeclaration{
				Token: token.Token{Type: token.VAR, Literal: "VAR"},
				Declarations: []*VarDeclStatement{{
					Token:    token.Token{Type: token.IDENT, Literal: "myVar"},
					Name:     &Identifier{Token: token.Token{Type: token.IDENT, Literal: "myVar"}, Value: "myVar"},
					DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}},
					Value: &IntegerLiteral{
						Token: token.Token{Type: token.INT, Literal: "5"}, Value: 5,
					}},
				}},
		},
	}
	expected := "VAR\n\tmyVar : INT := 5;\nEND_VAR"
	if program.String() != expected {
		t.Errorf("program.String() wrong.\nwant=%q\ngot=%q", expected, program.String())
	}
}

func TestMultiStatementProgramString(t *testing.T) {
	program := &Program{
		Statements: []Statement{
			&VarDeclStatement{
				Name:     &Identifier{Value: "a"},
				DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}},
			},
			&AssignmentStatement{
				Left:  &Identifier{Value: "a"},
				Value: &IntegerLiteral{Value: 1},
			},
		},
	}
	expected := "a : INT;a := 1;"
	if program.String() != expected {
		t.Errorf("program.String() wrong.\nwant=%q\ngot=%q", expected, program.String())
	}
}

func TestEmptyProgram(t *testing.T) {
	program := &Program{
		Statements: []Statement{},
	}

	if program.String() != "" {
		t.Errorf("program.String() for empty program should be empty. got=%q", program.String())
	}
	if program.TokenLiteral() != "" {
		t.Errorf("program.TokenLiteral() for empty program should be empty. got=%q", program.TokenLiteral())
	}
	line, col := program.Pos()
	if line != 0 || col != 0 {
		t.Errorf("program.Pos() for empty program should be (0,0). got=(%d,%d)", line, col)
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
		{&IntegerLiteral{Value: 123}, "123"},
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
		{&BitStringLiteral{Value: 0xABCD, Width: 16}, "WORD#16#ABCD"},
		{&BitStringLiteral{Value: 0x12345678, Width: 32}, "DWORD#16#12345678"},
		{&BitStringLiteral{Value: 0x1122334455667788, Width: 64}, "LWORD#16#1122334455667788"},
		{&BitStringLiteral{Value: 0xFFF, Width: 12}, "BITSTRING#12#FFF"},
		{&ThisExpression{}, "THIS"},
		{&SuperExpression{}, "SUPER"},
		{&DereferenceExpression{
			Pointer: &Identifier{Value: "MyRef"},
		}, "(MyRef^)"},
		{&ReferenceType{
			BaseType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}},
		}, "REFERENCE TO INT"},
		{&UnsignedIntegerLiteral{Token: token.Token{Type: token.UINT, Literal: "UINT#65535"}, Value: 65535}, "UINT#65535"},
		{&EnumeratedValueLiteral{TypeName: &Identifier{Value: "MyEnum"}, Value: &Identifier{Value: "EnumValue"}}, "MyEnum#EnumValue"},
		{&FunctionParameter{Name: &Identifier{Value: "p"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}}, "p : INT"},
		{&TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}, "INT"},
		{&DirectVariable{Token: token.Token{Type: token.DIRECT_VAR, Literal: "%IX0.0"}, Address: "IX0.0"}, "%IX0.0"},
		{&AtDeclaration{Location: &DirectVariable{Address: "QX1.2"}}, "AT %QX1.2"},

		// Expressions
		{&PrefixExpression{Token: token.Token{Type: token.MINUS, Literal: "-"}, Operator: "-", Right: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "5"}, Value: 5}}, "(-5)"},
		{&InfixExpression{Left: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "5"}, Value: 5}, Operator: "+", Right: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "10"}, Value: 10}}, "(5 + 10)"},
		{&MemberAccessExpression{Struct: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "myStruct"}, Value: "myStruct"}, Member: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "field"}, Value: "field"}}, "myStruct.field"},
		{&CallExpression{Function: &Identifier{Value: "myFunc"}}, "myFunc()"},
		{&CallExpression{Function: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "MyFunc"}, Value: "MyFunc"}, Arguments: []Expression{&IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "1"}, Value: 1}, &NamedArgument{Name: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "In1"}, Value: "In1"}, Value: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "2"}, Value: 2}}}}, "MyFunc(1, In1 := 2)"},
		{&NamedArgument{Name: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "Param"}, Value: "Param"}, Value: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "10"}, Value: 10}}, "Param := 10"},
		{&OutputArgument{Source: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "Out"}, Value: "Out"}, Target: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "Result"}, Value: "Result"}}, "Out => Result"},
		{&OutputArgument{Source: &Identifier{Value: "Out"}}, "Out => "},
		{&OutputArgument{Target: &Identifier{Value: "Res"}}, " => Res"},
		{&ArrayLiteral{Elements: []Expression{&IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "1"}, Value: 1}, &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "2"}, Value: 2}}}, "[1, 2]"},
		{&IndexExpression{Left: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "myArr"}, Value: "myArr"}, Index: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "0"}, Value: 0}}, "(myArr[0])"},
		{&HashLiteral{Pairs: map[Expression]Expression{&StringLiteral{Token: token.Token{Type: token.STRING_LITERAL, Literal: `"key"`}, Value: "key"}: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "1"}, Value: 1}}}, `{"key": 1}`},
		{&HashLiteral{
			Pairs: map[Expression]Expression{
				&StringLiteral{Value: "zulu", Token: token.Token{Literal: `"zulu"`}}:   &IntegerLiteral{Value: 3},
				&StringLiteral{Value: "alpha", Token: token.Token{Literal: `"alpha"`}}: &IntegerLiteral{Value: 1},
				&StringLiteral{Value: "mike", Token: token.Token{Literal: `"mike"`}}:   &IntegerLiteral{Value: 2},
			},
		}, `{"alpha": 1, "mike": 2, "zulu": 3}`},
		{&FunctionLiteral{
			Token: token.Token{Type: token.FUNCTION, Literal: "FUNCTION"},
			Parameters: []*FunctionParameter{
				{Name: &Identifier{Value: "x"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}},
			},
			ReturnType: &TypeSpecifier{Token: token.Token{Type: token.BOOL, Literal: "BOOL"}},
			Body:       &BlockStatement{},
		},
			"FUNCTION(x : INT) : BOOL "}, // cspell:disable-line
		{&FunctionLiteral{Token: token.Token{Type: token.IDENT, Literal: "fn"}, Name: "myFn", Body: &BlockStatement{}}, "fn<myFn>() "},
		{&MacroLiteral{Token: token.Token{Type: token.MACRO, Literal: "macro"}, Parameters: []*Identifier{{Token: token.Token{Type: token.IDENT, Literal: "x"}, Value: "x"}}, Body: &BlockStatement{Statements: []Statement{}}}, "macro(x) "}, // Corrected in previous turn
		{&ArrayRepetition{Factor: &IntegerLiteral{Value: 3}, Elements: []Expression{&IntegerLiteral{Value: 0}}}, "3(0)"},
		{&FbTaskAssociation{FbName: &Identifier{Value: "FB1"}, TaskName: &Identifier{Value: "TaskA"}}, "FB1 WITH TaskA"},

		// Statements
		{&AssignmentStatement{Token: token.Token{Type: token.IDENT, Literal: "x"}, Left: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "x"}, Value: "x"}, Value: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "10"}, Value: 10}}, "x := 10;"},
		{&ReturnStatement{Token: token.Token{Type: token.RETURN, Literal: "RETURN"}, ReturnValue: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "5"}, Value: 5}}, "RETURN 5;"},
		{&ExitStatement{Token: token.Token{Type: token.EXIT, Literal: "EXIT"}}, "EXIT;"}, // Corrected in previous turn
		{&ExpressionStatement{Expression: nil}, ""},
		{&BlockStatement{Statements: []Statement{&ExpressionStatement{Token: token.Token{Type: token.IDENT, Literal: "x"}, Expression: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "x"}, Value: "x"}}}}, "x;"}, // Corrected in previous turn
		{&IfStatement{Token: token.Token{Type: token.IF, Literal: "IF"}, Condition: &Boolean{Token: token.Token{Type: token.TRUE, Literal: "TRUE"}, Value: true}, Consequence: &BlockStatement{Statements: []Statement{}}}, "IF TRUE THEN \n\t\nEND_IF"},
		{&IfStatement{
			Condition:   &Boolean{Token: token.Token{Type: token.TRUE, Literal: "TRUE"}, Value: true},
			Consequence: &BlockStatement{Statements: []Statement{}},
			Alternative: &ReturnStatement{Token: token.Token{Type: token.RETURN, Literal: "RETURN"}}, // Invalid alternative type to hit default
		}, "IF TRUE THEN \n\t\nEND_IF"},
		{&IfStatement{Token: token.Token{Type: token.IF, Literal: "IF"}, Condition: &Boolean{Token: token.Token{Type: token.TRUE, Literal: "TRUE"}, Value: true}, Consequence: &BlockStatement{Statements: []Statement{}}, Alternative: &BlockStatement{Token: token.Token{Type: token.ELSE, Literal: "ELSE"}, Statements: []Statement{}}}, "IF TRUE THEN \n\t\nELSE\n\t\nEND_IF"},
		{&IfStatement{Token: token.Token{Type: token.IF, Literal: "IF"}, Condition: &Boolean{Token: token.Token{Type: token.TRUE, Literal: "TRUE"}, Value: true}, Consequence: &BlockStatement{Statements: []Statement{}}, Alternative: &IfStatement{Token: token.Token{Type: token.ELSIF, Literal: "ELSIF"}, Condition: &Boolean{Token: token.Token{Type: token.FALSE, Literal: "FALSE"}, Value: false}, Consequence: &BlockStatement{Statements: []Statement{}}, Alternative: &BlockStatement{Token: token.Token{Type: token.ELSE, Literal: "ELSE"}, Statements: []Statement{}}}}, "IF TRUE THEN \n\t\n ELSIF FALSE THEN \n\t\nELSE\n\t\nEND_IF"}, // Corrected in previous turn
		{&ForLoopStatement{Token: token.Token{Type: token.FOR, Literal: "FOR"}, ControlVar: &AssignmentStatement{Left: &Identifier{Value: "i"}, Value: &IntegerLiteral{Value: 1}}, EndValue: &IntegerLiteral{Value: 10}, Body: &BlockStatement{}}, "FOR i := 1; TO 10 DO  END_FOR"},
		{&ForLoopStatement{Token: token.Token{Type: token.FOR, Literal: "FOR"}, ControlVar: &AssignmentStatement{Left: &Identifier{Value: "i"}, Value: &IntegerLiteral{Value: 1}}, EndValue: &IntegerLiteral{Value: 10}, StepValue: &IntegerLiteral{Value: 2}, Body: &BlockStatement{}}, "FOR i := 1; TO 10 BY 2 DO  END_FOR"},
		{&WhileStatement{Token: token.Token{Type: token.WHILE, Literal: "WHILE"}, Condition: &Boolean{Token: token.Token{Type: token.TRUE, Literal: "TRUE"}, Value: true}, Body: &BlockStatement{Statements: []Statement{}}}, "WHILE TRUE DO  END_WHILE"},         // Corrected in previous turn
		{&RepeatStatement{Token: token.Token{Type: token.REPEAT, Literal: "REPEAT"}, Body: &BlockStatement{Statements: []Statement{}}, Condition: &Boolean{Token: token.Token{Type: token.TRUE, Literal: "TRUE"}, Value: true}}, "REPEAT  UNTIL TRUE END_REPEAT"}, // Corrected in previous turn
		{&CaseStatement{
			Expression: &Identifier{Value: "myVar"},
			Cases: []*CaseBranch{
				{
					Values:      []Expression{&IntegerLiteral{Value: 1}},
					Consequence: &BlockStatement{Statements: []Statement{&ExpressionStatement{Expression: &Identifier{Value: "doThis"}}}},
				},
			},
			Alternative: &BlockStatement{Statements: []Statement{&ExpressionStatement{Expression: &Identifier{Value: "doDefault"}}}},
		}, "CASE myVar OF\n\t1: doThis;\nELSE\n\tdoDefault;\nEND_CASE"},
		{&ActionStatement{Name: &Identifier{Value: "MyAction"}, Body: &BlockStatement{}}, "ACTION MyAction\n\nEND_ACTION"},
		{&StepStatement{
			Name: &Identifier{Value: "MyStep"},
			Actions: []*ActionBlockStatement{
				{ActionName: &Identifier{Value: "Action1"}, Qualifier: &Identifier{Value: "N"}},
			},
			Body: &BlockStatement{Statements: []Statement{&ExpressionStatement{Expression: &Identifier{Value: "x"}}}},
		}, "STEP MyStep:\n\tAction1(N);\n\nx;\nEND_STEP"},
		{&StepStatement{IsInitial: true, Name: &Identifier{Value: "Init"}}, "INITIAL_STEP Init:\nEND_STEP"},
		{&TransitionStatement{
			From:      []*Identifier{{Value: "S1"}},
			To:        []*Identifier{{Value: "S2"}},
			Condition: &Boolean{Token: token.Token{Type: token.TRUE, Literal: "TRUE"}, Value: true},
		}, "TRANSITION FROM S1 TO S2 := TRUE;\nEND_TRANSITION"},
		{&IlInstructionStatement{Label: &Identifier{Value: "Loop"}, Operator: "LD", Operand: &Identifier{Value: "myVar"}}, "Loop: LD myVar"},
		{&IlInstructionStatement{Operator: "RET"}, "RET"},
		{&ActionBlockStatement{ActionName: &Identifier{Value: "Action1"}, Qualifier: &Identifier{Value: "N"}}, "Action1(N);"},
		{&NamespaceDeclaration{
			Name: &MemberAccessExpression{
				Struct: &Identifier{Value: "MyCompany"},
				Member: &Identifier{Value: "MyLibrary"},
			},
			Statements: []Statement{&FunctionDeclaration{Name: &Identifier{Value: "MyFunc"}}}}, "NAMESPACE MyCompany.MyLibrary\nFUNCTION MyFunc : \n\nEND_FUNCTION\nEND_NAMESPACE"},

		// Declarations
		{&VarDeclStatement{Token: token.Token{Type: token.IDENT, Literal: "myVar"}, Name: &Identifier{Value: "myVar"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}, Value: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "5"}, Value: 5}}, "myVar : INT := 5;"},
		{&VarDeclStatement{Token: token.Token{Type: token.VAR, Literal: "VAR"}, Name: &Identifier{Value: "myVar"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}}, "VAR myVar : INT;"}, // This will fail if the logic in String() is wrong
		{&VarDeclStatement{Token: token.Token{Type: token.IDENT, Literal: "myVar"}, Name: &Identifier{Value: "myVar"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.BOOL, Literal: "BOOL"}}, Location: &AtDeclaration{Location: &DirectVariable{Address: "IX0.0"}}}, "myVar AT %IX0.0 : BOOL;"},
		{&VarDeclStatement{Name: &Identifier{Value: "myAccess"}, AccessPath: &Identifier{Value: "path.to.var"}}, "myAccess : path.to.var;"},
		{&VarBlockDeclaration{Declarations: []*VarDeclStatement{{Name: &Identifier{Value: "x"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}}}}, "VAR\n\tx : INT;\nEND_VAR"},
		{&TypeDeclaration{Name: &Identifier{Value: "MyType"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}}, "MyType : INT;"},
		{&TypeDeclaration{Name: &Identifier{Value: "MySubType"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}, Subrange: &InfixExpression{Left: &IntegerLiteral{Value: 0}, Operator: "..", Right: &IntegerLiteral{Value: 100}}}, "MySubType : INT (0 .. 100);"},
		{&TypeDeclaration{Name: &Identifier{Value: "MyInitType"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}, InitialValue: &IntegerLiteral{Value: 10}}, "MyInitType : INT := 10;"},
		{&TypeBlockDeclaration{Declarations: []*TypeDeclaration{{Name: &Identifier{Value: "MyType"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}}}}, "TYPE\n\tMyType : INT;\nEND_TYPE"},
		{&StructDefinition{Members: []*VarDeclStatement{{Name: &Identifier{Value: "Field1"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}}}}, "STRUCT\n\tField1 : INT;\nEND_STRUCT"},
		{&EnumDefinition{Values: []*Identifier{{Value: "RED"}, {Value: "GREEN"}}}, "(RED, GREEN)"},
		{&ArrayDefinition{Token: token.Token{Type: token.ARRAY, Literal: "ARRAY"}, Ranges: []Expression{&InfixExpression{Left: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "1"}, Value: 1}, Operator: "..", Right: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "10"}, Value: 10}}}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}}, "ARRAY [1 .. 10] OF INT"},
		{&ArrayDefinition{
			Token:    token.Token{Type: token.ARRAY, Literal: "ARRAY"},
			Ranges:   []Expression{&Identifier{Value: "MyRangeVar"}}, // This will trigger the 'else'
			DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}},
		}, "ARRAY [MyRangeVar] OF INT"},
		{&ArrayDefinition{Token: token.Token{Type: token.ARRAY, Literal: "ARRAY"}, Ranges: []Expression{&InfixExpression{Left: &IntegerLiteral{Value: 1}, Operator: "..", Right: &IntegerLiteral{Value: 10}}, &InfixExpression{Left: &IntegerLiteral{Value: 1}, Operator: "..", Right: &IntegerLiteral{Value: 20}}}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}}, "ARRAY [1 .. 10, 1 .. 20] OF INT"},
		{&FunctionDeclaration{Name: &Identifier{Value: "MyFunc"}, ReturnType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}, Body: &BlockStatement{Statements: []Statement{}}}, "FUNCTION MyFunc : INT\n\nEND_FUNCTION"},
		{&FunctionDeclaration{Name: &Identifier{Value: "MyFunc"}}, "FUNCTION MyFunc : \n\nEND_FUNCTION"},
		{&FunctionBlockDeclaration{Name: &Identifier{Value: "MyFB"}, Body: &BlockStatement{Statements: []Statement{}}}, "FUNCTION_BLOCK MyFB\n\nEND_FUNCTION_BLOCK"},
		{&ProgramDeclaration{Name: &Identifier{Value: "MyProg"}, Body: &BlockStatement{Statements: []Statement{}}}, "PROGRAM MyProg\n\nEND_PROGRAM"},
		{&ProgramDeclaration{Name: &Identifier{Value: "MyProg"}}, "PROGRAM MyProg\n\nEND_PROGRAM"},
		{&ExternalVarDeclaration{Vars: []*VarDeclStatement{{Name: &Identifier{Value: "ExtVar"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}}}}, "VAR_EXTERNAL\n\tExtVar : INT;\nEND_VAR"},
		{&ConfigVarDeclaration{ProgramInstanceName: &Identifier{Value: "MyProg"}, Declarations: []*VarDeclStatement{{Name: &Identifier{Value: "CfgVar"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.BOOL, Literal: "BOOL"}}}}}, "VAR_CONFIG MyProg\n\tCfgVar : BOOL;\nEND_VAR"}, // This will fail if the logic in String() is wrong
		{&TempVarDeclaration{Vars: []*VarDeclStatement{{Name: &Identifier{Value: "TmpVar"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.REAL, Literal: "REAL"}}}}}, "VAR_TEMP\n\tTmpVar : REAL;\nEND_VAR"},
		{&AccessVarDeclaration{Vars: []*VarDeclStatement{{Name: &Identifier{Value: "AccVar"}, AccessPath: &Identifier{Value: "MyFB"}}}}, "VAR_ACCESS\n\tAccVar : MyFB;\nEND_VAR"},
		{&GlobalVarDeclaration{Vars: []*VarDeclStatement{{Name: &Identifier{Value: "GlbVar"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}}}}, "VAR_GLOBAL\n\tGlbVar : INT;\nEND_VAR"},
		{&ConfigurationDeclaration{
			Token: token.Token{Type: token.CONFIGURATION, Literal: "CONFIGURATION"},
			Name:  &Identifier{Value: "MyConfig"},
			GlobalVars: []*GlobalVarDeclaration{
				{Vars: []*VarDeclStatement{{Name: &Identifier{Value: "g"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.BOOL, Literal: "BOOL"}}}}}},
		}, "CONFIGURATION MyConfig\nVAR_GLOBAL\n\tg : BOOL;\nEND_VAR\nEND_CONFIGURATION"},
		{&ConfigurationDeclaration{
			Name: &Identifier{Value: "FullConfig"},
			Resources: []*ResourceDeclaration{
				{
					Name:         &Identifier{Value: "CPU1"},
					ResourceType: &Identifier{Value: "PLC"},
				},
			},
			AccessVars: []*AccessVarDeclaration{
				{
					Vars: []*VarDeclStatement{
						{
							Name:       &Identifier{Value: "RemoteVar"},
							AccessPath: &Identifier{Value: "OtherProgram.Var"},
						},
					},
				},
			},
			VarConfigs: []*ConfigVarDeclaration{
				{
					ProgramInstanceName: &Identifier{Value: "MyProgInstance"},
					Declarations: []*VarDeclStatement{
						{
							Name:     &Identifier{Value: "ConfigParam"},
							DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}},
							Value:    &IntegerLiteral{Value: 100},
						},
					},
				},
			},
		}, "CONFIGURATION FullConfig\nRESOURCE CPU1 ON PLC\nEND_RESOURCE\nVAR_ACCESS\n\tRemoteVar : OtherProgram.Var;\nEND_VAR\nVAR_CONFIG MyProgInstance\n\tConfigParam : INT := 100;\nEND_VAR\nEND_CONFIGURATION"},
		{&ResourceDeclaration{
			Name:         &Identifier{Value: "Res1"},
			ResourceType: &Identifier{Value: "PLC1"},
			Tasks: []*TaskDeclaration{
				{Name: &Identifier{Value: "Task1"}, Priority: &IntegerLiteral{Value: 1}},
			},
			Programs: []*ProgramConfiguration{
				{InstanceName: &Identifier{Value: "Prog1"}, TypeName: &Identifier{Value: "ProgType"}},
			},
		}, "RESOURCE Res1 ON PLC1\nTASK Task1(PRIORITY := 1)\nPROGRAM Prog1 : ProgType;\nEND_RESOURCE"},
		{&ResourceDeclaration{
			Name:         &Identifier{Value: "MyRes"},
			ResourceType: &Identifier{Value: "MyPLC"},
			GlobalVars: []*GlobalVarDeclaration{
				{
					Vars: []*VarDeclStatement{
						{
							Name:     &Identifier{Value: "g_var"},
							DataType: &TypeSpecifier{Token: token.Token{Type: token.BOOL, Literal: "BOOL"}},
						},
					},
				},
			},
		}, "RESOURCE MyRes ON MyPLC\nVAR_GLOBAL\n\tg_var : BOOL;\nEND_VAR\nEND_RESOURCE"},
		{&TaskDeclaration{
			Name:     &Identifier{Value: "Task1"},
			Interval: &TimeLiteral{Value: "T#100ms"},
			Priority: &IntegerLiteral{Value: 1},
		}, "TASK Task1(INTERVAL := T#100ms, PRIORITY := 1)"},
		{&TaskDeclaration{
			Name:     &Identifier{Value: "SingleTask"},
			Single:   &Boolean{Value: true, Token: token.Token{Literal: "TRUE"}},
			Priority: &IntegerLiteral{Value: 0},
		}, "TASK SingleTask(SINGLE := TRUE, PRIORITY := 0)"},
		{&ProgramConfiguration{InstanceName: &Identifier{Value: "ProgInst"}, TypeName: &Identifier{Value: "ProgType"}}, "PROGRAM ProgInst : ProgType;"},
		{&ProgramConfiguration{
			InstanceName: &Identifier{Value: "ProgInstWithTask"},
			TaskName:     &Identifier{Value: "MyTask"},
			TypeName:     &Identifier{Value: "ProgType"},
		}, "PROGRAM ProgInstWithTask WITH MyTask : ProgType;"},
		{&ProgramConfiguration{
			InstanceName: &Identifier{Value: "ProgWithParams"},
			TypeName:     &Identifier{Value: "ProgType"},
			Parameters:   []Expression{&NamedArgument{Name: &Identifier{Value: "In"}, Value: &Boolean{Value: true, Token: token.Token{Literal: "TRUE"}}}},
			FbTasks:      []*FbTaskAssociation{{FbName: &Identifier{Value: "FB1"}, TaskName: &Identifier{Value: "TaskA"}}},
		}, "PROGRAM ProgWithParams : ProgType(FB1 WITH TaskA, In := TRUE);"},
		{&SFCProgram{Elements: []Statement{
			&StepStatement{Name: &Identifier{Value: "Step1"}},
			&TransitionStatement{From: []*Identifier{{Value: "Step1"}}, To: []*Identifier{{Value: "Step2"}}, Condition: &Identifier{Value: "Cond1"}},
		}}, "STEP Step1:\nEND_STEP\nTRANSITION FROM Step1 TO Step2 := Cond1;\nEND_TRANSITION\n"},
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

func TestGetMemberType(t *testing.T) {
	structDef := &StructDefinition{
		Members: []*VarDeclStatement{
			{Name: &Identifier{Value: "Field1"}, DataType: &TypeSpecifier{Token: token.Token{Type: token.INT, Literal: "INT"}}},
			{Name: &Identifier{Value: "Field2"}, DataType: &ArrayDefinition{
				DataType: &TypeSpecifier{Token: token.Token{Type: token.BOOL, Literal: "BOOL"}},
			}},
		},
	}

	tests := []struct {
		memberName string
		expected   string
	}{
		{"Field1", "INT"},
		{"Field2", "BOOL"},
		{"NonExistent", ""},
	}

	for _, tt := range tests {
		actual := structDef.GetMemberType(tt.memberName)
		if actual != tt.expected {
			t.Errorf("GetMemberType(%q) wrong. want=%q, got=%q", tt.memberName, tt.expected, actual)
		}
	}
}

func TestTypedLiteral(t *testing.T) {
	tl := &TypedLiteral{
		Token:    token.Token{Type: token.INT, Literal: "INT", Row: 1, Column: 1},
		TypeName: "INT",
		Value:    &IntegerLiteral{Value: 123},
	}

	// Test Pos()
	row, col := tl.Pos()
	if row != 1 || col != 1 {
		t.Errorf("Pos() wrong. want=(1,1), got=(%d,%d)", row, col)
	}

	// Test TokenLiteral()
	if tl.TokenLiteral() != "INT" {
		t.Errorf("TokenLiteral() wrong. want=%q, got=%q", "INT", tl.TokenLiteral())
	}

	// Test String()
	expectedString := "INT#123"
	if tl.String() != expectedString {
		t.Errorf("String() wrong. want=%q, got=%q", expectedString, tl.String())
	}
}

func TestAstHelpers(t *testing.T) {
	testToken := func(lit string) token.Token {
		return token.Token{Row: 10, Column: 5, Literal: lit}
	}
	testComments := []string{"// a comment"}

	tests := []struct {
		name             string
		node             Node
		expectedLine     int
		expectedCol      int
		expectedLiteral  string
		expectedComments []string
	}{
		{
			name:             "VarDeclStatement",
			node:             &VarDeclStatement{Token: testToken("VAR"), LeadingComments: testComments},
			expectedLine:     10,
			expectedCol:      5,
			expectedLiteral:  "VAR",
			expectedComments: testComments,
		},
		{
			name:             "IfStatement",
			node:             &IfStatement{Token: testToken("IF"), LeadingComments: testComments},
			expectedLine:     10,
			expectedCol:      5,
			expectedLiteral:  "IF",
			expectedComments: testComments,
		},
		{
			name:             "FunctionDeclaration",
			node:             &FunctionDeclaration{Token: testToken("FUNCTION"), LeadingComments: testComments},
			expectedLine:     10,
			expectedCol:      5,
			expectedLiteral:  "FUNCTION",
			expectedComments: testComments,
		},
		{
			name:             "ProgramDeclaration",
			node:             &ProgramDeclaration{Token: testToken("PROGRAM"), LeadingComments: testComments},
			expectedLine:     10,
			expectedCol:      5,
			expectedLiteral:  "PROGRAM",
			expectedComments: testComments,
		},
		{
			name:             "FunctionBlockDeclaration",
			node:             &FunctionBlockDeclaration{Token: testToken("FUNCTION_BLOCK"), LeadingComments: testComments},
			expectedLine:     10,
			expectedCol:      5,
			expectedLiteral:  "FUNCTION_BLOCK",
			expectedComments: testComments,
		},
		{
			name:             "TypeBlockDeclaration",
			node:             &TypeBlockDeclaration{Token: testToken("TYPE"), LeadingComments: testComments},
			expectedLine:     10,
			expectedCol:      5,
			expectedLiteral:  "TYPE",
			expectedComments: testComments,
		},
		{
			name:             "VarBlockDeclaration",
			node:             &VarBlockDeclaration{Token: testToken("VAR"), LeadingComments: testComments},
			expectedLine:     10,
			expectedCol:      5,
			expectedLiteral:  "VAR",
			expectedComments: testComments,
		},
		{
			name:             "AssignmentStatement",
			node:             &AssignmentStatement{Token: testToken(":="), LeadingComments: testComments},
			expectedLine:     10,
			expectedCol:      5,
			expectedLiteral:  ":=",
			expectedComments: testComments,
		},
		{
			name:             "ExpressionStatement",
			node:             &ExpressionStatement{Token: testToken("myFunc"), LeadingComments: testComments},
			expectedLine:     10,
			expectedCol:      5,
			expectedLiteral:  "myFunc",
			expectedComments: testComments,
		},
		{
			name:             "ForLoopStatement",
			node:             &ForLoopStatement{Token: testToken("FOR"), LeadingComments: testComments},
			expectedLine:     10,
			expectedCol:      5,
			expectedLiteral:  "FOR",
			expectedComments: testComments,
		},
		{
			name:             "WhileStatement",
			node:             &WhileStatement{Token: testToken("WHILE"), LeadingComments: testComments},
			expectedLine:     10,
			expectedCol:      5,
			expectedLiteral:  "WHILE",
			expectedComments: testComments,
		},
		{
			name:             "RepeatStatement",
			node:             &RepeatStatement{Token: testToken("REPEAT"), LeadingComments: testComments},
			expectedLine:     10,
			expectedCol:      5,
			expectedLiteral:  "REPEAT",
			expectedComments: testComments,
		},
		{
			name:             "CaseStatement",
			node:             &CaseStatement{Token: testToken("CASE"), LeadingComments: testComments},
			expectedLine:     10,
			expectedCol:      5,
			expectedLiteral:  "CASE",
			expectedComments: testComments,
		},
		{
			name:             "TypeDeclaration",
			node:             &TypeDeclaration{Token: testToken("MyType"), LeadingComments: testComments},
			expectedLine:     10,
			expectedCol:      5,
			expectedLiteral:  "MyType",
			expectedComments: testComments,
		},
		{
			name:            "SFCProgram - with elements",
			node:            &SFCProgram{Elements: []Statement{&StepStatement{Token: testToken("STEP")}}},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "STEP",
		},
		{
			name:            "SFCProgram - no elements",
			node:            &SFCProgram{Token: testToken("SFC")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "SFC",
		},
		{
			name: "Program - with statements",
			node: &Program{
				Statements: []Statement{
					&VarDeclStatement{Token: testToken("VAR")},
				},
			},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "VAR",
		},
		// --- Additional Declarations ---
		{
			name:             "ConfigurationDeclaration",
			node:             &ConfigurationDeclaration{Token: testToken("CONFIGURATION"), LeadingComments: testComments},
			expectedLine:     10,
			expectedCol:      5,
			expectedLiteral:  "CONFIGURATION",
			expectedComments: testComments,
		},
		{
			name:            "ResourceDeclaration",
			node:            &ResourceDeclaration{Token: testToken("RESOURCE")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "RESOURCE",
		},
		{
			name:            "TaskDeclaration",
			node:            &TaskDeclaration{Token: testToken("TASK")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "TASK",
		},
		{
			name:            "ProgramConfiguration",
			node:            &ProgramConfiguration{Token: testToken("PROGRAM")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "PROGRAM",
		},
		{
			name:            "ExternalVarDeclaration",
			node:            &ExternalVarDeclaration{Token: testToken("VAR_EXTERNAL")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "VAR_EXTERNAL",
		},
		{
			name:            "ConfigVarDeclaration",
			node:            &ConfigVarDeclaration{Token: testToken("VAR_CONFIG")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "VAR_CONFIG",
		},
		{
			name:            "TempVarDeclaration",
			node:            &TempVarDeclaration{Token: testToken("VAR_TEMP")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "VAR_TEMP",
		},
		{
			name:            "AccessVarDeclaration",
			node:            &AccessVarDeclaration{Token: testToken("VAR_ACCESS")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "VAR_ACCESS",
		},
		{
			name:            "GlobalVarDeclaration",
			node:            &GlobalVarDeclaration{Token: testToken("VAR_GLOBAL")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "VAR_GLOBAL",
		},
		{
			name:            "StructDefinition",
			node:            &StructDefinition{Token: testToken("STRUCT")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "STRUCT",
		},
		{
			name:            "EnumDefinition",
			node:            &EnumDefinition{Token: testToken("(")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "(",
		},
		{
			name:            "ArrayDefinition",
			node:            &ArrayDefinition{Token: testToken("ARRAY")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "ARRAY",
		},
		{
			name:            "AtDeclaration",
			node:            &AtDeclaration{Token: testToken("AT")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "AT",
		},
		// --- Additional Statements ---
		{
			name:            "ReturnStatement",
			node:            &ReturnStatement{Token: testToken("RETURN")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "RETURN",
		},
		{
			name:            "ExitStatement",
			node:            &ExitStatement{Token: testToken("EXIT")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "EXIT",
		},
		{
			name:            "BlockStatement",
			node:            &BlockStatement{Token: testToken("{")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "{",
		},
		{
			name:            "ActionStatement",
			node:            &ActionStatement{Token: testToken("ACTION")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "ACTION",
		},
		{
			name:            "IlInstructionStatement",
			node:            &IlInstructionStatement{Token: testToken("LD")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "LD",
		},
		{
			name:            "StepStatement",
			node:            &StepStatement{Token: testToken("STEP")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "STEP",
		},
		{
			name:            "TransitionStatement",
			node:            &TransitionStatement{Token: testToken("TRANSITION")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "TRANSITION",
		},
		{
			name:            "ActionBlockStatement",
			node:            &ActionBlockStatement{Token: testToken("MyAction")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "MyAction",
		},
		{
			name:            "CaseBranch",
			node:            &CaseBranch{Token: testToken("1")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "1",
		},
		// --- Expressions and Literals ---
		{
			name:            "Identifier",
			node:            &Identifier{Token: testToken("myVar")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "myVar",
		},
		{
			name:            "IntegerLiteral",
			node:            &IntegerLiteral{Token: testToken("123")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "123",
		},
		{
			name:            "StringLiteral",
			node:            &StringLiteral{Token: testToken(`"hello"`)},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: `"hello"`,
		},
		{
			name:            "Boolean",
			node:            &Boolean{Token: testToken("TRUE")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "TRUE",
		},
		{
			name:            "PrefixExpression",
			node:            &PrefixExpression{Token: testToken("-")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "-",
		},
		{
			name:            "InfixExpression",
			node:            &InfixExpression{Token: testToken("+")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "+",
		},
		{
			name:            "CallExpression",
			node:            &CallExpression{Token: testToken("(")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "(",
		},
		{
			name:            "IndexExpression",
			node:            &IndexExpression{Token: testToken("[")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "[",
		},
		{
			name:            "ArrayLiteral",
			node:            &ArrayLiteral{Token: testToken("[")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "[",
		},
		{
			name:            "HashLiteral",
			node:            &HashLiteral{Token: testToken("{")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "{",
		},
		{
			name:            "FunctionLiteral",
			node:            &FunctionLiteral{Token: testToken("fn")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "fn",
		},
		{
			name:            "MacroLiteral",
			node:            &MacroLiteral{Token: testToken("macro")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "macro",
		},
		{
			name:            "DirectVariable",
			node:            &DirectVariable{Token: testToken("%QX0.1")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "%QX0.1",
		},
		{
			name:            "TypeSpecifier",
			node:            &TypeSpecifier{Token: testToken("BOOL")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "BOOL",
		},
		{
			name:            "MemberAccessExpression",
			node:            &MemberAccessExpression{Token: testToken(".")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: ".",
		},
		{
			name:            "FunctionParameter",
			node:            &FunctionParameter{Name: &Identifier{Token: testToken("param1")}},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "param1",
		},
		{
			name:            "NamedArgument",
			node:            &NamedArgument{Token: testToken("arg")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "arg",
		},
		{
			name:            "OutputArgument",
			node:            &OutputArgument{Token: testToken("=>")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "=>",
		},
		{
			name:            "ArrayRepetition",
			node:            &ArrayRepetition{Token: testToken("3")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "3",
		},
		// --- Additional Literals ---
		{
			name:            "UnsignedIntegerLiteral",
			node:            &UnsignedIntegerLiteral{Token: testToken("UINT#42")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "UINT#42",
		},
		{
			name:            "RealLiteral",
			node:            &RealLiteral{Token: testToken("3.14")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "3.14",
		},
		{
			name:            "LRealLiteral",
			node:            &LRealLiteral{Token: testToken("LREAL#1.23")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "LREAL#1.23",
		},
		{
			name:            "WStringLiteral",
			node:            &WStringLiteral{Token: testToken(`"wide"`)},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: `"wide"`,
		},
		{
			name:            "EnumeratedValueLiteral",
			node:            &EnumeratedValueLiteral{Token: testToken("COLOR")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "COLOR",
		},
		{
			name:            "BitStringLiteral",
			node:            &BitStringLiteral{Token: testToken("BYTE#FF")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "BYTE#FF",
		},
		{
			name:            "DateLiteral",
			node:            &DateLiteral{Token: testToken("D#2026-09-06")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "D#2026-09-06",
		},
		{
			name:            "TimeOfDayLiteral",
			node:            &TimeOfDayLiteral{Token: testToken("TOD#15:04:05")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "TOD#15:04:05",
		},
		{
			name:            "DateAndTimeLiteral",
			node:            &DateAndTimeLiteral{Token: testToken("DT#2026-09-06-15:04:05")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "DT#2026-09-06-15:04:05",
		},
		{
			name:            "TimeLiteral",
			node:            &TimeLiteral{Token: testToken("T#5s")},
			expectedLine:    10,
			expectedCol:     5,
			expectedLiteral: "T#5s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line, col := tt.node.Pos()
			if line != tt.expectedLine || col != tt.expectedCol {
				t.Errorf("Pos() wrong. want=(%d,%d), got=(%d,%d)", tt.expectedLine, tt.expectedCol, line, col)
			}

			if lit := tt.node.TokenLiteral(); lit != tt.expectedLiteral {
				t.Errorf("TokenLiteral() wrong. want=%q, got=%q", tt.expectedLiteral, lit)
			}

			if getter, ok := tt.node.(interface{ GetLeadingComments() []string }); ok {
				comments := getter.GetLeadingComments()
				if !reflect.DeepEqual(comments, tt.expectedComments) {
					t.Errorf("GetLeadingComments() wrong. want=%v, got=%v", tt.expectedComments, comments)
				}
			} else if tt.expectedComments != nil {
				t.Errorf("node type %T was expected to have GetLeadingComments, but does not", tt.node)
			}
		})
	}
}

func TestStatementNodes(t *testing.T) {
	// This test serves as a compile-time check to ensure that all AST nodes
	// intended to be statements correctly implement the Statement interface.
	// The `statementNode()` method is a marker method for this purpose.
	// Assigning them to a []Statement slice will cause a compilation error
	// if any of them do not implement the interface.
	statements := []Statement{
		&VarDeclStatement{},
		&ConfigurationDeclaration{},
		&ResourceDeclaration{},
		&TaskDeclaration{},
		&ProgramConfiguration{},
		&ExpressionStatement{},
		&AssignmentStatement{},
		&ReturnStatement{},
		&ExitStatement{},
		&BlockStatement{},
		&IfStatement{},
		&ForLoopStatement{},
		&WhileStatement{},
		&RepeatStatement{},
		&CaseBranch{},
		&CaseStatement{},
		&FunctionBlockDeclaration{},
		&ProgramDeclaration{},
		&ExternalVarDeclaration{},
		&ConfigVarDeclaration{},
		&TempVarDeclaration{},
		&AccessVarDeclaration{},
		&GlobalVarDeclaration{},
		&VarBlockDeclaration{},
		&TypeDeclaration{},
		&TypeBlockDeclaration{},
		&ActionStatement{},
		&FunctionDeclaration{},
		&IlInstructionStatement{},
		&StepStatement{},
		&TransitionStatement{},
		&SFCProgram{},
		&ActionBlockStatement{},
		&InterfaceDeclaration{},
		&MethodDeclaration{},
		&MethodImplementation{},
		&PropertyDeclaration{},
		&NamespaceDeclaration{},
	}

	// The loop simply iterates to prevent the compiler from optimizing away
	// the 'statements' variable. The real test is the compilation itself.
	for i, s := range statements {
		if s == nil {
			t.Errorf("statement at index %d is nil", i)
		}
		s.statementNode() // Calling the marker method for good measure.
	}
}

func TestExpressionNodes(t *testing.T) {
	// This test serves as a compile-time check to ensure that all AST nodes
	// intended to be expressions correctly implement the Expression interface.
	// The `expressionNode()` method is a marker method for this purpose.
	expressions := []Expression{
		&TypeSpecifier{},
		&AtDeclaration{},
		&DirectVariable{},
		&Identifier{},
		&Boolean{},
		&IntegerLiteral{},
		&UnsignedIntegerLiteral{},
		&RealLiteral{},
		&LRealLiteral{},
		&WStringLiteral{},
		&EnumeratedValueLiteral{},
		&BitStringLiteral{},
		&TypedLiteral{},
		&PrefixExpression{},
		&InfixExpression{},
		&MemberAccessExpression{},
		&FunctionParameter{},
		&FunctionLiteral{},
		&CallExpression{},
		&NamedArgument{},
		&OutputArgument{},
		&StringLiteral{},
		&TimeLiteral{},
		&DateLiteral{},
		&TimeOfDayLiteral{},
		&DateAndTimeLiteral{},
		&ArrayLiteral{},
		&ArrayRepetition{},
		&IndexExpression{},
		&HashLiteral{},
		&MacroLiteral{},
		&StructDefinition{},
		&EnumDefinition{},
		&FbTaskAssociation{},
		&ArrayDefinition{},
		// BlockStatement can also be an expression in some contexts (like IL)
		&ThisExpression{},
		&SuperExpression{},
		&BlockStatement{},
	}

	// The loop simply iterates to prevent the compiler from optimizing away
	// the 'expressions' variable. The real test is the compilation itself.
	for i, e := range expressions {
		if e == nil {
			t.Errorf("expression at index %d is nil", i)
		}
		e.expressionNode() // Calling the marker method for good measure.
	}
}
