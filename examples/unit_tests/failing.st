(* Tests that fail on purpose, to show how each kind of failure is
   reported. Run with
       beedance -test -engines all -go-timeout 5s motor.st failing.st *)

(* A failed check: failures <> 0, with the message of the last one. *)
PROGRAM TEST_WrongExpectation
VAR_OUTPUT failures : INT; message : STRING; END_VAR
IF ABS(Scale(raw := 27648) - 99.0) > 0.001 THEN
    failures := failures + 1; message := 'expected 99 %';
END_IF;
END_PROGRAM

(* Never done: a test with done that is not done after -max-scans fails. *)
PROGRAM TEST_NeverDone
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR m : MotorStarter; END_VAR
m(start := TRUE, stop := TRUE, delay := T#1s);  (* stop holds it off *)
done := m.run;
END_PROGRAM

(* A runaway loop: the evaluator and the VM stop it with their per-scan
   budget; the go engine stops its process after -go-timeout. *)
PROGRAM TEST_RunawayLoop
VAR_OUTPUT failures : INT; END_VAR
VAR i : INT; END_VAR
WHILE TRUE DO
    i := i + 1;
END_WHILE;
END_PROGRAM

(* Not a test by the convention: it has no VAR_OUTPUT failures. *)
PROGRAM TEST_NoFailuresOutput
VAR x : INT; END_VAR
x := 1;
END_PROGRAM
