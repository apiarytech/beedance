PROGRAM TrafficLight
	VAR
		State : INT := 0;
		StateTimer : TON;
		Green_Light : BOOL;
		Yellow_Light : BOOL;
		Red_Light : BOOL;
	END_VAR

	(* Call the timer instance on every scan *)
	StateTimer(IN := TRUE, PT := T#5s);

	CASE State OF
		0 : (* Green State *)
			Green_Light := TRUE;
			Yellow_Light := FALSE;
			Red_Light := FALSE;
			IF StateTimer.Q THEN State := 1; StateTimer(IN := FALSE); END_IF

		1 : (* Yellow State *)
			Green_Light := FALSE;
			Yellow_Light := TRUE;
			IF StateTimer.Q THEN State := 2; StateTimer(IN := FALSE); END_IF

		2 : (* Red State *)
			Yellow_Light := FALSE;
			Red_Light := TRUE;
			IF StateTimer.Q THEN State := 0; StateTimer(IN := FALSE); END_IF
	END_CASE
END_PROGRAM