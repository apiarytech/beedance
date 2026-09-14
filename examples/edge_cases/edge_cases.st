(*
 * This file tests the beedance parser and evaluator against known edge cases
 * and complex language constructs discovered during the development of other
 * IEC 61131-3 compilers, such as matiec.
 *)

(*
 * =============================================================================
 * Edge Case 1: Symbol Disambiguation
 *
 * The same identifier 'Ambiguous' is used as a TYPE, FUNCTION, and VARIABLE.
 * The parser should create a valid AST, and the evaluator should resolve the
 * correct symbol based on the context of its usage.
 * =============================================================================
 *)

TYPE
	Ambiguous : INT := 100; (* 1. A user-defined type named 'Ambiguous' *)
END_TYPE

FUNCTION Ambiguous : INT (* 2. A function named 'Ambiguous' *)
	VAR_INPUT
		InVal : INT;
	END_VAR
	Ambiguous := InVal * 2;
END_FUNCTION

PROGRAM Test_Symbol_Disambiguation
	VAR
		Ambiguous : INT := 5; (* 3. A variable named 'Ambiguous' *)
		Result1 : INT;
		Result2 : INT;
		MyTypedVar : Ambiguous; (* 4. Using the TYPE 'Ambiguous' *)
	END_VAR

	(* Use the variable 'Ambiguous' *)
	Result1 := Ambiguous + 1; (* In the evaluator, this should resolve to 5 + 1 = 6 *)

	(* Use the function 'Ambiguous' *)
	Result2 := Ambiguous(InVal := 20); (* This should resolve to 20 * 2 = 40 *)

	(* Use the type 'Ambiguous' *)
	MyTypedVar := 10; (* This should be a valid assignment *)
END_PROGRAM


(*
 * =============================================================================
 * Edge Case 2: IL Operators as Variable Names in Structured Text (ST)
 *
 * The IEC standard allows IL mnemonics like LD, ST, ADD to be used as
 * variable names. The parser should not treat them as reserved keywords.
 * =============================================================================
 *)

PROGRAM Test_IL_Operators_As_Vars
	VAR
		LD : INT := 10;
		ADD : INT := 20;
		ST : INT;
		Result : INT;
	END_VAR

	(* This should be parsed as a valid ST assignment, not IL. *)
	ST := LD + ADD; (* Evaluator should result in ST = 30 *)
	Result := ST * 2; (* Evaluator should result in Result = 60 *)
END_PROGRAM


(*
 * =============================================================================
 * Edge Case 3: Complex Grammar for TYPE declarations
 *
 * A TYPE declaration with an initial value, especially for STRINGs, caused
 * issues for older LALR(1) parsers. Beedance's Pratt parser handles this.
 * =============================================================================
 *)

TYPE
	MyString : STRING := 'Default Greeting';
	MyRangedInt : INT(-100..100) := 50;
END_TYPE

PROGRAM Test_Type_Declaration_Ambiguity
	VAR
		Greeting : MyString; (* Evaluator should see initial value 'Default Greeting' *)
		AnotherGreeting : MyString := 'Hello There';
		MyNum : MyRangedInt; (* Evaluator should see initial value 50 *)
	END_VAR

	(* The test for this is implicit in the evaluator's handling of initial values. *)
END_PROGRAM
