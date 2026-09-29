package evaluator

import "testing"

func TestFunctionCallEdges(t *testing.T) {
	const inc = "FUNCTION Inc : INT VAR_IN_OUT io : INT; END_VAR io := io + 1; Inc := io; END_FUNCTION "
	checkEval(t, []evalCase{
		// A VAR_IN_OUT can be passed positionally, and passed on to another function.
		{inc + "VAR v : INT := 4; END_VAR Inc(v); v;", "5"},
		{"FUNCTION G : INT VAR_IN_OUT io : INT; END_VAR io := io * 2; G := io; END_FUNCTION " +
			"FUNCTION F : INT VAR_IN_OUT io : INT; END_VAR G(io); F := io; END_FUNCTION VAR v : INT := 4; END_VAR F(v); v;", "8"},
		{inc + "Inc(5);", "ERROR: argument for VAR_IN_OUT parameter 'io' must be a variable"},
		{inc + "Inc(io := 5);", "ERROR: argument for VAR_IN_OUT parameter 'io' must be a variable"},
		// Outputs can be written to any assignable target.
		{"FUNCTION F : INT VAR_OUTPUT o : INT; END_VAR o := 3; F := 1; END_FUNCTION VAR a : ARRAY[0..1] OF INT; END_VAR F(o => a[0]); a;", "[3, 0]"},
		// An unknown return type leaves the result untyped.
		{"FUNCTION F : Nope F := 1; END_FUNCTION F();", "1"},
		{"FUNCTION F : INT VAR_INPUT a : INT; END_VAR F := a; END_FUNCTION F(missing);", "ERROR: identifier not found: missing"},
		{"FUNCTION F : INT VAR_INPUT a : INT; END_VAR F := a; END_FUNCTION F(1, 2);", "ERROR: too many arguments in function call"},
		{"FUNCTION F : INT VAR_INPUT a : INT; END_VAR F := a; END_FUNCTION F(a := missing);", "ERROR: identifier not found: missing"},
		{"FUNCTION F : INT VAR_INPUT a : INT := missing; END_VAR F := a; END_FUNCTION F();", "ERROR: identifier not found: missing"},
		{"FUNCTION F : INT VAR_OUTPUT o : INT := missing; END_VAR F := 1; END_FUNCTION F();", "ERROR: identifier not found: missing"},
		{"FUNCTION F : INT VAR x : INT := missing; END_VAR F := 1; END_FUNCTION F();", "ERROR: identifier not found: missing"},
		{"FUNCTION F : INT F := missing; END_FUNCTION F();", "ERROR: identifier not found: missing"},
		{"VAR x : INT; END_VAR x := 5; x();", "ERROR: not a function: LINT"},
	})
}

func TestProgramCallEdges(t *testing.T) {
	checkEval(t, []evalCase{
		{"PROGRAM Pg VAR_INPUT i : INT; END_VAR VAR_OUTPUT o : INT; END_VAR o := i * 2; END_PROGRAM VAR res : INT; END_VAR Pg(i := 4, o => res); res;", "8"},
		{"PROGRAM Pg VAR_OUTPUT o : INT; END_VAR o := 1; END_PROGRAM VAR a : ARRAY[0..1] OF INT; END_VAR Pg(o => a[1]); a;", "[0, 1]"},
		{"PROGRAM Pg VAR_IN_OUT io : INT; END_VAR io := io + 1; END_PROGRAM VAR v : INT := 1; END_VAR Pg(io := v); v;", "2"},
		{"PROGRAM Pg VAR_INPUT i : INT; END_VAR END_PROGRAM Pg(i := missing);", "ERROR: identifier not found: missing"},
		{"PROGRAM Pg VAR_TEMP t1 : INT := missing; END_VAR END_PROGRAM Pg();", "ERROR: identifier not found: missing"},
		{"PROGRAM Pg VAR x : INT; END_VAR x := missing; END_PROGRAM Pg();", "ERROR: identifier not found: missing"},
	})
}

