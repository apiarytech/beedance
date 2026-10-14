(* The arithmetic workload of the embedded benchmark: a loop of integer and
   real operations, the instruction dispatch and arithmetic of each engine
   with no function block or I/O. acc ends at 303 every scan. *)
PROGRAM Main
VAR
    i : DINT;
    acc : DINT;
    x : REAL;
END_VAR
acc := 0;
FOR i := 1 TO 100 DO
    acc := acc + (i * 3) MOD 7;
    x := x * 0.5 + 1.0;
END_FOR;
END_PROGRAM
