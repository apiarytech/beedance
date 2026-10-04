FUNCTION_BLOCK ACTUATOR_2P
VAR_INPUT
	IN : BYTE;
	TEST : BOOL;
	ARE : BOOL := TRUE;
END_VAR
VAR_IN_OUT
	ARX : BOOL;
END_VAR
VAR_INPUT CONSTANT
	CYCLE_TIME : TIME;
	SENS : BYTE;
	SELF_ACT_TIME : TIME;
	SELF_ACT_PULSE : TIME;
	SELF_ACT_CYCLES : INT := 1;
END_VAR
VAR_OUTPUT
	OUT : BOOL;
	ARO : BOOL;
END_VAR
VAR
	timer : AUTORUN;
	pwgen : GEN_PULSE;
END_VAR

(* run the autorun timer *)
timer(trun := SELF_ACT_PULSE * (2 * SELF_ACT_CYCLES), toff := SELF_ACT_TIME, Test := TEST, ARE := ARE, arx := ARX);
ARO := timer.ARO;

(* run the pulse width generator *)
IF aro THEN
	pwgen(PTL := SELF_ACT_PULSE, PTH := SELF_ACT_PULSE);
	out := pwgen.Q;
ELSIF in < sens THEN
	out := FALSE;
ELSIF in > BYTE#255 - sens THEN
	out := TRUE;
ELSE
	pwgen.PTH := (CYCLE_TIME * BAND_B(IN,SENS)) / 255;
	pwgen.PTL := CYCLE_TIME - pwgen.PTH;
	pwgen();
	out := pwgen.Q;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK ACTUATOR_3P
VAR_INPUT
	IN : BYTE;
	TEST : BOOL;
	ARE : BOOL := TRUE;
	END_POS : BOOL;
END_VAR
VAR_INPUT CONSTANT
	T_RUN : TIME := t#60s;
	T_EXT : TIME := T#10s;
	T_CAL : TIME := t#600s;
	T_DIAG : TIME := T#10d;
	SWITCH_AVAIL : BOOL;
END_VAR
VAR_IN_OUT
	ARX : BOOL;
END_VAR
VAR_OUTPUT
	OUT1 : BOOL;
	OUT2 : BOOL;
	POS : BYTE;
	ERROR : BOOL;
	STATUS : BYTE;
END_VAR
VAR
	ramp : _RMP_NEXT;
	tx : TIME ;
	next_cal : TIME;
	next_diag : TIME;
	last: TIME;
	start: TIME;
END_VAR

(* read system timer *)
tx := DWORD_TO_TIME(T_PLC_MS());

(* check test input *)
IF TEST THEN
	status := 103;
	start := tx;
	ARX := TRUE;
END_IF;

CASE status OF
	0:	(* power on setup *)
		IF ARE AND NOT ARX THEN
			status := 103;
			start := tx;
			ARX := TRUE;
		END_IF;

	100:	(* normal operation *)
		(* check for auto diagnostics *)
		IF T_DIAG > T#0s AND tx > next_diag AND ARE AND NOT ARX THEN
				status := 103;
				start := tx;
				ARX := TRUE;

		(* check for auto calibration *)
		ELSIF T_CAL > T#0s AND tx > next_cal AND ARE AND NOT ARX THEN
			IF pos > BYTE#127 THEN
				OUT1 := TRUE;
				OUT2 := FALSE;
				ramp.IN := 255;
				ARX := TRUE;
			ELSE
				OUT1 := FALSE;
				OUT2 := TRUE;
				ramp.IN := 0;
				ARX := TRUE;
			END_IF;
			status := 101;
			start := tx;
		ELSE
			(* increment next_cal if not active *)
			IF NOT(OUT1 OR OUT2) THEN next_cal := next_cal + (tx-last); END_IF;
			(* set ramp generator to IN *)
			ramp.IN := IN;
		END_IF;

	101:	(* calibrate *)
		IF tx - start < T_EXT THEN
			next_cal := tx + T_CAL;
		ELSIF SWITCH_AVAIL AND END_POS THEN
			STATUS := 100;
			ARX := FALSE;
		ELSIF tx - start > T_EXT + T_RUN THEN
			ERROR := SWITCH_AVAIL;
			ARX := FALSE;
		END_IF;

	103:	(* diagnostics up*)
		(* run up for T_ext *)
		IF tx - start < T_EXT THEN
			ERROR := FALSE;
			ramp.TR := T_RUN;
			ramp.TF := T_RUN;
			OUT1 := TRUE;
			OUT2 := FALSE;
			ramp.IN := 255;
		ELSIF SWITCH_AVAIL AND END_POS THEN
			ramp.TR := tx - start;
			STATUS := 104;
		ELSIF tx - start > T_EXT + T_RUN THEN
			ERROR := SWITCH_AVAIL;
			STATUS := 104;
			start := tx;
		END_IF;

	104:	(* diagnostics dn*)
		IF tx - start < T_ext THEN
			OUT1 := FALSE;
			OUT2 := TRUE;
			ramp.IN := 0;
			next_diag := tx + T_DIAG;
		ELSIF SWITCH_AVAIL AND END_POS THEN
			ramp.TR := tx - start;
			(* check if runtimes differ by more than 10% *)
			IF DINT_TO_TIME(ABS(TIME_TO_DINT(ramp.TR) - TIME_TO_DINT(ramp.TF))*10) > T_RUN THEN error := TRUE; END_IF;
			STATUS := 100;
			ARX := FALSE;
			next_cal := tx + T_CAL;
		ELSIF tx - start > T_EXT + T_RUN THEN
			IF SWITCH_AVAIL THEN ERROR := TRUE; END_IF;
			STATUS := 100;
			ARX := FALSE;
			next_CAL := tx + T_CAL;
		END_IF;
END_CASE;

(* internal flap simulation and output activation *)
ramp(OUT := POS);
IF STATUS = 100 THEN
	OUT1 := ramp.UP;
	OUT2 := ramp.DN;
END_IF;

(* adjust position if end switch is active *)
IF SWITCH_AVAIL AND END_POS THEN
	POS := SEL(POS > 127, 0, 255);
	next_cal := tx + T_CAL;
END_IF;

(* set last to tx for next cycle *)
last := tx;
END_FUNCTION_BLOCK

FUNCTION_BLOCK ACTUATOR_A
VAR_INPUT
	I1 : BYTE;
	IS : BOOL;
	I2 : BYTE;
	RV : BOOL;
	DX : BOOL;
END_VAR
VAR_INPUT CONSTANT
	RUNTIME : TIME;
	SELF_ACT_TIME : TIME;
	OUT_MIN : WORD;
	OUT_MAX : WORD;
END_VAR
VAR_OUTPUT
	Y : WORD;
END_VAR
VAR
	timer : CYCLE_4;
	dx_edge: BOOL;
END_VAR

