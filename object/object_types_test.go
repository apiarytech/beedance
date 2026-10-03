package object

import (
	"strings"
	"testing"
	"time"

	"github.com/apiarytech/beedance/ast"
)

func ident(name string) *ast.Identifier { return &ast.Identifier{Value: name} }

// decl returns a variable declaration `name : typeName`.
func decl(name, typeName string) *ast.VarDeclStatement {
	return &ast.VarDeclStatement{Name: ident(name), DataType: ident(typeName)}
}

func TestObjectTypesAndInspect(t *testing.T) {
	fbi := &FunctionBlockInstance{Definition: &FunctionBlock{Name: ident("Motor")}}
	cf := &CompiledFunction{}
	tests := []struct {
		obj         Object
		wantType    ObjectType
		wantInspect string
	}{
		{&Hash{Pairs: map[HashKey]HashPair{}}, HASH_OBJ, "{}"},
		{&Method{Definition: &ast.MethodImplementation{Name: ident("Start")}}, METHOD_OBJ, "METHOD(Start)"},
		{&Method{}, METHOD_OBJ, "METHOD(<unnamed>)"},
		{&SuperContext{Instance: fbi}, SUPER_CONTEXT_OBJ, "SUPER_CONTEXT(FUNCTION_BLOCK_INSTANCE(Motor))"},
		{&InterfaceDefinition{Name: ident("IRun")}, INTERFACE_DEFINITION_OBJ, "INTERFACE IRun"},
		{&ProgramInstance{Definition: &Program{Name: ident("Main")}}, PROGRAM_INSTANCE_OBJ, "INSTANCE OF Main"},
		{&ProgramInstance{}, PROGRAM_INSTANCE_OBJ, "PROGRAM_INSTANCE"},
		{&Closure{Fn: cf}, CLOSURE_OBJ, ""},
		{&Constant{Value: &Int{Value: 3}}, CONSTANT_OBJ, "3"},
		{&NamedArgument{Name: "IN", Value: &Boolean{Value: true}}, NAMED_ARGUMENT_OBJ, "IN := true"},
		{&StructDefinition{Name: ident("Pt"), Members: []*ast.VarDeclStatement{decl("x", "INT")}}, STRUCT_DEFINITION_OBJ, ""},
		{&EnumDefinition{Name: ident("Color"), Values: []*ast.Identifier{ident("Red"), ident("Blue")}}, ENUM_DEFINITION_OBJ, "TYPE Color : (Red, Blue);"},
		{&ArrayDefinition{Name: ident("Row"), Ranges: []ast.Expression{ident("0..3")}, DataType: ident("INT")}, ARRAY_DEFINITION_OBJ, "TYPE Row : ARRAY [0..3] OF INT;"},
		{&Task{Name: "Fast", Priority: 1, Interval: 10 * time.Millisecond}, "TASK", "TASK(Fast, Priority: 1, Interval: 10ms)"},
		{&Scheduler{Tasks: []*Task{{Name: "A"}, {Name: "B", Priority: 2}}}, "SCHEDULER", "SCHEDULER(TASK(A, Priority: 0, Interval: 0s), TASK(B, Priority: 2, Interval: 0s))"},
		{&UncompiledMacro{Parameters: []*ast.Identifier{ident("a"), ident("b")}, Body: &ast.BlockStatement{}}, UNCOMPILED_MACRO_OBJ, ""},
	}
	for _, tt := range tests {
		if got := tt.obj.Type(); got != tt.wantType {
			t.Errorf("%T.Type() = %s, want %s", tt.obj, got, tt.wantType)
		}
		if tt.wantInspect != "" && tt.obj.Inspect() != tt.wantInspect {
			t.Errorf("%T.Inspect() = %q, want %q", tt.obj, tt.obj.Inspect(), tt.wantInspect)
		}
	}

	// Objects whose printed form includes an address or a multi-line body.
	if got := cf.Type(); got != COMPILED_FUNCTION_OBJ {
		t.Errorf("CompiledFunction.Type() = %s", got)
	}
	prefixes := map[Object]string{
		cf:               "CompiledFunction[",
		&Closure{Fn: cf}: "Closure[",
		&StructDefinition{Name: ident("Pt"), Members: []*ast.VarDeclStatement{decl("x", "INT")}}:             "TYPE Pt : STRUCT\n",
		&UncompiledMacro{Parameters: []*ast.Identifier{ident("a"), ident("b")}, Body: &ast.BlockStatement{}}: "macro(a, b) {\n",
	}
	for obj, prefix := range prefixes {
		if got := obj.Inspect(); !strings.HasPrefix(got, prefix) {
			t.Errorf("%T.Inspect() = %q, want prefix %q", obj, got, prefix)
		}
	}
	if got := (&StructDefinition{Name: ident("Pt"), Members: []*ast.VarDeclStatement{decl("x", "INT")}}).Inspect(); !strings.HasSuffix(got, "END_STRUCT") {
		t.Errorf("StructDefinition.Inspect() = %q, want suffix END_STRUCT", got)
	}
}

func TestUnhashableObjects(t *testing.T) {
	if (&Jump{TargetLabel: "L1"}).HashKey() != (HashKey{}) || (&Return{}).HashKey() != (HashKey{}) {
		t.Error("Jump and Return should have empty hash keys")
	}
}

func TestActionIsStored(t *testing.T) {
	for _, q := range []string{"S", "SD", "SL", "DS"} {
		if !(&Action{Qualifier: q}).IsStored() {
			t.Errorf("qualifier %s should be stored", q)
		}
	}
	for _, q := range []string{"N", "R", "P", ""} {
		if (&Action{Qualifier: q}).IsStored() {
			t.Errorf("qualifier %q should not be stored", q)
		}
	}
}

