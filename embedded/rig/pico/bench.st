(* Bench test for the Pico rig, with GP2 wired to GP10 and ADC0 at 2048:
   beedance -test -io serial:/dev/ttyACM0 embedded/rig/pico/bench.st
   It drives %QX0.0 (GP2), waits for it on %IX0.0 (GP10) and checks %IW0 (ADC0). *)
PROGRAM TEST_Bench
VAR_OUTPUT failures : INT; message : STRING; done : BOOL; END_VAR
VAR
	q AT %QX0.0 : BOOL;
	back AT %IX0.0 : BOOL;
	pot AT %IW0 : INT;
	scans : INT;
END_VAR
scans := scans + 1;
q := TRUE;
IF pot <> 2048 THEN failures := failures + 1; message := 'ADC0 not 2048'; END_IF;
IF back THEN done := TRUE; END_IF;
IF scans > 50 THEN failures := failures + 1; message := 'GP10 never followed GP2'; done := TRUE; END_IF;
END_PROGRAM