func TestFunctionBlockCallEdges(t *testing.T) {
	const out7 = "FUNCTION_BLOCK Fb VAR_OUTPUT o : INT; END_VAR o := 7; END_FUNCTION_BLOCK "
	checkEval(t, []evalCase{
		// VAR_TEMP starts afresh on every call.
		{"FUNCTION_BLOCK Fb VAR_INPUT i : INT; END_VAR VAR_OUTPUT o : INT; END_VAR VAR_TEMP t1 : INT := 2; END_VAR o := i * t1; END_FUNCTION_BLOCK VAR f : Fb; res : INT; END_VAR f(i := 3, o => res); res;", "6"},
		// Outputs can go to array elements and structure members.
		{out7 + "VAR f : Fb; a : ARRAY[0..1] OF INT; END_VAR f(o => a[1]); a;", "[0, 7]"},
		{out7 + "TYPE Pt : STRUCT px : INT; END_STRUCT; END_TYPE VAR f : Fb; p : Pt; END_VAR f(o => p.px); p.px;", "7"},
		// With EN = FALSE the body does not run, but outputs are still mapped.
		{out7 + "VAR f : Fb; v : INT; END_VAR f(EN := FALSE, o => v); v;", "0"},
		// A VAR_IN_OUT keeps referring to the caller's variable; positional in-outs follow the inputs.
		{"FUNCTION_BLOCK Fb VAR_IN_OUT io : INT; END_VAR io := io + 1; END_FUNCTION_BLOCK VAR f : Fb; v : INT := 1; END_VAR f(io := v); f(io := v); v;", "3"},
		{"FUNCTION_BLOCK Fb VAR_INPUT a : INT; END_VAR VAR_IN_OUT io : INT; END_VAR io := io + a; END_FUNCTION_BLOCK VAR f : Fb; v : INT := 1; END_VAR f(10, v); v;", "11"},
		{out7 + "VAR f : Fb; i : INT; END_VAR f(o => i[1]);", "ERROR: index operator not supported for assignment: LINT"},
		{out7 + "VAR f : Fb; a : ARRAY[0..1] OF INT; END_VAR f(o => a[9]);", "ERROR: index out of bounds: 9"},
		{out7 + "VAR CONSTANT k : INT := 1; END_VAR VAR f : Fb; END_VAR f(o => k);", "ERROR: cannot assign to constant variable 'k'"},
		{out7 + "VAR f : Fb; END_VAR f(o => missing[0]);", "ERROR: identifier not found: missing"},
		{"FUNCTION_BLOCK Fb VAR_TEMP t1 : INT := missing; END_VAR END_FUNCTION_BLOCK VAR f : Fb; END_VAR f();", "ERROR: identifier not found: missing"},
		{"FUNCTION_BLOCK Fb VAR_INPUT i : INT; END_VAR END_FUNCTION_BLOCK VAR f : Fb; END_VAR f(i := missing);", "ERROR: identifier not found: missing"},
		{"FUNCTION_BLOCK Fb VAR x : INT; END_VAR x := missing; END_FUNCTION_BLOCK VAR f : Fb; END_VAR f();", "ERROR: identifier not found: missing"},
		{"FUNCTION_BLOCK TwoFb VAR_INPUT a : INT; END_VAR END_FUNCTION_BLOCK VAR f : TwoFb; END_VAR f(1, 2, 3);", "ERROR: too many arguments in function call"},
	})
}

