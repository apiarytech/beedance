(* Unit tests for motor.st. Every PROGRAM named TEST_... is a test:

     VAR_OUTPUT failures : INT;    checks that failed: 0 to pass
     VAR_OUTPUT message  : STRING; optional: what failed
     VAR_OUTPUT done     : BOOL;   optional: run scan by scan until TRUE

   Without done a test runs one scan. With done it runs scan by scan on a
   simulated clock (-interval apart, 10 ms by default) until done is TRUE. *)

(* One scan: a function's results. *)
PROGRAM TEST_Scale
VAR_OUTPUT failures : INT; message : STRING; END_VAR
IF ABS(Scale(raw := 0)) > 0.001 THEN
    failures := failures + 1; message := 'Scale(0) <> 0';
END_IF;
IF ABS(Scale(raw := 27648) - 100.0) > 0.001 THEN
    failures := failures + 1; message := 'Scale(27648) <> 100';
END_IF;
IF ABS(Scale(raw := 13824) - 50.0) > 0.001 THEN
    failures := failures + 1; message := 'Scale(13824) <> 50';
END_IF;
END_PROGRAM

(* Scan by scan: the motor must not run before its 5 s delay, and must
   run once it has passed. *)
PROGRAM TEST_MotorStartsAfterDelay
VAR_OUTPUT failures : INT; message : STRING; done : BOOL; END_VAR
VAR
    m     : MotorStarter;
    scans : INT;
END_VAR
scans := scans + 1;
m(start := TRUE, stop := FALSE, fault := FALSE, delay := T#5s);
(* At 10 ms a scan, scan 400 is at 3.99 s: too early to run. *)
IF scans = 400 AND m.run THEN
    failures := failures + 1; message := 'ran before the delay';
END_IF;
IF m.run THEN
    IF m.status.starts <> 1 THEN
        failures := failures + 1; message := 'starts <> 1';
    END_IF;
    done := TRUE;
END_IF;
END_PROGRAM

(* A sequence of steps: start, run, then a fault stops the motor at once. *)
PROGRAM TEST_MotorStopsOnFault
VAR_OUTPUT failures : INT; message : STRING; done : BOOL; END_VAR
VAR
    m     : MotorStarter;
    scans : INT;
    fault : BOOL;
END_VAR
scans := scans + 1;
fault := scans >= 20;  (* the fault comes at scan 20 *)
m(start := TRUE, stop := FALSE, fault := fault, delay := T#100ms);
IF scans = 19 AND NOT m.run THEN
    failures := failures + 1; message := 'not running before the fault';
END_IF;
IF scans = 20 THEN
    IF m.run OR m.status.running THEN
        failures := failures + 1; message := 'still running on fault';
    END_IF;
    done := TRUE;
END_IF;
END_PROGRAM

(* A long timer: a 5-minute delay takes 30,001 scans at the default 10 ms,
   301 with -interval 1s. The clock is simulated, so it runs in well under
   a second per engine, not 5 minutes. *)
PROGRAM TEST_LongDelay
VAR_OUTPUT failures : INT; message : STRING; done : BOOL; END_VAR
VAR m : MotorStarter; END_VAR
m(start := TRUE, delay := T#5m);
done := m.run;
END_PROGRAM