(* run the system timer, if self_act_time = t#0s then state is set to 0 *)
timer(T0 := RUNTIME, T1 := RUNTIME, T3 := SELF_ACT_TIME, sl := DX AND NOT dx_edge, sx := 0, S0 := SELF_ACT_TIME > t#0s);
dx_edge := dx;

CASE timer.STATE OF
	0: 	(* self act min value *)
		Y := OUT_MIN;
	1:	(* self_act max value *)
		Y := OUT_MAX;
	3:	(* normal operation *)
		IF rv THEN
			Y := OUT_MAX - (OUT_MAX - OUT_MIN) * SEL(IS, I1, I2) / 255;
		ELSE
			Y := (OUT_MAX - OUT_MIN) * SEL(IS, I1, I2) / 255 + OUT_MIN;
		END_IF;
END_CASE;
END_FUNCTION_BLOCK

FUNCTION_BLOCK ACTUATOR_COIL
VAR_INPUT
	IN : BOOL;
END_VAR
VAR_INPUT CONSTANT
	self_act_cycle : TIME := t#10d;
	self_act_time : TIME := t#1s;
END_VAR
VAR_OUTPUT
	OUT :BOOL;
	STATUS : BYTE;
END_VAR
VAR
	last : DWORD;
	init: BOOL;
	tx: DWORD;
	now: DWORD;
END_VAR

(* read system time *)
now := T_PLC_MS();

(* initialize for first cycle *)
IF NOT init THEN
	last := now;
	init := TRUE;
ELSIF IN THEN
	OUT := TRUE;
	STATUS := 101; (* activated by input *)
	LAST := now;
ELSE
	OUT := FALSE;
	STATUS := 100; (* disabled *)
	(* now we need to check for self activation *)
	tx := now - last;
	IF (self_act_cycle > T#0s) AND (tx >= TIME_TO_DWORD(self_act_cycle)) THEN
		OUT := TRUE;
		STATUS := 102; (* auto activation *)
		IF tx >= TIME_TO_DWORD(self_act_cycle + self_act_time) THEN
			last := now;
			OUT := FALSE;
			STATUS := 100; (* idle *)
		END_IF;
	END_IF;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK ACTUATOR_PUMP
VAR_INPUT
	IN : BOOL;
	MANUAL : BOOL;
	RST : BOOL;
END_VAR
VAR_INPUT CONSTANT
	MIN_ONTIME : TIME := t#10s;
	MIN_OFFTIME : TIME := t#10s;
	RUN_EVERY : TIME := t#10000m;
END_VAR
VAR_OUTPUT
	PUMP : BOOL;
END_VAR
VAR_IN_OUT
	RUNTIME : UDINT;
	CYCLES : UDINT;
END_VAR
VAR
	tx : TIME;
	last_change : TIME;
	meter : ontime;
	old_man : BOOL;
	init: BOOL;
END_VAR

tx := DWORD_TO_TIME(T_PLC_MS());

IF NOT init THEN
	init := TRUE;
	last_change := tx;
ELSIF rst THEN
	rst := FALSE;
	runtime := 0;
	cycles := 0;
ELSIF manual AND NOT pump AND NOT old_man THEN
	last_change := tx;
	pump := TRUE;
ELSIF NOT manual AND old_man AND pump AND NOT in THEN
	last_change := tx;
	pump := FALSE;
ELSIF in AND NOT pump AND tx - last_change >= min_offtime THEN
	last_change := tx;
	pump := TRUE;
ELSIF pump AND NOT in AND NOT manual AND tx - last_change >= min_ontime THEN
	last_change := tx;
	pump := FALSE;
ELSIF NOT pump AND (tx - last_change >= run_every) AND (run_every > T#0s) THEN
	last_change := tx;
	pump := TRUE;
END_IF;

meter(in := pump, seconds := runtime, cycles := cycles);
old_man := manual;
END_FUNCTION_BLOCK

FUNCTION_BLOCK ACTUATOR_UD
VAR_INPUT
	UD : BOOL;
	ON : BOOL;
	Manual : BOOL;
	Up : BOOL;
	Dn : BOOL;
	off : BOOL;
	YUP_in : BOOL;
	YDN_in : BOOL;
END_VAR
VAR_INPUT CONSTANT
	TON : TIME;
	TOFF : TIME;
	out_return : BOOL;
END_VAR
VAR_OUTPUT
	Yup : BOOL;
	Ydn : BOOL;
	status : BYTE;
END_VAR
VAR
	tx : TIME;
	last : TIME;
	init: BOOL;
END_VAR

(* read system time *)
tx := DWORD_TO_TIME(T_PLC_MS());

IF NOT init THEN
	(* INITIALIZATION AT FIRST CALL *)
	last := tx;
	init := TRUE;
ELSIF off THEN
	(* emergency shut off detected *)
	Yup := FALSE;
	ydn := FALSE;
	last := tx;
	status := 101;

(* Watch FOR MIN TON TIME *)
ELSIF ((Yup OR Ydn) AND tx - last < Ton) THEN
	RETURN;
ELSIF NOT yup AND NOT ydn AND tx - last < toff THEN
	RETURN;

(* manual operation *)
ELSIF manual THEN
	status := 102;
	(* directional chage set both outs 0 first *)
	IF yup AND NOT up OR ydn AND NOT dn THEN
		ydn := FALSE;
		yup := FALSE;
		last := tx;
	(* manual up operation *)
	ELSIF UP AND NOT DN AND NOT OFF THEN
		Ydn := FALSE;
		Yup := TRUE;
		last := tx;
		status := 103;
	(* manual down operation *)
	ELSIF DN AND NOT UP AND NOT OFF THEN
		Yup := FALSE;
		Ydn := TRUE;
		last := tx;
		status := 104;
	(* maual mode but no up or down *)
	ELSE
		IF YUP OR YDN THEN LAST := tx; END_IF;
		Yup := FALSE;
		ydn := FALSE;
	END_IF;

(* automatic operation *)
ELSE
	(* directional change set outputs low first *)
	IF yup AND NOT ud OR ydn AND UD THEN
		yup := FALSE;
		ydn := FALSE;
		last := tx;
	(* automatic up operation *)
	ELSIF Ud AND on AND NOT OFF THEN
		Ydn := FALSE;
		Yup := TRUE;
		last := tx;
		status := 111;

	(* automatic down operation *)
	ELSIF NOT UD AND on AND NOT OFF THEN
		Yup := FALSE;
		Ydn := TRUE;
		last := tx;
		status := 112;
	(* standby mode *)
	ELSE
		IF Yup OR Ydn THEN last := tx; END_IF;
		Yup := FALSE;
		Ydn := FALSE;
		status := 110;
	END_IF;
END_IF;

(* make sure Yup and Ydn are never on at the same time *)
IF Ydn AND Yup_in AND out_return THEN
	Ydn := FALSE;
	status := 1;
END_IF;
IF Yup AND Ydn_in AND out_return THEN
	Yup := FALSE;
	status := 2;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK AUTORUN
VAR_INPUT
	IN : BOOL;
	TEST : BOOL;
	ARE : BOOL := TRUE;
END_VAR
VAR_IN_OUT
	ARX : BOOL;
END_VAR
VAR_INPUT CONSTANT
	TRUN : TIME;
	TOFF : TIME;
END_VAR
VAR_OUTPUT
	OUT : BOOL;
	ARO : BOOL;
END_VAR
VAR
	timer : _RMP_B;
	val : BYTE;
END_VAR

(* run the integrator when T_off > T#0s *)
IF TOFF > T#0s THEN
	timer(DIR:= OUT, TR := SEL(OUT, TOFF, TRUN), RMP := val);
ELSE
	val := 255;
END_IF;

(* when timer = 0 or test then initiate autorun until timer reaches 255 *)
IF (NOT ARX AND ARE AND val = 0) OR TEST THEN
	val := 0;
	ARO := TRUE;
	ARX := TRUE;
ELSIF val = 255 AND ARO THEN
	ARO := FALSE;
	ARX := FALSE;
END_IF;

(* output is true when autorun ARO is true or in = TRUE *)
out := IN OR ARO;
END_FUNCTION_BLOCK

// Changed for beedance: SINGLE is a keyword of IEC 61131-3, so it is SINGLE_ here.
FUNCTION_BLOCK CLICK
VAR_INPUT
	in : BOOL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	single_ : BOOL;
	double : BOOL;
	triple : BOOL;
	status: BYTE;
END_VAR
VAR_INPUT CONSTANT
	T_debounce : TIME := t#10ms;
	T_short : TIME := t#200ms;
	T_pause : TIME := t#500ms;
	T_reconfig : TIME := t#1m;
END_VAR
VAR
	s_in : SW_RECONFIG;
	tx: TIME;
	state: INT;
	last: TIME;
END_VAR

(* reconfiguration and debounce circuitry *)
s_in(in := in, TD := T_debounce, TR := T_reconfig);

(* read system time *)
tx := DWORD_TO_TIME(T_PLC_MS());

(* count the mumber of clicks *)
IF s_in.Q XOR Q THEN
	last := tx;
	IF s_in.Q THEN state := state + 1; END_IF;
END_IF;
Q := s_in.Q;

(* set outputs *)
IF state > 0 THEN
	IF (Q AND tx - last > t_short) OR (NOT Q AND tx - last > t_pause) THEN
		CASE state OF
			1:	single_ := TRUE;
				status := 111;
			2:	double := TRUE;
				status := 112;
			3:	triple := TRUE;
				status := 113;
		END_CASE;
		state := 0;
	END_IF;
ELSIF NOT q THEN
	single_ := FALSE;
	double := FALSE;
	triple := FALSE;
	status := 110;
	last := tx;
	state := 0;
END_IF;
END_FUNCTION_BLOCK

// Changed for beedance: SINGLE is a keyword of IEC 61131-3, so it is SINGLE_ here.
FUNCTION_BLOCK CLICK_MODE
VAR_INPUT
	IN : BOOL;
END_VAR
VAR_OUTPUT
	SINGLE_ : BOOL;
	DOUBLE : BOOL;
	LONG : BOOL;
	TP_LONG : BOOL;
END_VAR
VAR_INPUT CONSTANT
	T_LONG : TIME := t#500ms;
END_VAR
VAR
	Timer : TP;
	cnt : INT;
	last : BOOL;
END_VAR

(* when input goes high start the timer to decode pulses *)
timer(in := IN, PT := T_LONG);
single_ := FALSE;
double := FALSE;

IF timer.Q THEN
	(* decode pulses while the timer is active *)
	IF NOT in AND last THEN	cnt := cnt + 1; END_IF;
ELSE
	CASE cnt OF
		1 : single_ := TRUE;
		2 : double := TRUE;
	END_CASE;
	cnt := 0;
END_IF;
last := in;
TP_LONG := NOT timer.Q AND (NOT LONG) AND IN;
LONG := NOT timer.Q AND in;
END_FUNCTION_BLOCK

FUNCTION_BLOCK DEBOUNCE
VAR_INPUT
	IN : BOOL;
	TD : TIME;
	PM : BOOL;
END_VAR

VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	deb : TOF;
END_VAR

IF NOT deb.Q AND IN THEN
	(* rising edge on input detetced a *)
	Q := TRUE;
ELSIF NOT PM THEN
	Q := deb.Q;
ELSE
	Q := FALSE;
END_IF;

(* run debounce timer *)
deb(IN := IN, PT := TD);
END_FUNCTION_BLOCK

// Changed for beedance: SINGLE is a keyword of IEC 61131-3, so it is SINGLE_ here.
FUNCTION_BLOCK DIMM_2
VAR_INPUT
	SET : BOOL;
	VAL : BYTE := 255;
	I1 : BOOL;
	I2 : BOOL;
	RST : BOOL;
END_VAR
VAR_INPUT CONSTANT
	T_DEBOUNCE : TIME := T#10ms;
	T_ON_MAX : TIME := T#0h;
	T_DIMM_START : TIME := T#1s;
	T_DIMM: TIME := T#3s;
	MIN_ON : BYTE := 50;
	MAX_ON : BYTE := 255;
	RST_OUT : BOOL := TRUE;
	SOFT_DIMM : BOOL := TRUE;
	DBL1_TOG : BOOL;
	DBL2_TOG : BOOL;
	DBL1_SET : BOOL;
	DBL2_SET : BOOL;
	DBL1_POS : BYTE;
	DBL2_POS : BYTE;
END_VAR
VAR_OUTPUT
	Q : BOOL := FALSE;
	D1 : BOOL;
	D2 : BOOL;
END_VAR
VAR_IN_OUT
	OUT : BYTE;
END_VAR
VAR
	t1, t2 : TOF;
	dc1, dc2 : CLICK_MODE;
	t3 : TON;
	dim : _RMP_B;
END_VAR

(* debouncing for inputs i1 and i2 *)
t1(IN := I1, PT := T_DEBOUNCE);
t2(IN := I2, PT := T_DEBOUNCE);
dc1(in := t1.Q, T_LONG := T_DIMM_START);
dc2(in := t2.Q, T_LONG := T_DIMM_START);

IF RST THEN
	IF RST_OUT THEN out := 0; END_IF;
	Q := FALSE;
	D1 := FALSE;
	D2 := FALSE;
ELSIF set THEN
	OUT := VAL;
	Q := TRUE;
ELSIF dc1.SINGLE_ THEN
	(* single click on i1 turns the dimmer on *)
	out := LIMIT(MAX(MIN_ON,1), out, MAX_ON);
	Q := TRUE;
ELSIF dc2.SINGLE_ THEN
	(* single click on i2 turns the dimmer off *)
	Q := FALSE;
ELSIF dc1.TP_LONG THEN
	(* continuous on at i1 dimms up *)
	IF NOT Q THEN
		out := SEL(SOFT_DIMM, LIMIT(MAX(MIN_ON,1), out, MAX_ON), 1);
	END_IF;
	Q := TRUE;
	dim.DIR := TRUE;
ELSIF dc2.TP_LONG THEN
	(* continuous on at i2 dimms down *)
	dim.DIR := FALSE;
END_IF;

(* set the double click output *)
IF NOT DBL1_TOG THEN D1 := FALSE; END_IF;
IF dc1.DOUBLE THEN
	IF DBL1_SET THEN
		OUT := DBL1_POS;
		Q := TRUE;
	ELSE
		D1 := NOT D1;
	END_IF;
END_IF;
IF NOT DBL2_TOG THEN D2 := FALSE; END_IF;
IF dc2.DOUBLE THEN
	IF DBL2_SET THEN
		OUT := DBL2_POS;
		Q := TRUE;
	ELSE
		D2 := NOT D2;
	END_IF;
END_IF;

(* while dimming is active ramp the output out up or down *)
dim(dir := dc2.LONG, e := dc1.LONG OR dc2.LONG , TR := T_DIMM, rmp := out);

(* turn off output if a value of 0 is reached *)
IF out = 0 THEN Q := FALSE; END_IF;

(* limit the maximum runtime *)
IF t_ON_MAX > t#0s THEN
	t3(in := q, pt := T_ON_MAX);
	Q := Q XOR t3.Q;
END_IF;
END_FUNCTION_BLOCK

// Changed for beedance: SINGLE is a keyword of IEC 61131-3, so it is SINGLE_ here.
FUNCTION_BLOCK DIMM_I
VAR_INPUT
	SET : BOOL;
	VAL : BYTE := 255;
	IN : BOOL;
	RST : BOOL;
END_VAR
VAR_INPUT CONSTANT
	T_DEBOUNCE : TIME := T#10ms;
	T_RECONFIG : TIME := T#10s;
	T_ON_MAX : TIME := T#0h;
	T_DIMM_START : TIME := T#1s;
	T_DIMM: TIME := T#3s;
	MIN_ON : BYTE := 50;
	MAX_ON : BYTE := 255;
	SOFT_DIMM : BOOL := TRUE;
	DBL_toggle : BOOL;
	RST_OUT : BOOL;
END_VAR
VAR_OUTPUT
	Q : BOOL := FALSE;
	DBL : BOOL;
END_VAR
VAR_IN_OUT
	OUT : BYTE;
END_VAR
VAR
	config : SW_RECONFIG;
	decode : CLICK_MODE;
	t3 : TON;
	dim : _RMP_B;
	dir : BOOL;
END_VAR

(* the input is first sent through auto reconfiguration and debouncing *)
config(IN := IN, TD := T_DEBOUNCE, TR := T_RECONFIG);
decode(in := config.Q, T_LONG := T_DIMM_START);

IF RST THEN
	IF rst_out THEN out := 0; END_IF;
	Q := FALSE;
	dir := out > 127;
ELSIF set THEN
	out := val;
	Q := TRUE;
	(* set direction to up when value < 127 otherwise set dir down dir is reversed because next action will reverse again *)
	dir := out > 127;
ELSIF decode.SINGLE_ THEN
	(* a single click reverses output Q *)
	Q := NOT Q;
	(* when dimmer is turned on we need to limit out to min and max limits *)
	IF Q THEN	out := LIMIT(MAX(MIN_ON,1), out, MAX_ON);	END_IF;
	(* set the appropriate direction of dimmer dir is reversed because next action will reverse again *)
	dir := out > 127;
ELSIF decode.TP_LONG THEN
	IF NOT Q THEN
		IF SOFT_DIMM THEN
			OUT := 1;
			dir := TRUE;
		ELSE
			OUT := LIMIT(MAX(MIN_ON,1), out, MAX_ON);
			DIR := out < 127;
		END_IF;
		Q := TRUE;
	ELSE
		(* reverse direction with every long click *)
		dir := NOT dir;
	END_IF;
END_IF;

(* set the double click output *)
IF NOT dbl_toggle THEN dbl := FALSE; END_IF;
IF decode.DOUBLE THEN dbl := NOT dbl; END_IF;

(* while dimming is active ramp the output out up or down *)
dim(DIR := dir, e := decode.LONG AND Q, TR := T_DIMM, rmp := out);

(* reverse direction when limits are reached *)
IF out = 0 THEN
	dir := TRUE;
ELSIF out = 255 THEN
	dir := FALSE;
END_IF;

(* limit the maximum runtime *)
IF t_ON_MAX > t#0s THEN
	t3(in := q, pt := T_ON_MAX);
	Q := Q XOR t3.Q;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK F_LAMP
VAR_INPUT
	SWITCH : BOOL;
	DIMM : BYTE := 255;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	LAMP : BYTE;
	STATUS : BYTE;
END_VAR
VAR_IN_OUT
	ONTIME : UDINT;
	CYCLES : UDINT;
END_VAR
VAR_INPUT CONSTANT
	T_NO_DIMM : UINT := 100;
	T_MAINTENANCE : UINT := 15000;
END_VAR
VAR
	runtime : ONTIME;
END_VAR

runtime(in := switch, seconds := ontime, cycles := cycles);

IF RST THEN
	ONTIME := 0;
	CYCLES := 0;
END_IF;
IF SWITCH THEN
	IF ontime < T_NO_DIMM * 3600 THEN
		LAMP := 255;
		STATUS := 111;
	ELSE
		LAMP := DIMM;
		STATUS := 112;
	END_IF;
ELSE
	LAMP := 0;
	STATUS := 110;
END_IF;

IF ontime >= T_maintenance * 3600 AND T_Maintenance > 0 THEN
	status := 120;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK PULSE_LENGTH
VAR_INPUT
	in : BOOL;
END_VAR
VAR_OUTPUT
	short : BOOL;
	middle : BOOL;
	long : BOOL;
END_VAR
VAR_INPUT CONSTANT
	T_short : TIME := t#100ms;
	T_long : TIME := t#1s;
END_VAR
VAR
	tx : TIME;
	tn: TIME;
	edge : BOOL;
END_VAR

(* read system time *)
tx := DWORD_TO_TIME(T_PLC_MS());

(* reset outputs, they should only be active 1 cycle *)
short := FALSE;
Middle := FALSE;
long := FALSE;

(* detect rising edge *)
IF in AND NOT edge THEN
	edge := TRUE;
	tn := tx;

(* detect for falling edge *)
ELSIF NOT in AND edge THEN
	edge := FALSE;
	tn := tx - tn;
	IF tn < t_short THEN
		short := TRUE;
	ELSIF tn > t_long THEN
		long := TRUE;
	ELSE
		Middle := TRUE;
	END_IF;

(* generate long pulse as soon as T_long is reached *)
ELSIF in AND tx - tn > t_long THEN
	long := TRUE;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK PULSE_T
VAR_INPUT
	IN : BOOL;
	T1 : TIME;
	T2 : TIME;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	init: BOOL;
	last: TIME;
	tx: TIME;
	edge: BOOL;
END_VAR

tx := DWORD_TO_TIME(T_PLC_MS());
IF NOT init THEN
	init := TRUE;
	last := tx;
ELSIF RST THEN
	(* asynchronous reset *)
	Q := FALSE;
ELSIF IN AND NOT edge AND NOT Q THEN
	(* a rising edge on in will reversre the output status, if new pulse is startet the start time is stored in last *)
	last := tx;
	Q := TRUE;
ELSIF NOT IN AND edge AND ((tx - last) > T1) THEN
	(* a falling edge on IN will clear the output if in was high for more then T1 *)
	Q := FALSE;
ELSIF (tx - last) >= T2 THEN
	(* timeout for long pulse if second click did not occur or in stays high *)
	Q := FALSE;
END_IF;

edge := IN;
END_FUNCTION_BLOCK

FUNCTION_BLOCK SW_RECONFIG
VAR_INPUT
	IN : BOOL;
	TD : TIME;
	TR : TIME;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	T1, T2 : TON;
	INV: BOOL;
END_VAR

(* run debounce circuit *)
t1(in := IN, PT := TD);

IF tr > t#0s THEN
	(* generate output *)
	Q := t1.Q XOR inv;
	(* reconfiguration timer *)
	T2(in := Q, pt := TR);
	(* when T2.Q goes high then reverse INV *)
	IF T2.Q THEN INV := NOT INV; END_IF;
ELSE
	Q := t1.Q;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK SWITCH_I
VAR_INPUT
	set : BOOL;
	in : BOOL;
	rst : BOOL;
END_VAR
VAR_INPUT CONSTANT
	T_debounce : TIME := T#10ms;
	T_reconfig : TIME := T#1s;
	T_on_max : TIME := T#0h;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	state : BYTE;
	edge : BOOL;
	r_edge : BOOL;
	T_on : TIME;
	tx: TIME;
END_VAR

(* read system time *)
tx := DWORD_TO_TIME(T_PLC_MS());

(* asynchronous set and reset first *)
IF set AND NOT rst THEN
	Q := TRUE;
	T_on := tx;
ELSIF rst THEN
	Q := FALSE;
(* check FOR input edge AND start debounce timer *)
ELSIF (in XOR edge) AND NOT (state = 1) THEN
	state := 1;
	T_on := Tx;
(* edge was detected and debounce time elapsed *)
ELSIF (state = 1) AND (Tx - t_debounce >= T_on) THEN
	state := 2;
	(* react on rising or falling edge depending on r_edge *)
	IF r_edge XOR in THEN Q := NOT Q; END_IF;
(* wait for T_reconfig and adjust r_edge *)
ELSIF state = 2 AND Tx - t_reconfig >= T_on THEN
	r_edge := in;
END_IF;
IF Q AND T_on_max > t#0ms AND Tx >= T_on + T_on_max THEN
	Q := FALSE;
END_IF;
edge := in;
END_FUNCTION_BLOCK

FUNCTION_BLOCK SWITCH_X
VAR_INPUT
	IN1 : BOOL;
	IN2 : BOOL;
	IN3 : BOOL;
	IN4 : BOOL;
	IN5 : BOOL;
	IN6 : BOOL;
END_VAR
VAR_OUTPUT
	Q1 : BOOL;
	Q2 : BOOL;
	Q3 : BOOL;
	Q4 : BOOL;
	Q5 : BOOL;
	Q6 : BOOL;
	Q31 : BOOL;
	Q41 : BOOL;
	Q51 : BOOL;
	Q61 : BOOL;
	Q32 : BOOL;
	Q42 : BOOL;
	Q52 : BOOL;
	Q62 : BOOL;
END_VAR
VAR_INPUT CONSTANT
	t_debounce : TIME := t#50ms;
END_VAR
VAR
	init : BOOL;
	T1, T2, T3, T4, T5, T6 : TOF;
	tx: TIME;
	x1: BOOL;
	x2: BOOL;
	E1, E2 : BOOL;
END_VAR

(* initialize on startup *)
IF NOT init THEN
	init := TRUE;
	IF t_debounce < t#50ms THEN tx := t#50ms; ELSE tx := t_debounce; END_IF;
	T1(PT := Tx);
	T2(PT := Tx);
	T3(PT := Tx);
	T4(PT := Tx);
	T5(PT := Tx);
	T6(PT := Tx);
ELSE
	Q1 := FALSE;
	Q2 := FALSE;
	Q3 := FALSE;
	Q4 := FALSE;
	Q5 := FALSE;
	Q6 := FALSE;
	Q31 := FALSE;
	Q41 := FALSE;
	Q51 := FALSE;
	Q61 := FALSE;
	Q32 := FALSE;
	Q42 := FALSE;
	Q52 := FALSE;
	Q62 := FALSE;
END_IF;

(* read inputs and debounce *)
	T1(IN := IN1);
	T2(IN := IN2);
	T3(IN := IN3);
	T4(IN := IN4);
	T5(IN := IN5);
	T6(IN := IN6);

(* detect edge of IN1 and IN2 *)
IF t1.Q AND NOT E1 THEN X1 := TRUE; END_IF;
IF t2.Q AND NOT E2 THEN X2 := TRUE; END_IF;

IF t1.Q THEN
	IF t3.q THEN
		q31 := TRUE;
		X1 := FALSE;
	ELSIF t4.q THEN
		q41 := TRUE;
		X1 := FALSE;
	ELSIF t5.q THEN
		q51 := TRUE;
		X1 := FALSE;
	ELSIF t6.q THEN
		q61 := TRUE;
		X1 := FALSE;
	END_IF;
ELSIF t2.Q THEN
	IF t3.q THEN
		q32 := TRUE;
		X2 := FALSE;
	ELSIF t4.q THEN
		q42 := TRUE;
		X2 := FALSE;
	ELSIF t5.q THEN
		q52 := TRUE;
		X2 := FALSE;
	ELSIF t6.q THEN
		q62 := TRUE;
		X2 := FALSE;
	END_IF;
(* in1 was active alone *)
ELSIF NOT T1.Q AND E1 AND X1 THEN
	Q1 := TRUE;
	X1 := FALSE;
ELSIF NOT T2.Q AND E2 AND X2 THEN
	Q2 := TRUE;
	X2 := FALSE;
ELSIF T3.Q THEN
	Q3 := TRUE;
ELSIF T4.Q THEN
	Q4 := TRUE;
ELSIF T5.Q THEN
	Q5 := TRUE;
ELSIF T6.Q THEN
	Q6 := TRUE;
END_IF;

(* save state of in1 and in2 *)
E1 := T1.Q;
E2 := T2.Q;
END_FUNCTION_BLOCK

FUNCTION_BLOCK TIMER_1
VAR_INPUT
	E : BOOL := TRUE;
	DTI : DT;
	START : TOD;
	DURATION : TIME;
	DAY : BYTE := 2#0111_1111;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	stop : TIME;
END_VAR

IF E THEN
	stop := TOD_TO_TIME(START) + DURATION;
	IF stop > T#24h THEN stop := stop - T#24h; END_IF;
	Q := TIMECHECK(DT_TO_TOD(DTI), START, TIME_TO_TOD(stop)) AND (SHR(BYTE#128, DAY_OF_WEEK(DT_TO_DATE(DTI))) AND DAY) > BYTE#0;
ELSE
	Q := FALSE; (* clear output when e is not true *)
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK TIMER_2
VAR_INPUT
	DT_in : DT;
	start : TOD;
	duration : TIME;
	mode : BYTE;
	HOLIDAY : BOOL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	last_check : TIME;
	dat : DATE;
	daytime : TOD;
	activation: TIME;
	tx: TIME;
	enabled: BOOL;
	init: BOOL;
	wday : INT;
END_VAR

VAR RETAIN
	run_date : DATE;
END_VAR

(* read system time *)
tx := DWORD_TO_TIME(T_PLC_MS());

IF NOT init THEN
	init := TRUE;
	last_check := tx - t#100ms;
END_IF;

(* for performance resons exit if execution is not necessary *)
IF tx - last_check < T#100ms THEN RETURN; END_IF;

(* calculate temp variables *)
dat := DT_TO_DATE(dt_in);
daytime := DT_TO_TOD(dt_in);
wday := DAY_OF_WEEK(dat);

(* calculate if any action is necessary today *)
CASE mode OF
	1..7:	(* mondays to fridays *)
		enabled := wday = BYTE_TO_INT(mode);
	11:	(* every day *)
		enabled := TRUE;
	12..16:	(* every nth day *)
		enabled := (DATE_TO_DWORD(dat)/86400) MOD (mode - 10) = 0;
	20:	(* monday - friday *)
		enabled := wday <= 5;
	21:	(* saturday and sunday *)
		enabled := wday > 5;
	22: (* workdays only *)
		enabled := wday <= 5 AND NOT holiday;
	23:	(* holidays and weekends *)
		enabled := wday > 5 OR holiday;
	24:	(* holidays only *)
		enabled := holiday;
	25:	(* 1st day of the month *)
		enabled := DAY_OF_MONTH(dat) = 1;
	26:	(* last day of month *)
		enabled := DAY_OF_MONTH(dat + t#1d) = 1;
	27:	(* 31st of december *)
		enabled := DAY_OF_MONTH(dat) = 31 AND MONTH_OF_DATE(dat) = 12;
	28:	(* 1st of january *)
		enabled := DAY_OF_YEAR(dat) = 1;
ELSE
	enabled := FALSE;
END_CASE;

(* set output if necessary *)
IF enabled AND NOT Q AND daytime >= start AND run_date <> dat THEN
	Q := TRUE;
	activation := tx;
	run_date := dat;
ELSIF Q AND tx - activation >= duration THEN
	Q := FALSE;
END_IF;
last_check := Tx;
END_FUNCTION_BLOCK

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION TIMER_EVENT_DECODE : TIMER_EVENT
// VAR_INPUT
// 	EVENT : STRING(STRING_LENGTH);
// 	LANG :	INT;
// END_VAR
// VAR
// 	pos: INT;
// 	start : INT;
// 	stop: INT;
// 	pt : POINTER TO ARRAY[1..255] OF BYTE;
// 	step: INT;
// 	tmp : STRING(60);
// END_VAR
//
// (* a timer event is specified as follows:
// 	$typ;ch;day;start;duration;land;lor#
// $ 		= start of definition
// typ 	= type of definition	(number)
// channel	= channel number		(number)
// day 	= day number			(number)
// start	= start time			(time of day)
// duration= duration of event		(time)
// land	= mask for output AND	(8bit value)
// lor		= mask for ourput OR	(8bit value)
// #		= end of definition
// *)
//
// (* check for start and ending character *)
// IF (LEFT(event,1) <> '<') AND (RIGHT(event,1) <> '>') THEN RETURN; END_IF;
// stop := LEN(event);
// pt := ADR(event);
// start := 2;
//
// (* parse the string for next ; *)
// FOR pos := 2 TO stop DO
// 	IF pt^[pos] = 59 OR pt^[pos] = 62 THEN
// 		tmp := MID(event, pos - start, start);
// 		CASE step OF
// 			0:	(* read type *)
// 				TIMER_EVENT_DECODE.typ := FSTRING_TO_BYTE(tmp);
//
// 			1:	(* read channel *)
// 				TIMER_EVENT_DECODE.channel := FSTRING_TO_BYTE(tmp);
//
// 			2:	(* read day *)
// 				IF IS_CC(tmp,'0123456789abcdefABCDEF#') THEN
// 					TIMER_EVENT_DECODE.day := FSTRING_TO_BYTE(tmp);
// 				ELSIF TIMER_EVENT_DECODE.typ = 2 THEN
// 					TIMER_EVENT_DECODE.DAY := FSTRING_TO_WEEK(tmp, LANG);
// 				ELSE
// 					TIMER_EVENT_DECODE.DAY := INT_TO_BYTE(FSTRING_TO_WEEKDAY(tmp, LANG));
// 				END_IF;
//
// 			3:	(* read start *)
// 				TIMER_EVENT_DECODE.start := STRING_TO_TOD(tmp);
//
// 			4:	(* read duaration *)
// 				TIMER_EVENT_DECODE.duration := STRING_TO_TIME(tmp);
//
// 			5:	(* read land *)
// 				TIMER_EVENT_DECODE.land := FSTRING_TO_BYTE(tmp);
//
// 			6:	(* read lor *)
// 				TIMER_EVENT_DECODE.LOR := FSTRING_TO_BYTE(tmp);
// 		END_CASE;
// 		start := pos + 1;
// 		step := step + 1;
// 	END_IF;
// END_FOR;
// END_FUNCTION

// Changed for beedance: a date or time of day literal is written in full, as D#yyyy-mm-dd and TOD#hh:mm:ss.
FUNCTION_BLOCK TIMER_EXT
VAR_INPUT
	ENA : BOOL := TRUE;
	ON : BOOL;
	OFF : BOOL;
	MAN : BOOL;
	SWITCH : BOOL;
	DT_IN : DT;
	SUN_RISE : TOD;
	SUN_SET : TOD;
	HOLIDAY : BOOL;
END_VAR
VAR_INPUT CONSTANT
	T_DEBOUNCE : TIME := T#100ms;
	T_RISE_START : TIME;
	T_RISE_STOP : TIME;
	T_SET_START : TIME;
	T_SET_STOP : TIME;
	T_DAY_START : TOD;
	T_DAY_STOP : TOD;
	ENABLE_SATURDAY : BOOL;
	ENABLE_SUNDAY : BOOL;
	ENABLE_HOLIDAY : BOOL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	STATUS: BYTE;
END_VAR
VAR
	mx : MANUAL_2;
	deb : DEBOUNCE;
	tdx: TOD;
	wdx: INT;
	tc : TIME;
	tx: DWORD;
	tl: DWORD;
	qx: BOOL;
	init: BOOL;
END_VAR

(* l�sche die millisekunden bei sunrise und sunset *)
SUN_RISE := DWORD_TO_TOD(TOD_TO_DWORD(SUN_RISE) / 1000 * 1000);
SUN_SET := DWORD_TO_TOD(TOD_TO_DWORD(SUN_SET) / 1000 * 1000);

(* measure cycle time tc and make sure this circuitry is not called more then once in 200ms*)
tx := T_PLC_MS();
IF NOT init THEN
	init := TRUE;
	tl := tx;
END_IF;
tc := DWORD_TO_TIME(tx - tl);
IF tc < t#200ms THEN RETURN; END_IF;
tl := tx;

(* debounce *)
deb(in := SWITCH, TD := T_DEBOUNCE, PM := TRUE);

(* read time of day and strip off all milliseconds *)
tdx := DT_TO_TOD(dt_in);
wdx := DAY_OF_WEEK(DT_TO_DATE(dt_in));

(* automatic output control *)
IF deb.Q THEN
	qx := NOT qx;
	status := 110;
ELSIF holiday AND NOT enable_holiday THEN
	qx := FALSE;
ELSIF wdx = 6 AND NOT enable_saturday THEN
	qx := FALSE;
ELSIF wdx = 7 AND NOT enable_sunday THEN
	qx := FALSE;
ELSIF T_day_start > TOD#00:00:00 AND tdx - T_DAY_START <= tc THEN
	(* turn on at specified daytime *)
	qx := TRUE;
	status := 111;
ELSIF T_DAY_STOP > TOD#00:00:00 AND tdx - T_DAY_STOP <= tc THEN
	(* turn off at specific daytime *)
	qx := FALSE;
	status := 112;
ELSIF T_RISE_START > T#0s AND tdx - SUN_RISE + T_RISE_START <= tc THEN
	(* turn on before sunrise *)
	qx := TRUE;
	status := 113;
ELSIF T_RISE_STOP > T#0s AND tdx - SUN_RISE - T_RISE_STOP <= tc THEN
	(* turn off after sunrise *)
	qx := FALSE;
	status := 114;
ELSIF T_SET_start > T#0s AND tdx - SUN_SET + T_SET_START <= tc THEN
	(* turn on before sunset *)
	qx := TRUE;
	status := 115;
ELSIF T_SET_STOP > t#0s AND tdx - SUN_SET - T_SET_STOP <= tc THEN
	(* turn off after sunset *)
	qx := FALSE;
	status := 116;
END_IF;

(* mnaual and output control *)
mx(in := qx, ena := ENA, on := ON, off := OFF, man := MAN);
Q := mx.Q;

(* set the status output *)
IF mx.STATUS > 100 THEN status := mx.STATUS; END_IF;
END_FUNCTION_BLOCK

// Changed for beedance: a date or time of day literal is written in full, as D#yyyy-mm-dd and TOD#hh:mm:ss.
FUNCTION_BLOCK TIMER_P4
VAR_INPUT
	DTIME : DT;
	TREF_0 : TOD;
	TREF_1 : TOD;
	HOLY : BOOL;
	L0, L1, L2, L3 : BOOL;
	OFS : BYTE;
	ENQ : BOOL;
	MAN : BOOL;
	MI : BYTE;
	RST : BOOL;
END_VAR
VAR_IN_OUT
	PROG : ARRAY[0..array_max] OF TIMER_EVENT;
END_VAR
VAR_OUTPUT
	Q0, Q1, Q2, Q3 : BOOL;
	STATUS : BYTE;
END_VAR
VAR CONSTANT
	array_max : INT := 63;
	channel_max : INT := 3;
END_VAR
VAR
	day_start : DT;
	start : DT;
	event : TIMER_EVENT;
	pos : INT;
	last_execute: DT;
	current_day : DINT;
	mask: BYTE;
	ma : ARRAY[0..channel_max] OF BYTE;
	mo : ARRAY[0..channel_max] OF BYTE;
	qn : ARRAY[0..channel_max] OF BOOL;
	qs : ARRAY[0..channel_max] OF BOOL;
	channel: INT;
	tx: TOD;
END_VAR

(* for performance reasons we only execute the code at the beginning of a new second *)

IF RST THEN
	(* reset all events to last active on 1.1.1970 *)
	last_execute := DT#1970-01-01-00:00:00;
	FOR pos := 0 TO ARRAY_MAX DO
		IF (PROG[pos].CHANNEL >= OFS) AND (PROG[pos].CHANNEL < (OFS + 4)) THEN PROG[pos].LAST := last_execute; END_IF;
	END_FOR;

	(* reset all set markers and logic masks *)
	FOR pos := 0 TO channel_max DO
		qs[pos] := FALSE;
	END_FOR;

(* normal operation *)
ELSIF dtime <> last_execute THEN
	(* save the actual second for next execution test *)
	last_execute := DTIME;

	(* calculate dt for the start of the day to be used in comparisons *)
	day_start := DATE_TO_DT(DT_TO_DATE(DTIME));

	(* set all qn to FALSE *)
	FOR pos := 0 TO channel_max DO qn[pos] := FALSE; END_FOR;

	(* search the array PROG for timer events *)
	FOR pos := 0 TO ARRAY_MAX DO
		event := prog[pos];
		channel := event.CHANNEL - OFS;
		IF (event.TYP > 0) AND (channel >= 0) AND (channel <= channel_max) THEN
			ma[channel] := event.LAND;
			mo[channel] := event.LOR;
			CASE event.TYP OF
				1:	(* daily event *)
					start := day_start + TOD_TO_TIME(event.start);
					qn[channel] := dtime >= start AND dtime <= start + event.DURATION;

				2: 	(* event by selected weekdays *)
					IF (SHR(BYTE#128, DAY_OF_WEEK(DT_TO_DATE(Dtime))) AND event.DAY) > 0 THEN
						start := day_start + TOD_TO_TIME(event.start);
						qn[channel] := dtime >= start AND dtime <= start + event.DURATION;
					END_IF;

				3:	(* event every N days *)
					IF current_day MOD event.DAY = 0 THEN
						start := day_start + TOD_TO_TIME(event.start);
						qn[channel] := dtime >= start AND dtime <= start + event.DURATION;
					END_IF;

				10:	(* weekly event exactly once a week the weekday is specified in day, 1= mo *)
					IF DAY_OF_WEEK(DT_TO_DATE(DTIME)) = BYTE_TO_INT(event.DAY) THEN
						start := day_start + TOD_TO_TIME(event.start);
						qn[channel] := dtime >= start AND dtime <= start + event.DURATION;
					END_IF;

				20:	(* monthly event *)
					IF DAY_OF_MONTH(DT_TO_DATE(DTIME)) = BYTE_TO_INT(event.DAY) THEN
						start := day_start + TOD_TO_TIME(event.start);
						qn[channel] := dtime >= start AND dtime <= start + event.DURATION;
					END_IF;

				21:	(* event on last day of month *)
					IF DT_TO_DATE(DTIME) = MONTH_END(DT_TO_DATE(dtime)) THEN
						start := day_start + TOD_TO_TIME(event.start);
						qn[channel] := dtime >= start AND dtime <= start + event.DURATION;
					END_IF;

				30:	(* yearly event on a specific day specified in day *)
					IF DAY_OF_YEAR(DT_TO_DATE(DTIME)) = BYTE_TO_INT(event.DAY) THEN
						start := day_start + TOD_TO_TIME(event.start);
						qn[channel] := dtime >= start AND dtime <= start + event.DURATION;
					END_IF;

				31:	(* event on last day of year *)
					IF DT_TO_DATE(DTIME) = YEAR_END(YEAR_OF_DATE(DT_TO_DATE(dtime))) THEN
						start := day_start + TOD_TO_TIME(event.start);
						qn[channel] := dtime >= start AND dtime <= start + event.DURATION;
					END_IF;

				40:	(* event on leap days *)
					IF LEAP_DAY(DT_TO_DATE(dtime)) THEN
						start := day_start + TOD_TO_TIME(event.start);
						qn[channel] := dtime >= start AND dtime <= start + event.DURATION;
					END_IF;

				41: (* event on holidays *)
					IF HOLY THEN
						start := day_start + TOD_TO_TIME(event.start);
						qn[channel] := dtime >= start AND dtime <= start + event.DURATION;
					END_IF;

				42: (* event on holidays and weekends *)
					IF (Holy OR (DAY_OF_WEEK(DT_TO_DATE(dtime)) = 6) OR (DAY_OF_WEEK(DT_TO_DATE(dtime)) = 7)) THEN
						start := day_start + TOD_TO_TIME(event.start);
						qn[channel] := dtime >= start AND dtime <= start + event.DURATION;
					END_IF;

				43:	(* event on workdays Mo-FR and no holiday *)
					IF DAY_OF_WEEK(DT_TO_DATE(dtime)) < 6 THEN
						start := day_start + TOD_TO_TIME(event.start);
						qn[channel] := dtime >= start AND dtime <= start + event.DURATION;
					END_IF;

				50: (* event after reference time *)
					CASE prog[pos].DAY OF
						0:	tx := tref_0;
						1:	tx := tref_1;
					ELSE
						tx := TOD#00:00:00;
					END_CASE;
					start := day_start + TOD_TO_TIME(tx) + TOD_TO_TIME(event.start);
					qn[channel] := dtime >= start AND dtime <= start + event.DURATION;

				51:	(* event before reference time *)
					CASE prog[pos].DAY OF
						0:	tx := tref_0;
						1:	tx := tref_1;
					ELSE
						tx := TOD#00:00:00;
					END_CASE;
					start := day_start + TOD_TO_TIME(tx) - TOD_TO_TIME(event.start);
					qn[channel] := dtime >= start AND dtime <= start + event.DURATION;

				52 : (* set output at specific time + offset  *)
					CASE prog[pos].DAY OF
						0:	tx := tref_0;
						1:	tx := tref_1;
					ELSE
						tx := TOD#00:00:00;
					END_CASE;
					start := day_start + TOD_TO_TIME(tx) + TOD_TO_TIME(event.start);
					IF dtime >= start AND day_start > event.LAST THEN
						qs[channel] := TRUE;
						prog[pos].LAST := day_start;
					END_IF;

				53 : (* reset output at specific time + offset *)
					CASE prog[pos].DAY OF
						0:	tx := tref_0;
						1:	tx := tref_1;
					ELSE
						tx := TOD#00:00:00;
					END_CASE;
					start := day_start + TOD_TO_TIME(tx) + TOD_TO_TIME(event.start);
					IF dtime >= start AND day_start > event.LAST THEN
						qs[channel] := FALSE;
						prog[pos].LAST := day_start;
					END_IF;

				54 : (* set output at specific time - offset  *)
					CASE prog[pos].DAY OF
						0:	tx := tref_0;
						1:	tx := tref_1;
					ELSE
						tx := TOD#00:00:00;
					END_CASE;
					start := day_start + TOD_TO_TIME(tx) - TOD_TO_TIME(event.start);
					IF dtime >= start AND day_start > event.LAST THEN
						qs[channel] := TRUE;
						prog[pos].LAST := day_start;
					END_IF;

				55 : (* reset output at specific time - offset *)
					CASE prog[pos].DAY OF
						0:	tx := tref_0;
						1:	tx := tref_1;
					ELSE
						tx := TOD#00:00:00;
					END_CASE;
					start := day_start + TOD_TO_TIME(tx) - TOD_TO_TIME(event.start);
					IF dtime >= start AND day_start > event.LAST THEN
						qs[channel] := FALSE;
						prog[pos].LAST := day_start;
					END_IF;
			END_CASE;
		END_IF;
	END_FOR;
END_IF;

(* prepare the logical input mask *)
mask := 255;
mask.0 := L0;
mask.1 := L1;
mask.2 := L2;
mask.3 := L3;

(* set the outputs *)
Q0 := ENQ AND ((qn[0] OR qs[0]) AND ((ma[0] AND mask) = ma[0]) OR ((mo[0] AND mask) > 0) OR (man AND MI.0));
Q1 := ENQ AND ((qn[1] OR qs[1]) AND ((ma[1] AND mask) = ma[1]) OR ((mo[1] AND mask) > 1) OR (man AND MI.1));
Q2 := ENQ AND ((qn[2] OR qs[2]) AND ((ma[2] AND mask) = ma[2]) OR ((mo[2] AND mask) > 2) OR (man AND MI.2));
Q3 := ENQ AND ((qn[3] OR qs[3]) AND ((ma[3] AND mask) = ma[3]) OR ((mo[3] AND mask) > 3) OR (man AND MI.3));

(* set status *)
IF NOT enq THEN
	status := 100;
ELSIF man THEN
	status := 101;
ELSE
	status := 102;
END_IF;
END_FUNCTION_BLOCK

FUNCTION AIR_DENSITY : REAL
VAR_INPUT
	T : REAL;
	P : REAL;
	RH : REAL;
END_VAR
VAR CONSTANT
	RL : REAL := 287.05;
	RX : REAL := 3.773319E-3;
END_VAR

AIR_DENSITY := phys.PN * (1.0 - RH * SDD(T, TRUE) * RX / P) / (RL * ( T - phys.T0));
END_FUNCTION

FUNCTION AIR_ENTHALPY : REAL
VAR_INPUT
	T : REAL;
	RH : REAL;
END_VAR
VAR CONSTANT
	CPL : REAL := 1.00482;
	CW : REAL := 1.86;
	LH : REAL := 2500.78;
END_VAR

AIR_ENTHALPY := CPL * T + (CW * T + LH) * DEW_CON(RH,T);
END_FUNCTION

// Changed for beedance: a chained assignment, a := b := c, is b := c; a := b.
FUNCTION_BLOCK BOILER
VAR_INPUT
	T_upper : REAL;
	T_lower : REAL;
	Pressure : BOOL := TRUE;
	enable : BOOL := TRUE;
	Req_1 : BOOL;
	Req_2 : BOOL;
	Boost : BOOL;
END_VAR
VAR_INPUT CONSTANT
	t_upper_min : REAL := 50.0;
	t_upper_max : REAL := 60.0;
	T_lower_enable : BOOL;
	T_lower_max : REAL := 60.0;
	T_request_1 : REAL := 70.0;
	T_request_2 : REAL := 50.0;
	T_request_hys : REAL := 5.0;
	T_protect_high : REAL := 80.0;
	T_protect_low : REAL := 10.0;
END_VAR
VAR_OUTPUT
	Heat : BOOL;
	error : BOOL;
	status : BYTE;
END_VAR
VAR
	edge: BOOL;
	boost_mode: BOOL;
	flag_0 : BOOL;
	flag_1: BOOL;
	flag_2: BOOL;
END_VAR

(* read sensors and check for valid data *)
IF t_upper > t_protect_high THEN
	status := 1;
	heat := FALSE;
	error := TRUE;
ELSIF t_upper < t_protect_low THEN
	status := 2;
	heat :=	TRUE;
	error := TRUE;
ELSIF t_lower > T_protect_high AND t_lower_enable THEN
	status := 3;
	heat := FALSE;
	error := TRUE;
ELSIF t_lower < t_protect_low AND t_lower_enable THEN
	status := 4;
	heat := TRUE;
	error := TRUE;
ELSIF NOT pressure THEN
	status := 5;
	heat := FALSE;
	error := TRUE;
ELSIF req_1 OR req_2 OR enable OR boost THEN
	error := FALSE;

	(* determine if heat needs to be turned on *)
	IF boost AND NOT edge AND t_upper < t_upper_max THEN
		status := 101;
		heat := TRUE;
		boost_mode := TRUE;
	ELSIF enable AND t_upper < T_upper_min THEN
		status := 102;
		heat := TRUE;
	ELSIF req_1 AND t_upper < T_request_1 THEN
		status := 103;
		heat := TRUE;
	ELSIF req_2 AND t_upper < t_request_2 THEN
		status := 104;
		heat := TRUE;
	END_IF;

	(* determine the shut off temperature *)
	IF heat THEN
		IF (enable OR boost_mode) THEN
			flag_0 := TRUE;
			IF T_lower_enable AND T_lower > T_lower_max THEN
				boost_mode := FALSE; flag_0 := boost_mode;
			ELSIF NOT T_lower_enable AND T_upper > T_upper_max THEN
				boost_mode := FALSE; flag_0 := boost_mode;
			END_IF;
		ELSE
			flag_0 := FALSE;
		END_IF;
		flag_1 := (req_1 AND T_upper > T_request_1 + T_request_hys) AND req_1;
		flag_2 := (req_2 AND T_upper > T_request_2 + T_request_hys) AND req_2;

		(* shut off heat if no longer needed *)
		heat := flag_0 OR flag_1 OR flag_2;
		IF heat = FALSE THEN status := 100; END_IF;
	END_IF;
ELSE
	status := 100;
	heat := FALSE;
	error := FALSE;
END_IF;
edge := boost;
END_FUNCTION_BLOCK

FUNCTION_BLOCK BURNER
VAR_INPUT
	in : BOOL;
	stage2 : BOOL;
	over_temp : BOOL;
	oil_temp : BOOL := TRUE;
	Flame : BOOL;
	rst : BOOL;
	rst_timer : BOOL;
END_VAR
VAR_INPUT CONSTANT
	pre_heat_time : TIME := t#5s;
	pre_vent_time : TIME := t#15s;
	pre_ignite_time : TIME := t#5s;
	post_ignite_time : TIME := t#25s;
	stage2_delay : TIME := t#10s;
	safety_time : TIME := t#5s;
	lockout_time : TIME := t#10s;
	multiple_ignition : BOOL := TRUE;
	KW1 : REAL;
	KW2 : REAL;
END_VAR
VAR_OUTPUT
	motor : BOOL;
	coil1 : BOOL;
	coil2 : BOOL;
	pre_heat: BOOL;
	ignite : BOOL;
	KWh : REAL;
	status : BYTE;
	fail : BOOL;
END_VAR
VAR_IN_OUT
	runtime1 : UDINT;
	runtime2 : UDINT;
	cycles : UDINT;
END_VAR
VAR
	state : BYTE;
	last : TIME;
	tx: TIME;
	last_change: TIME;
	timer1 : ONTIME;
	timer2 : ONTIME;
	oil_temp_last: BOOL;
	cycles2 : UDINT;
END_VAR

(* read system time *)
tx := DWORD_TO_TIME(T_PLC_MS());

(* check rst input and overtemp *)
IF rst OR over_temp OR state = 0 THEN
	IF status > 0 AND tx - last_change >= lockout_time AND rst THEN
		status := 110;
		fail := FALSE;
		state := 1;
	ELSE
		(* normaler reset *)
		motor := FALSE;
		coil1 := FALSE;
		coil2 := FALSE;
		ignite := FALSE;
		pre_heat := FALSE;
		IF over_temp THEN
			status := 9;
			fail := TRUE;
		END_IF;
		last_change := tx;
		last := tx;
		state := 1;
	END_IF;
END_IF;

(* check for timer rst and rst timer if true *)
IF rst_timer THEN
	runtime1 := 0;
	runtime2 := 0;
	cycles := 0;
	cycles2 := 0;
END_IF;

(* quit here if an error is present *)
IF (status > 0 AND status < 100) OR rst THEN RETURN; END_IF;

(* start sequence *)
CASE state OF

1:	(* in signal starts oil pre heating *)
	IF in AND flame THEN
		state := 7;
		pre_heat := FALSE;
		status := 2;
		last_change := tx;
	ELSIF in THEN
		pre_heat := TRUE;
		state := 2;
		last_change := tx;
	END_IF;

2:	(* after pre_heating time start motor *)
	IF (tx- last_change >= pre_heat_time AND oil_temp) OR (oil_temp AND NOT oil_temp_last) THEN
		motor := TRUE;
		state := 3;
		last_change := tx;
	(* pre_heat_time ist abgelaufen und oil_temp ist nicht aktiv *)
	ELSIF tx - last_change >= pre_heat_time AND NOT oil_temp THEN
		state := 7;
		pre_heat := FALSE;
		status := 1;
		last_change := tx;
	(* flame monitor cannot be active at this time *)
	ELSIF flame THEN
		state := 7;
		pre_heat := FALSE;
		status := 2;
		last_change := tx;
	END_IF;

3:	(* abwarten bis z�ndung eingeschaltet werden kann *)
	IF tx - last_change >= pre_vent_time - pre_ignite_time THEN
		ignite := TRUE;
		state := 4;
		last_change := tx;
	(* flame monitor cannot be active at this time *)
	ELSIF flame THEN
		state := 7;
		pre_heat := FALSE;
		motor := FALSE;
		status := 3;
		last_change := tx;
	END_IF;

4:	(* warten bis oelzufuhr geoeffnet werden darf *)
	IF tx - last_change >= pre_ignite_time THEN
		coil1 := TRUE;
		state := 5;
		last_change := tx;
	END_IF;

5:	(* warten auf flammwaechter und falls noetig abschalten *)
	IF tx - last_change >= safety_time OR flame THEN
		IF NOT flame THEN
			(* notabschaltung da flammwaechster nicht angesprochen hat *)
			state := 7;
			motor := FALSE;
			coil1 := FALSE;
			pre_heat := FALSE;
			ignite := FALSE;
			status := 4;
			last_change := tx;
		ELSE
			state := 6;
			last_change := tx;
		END_IF;
	END_IF;

6:	(* brenner l�uft, flammueberwachung und nach ablauf der nachigniteszeit ignite abschalten *)
	IF NOT flame AND NOT multiple_ignition THEN
		(* notabschaltung da flammwaechster keine flamme meldet *)
		state := 7;
		motor := FALSE;
		coil1 := FALSE;
		coil2 := FALSE;
		pre_heat := FALSE;
		ignite := FALSE;
		status := 5;
		last_change := tx;
	ELSIF NOT flame AND multiple_ignition THEN
		ignite := TRUE;
		state := 5;
		coil2 := FALSE;
		last_change := tx;
	ELSE
		IF tx - last_change >= post_ignite_time THEN
			(* post_ignite_time abgelaufen, ignite abschalten *)
			ignite := FALSE;
		END_IF;
		IF tx - last_change >= stage2_delay AND stage2 THEN
			coil2 := TRUE;
		ELSE
			coil2 := FALSE;
		END_IF;
	END_IF;
END_CASE;

(* abschaltung wenn kein eingangssignal *)
IF NOT in THEN
	state := 1;
	motor := FALSE;
	coil1 := FALSE;
	coil2 := FALSE;
	ignite := FALSE;
	pre_heat := FALSE;
	last_change := tx;
END_IF;

(* runtimez�hler *)
timer1(in := flame AND in AND motor AND coil1 AND NOT coil2, seconds := runtime1, cycles := cycles);
timer2(in := flame AND in AND motor AND coil1 AND coil2, seconds := runtime2, cycles := cycles2);
KWH := UDINT_TO_REAL(runtime1) * KW1 / 3600.0 + UDINT_TO_REAL(runtime2) * KW2 / 3600.0;

(* zeit fuer naechsten aufruf merken *)
last := tx;

(* set fail output IF ERROR and Status  if normal operation *)

IF status > 0 AND status < 100 THEN
	fail := TRUE;
ELSE
	fail := FALSE;
	IF NOT in THEN
		status := 110;
	ELSIF flame AND in AND motor AND coil2 AND coil1 THEN
		status := 113;
	ELSIF flame AND in AND motor AND coil1 THEN
		status := 112;
	ELSE
		status := 111;
	END_IF;
END_IF;
END_FUNCTION_BLOCK

FUNCTION DEW_CON : REAL
VAR_INPUT
	RH : REAL;
	T : REAL;
END_VAR

(*	this code is good to read but need to be improoved
	SDD := 6.1078*EXPT(10,(a*T)/(b+T));
	DD := RH/100 * SDD;
	V := LOG(DD/6.1078);
	TD := b*V/(a-v);
	dew_con := 0.1 * mw / RU * DD / (T - TK);
*)
IF RH > 0.0 AND T > -50.0 THEN
	DEW_CON := 2.166824303E-2 * RH * SDD(T,FALSE)  / (T - phys.T0);
ELSE
	DEW_CON := 0.0;
END_IF;

(*
Bezeichnungen:
r = relative Luftfeuchte
T = Temperatur in �C
TK = Temperatur in Kelvin (TK = T + 273.15)
TD = Taupunkttemperatur in �C
DD = Dampfdruck in hPa
SDD = S�ttigungsdampfdruck in hPa

Parameter:
a = 7.5, b = 237.3 f�r T >= 0
a = 7.6, b = 240.7 f�r T < 0 �ber Wasser (Taupunkt)
a = 9.5, b = 265.5 f�r T < 0 �ber Eis (Frostpunkt)

R* = 8314.3 J/(kmol*K) (universelle Gaskonstante)
mw = 18.016 kg (Molekulargewicht des Wasserdampfes)
AF = absolute Feuchte in g Wasserdampf pro m3 Luft

Formeln:

   1. SDD(T) = 6.1078 * 10^((a*T)/(b+T))
   2. DD(r,T) = r/100 * SDD(T)
   3. r(T,TD) = 100 * SDD(TD) / SDD(T)
   4. TD(r,T) = b*v/(a-v) mit v(r,T) = log10(DD(r,T)/6.1078)
   5. AF(r,TK) = 10^5 * mw/R* * DD(r,T)/TK; AF(TD,TK) = 10^5 * mw/R* * SDD(TD)/TK

*)
END_FUNCTION

FUNCTION DEW_RH : REAL
VAR_INPUT
	VC : REAL;
	T : REAL;
END_VAR

DEW_RH := LIMIT(0.0, VC / DEW_CON(1.0, T), 100.0);
END_FUNCTION

FUNCTION DEW_TEMP : REAL
VAR_INPUT
	RH : REAL;
	T : REAL;
END_VAR
VAR CONSTANT
	a : REAL := 7.5;
	b : REAL := 237.3;
END_VAR
VAR
	V : REAL;
END_VAR

IF rh > 0.0 THEN
(*	old code was rewritten for better performance
	SDD := 6.1078*EXPT(10,(a*T)/(b+T));
	DD := RH/100 * SDD;
	V := LOG(DD/6.1078);
	dew_temp := b*V/(a-v);
*)
	V := LOG(RH * 0.01 * EXP10((a*T) / (b+T)));
	DEW_TEMP := b*V / (a-V);
ELSE
	DEW_TEMP:= phys.T0;
END_IF;

(*
Bezeichnungen:
r = relative Luftfeuchte
T = Temperatur in �C
TK = Temperatur in Kelvin (TK = T + 273.15)
TD = Taupunkttemperatur in �C
DD = Dampfdruck in hPa
SDD = S�ttigungsdampfdruck in hPa

Parameter:
a = 7.5, b = 237.3 f�r T >= 0
a = 7.6, b = 240.7 f�r T < 0 �ber Wasser (Taupunkt)
a = 9.5, b = 265.5 f�r T < 0 �ber Eis (Frostpunkt)

R* = 8314.3 J/(kmol*K) (universelle Gaskonstante)
mw = 18.016 kg (Molekulargewicht des Wasserdampfes)
AF = absolute Feuchte in g Wasserdampf pro m3 Luft

Formeln:

   1. SDD(T) = 6.1078 * 10^((a*T)/(b+T))
   2. DD(r,T) = r/100 * SDD(T)
   3. r(T,TD) = 100 * SDD(TD) / SDD(T)
   4. TD(r,T) = b*v/(a-v) mit v(r,T) = log10(DD(r,T)/6.1078)
   5. AF(r,TK) = 10^5 * mw/R* * DD(r,T)/TK; AF(TD,TK) = 10^5 * mw/R* * SDD(TD)/TK

*)
END_FUNCTION

FUNCTION HEAT_INDEX : REAL
VAR_INPUT
	T : REAL;
	RH : REAL;
END_VAR
VAR
	RH2 : REAL;
	T2 : REAL;
END_VAR

IF RH < 20.0 OR T < 20.0 THEN
	HEAT_INDEX := T;
ELSE
	RH2 := RH * RH;
	T := C_TO_F(T);
	T2 := T * T;
	HEAT_INDEX := -42.379 + 2.04901523 * T - 6.83783E-3 * T2
			+ RH * (10.1433127 - 0.22475541 * T + 1.22874E-3 * T2)
			+ RH2 * (8.5282E-4 * T - 5.481717E-2 - 1.99E-6 * T2);
	HEAT_INDEX := F_TO_C(HEAT_INDEX);
END_IF;
END_FUNCTION

FUNCTION_BLOCK HEAT_METER
VAR_INPUT
	TF, TR : REAL;
	LPH : REAL;
	E : BOOL;
	RST : BOOL;
END_VAR
VAR_INPUT CONSTANT
	CP : REAL;
	Density : REAL;
	Content : REAL;
	Pulse_Mode : BOOL;
	Return_meter : BOOL;
	AVG_time : TIME := T#5s;
END_VAR
VAR_OUTPUT
	C : REAL;
END_VAR
VAR_IN_OUT
	Y : REAL;
END_VAR
VAR
	tx, last : DWORD;
	int1 : FT_INT2;
	edge : BOOL;
	x: REAL;
	init: BOOL;
	y_last: REAL;
END_VAR

IF rst THEN
	int1(rst := TRUE);
	int1.rst := FALSE;
	C := 0.0;
	Y := 0.0;
ELSIF e THEN
	X := (WATER_DENSITY(SEL(return_meter, TF, TR), FALSE) * (WATER_ENTHALPY(TF) - WATER_ENTHALPY(TR)) * (1.0 - content) + CP * Density * content * (TF-TR));
END_IF;

(* integrate or add consumption *)
int1(run := NOT pulse_mode AND e, in := X * LPH * 2.77777777777E-4);
IF pulse_mode THEN
	IF NOT edge AND E THEN Y := Y + X * LPH; END_IF;
ELSE
	Y := int1.Out;
END_IF;

(* store the value of e *)
edge := e;

(* read system_time *)
tx := T_PLC_MS();

(* only init at startup necessary *)
IF NOT init THEN
	init := TRUE;
	last := tx;
END_IF;

(* calculate the current consumption *)
IF (tx - last >= TIME_TO_DWORD(AVG_TIME)) AND (avg_time > T#0s) THEN
	last := tx;
	C := (Y - Y_last) * 3.6E6 / DWORD_TO_REAL(TIME_TO_DWORD(AVG_TIME));
	Y_last := Y;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK HEAT_TEMP
VAR_INPUT
	T_EXT : REAL;
	T_INT : REAL;
	OFFSET : REAL;
	T_REQ : REAL;
END_VAR
VAR_INPUT CONSTANT
	TY_MAX : REAL := 70.0;
	TY_MIN : REAL := 25.0;
	TY_CONFIG : REAL := 70.0;
	T_INT_CONFIG : REAL := 20.0;
	T_EXT_CONFIG : REAL := -15.0;
	T_DIFF : REAL := 10.0;
	C : REAL := 1.33;
	H : REAL := 3.0;
END_VAR
VAR_OUTPUT
	TY : REAL;
	HEAT : BOOL;
END_VAR
VAR
	tr : REAL;
	tx: REAL;
END_VAR

tr := T_INT + OFFSET;
tx := (tr - T_EXT) / (T_INT_CONFIG - T_EXT_CONFIG);

IF T_EXT + H > tr THEN
	TY := 0.0;
ELSE
	TY := LIMIT(TY_MIN, tr + T_DIFF * 0.5 * tx + (TY_CONFIG - T_DIFF * 0.5 - tr) * EXPT(tx, 1.0 / C), TY_MAX);
END_IF;

TY := MAX(TY, T_REQ);
HEAT := TY > 0.0;

(*

tv=ti+(tvmax-ti)*((ti-tau)/(ti-taumin))^(1/n)

tv aktuelle Vorlauftemp.
ti Raumtemperatur innen (20�C)
tau aktuelle Au�entemperatur
taumin minimale Au�entemperatur -12�C oder -15�C
n Heizk�rperexponent (meistens 1,33)
Fu�bodenheizung = 1,1
Wandheizung 1,3

Normen z. B. DIN 4703, DIN 4725 oder die Literatur, z. B Recknagel Abschnitt 1.3.5.

*)
END_FUNCTION_BLOCK

FUNCTION_BLOCK LEGIONELLA
VAR_INPUT
	manual : BOOL;
	temp_boiler : REAL;
	temp_return : REAL := 100.0;
	DT_in: DT;
	rst : BOOL;
END_VAR
VAR_INPUT CONSTANT
	T_start: TOD := tod#03:00:00;
	day : INT := 7;
	temp_set : REAL := 70.0;
	temp_offset : REAL := 10.0;
	temp_hys : REAL := 5.0;
	t_max_heat: TIME := t#10m;
	t_max_ret : TIME := t#10m;
	tp_0: TIME := t#5m;
	TP_1: TIME := t#5m;
	tp_2: TIME := t#5m;
	tp_3: TIME := t#5m;
	tp_4: TIME := t#5m;
	tp_5: TIME := t#5m;
	tp_6: TIME := t#5m;
	tp_7: TIME := t#5m;
END_VAR
VAR_OUTPUT
	Heat : BOOL;
	pump : BOOL;
	valve0 : BOOL;
	valve1 : BOOL;
	valve2 : BOOL;
	valve3 : BOOL;
	valve4 : BOOL;
	valve5 : BOOL;
	valve6 : BOOL;
	valve7 : BOOL;
	run: BOOL;
	Status : BYTE;
END_VAR
VAR
	X1: TIMER_1;
	x2: SEQUENCE_8;
	x3: HYST_1;
	init: BOOL;
END_VAR

(* startup initialization *)
IF NOT init THEN
	init := TRUE;
	X1.day := SHR(BYTE#128,day);
	X1.start := T_start;
	X3.low := Temp_offset + temp_set;
	X3.high := Temp_hys + X3.low;
	X2.wait0 := T_max_heat;
	X2.delay0 := TP_0;
	X2.delay1 := TP_1;
	X2.delay2 := TP_2;
	X2.delay3 := TP_3;
	X2.delay4 := TP_4;
	X2.delay5 := TP_5;
	X2.delay6 := TP_6;
	X2.delay7 := TP_7;
	X2.wait1 := T_max_ret;
	X2.wait2 := T_max_ret;
	X2.wait3 := T_max_ret;
	X2.wait4 := T_max_ret;
	X2.wait5 := T_max_ret;
	X2.wait6 := T_max_ret;
	X2.wait7 := T_max_ret;
	X2();
END_IF;

(* oerational code *)
X1(DTi := DT_in);
IF X1.Q OR MANUAL OR x2.run THEN
	X3(in := temp_boiler);
	X2.in0 := X3.Q OR x3.win;
	X2.in1 := temp_return >= temp_set;
	X2.in2 := x2.in1;
	X2.in3 := x2.in1;
	X2.in4 := x2.in1;
	X2.in5 := x2.in1;
	X2.in6 := x2.in1;
	X2.in7 := x2.in1;
	X2.rst := rst;
	X2(start := X1.Q OR MANUAL);
	run := x2.run;
	pump := x2.qx;
	Heat := NOT X3.Q AND x2.run;
	valve0 := X2.Q0;
	valve1 := X2.Q1;
	valve2 := X2.Q2;
	valve3 := X2.Q3;
	valve4 := X2.Q4;
	valve5 := X2.Q5;
	valve6 := X2.Q6;
	valve7 := X2.Q7;
	pump := X2.qx;
	status := X2.status;
ELSE;
	X2(start :=0);
	status := x2.status;
END_IF;
END_FUNCTION_BLOCK

FUNCTION SDD : REAL
VAR_INPUT
	T : REAL;
	ICE: BOOL;
END_VAR

IF ice THEN
	SDD := 611.153 * EXP(22.4433 * T / (272.186 + T));
ELSE
	SDD := 611.213 * EXP(17.5043 * T / (241.2 + T));
END_IF;
END_FUNCTION

FUNCTION SDD_NH3 : REAL
VAR_INPUT
	T : REAL;
END_VAR

(*
calculate according to NIST
Antoine Equation Parameters
log10(P) = A - (B / (T + C))
	P = vapor pressure (bar)
	T = temperature (K)
Temperature (K)	A	B	C	Reference	Comment

 T = 164.0 - 239.5, A =	3.18757, B =506.713, C=-80.78	Stull, 1947	Coefficents calculated by NIST from author's data.
 T = 239.5 - 371.4, A =	4.86886, B = 1113.928, C = -10.409	Stull, 1947	Coefficents calculated by NIST from author's data.
*)

IF T < -33.65 THEN
	SDD_NH3 := EXP(7.3396511649 - ( 1166.7498002 / ( T + 192.37 )));
ELSE
	SDD_NH3 := EXP(11.210964456 - ( 2564.9140075 / ( T + 262.741 )));
END_IF;
END_FUNCTION

FUNCTION SDT_NH3 : REAL
VAR_INPUT
	P : REAL;
END_VAR

(*
calculate according to NIST
Antoine Equation Parameters
log10(P) = A - (B / (T + C))
	P = vapor pressure (bar)
	T = temperature (K)
Temperature (K)	A	B	C	Reference	Comment

 T = 164.0 - 239.5, A =	3.18757, B =506.713, C=-80.78	Stull, 1947	Coefficents calculated by NIST from author's data.
 T = 239.5 - 371.4, A =	4.86886, B = 1113.928, C = -10.409	Stull, 1947	Coefficents calculated by NIST from author's data.
*)

IF P < 1.0E-3 THEN
	SDT_NH3 := -110.0;
ELSIF P < 1.0 THEN
	SDT_NH3 := 506.713 / (3.18757 - LOG(P)) - 192.37 ;
ELSE
	SDT_NH3 := 1113.928 / (4.86886 - LOG(P)) - 262.71 ;
END_IF;
END_FUNCTION

FUNCTION_BLOCK T_AVG24
VAR_INPUT
	TS : INT;
	DTI : DT;
	RST : BOOL;
END_VAR
VAR_INPUT CONSTANT
	T_FILTER : TIME := T#10m;
	SCALE : REAL := 1.0;
	OFS : REAL;
END_VAR
VAR_OUTPUT
	TA : REAL;
	TP : BOOL;
END_VAR
VAR_IN_OUT
	T24 : REAL;
	T24_MAX : REAL;
	T24_MIN : REAL;
END_VAR
VAR
	samples : ARRAY[0..47] OF INT;
	pos : INT;
	init: BOOL;
	ft1 : FILTER_I;
	sum : DINT;
	last: DT;
END_VAR
VAR_TEMP
	tmp_max : INT;
	tmp_min : INT;
	i: INT;
END_VAR

(* first filter the sensor readings *)
ft1(X := TS, T := T_FILTER);

IF rst OR NOT init THEN
	init := TRUE;
	IF T24 = -1000.0 THEN t24 := INT_TO_REAL(ft1.Y) * 0.1; END_IF;
	FOR pos := 0 TO 47 DO
		samples[pos] := REAL_TO_INT(T24 * 10.0);
	END_FOR;
	pos := 0;
	sum := INT_TO_DINT(samples[0]) * 48;
	(* calculate the output value *)
	TA := (INT_TO_REAL(ft1.Y) * 0.1 + OFS) * SCALE;
	T24 := (DINT_TO_REAL(sum) * 0.00208333333333 + OFS) * SCALE;
	tp := TRUE;
ELSIF DT_TO_DWORD(dti) / 60 MOD 30 = 0 AND DTI > last THEN
	(* make sure we onlyexecute once every 30 mins *)
	last := DTI + T#1m;
	(* sample the temperature every 30 mins and store in ring buffer samples *)
	(* sum always holds the sum of the array *)
	(* subtract the oldest sample from sum *)
	sum := sum - INT_TO_DINT(samples[pos]);
	(* strore the new sample *)
	samples[pos] := ft1.Y;
	(* add the new sample to sum *)
	sum := sum + INT_TO_DINT(samples[pos]);
	(* increment the counter for the next sample *)
	pos := INC1(pos, 48);
	(* calculate the output value *)
	TA := (INT_TO_REAL(ft1.Y) * 0.1 + OFS) * SCALE;
	T24 := (DINT_TO_REAL(sum) * 0.00208333333333 + OFS) * SCALE;
	(* calculate the min and max over the last 24 hours *)
	tmp_max := -32000;
	tmp_min := 32000;
	FOR i := 0 TO 47 DO
		tmp_max := MAX(tmp_max, samples[i]);
		tmp_min := MIN(tmp_min, samples[i]);
	END_FOR;
	T24_MAX := (INT_TO_REAL(tmp_max) * 0.1 + OFS) * SCALE;
	T24_MIN := (INT_TO_REAL(tmp_min) * 0.1 + OFS) * SCALE;
	tp := TRUE;
ELSE
	tp := FALSE;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK TANK_LEVEL
VAR_INPUT
	LEVEL : BOOL;
	LEAK : BOOL;
	ACLR : BOOL;
END_VAR
VAR_INPUT CONSTANT
	MAX_VALVE_TIME : TIME;
	LEVEL_DELAY_TIME : TIME;
END_VAR
VAR_OUTPUT
	VALVE : BOOL;
	ALARM : BOOL;
	STATUS : BYTE;
END_VAR
VAR
	cx : ACTUATOR_COIL;
	tn : TON;
	tl : TONOF;
	open: BOOL;
END_VAR

(* preprocess the level information *)
tl(in := level, T_ON := level_delay_time, T_OFF := level_delay_time);
open := tl.Q;

(* start logic *)
IF ALARM THEN
	(* check for ACLR if ALARM is present *)
	IF ACLR THEN
		ALARM := FALSE;
		STATUS := 101; (* aclr pressed *)
		cx(in := FALSE);
	END_IF;
	RETURN;
ELSIF LEAK THEN
	(* leakeage detected *)
	cx(in := FALSE);
	ALARM := TRUE;
	STATUS := 1;	(* leakeage error *)
ELSIF open THEN
	(* valve needs to be opened because level is too low *)
	cx(in := TRUE);
	STATUS := 102; (* valve open by low level *)
ELSE
	(* valve needs to be closed *)
	cx(in := FALSE);
	STATUS := 100; (* valve idle *)
END_IF;

(* check if valve is open too long and generate alarm if necessary *)
tn(in := cx.out AND (MAX_VALVE_TIME > T#0s), PT := MAX_VALVE_TIME);
IF tn.Q THEN
	ALARM := TRUE;
	STATUS := 2; (* overtime error *)
	cx(in := FALSE);
END_IF;

(* set output signal *)
VALVE := cx.out;
END_FUNCTION_BLOCK

FUNCTION TANK_VOL1 : REAL
VAR_INPUT
	TR : REAL;
	TL : REAL;
	H : REAL;
END_VAR

TANK_VOL1 := CIRCLE_SEG(TR, H) * TL;
END_FUNCTION

FUNCTION TANK_VOL2 : REAL
VAR_INPUT
	TR : REAL;
	H : REAL;
END_VAR

TANK_VOL2 := math.PI * H * H * (TR - H/3.0);
END_FUNCTION

// Changed for beedance: a date or time of day literal is written in full, as D#yyyy-mm-dd and TOD#hh:mm:ss.
FUNCTION_BLOCK TEMP_EXT
VAR_INPUT
	t_ext1 : REAL;
	t_ext2 : REAL;
	t_ext3 : REAL;
	t_ext_config : BYTE;
	dt_in : DT;
END_VAR
VAR_INPUT CONSTANT
	t_ext_min : REAL := -40.0;
	t_ext_max : REAL := 60.0;
	t_ext_default : REAL := -10.0;
	heat_period_start : DATE := D#1970-09-01;
	heat_period_stop : DATE := D#1970-04-30;
	cool_period_start : DATE := D#1970-04-01;
	cool_period_stop : DATE := D#1970-09-30;
	heat_start_temp_day : REAL := 15.0;
	heat_start_temp_night : REAL := 10.0;
	heat_stop_temp : REAL := 18.0;
	cool_start_temp_day : REAL := 26.0;
	cool_start_temp_night : REAL := 26.0;
	cool_stop_temp : REAL := 24.0;
	start_day : TOD := tod#09:00:00;
	start_night : TOD := tod#21:00:00;
	CYCLE_TIME : TIME := t#10m;
END_VAR
VAR_OUTPUT
	t_ext : REAL;
	heat : BOOL;
	cool : BOOL;
END_VAR
VAR
	tx: TIME;
	last_run: TIME;
	init: BOOL;
	cool_start: DATE;
	cool_stop: DATE;
	heat_start: DATE;
	heat_stop: DATE;
	xdate: DATE;
	day: BOOL;
END_VAR

IF NOT init THEN
	init := TRUE;
	heat_start := SET_DATE(1972,MONTH_OF_DATE(heat_period_start),DAY_OF_MONTH(heat_period_start));
	heat_stop := SET_DATE(1972,MONTH_OF_DATE(heat_period_stop),DAY_OF_MONTH(heat_period_stop));
	cool_start := SET_DATE(1972,MONTH_OF_DATE(cool_period_start),DAY_OF_MONTH(cool_period_start));
	cool_stop := SET_DATE(1972,MONTH_OF_DATE(cool_period_stop),DAY_OF_MONTH(cool_period_stop));
END_IF;

(* read system time *)
tx := DWORD_TO_TIME(T_PLC_MS());

(* for efficiency exit isf this program has been executed less then 1min ago *)
IF tx - last_run < cycle_time THEN RETURN; END_IF;

(* calculate frequently used variables *)
xdate := SET_DATE(1972,MONTH_OF_DATE(DT_TO_DATE(dt_in)),DAY_OF_MONTH(DT_TO_DATE(dt_in)));
day := DT_TO_TOD(dt_in) >= start_day AND  DT_TO_TOD(dt_in) < start_night;

(* read the correct external temperature depending on the t_ext_config setting *)
t_ext := MULTI_IN(t_ext1, t_ext2, t_ext3, t_ext_default, t_ext_min, t_ext_max, t_ext_config);

(* dtermine heating *)
IF (heat_start <= heat_stop AND xdate >= heat_start AND xdate <= heat_stop) OR (heat_start > heat_stop AND (xdate >= heat_start OR xdate <= heat_stop)) THEN
	(* heating period id true check for temperature *)
	IF day AND t_ext <= heat_start_temp_day THEN
		heat := TRUE;
	ELSIF NOT day AND t_ext <= heat_start_temp_night THEN
		heat := TRUE;
	ELSIF t_ext >= heat_stop_temp THEN
		heat := FALSE;
	END_IF;
ELSE
	heat := FALSE;
END_IF;

(* dtermine cooling *)
IF (cool_start <= cool_stop AND xdate >= cool_start AND xdate <= cool_stop) OR (cool_start > cool_stop AND (xdate >= cool_start OR xdate <= cool_stop)) THEN
	(* cooling period is true check for temperature *)
	IF day AND t_ext >= cool_start_temp_day THEN
		cool := TRUE;
	ELSIF NOT day AND t_ext >= cool_start_temp_night THEN
		cool := TRUE;
	ELSIF t_ext <= cool_stop_temp THEN
		cool := FALSE;
	END_IF;
ELSE
	cool := FALSE;
END_IF;

last_run := tx;
END_FUNCTION_BLOCK

FUNCTION WATER_CP : REAL
VAR_INPUT
	T : REAL;
END_VAR
VAR
	Pts : INT := 10;
	data : ARRAY[1..20, 0..1] OF REAL := 0.0,4.228, 5.0,4.20, 10.0,4.188, 15.0,4.184, 50.0,4.181, 60.0,4.183, 70.0,4.187, 80.0,4.194, 90.0,4.204, 100.0,4.22;
END_VAR

WATER_CP := LINEAR_INT(T, Data, Pts);

(*
    �C - die Celsius-Temperatur in Grad Celsius
    v � das spezifische Volumen in Kubikdezimeter je Kilogramm dm�/kg
    h � die spezifische Enthalpie in Kilojoule je Kilogramm 	kJ/kg
    u � die spezifische Innere Energie in Kilojoule je Kilogramm 	kJ/kg
    s � die spezifische Entropie in Kilojoule je Kilogramm mal Kelvin 	kJ/(kg�K)
    cp - die spezifische W�rmekapazit�t bei konstantem Druck in Kilojoule je Kilogramm mal Kelvin 	kJ/(kg�K)
    Y � Volumenausdehnungskoeffizient in 10-3 durch Kelvin  10-3/K
    L � W�rmeleitf�higkeit in Milliwatt je Meter mal Kelvin  mW / (m�K)
    n � Viskosit�t in Mikropascal mal Sekunde uPa�s
    s ? � Oberfl�chenspannung in Millinewton je Meter  mN/m

�C 		v 				h 			u 		s 			cp 		Y 		L		n		s
-20		1,006580
0 		1,000160 		0,06 		-0,04 	-0,0001 	4,228 	-0,080 	561,0 	1792 	75,65
4		1,000028
5 		1,0000 		21,1 		21,0 	0,076 		4,200 	0,011 	570,6 	1518 	74,95
10 		1,0003 		42,1 		42,0 	0,151 		4,188 	0,087 	580,0 	1306 	74,22
15 		1,0009 		63,0 		62,9 	0,224 		4,184 	0,152 	589,4 	1137 	73,49
20 		1,001797 		83,9 		83,8 	0,296 		4,183 	0,209 	598,4 	1001 	72,74
25 		1,0029 		104,8 		104,7 	0,367 		4,183 	0,259 	607,2 	890,4 	71,98
30 		1,0044 		125,8 		125,7 	0,437 		4,183 	0,305 	615,5 	797,7 	71,20
35 		1,0060 		146,7 		146,6 	0,505 		4,183 	0,347 	623,3 	719,6 	70,41
40 		1,007842 		167,58 	167,5 	0,572 		4,182 	0,386 	630,6 	653,3 	69,60
45 		1,0099 		188,5 		188,4 	0,638 		4,182 	0,423 	637,3 	596,3 	68,78
50 		1,0121 		209,4 		209,3 	0,704 		4,181 	0,457 	643,6 	547,1 	67,95
60 		1,017089 		251,2 		251,1 	0,831 		4,183 	0,522 	654,4 	466,6 	66,24
70 		1,0227 		293,1 		293,0 	0,955 		4,187 	0,583 	663,1 	404,1 	64,49
80 		1,029027 		335,0 		334,9 	1,075 		4,194 	0,640 	670,0 	354,5 	62,68
90 		1,0359 		377,0 		376,9 	1,193 		4,204 	0,696 	675,3 	314,6 	60,82
99,63 	1,0431 		417,5 		417,4 	1,303 		4,217 	0,748 	679,0 	283,0 	58,99
100	1,043453		419,1

Stoffdaten von Wasser Quelle Wickipedia

Wasser hat unter Normaldruck seine gr��te Dichte von 1000 Kilogramm pro Kubikmeter bei 3,98 �C und zeigt damit eine Dichteanomalie.
Diese besteht darin, dass sich Wasser unterhalb von 3,98 �C bei weiterer Temperaturverringerung, auch beim Wechsel zum festen
Aggregatzustand, wieder ausdehnt, was man nur von wenigen Stoffen kennt. Die derzeit genauesten publizierten Werte f�r die maximale
Dichte liegen bei (999,974950 � 0,00084) kg/m� und bei (3,983 � 0,00067) �C f�r die Temperatur, bei der dieser Wert erreicht wird.
Die Werte stellen einen Mittelwert der von verschiedenen physikalischen Instituten ver�ffentlichten Zahlen dar (Stand 2001).

Die Berechnung der Dichte von luftfreiem Wasser DLF in Abh�ngigkeit von der Temperatur T ([T] = �C) kann mit Hilfe der folgenden
Virialgleichung erfolgen:
: \frac{D_{\rm LF}}{\rm g/l} = \frac{a_0 + a_1 T + a_2 T^2 + a_3 T^3 + a_4 T^4 + a_5 T^5}{1 + b T}.
mit den Koeffizienten: a0 = 999,83952; a1 = 16,952577 (�C)-1; a2 = -7,9905127�10-3 (�C)-2; a3 = -4,6241757�10-5 (�C)-3;
a4 = 1,0584601�10-7 (�C)-4; a5 = -2,8103006�10-10 (�C)-5 und b = 0,0168872.
F�r die Berechnung der Dichte von luftges�ttigtem Wasser korrigiert man den Wert nach
DLG/(g/l) = DLF/(g/l) - 0,004612 + 0,000106 (�C)-1�T. (Quelle: PTB-Mitteilungen 100/3-90)

*)
END_FUNCTION

FUNCTION WATER_DENSITY : REAL
VAR_INPUT
	T : REAL;
	Sat : BOOL;
END_VAR
VAR CONSTANT
	a0 : REAL := 999.83952;
	a1 : REAL := 16.952577;
	a2 : REAL := -7.9905127E-3;
	a3 : REAL := -4.6241757E-5;
	a4 : REAL := 1.0584601E-7;
	a5 : REAL := -2.8103006E-10;
	b  : REAL := 0.0168872;
END_VAR
VAR
	T2: REAL;
	T4: REAL;
END_VAR

T2 := T * T;
T4 := T2 * T2;
WATER_DENSITY := (a0 + a1*T + a2*T2 + a3*T2*T + a4*T4 + a5*T4*T) / (1.0 + b*T);
IF sat THEN WATER_DENSITY := WATER_DENSITY - 0.004612 + 0.000106 * T; END_IF;

(*
    �C - die Celsius-Temperatur in Grad Celsius
    v � das spezifische Volumen in Kubikdezimeter je Kilogramm dm�/kg
    h � die spezifische Enthalpie in Kilojoule je Kilogramm 	kJ/kg
    u � die spezifische Innere Energie in Kilojoule je Kilogramm 	kJ/kg
    s � die spezifische Entropie in Kilojoule je Kilogramm mal Kelvin 	kJ/(kg�K)
    cp - die spezifische W�rmekapazit�t bei konstantem Druck in Kilojoule je Kilogramm mal Kelvin 	kJ/(kg�K)
    Y � Volumenausdehnungskoeffizient in 10-3 durch Kelvin  10-3/K
    L � W�rmeleitf�higkeit in Milliwatt je Meter mal Kelvin  mW / (m�K)
    n � Viskosit�t in Mikropascal mal Sekunde uPa�s
    s ? � Oberfl�chenspannung in Millinewton je Meter  mN/m

�C 		v 				h 			u 		s 			cp 		Y 		L		n		s
-20		1,006580
0 		1,000160 		0,06 		-0,04 	-0,0001 	4,228 	-0,080 	561,0 	1792 	75,65
4		1,000028
5 		1,0000 		21,1 		21,0 	0,076 		4,200 	0,011 	570,6 	1518 	74,95
10 		1,0003 		42,1 		42,0 	0,151 		4,188 	0,087 	580,0 	1306 	74,22
15 		1,0009 		63,0 		62,9 	0,224 		4,184 	0,152 	589,4 	1137 	73,49
20 		1,001797 		83,9 		83,8 	0,296 		4,183 	0,209 	598,4 	1001 	72,74
25 		1,0029 		104,8 		104,7 	0,367 		4,183 	0,259 	607,2 	890,4 	71,98
30 		1,0044 		125,8 		125,7 	0,437 		4,183 	0,305 	615,5 	797,7 	71,20
35 		1,0060 		146,7 		146,6 	0,505 		4,183 	0,347 	623,3 	719,6 	70,41
40 		1,007842 		167,58 	167,5 	0,572 		4,182 	0,386 	630,6 	653,3 	69,60
45 		1,0099 		188,5 		188,4 	0,638 		4,182 	0,423 	637,3 	596,3 	68,78
50 		1,0121 		209,4 		209,3 	0,704 		4,181 	0,457 	643,6 	547,1 	67,95
60 		1,017089 		251,2 		251,1 	0,831 		4,183 	0,522 	654,4 	466,6 	66,24
70 		1,0227 		293,1 		293,0 	0,955 		4,187 	0,583 	663,1 	404,1 	64,49
80 		1,029027 		335,0 		334,9 	1,075 		4,194 	0,640 	670,0 	354,5 	62,68
90 		1,0359 		377,0 		376,9 	1,193 		4,204 	0,696 	675,3 	314,6 	60,82
99,63 	1,0431 		417,5 		417,4 	1,303 		4,217 	0,748 	679,0 	283,0 	58,99
100	1,043453		419,1

Stoffdaten von Wasser Quelle Wickipedia

*)
END_FUNCTION

FUNCTION WATER_ENTHALPY : REAL
VAR_INPUT
	T : REAL;
END_VAR
VAR
	Pts : INT := 11;
	data : ARRAY[1..20, 0..1] OF REAL := 0.0,0.06, 10.0,42.1, 20.0,83.9, 30.0,125.8, 40.0,167.58, 50.0,209.4, 60.0,251.2, 70.0,293.1, 80.0,335.0, 90.0, 377.0, 100.0,419.1;
END_VAR

WATER_ENTHALPY := LINEAR_INT(T, Data, Pts);

(*
    �C - die Celsius-Temperatur in Grad Celsius
    v � das spezifische Volumen in Kubikdezimeter je Kilogramm dm�/kg
    h � die spezifische Enthalpie in Kilojoule je Kilogramm 	kJ/kg
    u � die spezifische Innere Energie in Kilojoule je Kilogramm 	kJ/kg
    s � die spezifische Entropie in Kilojoule je Kilogramm mal Kelvin 	kJ/(kg�K)
    cp - die spezifische W�rmekapazit�t bei konstantem Druck in Kilojoule je Kilogramm mal Kelvin 	kJ/(kg�K)
    Y � Volumenausdehnungskoeffizient in 10-3 durch Kelvin  10-3/K
    L � W�rmeleitf�higkeit in Milliwatt je Meter mal Kelvin  mW / (m�K)
    n � Viskosit�t in Mikropascal mal Sekunde uPa�s
    s ? � Oberfl�chenspannung in Millinewton je Meter  mN/m

�C 		v 				h 			u 		s 			cp 		Y 		L		n		s
-20		1,006580
0 		1,000160 		0,06 		-0,04 	-0,0001 	4,228 	-0,080 	561,0 	1792 	75,65
4		1,000028
5 		1,0000 		21,1 		21,0 	0,076 		4,200 	0,011 	570,6 	1518 	74,95
10 		1,0003 		42,1 		42,0 	0,151 		4,188 	0,087 	580,0 	1306 	74,22
15 		1,0009 		63,0 		62,9 	0,224 		4,184 	0,152 	589,4 	1137 	73,49
20 		1,001797 		83,9 		83,8 	0,296 		4,183 	0,209 	598,4 	1001 	72,74
25 		1,0029 		104,8 		104,7 	0,367 		4,183 	0,259 	607,2 	890,4 	71,98
30 		1,0044 		125,8 		125,7 	0,437 		4,183 	0,305 	615,5 	797,7 	71,20
35 		1,0060 		146,7 		146,6 	0,505 		4,183 	0,347 	623,3 	719,6 	70,41
40 		1,007842 		167,58 	167,5 	0,572 		4,182 	0,386 	630,6 	653,3 	69,60
45 		1,0099 		188,5 		188,4 	0,638 		4,182 	0,423 	637,3 	596,3 	68,78
50 		1,0121 		209,4 		209,3 	0,704 		4,181 	0,457 	643,6 	547,1 	67,95
60 		1,017089 		251,2 		251,1 	0,831 		4,183 	0,522 	654,4 	466,6 	66,24
70 		1,0227 		293,1 		293,0 	0,955 		4,187 	0,583 	663,1 	404,1 	64,49
80 		1,029027 		335,0 		334,9 	1,075 		4,194 	0,640 	670,0 	354,5 	62,68
90 		1,0359 		377,0 		376,9 	1,193 		4,204 	0,696 	675,3 	314,6 	60,82
99,63 	1,0431 		417,5 		417,4 	1,303 		4,217 	0,748 	679,0 	283,0 	58,99
100	1,043453		419,1

Stoffdaten von Wasser Quelle Wickipedia

Wasser hat unter Normaldruck seine gr��te Dichte von 1000 Kilogramm pro Kubikmeter bei 3,98 �C und zeigt damit eine Dichteanomalie.
Diese besteht darin, dass sich Wasser unterhalb von 3,98 �C bei weiterer Temperaturverringerung, auch beim Wechsel zum festen
Aggregatzustand, wieder ausdehnt, was man nur von wenigen Stoffen kennt. Die derzeit genauesten publizierten Werte f�r die maximale
Dichte liegen bei (999,974950 � 0,00084) kg/m� und bei (3,983 � 0,00067) �C f�r die Temperatur, bei der dieser Wert erreicht wird.
Die Werte stellen einen Mittelwert der von verschiedenen physikalischen Instituten ver�ffentlichten Zahlen dar (Stand 2001).

Die Berechnung der Dichte von luftfreiem Wasser DLF in Abh�ngigkeit von der Temperatur T ([T] = �C) kann mit Hilfe der folgenden
Virialgleichung erfolgen:
: \frac{D_{\rm LF}}{\rm g/l} = \frac{a_0 + a_1 T + a_2 T^2 + a_3 T^3 + a_4 T^4 + a_5 T^5}{1 + b T}.
mit den Koeffizienten: a0 = 999,83952; a1 = 16,952577 (�C)-1; a2 = -7,9905127�10-3 (�C)-2; a3 = -4,6241757�10-5 (�C)-3;
a4 = 1,0584601�10-7 (�C)-4; a5 = -2,8103006�10-10 (�C)-5 und b = 0,0168872.
F�r die Berechnung der Dichte von luftges�ttigtem Wasser korrigiert man den Wert nach
DLG/(g/l) = DLF/(g/l) - 0,004612 + 0,000106 (�C)-1�T. (Quelle: PTB-Mitteilungen 100/3-90)

*)
END_FUNCTION

FUNCTION WCT : REAL
VAR_INPUT
	T : REAL;
	V : REAL;
END_VAR

IF V < 5.0 OR T > 10.0 THEN
	WCT := T;
ELSE
	WCT := 13.12 + 0.6215 * T +(0.3965 * T - 11.37) * EXP(LN(v) * 0.16);
END_IF;
END_FUNCTION

FUNCTION_BLOCK BLIND_ACTUATOR
VAR_INPUT
	UP, DN : BOOL;
	S_IN : BYTE;
	T_UD : TIME := T#10s;
	T_ANGLE : TIME := T#3s;
END_VAR
VAR_INPUT CONSTANT
	T_LOCKOUT : TIME := t#100ms;
END_VAR
VAR_OUTPUT
	POS, ANG : BYTE;
	QU, QD : BOOL;
	STATUS : BYTE;
END_VAR
VAR
	position, angle : RMP_B;
	lock : INTERLOCK;
END_VAR

(* make sure only one motor is active at a given time *)
lock(i1 := UP, I2 := DN, TL := T_lockout);

(* ramp up or down to simulate the angle position of the blind slats *)
angle(e := lock.Q1 OR lock.Q2, UP := lock.Q1, PT := T_Angle);
position(e := lock.Q1 AND angle.high OR lock.Q2 AND angle.low, up := lock.Q1, PT := T_UD);

(* set the outputs *)
pos := position.Out;
ang := angle.Out;

(* set the outputs *)
QU := lock.Q1;
QD := lock.Q2;

(* set the status output *)
IF UP AND DN THEN
	 status := 1;			(* error up and down together are not allowed *)
ELSIF UP THEN
	status := 121;
ELSIF DN THEN
	status := 122;
ELSE
	status := S_IN;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK BLIND_CONTROL
VAR_INPUT
	UP, DN : BOOL;
	S_IN : BYTE;
	PI : BYTE;
	AI : BYTE;
	T_UD, T_ANGLE : TIME;
END_VAR
VAR_INPUT CONSTANT
	SENS : BYTE := 5;
	T_LOCKOUT: TIME := T#100ms;
END_VAR
VAR_OUTPUT
	POS, ANG : BYTE;
	MU, MD : BOOL;
	STATUS : BYTE;
END_VAR
VAR
	act : BLIND_ACTUATOR;
	delta : BYTE;
	bTimeTest:BOOL:=FALSE;
	iPos:BYTE;
	iAngel:BYTE;
END_VAR

IF bTimeTest THEN
	PI:=iPos;
	AI:=iAngel;
	UP:=TRUE;
	DN:=TRUE;
END_IF

(* Check Position*)
act(T_UD:=T_UD, T_ANGLE:=T_ANGLE, T_lockout := T_Lockout);

IF UP AND DN THEN
	(* automatic modus detected *)
	(* first find correct position *)
	IF BYTE_TO_INT(act.pos) < BYTE_TO_INT(pi) - delta THEN
		act.UP := TRUE;
		act.DN := FALSE;
		delta := 0;
		STATUS := 121;
	ELSIF BYTE_TO_INT(act.pos) > BYTE_TO_INT(pi) + delta THEN
		act.UP := FALSE;
		act.DN := TRUE;
		delta := 0;
		STATUS := 122;
	(* regulate angle *)
	ELSIF BYTE_TO_INT(act.ang) < BYTE_TO_INT(ai) - delta AND T_angle > T#100ms THEN
		act.UP := TRUE;
		act.DN := FALSE;
		delta := sens/2;
		STATUS := 123;
	ELSIF BYTE_TO_INT(act.ang) > BYTE_TO_INT(ai) + delta AND T_angle > T#100ms THEN
		act.UP := FALSE;
		act.DN := TRUE;
		delta := sens/2;
		STATUS := 124;
	(* correct position reached *)
	ELSE
		act.UP := FALSE;
		act.DN := FALSE;
		delta := sens;
		STATUS := S_IN;
	END_IF;
ELSE
	act.UP := UP;
	act.DN := DN;
	STATUS := S_IN;
END_IF;

(* blind control calls blind_actuator *)
act(T_UD:=T_UD, T_ANGLE:=T_ANGLE, T_lockout := T_Lockout);
pos := act.pos;
ang := act.ang;
MU := act.QU;
md := act.QD;
status := act.status;
END_FUNCTION_BLOCK

FUNCTION_BLOCK BLIND_CONTROL_S
VAR_INPUT
	UP, DN : BOOL;
	S_IN : BYTE := 125;
	PI : BYTE;
	T_UP : TIME;
	T_DN : TIME;
	RU : BOOL;
	RD : BOOL;
END_VAR
VAR_INPUT CONSTANT
	T_LOCKOUT: TIME := T#100ms;
	T_EXT : TIME;
	EXT_TRIG : BYTE := 5;
	R_POS_TOP : BYTE := 255;
	R_POS_BOT : BYTE;
END_VAR
VAR_OUTPUT
	POS : BYTE;
	MU, MD : BOOL;
	STATUS : BYTE;
END_VAR
VAR
	rmp : _RMP_NEXT;
	tx, last : TIME;
	PI_last: BYTE;
END_VAR

(* status definition
	121 = manual up				manual up operation UP = true and DN = false
	122 = manual down			manual down operation DN = true and UP = false
	123 = auto positioning		automatic mode DN = UP = TRUE
	124 = revert					revert from top or bottom
	125 = nothing					default input for S_IN
	127 = Lockout time			lockout between directional change
	128 = calibrate				calibrate after power up
	129 = extend					extend runtime at top or bottom for continuous calibration
*)

(* read system time *)
tx := DWORD_TO_TIME(T_PLC_MS());

(* check inputs and start state machine *)
IF UP  AND NOT DN THEN
	(*  manual UP *)
	rmp.IN := 255;
	STATUS := 121;
ELSIF DN AND NOT UP THEN
	(* manual DN *)
	rmp.IN := 0;
	STATUS := 122;
ELSIF NOT (UP OR DN) THEN
	(* manual standby mode *)
	rmp.IN := PI;
	STATUS := S_IN;
END_IF;

(* simulate pos output *)
rmp(E := UP OR DN, TR := T_UP, TF := T_DN, TL := T_LOCKOUT, OUT := POS);

(* state machine *)
CASE STATUS OF
	0:		(* power up init *)
		last := tx;
		PI_last := PI XOR BYTE#255;
		STATUS := 128;		(* calibrate *)

	121: 	(* manual up *)
		MU := TRUE;
		MD := FALSE;
		IF POS >= (BYTE#255 - EXT_TRIG) THEN
			(* extend cycle if top is reached *)
			POS := 255;
			last := tx;
			STATUS := 129;	(* extend *)
		END_IF;

	122:	(* manual down *)
		MD := TRUE;
		MU := FALSE;
		IF POS <= EXT_TRIG THEN
			(* extend cycle if bottom is reached *)
			POS := 0;
			last := tx;
			STATUS := 129;	(* extend *)
		END_IF;

	123:	(* auto positioning *)
		MD := rmp.DN;
		MU := rmp.UP;
		IF NOT (rmp.DN OR rmp.UP) THEN
			(* automatic position is reached *)
			IF POS <= EXT_TRIG THEN
				(* extend at bottom *)
				MD := TRUE;
				last := tx;
				STATUS := 129;	(* extend *)
			ELSIF POS >= (BYTE#255 - EXT_TRIG)  THEN
				(* extend at top *)
				MU := TRUE;
				last := tx;
				STATUS := 129;	(* extend *)
			ELSE
				STATUS := S_IN;
			END_IF;
		END_IF;

	124:	(* revert from top or bottom *)
		MD := rmp.DN;
		MU := rmp.UP;
		IF NOT(rmp.DN OR rmp.UP) THEN
(*		IF POS = rmp.IN THEN  *)
			PI_last := PI;
			STATUS := S_IN;
		END_IF;

	127:	(* lockout time *)
		IF (tx - last) >= T_LOCKOUT THEN
				STATUS := S_IN;
		END_IF;

	128:	(* calibration *)
		MU := TRUE;
		MD := FALSE;
		rmp.IN := 255;
		IF (tx - last) >= (T_UP +  T_EXT) THEN
			MU := FALSE;
			last := tx;
			STATUS := 127;	(* lockout *)
		END_IF;

	129: (* extend mode *)
		IF (tx - last) >= T_EXT THEN
			MU := FALSE;
			MD := FALSE;
			last := tx;
			STATUS := 127;	(* lockout *)
		END_IF;

ELSE
	MU := FALSE;
	MD := FALSE;
	IF PI <> PI_last THEN
		PI_last := PI;
		rmp.IN := PI;
		STATUS := 123;	(* auto positioning *)
	ELSIF POS = 0 AND RU THEN
		rmp.IN := R_POS_BOT;
		STATUS := 124;	(* revert *)
	ELSIF pos = 255 AND RD THEN
		rmp.IN := R_POS_TOP;
		STATUS := 124;	(* revert *)
	ELSE
		STATUS := S_IN;
	END_IF;
END_CASE;
END_FUNCTION_BLOCK

// Changed for beedance: SINGLE is a keyword of IEC 61131-3, so it is SINGLE_ here.
FUNCTION_BLOCK BLIND_INPUT
VAR_INPUT
	POS, ANG : BYTE;
	S1, S2 : BOOL;
	IN : BOOL;
	PI, AI : BYTE;
END_VAR
VAR_INPUT CONSTANT
	SINGLE_SWITCH : BOOL;
	CLICK_EN : BOOL := TRUE;
	CLICK_TIME : TIME := T#500ms;
	MAX_RUNTIME : TIME := T#60s;
	MANUAL_TIMEOUT: TIME := T#1h;
	DEBOUNCE_TIME : TIME := T#20ms;
	DBL_CLK1 : BOOL := FALSE;
	DBL_POS1 : BYTE;
	DBL_ANG1 : BYTE;
	DBL_CLK2 : BOOL := FALSE;
	DBL_POS2 : BYTE := 255;
	DBL_ANG2 : BYTE := 255;
	D1_TOGGLE : BOOL := TRUE;
	D2_TOGGLE : BOOL := TRUE;
	MASTER_MODE : BOOL;
END_VAR
VAR_OUTPUT
	QU : BOOL := TRUE;
	QD : BOOL := TRUE;
	STATUS : BYTE;
	PO : BYTE := 255;
	AO : BYTE := 255;
	D1, D2 : BOOL;
END_VAR
VAR
	s1e, s2e : TOF;
	s1d, s2d : CLICK_MODE;
	dir : BOOL;
	tx, last : TIME;
END_VAR

(* read system timer *)
tx := DWORD_TO_TIME(T_PLC_MS());

(* inputs S1 and S2 are debounced and then decoded for multiple clicks *)
s1e(in := S1, pt := DEBOUNCE_TIME);
s2e(in := S2, pt := DEBOUNCE_TIME);
s1d(in := (s1e.Q AND NOT SINGLE_SWITCH) OR (s1e.Q AND SINGLE_SWITCH AND dir), t_long := CLICK_TIME);
s2d(in := (s2e.Q AND NOT SINGLE_SWITCH) OR (s1e.Q AND SINGLE_SWITCH AND NOT dir), t_long := CLICK_TIME);

(* if d1 and d2 are not toggle clear them to make sure they are only active for oine cycle *)
IF NOT d1_toggle THEN d1 := FALSE; END_IF;
IF NOT d2_toggle THEN d2 := FALSE; END_IF;

(* decode actions *)
IF (s1d.LONG AND s2d.LONG) OR (status = 139) THEN
   status := 139;
   IF NOT (s1d.LONG OR s2d.LONG)  THEN status := 130; END_IF;
ELSIF s1d.tp_LONG THEN
	status := 132;
ELSIF s2d.tp_LONG THEN
	status := 133;
ELSIF  s1d.SINGLE_ THEN
	IF click_en THEN
		(* if running then we stop otherwise start new direction *)
		IF QU XOR QD THEN
			status := 131;
		ELSE
			status := 134;
			last := tx;
			dir := NOT dir;
		END_IF;
	END_IF;
ELSIF  s2d.SINGLE_ THEN
	IF click_en THEN
		(* if running then we stop otherwise start new direction *)
		IF QU XOR QD THEN
			status := 131;
		ELSE
			status := 135;
			last := tx;
			dir := NOT dir;
		END_IF;
	END_IF;
ELSIF in THEN
	status := 136;
	last := tx;
ELSIF s1d.DOUBLE THEN
	IF dbl_clk1 THEN
		(* a position need to be set for a double click on S1 *)
		status := 137;
		last := tx;
	ELSE
		(* toggle the output when dbl_clk1 is not active *)
		D1 := NOT D1;
	END_IF;
ELSIF s2d.DOUBLE THEN
	IF dbl_clk2 THEN
		(* a position need to be set for a double click on S2 *)
		status := 138;
		last := tx;
	ELSE
		(* toggle the output when dbl_clk2 is not active *)
		(* in single switch mode we need to toggle d1 instead of d2 *)
		IF single_switch THEN D1 := NOT D1; ELSE D2 := NOT D2; END_IF;
	END_IF;
END_IF;

(* state machine *)
CASE status OF
	0:		(* power up operation *)
			status := 130;

	130:	(* automatic operation standby *)
			IF NOT master_mode THEN
				PO := POS;
				AO := ANG;
			END_IF;
			QU := TRUE;
			QD := TRUE;

	131:	(* manual operation standby *)
			QU := FALSE;
			QD := FALSE;
			PO := POS;
			AO := ANG;
			(* manual mode will be ended when timeout is reached *)
			IF tx - last >= manual_timeout THEN
				status := 130;
			END_IF;

	132:	(* manual operation up *)
			QU := TRUE;
			QD := FALSE;
			PO := POS; AO := ANG;
			last := tx;
			IF NOT s1d.LONG THEN status := 131; dir := NOT dir; END_IF;

	133:	(* manual operation down *)
			QU := FALSE;
			QD := TRUE;
			PO := POS; AO := ANG;
			last := tx;
			IF NOT s2d.LONG THEN	status := 131; dir := NOT dir; END_IF;

	134:	(* manual operation single click up *)
			QU := TRUE;
			QD := FALSE;
			PO := POS; AO := ANG;
			IF tx - last >= max_runtime THEN status := 131; END_IF;

	135:	(* manual operation single click dn *)
			QU := FALSE;
			QD := TRUE;
			PO := POS; AO := ANG;
			IF tx - last >= max_runtime THEN status := 131;	END_IF;

	136:	(* forced input in = true *)
			QU := TRUE;
			QD := TRUE;
			PO := PI; AO := AI;
			IF tx-last >= max_runtime THEN status := 130; END_IF;

	137:	(* double click1 position *)
			QU := TRUE;
			QD := TRUE;
			PO := DBL_POS1;
			AO := DBL_ANG1;
			IF tx - last >= max_runtime THEN status := 131; END_IF;

	138:	(* double click2 position *)
			QU := TRUE;
			QD := TRUE;
			PO := DBL_POS2;
			AO := DBL_ANG2;
			IF tx - last >= max_runtime THEN status := 131; END_IF;

	   139:   (* manual operation standby *)
	         QU := FALSE;
	         QD := FALSE;
	         PO := POS;
	         AO := ANG;

END_CASE;
END_FUNCTION_BLOCK

FUNCTION_BLOCK BLIND_NIGHT
VAR_INPUT
	UP, DN : BOOL;
	S_IN : BYTE;
	PI, AI : BYTE;
	E_NIGHT : BOOL := TRUE;
	E_DAY	: BOOL := TRUE;
	DTIN : DT;
	SUNRISE, SUNSET : TOD;
END_VAR
VAR_INPUT CONSTANT
	SUNRISE_OFFSET : TIME;
	SUNSET_OFFSET : TIME;
	NIGHT_POSITION : BYTE;
	NIGHT_ANGLE : BYTE;
END_VAR
VAR_OUTPUT
	QU, QD : BOOL;
	STATUS : BYTE;
	PO, AO : BYTE;
END_VAR
VAR
	night : BOOL;
	last_night, last_day : DATE;
END_VAR

IF NOT (up AND dn) AND night THEN
	(* manual operation at night will cancel operation for one night *)
	night := FALSE;
ELSIF (DT_TO_TOD(dtin) > sunset + sunset_offset) AND (last_night < DT_TO_DATE(dtin)) AND NOT night AND e_night THEN
	(* enable night *)
	night := TRUE;
	last_night := DT_TO_DATE(dtin);
ELSIF (DT_TO_TOD(dtin) > sunrise + sunrise_offset) AND (last_day < DT_TO_DATE(dtin)) AND night AND e_day AND (last_night < DT_TO_DATE(dtin)) THEN
	(* disable night *)
	night := FALSE;
	last_day := DT_TO_DATE(dtin);
END_IF;

(* shade at night only in auto mode and enable = true *)
IF UP AND DN AND night THEN
	status := 141;
	po := night_position;
	ao := night_angle;
ELSE
	QU := UP;
	QD := DN;
	po := pi;
	ao := ai;
	status := s_in;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK BLIND_SCENE
VAR_INPUT
	UP, DN : BOOL;
	S_IN : BYTE;
	PI, AI : BYTE;
	ENABLE : BOOL;
	SWRITE : BOOL;
	SCENE : BYTE;
END_VAR
VAR_OUTPUT
	QU, QD : BOOL;
	STATUS : BYTE;
	PO, AO : BYTE;
END_VAR
VAR RETAIN
	sx : ARRAY[0..15,0..2] OF BYTE;
END_VAR
VAR_TEMP
	x : BYTE;
END_VAR

(* delete all bits except the lower 4 of scene *)
x := scene AND 16#0F;

IF enable AND sx[x,2] > 0 AND UP AND DN THEN
	po := sx[x,0];
	ao := sx[x,1];
	status := 160 + x;  (* Status number is 160 to 175 for the 16 scenes *)
	QU := TRUE;
	QD := TRUE;
ELSE
	QU := UP;
	QD := DN;
	status := S_IN;
	po := pi;
	ao := ai;
END_IF;

(* write scene if necessary *)
IF swrite THEN
	status := 176;		(* write scene *)
	sx[x,0] := pi;
	sx[x,1] := ai;
	IF enable THEN sx[x,2] := 1; ELSE sx[x,2] := 0; END_IF; (* if sx[x,2] = 0 the scene is disabled *)
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK BLIND_SECURITY
VAR_INPUT
	UP, DN : BOOL;
	S_IN : BYTE;
	PI, AI : BYTE;
	FIRE : BOOL;
	WIND : BOOL;
	ALARM : BOOL;
	DOOR : BOOL;
	RAIN : BOOL;
END_VAR
VAR_INPUT CONSTANT
	Alarm_UP : BOOL := TRUE;
	WIND_UP :BOOL := TRUE;
	RAIN_UP : BOOL := FALSE;
END_VAR
VAR_OUTPUT
	QU, QD : BOOL;
	STATUS : BYTE;
	PO, AO : BYTE;
END_VAR

IF Fire THEN
	QU := TRUE;
	QD := FALSE;
	status := 111;
ELSIF Wind THEN
	QU := Wind_up;
	QD := NOT wind_up;
	status := 112;
ELSIF alarm THEN
	QU := Alarm_up;
	QD := NOT Alarm_up;
	status := 113;
ELSIF Door THEN
	QU := TRUE;
	QD := FALSE;
	status := 114;
ELSIF Rain AND NOT (up XOR dn) THEN
	QU := rain_up;
	QD := NOT rain_up;
	status := 115;
ELSE
	QU := UP;
	QD := DN;
	status := S_IN;
	po := pi;
	ao := ai;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK BLIND_SET
VAR_INPUT
	UP, DN : BOOL;
	S_IN : BYTE;
	PI, AI : BYTE;
	IN : BOOL;
	PX, AX : BYTE;
END_VAR
VAR_INPUT CONSTANT
	OVERRIDE_MANUAL : BOOL;
	RESTORE_POSITION : BOOL;
	RESTORE_TIME : TIME := T#60s;
END_VAR
VAR_OUTPUT
	QU, QD : BOOL;
	STATUS : BYTE;
	PO, AO : BYTE;
END_VAR
VAR
	tx: TIME;
	ps: BYTE;
	as: BYTE;
	last: TIME;
END_VAR

(* read system time *)
tx := DWORD_TO_TIME(T_PLC_MS());

(* check inputs *)
IF IN THEN
	IF override_manual OR (UP AND DN) THEN
		status := 178;
	END_IF;
END_IF;

(* state machine *)
CASE STATUS OF
	0: (* power on state *)
		status := S_IN;

	178: (* force values from PX and AX *)
		PO := PX;
		AO := AX;
		QU := TRUE;
		QD := TRUE;
		(* check for end of forced position *)
		IF NOT in THEN
			STATUS := SEL(RESTORE_POSITION, S_IN, 179);
			last := tx;
		END_IF;

	179: (* restore previous position *)
		PO := ps;
		AO := as;
		IF ((PO = PI) AND (AO = AI))  OR tx - last >= RESTORE_TIME THEN
			status := S_IN;
		END_IF;

ELSE
	(* no operation transfer inputs to outputs *)
	PO := PI;
	ps := PI;
	AO := AI;
	as := AI;
	STATUS := S_IN;
	QU := UP;
	QD := DN;
END_CASE;
END_FUNCTION_BLOCK

FUNCTION_BLOCK BLIND_SHADE
VAR_INPUT
	UP, DN : BOOL;
	S_IN : BYTE;
	PI, AI : BYTE;
	ENABLE : BOOL;
	SUN : BOOL;
END_VAR
VAR_IN_OUT
	CX : CALENDAR;
END_VAR
VAR_INPUT CONSTANT
	sunrise_offset : TIME := T#1h;
	sunset_preset : TIME := T#1h;
	direction : REAL := 180.0;
	angle_offset : REAL := 80.0;
	slat_width : REAL := 80.0;
	Slat_spacing : REAL := 60.0;
	Shade_delay : TIME := T#60s;
	Shade_pos : BYTE;
END_VAR
VAR_OUTPUT
	QU, QD : BOOL;
	STATUS : BYTE;
	PO, AO : BYTE;
END_VAR
VAR
	angle: REAL;
	sun_delay : TOF;
END_VAR

(* the input sun is sent through tof which will delay the shade for the time shade_delay *)
sun_delay(IN := sun, PT := shade_delay);

IF UP AND DN AND enable AND sun_delay.Q AND cx.SUN_HOR > direction - angle_offset AND cx.SUN_HOR < direction + angle_offset AND
	DT_TO_TOD(cx.UTC) > cx.SUN_RISE + sunrise_offset AND DT_TO_TOD(cx.UTC) < cx.SUN_SET - sunset_preset THEN
	status := 151;
	QU := UP;
	QD := DN;
	(* position is predefined *)
	po := shade_pos;
	(* shading is active now calculate the slat angle *)
	(* calculate the max angle for the blind *)
	angle := DEG(ATAN(slat_spacing / slat_width));
	(* check if sun angle is between 0 and max angle *)
	IF cx.SUN_VER > 0.0 AND cx.SUN_VER < angle THEN
		angle := cx.SUN_VER + DEG(ACOS(COS(RAD(cx.SUN_VER))*slat_spacing / Slat_width));
		ao := INT_TO_BYTE(LIMIT(0,TRUNC(angle * 2.833333333), 255));
	ELSE
		ao := 255;
	END_IF;
ELSE
	QU := UP;
	QD := DN;
	po := pi;
	ao := ai;
	status := S_IN;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK BLIND_SHADE_S
VAR_INPUT
   UP, DN : BOOL;
   S_IN : BYTE;
   PI : BYTE;
   ENABLE : BOOL;
   SUN : BOOL;
   HORZ1 : REAL := 100.0;
   HORZ2 : REAL := 260.0;
   VERT : REAL := 90.0;
   ALERT: BOOL := FALSE;
END_VAR
VAR_IN_OUT
   CX : calendar;
END_VAR
VAR_INPUT CONSTANT
   sunrise_offset : TIME := T#1h;
   sunset_preset : TIME := T#1h;
   shade_delay : TIME := T#60s;
   shade_pos : BYTE;
END_VAR
VAR_OUTPUT
   QU, QD : BOOL;
   STATUS : BYTE;
   PO : BYTE;
END_VAR
VAR
   sun_delay : TOF;
END_VAR
(* status definition
   151 = shadow               move shutter down for shadowing
   152 = alert                  move shutter up by setting QU:=TRUE when door is open.
*)

(* the input sun is sent through tof which will delay the shade for the time shade_delay *)
sun_delay(IN := sun, PT := shade_delay);

IF ALERT THEN
   QU := TRUE;
   QD := FALSE;
   STATUS := 152;
ELSIF UP AND DN AND ENABLE AND sun_delay.Q
AND (CX.SUN_HOR > HORZ1) AND (CX.SUN_HOR < HORZ2)
AND (CX.SUN_VER < VERT)
AND (DT_TO_TOD(CX.UTC) > CX.SUN_RISE + sunrise_offset) AND (DT_TO_TOD(CX.UTC) < CX.SUN_SET - sunset_preset) THEN
   QU := UP;
   QD := DN;
   STATUS := 151;
   (* Calculate Position, must be shade_pos or less *)
   PO := MIN(PI, shade_pos);
ELSE
   QU := UP;
   QD := DN;
   PO := PI;
   STATUS := S_IN;
END_IF;
END_FUNCTION_BLOCK

// Changed for beedance: a date or time of day literal is written in full, as D#yyyy-mm-dd and TOD#hh:mm:ss.
FUNCTION BUILDING_VERSION : DWORD
VAR_INPUT
	IN : BOOL;
END_VAR

IF in THEN
	BUILDING_VERSION := DATE_TO_DWORD(D#2011-02-03);
ELSE
	BUILDING_VERSION := 100;
END_IF;
END_FUNCTION

VAR_GLOBAL CONSTANT
END_VAR

VAR_GLOBAL
END_VAR

VAR_GLOBAL RETAIN

END_VAR
