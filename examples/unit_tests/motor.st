(* The code under test: a motor starter and a scaling function.
   The tests are in motor_tests.st; run them with
       beedance -test motor.st motor_tests.st *)

TYPE MotorStatus :
STRUCT
    running : BOOL;  (* the motor is running *)
    starts  : INT;   (* how many times it started *)
END_STRUCT;
END_TYPE

(* Scale converts a raw analog input (0..27648) to percent (0..100). *)
FUNCTION Scale : REAL
VAR_INPUT raw : INT; END_VAR
Scale := INT_TO_REAL(raw) * 100.0 / 27648.0;
END_FUNCTION

(* MotorStarter runs the motor once start has been held for delay, and
   stops it at once on stop or fault. *)
FUNCTION_BLOCK MotorStarter
VAR_INPUT
    start : BOOL;
    stop  : BOOL;
    fault : BOOL;
    delay : TIME;
END_VAR
VAR_OUTPUT
    run    : BOOL;
    status : MotorStatus;
END_VAR
VAR
    startDelay : TON;
END_VAR
startDelay(IN := start AND NOT stop AND NOT fault, PT := delay);
IF startDelay.Q AND NOT run THEN
    run := TRUE;
    status.starts := status.starts + 1;
END_IF;
IF stop OR fault THEN
    run := FALSE;
END_IF;
status.running := run;
END_FUNCTION_BLOCK