func TestProgramInspect(t *testing.T) {
	p := &Program{
		Name:       ident("Main"),
		VarInputs:  []*ast.VarDeclStatement{decl("in1", "INT")},
		VarOutputs: []*ast.VarDeclStatement{decl("out1", "BOOL")},
		VarInOuts:  []*ast.VarDeclStatement{decl("io1", "REAL")},
		// Every variable of every block is listed.
		VarTemp: []*ast.TempVarDeclaration{
			{Vars: []*ast.VarDeclStatement{decl("t1", "INT"), decl("t2", "INT")}},
			{Vars: []*ast.VarDeclStatement{decl("t3", "BOOL")}},
		},
		VarExternal: []*ast.ExternalVarDeclaration{{Vars: []*ast.VarDeclStatement{decl("e1", "INT")}}},
		VarGlobal:   []*ast.GlobalVarDeclaration{{Vars: []*ast.VarDeclStatement{decl("g1", "INT")}}},
		VarAccess:   []*ast.AccessVarDeclaration{{Vars: []*ast.VarDeclStatement{decl("a1", "INT")}}},
	}
	want := "PROGRAM Main (VAR_INPUT in1 : INT; VAR_OUTPUT out1 : BOOL; VAR_IN_OUT io1 : REAL; " +
		"VAR_TEMP t1 : INT; t2 : INT; t3 : BOOL; VAR_EXTERNAL e1 : INT; VAR_GLOBAL g1 : INT; VAR_ACCESS a1 : INT;)"
	if got := p.Inspect(); got != want {
		t.Errorf("Program.Inspect():\n got  %s\n want %s", got, want)
	}
	if p.Type() != PROGRAM_OBJ {
		t.Errorf("Program.Type() = %s", p.Type())
	}
}

func TestFunctionBlockInspectAllBlocks(t *testing.T) {
	fb := &FunctionBlock{
		Name: ident("Fb"),
		VarTemp: []*ast.TempVarDeclaration{
			{Vars: []*ast.VarDeclStatement{decl("t1", "INT"), decl("t2", "INT")}},
			{Vars: []*ast.VarDeclStatement{decl("t3", "BOOL")}},
		},
		VarExternal: []*ast.ExternalVarDeclaration{{Vars: []*ast.VarDeclStatement{decl("e1", "INT")}}},
	}
	want := "FUNCTION_BLOCK Fb (VAR_TEMP t1 : INT; t2 : INT; t3 : BOOL; VAR_EXTERNAL e1 : INT;)"
	if got := fb.Inspect(); got != want {
		t.Errorf("FunctionBlock.Inspect():\n got  %s\n want %s", got, want)
	}
}

func TestFunctionInspectAllBlocks(t *testing.T) {
	f := &Function{
		Name:       ident("F"),
		VarInputs:  []*ast.VarDeclStatement{decl("a", "INT")},
		VarOutputs: []*ast.VarDeclStatement{decl("o", "BOOL")},
		VarInOuts:  []*ast.VarDeclStatement{decl("io", "REAL")},
	}
	for _, part := range []string{"VAR_INPUT a : INT;", "VAR_OUTPUT o : BOOL;", "VAR_IN_OUT io : REAL;"} {
		if got := f.Inspect(); !strings.Contains(got, part) {
			t.Errorf("Function.Inspect() = %q, missing %q", got, part)
		}
	}
}

func TestEnvironmentSetLocalAndOuter(t *testing.T) {
	outer := NewEnvironment()
	outer.Set("x", &Int{Value: 1})
	inner := NewEnclosedEnvironment(outer)
	// SetLocal shadows rather than updating the outer scope.
	inner.SetLocal("x", &Int{Value: 2})
	if v, _ := outer.Get("x"); v.Inspect() != "1" {
		t.Errorf("outer x = %s, want 1", v.Inspect())
	}
	if v, _ := inner.Get("x"); v.Inspect() != "2" {
		t.Errorf("inner x = %s, want 2", v.Inspect())
	}
	if inner.Outer() != outer || outer.Outer() != nil {
		t.Error("Outer() does not return the enclosing environment")
	}
}

func TestBuiltinRegistry(t *testing.T) {
	// Work on the real registry, restoring it afterwards.
	savedByName, savedByIndex, savedList := builtinsByName, builtinsByIndex, Builtins
	builtinsByName, builtinsByIndex = map[string]*Builtin{}, map[int]BuiltinEntry{}
	defer func() { builtinsByName, builtinsByIndex, Builtins = savedByName, savedByIndex, savedList }()

	fn := func(args ...Object) Object { return &Int{Value: int16(len(args))} }
	RegisterBuiltin(2, "Z_FUNC", fn)
	RegisterBuiltin(0, "A_FUNC", fn)
	RegisterBuiltin(1, "M_FUNC", fn)
	FinalizeBuiltins()

	// Builtins is ordered by index, not by name or registration order.
	names := []string{}
	for i, entry := range Builtins {
		if entry.Index != i {
			t.Errorf("Builtins[%d].Index = %d", i, entry.Index)
		}
		names = append(names, entry.Name)
	}
	if got := strings.Join(names, ","); got != "A_FUNC,M_FUNC,Z_FUNC" {
		t.Errorf("Builtins order = %s", got)
	}
	b, ok := GetBuiltinByName("M_FUNC")
	if !ok || b.Fn(&Null{}, &Null{}).Inspect() != "2" {
		t.Error("GetBuiltinByName(M_FUNC) did not return the registered function")
	}
	if _, ok := GetBuiltinByName("NOPE"); ok {
		t.Error("GetBuiltinByName found an unregistered name")
	}
}
