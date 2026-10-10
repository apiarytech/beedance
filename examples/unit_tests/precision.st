(* REAL precision differs between engines: the evaluator and the VM hold a
   REAL in 64 bits, transpiled Go in 32 (iec.REAL is a float32), so 100/3
   is 33.333333 on one and 33.333332 on the other. -tol is the relative
   tolerance for comparing REAL and LREAL values between engines:

       beedance -test -engines all motor.st precision.st            agree
       beedance -test -engines all -tol 1e-9 motor.st precision.st  DIFFER *)

PROGRAM TEST_RealPrecision
VAR_OUTPUT failures : INT; END_VAR
VAR third : REAL; END_VAR
third := Scale(raw := 9216);  (* 9216 / 27648 * 100 = 33.33... *)
IF ABS(third - 33.3333) > 0.001 THEN
    failures := failures + 1;
END_IF;
END_PROGRAM