func TestMethodAndPropertyEdges(t *testing.T) {
	checkEval(t, []evalCase{
		{"FUNCTION_BLOCK Fb METHOD M : INT VAR_INPUT a : INT; b : INT; END_VAR M := a * 10 + b; END_METHOD END_FUNCTION_BLOCK VAR f : Fb; END_VAR f.M(1, 2);", "12"},
		{"FUNCTION_BLOCK Fb METHOD M : Nope M := 1; END_METHOD END_FUNCTION_BLOCK VAR f : Fb; END_VAR f.M();", "1"},
		{"FUNCTION_BLOCK Dv METHOD M : INT M := THIS^.Q(); END_METHOD METHOD Q : INT Q := 4; END_METHOD END_FUNCTION_BLOCK VAR d1 : Dv; END_VAR d1.M();", "4"},
		{"FUNCTION_BLOCK Dv METHOD M : INT VAR_INPUT a : INT; END_VAR M := a; END_METHOD END_FUNCTION_BLOCK VAR d1 : Dv; END_VAR d1.M(1, 2);", "ERROR: too many arguments in call to method 'M'"},
		{"FUNCTION_BLOCK Dv METHOD M : INT VAR_INPUT a : INT; END_VAR M := a; END_METHOD END_FUNCTION_BLOCK VAR d1 : Dv; END_VAR d1.M(missing);", "ERROR: identifier not found: missing"},
		{"FUNCTION_BLOCK Fb METHOD M : INT VAR_INPUT a : INT; END_VAR M := a; END_METHOD END_FUNCTION_BLOCK VAR f : Fb; END_VAR f.M(a := missing);", "ERROR: identifier not found: missing"},
		{"FUNCTION_BLOCK Fb METHOD M : INT VAR_INPUT a : INT := missing; END_VAR M := a; END_METHOD END_FUNCTION_BLOCK VAR f : Fb; END_VAR f.M();", "ERROR: identifier not found: missing"},
		{"FUNCTION_BLOCK Fb METHOD M : INT VAR x : INT := missing; END_VAR M := 1; END_METHOD END_FUNCTION_BLOCK VAR f : Fb; END_VAR f.M();", "ERROR: identifier not found: missing"},
		{"FUNCTION_BLOCK Fb END_FUNCTION_BLOCK VAR f : Fb; END_VAR f.nope;", "ERROR: member 'nope' not found in function block instance 'Fb'"},
		{"FUNCTION_BLOCK Fb PROPERTY W : INT SET END_SET END_PROPERTY END_FUNCTION_BLOCK VAR f : Fb; END_VAR f.W;", "ERROR: property 'W' is write-only"},
		{"FUNCTION_BLOCK Fb PROPERTY R2 : INT GET R2 := 1; END_GET END_PROPERTY END_FUNCTION_BLOCK VAR f : Fb; END_VAR f.R2 := 5;", "ERROR: property 'R2' is read-only"},
		{"FUNCTION_BLOCK Fb PROPERTY W : INT SET x := missing; END_SET END_PROPERTY END_FUNCTION_BLOCK VAR f : Fb; END_VAR f.W := 5;", "ERROR: identifier not found: missing"},
		{"FUNCTION_BLOCK Fb VAR PRIVATE v : INT; END_VAR PROPERTY Pv : INT PRIVATE GET Pv := v; END_GET END_PROPERTY END_FUNCTION_BLOCK VAR f : Fb; END_VAR f.Pv;", "ERROR: cannot access getter for property 'Pv': member is private"},
		{"FUNCTION_BLOCK Base METHOD M : INT M := 1; END_METHOD END_FUNCTION_BLOCK FUNCTION_BLOCK Dv METHOD M : INT M := SUPER^.M(); END_METHOD END_FUNCTION_BLOCK VAR d1 : Dv; END_VAR d1.M();", "ERROR: SUPER call on a function block that does not extend another"},
		{"FUNCTION_BLOCK Base END_FUNCTION_BLOCK FUNCTION_BLOCK Dv EXTENDS Base METHOD M : INT M := SUPER^.Nope(); END_METHOD END_FUNCTION_BLOCK VAR d1 : Dv; END_VAR d1.M();", "ERROR: method 'Nope' not found in any parent function block"},
		{"VAR i : INT; END_VAR i^;", "ERROR: dereference operator (^) not applicable to type LINT"},
	})
}

func TestNamespaceAndInterfaceEdges(t *testing.T) {
	checkEval(t, []evalCase{
		{"NAMESPACE N FUNCTION F : INT F := 3; END_FUNCTION END_NAMESPACE N.F();", "3"},
		{"NAMESPACE N FUNCTION F : INT F := 3; END_FUNCTION END_NAMESPACE N.nope;", "ERROR: member 'nope' not found in namespace 'N'"},
		{"INTERFACE I METHOD M : INT; END_INTERFACE FUNCTION_BLOCK Fb IMPLEMENTS I END_FUNCTION_BLOCK VAR f : Fb; END_VAR", "ERROR: function block 'Fb' does not implement method 'M' from interface 'I'"},
		{"INTERFACE I PROPERTY P : INT GET; END_INTERFACE FUNCTION_BLOCK Fb IMPLEMENTS I END_FUNCTION_BLOCK VAR f : Fb; END_VAR", "ERROR: function block 'Fb' does not implement property 'P' from interface 'I'"},
		{"FUNCTION_BLOCK Fb IMPLEMENTS Nope END_FUNCTION_BLOCK VAR f : Fb; END_VAR", "ERROR: interface 'Nope' not found"},
		{"FUNCTION_BLOCK X END_FUNCTION_BLOCK FUNCTION_BLOCK Fb IMPLEMENTS X END_FUNCTION_BLOCK VAR f : Fb; END_VAR", "ERROR: 'X' is not an interface"},
		{"FUNCTION_BLOCK Fb EXTENDS Nope END_FUNCTION_BLOCK VAR f : Fb; END_VAR", "ERROR: parent function block 'Nope' not found during interface check"},
	})
}
