// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION _BUFFER_CLEAR : BOOL
// VAR_INPUT
// 	PT : POINTER TO BYTE;
// 	SIZE : UINT;
// END_VAR
// VAR
// 	ptw : POINTER TO DWORD;
// 	temp: DWORD;
// 	end, end32 : DWORD;
// END_VAR
//
// (* this routine uses 32 bit access to gain speed *)
// (* first access bytes till pointer is aligned for 32 bit access *)
// temp := pt;
// end := temp + UINT_TO_DWORD(size);
// end32 := end - 3;
// WHILE (pt < end) AND ((temp AND 16#00000003) > 0) DO
// 	pt^ := 0;
// 	pt := pt + 1;
// 	temp := temp + 1;
// END_WHILE;
// (* pointer is aligned, now copy 32 bits at a time *)
// ptw := pt;
// WHILE ptw < end32 DO (* *)
// 	ptw^ := 0;
// 	ptw := ptw + 4;
// END_WHILE;
// (* copy the remaining bytes in byte mode *)
// pt := ptw;
// WHILE pt < end DO
// 	pt^ := 0;
// 	pt := pt + 1;
// END_WHILE;
//
// _BUFFER_CLEAR := TRUE;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION _BUFFER_INIT : BOOL
// VAR_INPUT
// 	PT : POINTER TO BYTE;
// 	SIZE : UINT;
// 	INIT : BYTE;
// END_VAR
// VAR
// 	ptw : POINTER TO DWORD;
// 	temp : DWORD;
// 	end, end32 : DWORD;
// END_VAR
//
// (* this routine uses 32 bit access to gain speed *)
// (* first access bytes till pointer is aligned for 32 bit access *)
// temp := pt;
// end := temp + UINT_TO_DWORD(size);
// end32 := end - 3;
// WHILE (pt < end) AND ((temp AND 16#00000003) > 0) DO
// 	pt^ := init;
// 	pt := pt + 1;
// 	temp := temp + 1;
// END_WHILE;
// (* pointer is aligned, now copy 32 bits at a time *)
// ptw := pt;
// temp := SHL(BYTE_TO_DWORD(init),24) OR SHL(BYTE_TO_DWORD(init),16) OR SHL(BYTE_TO_DWORD(init),8) OR BYTE_TO_DWORD(init);
// WHILE ptw < end32 DO
// 	ptw^ := temp;
// 	ptw := ptw + 4;
// END_WHILE;
// (* copy the remaining bytes in byte mode *)
// pt := ptw;
// WHILE pt < end DO
// 	pt^ := init;
// 	pt := pt + 1;
// END_WHILE;
//
// _BUFFER_INIT := TRUE;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION _BUFFER_INSERT : INT
// VAR_INPUT
// 	STR : STRING(STRING_LENGTH);
// 	POS : INT;
// 	PT : POINTER TO ARRAY[0..32767] OF BYTE;
// 	SIZE : UINT;
// END_VAR
// VAR
// 	end : INT;
// 	lx: INT;
// 	i: INT;
// END_VAR
//
// lx := LEN(str);
// end := pos + lx;
// (* first move the upper part of the buffer to make space for the string *)
// FOR i := UINT_TO_INT(size) - 1 TO end BY -1 DO
// 	pt^[i] := pt^[i-lx];
// END_FOR;
// (* copy the string *)
// _BUFFER_INSERT := _STRING_TO_BUFFER(str, pos, PT, size);
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION _BUFFER_UPPERCASE : BOOL
// VAR_INPUT
// 	PT : POINTER TO ARRAY[0..32000] OF BYTE;
// 	SIZE : INT;
// END_VAR
// VAR
// 	pos: INT;
// END_VAR
//
// pos := 0;
// WHILE pos < size DO
// 	PT^[pos] := TO_UPPER(pt^[pos]);
// 	pos := pos + 1;
// END_WHILE;
//
// _BUFFER_UPPERCASE := TRUE;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION _STRING_TO_BUFFER : INT
// VAR_INPUT
// 	STR : STRING(STRING_LENGTH);
// 	POS : INT;
// 	PT : POINTER TO ARRAY[0..32767] OF BYTE;
// 	SIZE : UINT;
// END_VAR
// VAR
// 	ps : POINTER TO BYTE;
// 	i: INT;
// 	end: INT;
// END_VAR
//
// ps := ADR(str);
// end := MIN(pos + LEN(str), UINT_TO_INT(size));
// IF end > 0 THEN end := end -1; END_IF;
// FOR i := pos TO end DO
// 	pt^[i] := ps^;
// 	ps := ps + 1;
// END_FOR;
//
// _STRING_TO_BUFFER := i;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION BUFFER_COMP : INT
// VAR_INPUT
// 	PT1 : POINTER TO ARRAY[0..32767] OF BYTE;
// 	SIZE1 : INT;
// 	PT2 : POINTER TO ARRAY[0..32767] OF BYTE;
// 	SIZE2 : INT;
// 	START : INT;
// END_VAR
// VAR
// 	i, j, end : INT;
// 	firstbyte: BYTE;
// END_VAR
//
// (* search for first character match *)
// IF size2 <= size1 THEN
// 	end := size1 - size2;
// 	firstbyte := PT2^[0];
// 	FOR i := START TO end DO
// 		IF PT1^[i] = firstbyte THEN
// 			(* first character matches, now compare rest of array *)
// 			j := 1;
// 			WHILE j < size2 DO
// 				IF pt2^[j] <> pt1^[j+i] THEN EXIT; END_IF;
// 				j := j + 1;
// 			END_WHILE;
// 			(* when J > size2 a match was found return the position i in buffer1 *)
// 			IF j = size2 THEN
// 				BUFFER_COMP := i;
// 				RETURN;
// 			END_IF;
// 		END_IF;
// 	END_FOR;
// END_IF;
// BUFFER_COMP := -1;
//
//
// (*
// hm 14. nov. 2008	rev 1.0
// 	original version
//
// hm	12. nov. 2009	rev 1.1
// 	performance increase
//
// *)
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION BUFFER_SEARCH : INT
// VAR_INPUT
// 	PT : POINTER TO ARRAY[0..32767] OF BYTE;
// 	SIZE : INT;
// 	STR : STRING(STRING_LENGTH);
// 	POS : INT;
// 	IGN : BOOL;
// END_VAR
// VAR
// 	ps : POINTER TO ARRAY[0..STRING_LENGTH] OF BYTE;
// 	chx : BYTE;
// 	i: INT;
// 	end: INT;
// 	k: INT;
// 	lx: INT;
// END_VAR
//
// ps := ADR(STR);
// lx := LEN(STR);
// end := MIN(SIZE - lx, SIZE - 1);
// lx := lx - 1;
// FOR i := pos TO end DO
// 	FOR k := 0 TO lx DO
// 		IF IGN THEN chx := TO_UPPER(pt^[i+k]); ELSE chx := pt^[i+k]; END_IF;
// 		IF ps^[k] <> chx THEN EXIT; END_IF;
// 	END_FOR;
// 	IF k > lx THEN
// 		BUFFER_SEARCH := i;
// 		RETURN;
// 	END_IF;
// END_FOR;
// BUFFER_SEARCH := -1;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION BUFFER_TO_STRING : STRING(STRING_LENGTH)
// VAR_INPUT
// 	PT : POINTER TO ARRAY[0..32767] OF BYTE;
// 	SIZE : UINT;
// 	START : UINT;
// 	STOP : UINT;
// END_VAR
// VAR
// 	ps : POINTER TO BYTE;
// 	i : UINT;
// 	stp: UINT;
// 	sta: UINT;
// END_VAR
//
// ps := ADR(BUFFER_TO_STRING);
// IF size = 0 THEN RETURN; END_IF;
// sta := MIN(start, size -1);
// stp := MIN(stop, size -1);
//
// (* check for maximum string_length *)
// IF UINT_TO_INT(stp - sta + 1) >= STRING_LENGTH THEN
// 	stp := sta + INT_TO_UINT(STRING_LENGTH) - 1;
// END_IF;
//
// FOR i := sta TO stp DO
// 	ps^ := pt^[i];
// 	ps := ps + 1;
// END_FOR;
//
// (* terminate the string *)
//
// ps^ := 0;
// END_FUNCTION

FUNCTION_BLOCK DRIVER_1
VAR_INPUT CONSTANT
	Toggle_Mode : BOOL;
	Timeout : TIME;
END_VAR
VAR_INPUT
	SET : BOOL;
	IN : BOOL;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	off : TON;
	edge: BOOL;
END_VAR

IF off.Q THEN Q := FALSE; END_IF;
IF rst THEN
	Q := FALSE;
ELSIF set THEN
	Q := TRUE;
ELSIF IN AND NOT edge THEN
	IF toggle_mode THEN q := NOT Q; ELSE q := TRUE; END_IF;
END_IF;
edge := in;
IF timeout > t#0s THEN off(in := Q, PT := Timeout); END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK DRIVER_4
VAR_INPUT CONSTANT
	Toggle_Mode : BOOL;
	Timeout : TIME;
END_VAR
VAR_INPUT
	SET : BOOL;
	IN0, IN1, IN2, IN3 : BOOL;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Q0, Q1, Q2, Q3 : BOOL;
END_VAR
VAR
	d0, d1, d2, d3 : DRIVER_1;
END_VAR

D0(Set:=set, in:=in0, rst:=rst, toggle_mode:=toggle_mode, timeout:=timeout);
D1(Set:=set, in:=in1, rst:=rst, toggle_mode:=toggle_mode, timeout:=timeout);
D2(Set:=set, in:=in2, rst:=rst, toggle_mode:=toggle_mode, timeout:=timeout);
D3(Set:=set, in:=in3, rst:=rst, toggle_mode:=toggle_mode, timeout:=timeout);
Q0 := D0.Q;
Q1 := D1.Q;
Q2 := D2.Q;
Q3 := D3.Q;
END_FUNCTION_BLOCK

FUNCTION_BLOCK DRIVER_4C
VAR_INPUT
	IN : BOOL;
	RST : BOOL;
END_VAR
VAR_INPUT CONSTANT
	Timeout : TIME;
	SX : ARRAY[1..7] OF BYTE := 1,3,7,15;
END_VAR
VAR_OUTPUT
	SN : INT;
	Q0 : BOOL;
	Q1 : BOOL;
	Q2 : BOOL;
	Q3 : BOOL;
END_VAR
VAR
	off : TON;
	edge: BOOL;
END_VAR

IF RST OR off.Q THEN
	SN := 0;
ELSIF IN AND NOT edge THEN
	SN := SN + 1;
	IF SN > 7 OR SX[SN] = 0 THEN SN := 0; END_IF;
END_IF;
edge := in;
IF SN > 0 THEN
	Q0 := SX[SN].0;
	Q1 := SX[SN].1;
	Q2 := SX[SN].2;
	Q3 := SX[SN].3;
ELSE
	Q0 := FALSE;
	Q1 := FALSE;
	Q2 := FALSE;
	Q3 := FALSE;
END_IF;

(* maximaum timeout *)
IF timeout > t#0s THEN off(in := SN > 0, PT := Timeout); END_IF;
END_FUNCTION_BLOCK

 FUNCTION_BLOCK FLOW_CONTROL
VAR_INPUT
	IN : BOOL;
	REQ : BOOL;
	ENQ : BOOL;
	RST : BOOL;
END_VAR
VAR_INPUT CONSTANT
	T_AUTO : TIME := T#1h;
	T_DELAY : TIME := T#23h;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	STATUS : BYTE;
END_VAR
VAR
	timer : TP_1D;
END_VAR


STATUS := 100;
IF RST THEN
	Q := FALSE;
	timer(rst := TRUE);
	timer.RST := FALSE;
	STATUS := 103;
ELSIF ENQ THEN
	IF IN THEN
		status := 101;
	END_IF;
	IF REQ THEN
		(* timer will generate a timed pulse after TP goes high *)
		timer.PT1 := T_AUTO;
		timer.PTD := T_DELAY;
		timer.IN := TRUE;
		STATUS := 102;
	END_IF;
END_IF;


(* set output and run timer *)
timer();
timer.IN := FALSE;
Q := (IN AND ENQ) OR timer.Q;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FT_Profile
VAR_INPUT
	K : REAL := 1.0;
	O : REAL;
	M : REAL := 1.0;
	E : BOOL;
END_VAR
VAR_INPUT CONSTANT
	value_0 : REAL;
	time_1 : TIME;
	value_1 : REAL;
	time_2 : TIME;
	value_2 : REAL;
	time_3 : TIME;
	value_3 : REAL;
	time_10 : TIME;
	value_10 : REAL;
	time_11 : TIME;
	value_11 : REAL;
	time_12 : TIME;
	value_12 : REAL;
	time_13 : TIME;
	value_13 : REAL;
END_VAR
VAR_OUTPUT
	Y : REAL;
	RUN : BOOL;
	ET : TIME;
END_VAR
VAR
	tx : TIME;
	edge : BOOL;
	state: BYTE;
	ta: TIME;
	tb: TIME;
	t0 : TIME;
	temp: REAL;
	va: REAL;
	vb: REAL;
END_VAR

(* read system time *)
tx := DWORD_TO_TIME(T_PLC_MS());

(* determine start condition *)
IF E AND NOT edge THEN
	RUN := TRUE;
	ET := t#0s;
	t0 := tx;
	ta := tx;
	tb := multime(time_1, M);
	va := value_0;
	vb := value_1;
	temp := value_0;
	state := 1;
END_IF;
edge := E;

(* generate startup profile *)
IF run THEN
	CASE state OF
		1:	IF tx - ta >= tb THEN
				ta := ta + tb;
				tb := multime(time_2 - time_1, M);
				va := value_1;
				vb := value_2;
				temp := value_1;
				state := 2;
			ELSE
				temp := ((vb - va) * TIME_TO_REAL(tx - ta) / TIME_TO_REAL(tb) + va);
			END_IF;
		2:	IF tx - ta >= tb THEN
				ta := ta + tb;
				tb := multime(time_3 - time_2, M);
				va := value_2;
				vb := value_3;
				temp := value_2;
				state := 3;
			ELSE
				temp := ((vb - va) * TIME_TO_REAL(tx - ta) / TIME_TO_REAL(tb) + va);
			END_IF;
		3:	IF tx - ta >= tb THEN
				ta := ta + tb;
				tb := multime(time_10 - time_3, M);
				va := value_3;
				vb := value_10;
				temp := value_3;
				state := 4;
			ELSE
				temp := ((vb - va) * TIME_TO_REAL(tx - ta) / TIME_TO_REAL(tb) + va);
			END_IF;
		4 :	IF tx - ta >= tb THEN
				ta := ta + tb;
				tb := multime(time_11 - time_10, M);
				va := value_10;
				vb := value_11;
				temp := value_10;
				IF E THEN state := 5; ELSE state := 6; END_IF;
			ELSE
				temp := ((vb - va) * TIME_TO_REAL(tx - ta) / TIME_TO_REAL(tb) + va);
			END_IF;
		5:	(* extend the signal while E is true *)
			IF E THEN
				ta := tx;
			ELSE
				state := 6;
			END_IF;
		6:	IF tx - ta >= tb THEN
				ta := ta + tb;
				tb := multime(time_12 - time_11, M);
				va := value_11;
				vb := value_12;
				temp := value_11;
				state := 7;
			ELSE
				temp := ((vb - va) * TIME_TO_REAL(tx - ta) / TIME_TO_REAL(tb) + va);
			END_IF;
		7:	IF tx - ta >= tb THEN
				ta := ta + tb;
				tb := multime(time_13 - time_12, M);
				va := value_12;
				vb := value_13;
				temp := value_12;
				state := 8;
			ELSE
				temp := ((vb - va) * TIME_TO_REAL(tx - ta) / TIME_TO_REAL(tb) + va);
			END_IF;
		8:	IF tx - ta >= tb THEN
				temp := value_13;
				run := FALSE;
			ELSE
				temp := ((vb - va) * TIME_TO_REAL(tx - ta) / TIME_TO_REAL(tb) + va);
			END_IF;
	END_CASE;
	Y := temp * K + O;
	ET := tx - t0;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK INC_DEC
VAR_INPUT
	CHa : BOOL;
	CHb : BOOL;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	dir : BOOL;
	cnt : INT;
END_VAR
VAR
	edgea : BOOL;
	clk: BOOL;
	clka: BOOL;
	clkb: BOOL;
	edgeb: BOOL;
	axb: BOOL;
END_VAR

axb := cha XOR chb;

(* create pulses for channel a *)
clka := cha XOR edgea;
edgea := cha;

clkb := chb XOR edgeb;
edgeb := chb;

(* create pulses for both channels *)
clk := clka OR clkb;

(* set the direction output *)
IF axb AND clka THEN dir := TRUE; END_IF;
IF axb AND clkb THEN dir := FALSE; END_IF;

(* increment or decrement the counter *)
IF clk AND dir THEN cnt := cnt + 1; END_IF;
IF clk AND NOT dir THEN cnt := cnt -1; END_IF;

(* reset the counter if rst active *)
IF rst THEN cnt := 0; END_IF;

END_FUNCTION_BLOCK

FUNCTION_BLOCK INTERLOCK
VAR_INPUT
	I1, I2 : BOOL;
	TL : TIME;
END_VAR
VAR_OUTPUT
	Q1, Q2 : BOOL;
END_VAR
VAR
	T1, T2 : TOF;
END_VAR

(* the input signal have a run delay to lockout the other input *)
T1(IN := I1, PT := TL);
T2(IN := I2, PT := TL);

Q1 := I1 AND NOT t2.Q;
Q2 := I2 AND NOT t1.Q;
END_FUNCTION_BLOCK

FUNCTION_BLOCK INTERLOCK_4
VAR_INPUT CONSTANT
END_VAR
VAR_INPUT
	I0 : BOOL;
	I1 : BOOL;
	I2 : BOOL;
	I3 : BOOL;
	E : BOOL;
	MODE : INT;

END_VAR
VAR_OUTPUT
	OUT : BYTE;
	TP : BOOL;
END_VAR
VAR
	in: BYTE;
	last: BYTE;
	old: BYTE;
	lmode: INT;
END_VAR

(* check if enable is active *)
IF E THEN
(* reset all vars when there is a mode change on thy fly *)
	IF mode <> lmode THEN
		out := 0;
		last := 0;
		old := 0;
		lmode := mode;
	END_IF;
	(* load inputs into in *)
	in.0 := I0;
	in.1 := I1;
	in.2 := I2;
	in.3 := I3;
	(* only execute when there is any change *)
	IF in <> last THEN
		(* only execute when inputs have chages state *)
		CASE mode OF
			0:	(* output directly display inputs as bits in byte out *)
				out := in;

			1:	(* the input with the highest number will be acepted *)
				IF in.3 THEN out := 8;
				ELSIF in.2 THEN out := 4;
				ELSIF in.1 THEN out := 2;
				ELSE out := in;
				END_IF;

			2:	(* input last pressed will be displayed only *)
				last := ((in XOR last) AND in);
				IF last.3 THEN out := 8;
				ELSIF last.2 THEN out := 4;
				ELSIF last.1 THEN out := 2;
				ELSE out := last;
				END_IF;

			3:	(* any input active will disable all other inputs *)
				IF (out AND in) = 0 THEN
					IF in.3 THEN out := 8;
					ELSIF in.2 THEN out := 4;
					ELSIF in.1 THEN out := 2;
					ELSE out := in;
					END_IF;
				END_IF;

		END_CASE;
		last := in;
	END_IF;
	tp := out <> old;
	old := out;
ELSE
	out := 0;
	last := 0;
	old := 0;
	lmode := 0;
	tp := FALSE;
END_IF;
END_FUNCTION_BLOCK

FUNCTION MANUAL : BOOL
VAR_INPUT
	IN : BOOL;
	ON : BOOL;
	OFF : BOOL;
END_VAR

MANUAL := NOT OFF AND (IN OR ON);
END_FUNCTION

FUNCTION_BLOCK MANUAL_1
VAR_INPUT
	IN : BOOL;
	MAN : BOOL;
	M_I : BOOL;
	SET : BOOL;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	STATUS : BYTE;
END_VAR
VAR
	S_edge : BOOL;
	r_edge : BOOL;
	edge: BOOL;
END_VAR

IF NOT man THEN
	Q := IN;
	STATUS := 100;
	edge := FALSE;
ELSIF NOT s_edge AND set THEN
	Q := TRUE;
	edge := TRUE;
	status := 101;
ELSIF NOT r_edge AND rst THEN
	Q := FALSE;
	edge := TRUE;
	status := 102;
ELSIF NOT edge THEN
	Q := M_I;
	status := 103;
END_IF;

(* remember levels of manual signals *)
s_edge := SET;
r_edge := RST;
END_FUNCTION_BLOCK

FUNCTION_BLOCK MANUAL_2
VAR_INPUT
	IN : BOOL;
	ENA : BOOL;
	ON : BOOL;
	OFF : BOOL;
	MAN : BOOL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	STATUS : BYTE;
END_VAR

IF ena THEN
	IF NOT ON AND NOT OFF THEN
		Q := IN;
		STATUS := 100;
	ELSIF on AND NOT off THEN
		Q := TRUE;
		STATUS := 101;
	ELSIF NOT on AND off THEN
		q := FALSE;
		STATUS := 102;
	ELSE
		Q := MAN;
		STATUS := 103;
	END_IF;
ELSE
	Q := FALSE;
	STATUS := 104;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK MANUAL_4
VAR_INPUT
	I0, I1, I2, I3 : BOOL;
	MAN : BOOL;
	STP : BOOL;
	M0, M1, M2, M3 : BOOL;
END_VAR
VAR_OUTPUT
	Q0, Q1, Q2, Q3 : BOOL;
	STATUS : BYTE;
END_VAR
VAR
	edge: BOOL;
	pos : INT;
	tog: BOOL;
END_VAR

IF man THEN
	IF NOT TOG THEN
		Q0 := M0;
		Q1 := M1;
		Q2 := M2;
		Q3 := M3;
		STATUS := 101;
	END_IF;
	IF STP AND NOT edge THEN
		tog := TRUE;
		CASE pos OF
			0:	Q0 := TRUE;
				Q1 := FALSE;
				Q2 := FALSE;
				Q3 := FALSE;
				STATUS := 110;
			1:	Q0 := FALSE;
				Q1 := TRUE;
				Q2 := FALSE;
				Q3 := FALSE;
				STATUS := 111;
			2:	Q0 := FALSE;
				Q1 := FALSE;
				Q2 := TRUE;
				Q3 := FALSE;
				STATUS := 112;
			3:	Q0 := FALSE;
				Q1 := FALSE;
				Q2 := FALSE;
				Q3 := TRUE;
				STATUS := 113;
		END_CASE;
		pos := INC(pos,1,3);
	END_IF;
ELSE
	Q0 := I0;
	Q1 := I1;
	Q2 := I2;
	Q3 := I3;
	STATUS := 100;
	tog := FALSE;
	pos := 0;
END_IF;

(* remember status of stp *)
edge := STP;
END_FUNCTION_BLOCK

FUNCTION_BLOCK PARSET
VAR_INPUT
	A0, A1 : BOOL;
END_VAR
VAR_INPUT CONSTANT
	X01, X02, X03, X04 : REAL;
	X11, X12, X13, X14 : REAL;
	X21, X22, X23, X24 : REAL;
	X31, X32, X33, X34 : REAL;
	TC : TIME;
END_VAR
VAR_OUTPUT
	P1, P2, P3, P4 : REAL;
END_VAR
VAR
	X : ARRAY[0..3,1..4] OF REAL;
	S1, S2, S3, S4 : REAL;
	tx, last : DWORD;
	start : BOOL;
	set : BYTE;
	init: BOOL;
END_VAR


(* read system_time *)
tx := T_PLC_MS();

(* init sequence *)
IF NOT init THEN
	set.0 := NOT A0;
	init := TRUE;
	X[0,1] := X01;
	X[0,2] := X02;
	X[0,3] := X03;
	X[0,4] := X04;
	X[1,1] := X11;
	X[1,2] := X12;
	X[1,3] := X13;
	X[1,4] := X14;
	X[2,1] := X21;
	X[2,2] := X22;
	X[2,3] := X23;
	X[2,4] := X24;
	X[3,1] := X31;
	X[3,2] := X32;
	X[3,3] := X33;
	X[3,4] := X34;
	P1 := X01;
	P2 := X02;
	P3 := X03;
	P4 := X04;
END_IF;

(* check for input change *)
IF (A0 XOR set.0) OR (A1 XOR set.1) THEN
	(* a new set is selected *)
	set.0 := A0;
	set.1 := A1;
	IF tc > t#0s THEN
		start := TRUE;
		last := tx;
		(* save the step speed for the output changes in S *)
		S1 := (X[set,1] - P1)/DWORD_TO_REAL(TIME_TO_DWORD(tc));
		S2 := (X[set,2] - P2)/DWORD_TO_REAL(TIME_TO_DWORD(tc));
		S3 := (X[set,3] - P3)/DWORD_TO_REAL(TIME_TO_DWORD(tc));
		S4 := (X[set,4] - P4)/DWORD_TO_REAL(TIME_TO_DWORD(tc));
	END_IF;
ELSIF start AND ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(last)) MOD 4294967296) < TIME_TO_DWORD(tc) THEN
	(* ramp the outputs to the new value *)
	P1 := X[set,1] - S1 * DWORD_TO_REAL(ULINT_TO_DWORD((TIME_TO_ULINT(Tc) + 4294967296 - DWORD_TO_ULINT(tx) + DWORD_TO_ULINT(last)) MOD 4294967296));
	P2 := X[set,2] - S2 * DWORD_TO_REAL(ULINT_TO_DWORD((TIME_TO_ULINT(Tc) + 4294967296 - DWORD_TO_ULINT(tx) + DWORD_TO_ULINT(last)) MOD 4294967296));
	P3 := X[set,3] - S3 * DWORD_TO_REAL(ULINT_TO_DWORD((TIME_TO_ULINT(Tc) + 4294967296 - DWORD_TO_ULINT(tx) + DWORD_TO_ULINT(last)) MOD 4294967296));
	P4 := X[set,4] - S4 * DWORD_TO_REAL(ULINT_TO_DWORD((TIME_TO_ULINT(Tc) + 4294967296 - DWORD_TO_ULINT(tx) + DWORD_TO_ULINT(last)) MOD 4294967296));
ELSE
	(* make sure outputs match the correct set values *)
	start := FALSE;
	P1 := X[set,1];
	P2 := X[set,2];
	P3 := X[set,3];
	P4 := X[set,4];
END_IF;

END_FUNCTION_BLOCK

FUNCTION_BLOCK PARSET2
VAR_INPUT
	X : REAL;
END_VAR
VAR_INPUT CONSTANT
	X01, X02, X03, X04 : REAL;
	X11, X12, X13, X14 : REAL;
	X21, X22, X23, X24 : REAL;
	X31, X32, X33, X34 : REAL;
	L1, L2, L3 : REAL;
	TC : TIME;
END_VAR
VAR_OUTPUT
	P1,P2,P3,P4 : REAL;
END_VAR
VAR
	Pset : parset;
	init: BOOL;
END_VAR


(* init sequence *)
IF NOT init THEN
	init := TRUE;
	pset(TC:=TC, X01:=X01, X02:=X02, X03:=X03, X04:=X04, X11:=X11, X12:=X12, X13:=X13, X14:=X14, X21:=X21, X22:=X22, X23:=X23, X24:=X24, X31:=X31, X32:=X32, X33:=X33, X34:=X34);
END_IF;
IF ABS(X) < L1 THEN
	pset(A0 := FALSE, A1 := FALSE);
ELSIF ABS(X) < L2 THEN
	pset(A0 := TRUE, A1 := FALSE);
ELSIF ABS(x) < L3 THEN
	pset(A0 := FALSE, A1 := TRUE);
ELSE
	pset(A0 := TRUE, A1 := TRUE);
END_IF;
P1 := pset.P1;
P2 := pset.P2;
P3 := pset.P3;
P4 := pset.P4;

END_FUNCTION_BLOCK

FUNCTION_BLOCK SIGNAL
VAR_INPUT
	IN : BOOL;
	SIG : BYTE;
	TS : TIME;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	tx: DWORD;
	step: BYTE;
END_VAR
VAR CONSTANT
	one : BYTE := 1;
END_VAR


IF in THEN
	(* an alarm is present read system time first *)
	tx := T_PLC_MS();
	(* calculate the step counter which is the lowest 3 bits of (time / ts) *)
	IF ts > t#0s THEN
		step := DWORD_TO_BYTE(ULINT_TO_DWORD(DWORD_TO_ULINT(tx) / TIME_TO_ULINT(ts)) AND 16#0000_0007);
	ELSE
		step := DWORD_TO_BYTE(SHR(tx,7) AND 16#0000_0007);
	END_IF;
	(* convert the value 0-7 in step into one bit only (bit 0-7) *)
	step := SHL(one,step);
	(* generate the output signal *)
	Q := (step AND sig) > 0;
ELSE
	Q := FALSE;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK SIGNAL_4
VAR_INPUT
	IN1 : BOOL;
	IN2 : BOOL;
	IN3 : BOOL;
	IN4 : BOOL;
	TS : TIME;
END_VAR
VAR_INPUT CONSTANT
	S1 : BYTE := 2#1111_1111;
	S2 : BYTE := 2#1111_0000;
	S3 : BYTE := 2#1010_1010;
	S4 : BYTE := 2#1010_0000;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	sig : SIGNAL;
END_VAR


(* an alarm is present read system time first *)
(* check if an alarm is present if yes set sig to the alarm pattern otherwise exit the routine *)
IF in1 THEN
	sig(in := TRUE, sig := s1, TS := TS);
ELSIF in2 THEN
	sig(in := TRUE, sig := s2, TS := TS);
ELSIF in3 THEN
	sig(in := TRUE, sig := s3, TS := TS);
ELSIF in4 THEN
	sig(in := TRUE, sig := s4, TS := TS);
ELSE
	sig(in := FALSE);
END_IF;

(* set the output *)
Q := sig.Q;

END_FUNCTION_BLOCK

FUNCTION_BLOCK SRAMP
VAR_INPUT
	X : REAL;
	A_UP : REAL;
	A_DN :REAL;
	VU_MAX : REAL;
	VD_MAX : REAL;
	LIMIT_HIGH : REAL;
	LIMIT_LOW : REAL;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Y : REAL;
	V : REAL;
END_VAR
VAR
	cycle_time : TC_S;
	init: BOOL;
END_VAR
VAR
	A_UP_work : REAL;
	A_DN_work : REAL;
	VU_MAX_work : REAL;
	VD_MAX_work : REAL;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
A_UP_work := A_UP;
A_DN_work := A_DN;
VU_MAX_work := VU_MAX;
VD_MAX_work := VD_MAX;

(* read the system_time in microseconds *)
cycle_time();

(* assure range of inputs *)
A_UP_work := MAX(0.0, A_UP_work);
A_DN_work := MIN(0.0, A_DN_work);
VU_MAX_work := MAX(0.0, VU_MAX_work);
VD_MAX_work := MIN(0.0, VD_MAX_work);

(* calculate the output offset *)
IF rst OR NOT init THEN
	init := TRUE;
	Y := 0.0;
	V := 0.0;
ELSIF X = Y THEN
	v := 0.0;
ELSIF X > Y THEN
	(* output is too low >> ramp up and brake at the end *)
	(* accelerate the speed and limit to vu_max *)
	v := MIN(v + A_UP_work * cycle_time.TC, VU_MAX_work);
	(* calculate the max speed to be able to brake and select the lowest *)
	v := MIN(SQRT((Y-X) * 2.0 * A_DN_work), v);
	(* calculate the output and obey limits *)
	y := LIMIT(limit_low, y + MIN(v * cycle_time.TC, X-Y), limit_high);
ELSIF X < Y THEN
	(* output is too high >> ramp dn and brake at the end *)
	(* accelerate the speed and limit to vd_max *)
	v := MAX(v + A_DN_work * cycle_time.TC, VD_MAX_work);
	(* calculate the max speed to be able to brake and select the lowest *)
	v := MAX(-SQRT((Y-X) * 2.0 * A_UP_work), v);
	(* calculate the output and obey limits *)
	y := LIMIT(limit_low, y + MAX(v * cycle_time.TC, X-Y), limit_high);
END_IF;



END_FUNCTION_BLOCK

FUNCTION_BLOCK TUNE
VAR_INPUT
	SET : BOOL;
	SU, SD : BOOL;
	RST : BOOL;
END_VAR
VAR_INPUT CONSTANT
	SS : REAL := 0.1;
	Limit_L : REAL;
	LIMIT_H : REAL := 100.0;
	RST_val : REAL;
	SET_val : REAL := 100.0;
	T1 : TIME := T#500ms;
	T2 : TIME := T#2s;
	S1 : REAL := 2.0;
	S2 : REAL := 10.0;
END_VAR
VAR_OUTPUT
	Y : REAL;
END_VAR
VAR
	tx : DWORD;
	start, start2 : DWORD;
	state : INT;
	in : BOOL;
	step : REAL;
	SPEED : REAL;
	Y_start : REAL;
	Y_start2: REAL;
END_VAR


(* read system time *)
tx := T_PLC_MS();

IF rst THEN
	Y := RST_val;
	state := 0;
ELSIF set THEN
	Y := SET_val;
	state := 0;
ELSIF state > 0 THEN
	(* key has been pushed state machine operating *)
	(* first read the correct input *)
	IF state = 1 THEN
		(* step up *)
		in := su;
	ELSE
		(* step down *)
		in := sd;
	END_IF;
	(* check for single step operation *)
	IF NOT in AND ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(start)) MOD 4294967296) <= TIME_TO_DWORD(T1) THEN
		Y := Y_start + step;
		state := 0;
	(* check if fast ramp needs to be generated *)
	ELSIF in AND ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(start)) MOD 4294967296) >= TIME_TO_DWORD(T2) THEN
		Y := Y_start2 + DWORD_TO_REAL(ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(start2)) MOD 4294967296)) * s2 / speed;
	(* check if slow ramp needs to be generated *)
	ELSIF in AND ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(start)) MOD 4294967296) >= TIME_TO_DWORD(T1) THEN
		Y := Y_start + DWORD_TO_REAL(ULINT_TO_DWORD((DWORD_TO_ULINT(ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(start)) MOD 4294967296)) + 4294967296 - TIME_TO_ULINT(T1)) MOD 4294967296)) * S1 / speed;
		start2 := tx;
		Y_start2 := Y;
	ELSIF NOT in THEN
		state := 0;
	END_IF;
ELSIF su THEN
	(* slow step up *)
	state := 1;
	start := tx;
	step := ss;
	speed := 1000.0;
	Y_start := Y;
ELSIF sd THEN
	(* slow step down *)
	state := 2;
	start := tx;
	step := -ss;
	speed := -1000.0;
	Y_start := Y;
END_IF;

(* make sure output does not exceed limits *)
Y := LIMIT(LIMIT_L, Y, LIMIT_H);
END_FUNCTION_BLOCK

FUNCTION_BLOCK TUNE2
VAR_INPUT
	SET : BOOL;
	SU, SD : BOOL;
	FU, FD : BOOL;
	RST : BOOL;
END_VAR
VAR_INPUT CONSTANT
	SS : REAL := 0.1;
	FS : REAL := 5.0;
	Limit_L : REAL;
	LIMIT_H : REAL := 100.0;
	RST_val : REAL;
	SET_val : REAL := 100.0;
	TR : TIME := T#500ms;
	S1 : REAL := 2.0;
	S2 : REAL := 10.0;
END_VAR
VAR_OUTPUT
	Y : REAL;
END_VAR
VAR
	tx : DWORD;
	start : DWORD;
	state : INT;
	in : BOOL;
	step : REAL;
	SPEED : REAL;
	Y_start : REAL;
END_VAR


(* read system time *)
tx := T_PLC_MS();

IF rst THEN
	Y := RST_val;
	state := 0;
ELSIF set THEN
	Y := SET_val;
	state := 0;
ELSIF state > 0 THEN
	(* key has been pushed state machine operating *)
	(* first read the correct input *)
	CASE state OF
		1 :	(* slow up *)
			in := su;
		2 :	(* slow down *)
			in := sd;
		3 :	(* fast up *)
			in := fu;
		4 :	(* fast down *)
			in := fd;
	END_CASE;
	(* check for single step operation *)
	IF NOT in AND ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(start)) MOD 4294967296) <= TIME_TO_DWORD(TR) THEN
		Y := Y_start + step;
		state := 0;
	(* check if ramp needs to be generated *)
	ELSIF in AND ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(start)) MOD 4294967296) >= TIME_TO_DWORD(TR) THEN
		Y := Y_start + DWORD_TO_REAL(ULINT_TO_DWORD((DWORD_TO_ULINT(ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(start)) MOD 4294967296)) + 4294967296 - TIME_TO_ULINT(TR)) MOD 4294967296)) * speed;
	ELSIF NOT in THEN
		state := 0;
	END_IF;
ELSIF su THEN
	(* slow step up *)
	state := 1;
	start := tx;
	step := ss;
	speed := s1 * 1.0E-3;
	Y_start := Y;
ELSIF sd THEN
	(* slow step down *)
	state := 2;
	start := tx;
	step := -ss;
	speed := -s1 * 1.0E-3;
	Y_start := Y;
ELSIF fu THEN
	(* fast step up *)
	state := 3;
	start := tx;
	step := fs;
	speed := s2 * 1.0E-3;
	y_start := Y;
ELSIF fd THEN
	(* fast step down *)
	state := 4;
	start := tx;
	step := -fs;
	speed := -s2 * 1.0E-3;
	y_start := Y;
END_IF;

(* make sure output does not exceed limits *)
Y := LIMIT(LIMIT_L, Y, LIMIT_H);
END_FUNCTION_BLOCK

FUNCTION BAND_B : BYTE
VAR_INPUT
	X : BYTE;
	B : BYTE;
END_VAR

IF X < B THEN
	BAND_B := 0;
ELSIF BYTE_TO_UINT(X) > 255 - BYTE_TO_UINT(B) THEN
	BAND_B := 255;
ELSE
	BAND_B := X;
END_IF;

END_FUNCTION

FUNCTION_BLOCK CONTROL_SET1
VAR_INPUT
	Kt : REAL;
	Tt : REAL;
	PI : BOOL;
	PID : BOOL;
END_VAR
VAR_INPUT CONSTANT
	P_K : REAL := 0.5;
	PI_K : REAL := 0.45;
	PI_TN : REAL := 0.83;
	PID_K : REAL := 0.6;
	PID_TN : REAL := 0.5;
	PID_TV : REAL := 0.125;
END_VAR
VAR_OUTPUT
	KP : REAL;
	TN : REAL;
	TV : REAL;
	KI : REAL;
	KD : REAL;
END_VAR


IF pi AND PID THEN
	KP := 0.0;
	TN := 0.0;
	TV := 0.0;
ELSIF PID THEN
	KP := PID_K * Kt;
	TN := PID_TN * Tt;
	TV := PID_TV * Tt;
ELSIF PI THEN
	KP := PI_K * Kt;
	TN := PI_TN * Tt;
ELSE
	KP := P_K * Kt;
END_IF;

(* KI and KD are calculated *)
IF tn > 0.0 THEN KI := KP / TN; ELSE KI := 0.0; END_IF;
KD := KP * TV;
END_FUNCTION_BLOCK

FUNCTION_BLOCK CONTROL_SET2
VAR_INPUT
	KS : REAL;
	TU : REAL;
	TG : REAL;
	PI : BOOL;
	PID : BOOL;
END_VAR
VAR_INPUT CONSTANT
	P_K : REAL := 1.0;
	PI_K : REAL := 0.9;
	PI_TN : REAL := 3.33;
	PID_K : REAL := 1.2;
	PID_TN : REAL := 2.0;
	PID_TV : REAL := 0.5;
END_VAR
VAR_OUTPUT
	KP : REAL;
	TN : REAL;
	TV : REAL;
	KI : REAL;
	KD : REAL;
END_VAR
VAR
	TX : REAL;
END_VAR


IF TU > 0.0 AND KS > 0.0 THEN TX := TG / TU / KS; END_IF;
IF pi AND PID THEN
	KP := 0.0;
	TN := 0.0;
	TV := 0.0;
ELSIF PID THEN
	KP := PID_K * TX;
	TN := PID_TN * TU;
	TV := PID_TV * TU;
ELSIF PI THEN
	KP := PI_K * TX;
	TN := PI_TN * TU;
ELSE
	KP := P_K * TX;
END_IF;

(* KI and KD are calculated *)
IF TN > 0.0 THEN KI := KP / TN; ELSE KI := 0.0; END_IF;
KD := KP * TV;
END_FUNCTION_BLOCK

FUNCTION CTRL_IN : REAL
VAR_INPUT
	SET_POINT, ACTUAL, NOISE : REAL;
END_VAR


(* calculate the process error DIFF *)
CTRL_IN := DEAD_ZONE(SET_POINT - ACTUAL, NOISE);
END_FUNCTION

FUNCTION_BLOCK CTRL_OUT
VAR_INPUT
	CI, OFFSET, MAN_IN, LIM_L, LIM_H : REAL;
	MANUAL : BOOL;
END_VAR
VAR_OUTPUT
	Y : REAL;
	LIM : BOOL;
END_VAR


Y := SEL(MANUAL, CI, MAN_IN) + OFFSET;

(* Limit the output *)
IF Y > LIM_L AND Y < LIM_H THEN
	LIM := FALSE;
ELSE
	Y := LIMIT(LIM_L, Y, LIM_H);
	LIM := TRUE;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK CTRL_PI
VAR_INPUT
	ACT, SET, SUP, OFS, M_I : REAL;
	MAN : BOOL;
	RST : BOOL;
	KP : REAL := 1.0;
	KI : REAL := 1.0;
	LL : REAL := -1000.0;
	LH : REAL := 1000.0;
END_VAR
VAR_OUTPUT
	Y : REAL;
	DIFF : REAL;
	LIM : BOOL;
END_VAR
VAR
	pi : FT_PIWL;
	co : CTRL_OUT;
END_VAR


DIFF := CTRL_IN(SET, ACT, SUP);
pi(in := DIFF, kp := KP, ki := KI, lim_l := LL, lim_h := LH, rst := RST);
co(ci := pi.Y, OFFSET := OFS, man_in := M_I, lim_l := LL, lim_h := LH, MANUAL := MAN);
Y := co.Y;
LIM := co.LIM;
END_FUNCTION_BLOCK

FUNCTION_BLOCK CTRL_PID
VAR_INPUT
	ACT, SET, SUP, OFS, M_I : REAL;
	MAN : BOOL;
	RST : BOOL;
	KP : REAL := 1.0;
	TN : REAL := 1.0;
	TV : REAL := 1.0;
	LL : REAL := -1000.0;
	LH : REAL := 1000.0;
END_VAR
VAR_OUTPUT
	Y : REAL;
	DIFF : REAL;
	LIM : BOOL;
END_VAR
VAR
	pid : FT_PIDWL;
	co : CTRL_OUT;
END_VAR


DIFF := CTRL_IN(SET, ACT, SUP);
pid(in := DIFF, kp := KP, tn := TN, tv := TV, lim_l := LL, lim_h := LH, rst := RST);
co(ci := pid.Y, OFFSET := OFS, man_in := M_I, lim_l := LL, lim_h := LH, MANUAL := MAN);
Y := co.Y;
LIM := co.LIM;
END_FUNCTION_BLOCK

FUNCTION_BLOCK CTRL_PWM
VAR_INPUT
	CI, MAN_IN : REAL;
	MANUAL : BOOL;
	F : REAL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	PW : PWM_DC;
END_VAR


PW(F := F, DC := SEL(MANUAL,CI,MAN_IN));
Q := PW.Q;
END_FUNCTION_BLOCK

FUNCTION DEAD_BAND : REAL
VAR_INPUT
	X : REAL;
	L : REAL;
END_VAR

IF X > L THEN
	DEAD_BAND := X - L;
ELSIF X < -L THEN
	DEAD_BAND := X + L;
ELSE
	DEAD_BAND := 0.0;
END_IF;
END_FUNCTION

FUNCTION_BLOCK DEAD_BAND_A
VAR_INPUT
	X : REAL;
	T : TIME;
	KL : REAL := 1.0;
	LM : REAL;
END_VAR
VAR_OUTPUT
	Y : REAL;
	L : REAL;
END_VAR
VAR
	tp1 : FT_PT1;
	tp2 : FT_PT1;
END_VAR

(* filter the input signal *)
tp1(in := X, T:= T);

(* filter the HF portion to generate a stable L *)
tp2(in := ABS(tp1.out - X), T := MULTIME(T, 4.0));

(* now we determine L which is half the width of the dead band *)
L := MIN(KL * tp2.out, LM);

IF X > L THEN
	Y := X - L;
ELSIF X < -L THEN
	Y := X + L;
ELSE
	Y := 0.0;
END_IF;
END_FUNCTION_BLOCK

FUNCTION DEAD_ZONE : REAL
VAR_INPUT
	X : REAL;
	L : REAL;
END_VAR

IF ABS(x) > L THEN
	dead_zone := X;
ELSE
	DEAD_ZONE := 0.0;
END_IF;
END_FUNCTION

FUNCTION_BLOCK DEAD_ZONE2
VAR_INPUT
	X : REAL;
	L : REAL;
END_VAR
VAR_OUTPUT
	Y: REAL;
END_VAR

IF ABS(x) > L THEN
	Y := X;
ELSIF Y > 0.0 THEN
	Y := L;
ELSE
	Y := -L;
END_IF;

END_FUNCTION_BLOCK

FUNCTION_BLOCK FT_DERIV
VAR_INPUT
	in : REAL;
	K : REAL := 1.0;
	run : BOOL := TRUE;
END_VAR
VAR_OUTPUT
	out : REAL;
END_VAR
VAR
	old: REAL;
	tx: DWORD;
	last: DWORD;
	init: BOOL;
	tc: REAL;
END_VAR




(* read system time *)
tx := T_PLC_US();
tc := DWORD_TO_REAL(ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(last)) MOD 4294967296));
last := tx;

(* init on first startup *)
IF NOT init THEN
	init := TRUE;
	old := in;
ELSIF run AND tc > 0.0 THEN
	out := (in - old) / tc * 1000000.0 * K;
	old := in;
ELSE
	out := 0.0;
END_IF;



(*
hm 3.1.2007			rev 1.1
	added init code for startup
	set the default for K to 1

hm	15. sep 2007	rev 1.2
	replaced Time() with T_PLC_US for compatibility and performance reasons
	increased accuracy and work in microseconds internally

hm 29 oct 2007	rev 1.3
	prohibit calculation when tx - last = 0 to avoid division by 0 and increase accuracy on fast systems

hm	6. nov. 2008	rev 1.4
	improved performance

hm	11. mar. 2009	rev 1.5
	inproved code
*)
END_FUNCTION_BLOCK

FUNCTION_BLOCK FT_IMP
VAR_INPUT
	in : REAL;
	T : TIME;
	K : REAL := 1.0;
END_VAR
VAR_OUTPUT
	out : REAL;
END_VAR
VAR
	t1 : FT_PT1;
END_VAR

T1(in:= in, T:=T);
out := (in - t1.out) * K;

(*
hm 3.1.2007	rev 1.1
	added factor K

*)
END_FUNCTION_BLOCK

FUNCTION_BLOCK FT_INT
VAR_INPUT
	IN : REAL;
	K : REAL := 1;
	RUN : BOOL := TRUE;
	RST : BOOL;
	OUT_MIN : REAL := -1E37;
	OUT_MAX : REAL := 1E37;
END_VAR
VAR_OUTPUT
	OUT : REAL;
	LIM : BOOL;
END_VAR
VAR
	INTeg : INTEGRATE;
END_VAR

IF rst THEN
	out := 0.0;
ELSE
	integ(X := IN, E := RUN, K := K, Y := out);
END_IF;

(* limit the outputs *)
IF out >= OUT_MAX THEN
	out := out_max;
	LIM := TRUE;
ELSIF out <= out_min THEN
	out := out_min;
	lim := TRUE;
ELSE
	lim := FALSE;
END_IF;


(*
hm 13.12.2006	rev 1.1
	changed to "trapezregel" which increases accuracy
	before out := out + in * time new: out := out + (in + in_last) / 2 * time

hm 15.1.2007		rev 1.2
	added default for k to be 1.

hm	15.9.2007		rev 1.3
	replaced time() with T_PLC_US for compatibility and performance reason
	increased internal accuracy to microseconds

hm 29. oct 2007		rev 1.4
	changed code so int will not be called is time difference is 0 to increase accuracy on systems
	with cycle times below 1ms

hm	2. dec 2007	 	rev 1.5
	changed code for better performance

hm	5. jan 2008		rev 1.6
	further improvements in performance

hm	8. feb 2008		rev 1.7
	deleted limits +/- 1000 now limit is the range of real
	added variables out1 and out2 to extend the resolution
	corrected an error with elapsed time
	deleted unusfull output ET

hm	13. mar 2008	rev 1.8
	changed preset value out_min to -1e37.
	stop integrator at the limits.

hm	16. mar 2008	rev 1.9
	added type conversion to avoid warning under codesys 3.0

hm	2. jun. 2008	rev 2.0
	rewritten with clear code
	limits can now be set without run

hm	3. nov. 2008	rev 2.1
	optimized code using INTEGRATE and LIMX

hm	10. mar. 2009	rev 2.2
	removed nested comments
*)

END_FUNCTION_BLOCK

FUNCTION_BLOCK FT_INT2
VAR_INPUT
	IN : REAL;
	K : REAL := 1.0;
	RUN : BOOL := TRUE;
	RST : BOOL;
	OUT_MIN : REAL := -1.0E38;
	OUT_MAX : REAL := 1.0E38;
END_VAR
VAR_OUTPUT
	OUT : REAL;
	LIM : BOOL;
END_VAR
VAR
	integ: INTEGRATE;
	ix : REAL;
	val : REAL2;
END_VAR

IF RST THEN
	val := R2_SET(0.0);
	out := 0.0;
ELSE
	integ(X := IN, E := RUN, K := K, Y := ix);
	val := R2_ADD(val, ix);
	ix := 0.0;
	OUT := val.RX;
END_IF;

(* check output for limits *)
IF out > OUT_MIN AND out < OUT_MAX THEN
	LIM := FALSE;
ELSE
	OUT := LIMIT(OUT_MIN, OUT, OUT_MAX);
	val := R2_SET(OUT);
	LIM := TRUE;
END_IF;

END_FUNCTION_BLOCK

FUNCTION_BLOCK FT_PD
VAR_INPUT
	IN : REAL;
	KP : REAL := 1.0;
	TV : REAL := 1.0;
END_VAR
VAR_OUTPUT
	Y : REAL;
END_VAR
VAR
	diff : FT_DERIV;
END_VAR


(* run differentiator *)
diff(IN := IN, K := TV);

(* combine both values *)
Y := KP * (diff.out + IN);
END_FUNCTION_BLOCK

FUNCTION_BLOCK FT_PDT1
VAR_INPUT
	IN : REAL;
	KP : REAL := 1.0;
	TV : REAL := 1.0;
	T1 : REAL := 1.0;
END_VAR
VAR_OUTPUT
	Y : REAL;
END_VAR
VAR
	diff : FT_DERIV;
	TP : FT_PT1;
END_VAR


(* run differentiator *)
diff(IN := IN, K := TV);

(* Run PT1 filter *)
tp(in := diff.out, T := REAL_TO_TIME(T1));

(* combine both values *)
Y := KP * (tp.out + IN);
END_FUNCTION_BLOCK

FUNCTION_BLOCK FT_PI
VAR_INPUT
	IN : REAL;
	KP : REAL := 1.0;
	KI : REAL := 1.0;
	ILIM_L : REAL := -1E38;
	ILIM_H : REAL := 1E38;
	IEN : BOOL := TRUE;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Y : REAL;
	LIM : BOOL;
END_VAR
VAR
	integ : FT_INT;
END_VAR


(* run integrator *)
integ(IN := IN, K := KI, RUN := IEN, RST := RST, OUT_MIN := ILIM_L, OUT_MAX := ILIM_H);

(* check if integrator has reached its limits and set overflow *)
LIM := integ.LIM;
Y := KP * IN + integ.Out;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FT_PID
VAR_INPUT
	IN : REAL;
	KP : REAL := 1.0;
	TN : REAL := 1.0;
	TV : REAL := 1.0;
	ILIM_L : REAL := -1.0E38;
	ILIM_H : REAL := 1.0E38;
	IEN : BOOL := TRUE;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Y : REAL;
	LIM : BOOL;
END_VAR
VAR
	integ : FT_INT;
	diff : FT_DERIV;
END_VAR


(* run integrator only if TN > 0 *)
IF TN > 0.0 THEN
	integ(IN := IN, K := 1.0 / TN, RUN := IEN, RST := RST, OUT_MIN := ILIM_L, OUT_MAX := ILIM_H);
ELSE
	integ(RST := FALSE);
END_IF;

(* run differentiator *)
diff(IN := IN, K := TV);

(* combine both values *)
Y := KP * (integ.Out + diff.out + IN);

(* check if integrator has reached its limits and set overflow *)
LIM := integ.LIM;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FT_PIDW
VAR_INPUT
	IN : REAL;
	KP : REAL := 1.0;
	TN : REAL := 1.0;
	TV : REAL := 1.0;
	LIM_L : REAL := -1.0E38;
	LIM_H : REAL := 1.0E38;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Y : REAL;
	LIM : BOOL;
END_VAR
VAR
	integ : INTEGRATE;
	diff : FT_DERIV;
	YI: REAL;
END_VAR


(* run the integrator *)
IF tn = 0.0 OR rst THEN
	integ(E := FALSE, Y := YI);
	YI := 0.0;
ELSE
	integ(X := IN, K := 1.0 / TN, E := NOT LIM, Y := YI);
END_IF;

(* add up integrator and linear part *)
Y := KP * (IN + YI);

(* run differentiator *)
diff(IN := IN, K := TV);

(* set lim before differentiator is added to stop integrator if necessary *)
IF Y > LIM_L AND Y < LIM_H THEN
	LIM := FALSE;
ELSE
	LIM := TRUE;
END_IF;

(* add differential part and limit output Y *)

Y := LIMIT(LIM_L, Y + KP * diff.out, LIM_H);
END_FUNCTION_BLOCK

FUNCTION_BLOCK FT_PIDWL
VAR_INPUT
	IN : REAL;
	KP : REAL := 1.0;
	TN : REAL := 1.0;
	TV : REAL := 1.0;
	LIM_L : REAL := -1.0E38;
	LIM_H : REAL := 1.0E38;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Y : REAL;
	LIM : BOOL;
END_VAR
VAR
	piwl : FT_PIWL;
	diff : FT_DERIV;
END_VAR


(* if rst then *)
IF rst THEN
	piwl(rst := TRUE);
	piwl.RST := FALSE;
ELSE
	(* run PIWL controller first *)
	(* we need to check if TN = 0 and do alternative calls *)
	IF TN = 0.0 THEN
		piwl(in := IN * KP, KP := 1.0, KI := 0.0, LIM_L := LIM_L, LIM_H := LIM_H);
	ELSE
		piwl(in := IN * KP, KP := 1.0, KI := 1.0 / TN, LIM_L := LIM_L, LIM_H := LIM_H);
	END_IF;

	(* run differentiator and add_to_output *)
	diff(IN := IN, K := KP * TV);
	Y := piwl.Y + diff.out;

	(* limit the output *)
	IF Y < LIM_L THEN
		LIM := TRUE;
		Y := LIM_L;
	ELSIF Y > LIM_H THEN
		LIM := TRUE;
		Y := LIM_H;
	ELSE
		LIM := FALSE;
	END_IF;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FT_PIW
VAR_INPUT
	IN : REAL;
	KP : REAL := 1.0;
	KI : REAL := 1.0;
	LIM_L : REAL := -1E38;
	LIM_H : REAL := 1E38;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Y : REAL;
	LIM : BOOL;
END_VAR
VAR
	integ : FT_INT;
END_VAR


(* run integrator *)
integ(IN := IN, K := KI, RUN := NOT LIM, RST := RST);

(* set output value *)
Y := KP * IN + integ.Out;

(* check for limits and set integrator for anti wind up *)
IF Y < LIM_L THEN
	Y := LIM_L;
	LIM := TRUE;
ELSIF Y > LIM_H THEN
	Y := LIM_H;
	LIM := TRUE;
ELSE
	LIM := FALSE;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FT_PIWL
VAR_INPUT
	IN : REAL;
	KP : REAL := 1.0;
	KI : REAL := 1.0;
	LIM_L : REAL := -1.0E38;
	LIM_H : REAL := 1.0E38;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Y : REAL;
	LIM : BOOL;
END_VAR
VAR
	init: BOOL;
	tx: DWORD;
	tc : REAL;
	t_last: DWORD;
	in_last : REAL;
	i: REAL;
	p: REAL;
END_VAR


(* initialize at power_up *)
IF NOT init OR RST THEN
	init := TRUE;
	in_last := in;
	t_last := T_PLC_US();
	i := 0.0;
	tc := 0.0;
ELSE
	(* read last cycle time in Microseconds *)
	tx := T_PLC_US();
	tc := DWORD_TO_REAL(ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(t_last)) MOD 4294967296));
	t_last := tx;

	(* calculate proportional part *)
	p := KP * IN;

	(* run integrator *)
	i := (IN + in_last) * 5.0E-7 * KI * tc + i;
	in_last := IN;

	(* calculate output Y *)
	Y := p + i;

	(* check output for limits *)
	IF Y >= LIM_H THEN
		Y := LIM_H;
		IF ki <> 0.0 THEN
			i := LIM_H - p;
		ELSE
			i := 0.0;
		END_IF;
		LIM := TRUE;
	ELSIF Y <= LIM_L THEN
		Y := LIM_L;
		IF ki <> 0.0 THEN
			i := LIM_L - p;
		ELSE
			i := 0.0;
		END_IF;
		LIM := TRUE;
	ELSE
		LIM := FALSE;
	END_IF;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FT_PT1
VAR_INPUT
	in : REAL;
	T : TIME;
	K : REAL := 1.0;
END_VAR
VAR_OUTPUT
	out : REAL;
END_VAR
VAR
	last : DWORD;
	tx: DWORD;
	init: BOOL;
END_VAR

(* read system time *)
tx := T_PLC_US();

(* startup initialisation *)
IF NOT init OR T = t#0s THEN
	init := TRUE;
	out := K * in;
ELSE
	out := out + (in * K - out) * DWORD_TO_REAL(ULINT_TO_DWORD((DWORD_TO_ULINT(Tx) + 4294967296 - DWORD_TO_ULINT(last)) MOD 4294967296)) / TIME_TO_REAL(T) * 1.0E-3;
	IF ABS(out) < 1.0E-20 THEN out := 0.0; END_IF;
END_IF;
last := tx;


(*
hm 1.1.2007	rev 1.1
	corrected error while startup value was not correct
	for very small time values real output would run out of range.

hm 3.1.2007	rev 1.2
	corrected an error for falling edge failures.
	added output faktor K.

hm 27. 2. 2007	rev 1.3
	output will be input during init for definitive startup condition.

hm	15.9.2007	rev 1.4
	changed time() to T_PLC_US() for compatibilitxy resons
	increased internal accuracy to Microseconds instead of Milliseconds

hm	23. oct 2007	rev 1.5
	added out := in to the init statements

hm	30. nov 2007	rev 1.6
	changed out to be K * in during initialization

hm	5. jan 2008	rev 1.7
	improved code for better performance

hm	16. mar. 2008	rev 1.8
	added type conversion to avoid warning under codesys 3.0

hm	14. jun. 2008	rev 1.9
	improved code

hm	11. mar. 2009	rev 1.10
	real constants updated to new systax using dot

hm	18. jan. 2011	rev 1.11
	avoid underrun of out
*)
END_FUNCTION_BLOCK

FUNCTION_BLOCK FT_PT2
VAR_INPUT
	in : REAL;
	T : TIME;
	D : REAL;
	K : REAL := 1.0;
END_VAR
VAR_OUTPUT
	out : REAL;
END_VAR
VAR
	init: BOOL;
	int1 : INTEGRATE;
	int2 : INTEGRATE;
	tn: REAL;
	I1, I2 : REAL;
	tn2: REAL;
END_VAR

(* startup initialisation *)
IF NOT init OR T = T#0s THEN
	init := TRUE;
	out := K * in;
	I2 := out;
ELSE
	TN := TIME_TO_REAL(T) * 1.0E-3;
	tn2 := TN * TN;
	int1(X := in * K / tn2 - I1 * 0.5 * D / TN - I2 / TN2, Y := I1);
	int2(X := I1,Y := I2);
	out := I2;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FT_TN16
VAR_INPUT
	in : REAL;
	T : TIME;
END_VAR
VAR_OUTPUT
	out : REAL;
	trig : BOOL;
END_VAR
VAR
	length : INT := 16;
	X : ARRAY[0..15] OF REAL;
	cnt : INT;
	last : TIME;
	tx: TIME;
	init: BOOL;
END_VAR

(* read system time *)
tx := DWORD_TO_TIME(T_PLC_MS());

trig := FALSE;
IF NOT init THEN
	x[cnt] := in;
	init := TRUE;
	last := tx;
ELSIF tx - last >= T / length THEN
	IF cnt = length - 1 THEN cnt := 0; ELSE cnt := cnt + 1; END_IF;
	Out := X[cnt];
	x[cnt] := in;
	last := tx;
	Trig := TRUE;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FT_TN64
VAR_INPUT
	in : REAL;
	T : TIME;
END_VAR
VAR_OUTPUT
	out : REAL;
	trig: BOOL;
END_VAR
VAR
	length : INT := 64;
	X : ARRAY[0..63] OF REAL;
	cnt : INT;
	last : TIME;
	tx: TIME;
	init: BOOL;
END_VAR

(* read system time *)
tx := DWORD_TO_TIME(T_PLC_MS());

trig := FALSE;
IF NOT init THEN
	x[cnt] := in;
	init := TRUE;
	last := tx;
ELSIF tx - last >= T / length THEN
	IF cnt = length - 1 THEN cnt := 0; ELSE cnt := cnt + 1; END_IF;
	Out := X[cnt];
	x[cnt] := in;
	last := tx;
	trig := TRUE;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FT_TN8
VAR_INPUT
	in : REAL;
	T : TIME;
END_VAR
VAR_OUTPUT
	out : REAL;
	trig: BOOL;
END_VAR
VAR
	length : INT := 8;
	X : ARRAY[0..7] OF REAL;
	cnt : INT;
	last : TIME;
	tx: TIME;
	init: BOOL;
END_VAR

tx := DWORD_TO_TIME(T_PLC_MS());
trig := FALSE;
IF NOT init THEN
	x[cnt] := in;
	init := TRUE;
	last := tx;
ELSIF tx - last >= T / length THEN
	IF cnt = length - 1 THEN cnt := 0; ELSE cnt := cnt + 1; END_IF;
	Out := X[cnt];
	x[cnt] := in;
	last := tx;
	trig := TRUE;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK HYST
VAR_INPUT
	In : REAL;
	ON : REAL;
	OFF : REAL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	win : BOOL;
END_VAR

IF ON >= OFF THEN
	IF IN < OFF THEN
		Q := FALSE;
		WIN := FALSE;
	ELSIF  IN > ON THEN
		Q := TRUE;
		WIN := FALSE;
	ELSE
		WIN := TRUE;
	END_IF;
ELSE
	IF IN > OFF THEN
		Q := FALSE;
		WIN := FALSE;
	ELSIF  IN < ON THEN
		Q := TRUE;
		WIN := FALSE;
	ELSE
		WIN := TRUE;
	END_IF;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK HYST_1
VAR_INPUT
	In : REAL;
	high : REAL;
	low : REAL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	win : BOOL;
END_VAR

IF in < low THEN
	Q := FALSE;
	win := FALSE;
ELSIF in > high THEN
	Q := TRUE;
	win := FALSE;
ELSE
	win := TRUE;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK HYST_2
VAR_INPUT
	IN : REAL;
	VAL : REAL;
	HYS : REAL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	WIN: BOOL;
END_VAR
VAR
	tmp: REAL;
END_VAR

tmp := val - hys * 0.5;
IF in < tmp THEN
	Q := FALSE;
	win := FALSE;
ELSIF in > tmp + hys THEN
	Q := TRUE;
	win := FALSE;
ELSE
	win := TRUE;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK HYST_3
VAR_INPUT
	in : REAL;
	hyst : REAL;
	val1 : REAL;
	val2 : REAL;
END_VAR
VAR_OUTPUT
	Q1 : BOOL;
	Q2 : BOOL;
END_VAR
VAR
	X: REAL;
END_VAR

X := hyst * 0.5;
IF in < val1 - X THEN
	q1 := TRUE;
ELSIF in > val1 + X THEN
	q1 := FALSE;
END_IF;
IF in < val2 - X THEN
	q2 := FALSE;
ELSIF in > val2 + X THEN
	q2 := TRUE;
END_IF;

END_FUNCTION_BLOCK

FUNCTION_BLOCK INTEGRATE
VAR_INPUT
	E : BOOL := TRUE;
	X : REAL;
	K : REAL := 1.0;
END_VAR
VAR_IN_OUT
	Y : REAL;
END_VAR
VAR
	X_last : REAL;
	init: BOOL;
	last: DWORD;
	tx: DWORD;
END_VAR

(*read system time *)
tx := T_PLC_MS();

IF NOT init THEN
	init := TRUE;
	X_last := X;
ELSIF E THEN
	Y := (X + X_LAST) * 0.5E-3 * DWORD_TO_REAL(ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(last)) MOD 4294967296)) * K + Y;
	X_last := X;
END_IF;
last := tx;



(*
hm 3. nov. 2008	rev 1.0
original version
	
*)

END_FUNCTION_BLOCK

FUNCTION_BLOCK ASTRO
VAR_INPUT
	m : REAL;
	AE : REAL;
	PC : REAL;
	LJ : REAL;
END_VAR
VAR_OUTPUT
	Ym : REAL;
	YAE : REAL;
	YPC : REAL;
	YLJ : REAL;
END_VAR

YAE :=	AE
		+ m * 6.6845871535E-012
		+ PC * 206265.0
		+ LJ * 63240.0;
Ym := YAE * 149.597870E9;
YPC := YAE * 4.8481322570E-006;
YLJ := YAE * 1.5812776724E-005;


(*
L�nge Meter m SI-Basiseinheit
Astronomische Einheit* AE 1 AE = 149,597 870 � E9 m
Parsec pc 1 pc = 206265 AE = 30,857 � E15 m
Lichtjahr Lj 1 Lj = 9,460 530 � E15 m = 63240 AE = 0,306 59 pc
�ngstr�m � 1 � = E�l0 m
typograph. Punkt p 1 p = 0,376 065 mm � im Druckereigewerbe
inch** in 1 in = 2,54 � E�2 m = 25,4 mm***
foot ft 1 ft = 0,3048 m = 30,48 cm
yard yd 1 yd = 0,9144 m
mile mile 1 mile = 1609,344 m
Internat. Seemeile sm 1 sm = 1852 m
Fathom fm 1 fm = 1,829 m � in der Seeschifffahrt
*)

END_FUNCTION_BLOCK

FUNCTION BFT_TO_MS : REAL
VAR_INPUT
	BFT : INT;
END_VAR

BFT_TO_MS := EXPT(BFT, 1.5) * 0.836;
END_FUNCTION

FUNCTION C_TO_F : REAL
VAR_INPUT
	celsius : REAL;
END_VAR

C_TO_F := celsius * 1.8 + 32.0;
END_FUNCTION

FUNCTION C_TO_K : REAL
VAR_INPUT
	Celsius : REAL;
END_VAR

C_TO_K := Celsius - phys.T0;

END_FUNCTION

FUNCTION DEG_TO_DIR : STRING(3)
VAR_INPUT
	DEG : INT;
	N : INT;
	L : INT;
END_VAR
VAR
	ly: INT;
END_VAR

IF L = 0 THEN ly := LANGUAGE.DEFAULT; ELSE ly := MIN(L, LANGUAGE.LMAX); END_IF;
DEG_TO_DIR := LANGUAGE.DIRS[ly, ((WORD_TO_INT(SHL(INT_TO_WORD(DEG),N-1)) + 45) / 90) MOD WORD_TO_INT(SHL(WORD#2,N))*WORD_TO_INT(SHR(WORD#8,N))];



(*
DIR := ((SHL(DEG,N-1) + 45) / 90) MOD SHL(INT#2,N);
explanation :
DIR is calculated BY the following formula:
DIR := ((DIR + 45) / 90) MOD 4 if N = 1 digit
North = 0, East = 1 ....
DIR := ((DIR + 22,5) / 45) MOD 8 if N = 2 digit
convert to integer calculation
DIR := ((DIR*2 + 45) / 90) MOD 8
N = 0, NE = 1 ....
ther above formula replaces 2^N with shift for performance
*)



(* revision histroy
hm	11. jun. 2008	rev 1.0
	original release

hm	22. oct. 2008	rev 1.1
	changed size of string variables to 30
*)
END_FUNCTION

FUNCTION DIR_TO_DEG : INT
VAR_INPUT
	DIR : STRING(3);
	L : INT;
END_VAR
VAR
	ly: INT;
	i : INT;
END_VAR

IF L = 0 THEN ly := LANGUAGE.DEFAULT; ELSE ly := MIN(L, LANGUAGE.LMAX); END_IF;
FOR i := 0 TO 15 DO
	IF language.DIRS[ly, i] = DIR THEN EXIT; END_IF;
END_FOR;
DIR_TO_DEG := SHR(i * 45 + 1, 1);






(* revision histroy
hm	22. oct. 2008	rev 1.1
	original release

*)
END_FUNCTION

FUNCTION_BLOCK ENERGY
VAR_INPUT
	J : REAL;
	C : REAL;
	Wh : REAL;
END_VAR
VAR_OUTPUT
	YJ : REAL;
	YC : REAL;
	YWh : REAL;
END_VAR

YJ := J + Wh * 3600.0 + C * 4.1868;
YC := YJ * 0.238845896627496;
YWh := YJ * 2.7777777778E-004;

(*
Arbeit, Energie, Joule* J 1 J = 1 N � m = 1 W � s = (1/3,6) E�6 kW � h = 1 kg � m2/s2
W�rmemenge Kilowattstunde kW � h 1 kW � h = 3,6 MJ = 860 kcal
Elektronvolt eV 1 eV = 160,218 92 E�21 J
Erg erg 1 erg = 1E�7 J
Kalorie calorie 1 calalorie = 4,1868 J = 1,163 E�3 W � h
Therm therm 1 therm = 105,50 � 106 J
*)
END_FUNCTION_BLOCK

FUNCTION F_TO_C : REAL
VAR_INPUT
	fahrenheit : REAL;
END_VAR

F_TO_C := (fahrenheit - 32.0) * 0.5555555555555;
END_FUNCTION

FUNCTION F_TO_OM : REAL
VAR_INPUT
	F : REAL;
END_VAR

F_TO_OM := math.PI2 * F;
END_FUNCTION

FUNCTION F_TO_PT : TIME
VAR_INPUT
	F : REAL;
END_VAR

F_TO_PT := DWORD_TO_TIME(REAL_TO_DWORD(1.0 / F * 1000.0));

END_FUNCTION

FUNCTION GEO_TO_DEG : REAL
VAR_INPUT
	D : INT;
	M : INT;
	SEC : REAL;
END_VAR

GEO_TO_DEG := INT_TO_REAL(D) + INT_TO_REAL(M) * 0.016666666666667 + sec * 0.00027777777777778;


(* revision histroy
hm	22. jan. 2009	rev 1.0
	original release


*)
END_FUNCTION

FUNCTION K_TO_C : REAL
VAR_INPUT
	Kelvin : REAL;
END_VAR

K_TO_C := Kelvin + phys.T0;

END_FUNCTION

FUNCTION KMH_TO_MS : REAL
VAR_INPUT
	kmh : REAL;
END_VAR

KMH_TO_MS := kmh * 0.2777777777777;


END_FUNCTION

FUNCTION_BLOCK LENGTH
VAR_INPUT
	m : REAL;
	p : REAL;
	in : REAL;
	ft : REAL;
	yd : REAL;
	mile : REAL;
	sm : REAL;
	fm : REAL;
END_VAR
VAR_OUTPUT
	Ym : REAL;
	Yp : REAL;
	Yin : REAL;
	Yft : REAL;
	Yyd : REAL;
	Ymile : REAL;
	Ysm : REAL;
	Yfm : REAL;
END_VAR

Ym :=	m
		+ p * 0.000376065
		+ in * 0.0254
		+ ft * 0.3048
		+ yd * 0.9144
		+ mile * 1609.344
		+ sm * 1852.0
		+ fm * 1.829;
Yp := Ym * 2659.11478068951;
Yin := Ym * 39.37007874016;
Yft := Ym * 3.28083989501;
Yyd := Ym * 1.09361329834;
Ymile := Ym * 0.00062137119;
Ysm := Ym * 0.00053995680;
Yfm := Ym * 0.54674685621;

(*
L�nge Meter m SI-Basiseinheit
Astronomische Einheit* AE 1 AE = 149,597 870 � E9 m
Parsec pc 1 pc = 206265 AE = 30,857 � E15 m
Lichtjahr Lj 1 Lj = 9,460 530 � E15 m = 63240 AE = 0,306 59 pc
�ngstr�m � 1 � = E�l0 m
typograph. Punkt p 1 p = 0,376 065 mm � im Druckereigewerbe
inch** in 1 in = 2,54 � E�2 m = 25,4 mm***
foot ft 1 ft = 0,3048 m = 30,48 cm
yard yd 1 yd = 0,9144 m
mile mile 1 mile = 1609,344 m
Internat. Seemeile sm 1 sm = 1852 m
Fathom fm 1 fm = 1,829 m � in der Seeschifffahrt
*)

END_FUNCTION_BLOCK

FUNCTION MS_TO_BFT : INT
VAR_INPUT
	MS : REAL;
END_VAR

MS_TO_BFT := REAL_TO_INT(EXPT(MS * 1.196172, 0.666667));
END_FUNCTION

FUNCTION MS_TO_KMH : REAL
VAR_INPUT
	ms : REAL;
END_VAR

MS_TO_KMH := ms * 3.6;


END_FUNCTION

FUNCTION OM_TO_F : REAL
VAR_INPUT
	OM : REAL;
END_VAR

OM_TO_F := OM / math.PI2;
END_FUNCTION

FUNCTION_BLOCK PRESSURE
VAR_INPUT
	mws : REAL;
	torr : REAL;
	att : REAL;
	atm : REAL;
	pa : REAL;
	bar : REAL;
END_VAR
VAR_OUTPUT
	Ymws : REAL;
	Ytorr : REAL;
	Yatt : REAL;
	Yatm : REAL;
	Ypa : REAL;
	Ybar : REAL;
END_VAR

Ybar := bar +
		pa * 1.0E-5 +
		0.980665 * att +
		1.01325 * atm +
		0.001333224 * torr +
		0.0980665 * mws;
Ymws := ybar * 10.1971621297793;
Ytorr := ybar * 750.0615050434140;
Yatt := ybar * 1.0197162129779;
yatm := ybar * 0.9869232667160;
Ypa := ybar * 100000.0;


(*
Druck, Pascal Pa 1 Pa = 1 N/m2 = 1 kg/(s2 �E m) . 0,75 �E 10.2 mmHg
mechanische 1 MPa = 1 N/mm2 . fur Festigkeitsangaben
Spannung Bar bar 1 bar = 105 Pa = 103 mbar = 105 kg/(s2 �E m)
Millimeter- mmHg 1 mmHg = 133,322 Pa = 1,333 22 mbar
Quecksilbersaule . nur in Heilkunde zulassig
physik. Atmosphare atm 1 atm = 1,013 25 bar
techn. Atmosphare at 1 at = 1 kp/cm2 = 0,980665 bar
Torr Torr 1 Torr = (101325/760) Pa = 1,333224 mbar
Meter-Wassersaule mWS 1 mWS = 9806,65 Pa = 98,0665 mbar
psi lbf/in2 1 lbf/in2 = 68,947 57 mbar = 6894,757 Pa

*)
END_FUNCTION_BLOCK

FUNCTION PT_TO_F : REAL
VAR_INPUT
	PT : TIME;
END_VAR

PT_TO_F := 1000.0 / DWORD_TO_REAL(TIME_TO_DWORD(PT));

END_FUNCTION

FUNCTION_BLOCK SPEED
VAR_INPUT
	ms : REAL;
	kmh : REAL;
	kn : REAL;
	mh : REAL;
END_VAR
VAR_OUTPUT
	Yms : REAL;
	Ykmh : REAL;
	Ykn : REAL;
	Ymh : REAL;
END_VAR

Yms := ms +
		kmh * 0.27777777777778 +
		kn * 0.5144444 +
		mh * 0.44704;
Ykmh := Yms * 3.6;
Ykn := Yms * 1.94384466037535;
Ymh := Yms * 2.23693629205440;

(*
Geschwindigkeit Meter durch Sekunde m/s 1 m/s = 3,6 km/h
km durch (pro) Stunde, nicht �Stundenkilometer� verwenden
Knoten kn 1 kn = 1 sm/h = 0,5144 m/s

*)
END_FUNCTION_BLOCK

FUNCTION_BLOCK TEMPERATURE
VAR_INPUT
	K : REAL;
	C : REAL := -273.15;
	F : REAL := -459.67;
	Re : REAL := -218.52;
	Ra : REAL;
END_VAR
VAR_OUTPUT
	YK : REAL;
	YC : REAL;
	YF : REAL;
	YRe : REAL;
	YRa : REAL;
END_VAR

YK := K + (C + 273.15) + (F + 459.67) * 0.555555555555 + (Re * 1.25 + 273.15) + (Ra * 0.555555555555);
YC := YK -273.15;
YF := YK * 1.8 - 459.67;
YRe := (YK - 273.15) * 0.8;
YRa := YK * 1.8;
END_FUNCTION_BLOCK

FUNCTION_BLOCK ALARM_2
VAR_INPUT
	X : REAL;
	LO_1 : REAL;
	HI_1 : REAL;
	LO_2 : REAL;
	HI_2 : REAL;
	HYS : REAL;
END_VAR
VAR_OUTPUT
	Q1_LO : BOOL;
	Q1_HI : BOOL;
	Q2_LO : BOOL;
	Q2_HI : BOOL;
END_VAR
VAR
	tmp: REAL;
END_VAR

tmp := X - Hys * 0.5;
IF tmp > LO_1 THEN Q1_LO := FALSE; END_IF;
IF tmp > LO_2 THEN Q2_LO := FALSE; END_IF;
IF tmp > HI_1 THEN Q1_HI := TRUE; END_IF;
IF tmp > HI_2 THEN Q2_HI := TRUE; END_IF;
tmp := tmp + hys;
IF tmp < LO_1 THEN Q1_LO := TRUE; END_IF;
IF tmp < LO_2 THEN Q2_LO := TRUE; END_IF;
IF tmp < HI_1 THEN Q1_HI := FALSE; END_IF;
IF tmp < HI_2 THEN Q2_HI := FALSE; END_IF;

END_FUNCTION_BLOCK

FUNCTION_BLOCK BAR_GRAPH
VAR_INPUT
	X : REAL;
	rst : BOOL;
END_VAR
VAR_INPUT CONSTANT
	trigger_Low : REAL;
	trigger_High : REAL;
	Alarm_low : BOOL;
	Alarm_high : BOOL;
	log_scale : BOOL;
END_VAR
VAR_OUTPUT
	LOW, Q1, Q2, Q3, Q4, Q5, Q6, HIGH : BOOL;
	Alarm : BOOL;
	Status : BYTE;
END_VAR
VAR
	init : BOOL;
	T1, T2, T3, T4, T5 : REAL;
END_VAR
VAR_TEMP
	temp : REAL;
END_VAR

IF NOT init THEN
	init := TRUE;
	IF log_scale THEN
		temp := EXP(LN(Trigger_high / Trigger_low) * 0.166666666666666666666);
		T1 := trigger_low * temp;
		T2 := T1 * temp;
		T3 := T2 * temp;
		T4 := T3 * temp;
		T5 := T4 * temp;
	ELSE
		temp := (trigger_high - trigger_low) * 0.142857142;
		T1 := trigger_low + temp;
		T2 := T1 + temp;
		T3 := T2 + temp;
		T4 := T3 + temp;
		T5 := T4 + temp;
	END_IF;
END_IF;

(* clear outputs before checking *)
Q1 := FALSE;
Q2 := FALSE;
Q3 := FALSE;
Q4 := FALSE;
Q5 := FALSE;
Q6 := FALSE;
status := 110;

(* low, high and alarm can only be cleared with rst depending on alarm_low and alarm_high *)
IF NOT alarm_low THEN low := FALSE; END_IF;
IF NOT alarm_high THEN high := FALSE; END_IF;
IF rst THEN
	alarm := FALSE;
	low := FALSE;
	high := FALSE;
END_IF;

(* check and set outputs *)
IF X < trigger_low THEN
	Low := TRUE;
	status := 111;
	IF alarm_low THEN
		alarm := TRUE;
		status := 1;
	END_IF;
ELSIF X < T1 THEN
	Q1 := TRUE;
ELSIF x < t2 THEN
	Q2 := TRUE;
ELSIF x < t3 THEN
	Q3 := TRUE;
ELSIF x < T4 THEN
	Q4 := TRUE;
ELSIF x < T5 THEN
	q5 := TRUE;
ELSIF x < trigger_high THEN
	q6 := TRUE;
ELSE
	high := TRUE;
	status := 112;
	IF alarm_high THEN
		alarm := TRUE;
		status := 2;
	END_IF;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK CALIBRATE
VAR_INPUT
	X : REAL;
	CO : BOOL;
	CS : BOOL;
END_VAR
VAR_INPUT CONSTANT
	Y_Offset : REAL;
	Y_Scale : REAL;
END_VAR
VAR_OUTPUT
	Y : REAL;
END_VAR
VAR RETAIN
	offset : REAL;
	Scale : REAL := 1.0;
END_VAR

(* check for calibration *)
IF CO THEN
	OFFSET := Y_Offset - X;
ELSIF CS THEN
	SCALE := Y_scale / (X + OFFSET);
END_IF;
(* calculate output *)
Y := (X + OFFSET) * SCALE;
END_FUNCTION_BLOCK

FUNCTION_BLOCK CYCLE_TIME
VAR_INPUT
	RST : BOOL;
END_VAR
VAR_OUTPUT
	ct_min : TIME;
	ct_max : TIME;
	ct_last : TIME;
	systime : TIME;
	sysdays : INT;
	cycles : DWORD;
END_VAR
VAR
	last_cycle : TIME;
	tx: TIME;
	init: BOOL;
END_VAR

tx := DWORD_TO_TIME(T_PLC_MS()) - last_cycle;
IF rst THEN
	ct_min := t#10h;
	ct_max := t#0ms;
	cycles := 0;
ELSIF last_cycle > t#0s THEN
	IF tx < ct_min THEN ct_min := tx;
	ELSIF tx > ct_max THEN ct_max := tx;
	END_IF;
	ct_last := tx;
ELSIF ct_min = t#0s THEN
	ct_min := t#0s - t#1ms;
END_IF;
IF init THEN
	systime := systime + tx;
		IF systime >= t#1d THEN
			systime := systime - t#1d;
			sysdays := sysdays + 1;
		END_IF;
END_IF;
init := TRUE;
last_cycle := last_cycle + tx;
cycles := ULINT_TO_DWORD((DWORD_TO_ULINT(cycles) + 1) MOD 4294967296);
END_FUNCTION_BLOCK

FUNCTION_BLOCK DT_SIMU
VAR_INPUT
	START : DT;
	SPEED : REAL := 1.0;
END_VAR
VAR_OUTPUT
	DTS : DT;
END_VAR
VAR
	tc : DWORD;
	init: BOOL;
	last: DWORD;
	tx: DWORD;
	td: DWORD;
END_VAR

(* read system timer *)
tx := T_PLC_MS();
tc := REAL_TO_DWORD(DWORD_TO_REAL(ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(last)) MOD 4294967296)) * speed);

IF NOT init THEN
	init := TRUE;
	DTS := Start;
	tc := 0;
	last := tx;
ELSIF SPEED = 0.0 THEN
	DTS := DWORD_TO_DT(ULINT_TO_DWORD((DT_TO_ULINT(DTS) + 1) MOD 4294967296));
ELSIF tc >= 1000 THEN
	td := ULINT_TO_DWORD((DWORD_TO_ULINT(tc) / 1000) * 1000);
	DTS := DTS + DWORD_TO_TIME(td);
	last := ULINT_TO_DWORD((DWORD_TO_ULINT(last) + REAL_TO_ULINT(DWORD_TO_REAL(td) / speed)) MOD 4294967296);
END_IF;

END_FUNCTION_BLOCK

FUNCTION_BLOCK FLOW_METER
VAR_INPUT
	VX : REAL;
	E : BOOL;
	RST : BOOL;
END_VAR
VAR_INPUT CONSTANT
	PULSE_MODE : BOOL;
	UPDATE_TIME : TIME := t#1s;
END_VAR
VAR_OUTPUT
	F : REAL;
END_VAR
VAR_IN_OUT
	X : REAL;
	Y : UDINT;
END_VAR
VAR
	tx, tl : TIME;
	int1 : INTEGRATE;
	init: BOOL;
	e_last : BOOL;
	tmp: INT;
	x_last : REAL;
	y_last : UDINT;
END_VAR


IF NOT init THEN	(* init on power up *)
	init := TRUE;
	tl := tx;
	x_last := X;
	y_last := Y;
	int1.K := 2.7777777777777777E-4;
END_IF;

(* run integrator *)
int1(E := NOT (RST OR PULSE_MODE) AND E, X := VX, Y := X);	(* gated operation *)

IF RST THEN		(* reset *)
	X := 0.0;
	Y := 0;
	tl := tx;
	x_last := 0.0;
	y_last := 0;
ELSIF E AND PULSE_MODE THEN	(* check for pulse mode *)
	IF NOT e_last THEN X := X + VX; END_IF;
END_IF;
e_last := E;

(* reduce X to be less than 1 and increase Y respectively *)
IF X > 1.0 THEN
	tmp := FLOOR(X);
	Y := Y + INT_TO_UDINT(tmp);
	X := X - INT_TO_REAL(tmp);
END_IF;

(* calculate the current flow *)
tx := DWORD_TO_TIME(T_PLC_MS());
IF tx - tl >= UPDATE_TIME AND UPDATE_TIME > t#0s THEN
	F := (UDINT_TO_REAL(Y - y_last) + X - x_last) / TIME_TO_REAL(tx - tl) * 3.6E6;
	y_last := Y;
	x_last := X;
	tl := tx;
END_IF;

END_FUNCTION_BLOCK

FUNCTION_BLOCK M_D
VAR_INPUT
	start : BOOL;
	stop : BOOL;
	tmax : TIME := t#10d;
	rst: BOOL;
END_VAR
VAR_OUTPUT
	PT : TIME;
	ET : TIME;
	run : BOOL;
END_VAR
VAR
	edge : BOOL;
	T0 : TIME;
	tx: TIME;
	startup: BOOL;
END_VAR

(* check for rst input *)
IF rst OR et >= tmax THEN
	pt := t#0ms;
	et := t#0ms;
	startup := FALSE;
	run := FALSE;
END_IF;

(* avoid timers to start when input is true at startup *)
IF NOT startup THEN
	edge := start;
	startup := TRUE;
END_IF;

(* read system timer *)
tx := DWORD_TO_TIME(T_PLC_MS());

(* detect rising edge on start *)
IF start AND NOT edge AND NOT stop THEN
	t0 := tx;
	run := TRUE;
	pt := t#0s;
ELSIF stop AND run THEN
	pt := et;
	run := FALSE;
END_IF;
edge := start;
IF run THEN et := tx - t0; END_IF;

END_FUNCTION_BLOCK

FUNCTION_BLOCK M_T
VAR_INPUT
	IN : BOOL;
	TMAX : TIME := t#10d;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	PT : TIME;
	ET: TIME;
END_VAR
VAR
	edge : BOOL;
	start : TIME;
	tx: TIME;
END_VAR

(* read system time *)
tx := DWORD_TO_TIME(T_PLC_MS());

IF RST OR ET >= TMAX THEN
	PT :=  t#0s;
	ET := t#0s;
ELSIF IN THEN
	IF NOT edge THEN start := tx; END_IF;
	ET := tx - start;
ELSE
	PT := ET;
END_IF;
edge := IN;
END_FUNCTION_BLOCK

FUNCTION_BLOCK M_TX
VAR_INPUT
	in: BOOL;
	tmax : TIME := t#10d;
	rst : BOOL;
END_VAR
VAR_OUTPUT
	TH : TIME;
	TL : TIME;
	DC : REAL;
	F: REAL;
	ET : TIME;
END_VAR
VAR
	edge : BOOL;
	start: TIME;
	stop:TIME;
	tx: TIME;
	rise : BOOL;
	fall : BOOL;
	startup: BOOL;
END_VAR

(* check FOR rst input *)
IF rst OR (et >= tmax) THEN
	rise := FALSE;
	fall := FALSE;
	startup := FALSE;
	th := t#0ms;
	tl := t#0ms;
	DC := 0.0;
	F := 0.0;
	ET := t#0s;
END_IF;

(* avoid timers to start when input is true at startup *)
IF NOT startup THEN
	edge := in;
	startup := TRUE;
END_IF;

(* read system timer *)
tx := DWORD_TO_TIME(T_PLC_MS());

(* edge trigger rising and falling edge *)
IF in XOR edge THEN
	edge := in;
	(* rising edge starts measurement *)
	IF in THEN
		start := Tx;
		rise := TRUE;
		IF fall THEN tl := start - stop; END_IF;
		IF th > t#0ms AND tl > t#0ms THEN
			dc := TIME_TO_REAL(th) / TIME_TO_REAL(th+tl);
			f := 1000.0 / TIME_TO_REAL(th + tl);
		END_IF;
	(* falling edge starts second cycle measurement *)
	ELSE
		stop := Tx;
		fall := TRUE;
		IF rise THEN th := stop - start; END_IF;
		IF th > t#0ms AND tl > t#0ms THEN
			dc := TIME_TO_REAL(th) /  TIME_TO_REAL(th+tl);
			f := 1000.0 / TIME_TO_REAL(th + tl);
		END_IF;
	END_IF;
END_IF;
IF rise THEN et := tx - start; END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK METER
VAR_INPUT
	M1, M2 : REAL;
	I1, I2 : BOOL;
	D : REAL := 1.0;
	RST : BOOL;
END_VAR
VAR_IN_OUT
	MX : REAL;
END_VAR
VAR
	MR : REAL2;
	MX1, MX2 : REAL; (* current consumption value on M1 and M2 *)
	tx: DWORD;
	last: DWORD;
	tc: REAL;
	init: BOOL;
END_VAR

(* measure the last cycle time and make sure we onle execute once every millisecond *)
tx := T_PLC_MS();
IF NOT init THEN
	init := TRUE;
	last := tx;
	mr.RX := mx;
	mr.R1 := 0.0;
ELSIF tx = last THEN
	RETURN;
ELSE
	tc := DWORD_TO_REAL(ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(last)) MOD 4294967296)) * 0.001;
END_IF;
last := tx;

(* reset *)
IF rst THEN
	mr.R1 := 0.0;
	mr.RX := 0.0;
ELSE
	(* current consumption measurement *)
	IF I1 THEN MX1 := M1; ELSE MX1 := 0.0; END_IF;
	IF I2 THEN MX2 := M2; ELSE MX2 := 0.0; END_IF;
	(* add up the current values in a double real *)
	MR := R2_ADD(MR,(SEL(I1,0.0,mx1) + SEL(I2, 0.0, mx2)) / D * TC);
	(* set the current output value *)
	MX := mr.RX;
END_IF;

END_FUNCTION_BLOCK

FUNCTION_BLOCK METER_STAT
VAR_INPUT
	IN : REAL;
	DI : DATE;
	RST : BOOL;
END_VAR
VAR_IN_OUT
	Last_Day : REAL;
	Current_Day : REAL;
	Last_Week : REAL;
	Current_Week : REAL;
	Last_Month : REAL;
	Current_Month : REAL;
	Last_Year : REAL;
	Current_Year : REAL;
END_VAR
VAR RETAIN
	Year_Start : REAL;
	Month_Start : REAL;
	Week_Start : REAL;
	Day_Start : REAL;
	last_run: DATE;
END_VAR

IF rst THEN
	Last_Day := 0.0;
	Current_Day := 0.0;
	Day_start := IN;
	Last_week := 0.0;
	Current_week := 0.0;
	Week_start := in;
	Last_month := 0.0;
	Current_month := 0.0;
	month_start := in;
	last_year := 0.0;
	current_year := 0.0;
	year_start := in;
ELSE
	Current_Day := IN - Day_Start;
	Current_Week := In - Week_Start;
	Current_Month := IN - Month_Start;
	Current_Year := IN - Year_Start;
END_IF;

IF YEAR_OF_DATE(DI) > YEAR_OF_DATE(last_run) THEN
	(* a new year has started *)
	last_year := current_year;
	current_year := 0.0;
	year_start := in;
	last_month := current_month;
	current_month := 0.0;
	month_start := in;
	last_day := current_day;
	current_day := 0.0;
	day_start := in;
ELSIF MONTH_OF_DATE(DI) > MONTH_OF_DATE(last_run) THEN
	(* a new month has started, january is alrerady done by year change *)
	last_month := current_month;
	current_month := 0.0;
	month_start := in;
	last_day := current_day;
	current_day := 0.0;
	day_start := in;
ELSIF DAY_OF_YEAR(di) > DAY_OF_YEAR(last_run) THEN
	(* day has chaged, first day of year and first day of month has already been taken care of *)
	last_day := current_day;
	current_day := 0.0;
	day_start := in;
END_IF;
IF DAY_OF_WEEK(DI) < DAY_OF_WEEK(last_run) THEN
	(* a new week has started *)
	last_week := current_week;
	current_week := 0.0;
	week_start := in;
END_IF;
last_run := di;

END_FUNCTION_BLOCK

FUNCTION_BLOCK ONTIME
VAR_INPUT
	IN : BOOL;
	RST : BOOL;
END_VAR
VAR_IN_OUT
	SECONDS : UDINT;
	CYCLES : UDINT;
END_VAR
VAR
	tx: DWORD;
	last : DWORD;
	edge : BOOL;
	init: BOOL;
	ms: DWORD;
END_VAR

(* read system time *)
tx := T_PLC_MS();

(* make sure the first cycle works correctly *)
IF NOT init THEN
	init := TRUE;
	last := tx;
	ms := 0;
END_IF;
IF RST THEN
	SECONDS := 0;
	CYCLES := 0;
	last := tx;
	ms := 0;
ELSIF IN THEN
	(* add the current milliseconds *)
	ms := ULINT_TO_DWORD((DWORD_TO_ULINT(ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(last)) MOD 4294967296)) + DWORD_TO_ULINT(ms)) MOD 4294967296);
	IF ms >= 1000 THEN
		seconds := seconds + 1;
		ms := ULINT_TO_DWORD(DWORD_TO_ULINT(ms) - 1000);
	END_IF;
	cycles := cycles + BOOL_TO_UINT(NOT edge);
END_IF;
last := tx;
edge := in;
END_FUNCTION_BLOCK

FUNCTION T_PLC_MS : DWORD
VAR CONSTANT
	debug : BOOL := 0;
	N : INT := 0;
	offset : DWORD := 0;
END_VAR
VAR
	tx : TIME;
END_VAR

tx := TIME();
T_PLC_MS := ULINT_TO_DWORD(TIME_TO_ULINT(Tx) MOD 4294967296);
(* hier muss die korrektur f�r step7 stattfinden
plctime muss den vollen wertebereich von time ausnutzen:
wenn bei step7 time -24tage bis plus 24 tage ist dann muss der timer auch beim �berlauf auf -24tage springen 
und auf keinen fall auf 0 !!!!
f�r siemens muss ein weiterer fb im main eingebunden werden der sicherstellt das alle 32 bits durchgez�hlt werden.
es kann nur ein fb sein den er muss sich das oberste (32te) bit merken.
oder etwa spring s7 bei �berlauf auf -24 tage????? dann w�re keine korrektur n�tig.
*)
IF debug THEN
	T_PLC_MS := ULINT_TO_DWORD((DWORD_TO_ULINT(SHL(T_PLC_MS,N) OR ULINT_TO_DWORD(DWORD_TO_ULINT(SHL(DWORD#1,N))-1)) + DWORD_TO_ULINT(OFFSET)) MOD 4294967296);
END_IF;
END_FUNCTION

FUNCTION T_PLC_US : DWORD
VAR CONSTANT
	debug : BOOL := 0;
	N : INT := 0;
	offset : DWORD := 0;
END_VAR
VAR
	tx : TIME;
END_VAR

tx := TIME();
T_PLC_US := ULINT_TO_DWORD((TIME_TO_ULINT(Tx)*1000) MOD 4294967296);
(* hier muss die korrektur f�r step7 stattfinden
plctime muss den vollen wertebereich von time ausnutzen:
wenn bei step7 time -24tage bis plus 24 tage ist dann muss der timer auch beim �berlauf auf -24tage springen 
und auf keinen fall auf 0 !!!!
f�r siemens muss ein weiterer fb im main eingebunden werden der sicherstellt das alle 32 bits durchgez�hlt werden.
es kann nur ein fb sein den er muss sich das oberste (32te) bit merken.
oder etwa spring s7 bei �berlauf auf -24 tage????? dann w�re keine korrektur n�tig.
*)
IF debug THEN
	T_PLC_US := ULINT_TO_DWORD((DWORD_TO_ULINT(SHL(T_PLC_US,N) OR ULINT_TO_DWORD(DWORD_TO_ULINT(SHL(DWORD#1,N))-1)) + DWORD_TO_ULINT(OFFSET)) MOD 4294967296);
END_IF;
END_FUNCTION

FUNCTION_BLOCK TC_MS

VAR_OUTPUT
	TC : DWORD;
END_VAR
VAR
	init: BOOL;
	tx: DWORD;
	last: DWORD;
END_VAR

(* read system timer *)
tx := T_PLC_MS();

IF NOT init THEN
	init := TRUE;
	TC := 0;
ELSE
	tc := ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(last)) MOD 4294967296);
END_IF;
last := tx;

END_FUNCTION_BLOCK

FUNCTION_BLOCK TC_S

VAR_OUTPUT
	TC : REAL;
END_VAR
VAR
	init: BOOL;
	tx: DWORD;
	last: DWORD;
END_VAR

(* read system timer *)
tx := T_PLC_US();

IF NOT init THEN
	init := TRUE;
	TC := 0.0;
ELSE
	tc := DWORD_TO_REAL(ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(last)) MOD 4294967296))*1.0E-6;
END_IF;
last := tx;

END_FUNCTION_BLOCK

FUNCTION_BLOCK TC_US

VAR_OUTPUT
	TC : DWORD;
END_VAR
VAR
	init: BOOL;
	tx: DWORD;
	last: DWORD;
END_VAR

(* read system timer *)
tx := T_PLC_US();

IF NOT init THEN
	init := TRUE;
	TC := 0;
ELSE
	tc := ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(last)) MOD 4294967296);
END_IF;
last := tx;

END_FUNCTION_BLOCK

FUNCTION MULTI_IN : REAL
VAR_INPUT
	in_1 : REAL;
	in_2 : REAL;
	in_3 : REAL;
	default : REAL;
	in_min : REAL;
	in_max : REAL;
	mode : BYTE;
END_VAR
VAR
	count: INT;
	F1: BOOL;
	F2: BOOL;
	F3: BOOL;
END_VAR


F1 :=  in_1 > in_min AND in_1 < in_max;
F2 :=  in_2 > in_min AND in_2 < in_max;
F3 :=  in_3 > in_min AND in_3 < in_max;

CASE mode OF
0:	count := 0;
	IF F1 THEN
		count := count + 1;
		MULTI_IN := in_1;
	ELSE
		MULTI_IN := 0.0;
	END_IF;
	IF F2 THEN
		count := count + 1;
		MULTI_IN := MULTI_IN + in_2;
	END_IF;
	IF F3 THEN
		count := count + 1;
		MULTI_IN := MULTI_IN + in_3;
	END_IF;
	MULTI_IN := SEL(count = 0, MULTI_IN / INT_TO_REAL(count), default);

1:	MULTI_IN := SEL(F1, default, IN_1);

2:	MULTI_IN := SEL(F2, default, IN_2);

3:	MULTI_IN := SEL(F3, default, IN_3);

4:	MULTI_IN := default;

5:	MULTI_IN := SEL(F1, in_max, IN_1);
	IF F2 AND in_2 < MULTI_IN THEN MULTI_IN := in_2; END_IF;
	IF F3 AND in_3 < MULTI_IN THEN MULTI_IN := in_3; END_IF;
	IF MULTI_IN = in_max THEN MULTI_IN := default; END_IF;

6:	MULTI_IN := SEL(F1, in_min, IN_1);
	IF F2 AND in_2 > MULTI_IN THEN MULTI_IN := in_2; END_IF;
	IF F3 AND in_3 > MULTI_IN THEN MULTI_IN := in_3; END_IF;
	IF MULTI_IN = in_min THEN MULTI_IN := default; END_IF;

7:	IF F1 AND F2 AND F3 THEN MULTI_IN := MID3(in_1, in_2, in_3);
	ELSIF F1 AND F2 THEN MULTI_IN := MIN(in_1, in_2);
	ELSIF F1 AND F3 THEN MULTI_IN := MIN(in_1, in_3);
	ELSIF F2 AND F3 THEN MULTI_IN := MIN(in_2, in_3);
	ELSIF F1 THEN MULTI_IN := in_1;
	ELSIF F2 THEN MULTI_IN := in_2;
	ELSIF F3 THEN MULTI_IN := in_3;
	ELSE MULTI_IN := default;
	END_IF;

END_CASE;

(*
hm 1.1.2007		rev 1.1
	changed midr to mid3 function

hm	14. 10. 2008	rev 1.2
	corrected an error for in_3 overrange detection
	improved performance

hm	11. mar. 2009	rev 1.3
	changed real constants to use dot syntax

hm	18. jul. 2009	rev 1.4
	improved performance
*)
END_FUNCTION

FUNCTION RES_NI : REAL
VAR_INPUT
	T : REAL;
	R0 : REAL;
END_VAR
VAR CONSTANT
	A : REAL := 0.5485;
	B : REAL := 0.665E-3;
	C : REAL := 2.805E-9;
END_VAR
VAR
	T2: REAL;
END_VAR

T2 := T * T;
RES_NI := R0 + A * T + B * T2 + C * T2 * T2;

(*

 Auszug aus DIN 43760 f�r Ni100

�C		R		�C		R		�C	R		�C		R		�C		R

-60 	69,5 	-10 	94,6 	40 	123,0 	90 		154,9 	140 	190,9
-55 	71,9 	-5 		97,3 	45 	126,0 	95 		158,3 	145 	194,8
-50 	74,3 	0 		100,0 	50 	129,1 	100 	161,8 	150 	198,7
-45 	76,7 	5 		102,8 	55 	132,2 	105 	165,3 	155 	202,6
-40 	79,1 	10 		105,6 	60 	135,3 	110 	168,8 	160 	206,6
-35 	81,6 	15 		108,4 	65 	138,5 	115 	172,4 	165 	210,7
-30 	84,2 	20 		111,2 	70 	141,7 	120 	176,0 	170 	214,8
-25 	86,7 	25 		114,1 	75 	145,0 	125 	179,6 	175 	219,0
-20 	89,3 	30 		117,1 	80 	148,3 	130 	183,3 	180 	223,2
-15 	91,9 	35 		120,0 	85 	151,6 	135 	187,1 	  	 

*)
END_FUNCTION

FUNCTION RES_NTC : REAL
VAR_INPUT
	T : REAL;
	RN : REAL;
	B : REAL;
END_VAR

RES_NTC := RN * EXP(B * (1.0 / (T+273.15) - 0.00335401643468053));
END_FUNCTION

FUNCTION RES_PT : REAL
VAR_INPUT
	T : REAL;
	R0 : REAL;
END_VAR
VAR CONSTANT
	A : REAL := 3.90802E-3 ;
	B : REAL := -5.802E-7;
	C : REAL := -4.27350E-12;
END_VAR
VAR
	T2: REAL;
END_VAR

T2 := T * T;

IF T >= 0.0 THEN
	RES_PT := R0 * ( 1.0 + A * T + B * T2);
ELSE
	RES_PT := R0 * ( 1.0 + A * T + B * T2 + C * (T - 100.0) * T2 * T);
END_IF;


(*
 Auszug aus DIN 43760 f�r Pt100

�C		R		�C		R		�C	R			�C		R			�C		R			�C		R

-200 	18,49 	-100 	60,25 	0 	100,00 	100 	138,50 	200 	175,84 	300 	212,02
-195 	20,65 	-95 	62,28 	5 	101,95 	105 	140,39 	205 	177,68 	305 	213,80
-190 	22,80 	-90 	64,30 	10 	103,90 	110 	142,29 	210 	179,51 	310 	215,57
-185 	24,94 	-85 	66,31 	15 	105,85 	115 	144,17 	215 	181,34 	315 	217,35
-180 	27,08 	-80 	68,33 	20 	107,79 	120 	146,06 	220 	183,17 	320 	219,12
-175 	29,20 	-75 	70,33 	25 	109,73 	125 	147,94 	225 	184,99 	325 	220,88
-170 	31,32 	-70 	72,33 	30 	111,67 	130 	149,82 	230 	186,82 	330 	222,65
-165 	33,43 	-65 	74,33 	35 	113,61 	135 	151,70 	235 	188,63 	335 	224,41
-160 	35,33 	-60 	76,33 	40 	115,54 	140 	153,58 	240 	190,45 	340 	226,17
-155 	37,63 	-55 	78,32 	45 	117,47 	145 	155,45 	245 	192,26 	345 	227,92
-150 	39,71 	-50 	80,31 	50 	119,40 	150 	157,31 	250 	194,07 	350 	229,67
-145 	41,79 	-45 	82,29 	55 	121,32 	155 	159,18 	255 	195,88 	355 	231,42
-140 	43,87 	-40 	84,27 	60 	123,24 	160 	161,04 	260 	197,69 	360 	233,17
-135 	45,94 	-35 	86,25 	65 	125,16 	165 	162,90 	265 	199,49 	365 	234,91
-130 	48,00 	-30 	88,22 	70 	127,07 	170 	164,76 	270 	201,29 	370 	236,65
-125 	50,06 	-25 	90,19 	75 	128,98 	175 	166,61 	275 	203,08 	375 	238,39
-120 	52,11 	-20 	92,16 	80 	130,89 	180 	168,46 	280 	204,88 	380 	240,13
-115 	54,15 	-15 	94,12 	85 	132,80 	185 	170,31 	285 	206,67 	385 	241,86
-110 	56,19 	-10 	96,09 	90 	134,70 	190 	172,16 	290 	208,45 	390 	243,59
-105 	58,22 	-5 		98,04 	95 	136,60 	195 	174,00 	295 	210,24 	395 	245,31

*)
END_FUNCTION

FUNCTION RES_SI : REAL
VAR_INPUT
	T : REAL;
	RS : REAL;
	TS : REAL;
END_VAR
VAR CONSTANT
	A : REAL := 7.64E-3;
	B : REAL := 1.66E-5;
END_VAR
VAR
	TX: REAL;
END_VAR

TX := T - TS;
RES_SI := RS * ( 1.0 + A * TX + B * TX * TX);
END_FUNCTION

FUNCTION SENSOR_INT : REAL
VAR_INPUT
	Voltage : REAL;
	Current : REAL;
	RP : REAL;
	RS : REAL;
END_VAR
VAR
	RG: REAL;
END_VAR

IF current <> 0.0 THEN
	RG := voltage / current;
	SENSOR_INT := RP * ( RG - RS) / ( RP + RS - RG);
END_IF;

(* revisaion history

hm 20.1.2007		rev 1.1
	deleted input R0 which was not needed.

hm	2. dec 2007	rev 1.2
	corrected an error in formula and improoved speed

hm	11. mar. 2009	rev 1.3
	changed real constants to use dot syntax

*)
END_FUNCTION

FUNCTION TEMP_NI : REAL
VAR_INPUT
	Res : REAL;
	R0 : REAL;
END_VAR

TEMP_NI := (SQRT(0.30085225 - 2.66E-3 * (R0 - Res)) - 0.5485) * 751.8796992;


END_FUNCTION

FUNCTION TEMP_NTC : REAL
VAR_INPUT
	RES : REAL;
	RN : REAL;
	B : REAL;
END_VAR

IF res > 0.0 THEN
	TEMP_NTC := B * 298.15 / (B + LN(RES / RN) * 298.15) -273.15;
END_IF;
END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION TEMP_PT : REAL
// VAR_INPUT
// 	Res : REAL;
// 	R0 : REAL;
// END_VAR
// VAR CONSTANT
// 	A : REAL := 3.9083E-3 ;
// 	B : REAL := -5.775E-7;
// 	accuracy : REAL := 0.01;
// END_VAR
// VAR
// 	step: REAL := 50.0;
// 	X: REAL;
// 	Y: REAL;
// 	t1: REAL;
// 	pt : POINTER TO DWORD;
// END_VAR
//
// X := A * R0;
// Y := B * R0;
// IF Res >= R0 THEN
// 	t1 := X * X - 4.0 * Y * (R0 - Res);
// 	IF t1 < 0.0 THEN
// 		TEMP_PT := 10000.0;
// 	ELSE
// 		TEMP_PT := (-X + SQRT(t1)) / (2.0 * Y);
// 	END_IF;
// ELSE
// 	pt := ADR(step);
// 	(* since the formula cannot be solved this is a successive approximation *)
// 	TEMP_PT := -100.0;
// 	WHILE step > accuracy DO
// 		(* test if result greater or less *)
// 		IF RES_PT(TEMP_PT, R0) < res THEN TEMP_PT := TEMP_PT + step; ELSE TEMP_PT := TEMP_PT - step; END_IF;
// 		pt^ := pt^ - 8388608;  (* this is a super fast divide by 2 method for non floating point CPUs *)
// 		(* the alternative code: step := step * 0.5; *)
// 	END_WHILE;
// END_IF;
//
// END_FUNCTION

FUNCTION TEMP_SI : REAL
VAR_INPUT
	Res : REAL;
	RS : REAL;
	TS : REAL;
END_VAR

TEMP_SI := (-7.64E-3 + SQRT(Res / RS * 6.64E-5 - 0.803E-5)) * 30120.48193 + TS;
END_FUNCTION

FUNCTION_BLOCK _RMP_B
VAR_INPUT
	DIR : BOOL;  (* true = up *)
	E : BOOL := TRUE;
	TR : TIME;
END_VAR
VAR_IN_OUT
	RMP : BYTE;
END_VAR
VAR
	tx, tl, tn : TIME;
	init : BOOL;
	last_dir : BOOL;
	start : BYTE;
END_VAR

(* read system timer *)
tx := DWORD_TO_TIME(T_PLC_MS());

IF E AND init AND (dir = last_dir) AND (RMP <> SEL(DIR, 0, 255)) AND TR = tn THEN
	RMP := FRMP_B(start, DIR, tx - tl, TR);
ELSE
	init := TRUE;
	tl := tx;
	tn := tr;
	start := RMP;
END_IF;
last_dir := dir;



(* revison history
hm	22. oct. 2008	rev 1.0
	original version

hm	20. nov. 2008	rev 1.1
	set default for E to be TRUE
	added init

hm	19. nov. 2011	rev 1.2
	new code
*)
END_FUNCTION_BLOCK

FUNCTION_BLOCK _RMP_NEXT
VAR_INPUT
	E : BOOL := TRUE;
	IN : BYTE;
	TR : TIME;
	TF : TIME;
	TL : TIME;
END_VAR
VAR_OUTPUT
	DIR : BOOL;    (* upwards = TRUE *)
	UP : BOOL;
	DN : BOOL;
END_VAR
VAR_IN_OUT
	OUT : BYTE;
END_VAR
VAR
	rmx : _RMP_B;
	dirx : TREND_DW;
	t_lock :TP;
	xen: BOOL;
	xdir: BOOL;
END_VAR

dirx(X := in);

t_lock(in := FALSE, pt := TL);

IF dirx.TU AND (OUT < IN) THEN
	IF NOT xdir AND xen THEN t_lock(in := TRUE); END_IF;
	xen := TRUE;
	xdir := TRUE;
ELSIF dirx.TD AND (OUT > IN) THEN
	IF xdir AND xen THEN t_lock(in := TRUE); END_IF;
	xen := TRUE;
	xdir := FALSE;
ELSIF xen THEN
	IF (xdir AND (out >= in)) OR (NOT xdir AND (out <= in)) THEN
		xen := FALSE;
		IF tl > t#0s THEN t_lock(IN := TRUE); END_IF;
	END_IF;
END_IF;

IF NOT t_lock.Q AND xen THEN
		UP := XDIR;
		DIR := XDIR;
		DN := NOT XDIR;
ELSE
	UP := FALSE;
	DN := FALSE;
END_IF;

rmx(rmp := OUT, E := E AND (UP OR DN) , dir := DIR, tr := SEL(dir, TF, TR));


(* revison history
hm	23. nov. 2008	rev 1.0
	original version

hm	24. jan. 2009	rev 1.1
	deleted unused vars tmp1, tmp2

hm	20. feb. 2009	rev 1.2
	improved algorithm
	added TL

hm	9. mar. 2009	rev 1.3
	input E was ignored
	removed double assignments

hm	18. jul. 2009	rev 1.4
	improved performance

*)
END_FUNCTION_BLOCK

FUNCTION_BLOCK _RMP_W
VAR_INPUT
	DIR : BOOL;
	E : BOOL := TRUE;
	TR : TIME;
END_VAR
VAR_IN_OUT
	RMP : WORD;
END_VAR
VAR
	tx, tl : DWORD;
	step : DINT;
	init: BOOL;
	last_dir : BOOL;
END_VAR

tx := T_PLC_MS();
IF E AND init THEN
	(* we need to set tl = tx when direction changes *)
	IF dir XOR last_dir THEN
	 tl := tx;
	 last_dir := dir;
	END_IF;
	(* check_elapsed time and calculate only if necessary *)
	IF tr > t#0s THEN
		step := ULINT_TO_DINT(DWORD_TO_ULINT(SHL(ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(tl)) MOD 4294967296), 16)) / TIME_TO_ULINT(TR));
	ELSE
		step := 65535;
	END_IF;
	IF step > 0 THEN
		(* perform the step on the ramp *)
		tl := tx;
		(* calculate the step response *)
		IF NOT dir THEN step := - step; END_IF;
		rmp := DINT_TO_WORD(LIMIT(0, WORD_TO_DINT(rmp) + step, 65535));
	END_IF;
ELSE
	tl := tx;
	init := TRUE;
END_IF;


(* revison history
hm	22. oct. 2008	rev 1.0
	original version

hm	20. nov. 2008	rev 1.1
	set default for E to be TRUE
	added init

*)
END_FUNCTION_BLOCK

FUNCTION_BLOCK GEN_PULSE
VAR_INPUT
	ENQ : BOOL := TRUE;
	PTH : TIME;
	PTL : TIME;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	tx: TIME;
	tn: TIME;
	init: BOOL;
END_VAR

IF enq THEN
	tx := DWORD_TO_TIME(T_PLC_MS());
	IF NOT init THEN init := TRUE; tn := tx; END_IF;
	IF tx - tn >= SEL(Q, PTL, PTH) THEN
		tn := tn + SEL(Q, PTL, PTH);
		Q := NOT Q;
	END_IF;
ELSE
	Q := FALSE;
	init := FALSE;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK GEN_PW2
VAR_INPUT
	ENQ : BOOL;
	TH1 : TIME;
	TL1 : TIME;
	TH2 : TIME;
	TL2 : TIME;
	TS : BOOL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	TH : TIME;
	TL : TIME;
END_VAR
VAR
	t_high : TIME;
	t_low : TIME;
	tx : TIME;
	start : TIME;
	init : BOOL;
	et : TIME;
END_VAR

(* read system timer *)
tx := DWORD_TO_TIME(T_PLC_MS());

(* startup initialization *)
IF NOT init THEN
	start := tx;
	init := TRUE;
	TH := T#0s;
	TL := T#0s;
END_IF;

(* timing selection *)
IF TS THEN
	t_high := TH2;
	t_low := TL2;
ELSE
	t_high := TH1;
	t_low := TL1;
END_IF;

(* normal operation *)
IF ENQ THEN
	et := tx - start;
	IF NOT Q THEN
		IF et >= t_low THEN
			Q := TRUE;
			start := tx;
			TL := T#0s;
		ELSE
			TL := et;
		END_IF;
	ELSE
		IF et >= t_high THEN
			Q := FALSE;
			start := tx;
			TH := T#0s;
		ELSE
			TH := et;
		END_IF;
	END_IF;
ELSE
	Q := FALSE;
	TH := T#0s;
	TL := T#0s;
	start := tx;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK GEN_RDM
VAR_INPUT
	PT : TIME;
	AM : REAL := 1;
	OS : REAL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	Out : REAL;
END_VAR
VAR
	tx : TIME;
	last : TIME;
	init : BOOL;
END_VAR

(* read system time and prepare input data *)
tx := DWORD_TO_TIME(T_PLC_MS()) - last;

(* init section *)
IF NOT init THEN
	init := TRUE;
	last := tx;
	tx := t#0s;
END_IF;

(* add last if one cycle is finished *)
IF tx >= pt THEN
	last := last + pt;
	tx := tx - pt;

	(* generate output signal *)
	out := am * (RDM(0) - 0.5) + os;
	q := TRUE;
ELSE
	q := FALSE;
END_IF;

END_FUNCTION_BLOCK

FUNCTION_BLOCK GEN_RDT

VAR_INPUT
	Enable		 : BOOL := TRUE ;
	Min_Time_ms	: TIME := t#1s ; (* min Taktzeit *)
	Max_Time_ms	: TIME := t#1.2s ; (* Max Taktzeit *)
	TP_Q		 : TIME := t#100ms ; (* Zeit Ausgang auf TRUE *)
END_VAR
VAR_OUTPUT
	xQ		 : BOOL ;
END_VAR
VAR
	(* Taktgenerator Simulation *)
	tonRDMTimer	: TON ; (* Zeitbaustein Taktgenerator *)
	tof_xQ		: TOF ; (* Ausschaltverz�gerung Taktgenerator *)
	tRDMTime	: TIME ; (* Sollzeit *)
	rRDMTime	: REAL ; (* Zufalswert Timer *)
END_VAR


(*
FUNCTION_BLOCK RDMTimer
###############################################################################
		 Ersteller / author :		 		 		 		 		 J.Schohaus
		 Datum / date:		 		 		 		 		 		 13.07.2007
###############################################################################
		 �nderungen / Datum / Ersteller :		 		 		 		 		 		 		 		 		 		 		 		 		 		 		 
		 moditication / date / author :		 		 
###############################################################################
		 Verwendete Bibliotheken		 ( * werden im Baustein nicht ben�tigt )
		 		 		 		 Oscat.lib
###############################################################################
Beschreibung:
*)

tonRDMTimer ( IN:= Enable , PT:= tRDMTime );
tof_xQ ( IN:= tonRDMTimer.Q , PT:= TP_Q );
XQ := tof_xq.Q;
IF tonRDMTimer.Q THEN
		 xQ := TRUE ;
		 rRDMTime := RDM ( last:= rRDMTime ) ;
		 tRDMTime := REAL_TO_TIME ( rRDMTime * DINT_TO_REAL(TIME_TO_DINT( Max_Time_ms - Min_Time_ms ) + TIME_TO_DINT(Min_Time_ms ))) ;
		 tonRDMTimer ( IN:= FALSE );
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK GEN_RMP
VAR_INPUT
	PT : TIME := t#1s;
	AM : REAL := 1.0;
	OS : REAL;
	DL : REAL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	OUT : REAL;
END_VAR
VAR
	tx : TIME;
	last : TIME;
	init : BOOL;
	temp : REAL;
	ltemp: REAL;
END_VAR
VAR
	DL_work : REAL;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
DL_work := DL;

(* read system time and prepare input data *)
tx := DWORD_TO_TIME(T_PLC_MS()) - last;
DL_work := MODR(DL_work,1.0);
IF DL_work < 0.0 THEN DL_work := 1.0 - DL_work; END_IF;

(* init section *)
IF NOT init THEN
	init := TRUE;
	last := tx;
	tx := t#0s;
END_IF;

(* add last if one cycle is finished *)
IF tx >= pt THEN
	last := last + pt;
	tx := tx - pt;
END_IF;

(* generate sine wave *)
ltemp := temp;
IF pt > t#0s THEN temp := FRACT(TIME_TO_REAL(tx + MULTIME(pt, DL_work)) / TIME_TO_REAL(pt)); END_IF;
out := am * temp + os;

(* boolean output Q *)
Q := temp < ltemp;
END_FUNCTION_BLOCK

FUNCTION_BLOCK GEN_SIN
VAR_INPUT
	PT : TIME;
	AM : REAL := 1.0;
	OS : REAL;
	DL : REAL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	Out : REAL;
END_VAR
VAR
	tx : TIME;
	last : TIME;
	init : BOOL;
	temp : REAL;
END_VAR
VAR
	DL_work : REAL;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
DL_work := DL;

(* read system time and prepare input data *)
tx := DWORD_TO_TIME(T_PLC_MS()) - last;
DL_work := MODR(DL_work,1.0);
IF DL_work < 0.0 THEN DL_work := 1.0 - DL_work; END_IF;

(* init section *)
IF NOT init THEN
	init := TRUE;
	last := tx;
	tx := t#0s;
END_IF;

(* add last if one cycle is finished *)
IF tx >= pt THEN
	last := last + pt;
	tx := tx - pt;
END_IF;

(* generate sine wave *)
IF pt > t#0s THEN temp := SIN(math.PI2 * DWORD_TO_REAL(TIME_TO_DWORD(tx + MULTIME(pt, DL_work))) / DWORD_TO_REAL(TIME_TO_DWORD(pt))); END_IF;
out := am * 0.5 *  temp + os;

(* boolean output Q *)
q := NOT SIGN_R(temp);
END_FUNCTION_BLOCK

FUNCTION_BLOCK GEN_SQR
VAR_INPUT
	PT : TIME;
	AM : REAL := 1.0;
	OS : REAL;
	DC : REAL := 0.5;
	DL : REAL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	Out : REAL;
END_VAR
VAR
	tx : TIME;
	last : TIME;
	init: BOOL;
END_VAR
VAR
	DC_work : REAL;
	DL_work : REAL;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
DC_work := DC;
DL_work := DL;

(* check dc = 1 or 0 *)
IF DC_work = 0.0 THEN
	out := -am * 0.5 + os;
	Q := FALSE;
	RETURN;
ELSIF DC_work = 1.0 THEN
	out := am * 0.5 + os;
	Q := TRUE;
	RETURN;
END_IF;

(* read system time and prepare input data *)
tx := DWORD_TO_TIME(T_PLC_MS()) - last;
DL_work := MODR(DL_work,1.0);
IF DL_work < 0.0 THEN DL_work := 1.0 - DL_work; END_IF;
DC_work := MODR(DC_work,1.0);
IF DC_work < 0.0 THEN DC_work := 1.0 - DC_work; END_IF;

(* init section *)
IF NOT init THEN
	init := TRUE;
	last := tx;
	tx := t#0s;
END_IF;

(* add last if one cycle is finished *)
IF tx >= pt THEN
	last := last + pt;
	tx := tx - pt;
END_IF;

(* check if falling or rising edge first *)
IF MULTIME(pt, DL_work + DC_work) >= pt THEN
	(* generate falling edge *)
	IF tx >= MULTIME(pt, DL_work + DC_work - 1) THEN
		out := -am * 0.5 + os;
		Q := FALSE;
	END_IF;
	(* generate rising edge *)
	IF tx >= MULTIME(pt, DL_work) THEN
		out := am * 0.5 + os;
		Q := TRUE;
	END_IF;
ELSE
	(* generate rising edge first *)
	IF tx >= MULTIME(pt, DL_work) THEN
		out := am * 0.5 + os;
		Q := TRUE;
	END_IF;
	(* generate falling edge *)
	IF tx >= MULTIME(pt, DL_work + DC_work) THEN
		out := -am * 0.5 +os;
		Q := FALSE;
	END_IF;
END_IF;

END_FUNCTION_BLOCK

FUNCTION_BLOCK PWM_DC
VAR_INPUT
	F : REAL;
	DC : REAL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	clk: CLK_PRG;
	pulse: TP_X;
	tmp: REAL;
END_VAR

IF F > 0.0 THEN
	tmp := 1000.0 / F;
	CLK(PT := REAL_TO_TIME(tmp));
	Pulse(in := clk.Q, pt := REAL_TO_TIME(tmp * DC));
	Q := pulse.Q;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK PWM_PW
VAR_INPUT
	F : REAL;
	PW : TIME;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	clk: CLK_PRG;
	pulse: TP_X;
END_VAR

IF F > 0.0 THEN
	CLK(PT := REAL_TO_TIME(1000.0 / F));
	Pulse(in := clk.Q, pt := pw);
	Q := pulse.Q;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK RMP_B
VAR_INPUT
	SET : BOOL;
	PT : TIME;
	E : BOOL := TRUE;
	UP : BOOL := TRUE;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	OUT : BYTE;
	BUSY: BOOL;
	HIGH : BOOL;
	LOW : BOOL;
END_VAR
VAR
	rmp : _RMP_B;
END_VAR

(* generate ramp *)
rmp(dir := UP, E := E, TR := PT, rmp := out);

(* set or reset operation *)
IF RST THEN
	out := 0;
ELSIF SET THEN
	out := 255;
END_IF;

(* checks for outputs stable and reset or set busy flag *)
low := out = 0;
high := out = 255;
busy := NOT (low OR high) AND E;
END_FUNCTION_BLOCK

FUNCTION_BLOCK RMP_SOFT
VAR_INPUT
	IN : BOOL;
	VAL : BYTE;
END_VAR
VAR_INPUT CONSTANT
	PT_ON: TIME;
	PT_OFF : TIME;
END_VAR
VAR_OUTPUT
	OUT : BYTE;
END_VAR
VAR
	rmp : _RMP_B;
	tmp: BYTE;
END_VAR

tmp := SEL(in, 0, val);
IF tmp > out THEN
	(* we need to ramp down *)
	rmp(dir := TRUE, E := TRUE, TR := PT_ON, rmp := out);
	(* make sure out does not surpass tmp *)
	out := MIN(out, tmp);
ELSIF tmp < out THEN
	(* we need to ramp up *)
	rmp(dir := FALSE, E := TRUE, TR := PT_OFF, rmp := out);
	(* make sure out does not surpass tmp *)
	out := MAX(out, tmp);
ELSE
	(* no ramp necessary *)
	rmp(E := FALSE, rmp := out);
END_IF;

END_FUNCTION_BLOCK

FUNCTION_BLOCK RMP_W
VAR_INPUT
	SET : BOOL;
	PT : TIME;
	E : BOOL := TRUE;
	UP : BOOL := TRUE;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	out : WORD;
	busy : BOOL;
	high : BOOL;
	low : BOOL;
END_VAR
VAR
	rmp : _RMP_W;
END_VAR

(* generate ramp *)
rmp(dir := UP, E := E, TR := PT, rmp := out);

(* set or reset operation *)
IF RST THEN
	out := 0;
ELSIF SET THEN
	out := 65535;
END_IF;

(* checks for outputs stable and reset or set busy flag *)
low := out = 0;
high := out = 65535;
busy := NOT (low OR high) AND E;

END_FUNCTION_BLOCK

FUNCTION AIN : REAL
VAR_INPUT
	in : DWORD;
END_VAR
VAR_INPUT CONSTANT
	Bits : BYTE;
	sign : BYTE := 255;
	low : REAL;
	high : REAL := 10.0;
END_VAR
VAR CONSTANT
	ff : DWORD := 16#FFFFFFFF;
END_VAR
VAR
	temp1 : DWORD;
	temp2 : DWORD;
	sx : BOOL;
END_VAR

(* extract the sign bit *)
IF sign < 32 THEN
	temp1 := SHR(in,sign);
	sx := temp1.0;
ELSE
	sx := FALSE;
END_IF;
temp1 := SHR(ff, 32 - BYTE_TO_INT(bits));
temp2 := in AND temp1;
AIN := (high - low) * DWORD_TO_REAL(temp2) / DWORD_TO_REAL(temp1) + low;
IF sx THEN AIN := -AIN; END_IF;
END_FUNCTION

FUNCTION_BLOCK AIN1
VAR_INPUT
	in : DWORD;
END_VAR
VAR_INPUT CONSTANT
	sign_bit : INT := 255;
	error_bit : INT := 255;
	error_code_en : BOOL;
	error_code : DWORD;
	overflow_bit : INT := 255;
	overflow_code_en : BOOL;
	overflow_code : DWORD;
	Bit_0 : INT;
	Bit_N : INT := 31;
	out_min : REAL;
	out_max : REAL := 10.0;
	code_min : DWORD;
	code_max : DWORD := 16#FFFFFFFF;
	error_output : REAL;
	overflow_output : REAL := 10.0;
END_VAR
VAR_OUTPUT
	out : REAL;
	sign : BOOL;
	error : BOOL;
	overflow : BOOL;
END_VAR
VAR
	tB: DWORD;
END_VAR

(* extract error bit *)
error := ((SHR(in,error_bit) AND 16#0000_0001) = 1) OR (error_code_en AND error_code = in);
IF error THEN
	out := error_output;
	RETURN;
END_IF;

(* strip off the data input *)
tb := SHR(SHL(in, 31 - bit_N), 31 - bit_N + Bit_0);

(* extract overflow bit *)
overflow := ((SHR(in,overflow_bit) AND 16#0000_0001) = 1) OR (overflow_code_en AND overflow_code = in) OR (tb < code_min OR tb > code_max);
IF overflow THEN
	out := overflow_output;
	RETURN;
END_IF;

(* extract sign bit *)
sign := (SHR(in,sign_bit) AND 16#0000_0001) = 1;

(* convert in to out *)
out := (DWORD_TO_REAL(ULINT_TO_DWORD((DWORD_TO_ULINT(tb) + 4294967296 - DWORD_TO_ULINT(code_min)) MOD 4294967296)) * (out_max - out_min) / DWORD_TO_REAL(ULINT_TO_DWORD((DWORD_TO_ULINT(code_max) + 4294967296 - DWORD_TO_ULINT(code_min)) MOD 4294967296)) + out_min);
IF sign THEN out := out * -1.0; END_IF;
END_FUNCTION_BLOCK

FUNCTION AOUT : DWORD
VAR_INPUT
	in : REAL;
END_VAR
VAR_INPUT CONSTANT
	Bits : BYTE;
	sign : BYTE := 255;
	low : REAL;
	high : REAL := 10.0;
END_VAR
VAR CONSTANT
	ff : DWORD := 2#1;
END_VAR
VAR
	in2 : REAL;
	sx : BOOL;
END_VAR

(* if sign bit is defined *)
IF sign < 32 THEN
	sx := sign_R(in);
	in2 := ABS(in);
ELSE
	in2 := in;
END_IF;

(* begrenze in auf die zul�ssigen werte *)
in2 := LIMIT(low, in2, high);

(* Berechne den Ausgangswert *)
Aout := REAL_TO_DWORD((in2 - low) / (high - low) * UDINT_TO_REAL(DWORD_TO_UDINT(SHL(ff,bits)) - 1));
IF sx THEN Aout := SHL(ff,sign) OR Aout; END_IF;
END_FUNCTION

FUNCTION AOUT1 : DWORD
VAR_INPUT
	in : REAL;
END_VAR
VAR_INPUT CONSTANT
	Bit_0 : INT;
	Bit_N : INT := 31;
	sign : INT := 255;
	low : REAL;
	high : REAL := 10.0;
END_VAR
VAR CONSTANT
	ff : DWORD := 2#1;
END_VAR
VAR
	sx: BOOL;
	in2 : REAL;
END_VAR

(* if sign bit is defined *)
IF sign < 32 THEN
	sx := SIGN_R(in);
	in2 := ABS(in);
ELSE
	in2 := in;
END_IF;

(* begrenze in auf die zul�ssigen werte *)
in2 := LIMIT(low, in2, high);

(* Berechne den Ausgangswert *)
AOUT1 := SHL(REAL_TO_DWORD((in2 - low) / (high - low) * UDINT_TO_REAL(DWORD_TO_UDINT(SHL(ff,bit_n - Bit_0 + 1)) - 1)),Bit_0);
IF sx THEN AOUT1 := SHL(ff,sign) OR AOUT1; END_IF;
END_FUNCTION

FUNCTION BYTE_TO_RANGE : REAL
VAR_INPUT
	X : BYTE;
	low : REAL;
	high : REAL;
END_VAR

BYTE_TO_RANGE := (high - low) * BYTE_TO_REAL(X) / 255.0 + low;

END_FUNCTION

FUNCTION_BLOCK DELAY
VAR_INPUT
	IN : REAL;
	N : INT;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	OUT : REAL;
END_VAR
VAR
	buf : ARRAY[0..31] OF REAL;
	i : INT;
	init: BOOL;
	stop: INT;
END_VAR

stop := LIMIT(0,N,32) - 1;
IF rst OR NOT init THEN
	init := TRUE;
	FOR i := 0 TO stop DO buf[i] := in; END_FOR;
	out := in;
	i := 0;
ELSIF stop < 0 THEN
	out := in;
ELSE
	out := buf[i];
	buf[i] := in;
	i := INC1(i, N);
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK DELAY_4
VAR_INPUT
	in : REAL;
END_VAR
VAR_OUTPUT
	out1 : REAL;
	out2 : REAL;
	out3 : REAL;
	out4 : REAL;
END_VAR
VAR
	temp: REAL;
END_VAR

out4 := out3;
out3 := out2;
out2 := out1;
out1 := temp;
temp := in;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FADE
VAR_INPUT
	IN1 : REAL;
	IN2 : REAL;
	F : BOOL;
	TF : TIME;
	rst : BOOL;
END_VAR
VAR_OUTPUT
	Y : REAL;
END_VAR
VAR
	rmx : RMP_W;
END_VAR


rmx(rst := rst AND NOT F, set := rst AND F, pt := TF, up := F);
Y := (in2 - In1) / 65535.0 * WORD_TO_REAL(rmx.out) + in1;


(* code for rev 1.1
IF rst THEN
	rmx(set := F, rst := NOT F);
ELSIF F AND (NOT rmx.high) THEN
	rmx(PT := TF, UP := TRUE, e := TRUE, rst := FALSE, set := FALSE);
ELSIF (NOT F) AND (NOT rmx.low) THEN
	rmx(PT := TF, UP := FALSE, e := TRUE, rst := FALSE, set := FALSE);
ELSE
	rmx(e := FALSE, rst := FALSE, set := FALSE);
END_IF;
Y := (WORD_TO_REAL(rmx.out) * in1 + WORD_TO_REAL(FF - rmx.out) * IN2) / FF;
*)
END_FUNCTION_BLOCK

FUNCTION_BLOCK FILTER_DW
VAR_INPUT
	X : DWORD;
	T : TIME;
END_VAR
VAR_OUTPUT
	Y : DWORD;
END_VAR
VAR
	last : DWORD;
	tx: DWORD;
	init: BOOL;
	Yi : REAL;
END_VAR

(* read system time *)
tx := T_PLC_MS();

(* startup initialisation *)
IF NOT init OR T = t#0s THEN
	init := TRUE;
	Yi := DWORD_TO_REAL(X);
ELSE
	Yi := Yi + (DWORD_TO_REAL(X) - DWORD_TO_REAL(Y)) * DWORD_TO_REAL(ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(last)) MOD 4294967296)) / TIME_TO_REAL(T);
END_IF;
last := tx;

Y := REAL_TO_DWORD(Yi);



(*
hm 10. oct. 2008	rev 1.0
	original version

hm	3. nov. 2008	REV 1.1
	corrected an overflow problem
*)
END_FUNCTION_BLOCK

FUNCTION_BLOCK FILTER_I
VAR_INPUT
	X : INT;
	T : TIME;
END_VAR
VAR_OUTPUT
	Y : INT;
END_VAR
VAR
	Yi : DINT;
	last : DWORD;
	tx: DWORD;
	init: BOOL;
END_VAR

(* read system time *)
tx := T_PLC_MS();

(* startup initialisation *)
IF NOT init OR T = t#0s THEN
	init := TRUE;
	Yi := INT_TO_DINT(X) * 1000;
ELSE
	(* to increase accuracy of the filter we calculate internal Yi wich is Y * 1000 *)
	Yi := Yi + INT_TO_DINT(X - Y) * DWORD_TO_DINT(ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(last)) MOD 4294967296)) * 1000 / TIME_TO_DINT(T);
END_IF;
last := tx;
Y := DINT_TO_INT(yi / 1000);



(*
hm 8. nov. 2008	rev 1.0
	original version

*)
END_FUNCTION_BLOCK

FUNCTION_BLOCK FILTER_MAV_DW
VAR_INPUT
	X : DWORD;
	N : UINT;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Y : DWORD;
END_VAR
VAR
	init: BOOL;
	buffer : ARRAY[0..31] OF DWORD;
	i: INT;
END_VAR
VAR_TEMP
	tmp: INT;
END_VAR
VAR
	N_work : UINT;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
N_work := N;

(* limit N to size of buffer *)
N_work := MIN(N_work, 32);

(* startup initialisation *)
IF NOT init OR rst OR N_work = 0 THEN
	init := TRUE;
	tmp := UINT_TO_INT(N_work)-1;
	FOR i := 0 TO tmp DO
		buffer[i] := X;
	END_FOR;
	Y := X;
ELSE
	tmp := UINT_TO_INT(N_work);
	i := INC1(i, tmp);
	Y := ULINT_TO_DWORD((DWORD_TO_ULINT(Y) + DWORD_TO_ULINT(ULINT_TO_DWORD((DWORD_TO_ULINT(X) + 4294967296 - DWORD_TO_ULINT(buffer[i])) MOD 4294967296)) / N_work) MOD 4294967296);
	buffer[i] := X;
END_IF;




(*
hm 13. oct. 2008	rev 1.0
	original version

hm	27. oct. 2008	rev 1.1
	added typecast to avoid warnings

hm	24. nov. 2008	rev 1.2
	added typecasts to avoid warnings
	avoid divide by 0 if N = 0

hm	23. feb. 2009	rev 1.3
	limit N to max array size

*)
END_FUNCTION_BLOCK

FUNCTION_BLOCK FILTER_MAV_W
VAR_INPUT
	X : WORD;
	N : UINT;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Y : WORD;
END_VAR
VAR
	init: BOOL;
	buffer : ARRAY[1..32] OF WORD;
	i: INT;
	sum : DWORD;
END_VAR
VAR_TEMP
	tmp : INT;
END_VAR
VAR
	N_work : UINT;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
N_work := N;

(* limit N to size of buffer *)
N_work := MIN(N_work, 32);

(* startup initialisation *)
IF NOT init OR rst OR N_work = 0 THEN
	init := TRUE;
	tmp := UINT_TO_INT(N_work) - 1;
	FOR i := 1 TO tmp DO
		buffer[i] := X;
	END_FOR;
	sum := ULINT_TO_DWORD(WORD_TO_ULINT(Y) * N_work);
	Y := X;
ELSE
	tmp := UINT_TO_INT(N_work);
	i := INC1(i, tmp);
	sum := ULINT_TO_DWORD((DWORD_TO_ULINT(sum) + WORD_TO_ULINT(X) + 4294967296 - WORD_TO_ULINT(buffer[i])) MOD 4294967296);
	Y := ULINT_TO_WORD(DWORD_TO_ULINT(sum) / N_work);
	buffer[i] := X;
END_IF;



(*
hm 13. oct. 2008	rev 1.0
	original version

hm	18. oct. 2008	rev 1.1
	added typecast to avoid warnings

hm	24. nov. 2008	rev 1.2
	added typecasts to avoid warnings
	avoid devide by 0 if N = 0

hm	23. feb. 2009	rev 1.3
	limit N to max array size

hm	26. mar. 2011	rev 1.4
	corrected error in calculation
*)
END_FUNCTION_BLOCK

FUNCTION_BLOCK FILTER_W
VAR_INPUT
	X : WORD;
	T : TIME;
END_VAR
VAR_OUTPUT
	Y : WORD;
END_VAR
VAR
	last : DWORD;
	tx: DWORD;
	init: BOOL;
	tmp: DWORD;
END_VAR

(* read system time *)
tx := T_PLC_MS();

(* startup initialisation *)
IF NOT init OR T = T#0s THEN
	init := TRUE;
	last := tx;
	Y := X;
ELSIF Y = X THEN
	last := tx;
ELSE
	tmp := ULINT_TO_DWORD(DWORD_TO_ULINT(ULINT_TO_DWORD((((WORD_TO_ULINT(X) + 65536 - WORD_TO_ULINT(Y)) MOD 65536) * DWORD_TO_ULINT(ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(last)) MOD 4294967296))) MOD 4294967296)) / TIME_TO_ULINT(T));
	IF tmp <> 0 THEN
		Y := DINT_TO_WORD(WORD_TO_DINT(Y) + DWORD_TO_DINT(tmp));
		last := tx;
	END_IF;
END_IF;

(*
hm 10. oct. 2008	rev 1.0
	original version

hm	3. nov. 2008	rev 1.1
	fixed overflow problem in formula

hm	25. jan. 2011	rev 1.2
	fixed error in formula
*)
END_FUNCTION_BLOCK

FUNCTION_BLOCK FILTER_WAV
VAR_INPUT
	X : REAL;
	W : ARRAY[0..15] OF REAL;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Y : REAL;
END_VAR
VAR
	init: BOOL;
	buffer : ARRAY[0..15] OF REAL;
	i: INT;
	n: INT;
END_VAR

(* startup initialisation *)
IF NOT init OR rst THEN
	init := TRUE;
	FOR i := 0 TO 15 DO
		buffer[i] := X;
	END_FOR;
	i := 15;
	Y := X;
ELSE
	i := INC1(i, 16);
	buffer[i] := X;
END_IF;

(* calculate the weighted average *)
Y := 0.0;
FOR n := 0 TO 15 DO
	Y := buffer[i] * W[n] + Y;
	i := DEC1(i, 16);
END_FOR;


(*
hm 	14. oct. 2008	rev 1.0
	original version

hm	27. oct. 2008	rev 1.1
	changed _DEC and _INC to DEC1 and INC1

hm	10. mar. 2009	rev 1.2
	real constants updated to new systax using dot

*)

END_FUNCTION_BLOCK

FUNCTION MIX : REAL
VAR_INPUT
	A, B : REAL;
	M : REAL;
END_VAR


MIX := (1.0 - M) * A + M * B;
END_FUNCTION

FUNCTION MUX_R2 : REAL
VAR_INPUT
	IN0 : REAL;
	IN1 : REAL;
	A : BOOL;
END_VAR

MUX_R2 := SEL(A, IN0, IN1);
END_FUNCTION

FUNCTION MUX_R4 : REAL
VAR_INPUT
	IN0 : REAL;
	IN1 : REAL;
	IN2 : REAL;
	IN3 : REAL;
	A0 : BOOL;
	A1 : BOOL;
END_VAR

IF A1 THEN
	MUX_R4 := SEL(A0, IN2, IN3);
ELSE
	MUX_R4 := SEL(A0, IN0, IN1);
END_IF;
END_FUNCTION

FUNCTION OFFSET : REAL
VAR_INPUT
	X : REAL;
	O1, O2, O3, O4 : BOOL;
	D : BOOL;
END_VAR
VAR_INPUT CONSTANT
	Offset_1 : REAL;
	Offset_2 : REAL;
	Offset_3 : REAL;
	Offset_4 : REAL;
	default : REAL;
END_VAR

IF D THEN OFFSET := default; ELSE OFFSET := X; END_IF;
IF O1 THEN OFFSET := OFFSET + offset_1; END_IF;
IF O2 THEN OFFSET := OFFSET + offset_2; END_IF;
IF O3 THEN OFFSET := OFFSET + offset_3; END_IF;
IF O4 THEN OFFSET := OFFSET + offset_4; END_IF;
END_FUNCTION

FUNCTION OFFSET2 : REAL
VAR_INPUT
	X : REAL;
	O1, O2, O3, O4 : BOOL;
	D : BOOL;
END_VAR
VAR_INPUT CONSTANT
	Offset_1 : REAL;
	Offset_2 : REAL;
	Offset_3 : REAL;
	Offset_4 : REAL;
	default : REAL;
END_VAR

IF D THEN OFFSET2 := default; ELSE OFFSET2 := X; END_IF;
IF O4 THEN OFFSET2 := OFFSET2 + offset_4;
ELSIF O3 THEN OFFSET2 := OFFSET2 + offset_3;
ELSIF O2 THEN OFFSET2 := OFFSET2 + offset_2;
ELSIF O1 THEN OFFSET2 := OFFSET2 + offset_1;
END_IF;
END_FUNCTION

FUNCTION OVERRIDE : REAL
VAR_INPUT
	X1, X2, X3 : REAL;
	E1 : BOOL;
	E2 : BOOL;
	E3 : BOOL;
END_VAR

IF E1 THEN OVERRIDE := X1; END_IF;
IF E2 AND ABS(x2) > ABS(OVERRIDE) THEN OVERRIDE := X2; END_IF;
IF E3 AND ABS(x3) > ABS(OVERRIDE) THEN OVERRIDE := X3; END_IF;

END_FUNCTION

FUNCTION RANGE_TO_BYTE : BYTE
VAR_INPUT
	X : REAL;
	low : REAL;
	high : REAL;
END_VAR

RANGE_TO_BYTE := INT_TO_BYTE(TRUNC((LIMIT(low, X, high) - low) * 255.0 / (high - low)));

END_FUNCTION

FUNCTION RANGE_TO_WORD : WORD
VAR_INPUT
	X : REAL;
	low : REAL;
	high : REAL;
END_VAR

RANGE_TO_WORD := DINT_TO_WORD(TRUNC((LIMIT(low,X,high)-low) * 65535.0 / (high - low)));

END_FUNCTION

FUNCTION SCALE : REAL
VAR_INPUT
	X : REAL;
	K : REAL;
	O : REAL;
	MX : REAL;
	MN : REAL;
END_VAR


SCALE := LIMIT(MN, X * K + O, MX);
END_FUNCTION

FUNCTION SCALE_B : REAL
VAR_INPUT
	X : BYTE;
	I_LO : BYTE;
	I_HI : BYTE;
	O_LO : REAL;
	O_HI : REAL;
END_VAR


IF I_HI = I_LO THEN
	SCALE_B := O_LO;
ELSE
	SCALE_B := (O_HI - O_LO) / (BYTE_TO_REAL(I_HI) - BYTE_TO_REAL(I_LO)) * BYTE_TO_REAL(LIMIT(I_LO, X, I_HI));
END_IF
END_FUNCTION

FUNCTION SCALE_B2 : REAL
VAR_INPUT
	in1: BYTE;
	in2: BYTE;
	K : REAL;
	O : REAL;
END_VAR
VAR_INPUT CONSTANT
	in1_min: REAL;
	in1_max: REAL := 1000.0;
	in2_min: REAL;
	in2_max: REAL := 1000.0;
END_VAR



SCALE_B2 := 	(((in1_max - in1_min)* BYTE_TO_REAL(in1) + (in2_max - in2_min)* BYTE_TO_REAL(in2)) * 0.003921569 + in1_min + in2_min) * K + O;
END_FUNCTION

FUNCTION SCALE_B4 : REAL
VAR_INPUT
	in1: BYTE;
	in2: BYTE;
	in3: BYTE;
	in4: BYTE;
	K : REAL;
	O : REAL;
END_VAR
VAR_INPUT CONSTANT
	in1_min: REAL;
	in1_max: REAL := 1000.0;
	in2_min: REAL;
	in2_max: REAL := 1000.0;
	in3_min: REAL;
	in3_max: REAL := 1000.0;
	in4_min: REAL;
	in4_max: REAL := 1000.0;
END_VAR



SCALE_B4 := 	(((in1_max - in1_min)* BYTE_TO_REAL(in1) + (in2_max - in2_min)* BYTE_TO_REAL(in2) + (in3_max - in3_min)* BYTE_TO_REAL(in3) + (in4_max - in4_min)* BYTE_TO_REAL(in4))* 0.003921569 + in1_min + in2_min + in3_min + in4_min) * K + O;
END_FUNCTION

FUNCTION SCALE_B8 : REAL
VAR_INPUT
	in1: BYTE;
	in2: BYTE;
	in3: BYTE;
	in4: BYTE;
	in5: BYTE;
	in6: BYTE;
	in7: BYTE;
	in8: BYTE;
	K : REAL;
	O : REAL;
END_VAR
VAR_INPUT CONSTANT
	in1_min: REAL;
	in1_max: REAL := 1000.0;
	in2_min: REAL;
	in2_max: REAL := 1000.0;
	in3_min: REAL;
	in3_max: REAL := 1000.0;
	in4_min: REAL;
	in4_max: REAL := 1000.0;
	in5_min: REAL;
	in5_max: REAL := 1000.0;
	in6_min: REAL;
	in6_max: REAL := 1000.0;
	in7_min: REAL;
	in7_max: REAL := 1000.0;
	in8_min: REAL;
	in8_max: REAL := 1000.0;
END_VAR


SCALE_B8 := 	(((in1_max - in1_min)* BYTE_TO_REAL(in1) + (in2_max - in2_min)* BYTE_TO_REAL(in2) + (in3_max - in3_min)* BYTE_TO_REAL(in3) + (in4_max - in4_min)* BYTE_TO_REAL(in4) +
				(in5_max - in5_min)* BYTE_TO_REAL(in5) + (in6_max - in6_min)* BYTE_TO_REAL(in6) + (in7_max - in7_min)* BYTE_TO_REAL(in7) + (in8_max - in8_min)* BYTE_TO_REAL(in8)) * 0.003921569
				 + in1_min + in2_min + in3_min + in4_min + in5_min + in6_min + in7_min + in8_min) * K + O;
END_FUNCTION

FUNCTION SCALE_D : REAL
VAR_INPUT
	X : DWORD;
	I_LO : DWORD;
	I_HI : DWORD;
	O_LO : REAL;
	O_HI : REAL;
END_VAR


IF I_HI = I_LO THEN
	SCALE_D := O_LO;
ELSE
	SCALE_D := (O_HI - O_LO) / (DWORD_TO_REAL(I_HI) - DWORD_TO_REAL(I_LO)) * (DWORD_TO_REAL(LIMIT(I_LO, X, I_HI)) - DWORD_TO_REAL(I_LO)) + O_LO;
END_IF;
END_FUNCTION

FUNCTION SCALE_R : REAL
VAR_INPUT
	X : REAL;
	I_LO : REAL;
	I_HI : REAL;
	O_LO : REAL;
	O_HI : REAL;
END_VAR


IF I_LO = I_HI THEN
	SCALE_R := O_LO;
ELSE
	SCALE_R := (O_HI - O_LO) / (I_HI - I_LO) * (LIMIT(I_LO, X, I_HI) - I_LO) + O_LO;
END_IF;
END_FUNCTION

FUNCTION SCALE_X2 : REAL
VAR_INPUT
	IN1: BOOL;
	IN2: BOOL;
	K : REAL;
	O : REAL;
END_VAR
VAR_INPUT CONSTANT
	IN1_MIN: REAL;
	IN1_MAX: REAL := 1000.0;
	IN2_MIN: REAL;
	IN2_MAX: REAL := 1000.0;
END_VAR



SCALE_X2 := (SEL(IN1, IN1_MIN, IN1_MAX)+ SEL(IN2, IN2_MIN, IN2_MAX)) * k + o;
END_FUNCTION

FUNCTION SCALE_X4 : REAL
VAR_INPUT
	IN1: BOOL;
	IN2: BOOL;
	IN3: BOOL;
	IN4: BOOL;
	K : REAL;
	O : REAL;
END_VAR
VAR_INPUT CONSTANT
	IN1_MIN: REAL;
	IN1_MAX: REAL := 1000.0;
	IN2_MIN: REAL;
	IN2_MAX: REAL := 1000.0;
	IN3_MIN: REAL;
	IN3_MAX: REAL := 1000.0;
	IN4_MIN: REAL;
	IN4_MAX: REAL := 1000.0;
END_VAR



SCALE_X4 := (SEL(IN1,IN1_MIN, IN1_MAX) + SEL(IN2, IN2_MIN, IN2_MAX)+SEL(IN3, IN3_MIN, IN3_MAX)+ SEL(IN4, IN4_MIN, IN4_MAX)) * k + o;
END_FUNCTION

FUNCTION SCALE_X8 : REAL
VAR_INPUT
	in1: BOOL;
	in2: BOOL;
	in3: BOOL;
	in4: BOOL;
	in5: BOOL;
	in6: BOOL;
	in7: BOOL;
	in8: BOOL;
	K : REAL;
	O : REAL;
END_VAR
VAR_INPUT CONSTANT
	in1_min: REAL;
	in1_max: REAL := 1000.0;
	in2_min: REAL;
	in2_max: REAL := 1000.0;
	in3_min: REAL;
	in3_max: REAL := 1000.0;
	in4_min: REAL;
	in4_max: REAL := 1000.0;
	in5_min: REAL;
	in5_max: REAL := 1000.0;
	in6_min: REAL;
	in6_max: REAL := 1000.0;
	in7_min: REAL;
	in7_max: REAL := 1000.0;
	in8_min: REAL;
	in8_max: REAL := 1000.0;
END_VAR



SCALE_X8 := (SEL(IN1,IN1_MIN, IN1_MAX)+ SEL(IN2,IN2_MIN, IN2_MAX)+SEL(IN3,IN3_MIN, IN3_MAX)+ SEL(IN4,IN4_MIN, IN4_MAX)
				+SEL(IN5,IN5_MIN, IN5_MAX)+ SEL(IN6,IN6_MIN, IN6_MAX)+SEL(IN7,IN7_MIN, IN7_MAX)+ SEL(IN8,IN8_MIN, IN8_MAX)) * k + o;
END_FUNCTION

FUNCTION_BLOCK SEL2_OF_3
VAR_INPUT
	IN1 : REAL;
	IN2 : REAL;
	IN3 : REAL;
	D : REAL;
END_VAR
VAR_OUTPUT
	Y : REAL;
	W : INT;
	E : BOOL;
END_VAR
VAR
	D12: BOOL;
	D23: BOOL;
	D31: BOOL;
END_VAR


D12 := ABS(IN1-IN2) <= D;
D23 := ABS(IN2-IN3) <= D;
D31 := ABS(IN3-IN1) <= D;

IF (D12 AND D23) OR (D12 AND D31) OR (D23 AND D31) THEN
	(* all 3 inputs are valid *)
	Y := (IN1 + IN2 + IN3) * 0.333333333333;
	E := FALSE;
	W := 0;
ELSIF D12 THEN
	(* only inut 1 and 2 are valid *)
	Y := (In1 + IN2) * 0.5;
	E := FALSE;
	W := 3;
ELSIF D23 THEN
	(* only inut 2 and 3 are valid *)
	Y := (In2 + IN3) * 0.5;
	E := FALSE;
	W := 1;
ELSIF D31 THEN
	(* only inut 3 and 1 are valid *)
	Y := (In3 + IN1) * 0.5;
	E := FALSE;
	W := 2;
ELSE
	(* no calculation possible *)
	E := TRUE;
	W := 4;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK SEL2_OF_3B
VAR_INPUT
	IN1 : BOOL;
	IN2 : BOOL;
	IN3 : BOOL;
	TD : TIME;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	W : BOOL;
END_VAR
VAR
	TDEL : TON;
END_VAR


Q := (IN1 AND IN2) OR (IN1 AND IN3) OR (IN2 AND IN3);
TDEL(IN := (in1 XOR in2) OR (in1 XOR in3) OR (in2 XOR in3), PT := TD);
W := TDEL.Q;
END_FUNCTION_BLOCK

FUNCTION_BLOCK SH
VAR_INPUT
	in : REAL;
	CLK : BOOL;
END_VAR
VAR_OUTPUT
	out : REAL;
	trig : BOOL;
END_VAR
VAR
	edge : BOOL;
END_VAR

IF clk AND NOT edge THEN
	out := in;
	trig := TRUE;
ELSE;
	trig := FALSE;
END_IF;
edge := clk;
END_FUNCTION_BLOCK

FUNCTION_BLOCK SH_1
VAR_INPUT
	in : REAL;
	PT : TIME;
END_VAR
VAR_OUTPUT
	out : REAL;
	Trig : BOOL;
END_VAR
VAR
	last : TIME;
	tx: TIME;
END_VAR

(* read system time *)
tx := DWORD_TO_TIME(T_PLC_MS());

IF tx - last >= PT THEN
	last := tx;
	out := in;
	trig := TRUE;
ELSE
	trig := FALSE;
END_IF;


END_FUNCTION_BLOCK

FUNCTION_BLOCK SH_2
VAR_INPUT
	in : REAL;
	PT : TIME;
	N : INT := 16;
	disc : INT;
END_VAR
VAR_OUTPUT
	out : REAL;
	trig : BOOL;
	avg : REAL;
	high: REAL;
	low: REAL;
END_VAR
VAR
	M : INT;
	buf : ARRAY[0..15] OF REAL;
	buf2 : ARRAY[0..15] OF REAL;
	last : TIME;
	i : INT;
	start: INT;
	temp: REAL;
	stop: INT;
	tx: TIME;
	d2: INT;
END_VAR

(* read system time *)
tx := DWORD_TO_TIME(T_PLC_MS());
d2 := disc / 2;

IF tx - last >= PT THEN
	last := tx;
	trig := TRUE;

	(* limit M to 0..16 *)
	M := LIMIT(1,N,16);

	(* edge detected lets take the sample *)
	FOR i := M - 1 TO 1 BY -1 DO buf2[i] := buf2[i-1]; END_FOR;
	buf2[0] := in;
	out := in;
	buf := buf2;

	(* sort the ARRAY lowest value AT 0 *)
	FOR start := 0 TO M - 2 DO
		FOR i := start+1 TO M - 1 DO
			IF buf[start] > buf[i] THEN
				temp := buf[start];
				buf[start] := buf[i];
				buf[i] := temp;
			END_IF;
		END_FOR;
	END_FOR;

	(* any calculation with the samples is here *)
	stop := M - 1 - d2;
	start := d2;
	IF NOT even(disc) THEN start := start + 1; END_IF;
	avg := 0;
	FOR i := start TO stop DO avg := avg + buf[i]; END_FOR;
	avg := avg / INT_TO_REAL(stop - start +1);
	low := buf[start];
	high := buf[stop];
ELSE
	Trig := FALSE;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK SH_T
VAR_INPUT
	IN : REAL;
	E : BOOL;
END_VAR
VAR_OUTPUT
	OUT : REAL;
END_VAR

IF E THEN out := in; END_IF;
END_FUNCTION_BLOCK

FUNCTION STAIR : REAL
VAR_INPUT
	X : REAL;
	D : REAL;
END_VAR

IF D > 0.0 THEN
	STAIR := DINT_TO_REAL(REAL_TO_DINT(X / D)) * D;
ELSE
	STAIR := X;
END_IF;
END_FUNCTION

FUNCTION_BLOCK STAIR2
VAR_INPUT
	X : REAL;
	D : REAL;
END_VAR
VAR_OUTPUT
	Y: REAL;
END_VAR

IF D > 0.0 THEN
	IF X >= Y + D OR X <= Y - D THEN Y := FLOOR(X/D) * D; END_IF;
ELSE
	Y := X;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK TREND
VAR_INPUT
	X : REAL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	TU : BOOL;
	TD : BOOL;
	D : REAL;
END_VAR
VAR
	last_X: REAL;
END_VAR

TU := X > last_X;
TD := X < last_X;
Q := TU OR TD;
D := X - LAST_X;

last_X := X;

END_FUNCTION_BLOCK

FUNCTION_BLOCK TREND_DW
VAR_INPUT
	X : DWORD;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	TU : BOOL;
	TD : BOOL;
	D : DWORD;
END_VAR
VAR
	last_X: DWORD;
END_VAR

IF X > last_X THEN
	TU := TRUE;
	TD := FALSE;
	D := ULINT_TO_DWORD(DWORD_TO_ULINT(X) - DWORD_TO_ULINT(last_X));
	Q := TRUE;
ELSIF X < last_X THEN
	TD := TRUE;
	TU := FALSE;
	D := ULINT_TO_DWORD(DWORD_TO_ULINT(last_X) - DWORD_TO_ULINT(X));
	Q := FALSE;
ELSE
	TU := FALSE;
	TD := FALSE;
	D := 0;
END_IF;
last_X := X;

END_FUNCTION_BLOCK

FUNCTION WORD_TO_RANGE : REAL
VAR_INPUT
	X : WORD;
	low : REAL;
	high : REAL;
END_VAR

WORD_TO_RANGE := (high - low) * WORD_TO_REAL(X) * 0.00001525902189669640 + low;

END_FUNCTION

// Commented out for beedance: it uses CHR_TO_STRING, which is commented out.
// FUNCTION LIST_ADD : BOOL
// VAR_INPUT
// 	SEP : BYTE;
// 	INS : STRING(LIST_LENGTH);
// END_VAR
// VAR_IN_OUT
// 	LIST : STRING(LIST_LENGTH);
// END_VAR
// VAR
// 	sx : STRING(1);
// END_VAR
// VAR
// 	INS_work : STRING;
// END_VAR
// (* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
// INS_work := INS;
//
// sx := CHR_TO_STRING(SEP);			(* convert separation character to string *)
// INS_work := CONCAT(sx, INS_work);					(* start element with separation character *)
//
// IF LEN(LIST) + LEN(INS_work) > LIST_LENGTH THEN
// 	LIST_ADD := FALSE;					(* return false if element does not fit *)
// ELSE
// 	LIST := CONCAT(LIST, INS_work);
// 	LIST_ADD := TRUE;
// END_IF;
//
//
// (* revision histroy
//
// hm	21. mar. 2011	rev 2.0
// 	original release
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION LIST_CLEAN : BOOL
// VAR_INPUT
// 	SEP : BYTE;
// END_VAR
// VAR_IN_OUT
// 	LIST : STRING(LIST_LENGTH);
// END_VAR
// VAR
// 	pt : POINTER TO ARRAY[1..LIST_LENGTH] OF BYTE;
// 	read : INT := 1;
// 	write : INT := 1;
// 	last: BYTE;
// 	c: BYTE;
// END_VAR
//
// pt := ADR(LIST);
//
// FOR read := 1 TO LIST_LENGTH DO
// 	c := pt^[read];								(* read character from list *)
// 	IF c = 0 THEN EXIT;						(* exit the loop if character definbes end of string *)
// 	ELSIF c <> SEP OR sep <> last THEN	(* copy element from read to write position unless a double sep character is present *)
// 		pt^[write] := c;
// 		write := write + 1;
// 	END_IF;
// 	last := c;									(* remember last character *)
// END_FOR;
//
// IF last = SEP THEN write := write - 1; END_IF;		(* if last character is sep then delete empty element at end *)
// IF write <= STRING_LENGTH THEN pt^[write] := 0;	 END_IF;		(* terminate string with 0 *)
//
// LIST_CLEAN := TRUE;						(* retrun TRUE *)
//
//
// (* revision histroy
// hm	28. jun. 2008	rev 1.0
// 	original release
//
// hm	19. jan. 2011	rev 1.1
// 	changed string(255) to string(LIST_LENGTH)
//
// hm	21. mar. 2011	rev 2.0
// 	all elements start with SEP
//
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION LIST_GET : STRING(LIST_LENGTH)
// VAR_INPUT
// 	SEP : BYTE;
// 	POS : INT;
// END_VAR
// VAR_IN_OUT
// 	LIST : STRING(LIST_LENGTH);
// END_VAR
// VAR
// 	i : INT := 1;
// 	o : INT := 1;
// 	pt : POINTER TO ARRAY[1..LIST_LENGTH] OF BYTE;
// 	po : POINTER TO ARRAY[1..LIST_LENGTH] OF BYTE;
// 	cnt: INT := 0;
// 	c: BYTE;
// END_VAR
//
// pt := ADR(LIST);							(* load pointers *)
// po := ADR(LIST_GET);
//
// REPEAT
// 	c := pt^[i];
// 	IF cnt = pos THEN						(* copy element to output string *)
// 		IF c = SEP  THEN EXIT; END_IF;		(* next element stop copy and finish *)
// 		po^[o] := c;
// 		o := o + 1;
// 	ELSIF c = SEP THEN
// 		cnt := cnt + 1;
// 	END_IF;
// 	i := i + 1;
// UNTIL
// 	(i = LIST_LENGTH) OR (c = 0)
// END_REPEAT;
// po^[o] := 0;						(* close the output string *)
//
//
//
// (* revision histroy
// hm	20. jun. 2008	rev 1.0
// 	original release
//
// hm	19. jan. 2011	rev 1.1
// 	changed string(255) to strring(LIST_LENGTH)
//
// hm	21. mar. 2011	rev 2.0
// 	all list elements start with SEP
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION LIST_INSERT : BOOL
// VAR_INPUT
// 	SEP : BYTE;
// 	POS : INT;
// 	INS : STRING(LIST_LENGTH);
// END_VAR
// VAR_IN_OUT
// 	LIST : STRING(LIST_LENGTH);
// END_VAR
// VAR
// 	pt : POINTER TO ARRAY[1..LIST_LENGTH] OF BYTE;
// 	read : INT := 1;
// 	cnt : INT := 1;
// 	sx : STRING(1);
// END_VAR
//
// (* load pointers *)
// pt := ADR(LIST);
//
// IF LEN(ins) + 1 + LEN(LIST) > LIST_LENGTH THEN
// 	LIST_INSERT := FALSE;
// ELSE
// 	LIST_INSERT := TRUE;
// 	(* search for position *)
// 	WHILE read < LIST_LENGTH DO
// 		IF cnt >= POS THEN
// 			sx := CHR_TO_STRING(SEP);
// 			INS := CONCAT(sx, INS);
// 			LIST := INSERT(LIST, INS, read - 1);
// 			LIST_INSERT := TRUE;
// 			RETURN;
// 		END_IF;
// 		IF pt^[read]  = 0 THEN				(* list is too short add empty element *)
// 			pt^[read] := SEP;
// 			pt^[read + 1] := 0;
// 		END_IF;
// 		read := read + 1;
// 		IF pt^[read] = SEP OR pt^[read] = 0 THEN cnt := cnt + 1; END_IF;
// 	END_WHILE;
// END_IF;
//
//
//
// (* revision histroy
// hm	28. jun. 2008	rev 1.0
// 	original release
//
// hm	17. dec. 2008	rev 1.1
// 	changes name of function chr to chr_to_string
//
// hm	19. jan. 2011	rev 1.2
// 	changed string(255) to string(LIST_LENGTH)
//
// hm	21. mar. 2011	rev 2.0
// 	all list elements start with SEP
//
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION LIST_LEN : INT
// VAR_INPUT
// 	SEP : BYTE;
// END_VAR
// VAR_IN_OUT
// 	LIST : STRING(LIST_LENGTH);
// END_VAR
// VAR
// 	pt : POINTER TO ARRAY[1..LIST_LENGTH] OF BYTE;
// 	pos: INT := 1;
// 	c: BYTE;
// END_VAR
//
// pt := ADR(LIST);
// LIST_LEN := 0;
// REPEAT
// 	c := pt^[pos];
// 	IF c = SEP THEN LIST_LEN := LIST_LEN + 1; END_IF;
// 	pos := pos + 1;
// UNTIL
// 	c = 0 OR pos > LIST_LENGTH
// END_REPEAT;
//
//
//
// (* revision histroy
// hm	25. jun. 2008	rev 1.0
// 	original release
//
// hm	16. oct. 2008	rev 1.1
// 	fixed a problem where list_len would only work up to LIST_LENGTH
//
// hm	19. jan. 2001	rev 1.2
// 	changed string(255) to string(LIST_LENGTH)
//
// hm	21. mar. 2011	rev 2.0
// 	all list elements start with SEP
//
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION_BLOCK LIST_NEXT
// VAR_INPUT
// 	SEP : BYTE;
// 	RST : BOOL;
// END_VAR
// VAR_IN_OUT
// 	LIST : STRING(LIST_LENGTH);
// END_VAR
// VAR_OUTPUT
// 	LEL : STRING(LIST_LENGTH);
// 	NUL : BOOL;
// END_VAR
// VAR
// 	pos : INT := 1;
// 	pt : POINTER TO ARRAY[1..LIST_LENGTH] OF BYTE;
// 	po : POINTER TO ARRAY[1..LIST_LENGTH] OF BYTE;
// 	c: BYTE;
// 	write: INT;
// END_VAR
//
// pt := ADR(LIST);
// po := ADR(LEL);
//
// IF rst THEN
// 	pos := 1;
// END_IF;
//
// IF pt^[pos] = 0 OR pos = LIST_LENGTH THEN
// 	LEL := '';
// 	NUL := TRUE;
// ELSE
// 	NUL := FALSE;
// 	write := 1;
// 	FOR pos := pos + 1 TO LIST_LENGTH DO
// 		c := pt^[pos];
// 		IF c = 0 OR c = SEP THEN
// 			po^[write] := 0;
// 			RETURN;
// 		ELSE
// 			po^[write] := pt^[pos];
// 			write := write + 1;
// 		END_IF;
// 	END_FOR;
// END_IF;
//
//
//
//
// (* revision histroy
// hm	25. jun. 2008	rev 1.0
// 	original release
//
// hm	19. jan. 2011	rev 1.1
// 	changed string(255) to string(LIST_LENGTH)
//
// hm	21. mar. 2011	rev 2.0
// 	all elements start with SEP
//
// *)
//
// END_FUNCTION_BLOCK

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION LIST_RETRIEVE : STRING(LIST_LENGTH)
// VAR_INPUT
// 	SEP : BYTE;
// 	POS : INT;
// END_VAR
// VAR_IN_OUT
// 	LIST : STRING(LIST_LENGTH);
// END_VAR
// VAR
// 	i :	INT;
// 	o : INT := 1;
// 	w : INT := 1;
// 	pt : POINTER TO ARRAY[1..LIST_LENGTH] OF BYTE;
// 	po : POINTER TO ARRAY[1..LIST_LENGTH] OF BYTE;
// 	cnt: INT := 0;
// 	c: BYTE;
// END_VAR
//
// (* load pointers *)
// pt := ADR(LIST);
// po := ADR(LIST_RETRIEVE);
//
// IF pos > 0 THEN
// 	FOR i := 1 TO LIST_LENGTH DO
// 		c := pt^[i];
// 		IF c = 0 THEN
// 			po^[o] := 0;
// 			IF cnt < pos THEN pt^[w + 1] := 0; ELSE pt^[w] := 0; END_IF;
// 			RETURN;
// 		ELSIF cnt = pos AND c <> SEP THEN				(* we have found the element *)
// 			po^[o] := pt^[i];
// 			o := o + 1;
// 		ELSIF cnt >= pos THEN
// 			pt^[w] := c;
// 			w := w + 1;
// 		ELSE
// 			w := i;
// 		END_IF;
// 		IF c = sep THEN cnt := cnt + 1; END_IF;
// 	END_FOR;
// ELSE
// 	LIST_RETRIEVE := '';
// END_IF;
//
//
// (* revision histroy
// hm	28. jun. 2008	rev 1.0
// 	original release
//
// hm	19. jan. 2011	rev 1.1
// 	changed string(255) to string(LIST_LENGTH)
//
// hm	21. mar. 2011	rev 2.0
// 	all elements start with SEP
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION LIST_RETRIEVE_LAST : STRING(LIST_LENGTH)
// VAR_INPUT
// 	SEP : BYTE;
// END_VAR
// VAR_IN_OUT
// 	LIST : STRING(LIST_LENGTH);
// END_VAR
// VAR
// 	i :	INT;
// 	last : INT := 1;
// 	pt : POINTER TO ARRAY[1..LIST_LENGTH] OF BYTE;
// 	c: BYTE;
// END_VAR
//
// (* load pointers *)
// pt := ADR(LIST);
//
// (* search position of last element *)
// FOR i := 1 TO LIST_LENGTH DO
// 	c := pt^[i];
// 	IF c = 0 THEN
// 		EXIT;
// 	ELSIF C = SEP THEN
// 		last := i;
// 	END_IF;
// END_FOR;
// (* return last element of list *)
// LIST_RETRIEVE_LAST := MID(LIST, LIST_LENGTH, last + 1);
// (* terminate list at i *)
// pt^[last] := 0;
//
//
//
// (* revision histroy
// hm	21. mar. 2011	rev 2.0
// 	new module
// *)
// END_FUNCTION

FUNCTION_BLOCK COUNT_BR
VAR_INPUT
	SET : BOOL;
	IN : BYTE;
	UP : BOOL;
	DN : BOOL;
	STEP : BYTE := 1;
	MX : BYTE := 255;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	CNT : BYTE;
END_VAR
VAR
	last_up: BOOL;
	last_dn: BOOL;
END_VAR

IF rst THEN
	cnt := 0;
ELSIF set THEN
	cnt := LIMIT(0,in,mx);
ELSIF up AND NOT last_up THEN
	cnt := INT_TO_BYTE(INC(cnt,step,mx));
ELSIF dn AND NOT last_dn THEN
	cnt := INT_TO_BYTE(inc(cnt,-step,mx));
END_IF;
last_up := up;
last_dn := dn;
END_FUNCTION_BLOCK

FUNCTION_BLOCK COUNT_DR
VAR_INPUT
	SET : BOOL;
	IN : DWORD;
	UP : BOOL;
	DN : BOOL;
	STEP : DWORD := 1;
	MX : DWORD := 16#FFFFFFFF;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	CNT : DWORD;
END_VAR
VAR
	last_up: BOOL;
	last_dn: BOOL;
END_VAR

IF rst THEN
	cnt := 0;
ELSIF set THEN
	cnt := LIMIT(DWORD#0,in,mx);
ELSIF up AND NOT last_up THEN
	IF STEP > ULINT_TO_DWORD((DWORD_TO_ULINT(MX) + 4294967296 - DWORD_TO_ULINT(CNT)) MOD 4294967296) THEN
		CNT := ULINT_TO_DWORD((DWORD_TO_ULINT(CNT) + 4294967296 - DWORD_TO_ULINT(MX) + DWORD_TO_ULINT(STEP) - 1) MOD 4294967296);
	ELSE
		CNT := ULINT_TO_DWORD((DWORD_TO_ULINT(CNT) + DWORD_TO_ULINT(STEP)) MOD 4294967296);
	END_IF;
ELSIF dn AND NOT last_dn THEN
	IF STEP > CNT THEN
		CNT := ULINT_TO_DWORD((DWORD_TO_ULINT(CNT) + 4294967296 - DWORD_TO_ULINT(STEP) + DWORD_TO_ULINT(MX) + 1) MOD 4294967296);
	ELSE
		CNT := ULINT_TO_DWORD((DWORD_TO_ULINT(CNT) + 4294967296 - DWORD_TO_ULINT(STEP)) MOD 4294967296);
	END_IF;
END_IF;
last_up := up;
last_dn := dn;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FF_D2E
VAR_INPUT
	D0 : BOOL;
	D1 : BOOL;
	CLK : BOOL;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Q0 : BOOL;
	Q1 : BOOL;
END_VAR
VAR
	edge : BOOL;
END_VAR


IF rst THEN
	Q0 := FALSE;
	Q1 := FALSE;
ELSIF clk AND NOT edge THEN
	Q0 := D0;
	Q1 := D1;
END_IF;
edge := CLK;

END_FUNCTION_BLOCK

FUNCTION_BLOCK FF_D4E
VAR_INPUT
	D0 : BOOL;
	D1 : BOOL;
	D2 : BOOL;
	D3 : BOOL;
	CLK : BOOL;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Q0 : BOOL;
	Q1 : BOOL;
	Q2 : BOOL;
	Q3 : BOOL;
END_VAR
VAR
	edge : BOOL;
END_VAR


IF rst THEN
	Q0 := FALSE;
	Q1 := FALSE;
	Q2 := FALSE;
	Q3 := FALSE;
ELSIF clk AND NOT edge THEN
	Q0 := D0;
	Q1 := D1;
	Q2 := D2;
	Q3 := D3;
END_IF;
edge := CLK;

END_FUNCTION_BLOCK

FUNCTION_BLOCK FF_DRE
VAR_INPUT
	SET : BOOL;
	D : BOOL;
	CLK : BOOL;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	edge : BOOL;
END_VAR


IF rst OR set THEN
	Q := NOT rst;
ELSIF clk AND NOT edge THEN
	Q := D;
END_IF;
edge := CLK;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FF_JKE
VAR_INPUT
	SET : BOOL;
	J : BOOL;
	CLK : BOOL;
	K : BOOL;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	edge : BOOL;
END_VAR


IF rst OR set THEN
	Q := NOT rst;
ELSIF clk AND NOT edge THEN
	IF J XOR K THEN Q := J;
	ELSE Q := K XOR Q;
	END_IF;
END_IF;
edge := CLK;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FF_RSE
VAR_INPUT
	CS : BOOL;
	CR : BOOL;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	es, er : BOOL;
END_VAR


IF rst THEN
	(* asynchronous reset *)
	Q := FALSE;
ELSIF CR AND NOT er THEN
	(* rising edge on CR *)
	Q := FALSE;
ELSIF CS AND NOT es THEN
	(* rising edge on CS *)
	Q := TRUE;
END_IF;

es := CS;
er := CR;




END_FUNCTION_BLOCK

FUNCTION_BLOCK SELECT_8
VAR_INPUT
	E : BOOL;
	SET : BOOL;
	IN : BYTE;
	UP : BOOL;
	DN : BOOL;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Q0, Q1, Q2, Q3, Q4, Q5, Q6, Q7 : BOOL;
	STATE : INT;
END_VAR
VAR
	last_up: BOOL;
	last_dn : BOOL;
END_VAR

IF rst THEN
	state := 0;
ELSIF set THEN
	state := IN;
ELSIF up AND NOT last_up THEN
	state := inc(state,1,7);
ELSIF dn AND NOT last_dn THEN
	state := inc(state,-1,7);
END_IF;
last_UP := UP;
last_DN := DN;

Q0 := FALSE;
Q1 := FALSE;
Q2 := FALSE;
Q3 := FALSE;
Q4 := FALSE;
Q5 := FALSE;
Q6 := FALSE;
Q7 := FALSE;

IF E THEN
	CASE state OF
		0: Q0 := TRUE;
		1: Q1 := TRUE;
		2: Q2 := TRUE;
		3: Q3 := TRUE;
		4: Q4 := TRUE;
		5: Q5 := TRUE;
		6: Q6 := TRUE;
		7: Q7 := TRUE;
	END_CASE;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK SHR_4E
VAR_INPUT
	SET : BOOL;
	D0: BOOL;
	CLK: BOOL;
	RST: BOOL;
END_VAR
VAR_OUTPUT
	Q0: BOOL;
	Q1: BOOL;
	Q2: BOOL;
	Q3: BOOL;
END_VAR
VAR
	trig : R_TRIG;
END_VAR

(* trig.Q signals a rising edge on clk *)
trig(clk := clk);

IF set OR rst THEN
	Q0 := NOT rst;
	Q1 := Q0;
	Q2 := Q0;
	Q3 := Q0;
ELSIF trig.Q THEN
	Q3 := Q2;
	Q2 := Q1;
	Q1 := Q0;
	Q0 := D0;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK SHR_4UDE
VAR_INPUT
	SET : BOOL;
	D0: BOOL;
	D3: BOOL;
	CLK: BOOL;
	DN : BOOL;
	RST: BOOL;

END_VAR
VAR_OUTPUT
	Q0: BOOL;
	Q1: BOOL;
	Q2: BOOL;
	Q3: BOOL;
END_VAR
VAR
	trig : R_TRIG;
END_VAR

(* trig.Q signals a rising edge on clk *)
trig(clk := clk);

IF set OR rst THEN
	Q0 := NOT RST;
	Q1 := Q0;
	Q2 := Q0;
	Q3 := Q0;
ELSIF trig.Q THEN
	IF dn THEN
		Q0 := Q1;
		Q1 := Q2;
		Q2 := Q3;
		Q3 := D3;
	ELSE
		Q3 := Q2;
		Q2 := Q1;
		Q1 := Q0;
		Q0 := D0;
	END_IF;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK SHR_8PLE
VAR_INPUT
	Din : BOOL;
	Dload : BYTE;
	CLK: BOOL;
	UP: BOOL :=1;
	load : BOOL;
	RST: BOOL;
END_VAR
VAR_OUTPUT
	DOut: BOOL;
END_VAR
VAR
	edge : BOOL :=1;
	register : BYTE;
END_VAR

(* flankenerkennung clk wird high und edge war high reset ist nicht aktiv und set ist nicht aktiv *)
IF CLK AND edge AND NOT rst THEN
	edge := FALSE;	(* flanke wurde erkannt und weitere flankenerkennung wird verhindert bis edge wieder true ist *)
	(* hier ist der code f�r das flankenevent *)
	IF UP THEN						(*shift up *)
		register := SHL(register,1);
		register.0 := Din;
		Dout := register.7;
	ELSE								(* shift down *);
		register := SHR(register,1);
		register.7 := Din;
		Dout := register.0;
	END_IF;
	IF load THEN							(* the byte on Din will be loaded if load = true *)
		register := Dload;
		IF up THEN Dout := register.7; ELSE Dout := register.0; END_IF;
	END_IF;
END_IF;
IF NOT clk THEN edge := TRUE; END_IF;	(* sobald clk wieder low wird warten auf n�chste flanke *)
IF rst THEN									(* wenn reset aktiv dann ausgang r�cksetzen *)
	register := 0;
	Dout := FALSE;
END_IF;

END_FUNCTION_BLOCK

FUNCTION_BLOCK SHR_8UDE
VAR_INPUT
	SET : BOOL;
	D0: BOOL;
	D7: BOOL;
	CLK: BOOL;
	DN : BOOL;
	RST: BOOL;
END_VAR
VAR_OUTPUT
	Q0: BOOL;
	Q1: BOOL;
	Q2: BOOL;
	Q3: BOOL;
	Q4: BOOL;
	Q5: BOOL;
	Q6: BOOL;
	Q7: BOOL;
END_VAR
VAR
	trig : R_TRIG;
END_VAR

(* trig.Q signals a rising edge on clk *)
trig(clk := clk);

IF set OR rst THEN
	Q0 := NOT RST;
	Q1 := Q0;
	Q2 := Q0;
	Q3 := Q0;
	Q4 := Q0;
	Q5 := Q0;
	Q6 := Q0;
	Q7 := Q0;
ELSIF trig.Q THEN
	IF dn THEN
		Q0 := Q1;
		Q1 := Q2;
		Q2 := Q3;
		Q3 := Q4;
		Q4 := Q5;
		Q5 := Q6;
		Q6 := Q7;
		Q7 := D7;
	ELSE
		Q7 := Q6;
		Q6 := Q5;
		Q5 := Q4;
		Q4 := Q3;
		Q3 := Q2;
		Q2 := Q1;
		Q1 := Q0;
		Q0 := D0;
	END_IF;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK TOGGLE
VAR_INPUT
	CLK : BOOL;
	rst : BOOL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	edge : BOOL;
END_VAR


IF rst THEN
	q := 0;
ELSIF clk AND NOT edge THEN
	Q := NOT Q;
END_IF;
edge := clk;




END_FUNCTION_BLOCK

FUNCTION_BLOCK LTCH
VAR_INPUT
	D : BOOL;
	L : BOOL;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR


(* as long as L is true, the latch is transparent and and change of D is transferred to Q *)
(* of course only when there is no asynchronous reset *)
IF rst THEN				(* if asynchronous reset then Q=0 *)
	Q := FALSE;
ELSIF L THEN			(* latch is transparent *)
	Q := D;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK LTCH_4
VAR_INPUT
	D0 : BOOL;
	D1 : BOOL;
	D2 : BOOL;
	D3 : BOOL;
	L : BOOL;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Q0 : BOOL;
	Q1 : BOOL;
	Q2 : BOOL;
	Q3 : BOOL;
END_VAR


(* as long as L is true, the latch is transparent and and change of D is transferred to Q *)
(* of course only when there is no asynchronous reset *)
IF rst THEN			(* if asynchronous reset then Q=0 *)
	Q0 := FALSE;
	Q1 := FALSE;
	Q2 := FALSE;
	Q3 := FALSE;
ELSIF L THEN			(* latch is transparent *)
	Q0 := D0;
	Q1 := D1;
	Q2 := D2;
	Q3 := D3;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK STORE_8
VAR_INPUT
	Set : BOOL;
	D0,D1, D2, D3, D4, D5, D6, D7 : BOOL;
	Clr : BOOL;
	Rst : BOOL;
END_VAR
VAR_OUTPUT
	Q0, Q1, Q2, Q3, Q4, Q5, Q6, Q7 : BOOL;
END_VAR
VAR
	edge: BOOL;
END_VAR

IF rst OR set THEN
	q0 := NOT rst;
	q1 := q0;
	q2 := q0;
	q3 := q0;
	q4 := q0;
	q5 := q0;
	q6 := q0;
	q7 := q0;
ELSE
	IF D0 THEN Q0 := TRUE; END_IF;
	IF D1 THEN Q1 := TRUE; END_IF;
	IF D2 THEN Q2 := TRUE; END_IF;
	IF D3 THEN Q3 := TRUE; END_IF;
	IF D4 THEN Q4 := TRUE; END_IF;
	IF D5 THEN Q5 := TRUE; END_IF;
	IF D6 THEN Q6 := TRUE; END_IF;
	IF D7 THEN Q7 := TRUE; END_IF;

	IF clr AND NOT edge THEN
		IF q0 THEN q0 := FALSE;
		ELSIF q1 THEN q1 := FALSE;
		ELSIF q2 THEN q2 := FALSE;
		ELSIF q3 THEN q3 := FALSE;
		ELSIF q4 THEN q4 := FALSE;
		ELSIF q5 THEN q5 := FALSE;
		ELSIF q6 THEN q6 := FALSE;
		ELSE q7 := FALSE;
		END_IF;
	END_IF;
	edge := clr;
END_IF;


END_FUNCTION_BLOCK

FUNCTION BCDC_TO_INT : INT
VAR_INPUT
	IN : BYTE;
END_VAR


BCDC_TO_INT := BYTE_TO_INT(in AND 16#0F) + BYTE_TO_INT(SHR(in,4)) * 10;
END_FUNCTION

FUNCTION BIT_COUNT : INT
VAR_INPUT
	IN : DWORD;
END_VAR
VAR
	IN_work : DWORD;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
IN_work := IN;

WHILE IN_work > 0 DO
	IF IN_work.0 THEN Bit_Count := Bit_Count + 1; END_IF;
	IN_work := SHR(IN_work,1);
END_WHILE;
END_FUNCTION

FUNCTION BIT_LOAD_B : BYTE
VAR_INPUT
	IN : BYTE;
	VAL : BOOL;
	POS : INT;
END_VAR
VAR CONSTANT
	dat : BYTE := 1;
END_VAR

IF VAL THEN
	BIT_LOAD_B := in OR SHL(dat,pos);
ELSE
	BIT_LOAD_B := in AND (NOT SHL(dat,pos));
END_IF;

END_FUNCTION

FUNCTION BIT_LOAD_B2 : BYTE
VAR_INPUT
	I : BYTE;
	D : BOOL;
	P : INT;
	N : INT;
END_VAR

IF D THEN
	BIT_LOAD_B2 := ROL(SHR(BYTE#255, 8 - N) OR ROR(i, P), P);
ELSE
	BIT_LOAD_B2 := ROL(SHL(BYTE#255, N) AND ROR(I, P), P);
END_IF;

END_FUNCTION

FUNCTION BIT_LOAD_DW : DWORD
VAR_INPUT
	IN : DWORD;
	VAL : BOOL;
	POS : INT;
END_VAR
VAR CONSTANT
	dat : DWORD := 1;
END_VAR

IF val THEN
	BIT_LOAD_DW := in OR SHL(dat,pos);
ELSE
	BIT_LOAD_DW := in AND (NOT SHL(dat,pos));
END_IF;


END_FUNCTION

FUNCTION BIT_LOAD_DW2 : DWORD
VAR_INPUT
	I : DWORD;
	D : BOOL;
	P : INT;
	N : INT;
END_VAR

IF D THEN
	BIT_LOAD_DW2 := ROL(SHR(DWORD#4294967295, 32 - N) OR ROR(i, P), P);
ELSE
	BIT_LOAD_DW2 := ROL(SHL(DWORD#4294967295, N) AND ROR(I, P), P);
END_IF;

END_FUNCTION

FUNCTION BIT_LOAD_W : WORD
VAR_INPUT
	IN : WORD;
	VAL : BOOL;
	POS : INT;
END_VAR
VAR CONSTANT
	dat : WORD := 1;
END_VAR

IF val THEN
	BIT_LOAD_W := in OR SHL(dat,pos);
ELSE
	BIT_LOAD_W := in AND (NOT SHL(dat,pos));
END_IF;

END_FUNCTION

FUNCTION BIT_LOAD_W2 : WORD
VAR_INPUT
	I : WORD;
	D : BOOL;
	P : INT;
	N : INT;
END_VAR

IF D THEN
	BIT_LOAD_W2 := ROL(SHR(WORD#65535, 16 - N) OR ROR(i, P), P);
ELSE
	BIT_LOAD_W2 := ROL(SHL(WORD#65535, N) AND ROR(I, P), P);
END_IF;

END_FUNCTION

FUNCTION BIT_OF_DWORD : BOOL
VAR_INPUT
	in : DWORD;
	N : INT;
END_VAR

BIT_OF_DWORD := (SHR(in,N) AND 16#00000001) > 0;


(* old code used before rev 1.1
temp := SHR(in,n);
Bit_of_Dword := temp.0;
*)

END_FUNCTION

FUNCTION BIT_TOGGLE_B : BYTE
VAR_INPUT
	IN : BYTE;
	POS : INT;
END_VAR

BIT_TOGGLE_B := SHL(BYTE#1, POS) XOR IN;

END_FUNCTION

FUNCTION BIT_TOGGLE_DW : DWORD
VAR_INPUT
	IN : DWORD;
	POS : INT;
END_VAR

BIT_TOGGLE_DW := SHL(DWORD#1, POS) XOR IN;

END_FUNCTION

FUNCTION BIT_TOGGLE_W : WORD
VAR_INPUT
	IN : WORD;
	POS : INT;
END_VAR

BIT_TOGGLE_W := SHL(WORD#1, POS) XOR IN;

END_FUNCTION

FUNCTION BYTE_OF_BIT : BYTE
VAR_INPUT
	B0:BOOL;
	B1:BOOL;
	B2:BOOL;
	B3:BOOL;
	B4:BOOL;
	B5:BOOL;
	B6:BOOL;
	B7:BOOL;
END_VAR

Byte_of_bit := SHL(SHL(SHL(SHL(SHL(SHL(SHL(BOOL_TO_BYTE(B7),1) OR BOOL_TO_BYTE(B6),1) OR BOOL_TO_BYTE(B5),1) OR BOOL_TO_BYTE(B4),1)
	OR BOOL_TO_BYTE(B3),1) OR BOOL_TO_BYTE(B2),1) OR BOOL_TO_BYTE(B1),1) OR BOOL_TO_BYTE(B0);
END_FUNCTION

FUNCTION BYTE_OF_DWORD : BYTE
VAR_INPUT
	in : DWORD;
	N : BYTE;
END_VAR


BYTE_OF_DWORD := DWORD_TO_BYTE(SHR(in,SHL(n,3)));


END_FUNCTION

FUNCTION_BLOCK BYTE_TO_BITS
VAR_INPUT
	IN: BYTE;
END_VAR
VAR_OUTPUT
	B0: BOOL;
	B1: BOOL;
	B2: BOOL;
	B3: BOOL;
	B4: BOOL;
	B5: BOOL;
	B6: BOOL;
	B7: BOOL;
END_VAR

B0 := IN.0;
B1 := IN.1;
B2 := IN.2;
B3 := IN.3;
B4 := IN.4;
B5 := IN.5;
B6 := IN.6;
B7 := IN.7;
END_FUNCTION_BLOCK

FUNCTION BYTE_TO_GRAY : BYTE
VAR_INPUT
	IN : BYTE;
END_VAR


BYTE_TO_GRAY := IN XOR SHR(IN,1);
END_FUNCTION

FUNCTION CHECK_PARITY : BOOL
VAR_INPUT
	in : DWORD;
	p : BOOL;
END_VAR
VAR
	in_work : DWORD;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
in_work := in;

CHECK_PARITY := NOT p;
WHILE in_work > 0 DO
	CHECK_PARITY := CHECK_PARITY XOR in_work.0 XOR in_work.1 XOR in_work.2 XOR in_work.3;
	in_work := SHR(in_work,4);
END_WHILE;

(* code before rev 1.2
WHILE in > 0 DO
	IF in.0 THEN cnt := cnt + 1; END_IF;
	in := SHR(in,1);
END_WHILE;
check_parity := even(cnt) XOR P;
*)
END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION CHK_REAL : BYTE
// VAR_INPUT
// 	X : REAL;
// END_VAR
// VAR
// 	pt : POINTER TO DWORD;
// 	tmp : DWORD;
// END_VAR
//
//
// pt := ADR(X);				(* move real to dword, real_to_dword does not work becasze it treats dword as a number on many systems *)
// tmp := ROL(pt^,1);			(* rotate left foir easy checking, sign will be at lease significant digit *)
// IF tmp < 16#FF000000 THEN
// 	CHK_REAL := 16#00;		(* normalized and denormalized numbers *)
// ELSIF tmp = 16#FF000000 THEN
// 	CHK_REAL := 16#20;		(* X is +infinity see IEEE754 *)
// ELSIF tmp = 16#FF000001 THEN
// 	CHK_REAL := 16#40;		(* X is -infinity see IEEE754 *)
// ELSE
// 	CHK_REAL := 16#80;		(* X is NAN see IEEE754 *)
// END_IF;
// END_FUNCTION

FUNCTION_BLOCK DEC_2
VAR_INPUT
	D: BOOL;
	A: BOOL;
END_VAR
VAR_OUTPUT
	Q0: BOOL;
	Q1: BOOL;
END_VAR

Q0 := D AND NOT A;
Q1 := D AND A;
END_FUNCTION_BLOCK

FUNCTION_BLOCK DEC_4
VAR_INPUT
	D: BOOL;
	A0: BOOL;
	A1: BOOL;
END_VAR
VAR_OUTPUT
	Q0: BOOL;
	Q1: BOOL;
	Q2: BOOL;
	Q3: BOOL;
END_VAR

Q0 := D AND NOT A0 AND NOT A1;
Q1 := D AND A0 AND NOT A1;
Q2 := D AND NOT A0 AND A1;
Q3 := D AND A0 AND A1;
END_FUNCTION_BLOCK

FUNCTION_BLOCK DEC_8
VAR_INPUT
	D: BOOL;
	A0: BOOL;
	A1: BOOL;
	A2: BOOL;
END_VAR
VAR_OUTPUT
	Q0: BOOL;
	Q1: BOOL;
	Q2: BOOL;
	Q3: BOOL;
	Q4: BOOL;
	Q5: BOOL;
	Q6: BOOL;
	Q7: BOOL;
END_VAR
VAR
	X : INT;
END_VAR

X.0 := A0; X.1 := A1; X.2 := A2;

Q0 := FALSE;
Q1 := FALSE;
Q2 := FALSE;
Q3 := FALSE;
Q4 := FALSE;
Q5 := FALSE;
Q6 := FALSE;
Q7 := FALSE;

CASE X OF
	0 : Q0 := D;
	1 : Q1 := D;
	2 : Q2 := D;
	3 : Q3 := D;
	4 : Q4 := D;
	5 : Q5 := D;
	6 : Q6 := D;
	7 : Q7 := D;
END_CASE;
END_FUNCTION_BLOCK

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION DW_TO_REAL : REAL
// VAR_INPUT
// 	X : DWORD;
// END_VAR
// VAR
// 	pt : POINTER TO REAL;
// END_VAR
//
//
// pt := ADR(X);
// DW_TO_REAL := pt^;
// END_FUNCTION

FUNCTION DWORD_OF_BYTE : DWORD
VAR_INPUT
	B3 : BYTE;
	B2 : BYTE;
	B1 : BYTE;
	B0 : BYTE;
END_VAR

DWORD_OF_BYTE := SHL(SHL(SHL(BYTE_TO_DWORD(B3),8) OR BYTE_TO_DWORD(B2),8) OR BYTE_TO_DWORD(B1),8) OR BYTE_TO_DWORD(B0);
END_FUNCTION

FUNCTION DWORD_OF_WORD : DWORD
VAR_INPUT
	W1 : WORD;
	W0 : WORD;
END_VAR

DWORD_OF_WORD := SHL(WORD_TO_DWORD(W1),16) OR WORD_TO_DWORD(W0);
END_FUNCTION

FUNCTION GRAY_TO_BYTE : BYTE
VAR_INPUT
	IN : BYTE;
END_VAR


GRAY_TO_BYTE := SHR(IN,4) XOR IN;
GRAY_TO_BYTE := SHR(GRAY_TO_BYTE,2) XOR GRAY_TO_BYTE;
GRAY_TO_BYTE := SHR(GRAY_TO_BYTE,1) XOR GRAY_TO_BYTE;
END_FUNCTION

FUNCTION INT_TO_BCDC : BYTE
VAR_INPUT
	IN : INT;
END_VAR


INT_TO_BCDC := SHL(INT_TO_BYTE(IN / INT#10),4) OR INT_TO_BYTE(in MOD INT#10);
END_FUNCTION

FUNCTION MUX_2 : BOOL
VAR_INPUT
	D0: BOOL;
	D1: BOOL;
	A0: BOOL;
END_VAR

MUX_2 := SEL(A0, D0, D1);
END_FUNCTION

FUNCTION MUX_4 : BOOL
VAR_INPUT
	D0: BOOL;
	D1: BOOL;
	D2: BOOL;
	D3: BOOL;
	A0: BOOL;
	A1: BOOL;
END_VAR

IF A1 THEN
	MUX_4 := SEL(A0, D2, D3);
ELSE
	MUX_4 := SEL(A0, D0, D1);
END_IF;
END_FUNCTION

FUNCTION PARITY : BOOL
VAR_INPUT
	in : DWORD;
END_VAR
VAR
	in_work : DWORD;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
in_work := in;


WHILE in_work > 0 DO
	PARITY := PARITY XOR in_work.0 XOR in_work.1 XOR in_work.2 XOR in_work.3;
	in_work := SHR(in_work,4);
END_WHILE;

(* code before rev 1.2
WHILE in > 0 DO
	IF in.0 THEN cnt := cnt +1; END_IF;
	in := SHR(in,1);
END_WHILE;
IF (cnt MOD 2) = 1 THEN parity := TRUE; ELSE parity := FALSE; END_IF;
*)
END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION REAL_TO_DW : DWORD
// VAR_INPUT
// 	X : REAL;
// END_VAR
// VAR
// 	pt : POINTER TO DWORD;
// END_VAR
//
//
// pt := ADR(X);
// REAL_TO_DW := pt^;
// END_FUNCTION

FUNCTION REFLECT : DWORD
VAR_INPUT
	D : DWORD;
	L : INT;
END_VAR
VAR
	i : INT;
END_VAR
VAR
	D_work : DWORD;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
D_work := D;


REFLECT := 0;
FOR i := 1 TO L DO
	REFLECT := SHL(REFLECT, 1) OR BOOL_TO_DWORD(D_work.0);
	D_work := SHR(D_work, 1);
END_FOR;
REFLECT := REFLECT OR SHL(D_work, L);
END_FUNCTION

FUNCTION REVERSE : BYTE
VAR_INPUT
	IN : BYTE;
END_VAR

(* new code donated by rebler wolfgang *)
REVERSE := ROR(in,1) AND 2#10001000 OR ROL(in,1) AND 2#00010001 OR ROR(in,3) AND 2#01000100 OR ROL(in,3) AND 2#00100010;

(* old code
REVERSE := SHL(in,7) OR SHR(in,7) OR (ROR(in,3) AND 2#01000100) OR (ROL(in,3) AND 2#00100010)
	OR (SHL(in,1) AND 2#00010000) OR (SHR(in,1) AND 2#00001000);
*)
END_FUNCTION

FUNCTION SHL1 : DWORD
VAR_INPUT
	IN : DWORD;
	N : INT;
END_VAR
VAR CONSTANT
	temp : DWORD := 16#FFFF_FFFF;
END_VAR

SHL1 := SHR(temp,32-N) OR SHL(IN,N);
END_FUNCTION

FUNCTION SHR1 : DWORD
VAR_INPUT
	IN : DWORD;
	N : INT;
END_VAR
VAR CONSTANT
	temp : DWORD := 16#FFFF_FFFF;
END_VAR

SHR1 := SHL(temp,32-N) OR SHR(IN,N);
END_FUNCTION

FUNCTION SWAP_BYTE : WORD
VAR_INPUT
	IN : WORD;
END_VAR

SWAP_BYTE := ROL(in,8);
END_FUNCTION

FUNCTION SWAP_BYTE2 : DWORD
VAR_INPUT
	IN : DWORD;
END_VAR

Swap_Byte2 := (ROR(in,8) AND 16#FF00FF00) OR (ROL(in,8) AND 16#00FF00FF);

END_FUNCTION

FUNCTION WORD_OF_BYTE : WORD
VAR_INPUT
	B1 : BYTE;
	B0 : BYTE;
END_VAR

WORD_OF_BYTE := SHL(BYTE_TO_WORD(B1),8) OR BYTE_TO_WORD(B0);
END_FUNCTION

FUNCTION WORD_OF_DWORD : WORD
VAR_INPUT
	in : DWORD;
	N : BYTE;
END_VAR


WORD_OF_DWORD := DWORD_TO_WORD(SHR(in,SHL(n,4)));

END_FUNCTION

FUNCTION_BLOCK A_TRIG
VAR_INPUT
	IN : REAL;
	RES : REAL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	D : REAL;
END_VAR
VAR
	last_in : REAL;
END_VAR


D := IN - LAST_IN;
Q := ABS(D) > res;
IF Q THEN last_in := IN; END_IF;
D := IN - LAST_IN;
END_FUNCTION_BLOCK

FUNCTION_BLOCK B_TRIG
VAR_INPUT
	CLK : BOOL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	edge : BOOL;
END_VAR


Q := clk XOR edge;
edge := CLK;
END_FUNCTION_BLOCK

FUNCTION_BLOCK CLICK_CNT
VAR_INPUT
	IN : BOOL;
	N : INT;
	TC : TIME;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	tx : TP;
	edge: BOOL;
	cnt: INT := -1;
END_VAR


(* Q shall only be active for one cycle only *)
Q := FALSE;

IF in AND NOT edge AND NOT tx.q THEN
	(* a rising edge on in sets the counter to 0 *)
	cnt := 0;
ELSIF tx.Q AND NOT IN AND edge THEN
	(* count falling edges when tp.q is true *)
	cnt := cnt + 1;
ELSIF NOT tx.Q THEN
	Q := cnt = N;
	cnt := -1;
END_IF;

(* remember the status of IN *)
edge := IN;
tx(in := IN, pt := TC);
END_FUNCTION_BLOCK

FUNCTION_BLOCK CLICK_DEC
VAR_INPUT
	IN : BOOL;
	TC : TIME;
END_VAR
VAR_OUTPUT
	Q0, Q1, Q2, Q3 : BOOL;
END_VAR
VAR
	tx : TP;
	edge: BOOL;
	cnt: INT := -1;
END_VAR


(* Q shall only be active for one cycle only *)
IF in = FALSE THEN
	Q0 := FALSE;
	Q1 := FALSE;
	Q2:= FALSE;
	Q3 := FALSE;
END_IF;

IF in AND NOT edge AND NOT tx.q THEN
	(* a rising edge on in sets the counter to 0 *)
	cnt := 0;
ELSIF tx.Q AND NOT IN AND edge THEN
	(* count falling edges when tp.q is true *)
	cnt := cnt + 1;
ELSIF NOT tx.Q THEN
	CASE cnt OF
		0 : Q0 := TRUE;
		1 : Q1 := TRUE;
		2 : Q2 := TRUE;
		3 : Q3 := TRUE;
	END_CASE;
	cnt := -1;
END_IF;

(* remember the status of IN *)
edge := IN;
tx(in := IN, pt := TC);
END_FUNCTION_BLOCK

FUNCTION_BLOCK CLK_DIV
VAR_INPUT
	clk : BOOL;
	rst: BOOL;
END_VAR
VAR_OUTPUT
	Q0 : BOOL;
	Q1 : BOOL;
	Q2 : BOOL;
	Q3 : BOOL;
	Q4 : BOOL;
	Q5 : BOOL;
	Q6 : BOOL;
	Q7 : BOOL;
END_VAR
VAR
	cnt: BYTE;
END_VAR


IF rst THEN
	cnt:= 0;
	Q0 := 0;
	Q1 := 0;
	Q2 := 0;
	Q3 := 0;
	Q4 := 0;
	Q5 := 0;
	Q6 := 0;
	Q7 := 0;
ELSIF clk THEN
	cnt := ULINT_TO_BYTE((BYTE_TO_ULINT(cnt) + 1) MOD 256);
	Q0 := cnt.0;
	Q1 := cnt.1;
	Q2 := cnt.2;
	Q3 := cnt.3;
	Q4 := cnt.4;
	Q5 := cnt.5;
	Q6 := cnt.6;
	Q7 := cnt.7;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK CLK_N
VAR_INPUT
	N : INT;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	edge : BOOL;
	stime: DWORD;
	clk: BOOL;
END_VAR

stime := SHR(T_PLC_MS(),N);
clk := stime.0;
Q := clk XOR edge;
edge := CLK;
END_FUNCTION_BLOCK

	FUNCTION_BLOCK CLK_PRG
VAR_INPUT
	PT : TIME := t#10ms;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	init : BOOL;
	last : TIME;
	tx: TIME;
END_VAR

(* read system time *)
tx := DWORD_TO_TIME(T_PLC_MS());

(* initialize on startup *)
IF NOT init THEN
	init := TRUE;
	last := tx - pt;
END_IF;

(* generate output pulse when next_pulse is reached *)
Q := tx - last >= pt;
IF Q THEN last := tx; END_IF;


(* revision hiostory

hm 25 feb 2007	rev 1.1
	rewritten code for higher performance
	pt can now be changed during runtime

hm	17. sep 2007	rev 1.2
	replaced time() with t_plc_ms() for compatibility reasons

hm	25. oct. 2008	rev 1.3
	optimized code

*)
END_FUNCTION_BLOCK

FUNCTION_BLOCK CLK_PULSE
VAR_INPUT
	PT : TIME;
	N : INT;
	RST : BOOL ;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	CNT : INT;
	RUN : BOOL;
END_VAR
VAR
	tx, tn: DWORD;
	init: BOOL;
END_VAR

tx := T_PLC_MS();		(* read system *)
Q := FALSE;				(* reset Q we generate pulses only for one cycle *)
RUN := CNT < N;

IF NOT init OR RST THEN
	init := TRUE;
	CNT := 0;
	tn := ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(TIME_TO_DWORD(PT))) MOD 4294967296);
	RUN := FALSE;
ELSIF (cnt < N OR N = 0) AND ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(tn)) MOD 4294967296) >= TIME_TO_DWORD(PT) THEN		(* generate a pulse *)
	CNT := CNT + 1;
	Q := TRUE;
	tn := ULINT_TO_DWORD((DWORD_TO_ULINT(tn) + TIME_TO_ULINT(PT)) MOD 4294967296);
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK CYCLE_4
VAR_INPUT
	E : BOOL := TRUE;
	T0, T1, T2, T3 : TIME;
	S0 : BOOL;
	SX : INT;
	SL : BOOL;
END_VAR
VAR_OUTPUT
	STATE : INT;
END_VAR
VAR
	tx : TIME;
	last : TIME;
	init: BOOL;
END_VAR
VAR
	SL_work : BOOL;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
SL_work := SL;

(* read system timer *)
tx := DWORD_TO_TIME(T_PLC_MS());
(* init on first cycle *)
IF NOT init THEN
	init := TRUE;
	last := tx;
END_IF;

IF E THEN
	IF SL_work THEN
		(* when sx > 0 then the state sx is forced to start *)
		state:= LIMIT(0,SX,3);
		last := tx;
		(* this is to avoid to reset sx from the calling programm it does work fine on codesys but i am not sure about other systems, because we are writing to an input *)
		SL_work := FALSE;
	ELSE
		CASE state OF
			0 :	(* wait for T0 and switch to next cycle *)
				IF tx - last >= T0 THEN
					state := 1;
					last := tx;
				END_IF;
			1 : (* wait for T1 over 1st cycle *)
				IF tx - last >= T1 THEN
					state := 2;
					last := tx;
				END_IF;
			2 : (* wait for T1 over 1st cycle *)
				IF tx - last >= T2 THEN
					state := 3;
					last := tx;
				END_IF;
			3 : (* wait for T2 over 2nd cycle *)
				IF tx - last >= T3 THEN
					IF S0 THEN State := 0; END_IF; (* if S0 is false, the sequence stops at state 3 *)
					last := tx;
				END_IF;
		END_CASE;
	END_IF;
ELSE
	state := 0;
	last := tx;
END_IF;



(*
hm	3. nov. 2008	rev 1.0
	original version

*)
END_FUNCTION_BLOCK

FUNCTION_BLOCK D_TRIG
VAR_INPUT
	in : DWORD;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	X : DWORD;
END_VAR
VAR
	last_in : DWORD;
END_VAR


Q := in <> last_in;
X := ULINT_TO_DWORD((DWORD_TO_ULINT(in) + 4294967296 - DWORD_TO_ULINT(last_in)) MOD 4294967296);
last_in := in;
END_FUNCTION_BLOCK

FUNCTION_BLOCK GEN_BIT
VAR_INPUT
	in0 : DWORD;
	in1 : DWORD;
	in2 : DWORD;
	in3 : DWORD;
	clk : BOOL;
	steps : INT;
	rep : INT;
	rst : BOOL;
END_VAR
VAR_OUTPUT
	Q0 :BOOL;
	Q1 :BOOL;
	Q2 :BOOL;
	Q3 :BOOL;
	cnt : INT;
	run : BOOL;
END_VAR
VAR
	r0: DWORD;
	r1: DWORD;
	r2: DWORD;
	r3: DWORD;
	rx : INT := 1;
END_VAR

(* check if number of runs is finished or rep = 0 which means continuous *)

IF clk AND NOT rst THEN
	run := (rep = 0) OR (rx <= rep);
	IF run THEN
		(* check for step counter reached and reset to 0 if cnt = steps *)
		IF cnt = steps THEN
			cnt := 0;
		END_IF;

		(* when cnt = 0 then reload the inputs into the registers *)
		IF cnt = 0 THEN
			r0 := in0;
			r1 := in1;
			r2 := in2;
			r3 := in3;
		END_IF;

		(* when cnt < steps, shift the lowest bits to the outputs *)
		IF (cnt < steps) THEN
			Q0 := r0.0;
			Q1 := r1.0;
			Q2 := r2.0;
			Q3 := r3.0;
			r0 := SHR(r0,1);
			r1 := SHR(r1,1);
			r2 := SHR(r2,1);
			r3 := SHR(r3,1);
		END_IF;

		(* increment the step counter *)
		cnt := cnt +1;
		IF (cnt = steps) AND (rep <> 0) THEN rx := rx +1; END_IF;
		IF (rx > rep) AND (rep <> 0) THEN run := FALSE; END_IF;
	END_IF;
ELSE
	IF rst THEN
		run := FALSE;
		Q0 := FALSE;
		Q1 := FALSE;
		Q2 := FALSE;
		Q3 := FALSE;
		r0 := 0;
		r1 := 0;
		r2 := 0;
		r3 := 0;
		cnt := 0;
		rx := 1;
	END_IF;
END_IF;



(* revision histroy
hm	4 aug 2006	rev 1.0
	original version

hm 15. oct. 2008	rev 1.1
	improved performance

hm	14. mar. 2009	rev 1.2
	removed double assignments

*)
END_FUNCTION_BLOCK

FUNCTION_BLOCK GEN_SQ
VAR_INPUT
	PT : TIME;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	tn: DWORD;
	tx: DWORD;
	init : BOOL;
END_VAR

(* read system time *)
tx := T_PLC_MS();

IF NOT init THEN
	init := TRUE;
	tn := tx;
	Q := TRUE;
ELSIF ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(tn)) MOD 4294967296) >= SHR(TIME_TO_DWORD(PT),1) THEN
	Q := NOT Q;
	tn := ULINT_TO_DWORD((DWORD_TO_ULINT(tn) + DWORD_TO_ULINT(SHR(TIME_TO_DWORD(pt),1))) MOD 4294967296);
END_IF;




END_FUNCTION_BLOCK

FUNCTION_BLOCK SCHEDULER
VAR_INPUT
	E0 : BOOL;
	E1 : BOOL;
	E2 : BOOL;
	E3 : BOOL;
END_VAR
VAR_INPUT CONSTANT
	T0, T1, T2, T3 : TIME;
END_VAR
VAR_OUTPUT
	Q0, Q1, Q2, Q3 : BOOL;
END_VAR
VAR
	init : BOOL;
	s0, s1, s2, s3 : TIME;
	tx : TIME;
	c : INT;
END_VAR


(* read system_time *)
tx := DWORD_TO_TIME(T_PLC_MS());

IF NOT init THEN
	init := TRUE;
	s0 := tx - T0;
	s1 := tx - T1;
	s2 := tx - T2;
	s3 := tx - T3;
END_IF;

Q0 := FALSE;
Q1 := FALSE;
Q2 := FALSE;
Q3 :=FALSE;

CASE c OF
	0: 	IF tx - s0 >= T0 THEN
			Q0 := E0;
			s0 := tx;
		END_IF;
		c := 1;
	1: 	IF tx - s1 >= T1 THEN
			Q1 := E1;
			s1 := tx;
		END_IF;
		c := 2;
	2: 	IF tx - s2 >= T2 THEN
			Q2 := E2;
			s2 := tx;
		END_IF;
		c := 3;
	3: 	IF tx - s3 >= T3 THEN
			Q3 := E3;
			s3 := tx;
		END_IF;
		c := 0;
END_CASE;
END_FUNCTION_BLOCK

FUNCTION_BLOCK SCHEDULER_2
VAR_INPUT
	E0 : BOOL;
	E1 : BOOL;
	E2 : BOOL;
	E3 : BOOL;
END_VAR
VAR_INPUT CONSTANT
	C0, C1, C2, C3 : UINT;
	O0, O1, O2, O3 : UINT;
END_VAR
VAR_OUTPUT
	Q0, Q1, Q2, Q3 : BOOL;
END_VAR
VAR
	sx : UINT;
END_VAR


Q0 := E0 AND (sx MOD C0 - O0 = 0);
Q1 := E1 AND (sx MOD C1 - O1 = 0);
Q2 := E2 AND (sx MOD C2 - O2 = 0);
Q3 := E3 AND (sx MOD C3 - O3 = 0);

(* increment cycle counter every cycle *)
sx := sx + 1;
END_FUNCTION_BLOCK

FUNCTION_BLOCK SEQUENCE_4
VAR_INPUT
	in0 : BOOL := TRUE;
	in1 : BOOL := TRUE;
	in2 : BOOL := TRUE;
	in3 : BOOL := TRUE;
	start : BOOL;
	rst : BOOL;
	wait0 : TIME;
	delay0 : TIME;
	wait1 : TIME;
	delay1 : TIME;
	wait2 : TIME;
	delay2 : TIME;
	wait3 : TIME;
	delay3 : TIME;
END_VAR
VAR_INPUT CONSTANT
	stop_on_error : BOOL;
END_VAR
VAR_OUTPUT
	Q0 : BOOL;
	Q1 : BOOL;
	Q2 : BOOL;
	Q3 : BOOL;
	QX : BOOL;
	run: BOOL;
	step : INT := -1;
	status : BYTE;
END_VAR
VAR
	last : TIME;
	edge : BOOL;
	tx: TIME;
	init: BOOL;
END_VAR

(* read sps timer *)
tx := DWORD_TO_TIME(T_PLC_MS());

(* initialize on startup *)
IF NOT init THEN
	last := tx;
	init := TRUE;
	status := 110;
END_IF;

(* asynchronous reset *)
IF rst THEN
	step := -1;
	Q0 := 0;
	Q1 := 0;
	Q2 := 0;
	Q3 := 0;
	status := 110;
	run := 0;

(* edge on start input restarts the sequencer *)
ELSIF start AND NOT edge THEN
	step := 0;
	last := tx;
	status := 111;
	Q0 := 0;
	Q1 := 0;
	Q2 := 0;
	Q3 := 0;
	run := 1;
END_IF;
edge := start;

(* check if stop on status is necessary *)
IF status > 0 AND status < 100 AND stop_on_error THEN RETURN; END_IF;

(* sequence is running *)
IF run AND step = 0 THEN
	IF NOT q0 AND in0 AND tx - last <= wait0 THEN
		Q0 := TRUE;
		last := tx;
	ELSIF NOT q0 AND tx - last > wait0 THEN
		status := 1;
		run := FALSE;
	ELSIF q0 AND tx - last >= delay0 THEN
		step := 1;
		last := tx;
	END_IF;
END_IF;
IF run AND step = 1 THEN
	IF NOT q1 AND in1 AND tx - last <= wait1 THEN
		Q0 := FALSE;
		Q1 := TRUE;
		last := tx;
	ELSIF NOT q1 AND Tx - last > wait1 THEN
		status := 2;
		q0 := FALSE;
		run := FALSE;
	ELSIF q1 AND tx - last >= delay1 THEN
		step := 2;
		last := tx;
	END_IF;
END_IF;
IF run AND step = 2 THEN
	IF NOT q2 AND in2 AND tx - last <= wait2 THEN
		Q1 := FALSE;
		Q2 := TRUE;
		last := tx;
	ELSIF NOT q2 AND Tx - last > wait2 THEN
		status := 3;
		q1 := FALSE;
		run := FALSE;
	ELSIF q2 AND tx - last >= delay2 THEN
		step := 3;
		last := tx;
	END_IF;
END_IF;
IF run AND step = 3 THEN
	IF NOT q3 AND in3 AND tx - last <= wait3 THEN
		Q2 := FALSE;
		Q3 := TRUE;
		last := tx;
	ELSIF NOT q3 AND Tx - last > wait3 THEN
		status := 4;
		q2 := FALSE;
		run := FALSE;
	ELSIF q3 AND tx - last >= delay3 THEN
		step := -1;
		q3 := FALSE;
		run := FALSE;
		status := 110;
	END_IF;
END_IF;
QX := q0 OR q1 OR q2 OR q3;

(*
hm 1.10.06		rev 1.1
	corrected delay logic to be after event and not before
	added any output

hm 1.12.06		rev 1.2
	corrected failure in sequence logic.
	added init at startup to prevent from initial error after start.

hm 17.1.2007		rev 1.3
	changed output fault to status for better compatibility with other modules (ESR).
	added stop on error functionality and setup variable
	default for inputs in0..3 is true.
	renamed variable state to step

hm	17.sep 2007	rev 1.4
	replaced time() with T_PLC_MS() for compatibility reasons

hm	13. mar. 2009	rev 1.5
	renamed output any to qx for compatibility resons

*)
END_FUNCTION_BLOCK

FUNCTION_BLOCK SEQUENCE_64
VAR_INPUT
	START : BOOL;
	SMAX : INT;
	PROG : ARRAY[0..63]OF TIME;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	STATE : INT := -1;
	TRIG : BOOL;
END_VAR
VAR
	tx : TIME;
	edge: BOOL;
	last: TIME;
END_VAR


(* read system timer *)
tx := DWORD_TO_TIME(T_PLC_MS());
TRIG := FALSE;

IF RST THEN
	STATE := -1;

(* start sequence *)
ELSIF START AND NOT edge THEN
	STATE := 0;
	last := tx;
	TRIG := TRUE;

(* sequence generator *)
ELSIF (STATE >= 0) THEN
	IF (tx - last) >= PROG[STATE] THEN
		STATE := INC2(STATE, 1, -1, SMAX);
		last := tx;
		TRIG := TRUE;
	END_IF;
END_IF;

edge := START;
END_FUNCTION_BLOCK

FUNCTION_BLOCK SEQUENCE_8
VAR_INPUT
	in0 : BOOL := TRUE;
	in1 : BOOL := TRUE;
	in2 : BOOL := TRUE;
	in3 : BOOL := TRUE;
	in4 : BOOL := TRUE;
	in5 : BOOL := TRUE;
	in6 : BOOL := TRUE;
	in7 : BOOL := TRUE;
	start : BOOL;
	rst : BOOL;
	wait0 : TIME;
	delay0 : TIME;
	wait1 : TIME;
	delay1 : TIME;
	wait2 : TIME;
	delay2 : TIME;
	wait3 : TIME;
	delay3 : TIME;
	wait4 : TIME;
	delay4 : TIME;
	wait5 : TIME;
	delay5 : TIME;
	wait6 : TIME;
	delay6 : TIME;
	wait7 : TIME;
	delay7 : TIME;
END_VAR
VAR_INPUT CONSTANT
	stop_on_error : BOOL;
END_VAR
VAR_OUTPUT
	Q0 : BOOL;
	Q1 : BOOL;
	Q2 : BOOL;
	Q3 : BOOL;
	Q4 : BOOL;
	Q5 : BOOL;
	Q6 : BOOL;
	Q7 : BOOL;
	QX : BOOL;
	run: BOOL;
	step : INT := -1;
	status : BYTE;
END_VAR
VAR
	last : TIME;
	edge : BOOL;
	tx: TIME;
	init: BOOL;
END_VAR

(* read sps timer *)
tx := DWORD_TO_TIME(T_PLC_MS());

(* initialize on startup *)
IF NOT init THEN
	last := tx;
	init := TRUE;
	status := 110;
END_IF;

(* asynchronous reset *)
IF rst THEN
	step := -1;
	Q0 := 0;
	Q1 := 0;
	Q2 := 0;
	Q3 := 0;
	Q4 := 0;
	Q5 := 0;
	Q6 := 0;
	Q7 := 0;
	status := 110;
	run := 0;

(* edge on start input restarts the sequencer *)
ELSIF start AND NOT edge THEN
	step := 0;
	last := tx;
	status := 111;
	Q0 := 0;
	Q1 := 0;
	Q2 := 0;
	Q3 := 0;
	Q4 := 0;
	Q5 := 0;
	Q6 := 0;
	Q7 := 0;
	run := 1;
END_IF;
edge := start;

(* check if stop on error is necessary *)
IF status > 0 AND status < 100 AND stop_on_error THEN RETURN; END_IF;

(* sequence is running *)
IF run AND step = 0 THEN
	IF NOT q0 AND in0 AND tx - last <= wait0 THEN
		Q0 := TRUE;
		last := tx;
	ELSIF NOT q0 AND tx - last > wait0 THEN
		status := 1;
		run := FALSE;
	ELSIF q0 AND tx - last >= delay0 THEN
		step := 1;
		last := tx;
	END_IF;
END_IF;
IF run AND step = 1 THEN
	IF NOT q1 AND in1 AND tx - last <= wait1 THEN
		Q0 := FALSE;
		Q1 := TRUE;
		last := tx;
	ELSIF NOT q1 AND Tx - last > wait1 THEN
		status := 2;
		q0 := FALSE;
		run := FALSE;
	ELSIF q1 AND tx - last >= delay1 THEN
		step := 2;
		last := tx;
	END_IF;
END_IF;
IF run AND step = 2 THEN
	IF NOT q2 AND in2 AND tx - last <= wait2 THEN
		Q1 := FALSE;
		Q2 := TRUE;
		last := tx;
	ELSIF NOT q2 AND Tx - last > wait2 THEN
		status := 3;
		q1 := FALSE;
		run := FALSE;
	ELSIF q2 AND tx - last >= delay2 THEN
		step := 3;
		last := tx;
	END_IF;
END_IF;
IF run AND step = 3 THEN
	IF NOT q3 AND in3 AND tx - last <= wait3 THEN
		Q2 := FALSE;
		Q3 := TRUE;
		last := tx;
	ELSIF NOT q3 AND Tx - last > wait3 THEN
		status := 4;
		q2 := FALSE;
		run := FALSE;
	ELSIF q3 AND tx - last >= delay3 THEN
		step := 4;
		last := tx;
	END_IF;
END_IF;
IF run AND step = 4 THEN
	IF NOT q4 AND in4 AND tx - last <= wait4 THEN
		Q3 := FALSE;
		Q4 := TRUE;
		last := tx;
	ELSIF NOT q4 AND Tx - last > wait4 THEN
		status := 5;
		q3 := FALSE;
		run := FALSE;
	ELSIF q4 AND tx - last >= delay4 THEN
		step := 5;
		last := tx;
	END_IF;
END_IF;
IF run AND step = 5 THEN
	IF NOT q5 AND in5 AND tx - last <= wait5 THEN
		Q4 := FALSE;
		Q5 := TRUE;
		last := tx;
	ELSIF NOT q5 AND Tx - last > wait5 THEN
		status := 6;
		q4 := FALSE;
		run := FALSE;
	ELSIF q5 AND tx - last >= delay5 THEN
		step := 6;
		last := tx;
	END_IF;
END_IF;
IF run AND step = 6 THEN
	IF NOT q6 AND in6 AND tx - last <= wait6 THEN
		Q5 := FALSE;
		Q6 := TRUE;
		last := tx;
	ELSIF NOT q6 AND Tx - last > wait6 THEN
		status := 7;
		q5 := FALSE;
		run := FALSE;
	ELSIF q6 AND tx - last >= delay6 THEN
		step := 7;
		last := tx;
	END_IF;
END_IF;
IF run AND step = 7 THEN
	IF NOT q7 AND in7 AND tx - last <= wait7 THEN
		Q6 := FALSE;
		Q7 := TRUE;
		last := tx;
	ELSIF NOT q7 AND Tx - last > wait7 THEN
		status := 8;
		q6 := FALSE;
		run := FALSE;
	ELSIF q7 AND tx - last >= delay7 THEN
		step := -1;
		Q7 := FALSE;
		Run := FALSE;
		status := 110;
	END_IF;
END_IF;
QX := q0 OR q1 OR q2 OR q3 OR q4 OR q5 OR q6 OR q7;

(*
hm 1.10.06		rev 1.1
	corrected delay logic to be after event and not before
	added any output

hm 1.12.06		rev 1.2
	corrected failure in sequence logic.
	added init at startup to prevent from initial statuss after start.

hm 17.1.2007		rev 1.3
	changed output fault to status for better compatibility with other modules (ESR)
	added stop on error functionality and setup variable
	default for inputs in0..7 is true.
	renames variable state to step

hm	17.sep 2007	rev 1.4
	replaced time() with T_PLC_MS() for compatibility reasons

hm	13. mar. 2009	rev 1.5
	renamed output any to qx for compatibility resons

*)
END_FUNCTION_BLOCK

FUNCTION_BLOCK TMAX
VAR_INPUT
	IN : BOOL;
	PT : TIME;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	Z : BOOL;
END_VAR
VAR
	tx : TIME;
	start : TIME;
	last_in: BOOL;
END_VAR


(* read system timer *)
tx := DWORD_TO_TIME(T_PLC_MS());

Z := FALSE;

IF NOT in THEN
	Q := FALSE;
ELSIF IN AND NOT last_in THEN
	Q := TRUE;
	start := tx;
ELSIF (tx - start >= PT) AND Q THEN
	Q := FALSE;
	Z := TRUE;
END_IF;

last_in := IN;
END_FUNCTION_BLOCK

FUNCTION_BLOCK TMIN
VAR_INPUT
	IN : BOOL;
	PT : TIME;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	pm : TP;
END_VAR


pm(in := IN, PT := PT);
Q := IN OR pm.Q;
END_FUNCTION_BLOCK

FUNCTION_BLOCK TOF_1
VAR_INPUT
	IN : BOOL;
	PT : TIME;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	tx : TIME;
	start : TIME;
END_VAR


(* read system timer *)
tx := DWORD_TO_TIME(T_PLC_MS());

IF RST THEN
	Q := FALSE;
ELSIF IN THEN
	Q := TRUE;
	start := tx;
ELSIF tx - start >= PT THEN
	Q := FALSE;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK TONOF
VAR_INPUT
	IN : BOOL;
	  T_ON,T_OFF : TIME;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
   X : TON;
   old: BOOL;
   mode: BOOL;
END_VAR

IF IN XOR old THEN
  X(IN := FALSE, PT := SEL(IN,T_OFF,T_ON));
  mode := IN;
  old := IN;
END_IF;
X(IN := TRUE);
IF X.Q THEN Q := mode;END_IF;

END_FUNCTION_BLOCK

FUNCTION_BLOCK TP_1
VAR_INPUT
	IN : BOOL;
	PT : TIME;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
END_VAR
VAR
	tx : TIME;
	start : TIME;
	ix : BOOL;
END_VAR


(* read system timer *)
tx := DWORD_TO_TIME(T_PLC_MS());

IF RST THEN
	Q := FALSE;
ELSIF IN AND NOT ix THEN
	Q := TRUE;
	start := tx;
ELSIF tx - start >= PT THEN
	Q := FALSE;
END_IF;

ix:= IN;
END_FUNCTION_BLOCK

FUNCTION_BLOCK TP_1D
VAR_INPUT
	IN : BOOL;
	PT1 : TIME;
	PTD : TIME;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	W : BOOL;
END_VAR
VAR
	tx : TIME;
	start: TIME;
	ix: BOOL;
END_VAR
VAR
	IN_work : BOOL;
	RST_work : BOOL;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
IN_work := IN;
RST_work := RST;


(* read system timer *)
tx := DWORD_TO_TIME(T_PLC_MS());

IF RST_work THEN
	Q := FALSE;
	RST_work := FALSE;
	W := FALSE;
ELSIF W THEN
	IF tx - start >= PTD THEN
		W := FALSE;
	END_IF;
ELSIF IN_work AND NOT ix THEN
	Q := TRUE;
	start := tx;
	IN_work := FALSE;
ELSIF tx - start >= PT1 THEN
	Q := FALSE;
	W := TRUE;
	start := tx;
END_IF;

ix := IN_work;
END_FUNCTION_BLOCK

FUNCTION_BLOCK TP_X
VAR_INPUT
	IN : BOOL;
	PT : TIME;
END_VAR
VAR_OUTPUT
	Q : BOOL;
	ET : TIME;
END_VAR
VAR
	edge : BOOL;
	start : TIME;
	tx: TIME;
END_VAR


(* read system_time *)
tx := DWORD_TO_TIME(T_PLC_MS());

(* rising edge trigger *)
IF IN AND NOT edge THEN
	start := tx;
	Q := PT > t#0ms;
ELSIF Q THEN
	ET := tx - start;
	IF ET >= PT THEN
		Q := FALSE;
		ET := t#0ms;
	END_IF;
END_IF;
edge := IN;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FIFO_16
VAR_INPUT
	Din : DWORD;
	E : BOOL := TRUE;
	RD : BOOL;
	WD : BOOL;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Dout : DWORD;
	EMPTY : BOOL := TRUE;
	FULL : BOOL;
END_VAR
VAR
	fifo : ARRAY[0..n] OF DWORD;
	pr : INT;
	pw : INT;
END_VAR
VAR CONSTANT
	n : INT := 16;	(* changing this value will chage the number of stored elements in the fifo *)
END_VAR

IF RST THEN
	pw := pr;
	FULL := FALSE;
	EMPTY := TRUE;
	Dout := 0;
ELSIF E THEN
	IF NOT EMPTY AND RD THEN
		Dout := fifo[pr];
		pr := INC1(pr,n);
		EMPTY := pr = pw;
		FULL := FALSE;
	END_IF;
	IF NOT FULL AND WD THEN
		fifo[pw] := Din;
		pw := INC1(pw,n);
		FULL := pw = pr;
		EMPTY := FALSE;
	END_IF;
END_IF;

END_FUNCTION_BLOCK

FUNCTION_BLOCK FIFO_32
VAR_INPUT
	Din : DWORD;
	E : BOOL := TRUE;
	RD : BOOL;
	WD : BOOL;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Dout : DWORD;
	EMPTY : BOOL := TRUE;
	FULL : BOOL;
END_VAR
VAR
	fifo : ARRAY[0..n] OF DWORD;
	pr : INT;
	pw : INT;
END_VAR
VAR CONSTANT
	n : INT := 32;	(* changing this value will chage the number of stored elements in the fifo *)
END_VAR

IF RST THEN
	pw := pr;
	FULL := FALSE;
	EMPTY := TRUE;
	Dout := 0;
ELSIF E THEN
	IF NOT EMPTY AND RD THEN
		Dout := fifo[pr];
		pr := INC1(pr,n);
		EMPTY := pr = pw;
		FULL := FALSE;
	END_IF;
	IF NOT FULL AND WD THEN
		fifo[pw] := Din;
		pw := INC1(pw,n);
		FULL := pw = pr;
		EMPTY := FALSE;
	END_IF;
END_IF;


END_FUNCTION_BLOCK

FUNCTION_BLOCK STACK_16
VAR_INPUT
	Din : DWORD;
	E : BOOL := TRUE;
	RD : BOOL;
	WD : BOOL;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Dout : DWORD;
	EMPTY : BOOL := TRUE;
	FULL : BOOL;
END_VAR
VAR
	stack : ARRAY[0..n] OF DWORD;
	pt : INT;
END_VAR
VAR CONSTANT
	n : INT := 15;	(* changing this value will chage the number of stored elements in the fifo *)
END_VAR

IF RST THEN
	(* asynchronous reset for the fifo *)
	pt := 0;
	EMPTY := TRUE;
	FULL := FALSE;
	Dout := 0;
ELSIF E THEN
	IF NOT EMPTY AND RD THEN
		(* read one element *)
		pt := pt - 1;
		Dout := stack[pt];
		EMPTY := pt = 0;
		FULL := FALSE;
	END_IF;
	IF NOT FULL AND WD THEN
		(* write one element *)
		stack[pt] := Din;
		pt := pt + 1;
		FULL := pt > n;
		EMPTY := FALSE;
	END_IF;
END_IF;


END_FUNCTION_BLOCK

FUNCTION_BLOCK STACK_32
VAR_INPUT
	Din : DWORD;
	E : BOOL := TRUE;
	RD : BOOL;
	WD : BOOL;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	Dout : DWORD;
	EMPTY : BOOL := TRUE;
	FULL : BOOL;
END_VAR
VAR
	stack : ARRAY[0..n] OF DWORD;
	pt : INT;
END_VAR
VAR CONSTANT
	n : INT := 31;	(* changing this value will chage the number of stored elements in the fifo *)
END_VAR

IF RST THEN
	(* asynchronous reset for the fifo *)
	pt := 0;
	EMPTY := TRUE;
	FULL := FALSE;
	Dout := 0;
ELSIF E THEN
	IF NOT EMPTY AND RD THEN
		(* read one element *)
		pt := pt - 1;
		Dout := stack[pt];
		EMPTY := pt = 0;
		FULL := FALSE;
	END_IF;
	IF NOT FULL AND WD THEN
		(* write one element *)
		stack[pt] := Din;
		pt := pt + 1;
		FULL := pt > n;
		EMPTY := FALSE;
	END_IF;
END_IF;

END_FUNCTION_BLOCK

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION CRC_GEN : DWORD
//
// VAR_INPUT
// 	PT : POINTER TO ARRAY[0..32000] OF BYTE;
// 	SIZE : INT;
// 	PL : INT;
// 	PN : DWORD;
// 	INIT : DWORD;
// 	REV_IN : BOOL;
// 	REV_OUT : BOOL;
// 	XOR_OUT : DWORD;
// END_VAR
// VAR
// 	pos : INT;
// 	shift : INT;
// 	dx: BYTE;
// 	bits: INT;
// END_VAR
//
//
// (* align polygon *)
// shift := 32 - PL;
// PN := SHL(PN, shift);
//
// (* load first 4 bytes into register minimum message size is 4 bytes
// 	for smaller messages fill with 0#s at the beginning*)
// FOR pos := 0 TO 3 DO
// 	IF REV_IN THEN CRC_GEN := SHL(CRC_GEN, 8) OR REVERSE(PT^[pos]); ELSE CRC_GEN := SHL(CRC_GEN, 8) OR PT^[pos]; END_IF;
// END_FOR;
// pos := 4;
//
// (* xor with init value *)
// CRC_GEN := CRC_GEN XOR SHL(init, shift);
//
// (* calculate CRC for each byte *)
// WHILE pos < size DO
// 	IF REV_IN THEN DX := REVERSE(PT^[pos]); ELSE DX := PT^[pos]; END_IF;
// 	pos := pos + 1;
// 	(* crc calculation for one byte *)
// 	FOR bits := 0 TO 7 DO
// 		IF CRC_GEN.31 THEN
// 			CRC_GEN := (SHL(CRC_GEN, 1) OR BOOL_TO_DWORD(DX.7)) XOR PN;
// 		ELSE
// 			CRC_GEN := SHL(CRC_GEN, 1) OR BOOL_TO_DWORD(DX.7);
// 		END_IF;
// 		dx := SHL(dx, 1);
// 	END_FOR;
// END_WHILE;
//
// (* all bytes are processed, need to finish the registers 32 bits *)
// FOR bits := 0 TO 31 DO
// 	IF CRC_GEN.31 THEN
// 		CRC_GEN := (SHL(CRC_GEN, 1) ) XOR PN;
// 	ELSE
// 		CRC_GEN := SHL(CRC_GEN, 1);
// 	END_IF;
// END_FOR;
//
// (* final XOR *)
// CRC_GEN := SHR(CRC_GEN, shift) XOR XOR_OUT;
//
// (* reverse the crc_out put if necessary *)
// IF REV_OUT THEN CRC_GEN := REFLECT(CRC_GEN, PL); END_IF;
//
// (* suggested optimization by deninis klopke
// die betreffs genannte Funktion kann folgenderma�en optimiert werden:
// In Zeilen 33 und 35 des Implementierungsteils kann jeweils der Teil �OR BOOL_TO_DWORD(DX.7)� entfallen, da nach 8maligem Linksshift von �DX� in Zeile 26 der Wert von DX = BYTE#0 ist. Eine weitere Veroderung mit diesem Wert hat keine Auswirkungen auf das Ergebnis mehr.
// *)
//
// (* typical crc polynoms
//
// CRC-4-ITU 			x4 + x + 1 					(ITU G.704, p. 12) 	0x3 or 0xC (0x9)
// CRC-5-ITU 			x5 + x4 + x2 + 1 			(ITU G.704, p. 9) 	0x15 or 0x15 (0x0B) Bluetooth
// CRC-5-USB 			x5 + x2 + 1 				(use: USB token packets) 	0x05 or 0x14 (0x9)
// CRC-6-ITU 			x6 + x + 1 					(ITU G.704, p. 3) 	0x03 or 0x30 (0x21)
// CRC-7 				x7 + x3 + 1 				(use: telecom systems, MMC) 	0x09 or 0x48 (0x11)
// CRC-8-ATM 			x8 + x2 + x + 1 				(use: ATM HEC) 	0x07 or 0xE0 (0xC1)
// CRC-8-CCITT 		x8 + x7 + x3 + x2 + 1 		(use: 1-Wire bus) 	0x8D or 0xB1 (0x63)
// CRC-8-Dallas/Maxim 	x8 + x5 + x4 + 1 			(use: 1-Wire bus) 	0x31 or 0x8C (0x19)
// CRC-8 				x8 + x7 + x6 + x4 + x2 + 1 	0xD5 or 0xAB (0x57)
// CRC-8-SAE J1850 	x8 + x4 + x3 + x2 + 1 		0x1D or 0xB8
// CRC-10 				x10 + x9 + x5 + x4 + x + 1 	0x233 or 0x331 (0x263)
// CRC-12 				x12 + x11 + x3 + x2 + x + 1 (use: telecom systems) 	0x80F or 0xF01 (0xE03)
// CRC-15-CAN 			x15 + x14 + x10 + x8 + x7 + x4 + x3 + 1 	0x4599 or 0x4CD1 (0x19A3)
// CRC-16-Fletcher 	Not a CRC; see Fletcher's checksum 	Used in Adler-32 A & B CRCs
// CRC-16-CCITT 	x16 + x12 + x5 + 1 (XMODEM,X.25, V.41, Bluetooth, PPP, IrDA; known as "CRC-CCITT") 	0x1021 or 0x8408 (0x0811)
// CRC-16-IBM 	x16 + x15 + x2 + 1 (USB, many others; also known as "CRC-16") 	0x8005 or 0xA001 (0x4003)
// CRC-24-Radix-64 	x24 + x23 + x18 + x17 + x14 + x11 + x10 + x7 + x6 + x5 + x4 + x3 + x + 1 	0x864CFB or 0xDF3261 (0xBE64C3)
// CRC-32-Adler 	Not a CRC; see Adler-32 	See Adler-32
// CRC-32-MPEG2 	x32 + x26 + x23 + x22 + x16 + x12 + x11 + x10 + x8 + x7 + x5 + x4 + x2 + x + 1 	0x04C11DB7 or 0xEDB88320 (0xDB710641) Also used in IEEE 802.3
// CRC-32-IEEE 802.3 	x32 + x26 + x23 + x22 + x16 + x12 + x11 + x10 + x8 + x7 + x5 + x4 + x2 + x + 1 (V.42) 	0x04C11DB7 or 0xEDB88320 (0xDB710641)
// CRC-32C (Castagnoli) 	x32 + x28 + x27 + x26 + x25 + x23 + x22 + x20 + x19 + x18 + x14 + x13 + x11 + x10 + x9 + x8 + x6 + 1 	0x1EDC6F41 or 0x82F63B78 (0x05EC76F1)
// CRC-64-ISO 	x64 + x4 + x3 + x + 1 (use: ISO 3309) 	0x000000000000001B or 0xD800000000000000 (0xB000000000000001)
// CRC-64-ECMA-182 	x64 + x62 + x57 + x55 + x54 + x53 + x52 + x47 + x46 + x45 + x40 + x39 + x38 + x37 + x35 + x33 + x32 + x31 + x29 + x27 + x24 + x23 + x22 + x21 + x19 + x17 + x13 + x12 + x10 + x9 + x7 + x4 + x + 1
// (as described in ECMA-182 p.63) 	0x42F0E1EBA9EA3693 or 0xC96C5795D7870F42 (0x92D8AF2BAF0E1E85)
// *)
// END_FUNCTION

FUNCTION_BLOCK MATRIX
VAR_INPUT
	x1, x2, x3, x4, X5 : BOOL;
END_VAR
VAR_INPUT CONSTANT
	Release : BOOL;
END_VAR
VAR_OUTPUT
	code : BYTE;
	TP : BOOL;
	y1: BOOL := TRUE;
	y2, y3, y4 : BOOL;
END_VAR
VAR
	line : BYTE;
	X : ARRAY[0..3] OF BYTE; (* scan line inputs *)
	L : ARRAY[0..3] OF BYTE; (* scan line status *)
	i: INT;
	temp: BYTE;
END_VAR

TP := FALSE;
CODE := 0;
(* read scan lines *)
X[line].0 := X1;
X[line].1 := X2;
X[line].2 := X3;
X[line].3 := X4;
X[line].4 := X5;

(* compare for change *)
FOR i := 0 TO 3 DO
	IF X[i] <> L[i] THEN
		(* scan line information has changed code need to be found and generated *)
		temp := x[i] XOR L[i];
		IF temp.0 THEN
			CODE := 1;
			code.7 := X[i].0;
			L[i].0 := X[i].0;
		ELSIF temp.1 THEN
			code := 2;
			code.7 := X[i].1;
			L[i].1 := X[i].1;
		ELSIF temp.2 THEN
			code := 3;
			code.7 := X[i].2;
			L[i].2 := X[i].2;
		ELSIF temp.3 THEN
			CODE := 4;
			CODE.7 := X[i].3;
			L[i].3 := X[i].3;
		ELSIF temp.4 THEN
			CODE := 5;
			CODE.7 := X[i].4;
			L[i].4 := X[i].4;
		END_IF;
		TP := TRUE;
		CODE.4 := Line.0;
		CODE.5 := Line.1;
		CODE.6 := Line.2;

		(* check if release codes need to be killed *)
		IF NOT release AND CODE < 127 THEN
			CODE := 0;
			TP := FALSE;
		END_IF;
		EXIT;
	END_IF;
END_FOR;

(* increment scan line every cycle *)
line := ULINT_TO_BYTE((BYTE_TO_ULINT(line) + 1) MOD 256) AND 2#0000_0011;
temp := SHL(BYTE#1,line);
Y1 := temp.0;
Y2 := temp.1;
Y3 := temp.2;
Y4 := temp.3;

END_FUNCTION_BLOCK

// Commented out for beedance: it uses CODE, which is commented out.
// FUNCTION_BLOCK PIN_CODE
// VAR_INPUT
// 	CB : BYTE;
// 	E : BOOL;
// END_VAR
// VAR_INPUT CONSTANT
// 	PIN : STRING(8);
// END_VAR
// VAR_OUTPUT
// 	TP : BOOL;
// END_VAR
// VAR
// 	POS: INT := 1;
// END_VAR
//
// tp := FALSE;
// IF e THEN
// 	IF CB = CODE(pin, pos) THEN
// 		pos := pos + 1;
// 		IF pos > LEN(PIN) THEN
// 			(* proper code is detected *)
// 			TP := TRUE;
// 			pos := 1;
// 		END_IF;
// 	ELSE
// 		pos := 1;
// 	END_IF;
// END_IF;
//
// END_FUNCTION_BLOCK

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION _ARRAY_ABS : BOOL
// VAR_INPUT
// 	pt : POINTER TO ARRAY[0..32000] OF REAL;
// 	size : UINT;
// END_VAR
// VAR
// 	i: UINT;
// 	stop: UINT;
// END_VAR
//
// stop := SHR(size,2)-1;
// FOR i := 0 TO stop DO
// 	PT^[i] := ABS(PT^[i]);
// END_FOR;
// _ARRAY_ABS := TRUE;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION _ARRAY_ADD : BOOL
// VAR_INPUT
// 	pt : POINTER TO ARRAY[0..32000] OF REAL;
// 	size : UINT;
// 	X : REAL;
// END_VAR
// VAR
// 	i: UINT;
// 	stop: UINT;
// END_VAR
//
// stop := SHR(size,2)-1;
// FOR i := 0 TO stop DO
// 	PT^[i] := PT^[i] + X;
// END_FOR;
// _ARRAY_ADD := TRUE;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION _ARRAY_INIT : BOOL
// VAR_INPUT
// 	pt : POINTER TO ARRAY[0..32000] OF REAL;
// 	size : UINT;
// 	init : REAL;
// END_VAR
// VAR
// 	i: UINT;
// 	stop: UINT;
// END_VAR
//
// stop := SHR(size,2)-1;
// FOR i := 0 TO stop DO
// 	pt^[i] := init;
// END_FOR;
//
// _array_init := TRUE;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION _ARRAY_MEDIAN : REAL
// VAR_INPUT
// 	pt : POINTER TO ARRAY[0..32000] OF REAL;
// 	size : UINT;
// END_VAR
// VAR
// 	i: UINT;
// 	stop: UINT;
// END_VAR
//
// _ARRAY_SORT(pt,size);
// stop := SHR(size,2)-1;
// IF EVEN(UINT_TO_INT(stop)) THEN
// 	_ARRAY_MEDIAN := pt^[SHR(stop,1)];
// ELSE
// 	i := SHR(stop,1);
// 	_ARRAY_MEDIAN := (pt^[i] + pt^[i+1]) * 0.5;
// END_IF;
//
// (* old code
// stop := (size - SIZEOF(pt)) / SIZEOF(pt);
// FOR i := 0 TO stop - 1 DO
// 	FOR m := i + 1 TO stop DO
// 		IF pt^[i] > pt^[m] THEN
// 			temp := pt^[i];
// 			pt^[i] := pt^[m];
// 			pt^[m] := temp;
// 		END_IF;
// 	END_FOR;
// END_FOR;
// IF even(stop) THEN
// 	_array_median := pt^[stop/2];
// ELSE
// 	i := stop/2;
// 	_array_median := (pt^[i] + pt^[i+1])/2;
// END_IF;
// *)
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION _ARRAY_MUL : BOOL
// VAR_INPUT
// 	pt : POINTER TO ARRAY[0..32000] OF REAL;
// 	size : UINT;
// 	X : REAL;
// END_VAR
// VAR
// 	i: UINT;
// 	stop: UINT;
// END_VAR
//
// stop := SHR(size,2)-1;
// FOR i := 0 TO stop DO
// 	PT^[i] := PT^[i] * X;
// END_FOR;
// _ARRAY_MUL := TRUE;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION _ARRAY_SHUFFLE : BOOL
// VAR_INPUT
// 	pt : POINTER TO ARRAY[0..32000] OF REAL;
// 	size : UINT;
// END_VAR
// VAR
// 	temp : REAL;
// 	pos  : INT;
// 	i: INT;
// 	stop: INT;
// END_VAR
//
// stop := UINT_TO_INT(SHR(size,2)-1);
// FOR i := 0 TO stop DO
//         pos := RDM2(i+pos,0,stop);
//         (* swap elements *)
//         temp := pt^[i];
//         pt^[i] := pt^[pos];
//         pt^[pos] := temp;
// END_FOR;
//
// _ARRAY_SHUFFLE := TRUE;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION _ARRAY_SORT : BOOL
// VAR_INPUT
// 	PT : POINTER TO ARRAY[1..32000] OF REAL;
// 	SIZE : UINT;
// END_VAR
// VAR
// 	stack_count: UINT;				(* Laufvariable Stack*)
// 	stack: ARRAY[1..32] OF UINT;	(* Stackgr��e~ 1,6*Log(n)/log(2) *)
// 										(* Stackgr��e 32 => ~1 Mio Elemente *)
// 	links: UINT;						(* Anfangselement des Arrays *)
// 	rechts: UINT;						(* Endelement des Arrays *)
// 	pivot: REAL;						(* tempor�rer Schwellwert f�r Tauschbedingung *)
// 	i: UINT;							(* Laufvariable1 *)
// 	j: UINT;							(* Laufvariable2 *)
// 	ende_innen: BOOL;				(* Ende innere Schleife *)
// 	ende_aussen: BOOL;				(* Ende �u�ere Schleife *)
// 	tmp: REAL;						(* Hilfsvariable zum Tauschen von Werten *)
// END_VAR
//
// (* Startwerte zur Arraygr��e *)
// links := 1;					(* Anfangselement des Arrays *)
// rechts := SHR(size,2);		(* Endelement des Arrays *)
// stack_count := 1;			(* Laufvariable Stack *)
//
// WHILE NOT ende_aussen DO						(* �u�ere Schleife *)
// 	IF links < rechts THEN
// 		pivot := PT^[SHR(rechts+links,1)];			(* Wert des mittleren Elements als Pivot-Wert *)
// 		i := links -1;
// 		j := rechts +1;
//
// 		(* innere Schleife, teile Feld *)
// 		ende_innen := FALSE;
// 		REPEAT
//
// 			(* steigende Reihenfolge *)
// 			REPEAT	i := i+1;	UNTIL (PT^[i] >= pivot) OR NOT (i < rechts)	END_REPEAT;
// 			REPEAT	j := j-1;	UNTIL (PT^[j] <= pivot) OR NOT (j > links)	END_REPEAT;
//
//
// 			(*sinkende Reihenfolge *)
// (*			REPEAT	i := i+1;	UNTIL (PT^[i] <= pivot) OR NOT (i < rechts)	END_REPEAT	*)
// (*			REPEAT	j := j-1;	UNTIL (PT^[j] >= pivot) OR NOT (j > links)	END_REPEAT	*)
//
// 			IF i >= j THEN
// 				ende_innen := TRUE;
// 			ELSE
// 				   tmp := PT^[j];
// 				PT^[j] := PT^[i];
// 				PT^[i] := tmp;
// 			END_IF;
//
// 		UNTIL ende_innen END_REPEAT;
//
// 		(* Push stack *)
// 		stack[stack_count] := rechts;		(* rechten Teil sp�ter sortieren *)
// 		IF Stack_count < 32 THEN
// 			stack_count := stack_count +1;
// 		ELSE
// 			ende_aussen := TRUE;
// 			_ARRAY_SORT := FALSE;					(* Fehler: Stack zu klein *)
// 		END_IF;
//
// 		(* linken Teil sortiern *)
// 		rechts := MAX(links, i-1);
//
// 	ELSE
//
// 		IF stack_count = 1 THEN
// 			ende_aussen := TRUE;
// 		ELSE
//
// 			links := rechts+1;
//
// 			(* pop stack *)
// 			stack_count := stack_count -1;		(* jetzt rechten Teil sortierne *)
// 			rechts:= stack[stack_count];
// 		END_IF;
//
// 	END_IF;
//
// END_WHILE;
//
// _ARRAY_SORT := TRUE;				(* Sortierung beendet *)
//
//
// (* algorithm used before rev 2.0
//
// size := SHR(size,2);
// size2 := UINT_TO_INT(SHR(size,1));
// FOR i := SIZE2  TO 1 BY -1 DO
// 	j:=i;
//     WHILE j <= SIZE2 DO
//      	stop := j * 2 + 1;
//         IF stop > UINT_TO_INT(SIZE) THEN stop := stop - 1;
//         ELSIF pt^[stop-1] > pt^[stop] THEN stop := stop - 1;
// 		END_IF;
//         IF pt^[j] < pt^[stop] THEN
//          	temp := pt^[j];
//             pt^[j] := pt^[stop];
//             pt^[stop] := temp;
//         END_IF;
//         j := stop;
//     END_WHILE;
// END_FOR;
//
// *)
//
// (* heapsort
// FOR heapsize := UINT_TO_INT(size) TO 1 BY -1 DO
// 	popmax := pt^[1];
// 	pt^[1] := pt^[heapsize];
//    	i := 1;
// 	hs2 := SHR(heapsize,1);
// 	WHILE i <= hs2 DO
//        	stop := i * 2 + 1;
//         	IF stop > heapsize THEN stop := stop - 1;
//         	ELSIF pt^[stop-1] > pt^[stop] THEN stop := stop - 1;
// 	 	END_IF;
//         	IF pt^[i] < pt^[stop] THEN
//             		temp := pt^[i];
//             		pt^[i] := pt^[stop];
//             		pt^[stop] := temp;
//         	END_IF;
//         	i := stop;
//     	END_WHILE;
// 	pt^[heapsize] := popmax;
// END_FOR;
//
// _ARRAY_SORT2 := TRUE;
// *)
//
// (* old algorithm bubble sort used before rev 1.2
// stop := (size - SIZEOF(pt)) / SIZEOF(pt);
// FOR i := 0 TO stop - 1 DO
// 	FOR m := i + 1 TO stop DO
// 		IF pt^[i] > pt^[m] THEN
// 			temp := pt^[i];
// 			pt^[i] := pt^[m];
// 			pt^[m] := temp;
// 		END_IF;
// 	END_FOR;
// END_FOR;
// _array_sort := TRUE;
//
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION ARRAY_AVG : REAL
// VAR_INPUT
// 	pt : POINTER TO ARRAY[0..32000] OF REAL;
// 	size : UINT;
// END_VAR
// VAR
// 	i: UINT;
// 	stop: UINT;
// END_VAR
//
// stop := SHR(size,2)-1;
// ARRAY_AVG := pt^[0];
// FOR i := 1 TO stop DO
// 	ARRAY_AVG := ARRAY_AVG + pt^[i];
// END_FOR;
// ARRAY_AVG := ARRAY_AVG / UINT_TO_REAL(stop + 1);
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION ARRAY_GAV : REAL
// VAR_INPUT
// 	pt : POINTER TO ARRAY[0..32000] OF REAL;
// 	size : UINT;
// END_VAR
// VAR
// 	i: UINT;
// 	stop: UINT;
// END_VAR
//
// stop := SHR(size,2)-1;
// ARRAY_GAV := 1.0;
// FOR i := 0 TO stop DO
// 	IF pt^[i] > 0.0 THEN
// 		ARRAY_GAV := ARRAY_GAV * pt^[i];
// 	ELSE
// 		ARRAY_GAV := 0.0;
// 		RETURN;
// 	END_IF;
// END_FOR;
// ARRAY_GAV := SQRTN(ARRAY_GAV,UINT_TO_INT(stop)+1);
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION ARRAY_HAV : REAL
// VAR_INPUT
// 	pt : POINTER TO ARRAY[0..32000] OF REAL;
// 	size : UINT;
// END_VAR
// VAR
// 	i: UINT;
// 	stop: UINT;
// END_VAR
//
// stop := SHR(size,2)-1;
// FOR i := 0 TO stop DO
// 	IF pt^[i] <> 0.0 THEN
// 		ARRAY_HAV := ARRAY_HAV + 1.0 / pt^[i];
// 	ELSE
// 		ARRAY_HAV := 0.0;
// 		RETURN;
// 	END_IF;
// END_FOR;
// ARRAY_HAV := UINT_TO_REAL(stop + 1) / ARRAY_HAV;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION ARRAY_MAX : REAL
// VAR_INPUT
// 	pt : POINTER TO ARRAY[0..32000] OF REAL;
// 	size : UINT;
// END_VAR
// VAR
// 	i: UINT;
// 	stop: UINT;
// END_VAR
//
// stop := (size -SIZEOF(pt)) / SIZEOF(pt);
// array_max := pt^[0];
// FOR i := 1 TO stop DO
// 	IF pt^[i] > array_max THEN array_max := pt^[i]; END_IF;
// END_FOR;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION ARRAY_MIN : REAL
// VAR_INPUT
// 	pt : POINTER TO ARRAY[0..32000] OF REAL;
// 	size : UINT;
// END_VAR
// VAR
// 	i: UINT;
// 	stop: UINT;
// END_VAR
//
// stop := (size - SIZEOF(pt)) / SIZEOF(pt);
// array_min := pt^[0];
// FOR i := 1 TO stop DO
// 	IF pt^[i] < array_min THEN array_min := pt^[i]; END_IF;
// END_FOR;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION ARRAY_SDV : REAL
// VAR_INPUT
// 	pt : POINTER TO ARRAY[0..32000] OF REAL;
// 	size : UINT;
// END_VAR
//
// (* standard deviation is simply the square root of the variance *)
//
// array_sdv := SQRT(array_var(pt, size));
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION ARRAY_SPR : REAL
// VAR_INPUT
// 	pt : POINTER TO ARRAY[0..32000] OF REAL;
// 	size : UINT;
// END_VAR
// VAR
// 	i: UINT;
// 	stop: UINT;
// 	array_max : REAL;
// 	array_min : REAL;
// END_VAR
//
// stop := (size -SIZEOF(pt)) / SIZEOF(pt);
// array_min := pt^[0];
// array_max := pt^[0];
// FOR i := 1 TO stop DO
// 	IF pt^[i] > array_max THEN array_max := pt^[i];
// 	ELSIF pt^[i] < array_min THEN array_min := pt^[i];
// 	END_IF;
// END_FOR;
// array_spr := array_max - array_min;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION ARRAY_SUM : REAL
// VAR_INPUT
// 	pt : POINTER TO ARRAY[0..32000] OF REAL;
// 	size : UINT;
// END_VAR
// VAR
// 	i: UINT;
// 	stop: UINT;
// END_VAR
//
// stop := (size -SIZEOF(pt)) / SIZEOF(pt);
// array_sum := pt^[0];
// FOR i := 1 TO stop DO
// 	array_sum := array_sum + pt^[i];
// END_FOR;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION ARRAY_TREND : REAL
// VAR_INPUT
// 	pt : POINTER TO ARRAY[0..32000] OF REAL;
// 	size : UINT;
// END_VAR
// VAR
// 	i: UINT;
// 	stop: UINT;
// 	x: REAL;
// 	stop2: UINT;
// END_VAR
//
// stop := SHR(size,2)-1;
// stop2 := SHR(stop,1);
// FOR i := 0 TO stop2 DO x := x - pt^[i]; END_FOR;
// IF EVEN(UINT_TO_INT(stop)) THEN
// 	FOR i := stop2 TO stop DO X := X + pt^[i]; END_FOR;
// ELSE
// 	FOR i := stop2 + 1 TO stop DO X := X + pt^[i]; END_FOR;
// END_IF;
// ARRAY_TREND := x / UINT_TO_REAL(stop2 + 1);
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION ARRAY_VAR : REAL
// VAR_INPUT
// 	pt : POINTER TO ARRAY[0..32000] OF REAL;
// 	size : UINT;
// END_VAR
// VAR
// 	avg : REAL;
// 	i: UINT;
// 	stop: UINT;
// END_VAR
//
// (* at first we calualte the arithmetic average of the array *)
//
// stop := SHR(size,2)-1;
// avg := pt^[0];
// FOR i := 1 TO stop DO
// 	avg := avg + pt^[i];
// END_FOR;
// avg := avg / UINT_TO_REAL(stop + 1);
//
// (* in a second run we calculate the variance of the array *)
//
// array_var := (pt^[0] - avg) * (pt^[0] - avg);
// FOR i := 1 TO stop DO
// 	array_var := array_var + (pt^[i] - avg) * (pt^[i] - avg);
// END_FOR;
// ARRAY_VAR := ARRAY_VAR / UINT_TO_REAL(stop);
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION IS_SORTED : BOOL
// VAR_INPUT
// 	pt : POINTER TO ARRAY[0..32000] OF REAL;
// 	size : UINT;
// END_VAR
// VAR
//
// 	stop: INT;
// 	i: INT;
// END_VAR
//
// stop := UINT_TO_INT(SHR(size,2)-2);
// FOR i := 0 TO stop DO
// 	IF pt^[i] > pt^[i+1] THEN
// 		IS_SORTED := FALSE;
// 		RETURN;
// 	END_IF;
// END_FOR;
//
// IS_SORTED := TRUE;
//
// END_FUNCTION

FUNCTION CABS : REAL
VAR_INPUT
	X : complex;
END_VAR

CABS := HYPOT(x.re, x.im);
END_FUNCTION

FUNCTION CACOS : complex
VAR_INPUT
	X : complex;
END_VAR
VAR
	Y : complex;
END_VAR

y := CACOSH(x);
CACOS.re := y.im;
CACOS.im := -y.re;
END_FUNCTION

FUNCTION CACOSH : complex
VAR_INPUT
	X : complex;
END_VAR
VAR
	Y : complex;
END_VAR

y.re := (X.re - X.im)*(X.re + X.im) - 1.0;
y.im := 2.0 * X.re * X.im;
y    := CSQRT(y);
y.re := y.re + x.re;
y.im := y.im + x.im;
CACOSH  := CLOG(y);
END_FUNCTION

FUNCTION CADD : complex
VAR_INPUT
	X, Y : complex;
END_VAR

CADD.re := x.re + y.re;
CADD.im := x.im + y.im;
END_FUNCTION

FUNCTION CARG : REAL
VAR_INPUT
	X : complex;
END_VAR

CARG := ATAN2(X.im, X.re);
END_FUNCTION

FUNCTION CASIN : complex
VAR_INPUT
	X : complex;
END_VAR
VAR
	Y : complex;
END_VAR

y.re := -x.im;
y.im := x.re;
y := CASINH(y);
CASIN.re := y.im;
CASIN.im := -y.re;
END_FUNCTION

FUNCTION CASINH : complex
VAR_INPUT
	X : complex;
END_VAR
VAR
	Y : complex;
END_VAR

y.re := (X.re - X.im)*(X.re + X.im) + 1.0;
y.im := 2.0 * X.re * X.im;
y    := CSQRT(y);
y.re := y.re + x.re;
y.im := y.im + x.im;
CASINH  := CLOG(y);
END_FUNCTION

FUNCTION CATAN : complex
VAR_INPUT
	X : complex;
END_VAR
VAR
	r2: REAL;
	num, den : REAL;
END_VAR

r2 := x.re * x.re;
den := 1.0 - r2 - x.im * x.im;
CATAN.re := 0.5 * ATAN(2.0 * x.re / den);
num := x.im + 1.0;
num := r2 + num * num;
den := x.im - 1.0;
den := r2 + den * den;
CATAN.im := 0.25 * (LN(num)-LN(den));
END_FUNCTION

FUNCTION CATANH : complex
VAR_INPUT
	X : complex;
END_VAR
VAR
	i2: REAL;
	num, den : REAL;
END_VAR

i2 := x.im * x.im;
num := 1.0 + x.re;
num := i2 + num * num;
den := 1.0 - x.re;
den := i2 + den * den;
CATANH.re := 0.25 * (LN(num) - LN(den));
den := 1 - x.re * x.re - i2;
CATANH.im := 0.5 * ATAN(2.0 * x.im / den);
END_FUNCTION

FUNCTION CCON : complex
VAR_INPUT
	X : complex;
END_VAR

CCON.re := x.re;
CCON.im := -x.im;
END_FUNCTION

FUNCTION CCOS : complex
VAR_INPUT
	X : complex;
END_VAR

CCOS := CCOSH(CSET(-X.im, X.re));
END_FUNCTION

FUNCTION CCOSH : complex
VAR_INPUT
	X : complex;
END_VAR

CCOSH.re := cosH(x.re) * COS(x.im);
CCOSH.im := sinH(x.re) * SIN(x.im);
END_FUNCTION

FUNCTION CDIV : complex
VAR_INPUT
	X, Y : complex;
END_VAR
VAR
	Temp : REAL;
END_VAR

temp := Y.re * Y.re + Y.im * Y.im;
CDIV.re := (X.re * Y.re + X.im * Y.im) / temp;
CDIV.im := (X.im * Y.re - X.re * Y.im) / temp;
END_FUNCTION

FUNCTION CEXP : complex
VAR_INPUT
	X : complex;
END_VAR
VAR
	Temp : REAL;
END_VAR

temp := EXP(X.re);
CEXP.re := temp * COS(X.im);
CEXP.im := temp * SIN(X.im);
END_FUNCTION

FUNCTION CINV : complex
VAR_INPUT
	X : complex;
END_VAR
VAR
	temp : REAL;
END_VAR

temp := X.re * X.re + X.im * X.im;
CINV.re := X.re / temp;
CINV.im := -X.im / temp;
END_FUNCTION

FUNCTION CLOG : complex
VAR_INPUT
	X : complex;
END_VAR

CLOG.re := LN(HYPOT(X.re, X.im));
CLOG.im := ATAN2(X.im, X.re);
END_FUNCTION

FUNCTION CMUL : complex
VAR_INPUT
	X, Y : complex;
END_VAR

CMUL.re := X.re * Y.re - X.im * Y.im;
CMUL.im := X.re * Y.im + X.im * Y.re;
END_FUNCTION

FUNCTION CPOL : complex
VAR_INPUT
	L : REAL;
	A : REAL;
END_VAR

CPOL.re := L * COS(A);
CPOL.im := L * SIN(A);
END_FUNCTION

FUNCTION CPOW : complex
VAR_INPUT
	X : complex;
	Y : Complex;
END_VAR

CPOW := CEXP(CMUL(Y,CLOG(X)));
END_FUNCTION

FUNCTION CSET : complex
VAR_INPUT
	RE : REAL;
	IM : REAL;
END_VAR

CSET.re := RE;
CSET.im := IM;
END_FUNCTION

FUNCTION CSIN : complex
VAR_INPUT
	X : complex;
END_VAR

CSIN.re := cosH(X.im) * SIN(X.re);
CSIN.im := sinH(X.im) * COS(X.re);
END_FUNCTION

FUNCTION CSINH : complex
VAR_INPUT
	X : complex;
END_VAR

CSINH.re := sinH(X.re) * COS(X.im);
CSINH.im := cosH(X.re) * SIN(X.im);
END_FUNCTION

FUNCTION CSQRT : complex
VAR_INPUT
	X : complex;
END_VAR
VAR
	temp : REAL;
END_VAR

temp := HYPOT(x.re, x.im);
CSQRT.re :=  SQRT(0.5 * (temp + x.re));
CSQRT.im :=  sgn(x.im) * SQRT(0.5 * (temp - x.re));
END_FUNCTION

FUNCTION CSUB : complex
VAR_INPUT
	X, Y : complex;
END_VAR

CSUB.re := X.re - Y.re;
CSUB.im := X.im - Y.im;
END_FUNCTION

FUNCTION CTAN : complex
VAR_INPUT
	X : complex;
END_VAR
VAR
	temp : REAL;
	xi2: REAL;
	xr2: REAL;
END_VAR

xi2 := 2.0 * x.im;
xr2 := 2.0 * x.re;
temp := 1.0 / (COS(xr2) + COSH( xi2));
CTAN.re := temp * SIN(xr2);
CTAN.im := temp * SINH(xi2);
END_FUNCTION

FUNCTION CTANH : complex
VAR_INPUT
	X : complex;
END_VAR
VAR
	temp : REAL;
	xi2: REAL;
	xr2: REAL;
END_VAR

xi2 := 2.0 * x.im;
xr2 := 2.0 * x.re;
temp := 1.0 / (COSH(xr2) + COS(xi2));
CTANH.re := temp * SINH(xr2);
CTANH.im := temp * SIN(xi2);
END_FUNCTION

FUNCTION R2_ABS : REAL2
VAR_INPUT
	X : REAL2;
END_VAR

IF X.RX >= 0.0 THEN
	R2_ABS.RX := X.RX;
	R2_ABS.R1 := X.R1;
ELSE
	R2_ABS.RX := -X.RX;
	R2_ABS.R1 := -X.R1;
END_IF;
END_FUNCTION

FUNCTION R2_ADD : REAL2
VAR_INPUT
	X : Real2;
	Y : REAL;
END_VAR
VAR
	temp: REAL;
END_VAR

temp := X.RX;
R2_ADD.RX := Y + X.R1 + X.RX;
R2_ADD.R1 := temp - R2_ADD.RX + Y + X.R1;
END_FUNCTION

FUNCTION R2_ADD2 : REAL2
VAR_INPUT
	X : Real2;
	Y : REAL2;
END_VAR

R2_ADD2.R1 := X.R1 + Y.R1;
R2_ADD2.RX := X.RX + Y.RX;
END_FUNCTION

FUNCTION R2_MUL : REAL2
VAR_INPUT
	X : REAL2;
	Y : REAL;
END_VAR

R2_MUL.RX := X.RX * Y;
R2_MUL.R1 := X.R1 * Y;
END_FUNCTION

FUNCTION R2_SET : REAL2
VAR_INPUT
	X : REAL;
END_VAR

R2_SET.RX := X;
R2_SET.R1 := 0.0;
END_FUNCTION

FUNCTION F_LIN : REAL
VAR_INPUT
	X : REAL;
	A : REAL;
	B : REAL;
END_VAR

F_lin := A * X + B;


END_FUNCTION

FUNCTION F_LIN2 : REAL
VAR_INPUT
	X : REAL;
	X1: REAL;
	Y1 : REAL;
	X2 : REAL;
	Y2 : REAL;
END_VAR

F_LIN2 := (Y2 - Y1) / (X2 - X1) * (X - X1) + Y1;
END_FUNCTION

FUNCTION F_POLY : REAL
VAR_INPUT
	X : REAL;
	C : ARRAY[0..7] OF REAL;
END_VAR
VAR
END_VAR

F_POLY := ((((((( c[7] * x + c[6] ) * x + c[5] ) * x + c[4] ) * x + c[3] ) * x + c[2] ) * x + c[1] ) * x + c[0] ) ;

END_FUNCTION

FUNCTION F_POWER : REAL
VAR_INPUT
	a : REAL;
	x : REAL;
	n : REAL;
END_VAR

F_POWER := a * EXPT(X, N);

END_FUNCTION

FUNCTION F_QUAD : REAL
VAR_INPUT
	X : REAL;
	A : REAL;
	B : REAL;
	C : REAL;
END_VAR

F_QUAD :=  (A * X + B) * X + C;
END_FUNCTION

FUNCTION FRMP_B : BYTE
VAR_INPUT
	START : BYTE;
	DIR : BOOL;
	TD : TIME;
	TR : TIME;
END_VAR
VAR
END_VAR

IF TD < TR THEN
	FRMP_B := MIN(ULINT_TO_BYTE(DWORD_TO_ULINT(SHL(TIME_TO_DWORD(TD), 8)) / TIME_TO_ULINT(TR)), SEL(DIR, START, ULINT_TO_BYTE(255 - BYTE_TO_ULINT(START))));
	IF DIR THEN
		FRMP_B := ULINT_TO_BYTE(BYTE_TO_ULINT(START) + BYTE_TO_ULINT(FRMP_B));
	ELSE
		FRMP_B := ULINT_TO_BYTE(BYTE_TO_ULINT(START) - BYTE_TO_ULINT(FRMP_B));
	END_IF;
ELSE
	FRMP_B := SEL(DIR, 0, 255);
END_IF;
END_FUNCTION

FUNCTION_BLOCK FT_AVG
VAR_INPUT
	IN : REAL;
	E : BOOL := TRUE;
	N : INT := 32;
	RST : BOOL;
END_VAR
VAR_OUTPUT
	AVG : REAL;
END_VAR
VAR
	buff : DELAY;
	i: INT;
	init : BOOL;
END_VAR

(* limit n to a max of 32 because delay can do max 32 cycles *)
buff.N := LIMIT(0, N, 32);

IF NOT init OR rst THEN
	FOR i := 1 TO N DO
		buff(in := in);
	END_FOR;
	avg := in;
	init := TRUE;
ELSIF E THEN
	buff(in := in);
	avg := avg + (in - buff.out ) / INT_TO_REAL(N);
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FT_MIN_MAX
VAR_INPUT
	in : REAL;
	rst : BOOL;
END_VAR
VAR_OUTPUT
	mx : REAL;
	mn : REAL;
END_VAR
VAR
	init: BOOL;
END_VAR

IF rst OR NOT init THEN
	mn := in;
	mx := in;
	init := TRUE;
ELSIF in < mn THEN mn := in;
ELSIF in > mx THEN mx := in;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK FT_RMP
VAR_INPUT
	Rmp : BOOL := TRUE;
	in : REAL;
	KR : REAL;
	KF : REAL;
END_VAR
VAR_OUTPUT
	out : REAL;
	busy : BOOL;
	UD : BOOL;
END_VAR
VAR
	tx: TIME;
	last: TIME;
	init: BOOL;
END_VAR

(* read system time *)
tx := DWORD_TO_TIME(T_PLC_MS()) - last;

IF NOT init THEN
	init := TRUE;
	last := tx;
	tx := t#0s;
	out := in;
END_IF;
IF NOT rmp THEN
	out := in;
	busy := FALSE;
ELSIF out > in THEN
	(* ramp down *)
	out := out - TIME_TO_REAL(tx) * 0.001 * KF;
	out := MAX(in, out);
ELSIF out < in THEN
	(* ramp up *)
	out := out + TIME_TO_REAL(tx) * 0.001 * KR;
	out := MIN(in, out);
END_IF;

(* set busy and dir flags *)
IF out < in THEN
	busy := TRUE;
	ud := TRUE;
ELSIF out > in THEN
	busy := TRUE;
	ud := FALSE;
ELSE
	busy := FALSE;
END_IF;
last := last + tx;
END_FUNCTION_BLOCK

FUNCTION LINEAR_INT : REAL
VAR_INPUT
	X : REAL;
	XY : ARRAY[1..20,0..1] OF REAL;
	Pts : INT;
END_VAR
VAR
	i : INT;
END_VAR
VAR
	Pts_work : INT;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
Pts_work := Pts;

(*make sure n is bound within the array size *)
Pts_work := MIN(Pts_work,20);

(* calculate the linear segement interpolation *)
i := 2;
(* search for segment and calculate output
	below and above the defined segments we interpolate the last segment *)
WHILE (i < Pts_work) AND (XY[i,0] < X) DO
	i := i + 1;
END_WHILE;

(* calculate the output value on the corresponding segment coordinates *)
LINEAR_INT := ((XY[i,1] - XY[i-1,1]) * X - XY[i,1] * XY[i-1,0] + XY[i-1,1] * XY[i,0]) / (XY[i,0] - XY[i-1,0]);

END_FUNCTION

FUNCTION POLYNOM_INT : REAL
VAR_INPUT
	X : REAL;
	XY : ARRAY[1..5,0..1] OF REAL;
	Pts : INT;
END_VAR
VAR
	I, J : INT;
	stop: INT;
END_VAR
VAR
	XY_work : ARRAY [1 .. 5, 0 .. 1] OF REAL;
	Pts_work : INT;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
XY_work := XY;
Pts_work := Pts;

(*make sure n is bound within the array size *)
Pts_work := MIN(Pts_work, 5);

(* this part is only to calculate the polynom parameters, which are then stores in the Y array
	the array values, it is not needed during runtime, unless the parameters will change during runtime
	the remaining code without this setup code can be used within a function to calculate specific functions
	the content of the arrays is then used as constant values within the function *)

FOR i := 1 TO Pts_work DO
	stop := i + 1;
   	FOR j := Pts_work TO stop BY -1 DO
   		XY_work[j,1] := (XY_work[j,1] - XY_work[j-1,1]) / (XY_work[j,0] - XY_work[j-i,0]);
	END_FOR;
END_FOR;


(* this part is the actual calculation *)
POLYNOM_INT := 0.0;
FOR i := Pts_work TO 1 BY -1 DO
       POLYNOM_INT := POLYNOM_INT * (X - XY_work[i,0]) + XY_work[i,1];
END_FOR;
END_FUNCTION

FUNCTION CIRCLE_A : REAL
VAR_INPUT
	rx : REAL;
	ax : REAL;
END_VAR


circle_A := Rx * RX * 8.726646E-3 * Ax;

(* revision histroy
hm	16. oct 2007	rev 1.0
	original version

hm	4. dec 2007		rev 1.1
	changed code for better performance

hm	22. feb 2008	rev 1.2
	improved performance
*)
END_FUNCTION

FUNCTION CIRCLE_C : REAL
VAR_INPUT
	Rx : REAL;
	Ax : REAL;
END_VAR


circle_C := 1.7453293E-2 * Rx * Ax;

(* revision histroy
hm	16. oct 2007	rev 1.0
	original version

hm	22. feb 2008	rev 1.1
	improved performance
*)
END_FUNCTION

FUNCTION CIRCLE_SEG : REAL
VAR_INPUT
	RX : REAL;
	HX : REAL;
END_VAR

IF RX > 0.0 THEN
	CIRCLE_SEG := 2.0 * ACOS(1.0 - LIMIT(0.0, HX / RX, 2.0));
	CIRCLE_SEG := (CIRCLE_SEG - SIN(CIRCLE_SEG)) *RX * RX / 2.0;
END_IF;


(* revision histroy
hm	10. mar 2010	rev 1.0
	original version

*)
END_FUNCTION

FUNCTION CONE_V : REAL
VAR_INPUT
	rx : REAL;
	hx : REAL;
END_VAR


cone_V := 1.047197551 * RX * RX * hx;

(* revision histroy
hm	17. oct 2007	rev 1.0
	original version

hm	4. dec 2007		rev 1.1
	changed code for better performance

hm	22. feb 2008	rev 1.2
	changed code for better performance
*)
END_FUNCTION

FUNCTION ELLIPSE_A : REAL
VAR_INPUT
	R1 : REAL;
	R2 : REAL;
END_VAR


ELLIPSE_A := math.PI * R1 * R2;

(* revision histroy
hm	16. oct 2007	rev 1.0
	original version

hm	18. oct. 2008	rev 1.1
	using math constants
*)
END_FUNCTION

FUNCTION ELLIPSE_C : REAL
VAR_INPUT
	R1 : REAL;
	R2 : REAL;
END_VAR


ELLIPSE_C := math.PI * (3.0 * (R1+R2) - SQRT((3.0 * r1 + R2) * (3.0 * r2 + r1)));

(* revision histroy
hm	16. oct 2007	rev 1.0
	original version

hm	18. oct. 2008	rev 1.1
	using math constants

hm	10. mar. 2009	rev 1.2
	changed syntax of real constants to 0.0

*)
END_FUNCTION

FUNCTION SPHERE_V : REAL
VAR_INPUT
	rx : REAL;
END_VAR


sphere_V := 4.188790205 * Rx * RX * RX;

(* revision histroy
hm	16. oct 2007	rev 1.0
	original version

hm	4. dec 2007		rev 1.1
	changed code for better performance

hm	22. feb 2008		rev 1.2
	changed code for better performance
*)

END_FUNCTION

FUNCTION TRIANGLE_A : REAL
VAR_INPUT
	S1 : REAL;
	A : REAL;
	S2 : REAL;
	S3 : REAL;
END_VAR


IF A = 0.0 THEN
	TRIANGLE_A := SQRT((s1+s2+S3) * (s1+s2-S3) * (S2+S3-S1) * (S3+S1-S2)) * 0.25;
ELSE
	TRIANGLE_A := S1 * S2 * SIN(RAD(A)) * 0.5;
END_IF;

(* revision histroy
hm	16. oct 2007	rev 1.0
	original version

hm	22. feb 2008	rev 1.1
	improved performance

hm	10. mar. 2009	rev 1.2
	changed syntax of real constants to 0.0

*)
END_FUNCTION

FUNCTION V3_ABS : REAL
VAR_INPUT
	A : Vector_3;
END_VAR

V3_ABS := SQRT(A.X * A.X + A.Y * A.Y + A.Z * A.Z);
END_FUNCTION

FUNCTION V3_ADD : vector_3
VAR_INPUT
	A : Vector_3;
	B : Vector_3;
END_VAR

V3_ADD.X := A.X + B.X;
V3_ADD.Y := A.Y + B.Y;
V3_ADD.Z := A.Z + B.Z;
END_FUNCTION

FUNCTION V3_ANG : REAL
VAR_INPUT
	A : Vector_3;
	B : Vector_3;
END_VAR
VAR
	d : REAL;
END_VAR

d := V3_ABS(A) * V3_ABS(B);
IF d > 0 THEN
	V3_ANG := ACOS(LIMIT(-1.0, V3_DPRO(A, B) / d,1.0));
END_IF;
END_FUNCTION

FUNCTION V3_DPRO : REAL
VAR_INPUT
	A : Vector_3;
	B : Vector_3;
END_VAR

V3_DPRO := A.X * B.X + A.Y * B.Y + A.Z * B.Z;
END_FUNCTION

FUNCTION V3_NORM : vector_3
VAR_INPUT
	A : Vector_3;
END_VAR
VAR
	la: REAL;
END_VAR

la := V3_ABS(A);
IF la > 0.0 THEN
	V3_NORM := V3_SMUL(A, 1.0 / la);
END_IF;
END_FUNCTION

FUNCTION V3_NUL : BOOL
VAR_INPUT
	A : Vector_3;
END_VAR

V3_NUL := A.X = 0.0 AND A.Y = 0.0 AND A.Z = 0.0;
END_FUNCTION

FUNCTION V3_PAR : BOOL
VAR_INPUT
	A : Vector_3;
	B : Vector_3;
END_VAR

V3_PAR := V3_ABS(V3_XPRO(A, B)) = 0.0;
END_FUNCTION

FUNCTION V3_REV : vector_3
VAR_INPUT
	A : Vector_3;
END_VAR

V3_REV.X := -A.X;
V3_REV.Y := -A.Y;
V3_REV.Z := -A.Z;
END_FUNCTION

FUNCTION V3_SMUL : vector_3
VAR_INPUT
	A : Vector_3;
	M : REAL;
END_VAR

V3_SMUL.X := A.X * M;
V3_SMUL.Y := A.Y * M;
V3_SMUL.Z := A.Z * M;
END_FUNCTION

FUNCTION V3_SUB : vector_3
VAR_INPUT
	A : Vector_3;
	B : Vector_3;
END_VAR

V3_SUB.X := A.X - B.X;
V3_SUB.Y := A.Y - B.Y;
V3_SUB.Z := A.Z - B.Z;
END_FUNCTION

FUNCTION V3_XANG : REAL
VAR_INPUT
	A : Vector_3;
END_VAR
VAR
	la: REAL;
END_VAR

la := V3_ABS(a);
IF la > 0.0 THEN
	V3_XANG := ACOS(A.X / la);
END_IF;
END_FUNCTION

FUNCTION V3_XPRO : vector_3
VAR_INPUT
	A : Vector_3;
	B : Vector_3;
END_VAR

V3_XPRO.X := A.Y * B.Z - A.Z * B.Y;
V3_XPRO.Y := A.Z * B.X - A.X * B.Z;
V3_XPRO.Z := A.X * B.Y - A.Y * B.X;
END_FUNCTION

FUNCTION V3_YANG : REAL
VAR_INPUT
	A : Vector_3;
END_VAR
VAR
	la: REAL;
END_VAR

la := V3_ABS(a);
IF la > 0.0 THEN
	V3_YANG := ACOS(A.Y / la);
END_IF;
END_FUNCTION

FUNCTION V3_ZANG : REAL
VAR_INPUT
	A : Vector_3;
END_VAR
VAR
	la: REAL;
END_VAR

la := V3_ABS(a);
IF la > 0.0 THEN
	V3_ZANG := ACOS(A.Z / la);
END_IF;
END_FUNCTION

FUNCTION ACOSH : REAL
VAR_INPUT
	X : REAL;
END_VAR

ACOSH := LN(SQRT(X * X - 1.0) + X);
END_FUNCTION

FUNCTION ACOTH : REAL
VAR_INPUT
	X : REAL;
END_VAR

ACOTH := LN((x+1.0)/(x-1.0))*0.5;
END_FUNCTION

FUNCTION AGDF : REAL
VAR_INPUT
	X : REAL;
END_VAR

AGDF := LN((1.0 + SIN(X)) / COS(X));

(* comment
the current implementation gives sufficient accuracy only up to X = 1.57 or an output > 10.
is X closer to PI/2 then the function is more and more unreliable
*)

END_FUNCTION

FUNCTION ASINH : REAL
VAR_INPUT
	X : REAL;
END_VAR

ASINH := LN(SQRT(X * X + 1.0)+X);
END_FUNCTION

FUNCTION ATAN2 : REAL
VAR_INPUT
	Y : REAL;
	X : REAL;
END_VAR

IF X > 0.0 THEN
	ATAN2 := ATAN(Y/X);
ELSIF X < 0.0 THEN
	IF Y >=0.0 THEN
		ATAN2 := ATAN(Y/X) + MATH.PI;
	ELSE
		ATAN2 := ATAN(Y/X) - MATH.PI;
	END_IF;
ELSIF Y > 0.0 THEN
	ATAN2 := MATH.PI05;
ELSIF Y < 0.0 THEN
	ATAN2 := -MATH.PI05;
ELSE
	ATAN2 := 0.0;
END_IF;

END_FUNCTION

FUNCTION ATANH : REAL
VAR_INPUT
	X : REAL;
END_VAR

ATANH := LN((1.0 + x)/(1.0 - x)) * 0.5;

END_FUNCTION

FUNCTION BETA : REAL
VAR_INPUT
	X : REAL;
	Y : REAL;
END_VAR

BETA := GAMMA(X) * GAMMA(Y) / GAMMA(x + y);
END_FUNCTION

FUNCTION BINOM : DINT
VAR_INPUT
	N : INT;
	K : INT;
END_VAR

VAR
	i: INT;
END_VAR
VAR
	N_work : INT;
	K_work : INT;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
N_work := N;
K_work := K;

IF 2 * K_work > N_work THEN
	K_work := N_work - K_work;
END_IF;
IF K_work > N_work THEN
	RETURN;
ELSIF K_work = 0 OR K_work = N_work THEN
	BINOM := 1;
ELSIF K_work = 1 THEN
	BINOM := N_work;
ELSE
	BINOM := N_work;
	N_work := N_work + 1;
	FOR i := 2 TO K_work DO
		BINOM := BINOM * (N_work - i) / i;
	END_FOR;
END_IF;



(*
binomialkoeffizient(n, k)
1  wenn k = 0 return 1
2  wenn 2k > n
3      dann f�hre aus ergebnis \leftarrow binomialkoeffizient(n, n-k)
4  sonst f�hre aus ergebnis \leftarrow n
5          von i \leftarrow 2 bis k
6              f�hre aus ergebnis \leftarrow ergebnis \cdot (n + 1 - i)
7                        ergebnis \leftarrow ergebnis : i
8  return ergebnis
*)
END_FUNCTION

FUNCTION CAUCHY : REAL
VAR_INPUT
	X : REAL;
	T : REAL;
	U : REAL;
END_VAR
VAR
	tmp: REAL;
END_VAR

tmp := x - t;
CAUCHY := math.PI_INV * U / (U*U + tmp*tmp);



(* revision hisdtory
hm	26. oct. 2008	rev 1.0
	original version

*)
END_FUNCTION

FUNCTION CAUCHYCD : REAL
VAR_INPUT
	X : REAL;
	T : REAL;
	U : REAL;
END_VAR

CAUCHYCD := 0.5 + math.PI_INV * ATAN((X - T) / U);



(* revision hisdtory
hm	26. oct. 2008	rev 1.0
	original version

*)
END_FUNCTION

FUNCTION CEIL : INT
VAR_INPUT
	X : REAL;
END_VAR



CEIL := REAL_TO_INT(x);
IF CEIL < X THEN
	CEIL := CEIL + 1;
END_IF;

END_FUNCTION

FUNCTION CEIL2 : DINT
VAR_INPUT
	X : REAL;
END_VAR



CEIL2 := REAL_TO_DINT(X);
IF DINT_TO_REAL(CEIL2) < X THEN
	CEIL2 := CEIL2 + 1;
END_IF;

END_FUNCTION

FUNCTION CMP : BOOL
VAR_INPUT
	X,Y : REAL;
	N : INT;
END_VAR
VAR
	tmp : REAL;
END_VAR

tmp := ABS(x);
IF tmp > 0.0 THEN
	tmp := EXP10(INT_TO_REAL(FLOOR(LOG(tmp))-N+1));
ELSE
	tmp := EXP10(tmp);
END_IF;
CMP := ABS(X - Y) < tmp;


END_FUNCTION

FUNCTION COSH : REAL
VAR_INPUT
	X : REAL;
END_VAR
VAR
	t: REAL;
END_VAR

T := EXP(X);
COSH := (1.0 / T + T) * 0.5;

(* revision histroy
hm	12.1.2007	rev 1.0
	original version

hm	1.12.2007	rev 1.1
	changed code to improve performance

hm	5. jan 2008	rev 1.2
	further performance improvement

hm	10. mar. 2009	rev 1.3
	real constants updated to new systax using dot
*)
END_FUNCTION

FUNCTION COTH : REAL
VAR_INPUT
	X : REAL;
END_VAR

IF X > 20.0 THEN
	COTH :=1.0;
ELSIF X < -20.0 THEN
	COTH := -1.0;
ELSE
	COTH := 1.0 + 2.0 / (EXP(X * 2.0) - 1.0);
END_IF;


(* revision histroy
hm		12.1.2007	rev 1.0
	original version

hm	1.12.2007		rev 1.1
	changed code to improve performance

hm	8. jan 2008	rev 1.2
	further improvement in performance

hm	10. mar 2008	rev 1.3
	extended range of valid inputs to +/- INV
	changed formula for better accuracy

hm	10. mar. 2009	rev 1.4
	real constants updated to new systax using dot
*)
END_FUNCTION

FUNCTION D_TRUNC : DINT
VAR_INPUT
	X : REAL;
END_VAR


D_TRUNC := REAL_TO_DINT(X);
IF X > 0.0 THEN
	IF DINT_TO_REAL(D_TRUNC) > X THEN D_TRUNC := D_TRUNC - 1; END_IF;
ELSE
	IF DINT_TO_REAL(D_TRUNC) < X THEN D_TRUNC := D_TRUNC + 1; END_IF;
END_IF;


(* for systems that support a dint truncation this routine can be replaced by trunc() *)
END_FUNCTION

FUNCTION DEC1 : INT
VAR_INPUT
	X : INT;
	N : INT;
END_VAR

IF X = 0 THEN
	DEC1 := N - 1;
ELSE
	DEC1 := X - 1;
END_IF;


(* this is a very elegant version but 50% slower
X := (X - 1 + N) MOD N;
*)
END_FUNCTION

FUNCTION DEG : REAL
VAR_INPUT
	rad : REAL;
END_VAR

DEG := MODR(57.29577951308232 * RAD, 360.0);
END_FUNCTION

FUNCTION DIFFER : BOOL
VAR_INPUT
	in1 : REAL;
	in2 : REAL;
	X : REAL;
END_VAR

DIFFER := ABS(in1 - in2) > X;
END_FUNCTION

FUNCTION ERF : REAL
VAR_INPUT
	X : REAL;
END_VAR
VAR
	x2 : REAL;
	ax2 : REAL;
END_VAR

x2 := X*X;
ax2 := 0.147 * x2 + 1.0;
ERF := SQRT(1.0 - EXP(-X2 * ((0.27323954473516 + aX2)/(ax2)))) * SGN(x);
END_FUNCTION

FUNCTION ERFC : REAL
VAR_INPUT
	X : REAL;
END_VAR

ERFC := 1.0 - ERF(X);
END_FUNCTION

FUNCTION EVEN : BOOL
VAR_INPUT
	in : DINT;
END_VAR

EVEN := NOT in.0;
END_FUNCTION

FUNCTION EXP10 : REAL
VAR_INPUT
	X : REAL;
END_VAR

EXP10 := EXP(X * 2.30258509299405);
END_FUNCTION

FUNCTION EXPN : REAL
VAR_INPUT
	X : REAL;
	N : INT;
END_VAR
VAR
	sign: BOOL;
END_VAR
VAR
	X_work : REAL;
	N_work : INT;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
X_work := X;
N_work := N;

sign := N_work.15;
N_work := ABS(N_work);
IF N_work.0 THEN EXPN := X_work; ELSE EXPN := 1.0; END_IF;
N_work := N_work / 2;
WHILE N_work > 0 DO
   X_work := X_work * X_work;
   IF N_work.0 THEN EXPN := EXPN * X_work; END_IF;
   N_work := N_work / 2;
END_WHILE;
IF sign THEN EXPN := 1.0 / EXPN; END_IF;
END_FUNCTION

FUNCTION FACT : DINT
VAR_INPUT
	X : INT;
END_VAR

IF X >= 0 AND X <= 12 THEN
	FACT := math.FACTS[X];
ELSE
	FACT := -1;
END_IF;


(* working code without array
IF X > 12 OR X < 0 THEN
	FACT := -1;
ELSIF X < 7 THEN
	FACT := 1;
	FOR i := 2 TO X DO
		FACT := FACT * i;
	END_FOR;
ELSE
	FACT := 5040;
	FOR i := 8 TO X DO
		FACT := FACT * i;
	END_FOR;
END_IF;
*)
END_FUNCTION

FUNCTION FIB : DINT
VAR_INPUT
	X : INT;
END_VAR
VAR
	t1, t2: DINT;
END_VAR
VAR
	X_work : INT;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
X_work := X;

IF X_work < 0 OR X_work > 46 THEN
	FIB := -1;
ELSIF X_work < 2 THEN
	FIB := X_work;
	RETURN;
ELSE
	(* the fibonacci number is the sum of the two suceeding fibonaci numbers *)
	(* we store the numbers alternatively in t1 and t2 depending on pt *)
	t2 := 1;
	WHILE X_work > 3 DO
		X_work := X_work-2;
		t1 := t1 + t2;
		t2 := t1 + t2;
	END_WHILE;
	IF X_work > 2 THEN t1 := t1 + t2; END_IF;
	fib := t1 + t2;
END_IF;

(* alternative code for very big numbers

IF X < 31 THEN
	fib := 0.4472136 * (expn(1.618034,X) - expn(-0.618034,X));
ELSE
	fib := 0.4472133 * expn(1.618034,X);
END_IF;

*)
END_FUNCTION

FUNCTION FLOOR : INT
VAR_INPUT
	X : REAL;
END_VAR


FLOOR := REAL_TO_INT(X);
IF FLOOR > X THEN
	FLOOR := FLOOR - 1;
END_IF;
END_FUNCTION

FUNCTION FLOOR2 : DINT
VAR_INPUT
	X : REAL;
END_VAR


FLOOR2 := REAL_TO_DINT(X);
IF DINT_TO_REAL(FLOOR2) > X THEN
	FLOOR2 := FLOOR2 - 1;
END_IF;
END_FUNCTION

FUNCTION FRACT : REAL
VAR_INPUT
	x : REAL;
END_VAR

IF ABS(x) < 2.0E9 THEN
	FRACT := x - DINT_TO_REAL(D_TRUNC(x));
ELSE
	FRACT := 0.0;
END_IF;
END_FUNCTION

FUNCTION GAMMA : REAL
VAR_INPUT
	X : REAL;
END_VAR

IF x > 0.0 THEN
	GAMMA := SQRT(math.PI2 / X) * EXPT(math.E_INV * (x + 1.0 / (12.0 * x - 0.1 / X)), X);
END_IF;


(* the stirling formula is not very accurate for small values of X
IF X >=0 THEN GAMMA := SQRT(math.PI2 * X) * EXPT(X / math.E, X); END_IF;
*)
END_FUNCTION

FUNCTION GAUSS : REAL
VAR_INPUT
	X : REAL;
	U : REAL;
	SI: REAL;
END_VAR
VAR
	temp: REAL;
	si_inv: REAL;
END_VAR

temp := X - U;
si_inv := 1.0  / si;
GAUSS := EXP(Temp * Temp * si_inv * si_inv * - 0.5) * 0.39894228 * si_inv;



(* revision hisdtory
hm	6. apr. 2008	rev 1.0
	original version

hm	27. oct. 2008	rev 1.1
	optimized performance	

hm	10. mar. 2009	rev 1.2
	real constants updated to new systax using dot

*)
END_FUNCTION

FUNCTION GAUSSCD : REAL
VAR_INPUT
	X : REAL;
	U : REAL;
	SI: REAL;
END_VAR

GAUSSCD := (ERF((X - U) / (SI * 1.414213562)) + 1.0) * 0.5;



(* revision hisdtory
hm	6. apr. 2008	rev 1.0
	original version

hm	10. mar. 2009	rev 1.1
	real constants updated to new systax using dot

*)
END_FUNCTION

FUNCTION GCD : INT
VAR_INPUT
	A : DINT;
	B : DINT;
END_VAR
VAR
	t: DINT;
END_VAR
VAR
	A_work : DINT;
	B_work : DINT;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
A_work := A;
B_work := B;

IF A_work = 0 THEN
	GCD := DINT_TO_INT(ABS(B_work));
ELSIF B_work = 0 THEN
	GCD := DINT_TO_INT(ABS(A_work));
ELSE
	A_work := ABS(A_work);
	B_work := ABS(B_work);
	GCD := 1;
	WHILE NOT(A_work.0 OR B_work.0) DO
		A_work := A_work / 2;
		B_work := B_work / 2;
		GCD := GCD * 2;
	END_WHILE;
	WHILE A_work > 0 DO
		IF NOT(A_work.0) THEN A_work := A_work / 2;
		ELSIF NOT(B_work.0) THEN B_work := B_work / 2;
		ELSE
			t:= ABS(A_work-B_work) / 2;
			IF A_work < B_work THEN B_work := t; ELSE A_work := T; END_IF;
		END_IF;
	END_WHILE;
	GCD := GCD * DINT_TO_INT(B_work);
END_IF;
END_FUNCTION

FUNCTION GDF : REAL
VAR_INPUT
	X : REAL;
END_VAR

IF X = 0.0 THEN
	GDF := 0.0;
ELSIF X > 15.0 THEN
	GDF := math.PI05;
ELSIF X < -15.0 THEN
	GDF := -math.PI05;
ELSE
	GDF := ATAN(EXP(X)) * 2.0 - math.PI05;
END_IF;

END_FUNCTION

FUNCTION GOLD : REAL
VAR_INPUT
	X : REAL;
END_VAR

GOLD := (X + SQRT(X*X + 4.0)) * 0.5;

END_FUNCTION

FUNCTION HYPOT : REAL
VAR_INPUT
	X : REAL;
	Y : REAL;
END_VAR

HYPOT := SQRT(x*x + y*y);

END_FUNCTION

FUNCTION INC : INT
VAR_INPUT
	X : INT;
	D : INT;
	M : INT;
END_VAR

INC := (X + D + M + 1) MOD (M + 1);
END_FUNCTION

FUNCTION INC1 : INT
VAR_INPUT
	X : INT;
	N : INT;
END_VAR

IF X >= N - 1 THEN
	INC1 := 0;
ELSE
	INC1 := X + 1;
END_IF;
END_FUNCTION

FUNCTION INC2 : INT
VAR_INPUT
	X : INT;
	D : INT;
	L : INT;
	U : INT;
END_VAR
VAR
	tmp: INT;
END_VAR

tmp := U - L + 1;
INC2 := (X + D - L + tmp) MOD tmp + L;
END_FUNCTION

FUNCTION INV : REAL
VAR_INPUT
	X : REAL;
END_VAR



IF X <> 0.0 THEN INV := 1.0 / X; END_IF;

END_FUNCTION

// Commented out for beedance: it uses REAL_TO_DW, which is commented out.
// FUNCTION LAMBERT_W : REAL
// VAR_INPUT
// 	X : REAL;
// END_VAR
// VAR
// 	w : REAL;
// 	i : INT;
// 	we: REAL;
// 	w1e: REAL;
// 	last: DWORD;
// 	ewx: REAL;
// END_VAR
//
// (* check for valid input and return -1000 if too low *)
// IF x < -0.367879441171442 THEN
// 	LAMBERT_W := -1000.0;
// 	RETURN;
// (* return 0 if x = 0 *)
// ELSIF x = 0.0 THEN
// 	RETURN;
// (* first an estimate is calculated *)
// ELSIF x <= 500.0 THEN
// 	w := LN(x + 1.0);
// 	w := 0.665 * (1.0 + 0.0195 * w) * w + 0.04;
// ELSE
// 	w := LN(x - 4.0) - (1.0 - 1.0/LN(x)) * LN(LN(x));
// END_IF;
// (* use estimate to calculate exact result *)
// FOR i := 0 TO 5 DO
// 	ewx := EXP(w);
// 	we := w * ewx - x;
// 	w1e := (w+1.0) * ewx;
// 	last := REAL_TO_DW(w) AND 16#FFFF_FFFC;
// 	w := w - (we / (w1e - (w+2.0) * we / (2.0 * w + 2.0)));
// 	IF (REAL_TO_DW(w) AND 16#FFFF_FFFC) = last THEN EXIT; END_IF;
// END_FOR;
// LAMBERT_W := w;
//
//
// (* revision hisdtory
// hm	26. oct. 2008	rev 1.0
// 	original version
//
// hm	10. mar. 2009	rev 1.1
// 	real constants updated to new systax using dot
//
// *)
// END_FUNCTION

FUNCTION LANGEVIN : REAL
VAR_INPUT
	X : REAL;
END_VAR

IF X = 0.0 THEN
	LANGEVIN := 0.0;
ELSE
	LANGEVIN := COTH(X) - 1.0 / X;
END_IF;
END_FUNCTION

FUNCTION MAX3 : REAL
VAR_INPUT
	in1 : REAL;
	in2 : REAL;
	in3 : REAL;
END_VAR

MAX3 := MAX(MAX(IN1,IN2),IN3);
END_FUNCTION

FUNCTION MID3 : REAL
VAR_INPUT
	IN1 : REAL;
	IN2 : REAL;
	IN3 : REAL;
END_VAR
VAR
	IN1_work : REAL;
	IN2_work : REAL;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
IN1_work := IN1;
IN2_work := IN2;


IF IN1_work > IN2_work THEN MID3 := IN1_work; IN1_work := IN2_work; IN2_work := MID3; END_IF;
IF IN2_work > IN3 THEN IN2_work := IN3; END_IF;
MID3 := SEL(IN1_work > IN2_work, IN2_work, IN1_work);


(*
hm	1.1.2007	rev 1.1
	rewrote the function for better performance

hm	18. mar. 2011	rev 1.2
	improve performance
*)
END_FUNCTION

FUNCTION MIN3 : REAL
VAR_INPUT
	in1 : REAL;
	in2 : REAL;
	in3 : REAL;
END_VAR

MIN3 := MIN(MIN(IN1,IN2),IN3);
END_FUNCTION

FUNCTION MODR : REAL
VAR_INPUT
	IN : REAL;
	DIVI : REAL;
END_VAR


IF divi = 0.0 THEN
	MODR := 0.0;
ELSE
	MODR := in - DINT_TO_REAL(FLOOR2(in / divi)) * divi;
END_IF;
END_FUNCTION

FUNCTION MUL_ADD : REAL
VAR_INPUT
	X : REAL;
	K : REAL;
	O : REAL;
END_VAR

MUL_ADD := X * K + O;
END_FUNCTION

FUNCTION NEGX : REAL
VAR_INPUT
	X : REAL;
END_VAR



NEGX := -X;

END_FUNCTION

FUNCTION RAD : REAL
VAR_INPUT
	DEG : REAL;
END_VAR

RAD := MODR(0.0174532925199433 * DEG, math.PI2);
END_FUNCTION

FUNCTION RDM : REAL
VAR_INPUT
	last : REAL;
END_VAR
VAR
	tn : DWORD;
	tc : INT;
END_VAR

tn := T_PLC_MS();
tc := BIT_COUNT(tn);
tn.31 := tn.2;
tn.30 := tn.5;
tn.29 := tn.4;
tn.28 := tn.1;
tn.27 := tn.0;
tn.26 := tn.7;
tn.25 := tn.6;
tn.24 := tn.3;
tn := ROL(tn,BIT_COUNT(tn)) OR 16#80000001;
tn := ULINT_TO_DWORD(DWORD_TO_ULINT(tn) MOD 71474513 + INT_TO_ULINT(tc + 77));
RDM := FRACT(DWORD_TO_REAL(tn) / 10000000.0 * (math.E - LIMIT(0.0,last,1.0)));


(*
pt := ADR(temp);
pt^ := (T_PLC_MS() AND 16#007FFFFF) OR 16#3D000000;
RDM := fract(modR(temp*e+pi1, PI1-last) + modR(temp*PI1+e + last,e-last));
*)
END_FUNCTION

FUNCTION RDM2 : INT
VAR_INPUT
	last : INT;
	low : INT;
	high : INT;
END_VAR

RDM2 := TRUNC(RDM(FRACT(last * math.PI)) * (high - low + 1)) + low;
END_FUNCTION

FUNCTION RDMDW : DWORD
VAR_INPUT
	last : DWORD;
END_VAR
VAR
	RX: REAL;
	M: REAL;
END_VAR

M := BIT_COUNT(last);
RX := RDM(FRACT(M * math.PI));
RDMDW := SHL(REAL_TO_DWORD(rx*65535),16);
RX := RDM(FRACT(M * math.E));
RDMDW := RDMDW OR (REAL_TO_DWORD(rx*65535) AND 16#0000FFFF);
END_FUNCTION

FUNCTION REAL_TO_FRAC : FRACTION
VAR_INPUT
	X : REAL;					(* in einen Bruch umzuwandelnder Wert *)
	N : INT;					(* maximale Gr��e des Nenners *)
END_VAR
VAR
	temp : DINT;				(* Merker f�r Berechnungen *)

	sign: BOOL;					(* Vorzeichen vom Eingangswert *)
	X_gerundet: DINT;			(* Wert von Bruch, auf ganze Zahl gerundet *)
	X_ohne_Nachkomma: REAL;		(* Wert von Bruch, ohne Nachkommastellen *)
	Numerator:       DINT := 1;	(* Initialwert Zaehler  *)
	Denominator:     DINT := 0;	(* Initialwert Nenner *)
	Numerator_old:   DINT := 0;	(* Initialwert Zaehler der letzten Berechnung  *)
	Denominator_old: DINT := 1;	(* Initialwert Zaehler der letzten Berechnung   *)

END_VAR
VAR
	X_work : REAL;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
X_work := X;


IF X_work < 0.0 THEN
	sign := TRUE;								(* Vorzeichen merken *)
	X_work := ABS(X_work);								(* Absolutwert berechnen *)
END_IF;

REPEAT
	X_gerundet := REAL_TO_DINT(X_work);

	(* Zaehler berechnen *)
	temp := numerator * X_gerundet + numerator_old;		(* Zaehler um Vorkammawert erweitern *)
	numerator_old := numerator;							(* Zaehler der letzten Berechnung speichern *)
	numerator := temp;									(* Zaehler dieser Berechnung speichern *)

	(* Nenner berechnen *)
	temp := denominator * X_gerundet + denominator_old;	(* Nenner um Vorkammawert erweitern *)
	denominator_old := denominator;						(* Nenner der letzten Berechnung speichern *)
	denominator := temp;								(* Nenner dieser Berechnung speichern *)

	(* Restwert berechnen *)
	X_ohne_Nachkomma := DINT_TO_REAL(X_gerundet);
	IF X_work = X_ohne_Nachkomma THEN						(* Bruch geht ohne Rest auf *)
		IF ABS(denominator) <= N THEN						(* kein Rundungsfehler *)
			numerator_old := numerator;					(* Numerator_old wird von Funktion zur�ckgegeben *)
			denominator_old := denominator;			(* Denominator_old wird von Funktion zur�ckgegeben *)
		END_IF
		EXIT;											(* keine weitere Berechnung notwendig *)
	ELSE
		X_work := 1.0 / (X_work - X_ohne_Nachkomma);				(* Kehrwert vom Rest -> Neuer Bruch *)
	END_IF

UNTIL
	ABS(Denominator) > N
END_REPEAT

(* correct sign if X was negative *)
IF sign THEN
	REAL_TO_FRAC.NUMERATOR := -1 * ABS(DINT_TO_INT(numerator_old));
ELSE
	REAL_TO_FRAC.NUMERATOR :=  ABS(DINT_TO_INT(numerator_old));
END_IF
REAL_TO_FRAC.DENOMINATOR := ABS(DINT_TO_INT(denominator_old));
END_FUNCTION

FUNCTION RND : REAL
VAR_INPUT
	X : REAL;
	N : INT;
END_VAR
VAR
	M : REAL;
END_VAR

IF X = 0.0 THEN
	RND := 0.0;
ELSE
	M := EXPN(10.0,N - CEIL(LOG(ABS(X))));
	RND := DINT_TO_REAL(REAL_TO_DINT(X * M)) / M;
END_IF;


END_FUNCTION

 FUNCTION ROUND : REAL
VAR_INPUT
	in : REAL;
	N : INT;
END_VAR
VAR
	X: REAL;
END_VAR

X := setup.DECADES[LIMIT(0,N,8)];
ROUND := DINT_TO_REAL(REAL_TO_DINT(in * X)) / X;




END_FUNCTION

FUNCTION SGN : INT
VAR_INPUT
	X : REAL;
END_VAR


IF X > 0 THEN
	SGN := 1;
ELSIF X < 0 THEN
	SGN := -1;
ELSE
	SGN := 0;
END_IF;



(* revision histroy
hm	16. oct 2007	rev 1.0
	original version

hm	11. nov 2007	rev 1.1
	changed type of function from real to int

*)
END_FUNCTION

FUNCTION SIGMOID : REAL
VAR_INPUT
	X : REAL;
END_VAR

IF X > 20.0 THEN
	SIGMOID := 1.0;
ELSIF x < -85.0 THEN
	SIGMOID := 0.0;
ELSE
	SIGMOID := 1.0 / (1.0 + EXP(-X));
END_IF;
END_FUNCTION

FUNCTION SIGN_I : BOOL
VAR_INPUT
	IN : DINT;
END_VAR

SIGN_I := in.31;
END_FUNCTION

FUNCTION SIGN_R : BOOL
VAR_INPUT
	in : REAL;
END_VAR

SIGN_R := in < 0.0;
END_FUNCTION

FUNCTION SINC : REAL
VAR_INPUT
	X : REAL;
END_VAR

IF X = 0.0 THEN
	SINC := 1.0;
ELSE
	SINC := SIN(x) / x;
END_IF;


(* revision histroy
hm	11. mar. 2008	rev 1.0
	original version

hm	1.12.2007	rev 1.1
	changed code to improove performance

hm 10. mar. 2009	rev 1.2
	real constants updated to new systax using dot																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																											

*)
END_FUNCTION

FUNCTION SINH : REAL
VAR_INPUT
	X : REAL;
END_VAR

IF ABS(x) < 2E-3 THEN
	SINH := X;
ELSE
	SINH := (EXP(x) - EXP(-x)) * 0.5;
END_IF;
END_FUNCTION

FUNCTION SQRTN : REAL
VAR_INPUT
	X : REAL;
	N : INT;
END_VAR

IF N > 0 THEN
	SQRTN := EXP(LN(x) / INT_TO_REAL(n));
ELSE
	SQRTN := 0.0;
END_IF;
END_FUNCTION

FUNCTION TANC : REAL
VAR_INPUT
	X : REAL;
END_VAR

IF X = 0.0 THEN
	TANC := 1.0;
ELSE
	TANC := TAN(x) / x;
END_IF;


(* revision histroy
hm	23. oct. 2008	rev 1.0
	original version

hm 10. mar. 2009	rev 1.1
	real constants updated to new systax using dot																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																											

*)
END_FUNCTION

FUNCTION TANH : REAL
VAR_INPUT
	X : REAL;
END_VAR

IF X > 20.0 THEN
	TANH := 1.0;
ELSIF X < -20.0 THEN
	TANH := -1.0;
ELSE
	TANH := 1.0 - 2.0 / (EXP(2.0 * x) + 1.0);
END_IF;


(* revision hisdtory
hm		12.1.2007		rev 1.0
	original version

hm		1.12.2007		rev 1.1
	changed code to improve performance

hm		10. mar 2008	rev 1.2
	corrected an error in formula
	extended range of valid inputs to +/- INV

hm 10. mar. 2009	rev 1.3
	real constants updated to new systax using dot																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																																											

*)
END_FUNCTION

FUNCTION WINDOW : BOOL
VAR_INPUT
	low : REAL;
	in : REAL;
	high : REAL;
END_VAR

WINDOW := in < high AND in > low;
END_FUNCTION

FUNCTION WINDOW2 : BOOL
VAR_INPUT
	LOW : REAL;
	IN : REAL;
	HIGH : REAL;
END_VAR

WINDOW2 := IN >= LOW AND IN <= HIGH;
END_FUNCTION

FUNCTION_BLOCK ESR_COLLECT
VAR_INPUT
	ESR_0, ESR_1, ESR_2, ESR_3, ESR_4, ESR_5, ESR_6, ESR_7 : ARRAY [0..3] OF esr_data;
	rst : BOOL;
END_VAR
VAR_IN_OUT
	pos : INT;
END_VAR
VAR_OUTPUT
	ESR_OUT : ARRAY[0..31] OF esr_data;
END_VAR
VAR CONSTANT
	max_in : INT := 3; (* max limit of input array *)
	max_out : INT := 32; (* number of elements in array *)
END_VAR
VAR
	cnt : INT := -1;
END_VAR


IF rst OR cnt < 0 THEN
	pos := -1;
ELSE
	FOR cnt := 0 TO max_in DO
	IF esr_0[cnt].typ > 0 THEN pos := INC1(pos, max_out); esr_out[pos] := esr_0[cnt]; END_IF;
	IF esr_1[cnt].typ > 0 THEN pos := INC1(pos, max_out); esr_out[pos] := esr_1[cnt]; END_IF;
	IF esr_2[cnt].typ > 0 THEN pos := INC1(pos, max_out); esr_out[pos] := esr_2[cnt]; END_IF;
	IF esr_3[cnt].typ > 0 THEN pos := INC1(pos, max_out); esr_out[pos] := esr_3[cnt]; END_IF;
	IF esr_4[cnt].typ > 0 THEN pos := INC1(pos, max_out); esr_out[pos] := esr_4[cnt]; END_IF;
	IF esr_5[cnt].typ > 0 THEN pos := INC1(pos, max_out); esr_out[pos] := esr_5[cnt]; END_IF;
	IF esr_6[cnt].typ > 0 THEN pos := INC1(pos, max_out); esr_out[pos] := esr_6[cnt]; END_IF;
	IF esr_7[cnt].typ > 0 THEN pos := INC1(pos, max_out); esr_out[pos] := esr_7[cnt]; END_IF;
   END_FOR;
END_IF;
END_FUNCTION_BLOCK

FUNCTION_BLOCK ESR_MON_B8
VAR_INPUT
	s0, s1, s2, s3, s4, s5, s6, s7 : BOOL;
	DT_in : DT;
END_VAR
VAR_INPUT CONSTANT
	a0, a1, a2, a3, a4, a5, a6, a7 : STRING(10);
END_VAR
VAR_OUTPUT
	ESR_Flag : BOOL;
END_VAR
VAR_IN_OUT
	ESR_Out: ARRAY [0..3] OF esr_data;
END_VAR
VAR
	x0, x1, x2, x3, x4, x5, x6, x7 : BOOL;
	tx: TIME;
	cnt : INT;
END_VAR


(* read system timer *)
tx := DWORD_TO_TIME(T_PLC_MS());

ESR_Flag := FALSE;
esr_out[3].typ := 0;
esr_out[2].typ := 0;
esr_out[1].typ := 0;
esr_out[0].typ := 0;
cnt := 0;


(* check if inputs have chaged and fill buffer *)
IF s0 <> X0 THEN
	esr_out[cnt].typ := USINT_TO_BYTE(10 + BOOL_TO_USINT(s0));
	esr_out[cnt].adress := a0;
	esr_out[cnt].DS := DT_in;
	esr_out[cnt].TS := TX;
	X0 := S0;
	cnt := cnt + 1;
	esr_flag := TRUE;
END_IF;
IF s1 <> X1 THEN
	esr_out[cnt].typ := USINT_TO_BYTE(10 + BOOL_TO_USINT(s1));
	esr_out[cnt].adress := a1;
	esr_out[cnt].DS := DT_in;
	esr_out[cnt].TS := TX;
	X1 := S1;
	cnt := cnt + 1;
	esr_flag := TRUE;
END_IF;
IF s2 <> X2 THEN
	esr_out[cnt].typ := USINT_TO_BYTE(10 + BOOL_TO_USINT(s2));
	esr_out[cnt].adress := a2;
	esr_out[cnt].DS := DT_in;
	esr_out[cnt].TS := TX;
	X2 := S2;
	cnt := cnt + 1;
	esr_flag := TRUE;
END_IF;
IF s3 <> X3 THEN
	esr_out[cnt].typ := USINT_TO_BYTE(10 + BOOL_TO_USINT(s3));
	esr_out[cnt].adress := a3;
	esr_out[cnt].DS := DT_in;
	esr_out[cnt].TS := TX;
	X3 := S3;
	cnt := cnt + 1;
	esr_flag := TRUE;
END_IF;
IF s4 <> X4 AND cnt < 4 THEN
	esr_out[cnt].typ := USINT_TO_BYTE(10 + BOOL_TO_USINT(s4));
	esr_out[cnt].adress := a4;
	esr_out[cnt].DS := DT_in;
	esr_out[cnt].TS := TX;
	X4 := S4;
	cnt := cnt + 1;
	esr_flag := TRUE;
END_IF;
IF s5 <> X5  AND cnt < 4 THEN
	esr_out[cnt].typ := USINT_TO_BYTE(10 + BOOL_TO_USINT(s5));
	esr_out[cnt].adress := a5;
	esr_out[cnt].DS := DT_in;
	esr_out[cnt].TS := TX;
	X5 := S5;
	cnt := cnt + 1;
	esr_flag := TRUE;
END_IF;
IF s6 <> X6  AND cnt < 4 THEN
	esr_out[cnt].typ := USINT_TO_BYTE(10 + BOOL_TO_USINT(s6));
	esr_out[cnt].adress := a6;
	esr_out[cnt].DS := DT_in;
	esr_out[cnt].TS := TX;
	X6 := S6;
	cnt := cnt + 1;
	esr_flag := TRUE;
END_IF;
IF s7 <> X7  AND cnt < 4 THEN
	esr_out[cnt].typ := USINT_TO_BYTE(10 + BOOL_TO_USINT(s7));
	esr_out[cnt].adress := a7;
	esr_out[cnt].DS := DT_in;
	esr_out[cnt].TS := TX;
	X7 := S7;
	cnt := cnt + 1;
	esr_flag := TRUE;
END_IF;
END_FUNCTION_BLOCK

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION_BLOCK ESR_MON_R4
// VAR_INPUT
// 	R0, R1, R2, R3 : REAL;
// 	DT_in : DT;
// END_VAR
// VAR_INPUT CONSTANT
// 	a0, a1, a2, a3 : STRING(10);
// 	s0, s1, s2, s3 : REAL;
// END_VAR
// VAR_OUTPUT
// 	ESR_Flag : BOOL;
// END_VAR
// VAR_IN_OUT
// 	ESR_Out: ARRAY [0..3] OF esr_data;
// END_VAR
// VAR
// 	p0, p1, p2, p3 : POINTER TO DWORD;
// 	x0, x1, x2, x3 : REAL;
// 	tx: TIME;
// 	cnt : INT;
// END_VAR
//
//
// (* read system timer *)
// tx := DWORD_TO_TIME(T_PLC_MS());
// P0 := ADR(R0);
// P1 := ADR(R1);
// P2 := ADR(R2);
// P3 := ADR(R3);
//
// ESR_Flag := FALSE;
// esr_out[3].typ := 0;
// esr_out[2].typ := 0;
// esr_out[1].typ := 0;
// esr_out[0].typ := 0;
// cnt := 0;
//
// (* check if inputs have chaged and fill buffer *)
// IF DIFFER(R0,  X0, S0) THEN
// 	esr_out[cnt].typ := 20;
// 	esr_out[cnt].adress := a0;
// 	esr_out[cnt].DS := DT_in;
// 	esr_out[cnt].TS := TX;
// 	esr_out[cnt].data[0] := Byte_of_Dword(P0^,0);
// 	esr_out[cnt].data[1] := Byte_of_Dword(P0^,1);
// 	esr_out[cnt].data[2] := Byte_of_Dword(P0^,2);
// 	esr_out[cnt].data[3] := Byte_of_Dword(P0^,3);
// 	X0 := R0;
// 	cnt := cnt + 1;
// 	esr_flag := TRUE;
// END_IF;
// IF differ(R1,  X1, S1) THEN
// 	esr_out[cnt].typ := 20;
// 	esr_out[cnt].adress := a1;
// 	esr_out[cnt].DS := DT_in;
// 	esr_out[cnt].TS := TX;
// 	esr_out[cnt].data[0] := Byte_of_Dword(P1^,0);
// 	esr_out[cnt].data[1] := Byte_of_Dword(P1^,1);
// 	esr_out[cnt].data[2] := Byte_of_Dword(P1^,2);
// 	esr_out[cnt].data[3] := Byte_of_Dword(P1^,3);
// 	X1 := R1;
// 	cnt := cnt + 1;
// 	esr_flag := TRUE;
// END_IF;
// IF differ(R2,  X2, S2) THEN
// 	esr_out[cnt].typ := 20;
// 	esr_out[cnt].adress := a2;
// 	esr_out[cnt].DS := DT_in;
// 	esr_out[cnt].TS := TX;
// 	esr_out[cnt].data[0] := Byte_of_Dword(P2^,0);
// 	esr_out[cnt].data[1] := Byte_of_Dword(P2^,1);
// 	esr_out[cnt].data[2] := Byte_of_Dword(P2^,2);
// 	esr_out[cnt].data[3] := Byte_of_Dword(P2^,3);
// 	X2 := R2;
// 	cnt := cnt + 1;
// 	esr_flag := TRUE;
// END_IF;
// IF DIFFER(R3,  X3, S3) THEN
// 	esr_out[cnt].typ := 20;
// 	esr_out[cnt].adress := a3;
// 	esr_out[cnt].DS := DT_in;
// 	esr_out[cnt].TS := TX;
// 	esr_out[cnt].data[0] := Byte_of_Dword(P3^,0);
// 	esr_out[cnt].data[1] := Byte_of_Dword(P3^,1);
// 	esr_out[cnt].data[2] := Byte_of_Dword(P3^,2);
// 	esr_out[cnt].data[3] := Byte_of_Dword(P3^,3);
// 	X3 := R3;
// 	cnt := cnt + 1;
// 	esr_flag := TRUE;
// END_IF;
//
// END_FUNCTION_BLOCK

FUNCTION_BLOCK ESR_MON_X8
VAR_INPUT
	s0, s1, s2, s3, s4, s5, s6, s7 : BYTE;
	DT_in : DT;
	Mode : BYTE := 3;
END_VAR
VAR_INPUT CONSTANT
	a0, a1, a2, a3, a4, a5, a6, a7 : STRING(10);
END_VAR
VAR_OUTPUT
	ESR_Flag : BOOL;
END_VAR
VAR_IN_OUT
	ESR_Out: ARRAY [0..3] OF esr_data;
END_VAR
VAR
	x0, x1, x2, x3, x4, x5, x6, x7 : BYTE;
	tx: TIME;
	cnt : INT;
END_VAR


(* read system timer *)
tx := DWORD_TO_TIME(T_PLC_MS());

ESR_Flag := FALSE;
esr_out[3].typ := 0;
esr_out[2].typ := 0;
esr_out[1].typ := 0;
esr_out[0].typ := 0;
cnt := 0;


(* check if inputs have chaged and fill buffer *)
IF s0 <> X0 AND ((s0 < 100) OR (S0 > 99 AND S0 < 200 AND mode >= 2) OR (S0 > 199 AND mode = 3)) THEN
	esr_out[cnt] := status_to_ESR(s0, a0, DT_in, TX);
	X0 := S0;
	cnt := cnt + 1;
	esr_flag := TRUE;
END_IF;
IF s1 <> X1 AND ((s1 < 100) OR (S1 > 99 AND S1 < 200 AND mode >= 2) OR (S1 > 199 AND mode = 3)) THEN
	esr_out[cnt] := status_to_ESR(s1, a1, DT_in, TX);
	X1 := S1;
	cnt := cnt + 1;
	esr_flag := TRUE;
END_IF;
IF s2 <> X2 AND ((s2 < 100) OR (S2 > 99 AND S2 < 200 AND mode >= 2) OR (S2 > 199 AND mode = 3)) THEN
	esr_out[cnt] := status_to_ESR(s2, a2, DT_in, TX);
	X2 := S2;
	cnt := cnt + 1;
	esr_flag := TRUE;
END_IF;
IF s3 <> X3 AND ((s3 < 100) OR (S3 > 99 AND S3 < 200 AND mode >= 2) OR (S3 > 199 AND mode = 3)) THEN
	esr_out[cnt] := status_to_ESR(s3, a3, DT_in, TX);
	X3 := S3;
	cnt := cnt + 1;
	esr_flag := TRUE;
END_IF;
IF cnt < 4 AND s4 <> X4 AND ((s4 < 100) OR (S4 > 99 AND S4 < 200 AND mode >= 2) OR (S4 > 199 AND mode = 3)) THEN
	esr_out[cnt] := status_to_ESR(s4, a4, DT_in, TX);
	X4 := S4;
	cnt := cnt + 1;
	esr_flag := TRUE;
END_IF;
IF cnt < 4 AND s5 <> X5 AND ((s5 < 100) OR (S5 > 99 AND S5 < 200 AND mode >= 2) OR (S5 > 199 AND mode = 3)) THEN
	esr_out[cnt] := status_to_ESR(s5, a5, DT_in, TX);
	X5 := S5;
	cnt := cnt + 1;
	esr_flag := TRUE;
END_IF;
IF cnt < 4 AND s6 <> X6 AND ((s6 < 100) OR (S6 > 99 AND S6 < 200 AND mode >= 2) OR (S6 > 199 AND mode = 3)) THEN
	esr_out[cnt] := status_to_ESR(s6, a6, DT_in, TX);
	X6 := S6;
	cnt := cnt + 1;
	esr_flag := TRUE;
END_IF;
IF cnt < 4 AND s7 <> X7 AND ((s7 < 100) OR (S7 > 99 AND S7 < 200 AND mode >= 2) OR (S7 > 199 AND mode = 3)) THEN
	esr_out[cnt] := status_to_ESR(s7, a7, DT_in, TX);
	X7 := S7;
	cnt := cnt + 1;
	esr_flag := TRUE;
END_IF;
END_FUNCTION_BLOCK

FUNCTION OSCAT_VERSION : DWORD
VAR_INPUT
	IN : BOOL;
END_VAR

IF in THEN
	OSCAT_VERSION := DATE_TO_DWORD(D#2024-07-16);
ELSE
	OSCAT_VERSION := 335;
END_IF;
END_FUNCTION

FUNCTION STATUS_TO_ESR : esr_data
VAR_INPUT
	status : BYTE;
	adress : STRING(10);
	DT_in : DT;
	TS : TIME;
END_VAR

IF status < 100 THEN
	status_to_ESR.typ := 1;
ELSIF status < 200 THEN
	status_to_ESR.typ := 2;
ELSE
	status_to_ESR.typ := 3;
END_IF;
status_to_ESR.adress:= adress;
status_to_ESR.DS := DT_in;
status_to_ESR.TS := TS;
status_to_ESR.data[0] := status;

END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION BIN_TO_BYTE : BYTE
// VAR_INPUT
// 	BIN : STRING(12);
// END_VAR
// VAR
// 	pt : POINTER TO BYTE;
// 	i: INT;
// 	X: BYTE;
// 	stop: INT;
// END_VAR
//
// pt := ADR(bin);
// stop := LEN(bin);
// FOR I := 1 TO stop DO
// 	x := pt^;
// 	(* calculate the value of the digit *)
// 	IF X = 48 THEN
// 		BIN_TO_BYTE := SHL(BIN_TO_BYTE,1);
// 	ELSIF X = 49 THEN
// 		BIN_TO_BYTE := SHL(BIN_TO_BYTE,1) OR 1;
// 	END_IF;
// 	pt := pt + 1;
// END_FOR;
//
//
//
// (* revision histroy
// hm	18. jun. 2008	rev 1.0
// 	original release
//
// hm	20. sep. 2008	rev 1.1
// 	changed length of input string from 20 to 12
//
// hm	26. jul. 2009	rev 1.2
// 	optimized code
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION BIN_TO_DWORD : DWORD
// VAR_INPUT
// 	BIN : STRING(40);
// END_VAR
// VAR
// 	pt : POINTER TO BYTE;
// 	i: INT;
// 	X: BYTE;
// 	stop: INT;
// END_VAR
//
// pt := ADR(bin);
// stop := LEN(bin);
// FOR I := 1 TO stop DO
// 	x := pt^;
// 	(* calculate the value of the digit *)
// 	IF X = 48 THEN
// 		BIN_TO_DWORD := SHL(BIN_TO_DWORD,1);
// 	ELSIF X = 49 THEN
// 		BIN_TO_DWORD := SHL(BIN_TO_DWORD,1) OR 1;
// 	END_IF;
// 	pt := pt + 1;
// END_FOR;
//
//
//
// (* revision histroy
// hm	18. jun. 2008	rev 1.0
// 	original release
//
// hm	20. sep. 2008	rev 1.1
// 	changed length of input dtring from 20 to 40
//
// hm	26. jul 2009	rev 1.2
// 	optimized code
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION BYTE_TO_STRB : STRING(8)
// VAR_INPUT
// 	IN : BYTE;
// END_VAR
// VAR
// 	i: INT;
// 	pt : POINTER TO BYTE;
// END_VAR
//
//
// (* pointer f�r die ausgabe ermitteln *)
// pt := ADR(BYTE_TO_STRB);
// (* die 8 ausgabecharacter ermitteln und schreiben *)
// FOR i := 1 TO 8 DO
// 	pt^ := BOOL_TO_BYTE(in.7) + 48;
// 	in := SHL(in,1);
// 	pt := pt + 1;
// END_FOR;
//
// (* der ausgabestring muss noch mit 0 abgeschlossen werden *)
// pt^ := 0;
//
// (* code before rev 1.1
// FOR i := 1 TO 8 DO
// 	IF in.0 = 0 THEN temp := CONCAT('0', temp); ELSE temp := CONCAT('1', temp); END_IF;
// 	in := SHR(in, 1);
// END_FOR;
// BYTE_TO_STRB := temp;
// *)
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION BYTE_TO_STRH : STRING(2)
// VAR_INPUT
// 	IN : BYTE;
// END_VAR
// VAR
// 	temp : BYTE;
// 	PT : POINTER TO BYTE;
// END_VAR
//
//
// (* read pointer to output string *)
// PT := ADR(BYTE_TO_STRH);
// (* calculate high order hex value *)
// temp := SHR(in,4);
// (* convert value to hex character *)
// IF temp <= 9 THEN temp := temp + 48; ELSE temp := temp + 55; END_IF;
// (* write friat character to output string *)
// PT^ := temp;
// (* calculate low order hex value *)
// temp := in AND 2#00001111;
// IF temp <= 9 THEN temp := temp + 48; ELSE temp := temp + 55; END_IF;
// (* increment pointer and wirte low order character *)
// pt := pt + 1;
// pt^ := temp;
// (* set pointer at the end of the output string and close the string with 0 *)
// pt := pt + 1;
// pt^:= 0;
//
//
// (* code before rev 1.2
// FOR i := 1 TO 2 DO
// 	X := in AND 2#1111;
// 	IF X <= 9 THEN X := X + 48; ELSE X := X + 55; END_IF;
// 	Cx := CHR(X);
// 	temp := CONCAT(Cx, temp);
// 	in := SHR(in,4);
// END_FOR;
// BYTE_TO_STRH := temp;
// *)
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION CAPITALIZE : STRING(STRING_LENGTH)
// VAR_INPUT
// 	str : STRING(STRING_LENGTH);
// END_VAR
// VAR
// 	PT : POINTER TO BYTE;
// 	pos : INT;
// 	L : INT;
// 	first : BOOL := TRUE;
// END_VAR
//
// PT := ADR(CAPITALIZE);
// CAPITALIZE := str;
// L := LEN(str);
// FOR pos := 1 TO l DO
// 	IF first THEN pt^ := TO_UPPER(pt^);	END_IF;
// 	(* remember in first if the next char needs to capitalized *)
// 	first := pt^= 32;
// 	PT := pt + 1;
// END_FOR;
//
// (* revision histroy
// hm		4. mar 2008	rev 1.0
// 	original release
//
// hm	29. mar. 2008	rev 1.1
// 	changed STRING to STRING(STRING_LENGTH)
//
// hm	20. jun. 2008	rev 1.2
// 	improved performance
//
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses CODE, which is commented out.
// FUNCTION CHARCODE : BYTE
// VAR_INPUT
// 	STR : STRING(10);
// END_VAR
// VAR
// 	found: STRING(1);
// 	search : STRING(10);
// 	pos: INT;
// 	i: INT;
// END_VAR
//
// IF LEN(str) = 1 THEN
// 	CHARCODE := CODE(STR, 1);
// ELSIF str <> '' THEN
// 	(* construct search string *)
// 	search := CONCAT('&', str);
// 	search := CONCAT(search, ';');
// 	WHILE pos = 0 AND (i < 4) DO
// 		i := i + 1;
// 		pos := FIND(setup.CHARNAMES[i], search);
// 	END_WHILE;
// 	found := MID(setup.CHARNAMES[i], 1, pos - 1);
// 	CHARCODE := CODE(found, 1);
// END_IF;
//
// END_FUNCTION

// Commented out for beedance: it uses CHR_TO_STRING, which is commented out.
// FUNCTION CHARNAME : STRING(10)
// VAR_INPUT
// 	C : BYTE;
// END_VAR
// VAR
// 	pos: INT;
// 	i : INT;
// END_VAR
//
// IF C <> 0 THEN
// 	(* construct search string from code followed by $ sign, also clear charname string*)
// 	CHARNAME := CHR_TO_STRING(C);
// 	CHARNAME := CONCAT(CHARNAME,'&');
// 	CHARNAME := CONCAT(';', CHARNAME);
// 	WHILE pos = 0 AND (i < 4) DO
// 		i := i + 1;
// 		pos := FIND(setup.CHARNAMES[i], CHARNAME);
// 	END_WHILE;
// 	IF pos > 0 THEN
// 		CHARNAME := MID(setup.CHARNAMES[i], 10, pos + 3);
// 		(* search for end of name and truncate *)
// 		pos := FIND(CHARNAME, ';');
// 		CHARNAME := LEFT(CHARNAME,pos - 1);
// 	ELSE
// 		CHARNAME := CHR_TO_STRING(C);
// 	END_IF;
// ELSE
// 	CHARNAME := '';
// END_IF;
//
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION CHR_TO_STRING : STRING(1)
// VAR_INPUT
// 	C : BYTE;
// END_VAR
// VAR
// 	PT : POINTER TO BYTE;
// END_VAR
//
// PT := ADR(CHR_TO_STRING);
// PT^ := C;
// pt := pt + 1;
// pt^ := 0;
// END_FUNCTION

FUNCTION CLEAN : STRING(STRING_LENGTH)
VAR_INPUT
	IN : STRING(STRING_LENGTH);
	CX : STRING(80);
END_VAR
VAR
	pos: INT := 1;
	stop: INT;
END_VAR


(* copy input string *)
CLEAN := IN;
stop := LEN(in);
WHILE pos <= stop DO
	IF FIND(cx, MID(CLEAN, 1, pos)) > 0 THEN
		(* charcter found skip to next one *)
		pos := pos + 1;
	ELSE
		(* wrong chracter needs to be deleted *)
		CLEAN := DELETE(CLEAN, 1, pos);
		stop := stop - 1;	(* the string is one character shorter now *)
	END_IF;
END_WHILE;
END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION CODE : BYTE
// VAR_INPUT
// 	STR : STRING(STRING_LENGTH);
// 	POS : INT;
// END_VAR
// VAR
// 	PT : POINTER TO BYTE;
// END_VAR
//
// IF pos < 1 OR pos > LEN(str) THEN
// 	CODE := 0;
// 	RETURN;
// ELSE
// 	PT := ADR(STR) + INT_TO_DWORD(pos - 1);
// 	CODE := pt^;
// END_IF;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION COUNT_CHAR : INT
// VAR_INPUT
// 	str : STRING(STRING_LENGTH);
// 	chr : BYTE;
// END_VAR
// VAR
// 	l: INT;
// 	pt : POINTER TO BYTE;
// 	pos: INT;
// END_VAR
//
// PT := ADR(str);
// l := LEN(str);
// COUNT_CHAR := 0;
// FOR pos := 1 TO l DO
// 	IF pt^ = CHR THEN COUNT_CHAR := COUNT_CHAR + 1; END_IF;
//     PT := PT + 1;
// END_FOR;
// END_FUNCTION

FUNCTION COUNT_SUBSTRING : INT
VAR_INPUT
   SEARCH : STRING;
   STR : STRING;
END_VAR
VAR
   pos : INT;
   size : INT;
END_VAR
VAR
	STR_work : STRING;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
STR_work := STR;

COUNT_SUBSTRING := 0;
size := LEN(SEARCH);
REPEAT
   pos := FIND(STR_work,SEARCH);
   IF pos > 0 THEN
      STR_work := REPLACE(STR_work, '', size,pos);
      COUNT_SUBSTRING := COUNT_SUBSTRING + 1;
   END_IF;
UNTIL pos = 0
END_REPEAT;


END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION DEC_TO_BYTE : BYTE
// VAR_INPUT
// 	DEC : STRING(10);
// END_VAR
// VAR
// 	pt : POINTER TO BYTE;
// 	i: INT;
// 	X: BYTE;
// 	stop: INT;
// END_VAR
//
// pt := ADR(dec);
// stop := LEN(DEC);
// FOR I := 1 TO stop DO
// 	(* read the first character and subtract 48 to get value in decimal 0 = 48 *)
// 	x := pt^;
// 	(* calculate the value of the digit *)
// 	IF X > 47 AND x < 58 THEN
// 		DEC_TO_BYTE := DEC_TO_BYTE * 10 + X - 48;
// 	END_IF;
// 	pt := pt + 1;
// END_FOR;
//
//
//
// (* revision histroy
// hm	20. jun. 2008	rev 1.0
// 	original release
//
// hm	30. sep.2008	rev 1.1
// 	changed length of input string from 20 to 10
// 	corrected an error where decoding of characters 8 and 9 would fail
//
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION DEC_TO_DWORD : DWORD
// VAR_INPUT
// 	DEC : STRING(20);
// END_VAR
// VAR
// 	pt : POINTER TO BYTE;
// 	i: INT;
// 	X: BYTE;
// 	stop: INT;
// END_VAR
//
// pt := ADR(dec);
// stop := LEN(dec);
// FOR I := 1 TO stop DO
// 	(* read the first character and subtract 48 to get value in decimal 0 = 48 *)
// 	x := pt^;
// 	(* calculate the value of the digit *)
// 	IF X > 47 AND x < 58 THEN
// 		DEC_TO_DWORD := DEC_TO_DWORD * 10 + X - 48;
// 	END_IF;
// 	pt := pt + 1;
// END_FOR;
//
//
//
// (* revision histroy
// hm	20. jun. 2008	rev 1.0
// 	original release
//
// hm	30. sep. 2008	rev 1.1
// 	corrected an error where decoding of characters 8 and 9 would fail
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION DEC_TO_INT : INT
// VAR_INPUT
// 	DEC : STRING(10);
// END_VAR
// VAR
// 	pt : POINTER TO BYTE;
// 	i: INT;
// 	X: BYTE;
// 	sign: BOOL;
// 	stop: INT;
// END_VAR
//
// pt := ADR(dec);
// stop := LEN(dec);
// FOR I := 1 TO stop DO
// 	(* read the first character and subtract 48 to get value in decimal 0 = 48 *)
// 	x := pt^;
// 	(* calculate the value of the digit *)
// 	IF X > 47 AND x < 58 THEN
// 		DEC_TO_INT := DEC_TO_INT * 10 + X - 48;
// 	ELSIF X = 45 AND DEC_TO_INT = 0 THEN
// 		sign := TRUE;
// 	END_IF;
// 	pt := pt + 1;
// END_FOR;
// IF sign THEN DEC_TO_INT := -DEC_TO_INT; END_IF;
//
//
// (* revision histroy
// hm	20. jun. 2008	rev 1.0
// 	original release
//
// hm	30. sep. 2008	rev 1.1
// 	changed length of input string from 20 to 10
// 	corrected an error where decoding of characters 8 and 9 would fail
// *)
//
// END_FUNCTION

FUNCTION DEL_CHARS : STRING(STRING_LENGTH)
VAR_INPUT
	IN : STRING(STRING_LENGTH);
	CX : STRING(80);
END_VAR
VAR
	pos: INT := 1;
	stop: INT;
END_VAR


(* copy input string *)
DEL_CHARS := IN;
stop := LEN(in);
WHILE pos <= stop DO
	IF FIND(cx, MID(DEL_CHARS, 1, pos)) > 0 THEN
		(* wrong chracter needs to be deleted *)
		DEL_CHARS := DELETE(DEL_CHARS, 1, pos);
		stop := stop - 1;	(* the string is one character shorter now *)
	ELSE
		(* charcter not found skip to next one *)
		pos := pos + 1;
	END_IF;
END_WHILE;
END_FUNCTION

// Commented out for beedance: it uses CODE, which is commented out.
// FUNCTION DT_TO_STRF : STRING
// VAR_INPUT
// 	DTI : DT;
// 	MS : INT;
// 	FMT : STRING;
// 	LANG : INT;
// END_VAR
// VAR CONSTANT
// 	FILL : STRING(1) := '0';
// 	BLANK : STRING(1) := ' ';
// END_VAR
// VAR
// 	ly : INT;
// 	dx: DATE;
// 	fs: STRING(10);
// 	td: TOD;
// 	tmp : INT;
// 	pos : INT;
// 	f : INT;
// END_VAR
//
// IF LANG < 1 THEN ly := language.DEFAULT; ELSE ly := MIN(language.LMAX, LANG); END_IF;
//
// (* decode date and time information *)
// dx := DT_TO_DATE(DTI);
// td := DT_TO_TOD(DTI);
//
// (* parse the format string *)
// DT_TO_STRF := FMT;
// pos := FIND(DT_TO_STRF, '#');
// WHILE pos > 0 DO
// 	(* retrieve format identifier *)
// 	f := CODE(DT_TO_STRF, pos + 1);
// 	(* generate the return string according to the format character *)
// 	fs := '';
// 	CASE f OF
// 		65 : (* letter A retunrs the year in 4 digits *)
// 			fs := INT_TO_STRING(YEAR_OF_DATE(dx));
// 		66 : (* letter B returns the year in exactly 2 digits *)
// 			fs := RIGHT(INT_TO_STRING(YEAR_OF_DATE(dx)),2);
// 		67 : (* letter C returns the month with 1 or 2 digits *)
// 			fs := INT_TO_STRING(MONTH_OF_DATE(dx));
// 		68 : (* letter D returns the month with exactly 2 digits *)
// 			fs := INT_TO_STRING(MONTH_OF_DATE(dx));
// 			IF LEN(fs) < 2 THEN fs := CONCAT('0', fs); END_IF;
// 		69 : (* letter E returns the month with 3 characters *)
// 			fs := MONTH_TO_STRING(MONTH_OF_DATE(dx), ly, 3);
// 		70 : (* letter F returns the month with all characters *)
// 			fs := MONTH_TO_STRING(MONTH_OF_DATE(dx), ly, 0);
// 		71 : (* letter G returns the day with up to 2 digits *)
// 			fs := INT_TO_STRING(DAY_OF_MONTH(dx));
// 		72 : (* letter H returns the day of the month with exactly 2 digits *)
// 			fs := INT_TO_STRING(DAY_OF_MONTH(dx));
// 			IF LEN(fs) < 2 THEN fs := CONCAT(FILL, fs); END_IF;
// 		73 : (* letter I returns the weekday as the number 1..7 1 = monday *)
// 			fs := INT_TO_STRING(DAY_OF_WEEK(dx));
// 		74 : (* letter J returns the weekday in 2 character writing *)
// 			fs := WEEKDAY_TO_STRING(DAY_OF_WEEK(dx), ly, 2);
// 		75 : (* letter K returns the weekday with all characters *)
// 			fs := WEEKDAY_TO_STRING(DAY_OF_WEEK(dx), ly, 0);
// 		76 : (* letter L returns AM or PM for the given DateTime *)
// 			IF td >= TOD#12:00:00 THEN fs := 'PM'; ELSE fs := 'AM'; END_IF;
// 		77 : (* letter M returns the hour in 1 or 2 digit form 0..24h *)
// 			fs := INT_TO_STRING(HOUR(td));
// 		78 : (* letter N returns the hour in exactly 2 digit form 0..24h *)
// 			fs := INT_TO_STRING(HOUR(td));
// 			IF LEN(fs) < 2 THEN fs := CONCAT(FILL, fs); END_IF;
// 		79 : (* letter O returns the hour in 1 or 2 digit form 0..12h *)
// 			tmp := HOUR(td) MOD 12;
// 			IF tmp = 0 THEN tmp := 12; END_IF;
// 			fs := INT_TO_STRING(tmp);
// 		80 : (* letter P returns the hour in exactly 2 digit form 0..12h *)
// 			tmp := HOUR(td) MOD 12;
// 			IF tmp = 0 THEN tmp := 12; END_IF;
// 			fs := INT_TO_STRING(tmp);
// 			IF LEN(fs) < 2 THEN fs := CONCAT(FILL, fs); END_IF;
// 		81 : (* letter Q returns the minute of the hour in 1 or two digit form *)
// 			fs := INT_TO_STRING(MINUTE(td));
// 		82 : (* letter R returns the minute of the hour in exactly two digit form *)
// 			fs := INT_TO_STRING(MINUTE(td));
// 			IF LEN(fs) < 2 THEN fs := CONCAT(FILL, fs); END_IF;
// 		83 : (* letter S returns the second of the minute in 1 or two digit form *)
// 			fs := INT_TO_STRING(REAL_TO_INT(SECOND(td)));
// 		84 : (* letter T returns the second of the minute in exactly two digit form *)
// 			fs := INT_TO_STRING(REAL_TO_INT(SECOND(td)));
// 			IF LEN(fs) < 2 THEN fs := CONCAT(FILL, fs); END_IF;
// 		85 : (* letter U returns the milliseconds in 1 to 3 digits *)
// 			fs := INT_TO_STRING(MS);
// 		86 : (* letter V returns the milliseconds in exactly 3 digit form *)
// 			fs := INT_TO_STRING(MS);
// 			fs := CONCAT('00',fs);
// 			fs := RIGHT(fs, 3);
// 		87 : (* letter W returns the day of the month with exactly 2 digits first digit is filled with blank if necessary *)
// 			fs := INT_TO_STRING(DAY_OF_MONTH(dx));
// 			IF LEN(fs) < 2 THEN fs := CONCAT(BLANK, fs); END_IF;
// 		88 : (* letter X returns the month with exactly 2 digits first digit is filled with blank if necessary *)
// 			fs := INT_TO_STRING(MONTH_OF_DATE(dx));
// 			IF LEN(fs) < 2 THEN fs := CONCAT(BLANK, fs); END_IF;
// 		END_CASE;
// 	DT_TO_STRF := REPLACE(DT_TO_STRF, fs, 2, pos);
// 	pos := FIND(DT_TO_STRF, '#');
// END_WHILE;
//
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION DWORD_TO_STRB : STRING(32)
// VAR_INPUT
// 	IN : DWORD;
// END_VAR
// VAR
// 	pt : POINTER TO BYTE;
// 	i: INT;
// END_VAR
//
// (* pointer f�r die ausgabe ermitteln *)
// pt := ADR(DWORD_TO_STRB);
// (* die 8 ausgabecharacter ermitteln und schreiben *)
// FOR i := 1 TO 32 DO
// 	pt^:= BOOL_TO_BYTE(in.31) + 48;
// 	in := SHL(in,1);
// 	pt := pt + 1;
// END_FOR;
//
// (* der ausgabestring muss nochg mit 0 abgeschlossen werden *)
// pt^ := 0;
//
// END_FUNCTION

// Commented out for beedance: it uses FIX, which is commented out.
// FUNCTION DWORD_TO_STRF : STRING(20)
// VAR_INPUT
// 	IN : DWORD;
// 	N : INT;
// END_VAR
//
//
//
// (* limit N to max 20 characters *)
// (* convert dword to string first and cut to length N *)
// DWORD_TO_STRF := FIX(DWORD_TO_STRING(in),LIMIT(0,N,20),48,1);
//
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION DWORD_TO_STRH : STRING(8)
// VAR_INPUT
// 	IN : DWORD;
// END_VAR
// VAR
// 	i : INT;
// 	temp : BYTE;
// 	pt : POINTER TO BYTE;
// END_VAR
//
//
// (* read output adress to pointer *)
// pt := ADR(DWORD_TO_STRH) + 8;
// (* wirte the closing byte for the string *)
// pt^:= 0;
//
// (* write the 8 hex characters backwards *)
// FOR i := 1 TO 8 DO;
// 	(* decrement the pointer *)
// 	pt := pt - 1;
// 	(* read the lowest order hex value *)
// 	temp := DWORD_TO_BYTE(in AND 16#0000000F);
// 	(* convert value to hex character *)
// 	IF temp <= 9 THEN temp := temp + 48; ELSE temp := temp + 55; END_IF;
// 	(* write character to output string *)
// 	PT^ := temp;
// 	(* shift in for nect hex character *)
// 	in := SHR(in,4);
// END_FOR;
//
//
// (* code beofre rev 1.2
// FOR i := 1 TO 8 DO
// 	X := DWORD_TO_BYTE(in AND 2#1111);
// 	IF X <= 9 THEN X := X + 48; ELSE X := X + 55; END_IF;
// 	Cx := CHR(X);
// 	temp := CONCAT(Cx, temp);
// 	in := SHR(in,4);
// END_FOR;
// DWORD_TO_STRH := temp;
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses FINDB_NONUM, which is commented out.
// FUNCTION EXEC : STRING(STRING_LENGTH)
// VAR_INPUT
// 	str : STRING(STRING_LENGTH);
// END_VAR
// VAR
// 	pos: INT;
// 	R1: REAL;
// 	R2: REAL;
// 	operator: STRING(10);
// END_VAR
//
// (* extract both numbers and operator *)
// EXEC := UPPERCASE(TRIM(str));
//
// pos := FINDB_NONUM(EXEC);
// IF pos > 1 THEN R1 := STRING_TO_REAL(LEFT(EXEC,pos-1)); END_IF;
// R2 := STRING_TO_REAL(RIGHT(EXEC,LEN(EXEC)-pos));
// EXEC := LEFT(EXEC,pos);
// pos := FINDB_NUM(EXEC);
// operator := RIGHT(EXEC,LEN(EXEC) - pos);
// IF operator = '' AND LEN(str) = 0 THEN
// 	EXEC := '';
// 	RETURN;
// ELSIF operator = '' THEN
// 	EXEC := str;
// 	RETURN;
// END_IF;
// IF operator = '^' THEN
// 	EXEC := REAL_TO_STRING(EXPT(R1, R2));
// ELSIF operator = 'SQRT' THEN
// 	EXEC := REAL_TO_STRING(SQRT(R2));
// ELSIF operator = 'SIN' THEN
// 	EXEC := REAL_TO_STRING(SIN(r2));
// ELSIF operator = 'COS' THEN
// 	EXEC := REAL_TO_STRING(COS(r2));
// ELSIF operator = 'TAN' THEN
// 	EXEC := REAL_TO_STRING(TAN(R2));
// ELSIF operator = '*' THEN
// 	EXEC := REAL_TO_STRING(R1 * R2);
// ELSIF operator = '/' THEN
// 	IF R2 <> 0 THEN EXEC := REAL_TO_STRING(R1 / R2); ELSE EXEC := 'ERROR'; END_IF;
// ELSIF operator = '+' THEN
// 	EXEC := REAL_TO_STRING(r1 + r2);
// ELSIF operator = '-' THEN
// 	EXEC := REAL_TO_STRING(r1 - r2);
// ELSE
// 	EXEC := 'ERROR';
// END_IF;
//
// IF EXEC = 'ERROR' THEN
// 	RETURN;
// (* some systems deliver integer instead of real *)
// ELSIF FIND(EXEC,'.') = 0 THEN
// 	EXEC := CONCAT(EXEC, '.0');
// (* some systems deliver n. instead of n.0 ! *)
// ELSIF RIGHT(EXEC,1) = '.' THEN
// 	EXEC := CONCAT(EXEC,'0');
// END_IF;
// END_FUNCTION

// Commented out for beedance: it uses CHR_TO_STRING, which is commented out.
// FUNCTION FILL : STRING(STRING_LENGTH)
// VAR_INPUT
// 	C : BYTE;
// 	L : INT;
// END_VAR
// VAR
// 	i: INT;
// 	sx: STRING(1);
// END_VAR
// VAR
// 	L_work : INT;
// END_VAR
// (* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
// L_work := L;
//
// Sx := CHR_TO_STRING(c);
// (* limit L to maximum 80 characters the length of a standard string *)
// L_work := LIMIT(0,L_work,STRING_LENGTH);
// (* create a string of characters to be connected to str *)
// FOR i := 1 TO 8 DO
// 	FILL := CONCAT(FILL,FILL);
// 	IF L_work.7 THEN FILL := CONCAT(FILL,Sx); END_IF;
// 	L_work := SHL(L_work,1);
// END_FOR;
//
//
//
//
// (* revision histroy
// hm	29. mar. 2008	rev 1.0
// 	original release
//
// hm	17. dec. 2008	rev 1.1
// 	changed function chr to chr_to_string
//
// *)
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION FIND_CHAR : INT
// VAR_INPUT
// 	str : STRING(STRING_LENGTH);
// 	pos : INT;
// END_VAR
// VAR
// 	i: INT;
// 	pt : POINTER TO ARRAY[1..255] OF BYTE;
// 	stop: INT;
// 	X: BYTE;
// END_VAR
//
// pt := ADR(str);
// stop := LEN(str);
// FOR i := MAX(pos,1) TO stop DO;
// 	X := pt^[i];
// 	IF X > 31 AND ((setup.EXTENDED_ASCII AND X <> 127) OR (NOT setup.EXTENDED_ASCII AND X < 127)) THEN
// 		FIND_CHAR := i;
// 		RETURN;
// 	END_IF;
// END_FOR;
// FIND_CHAR := 0;
//
//
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION FIND_CTRL : INT
// VAR_INPUT
// 	str : STRING(STRING_LENGTH);
// 	pos : INT;
// END_VAR
// VAR
// 	i: INT;
// 	pt : POINTER TO ARRAY[1..255] OF BYTE;
// 	stop: INT;
// 	x: BYTE;
// END_VAR
//
// pt := ADR(str);
// stop := LEN(str);
// FOR i := MAX(pos,1) TO stop DO;
// 	x := PT^[I];
// 	IF x < 32 OR X = 127 THEN
// 		FIND_CTRL := i;
// 		RETURN;
// 	END_IF;
// END_FOR;
// FIND_CTRL := 0;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION FIND_NONUM : INT
// VAR_INPUT
// 	str : STRING(STRING_LENGTH);
// 	pos : INT;
// END_VAR
// VAR
// 	i: INT;
// 	pt : POINTER TO ARRAY[1..255] OF BYTE;
// 	end: INT;
// 	X: BYTE;
// END_VAR
//
// pt := ADR(str);
// end := LEN(str);
// FOR i := MAX(pos,1) TO end DO;
// 	X := pt^[i];
// 	IF (X < 48 AND X <> 46) OR X > 57 THEN
// 		FIND_NONUM := i;
// 		RETURN;
// 	END_IF;
// END_FOR;
// FIND_NONUM := 0;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION FIND_NUM : INT
// VAR_INPUT
// 	str : STRING(STRING_LENGTH);
// 	pos : INT;
// END_VAR
// VAR
// 	i : INT;
// 	pt : POINTER TO ARRAY[1..255] OF BYTE;
// 	stop : INT;
// 	X: BYTE;
// END_VAR
//
// pt := ADR(str);
// stop := LEN(str);
// FOR i := MAX(pos,1) TO stop DO;
// 	X := pt^[i];
// 	IF (X > 47 AND X < 58) OR X = 46 THEN
// 		FIND_NUM := i;
// 		RETURN;
// 	END_IF;
// END_FOR;
// FIND_NUM := 0;
// END_FUNCTION

FUNCTION FINDB : INT
VAR_INPUT
	str1 : STRING(STRING_LENGTH);
	str2 : STRING(STRING_LENGTH);
END_VAR
VAR
	pos: INT;
	length: INT;
END_VAR

length := LEN(str2);
FOR pos := LEN(str1) - length + 1 TO 1 BY -1 DO
	IF MID(str1,length,pos) = str2 THEN
		FindB := pos;
		RETURN;
	END_IF;
END_FOR;
FindB := 0;
END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION FINDB_NONUM : INT
// VAR_INPUT
// 	str : STRING(STRING_LENGTH);
// END_VAR
// VAR
// 	pos: INT;
// 	pt : POINTER TO BYTE;
// END_VAR
//
// pt := ADR(str) + LEN(str) - 1;
// FOR pos := LEN(str) TO 1 BY -1 DO;
// 	IF (PT^ < 48 AND PT^ <> 46) OR PT^ > 57 THEN
// 		FINDB_NONUM := pos;
// 		RETURN;
// 	END_IF;
// 	PT := PT - 1;
// END_FOR;
// FINDB_NONUM := 0;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION FINDB_NUM : INT
// VAR_INPUT
// 	str : STRING(STRING_LENGTH);
// END_VAR
// VAR
// 	pos: INT;
// 	pt : POINTER TO BYTE;
// END_VAR
//
// pt := ADR(str) + LEN(str) - 1;
// FOR pos := LEN(str) TO 1 BY -1 DO;
// 	IF (PT^ > 47 AND PT^ < 58) OR PT^ = 46 THEN
// 		FINDB_NUM := pos;
// 		RETURN;
// 	END_IF;
// 	PT := PT - 1;
// END_FOR;
// FINDB_NUM := 0;
// END_FUNCTION

FUNCTION FINDP : INT
VAR_INPUT
	str : STRING(STRING_LENGTH);
	src : STRING(STRING_LENGTH);
	pos: INT;
END_VAR
VAR
	i: INT;
	ls: INT;
	lx: INT;
	stp: INT;
END_VAR

ls := LEN(str);
lx := LEN(src);
IF ls < lx OR lx = 0 THEN RETURN; END_IF;
stp := ls - lx + 1;
FOR i := MAX(pos,1) TO stp DO
	IF MID(str,lx,i) = src THEN
		FINDP := i;
		RETURN;
	END_IF;
END_FOR;
FINDP := 0;

(* revision histroy
hm	4. feb. 2008	rev 1.0
	original release

hm	29. feb 2008	rev 1.1
	ADDED MAX(pos,1) in loop initialization

hm	29. mar. 2008	rev 1.2
	changed STRING to STRING(STRING_LENGTH)
*)	

END_FUNCTION

// Commented out for beedance: it uses FILL, which is commented out.
// FUNCTION FIX : STRING(STRING_LENGTH)
// VAR_INPUT
// 	str : STRING(STRING_LENGTH);
// 	L : INT;
// 	C : BYTE;
// 	M : INT;
// END_VAR
// VAR
// 	N : INT;
// 	SX: STRING(STRING_LENGTH);
// END_VAR
//
// (* make sure L does not exceed the limits of a string *)
// N := LIMIT(0,L,STRING_LENGTH) - LEN(str);
// IF N <= 0 THEN
// 	(* truncate the string at length N *)
// 	IF M = 1 THEN
// 		FIX := RIGHT(str,L);
// 	ELSE
// 		FIX := LEFT(str,L);
// 	END_IF;
// ELSIF M = 1 THEN
// 	(* connect fill characters at the beginning *)
// 	sx := FILL(C,N);
// 	FIX := CONCAT(sx,str);
// ELSIF M = 2 THEN
// 	(* center str beween fill characters *)
// 	(* for an uneven number of fill characters, there is one more at the end *)
// 	sx := FILL(C,SHR(N+1,1));
// 	FIX := CONCAT(str,sx);
// 	SX := LEFT(sx,SHR(N,1));
// 	FIX := CONCAT(sx,FIX);
// ELSE
// 	(* connect fill characters at the end *)
// 	SX := FILL(C,N);
// 	FIX := CONCAT(str,SX);
// END_IF;
//
//
// (* revision histroy
// hm	29. mar. 2008	rev 1.0
// 	original release
//
// *)
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION FLOAT_TO_REAL : REAL
// VAR_INPUT
// 	FLT : STRING(20);
// END_VAR
// VAR
// 	pt : POINTER TO ARRAY[1..20] OF BYTE;
// 	i: INT;
// 	X: BYTE;
// 	sign: INT := 1;
// 	stop: INT;
// 	tmp: DINT;
// 	d : INT;
// END_VAR
//
// pt := ADR(FLT);
// stop := LEN(FLT);
//
// (* we first check for sign and exit if first number or dot is reached *)
// FOR i := 1 TO stop DO
// 	X := pt^[i];
// 	IF X > 47 AND X < 58 OR X = 46 THEN
// 		EXIT;
// 	ELSIF X = 45 THEN
// 		(* code 45 is sign *)
// 		sign := -1;
// 	END_IF;
// END_FOR;
//
// (* now we scan numbers till end or dot or E is reached *)
// FOR i := i TO stop DO
// 	X := pt^[i];
// 	IF X = 44 OR X = 46 OR X = 69 OR X = 101 THEN
// 		EXIT;
// 	(* calculate the value of the digit *)
// 	ELSIF X > 47 AND x < 58 THEN
// 		tmp := tmp * 10 + X - 48;
// 	END_IF;
// END_FOR;
//
// (* process the portion after the comma if comma or dot is reached exit if exponent starts *)
// IF x = 44 OR X = 46 THEN
// 	FOR i := i + 1 TO stop DO
// 		X := pt^[i];
// 		IF X = 69 OR X = 101 THEN
// 			EXIT;
// 		ELSIF x > 47 AND x < 58 THEN
// 			tmp := tmp * 10 + X - 48;
// 			d := d - 1;
// 		END_IF;
// 	END_FOR;
// END_IF;
//
// (* process exponent if present *)
// IF X = 69 OR X = 101 THEN
// 	d := d + DEC_TO_INT(RIGHT(FLT, stop - i));
// END_IF;
//
// FLOAT_TO_REAL :=  EXPN(10, d) * DINT_TO_REAL(TMP * SIGN);
//
//
//
//
// (* revision histroy
// hm	22. jun. 2008	rev 1.0
// 	original release
//
// hm	2. oct. 2008	rev 1.1
// 	fixed an error, characters 8 and 9 would not be converted
//
// hm	22. oct. 2008	rev 1.2
// 	last fix was not done correctly
//
// hm	23. oct. 2008	rev 1.3
// 	optimzed code
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses BIN_TO_BYTE, which is commented out.
// FUNCTION FSTRING_TO_BYTE : BYTE
// VAR_INPUT
// 	IN : STRING(12);
// END_VAR
//
// IF LEFT(IN, 2) = '2#' THEN
// 	(* we need to convert binary *)
// 	FSTRING_TO_BYTE := BIN_TO_BYTE(RIGHT(in, LEN(in) - 2));
// ELSIF LEFT(in, 2) = '8#' THEN
// 	(* weneed to convert octals *)
// 	FSTRING_TO_BYTE := OCT_TO_BYTE(RIGHT(in, LEN(in) - 2));
// ELSIF LEFT(in, 3) = '16#' THEN
// 	(* we need to convert hexadecimal *)
// 	FSTRING_TO_BYTE := HEX_TO_BYTE(RIGHT(in, LEN(in) - 3));
// ELSE
// 	(* we assume decimal representation *)
// 	FSTRING_TO_BYTE := DEC_TO_BYTE(CLEAN(in,'0123456789'));
// END_IF;
//
//
// (* revision histroy
// hm	18. jun. 2008	rev 1.0
// 	original release
//
// hm	20. sep. 2008	rev 1.1
// 	changed length of input string from 20 to 12
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses FSTRING_TO_MONTH, which is commented out.
// FUNCTION FSTRING_TO_DT : DT
// VAR_INPUT
// 	SDT : STRING(60);
// 	FMT : STRING(60);
// END_VAR
// VAR CONSTANT
// 	ignore: STRING(1) := '*';  (* ignore character is * *)
// 	fchar : STRING(1) := '#';  (* format character is # *)
// END_VAR
// VAR
// 	c: STRING(1);
// 	tmp: STRING(20);
// 	end: INT;
// 	dy : INT := 1970;
// 	dm : INT := 1;
// 	dd : INT := 1;
// 	th : INT;
// 	tm : INT;
// 	ts : INT;
// END_VAR
// VAR
// 	SDT_work : STRING;
// 	FMT_work : STRING;
// END_VAR
// (* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
// SDT_work := SDT;
// FMT_work := FMT;
//
// (* scan input string *)
// WHILE FMT_work <> '' DO
// 	c := LEFT(FMT_work,1);
// 	IF c = ignore THEN
// 		(* skip ignore characters *)
// 		FMT_work := DELETE(FMT_work,1,1);
// 		SDT_work := DELETE(SDT_work,1,1);
// 	ELSIF C = fchar THEN
// 		(* format chracter found skip format char and read identifier *)
// 		(* store format identifier in c and skip to next char in fmt *)
// 		c := MID(FMT_work,1,2);
// 		FMT_work := DELETE(FMT_work,2,1);
// 		(* extract the substring until the end of sdt or to next char of fmt *)
// 		IF FMT_work = '' THEN
// 			tmp := SDT_work;
// 		ELSE
// 			(* serach for end of substring *)
// 			end := FIND(SDT_work, LEFT(FMT_work,1))-1;
// 			tmp := LEFT(SDT_work, end);
// 			SDT_work := DELETE(SDT_work, end,1);
// 		END_IF;
// 		(* extract information from substring *)
// 		IF c = 'Y' THEN
// 			dy := STRING_TO_INT(tmp);
// 			IF dy < 100 THEN dy := dy + 2000; END_IF;
// 		ELSIF c = 'M' THEN
// 			dm := STRING_TO_INT(tmp);
// 		ELSIF c = 'N' THEN
// 			dm := FSTRING_TO_MONTH(tmp,0);
// 		ELSIF c = 'D' THEN
// 			dd := STRING_TO_INT(tmp);
// 		ELSIF c = 'h' THEN
// 			th := STRING_TO_INT(tmp);
// 		ELSIF c = 'm' THEN
// 			tm := STRING_TO_INT(tmp);
// 		ELSIF c = 's' THEN
// 			ts := STRING_TO_INT(tmp);
// 		END_IF;
// 	ELSIF c = LEFT(SDT_work,1) THEN
// 		(* skip matching characters *)
// 		FMT_work := DELETE(FMT_work,1,1);
// 		SDT_work := DELETE(SDT_work,1,1);
// 	ELSE
// 		RETURN;
// 	END_IF;
// END_WHILE;
//
// FSTRING_TO_DT := SET_DT(dy,dm,dd,th,tm,ts);
//
//
// (* revision histroy
// hm	24. sep. 2008	rev 1.0
// 	original release
//
//
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses BIN_TO_DWORD, which is commented out.
// FUNCTION FSTRING_TO_DWORD : DWORD
// VAR_INPUT
// 	IN : STRING(40);
// END_VAR
//
// IF LEFT(IN, 2) = '2#' THEN
// 	(* we need to convert binary *)
// 	FSTRING_TO_DWORD := BIN_TO_DWORD(RIGHT(in, LEN(in) - 2));
// ELSIF LEFT(in, 2) = '8#' THEN
// 	(* weneed to convert octals *)
// 	FSTRING_TO_DWORD := OCT_TO_DWORD(RIGHT(in, LEN(in) - 2));
// ELSIF LEFT(in, 3) = '16#' THEN
// 	(* we need to convert hexadecimal *)
// 	FSTRING_TO_DWORD := HEX_TO_DWORD(RIGHT(in, LEN(in) - 3));
// ELSE
// 	(* we assume decimal representation *)
// 	FSTRING_TO_DWORD := DEC_TO_DWORD(CLEAN(in,'0123456789'));
// END_IF;
//
//
// (* revision histroy
// hm	18. jun. 2008	rev 1.0
// 	original release
//
// hm	20. sep. 2008	rev 1.1
// 	changed length of input dtring from 20 to 40
//
//
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses CAPITALIZE, which is commented out.
// FUNCTION FSTRING_TO_MONTH : INT
// VAR_INPUT
// 	MTH : STRING(20);
// 	LANG : INT;
// END_VAR
// VAR
// 	lx: INT;
// END_VAR
// VAR
// 	MTH_work : STRING;
// END_VAR
// (* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
// MTH_work := MTH;
//
// IF LANG = 0 THEN lx := LANGUAGE.DEFAULT; ELSE lx := MIN(LANG, LANGUAGE.LMAX); END_IF;
// MTH_work := TRIM(MTH_work);
// MTH_work := CAPITALIZE(LOWERCASE(MTH_work));
//
// FOR FSTRING_TO_MONTH := 1 TO 12 DO
// 	IF MTH_work = language.MONTHS[lx, FSTRING_TO_MONTH] THEN RETURN; END_IF;
// 	IF MTH_work = language.MONTHS3[lx, FSTRING_TO_MONTH] THEN RETURN; END_IF;
// END_FOR;
//
// (* since no name matched try to decode as integer *)
// FSTRING_TO_MONTH := STRING_TO_INT(MTH_work);
//
//
//
//
// (* revision histroy
// hm	25. sep. 2008	rev 1.0
// 	original release
//
// hm	19. oct. 2008	rev 1.1
// 	changed language setup constants
//
// hm	25. oct. 2008	rev 1.2
// 	optimized code
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses FSTRING_TO_WEEKDAY, which is commented out.
// FUNCTION FSTRING_TO_WEEK : BYTE
// VAR_INPUT
// 	WEEK : STRING(60);
// 	LANG : INT;
// END_VAR
// VAR
// 	pos: INT;
// END_VAR
// VAR
// 	WEEK_work : STRING;
// END_VAR
// (* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
// WEEK_work := WEEK;
//
// pos := FIND(WEEK_work, ',');
// WHILE pos > 0 DO
// 	FSTRING_TO_WEEK := FSTRING_TO_WEEK OR SHR(BYTE#128, FSTRING_TO_WEEKDAY(MID(WEEK_work, pos - 1, 1), LANG));
// 	WEEK_work := RIGHT(WEEK_work, LEN(WEEK_work) - pos);
// 	pos := FIND(WEEK_work, ',');
// END_WHILE;
// FSTRING_TO_WEEK := (FSTRING_TO_WEEK OR SHR(BYTE#128, FSTRING_TO_WEEKDAY(WEEK_work, LANG))) AND BYTE#127;
//
//
// (* revision histroy
// hm	18. jun. 2008	rev 1.0
// 	original release
//
// hm	25. oct. 2008	rev 1.1
// 	using language defauls and input lang
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses CAPITALIZE, which is commented out.
// FUNCTION FSTRING_TO_WEEKDAY : INT
// VAR_INPUT
// 	WDAY : STRING(20);
// 	LANG : INT;
// END_VAR
// VAR
// 	tmp: STRING(2);
// 	i: INT;
// 	ly: INT;
// END_VAR
//
// IF LANG = 0 THEN ly := LANGUAGE.DEFAULT; ELSE ly := MIN(LANG, LANGUAGE.LMAX); END_IF;
// (* tmp needs to be calculated first otherwise find can return wrong values *)
// tmp := TRIM(wday);
// tmp := CAPITALIZE(LOWERCASE(tmp));
// FOR i := 1 TO 7 DO
// 	IF language.WEEKDAYS2[ly, i] = tmp THEN
// 		FSTRING_TO_WEEKDAY := i;
// 		RETURN;
// 	END_IF;
// END_FOR;
// FSTRING_TO_WEEKDAY := STRING_TO_INT(WDAY);
//
//
//
//
// (* revision histroy
// hm	18. jun. 2008	rev 1.0
// 	original release
//
// hm	18. jul. 2008	rev 1.1
// 	changed nested call of left(trim()) for compatibility reasons
//
// hm	25. oct. 2008	rev 1.2
// 	using language constants
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION HEX_TO_BYTE : BYTE
// VAR_INPUT
// 	HEX : STRING(5);
// END_VAR
// VAR
// 	pt : POINTER TO BYTE;
// 	i: INT;
// 	X: BYTE;
// 	stop: INT;
// END_VAR
//
// pt := ADR(hex);
// stop := LEN(hex);
// FOR I := 1 TO stop DO
// 	(* read the first character and subtract 48 to get value in decimal 0 = 48 *)
// 	x := pt^;
// 	(* calculate the value of the digit *)
// 	IF X > 47 AND x < 58 THEN
// 		HEX_TO_BYTE := SHL(HEX_TO_BYTE,4) + X - 48;
// 	ELSIF X > 64 AND X < 71 THEN
// 		HEX_TO_BYTE := SHL(HEX_TO_BYTE,4) + X - 55;
// 	ELSIF X > 96 AND X < 103 THEN
// 		HEX_TO_BYTE := SHL(HEX_TO_BYTE,4) + X - 87;
// 	END_IF;
// 	pt := pt + 1;
// END_FOR;
//
//
//
//
// (* revision histroy
// hm	18. jun. 2008	rev 1.0
// 	original release
//
// hm	20. sep.2008	rev 1.1
// 	changed length of input string from 20 to 5
//
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION HEX_TO_DWORD : DWORD
// VAR_INPUT
// 	Hex : STRING(20);
// END_VAR
// VAR
// 	pt : POINTER TO BYTE;
// 	i: INT;
// 	X: BYTE;
// 	stop: INT;
// END_VAR
//
// pt := ADR(hex);
// stop := LEN(hex);
// FOR I := 1 TO stop DO
// 	(* read the first character and subtract 48 to get value in decimal 0 = 48 *)
// 	x := pt^;
// 	(* calculate the value of the digit *)
// 	IF X > 47 AND x < 58 THEN
// 		HEX_TO_DWORD := SHL(HEX_TO_DWORD,4) + X - 48;
// 	ELSIF X > 64 AND X < 71 THEN
// 		HEX_TO_DWORD := SHL(HEX_TO_DWORD,4) + X - 55;
// 	ELSIF X > 96 AND X < 103 THEN
// 		HEX_TO_DWORD := SHL(HEX_TO_DWORD,4) + X - 87;
// 	END_IF;
// 	pt := pt + 1;
// END_FOR;
//
//
//
// (* revision histroy
// hm	2.10.2007		rev 1.0
// 	original release
//
// hm	19.11.2007		rev 1.1
// 	changed type of function from int to dword
//
// hm 	4. mar 2008		rev 1.2
// 	added support for a..f and return 0 for invalid string
//
// hm	29. mar. 2008	rev 1.3
// 	changed STRING to STRING(8)
//
// hm	18. jun. 2008	rev 1.4
// 	changed input hex to STRING(20)
// 	function now ignores wrong characters
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION IS_ALNUM : BOOL
// VAR_INPUT
// 	str : STRING(STRING_LENGTH);
// END_VAR
// VAR
// 	L: INT;
// 	pt : POINTER TO BYTE;
// 	pos: INT;
// END_VAR
//
// PT := ADR(str);
// L := LEN(str);
// FOR pos := 1 TO L DO
// 	IF NOT (ISC_ALPHA(pt^) OR ISC_NUM(pt^)) THEN
// 		IS_ALNUM := FALSE;
// 		RETURN;
// 	END_IF;
//     	PT := PT + 1;
// END_FOR;
// IS_ALNUM := L > 0;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION IS_ALPHA : BOOL
// VAR_INPUT
// 	str : STRING(STRING_LENGTH);
// END_VAR
// VAR
// 	L: INT;
// 	pt : POINTER TO BYTE;
// 	pos: INT;
// END_VAR
//
// PT := ADR(str);
// L := LEN(str);
// FOR pos := 1 TO L DO
// 	IF NOT ISC_ALPHA(pt^) THEN
// 		IS_ALPHA := FALSE;
// 		RETURN;
// 	END_IF;
//     	PT := PT + 1;
// END_FOR;
// IS_ALPHA := L > 0;
// END_FUNCTION

FUNCTION IS_CC : BOOL
VAR_INPUT
	str : STRING(STRING_LENGTH);
	cmp : STRING(STRING_LENGTH);
END_VAR
VAR
	L: INT;
	pos: INT;
END_VAR

L := LEN(str);
FOR pos := 1 TO L DO
	IF FIND(CMP,MID(str,1,pos)) = 0 THEN RETURN; END_IF;
END_FOR;
IS_CC := L > 0;
END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION IS_CTRL : BOOL
// VAR_INPUT
// 	STR : STRING(STRING_LENGTH);
// END_VAR
// VAR
// 	L: INT;
// 	pt : POINTER TO BYTE;
// 	pos: INT;
// END_VAR
//
// PT := ADR(str);
// L := LEN(str);
// FOR pos := 1 TO L DO
// 	IF NOT (ISC_CTRL(pt^)) THEN
// 		IS_CTRL := FALSE;
// 		RETURN;
// 	END_IF;
//     	PT := PT + 1;
// END_FOR;
// IS_CTRL := L > 0;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION IS_HEX : BOOL
// VAR_INPUT
// 	str : STRING(STRING_LENGTH);
// END_VAR
// VAR
// 	L: INT;
// 	pt : POINTER TO BYTE;
// 	pos: INT;
// END_VAR
//
// PT := ADR(str);
// L := LEN(str);
// FOR pos := 1 TO L DO
// 	IF NOT (ISC_HEX(pt^)) THEN
// 		IS_HEX := FALSE;
// 		RETURN;
// 	END_IF;
//     	PT := PT + 1;
// END_FOR;
// IS_HEX := L > 0;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION IS_LOWER : BOOL
// VAR_INPUT
// 	str : STRING(STRING_LENGTH);
// END_VAR
// VAR
// 	l: INT;
// 	pt : POINTER TO BYTE;
// 	pos: INT;
// END_VAR
//
// PT := ADR(str);
// L := LEN(str);
// FOR pos := 1 TO L DO
// 	IF NOT (ISC_LOWER(pt^)) THEN
// 		IS_LOWER := FALSE;
// 		RETURN;
// 	END_IF;
//     	PT := PT + 1;
// END_FOR;
// IS_LOWER := L > 0;
// END_FUNCTION

FUNCTION IS_NCC : BOOL
VAR_INPUT
	str : STRING(STRING_LENGTH);
	cmp : STRING(STRING_LENGTH);
END_VAR
VAR
	L: INT;
	pos: INT;
END_VAR

L := LEN(str);
FOR pos := 1 TO L DO
	IF FIND(CMP,MID(str,1,pos)) > 0 THEN RETURN; END_IF;
END_FOR;
IS_NCC := TRUE;
END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION IS_NUM : BOOL
// VAR_INPUT
// 	str : STRING(STRING_LENGTH);
// END_VAR
// VAR
// 	L : INT;
// 	pt : POINTER TO BYTE;
// 	pos: INT;
// END_VAR
//
// PT := ADR(str);
// L := LEN(str);
// FOR pos := 1 TO L DO
// 	IF NOT (ISC_NUM(pt^)) THEN
// 		IS_NUM := FALSE;
// 		RETURN;
// 	END_IF;
//     	PT := PT + 1;
// END_FOR;
// IS_NUM := L > 0;
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION IS_UPPER : BOOL
// VAR_INPUT
// 	str : STRING(STRING_LENGTH);
// END_VAR
// VAR
// 	L: INT;
// 	pt : POINTER TO BYTE;
// 	pos: INT;
// END_VAR
//
// PT := ADR(str);
// L := LEN(str);
// FOR pos := 1 TO L DO
// 	IF NOT (ISC_UPPER(pt^)) THEN
// 		IS_UPPER := FALSE;
// 		RETURN;
// 	END_IF;
//     	PT := PT + 1;
// END_FOR;
// IS_UPPER := L > 0;
// END_FUNCTION

FUNCTION ISC_ALPHA : BOOL
VAR_INPUT
	IN : BYTE;
END_VAR

IF setup.EXTENDED_ASCII THEN
	ISC_ALPHA := (in > 64 AND in < 91) OR (in > 191  AND in <> 215 AND in <> 247) OR (in > 96 AND in < 123);
ELSE
	ISC_ALPHA := (IN > 64 AND IN < 91) OR (in > 96 AND in < 123);
END_IF;

END_FUNCTION

FUNCTION ISC_CTRL : BOOL
VAR_INPUT
	IN : BYTE;
END_VAR

ISC_CTRL := IN < 32 OR IN = 127;
END_FUNCTION

FUNCTION ISC_HEX : BOOL
VAR_INPUT
	IN : BYTE;
END_VAR

ISC_HEX := (IN > 47 AND IN < 58) OR (IN > 64 AND IN < 71) OR (IN > 96 AND IN < 103);
END_FUNCTION

FUNCTION ISC_LOWER : BOOL
VAR_INPUT
	IN : BYTE;
END_VAR

IF setup.EXTENDED_ASCII THEN
	ISC_LOWER := ((in > 96) AND (in < 123)) OR ((in > 222) AND (in <> 247));
ELSE
	ISC_LOWER := ((in > 96) AND (in < 123));
END_IF;
END_FUNCTION

FUNCTION ISC_NUM : BOOL
VAR_INPUT
	IN : BYTE;
END_VAR

ISC_NUM := IN > 47 AND IN < 58;
END_FUNCTION

FUNCTION ISC_UPPER : BOOL
VAR_INPUT
	IN : BYTE;
END_VAR

IF setup.EXTENDED_ASCII THEN
	ISC_UPPER :=  ((in > 64) AND (in < 91)) OR (((in > 191) AND (in < 223)) AND (in <> 215));
ELSE
	ISC_UPPER := ((in > 64) AND (in < 91));
END_IF;
END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION LOWERCASE : STRING(STRING_LENGTH)
// VAR_INPUT
// 	str : STRING(STRING_LENGTH);
// END_VAR
// VAR
// 	PT : POINTER TO BYTE;
// 	pos: INT;
// 	l: INT;
// END_VAR
//
// PT := ADR(LOWERCASE);
// lowercase := str;
// l := LEN(str);
// FOR pos := 1 TO l DO
// 	pt^ := TO_LOWER(pt^);
// 	PT := PT + 1;
// END_FOR;
//
// (* revision histroy
// hm	6. oct. 2006	rev 1.0
// 	original release
//
// hm	4. feb. 2008	rev 1.1
// 	improved performance
// 	added �.�,�
//
// hm	6. mar. 2008	rev 1.2
// 	added support for exteded Ascii
//
// hm	29. mar. 2008	rev 1.3
// 	changed STRING to STRING(STRING_LENGTH)
// *)
//
// END_FUNCTION

FUNCTION_BLOCK MESSAGE_4R
VAR_INPUT
	M0 : STRING(STRING_LENGTH);
	M1 : STRING(STRING_LENGTH);
	M2 : STRING(STRING_LENGTH);
	M3 : STRING(STRING_LENGTH);
	MM : INT := 3;
	ENQ : BOOL := TRUE;
	CLK : BOOL := TRUE;
	T1 : TIME := T#3s;
END_VAR
VAR_OUTPUT
	MX : STRING(STRING_LENGTH);
	MN : INT;
	TR : BOOL;
END_VAR
VAR
	timer : TON;
	edge: BOOL;
END_VAR


(* check for rising edge on EN *)
TR := FALSE;
IF ENQ THEN
	IF (NOT edge AND clk) OR timer.q THEN
			MN := INC1(MN, MM);
			TR := TRUE;
			timer(in := FALSE);
			CASE MN OF
				0 : MX := M0;
				1 : MX := M1;
				2 : MX := M2;
				3 : MX := M3;
			END_CASE;
	END_IF;
	edge := clk;
	timer( in := CLK, pt := T1);
ELSE
	MX := '';
	MN := 0;
END_IF;

END_FUNCTION_BLOCK

FUNCTION_BLOCK MESSAGE_8
VAR_INPUT
	IN1 : BOOL;
	IN2 : BOOL;
	IN3 : BOOL;
	IN4 : BOOL;
	IN5 : BOOL;
	IN6 : BOOL;
	IN7 : BOOL;
	IN8 : BOOL;
END_VAR
VAR_INPUT CONSTANT
	S1 : STRING(STRING_LENGTH);
	S2 : STRING(STRING_LENGTH);
	S3 : STRING(STRING_LENGTH);
	S4 : STRING(STRING_LENGTH);
	S5 : STRING(STRING_LENGTH);
	S6 : STRING(STRING_LENGTH);
	S7 : STRING(STRING_LENGTH);
	S8 : STRING(STRING_LENGTH);
END_VAR
VAR_OUTPUT
	M : STRING(STRING_LENGTH);
END_VAR


(* check if an alarm is present if yes set the output M otherwise clear M *)
IF in1 THEN
	M := S1;
ELSIF in2 THEN
	M := S2;
ELSIF in3 THEN
	M := S3;
ELSIF in4 THEN
	M := S4;
ELSIF in5 THEN
	M := S5;
ELSIF in6 THEN
	M := S6;
ELSIF in7 THEN
	M := S7;
ELSIF in8 THEN
	M := S8;
ELSE
	M := '';
END_IF;

END_FUNCTION_BLOCK

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION MIRROR : STRING(STRING_LENGTH)
// VAR_INPUT
// 	str : STRING(STRING_LENGTH);
// END_VAR
// VAR
// 	pi : POINTER TO ARRAY[1..255] OF BYTE;
// 	po : POINTER TO BYTE;
// 	lx: INT;
// 	i: INT;
// END_VAR
//
// pi := ADR(str);
// po := ADR(mirror);
// lx := LEN(str);
// FOR i := lx TO 1 BY - 1 DO
// 	po^ := pi^[i];
// 	po := po + 1;
// END_FOR;
// (* close output string *)
// po^:= 0;
//
//
// (* revision histroy
// hm	4. feb. 2008	rev 1.0
// 	original release
//
// hm	29. mar. 2008	rev 1.1
// 	changed STRING to STRING(STRING_LENGTH)
// *)
//
// END_FUNCTION

FUNCTION MONTH_TO_STRING : STRING(10)
VAR_INPUT
	MTH : INT;
	LANG : INT;
	LX : INT;
END_VAR
VAR
	ly : INT;
END_VAR

IF LANG <= 0 THEN Ly := LANGUAGE.DEFAULT; ELSE ly := MIN(LANG, language.LMAX); END_IF;
IF MTH < 1 OR MTH > 12 THEN
	RETURN;
ELSIF LX = 0 THEN
	MONTH_TO_STRING := language.MONTHS[ly, MTH];
ELSIF Lx = 3 THEN
	MONTH_TO_STRING := language.MONTHS3[ly, MTH];
END_IF;


(* revisison history

hm	21. sep. 2008	rev 1.0
	original version

hm	19. oct. 2008	rev 1.1
	changed language setup constants

hm 16.  jul 2024   rev 1.2
       addaed boundary check to respect array limits
*)
END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION OCT_TO_BYTE : BYTE
// VAR_INPUT
// 	OCT : STRING(10);
// END_VAR
// VAR
// 	pt : POINTER TO BYTE;
// 	i: INT;
// 	X: BYTE;
// 	stop: INT;
// END_VAR
//
// pt := ADR(oct);
// stop := LEN(oct);
// FOR I := 1 TO stop DO
// 	(* read the first character and subtract 48 to get value in decimal 0 = 48 *)
// 	x := pt^;
// 	(* calculate the value of the digit *)
// 	IF X > 47 AND x < 56 THEN
// 		OCT_TO_BYTE := SHL(OCT_TO_BYTE,3) + X - 48;
// 	END_IF;
// 	pt := pt + 1;
// END_FOR;
//
//
//
// (* revision histroy
// hm	18. jun. 2008	rev 1.0
// 	original release
//
// hm	20. sep. 2008	rev 1.1
// 	changed length of input string from 20 to 10
//
// *)
//
// END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION OCT_TO_DWORD : DWORD
// VAR_INPUT
// 	OCT : STRING(20);
// END_VAR
// VAR
// 	pt : POINTER TO BYTE;
// 	i: INT;
// 	X: BYTE;
// 	stop: INT;
// END_VAR
//
// pt := ADR(oct);
// stop := LEN(oct);
// FOR I := 1 TO stop DO
// 	(* read the first character and subtract 48 to get value in decimal 0 = 48 *)
// 	x := pt^;
// 	(* calculate the value of the digit *)
// 	IF X > 47 AND x < 56 THEN
// 		OCT_TO_DWORD := SHL(OCT_TO_DWORD,3) + X - 48;
// 	END_IF;
// 	pt := pt + 1;
// END_FOR;
//
//
//
//
// (* revision histroy
// hm	18. jun. 2008	rev 1.0
// 	original release
//
//
// *)
//
// END_FUNCTION

FUNCTION REAL_TO_STRF : STRING(20)
VAR_INPUT
	IN : REAL;
	N : INT;
	D : STRING(1);
END_VAR
VAR
	O: REAL;
	i: INT;
END_VAR
VAR
	N_work : INT;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
N_work := N;



(* LIMIT N to 0 .. 7 *)
N_work := LIMIT(0,N_work,7);
(* round the input to N digits and convert to string *)
O := ABS(in) * EXP10(N_work);
REAL_TO_STRF := DINT_TO_STRING(REAL_TO_DINT(O));
(* add zeroes in front to make sure sting is at least 8 digits long *)
FOR i := LEN(REAL_TO_STRF) TO N_work DO REAL_TO_STRF := CONCAT('0', REAL_TO_STRF); END_FOR;
(* add a dot if n > 0 *)
IF N_work > 0 THEN REAL_TO_STRF := INSERT(REAL_TO_STRF, D, LEN(REAL_TO_STRF) - N_work); END_IF;
(* add a minus sign if in is negative *)
IF in < 0 THEN REAL_TO_STRF := CONCAT('-', REAL_TO_STRF); END_IF;

END_FUNCTION

FUNCTION REPLACE_ALL : STRING(STRING_LENGTH)
VAR_INPUT
	str : STRING(STRING_LENGTH);
	src : STRING(STRING_LENGTH);
	rep : STRING(STRING_LENGTH);
END_VAR
VAR
	pos: INT;
	lp : INT;
	lx : INT;
END_VAR

REPLACE_ALL := str;
lx := LEN(src);
lp := LEN(rep);
pos := FINDP(REPLACE_ALL, src, 1);
WHILE pos > 0 DO
	REPLACE_ALL := REPLACE(REPLACE_ALL,rep,lx, pos);
	pos := FINDP(REPLACE_ALL, src, pos + lp);
END_WHILE;

(* revision histroy
hm	4. feb. 2008	rev 1.0
	original release

hm	29. mar. 2008	rev 1.1
	changed STRING to STRING(STRING_LENGTH)
*)	

END_FUNCTION

FUNCTION REPLACE_CHARS : STRING(STRING_LENGTH)
VAR_INPUT
	STR : STRING(STRING_LENGTH);
	SRC : STRING;
	REP : STRING;
END_VAR
VAR
	a, b : INT;
	c: STRING(1);
	stp: INT;
END_VAR
VAR
	SRC_work : STRING;
	REP_work : STRING;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
SRC_work := SRC;
REP_work := REP;

REPLACE_CHARS := STR;
(* make sure rep and src are of same length and length is > 0 *)
a := LEN(SRC_work);
b := LEN(REP_work);
IF a < b THEN
	REP_work := LEFT(REP_work, a);
ELSIF b < a THEN
	SRC_work := LEFT(SRC_work, b);
END_IF;

(* search the string and replace if necessary *)
stp := LEN(str);
FOR a := 1 TO stp DO
	c := MID(REPLACE_CHARS, 1, a);
	b := FIND(SRC_work, c);
	IF b > 0 THEN
		REPLACE_CHARS := REPLACE(REPLACE_CHARS, MID(REP_work, 1, b), 1, a);
	END_IF;
END_FOR;

END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION REPLACE_UML : STRING(STRING_LENGTH)
// VAR_INPUT
// 	str : STRING(STRING_LENGTH);
// END_VAR
// VAR
// 	L : INT;
// 	pt : POINTER TO BYTE;
// 	pto : POINTER TO BYTE;
// 	ptm : POINTER TO BYTE;
// 	pt1, pt2 : POINTER TO BYTE;
// 	su : STRING(2);
// 	pos : INT;
// END_VAR
//
// PT := ADR(str);
// pto := ADR(REPLACE_UML);
// ptm := pto + INT_TO_DWORD(string_length);
// pt1 := ADR(su);
// pt2 := pt1 + 1;
// L := LEN(str);
// WHILE pos < L AND pos < string_length DO
// 	IF pt^ < 127 THEN
// 		(* no uml character simlply copy the character*)
// 		pto^ := pt^;
// 	ELSE
// 		(* convert the uml character *)
// 		su := TO_UML(pt^);
// 		(* we must make sure pointer are not out of range *)
// 		pto^ := pt1^;
// 		IF pto < ptm AND pt2^ > 0 THEN
// 			pto := pto + 1;
// 			pto^ := pt2^;
// 		END_IF;
// 	END_IF;
// 	(* increment pointers *)
// 	pt := pt + 1;
// 	pto := pto + 1;
// 	pos := pos + 1;
// END_WHILE;
//
// (* properly close the output string *)
// pto^ := 0;
// END_FUNCTION

FUNCTION_BLOCK TICKER

VAR_INPUT
	N : INT;
	PT : TIME;
END_VAR

VAR_IN_OUT
	Text : STRING(STRING_LENGTH);
END_VAR

VAR_OUTPUT
	Display : STRING(STRING_LENGTH);
END_VAR
VAR
	delay : TP;
	step : INT;
END_VAR


(* generate next ticker when delay is low *)
IF N < LEN(text) THEN
	IF NOT delay.Q THEN
		(* increase step for next tick *)
		step := step + 1;
		IF step > LEN(text) THEN step := 1; END_IF;
		(* extract dispay from text *)
		display := MID(text, N, step);
		(* set delay timer for next tick *)
		delay(in := TRUE, PT := PT);
	ELSE;
		(* execute delay timer *)
		delay(in := FALSE);
	END_IF;
ELSE
	display := text;
END_IF;
END_FUNCTION_BLOCK

FUNCTION TO_LOWER : BYTE
VAR_INPUT
	IN : BYTE;
END_VAR

IF in > 64 AND in < 91 THEN
   TO_LOWER := in OR 16#20;
ELSIF (in > 191 AND in < 223) AND in <> 215 AND setup.EXTENDED_ASCII THEN
   TO_LOWER := in OR 16#20;
ELSE
   TO_LOWER := in;
END_IF;
END_FUNCTION

// Commented out for beedance: it uses CHR_TO_STRING, which is commented out.
// FUNCTION TO_UML : STRING(2)
// VAR_INPUT
// 	IN : BYTE;
// END_VAR
//
// CASE in OF
// 	196:	(* � *)
// 		TO_UML := 'Ae';
// 	214:	(* � *)
// 		TO_UML := 'Oe';
// 	220:	(* � *)
// 		TO_UML := 'Ue';
// 	223:	(* � *)
// 		TO_UML := 'ss';
// 	228:	(* � *)
// 		TO_UML := 'ae';
// 	246:	(* � *)
// 		TO_UML := 'oe';
// 	252:	(* � *)
// 		TO_UML := 'ue';
// ELSE
// 	TO_UML := CHR_TO_STRING(IN);
// END_CASE;
// END_FUNCTION

FUNCTION TO_UPPER : BYTE
VAR_INPUT
	IN : BYTE;
END_VAR

IF in > 96 AND in < 123 THEN
   TO_UPPER := in AND 16#DF;
ELSIF in > 223 AND in <> 247 AND in <> 255 AND setup.EXTENDED_ASCII THEN
   TO_UPPER := in AND 16#DF;
ELSE
   TO_UPPER := in;
END_IF;
END_FUNCTION

FUNCTION TRIM : STRING(STRING_LENGTH)
VAR_INPUT
	str : STRING(STRING_LENGTH);
END_VAR
VAR
	pos: INT;
END_VAR

TRIM := str;
REPEAT
	pos := FIND(trim,' ');
	IF pos > 0 THEN TRIM := REPLACE(TRIM,'',1,pos); END_IF;
UNTIL pos = 0	END_REPEAT;


(* revision histroy
hm	6.10.2006		rev 1.0
	original release

hm	20. mar. 2008	rev 1.1
	avoid to call replace with pos = 0 because some systems will produce an error

hm	29. mar. 2008	rev 1.2
	changed STRING to STRING(STRING_LENGTH)
*)
END_FUNCTION

FUNCTION TRIM1 : STRING(STRING_LENGTH)
VAR_INPUT
	str : STRING(STRING_LENGTH);
END_VAR
VAR
	pos: INT;
END_VAR

TRIM1 := str;
REPEAT
	pos := FIND(trim1,'  ');
	IF pos > 0 THEN TRIM1 := REPLACE(TRIM1,' ',2,pos); END_IF;
UNTIL pos = 0	END_REPEAT;
(* beginning and ending blanks need to be stripped off *)
IF LEFT(trim1,1) = ' ' THEN trim1 := DELETE(trim1,1,1); END_IF;
IF RIGHT(trim1,1) = ' ' THEN trim1 := DELETE(trim1,1,LEN(trim1)); END_IF;


(* revision histroy
hm	4. feb. 2008    rev 1.0
	original release

hm	20. mar. 2008	rev 1.1
	avoid to call replace with pos = 0 because some systems will produce an error

hm	29. mar. 2008	rev 1.2
	changed STRING to STRING(STRING_LENGTH)
*)
END_FUNCTION

FUNCTION TRIME : STRING(string_length)
VAR_INPUT
	str : STRING(STRING_LENGTH);
END_VAR

TRIME := str;
(* beginning blanks need to be stripped off *)
WHILE LEFT(TRIME,1) = ' ' DO
	TRIME := DELETE(TRIME,1,1);
END_WHILE;

(* ending blanks need to be stripped off *)
WHILE RIGHT(TRIME,1) = ' ' DO
	TRIME := DELETE(TRIME,1,LEN(TRIME));
END_WHILE;


(* revision histroy
hm	28. mar. 2008	rev 1.0
	original release

*)
END_FUNCTION

// Commented out for beedance: it uses POINTER TO or ADR, which IEC 61131-3 does not have.
// FUNCTION UPPERCASE : STRING(STRING_LENGTH)
// VAR_INPUT
// 	str : STRING(STRING_LENGTH);
// END_VAR
// VAR
// 	pt : POINTER TO BYTE;
// 	pos: INT;
// 	l: INT;
// END_VAR
//
// pt := ADR(UPPERCASE);
// UPPERCASE := str;
// l := LEN(str);
// FOR pos := 1 TO l DO
// 	pt^ := TO_UPPER(pt^);
// 	pt := pt + 1;
// END_FOR;
//
// (* revision histroy
// hm	6. oct. 2006	rev 1.0
// 	original release
//
// hm	4. feb. 2008	rev 1.1
// 	improved performance
// 	added �,�,�
//
// hm	6. mar. 2008	rev 1.2
// 	added support for exteded Ascii
//
// hm	29. mar. 2008	rev 1.3
// 	changed STRING to STRING(STRING_LENGTH)
//
// *)
//
// END_FUNCTION

FUNCTION WEEKDAY_TO_STRING : STRING(10)
VAR_INPUT
	WDAY : INT;
	LANG : INT;
	LX : INT;
END_VAR
VAR
	ly : INT;
END_VAR

IF LANG = 0 THEN ly := LANGUAGE.DEFAULT; ELSE ly := MIN(LANG, LANGUAGE.LMAX); END_IF;
IF wday < 1 OR wday > 7 THEN
	RETURN;
ELSIF LX = 0 THEN
	WEEKDAY_TO_STRING := language.WEEKDAYS[ly, WDAY];
ELSIF Lx = 2 THEN
	WEEKDAY_TO_STRING := language.WEEKDAYS2[ly, WDAY];
END_IF;


(* revisison history

hm	21. sep. 2008	rev 1.0
	original version

hm	19. oct. 2008	rev 1.1
	changed language setup constants
*)
END_FUNCTION

FUNCTION_BLOCK CALENDAR_CALC
VAR_INPUT
	SPE : BOOL;
	H : REAL := -0.83333333333;
END_VAR
VAR_IN_OUT
	XCAL : CALENDAR;
	HOLIDAYS : ARRAY[0..29] OF HOLIDAY_DATA;
END_VAR
VAR
	last : DT;
	last_day : DINT;
	holy : HOLIDAY;
	sun : SUN_TIME;
	last_hour: INT;
	utod: TOD;
	pos : SUN_POS;
	plast: DT;
END_VAR
VAR_TEMP
	dtemp : DINT;
	tmp : INT;
END_VAR



IF xcal.UTC <> last THEN
	(* run once per second *)
	(* update utc last calculated  *)
	last := XCAL.UTC;
	utod := DT_TO_TOD(xcal.UTC);

	(* calculate ltc from utc *)
	XCAL.LOCAL_DT := UTC_TO_LTIME(XCAL.UTC, XCAL.DST_EN, XCAL.OFFSET);
	XCAL.LOCAL_DATE := DT_TO_DATE(XCAL.LOCAL_DT);
	XCAL.LOCAL_TOD := DT_TO_TOD(XCAL.LOCAL_DT);
	dtemp := DAY_OF_DATE(XCAL.LOCAL_DATE);
	xcal.night := XCAL.LOCAL_TOD < XCAL.SUN_RISE OR XCAL.LOCAL_TOD > XCAL.SUN_SET;

	(* run once per hour *)
	tmp := HOUR(xcal.LOCAL_TOD);
	IF  tmp <> last_hour THEN
		XCAL.DST_ON := DST(XCAL.UTC) AND xcal.DST_EN;
		last_hour := tmp;
	END_IF;

	(* run once per day *)
	IF dtemp <> last_day THEN
		last_day := dtemp;
		(* a new day has started, recalculate daily events *)
		XCAL.YEAR := YEAR_OF_DATE(XCAL.LOCAL_DATE);
		XCAL.MONTH := MONTH_OF_DATE(XCAL.LOCAL_DATE);
		XCAL.DAY := DAY_OF_MONTH(XCAL.LOCAL_DATE);
		XCAL.WEEKDAY := DAY_OF_WEEK(XCAL.LOCAL_DATE);
		HOLY(date_in := XCAL.LOCAL_DATE, LANGU := xcal.LANGUAGE, HOLIDAYS := HOLIDAYS);
		XCAL.HOLIDAY := HOLY.Y;
		XCAL.HOLY_NAME := HOLY.NAME;
		sun(latitude := XCAL.LATITUDE, longitude := xcal.LONGITUDE, utc := DT_TO_DATE(xcal.UTC), H := H);
		XCAL.SUN_RISE := DINT_TO_TOD(TOD_TO_DINT(sun.sun_rise) + XCAL.OFFSET * 60000 + SEL(XCAL.DST_ON,DINT#0,3600000));
		XCAL.SUN_SET := DINT_TO_TOD(TOD_TO_DINT(sun.sun_set) + XCAL.OFFSET * 60000 + SEL(XCAL.DST_ON,DINT#0,3600000));
		XCAL.SUN_MIDDAY := DINT_TO_TOD(TOD_TO_DINT(sun.MIDDAY) + XCAL.OFFSET * 60000 + SEL(XCAL.DST_ON,DINT#0,3600000));
		XCAL.SUN_HEIGTH := sun.sun_declination;
		XCAL.WORK_WEEK := WORK_WEEK(XCAL.LOCAL_DATE);
	END_IF;

	(* calculate the suns position every 10 seconds when SPE = TRUE *)
	IF SPE AND xcal.UTC -  plast >= t#25s THEN
		plast := last;
		pos(latitude := xcal.LATITUDE, longitude := xcal.LONGITUDE, utc := xcal.UTC);
		xcal.SUN_HOR := pos.B;
		xcal.SUN_VER := pos.HR;
	END_IF;
END_IF;
END_FUNCTION_BLOCK

FUNCTION DATE_ADD : DATE
VAR_INPUT
	IDATE : DATE;
	D : INT;
	W : INT;
	M : INT;
	Y : INT;
END_VAR
VAR
	mo: INT;
	yr : INT;
	dm: INT;
END_VAR


DATE_ADD := UDINT_TO_DATE(DATE_TO_UDINT(IDATE) + INT_TO_UDINT(D + W * 7) * UDINT#86400);
yr := Y + YEAR_OF_DATE(DATE_ADD);
mo := M + MONTH_OF_DATE(DATE_ADD);
dm := DAY_OF_MONTH(DATE_ADD);
WHILE mo > 12 DO
	mo := mo - 12;
	yr := yr + 1;
END_WHILE;
WHILE mo < 1 DO
	mo := mo + 12;
	yr := yr - 1;
END_WHILE;
DATE_ADD := SET_DATE(yr, mo, dm);
END_FUNCTION

FUNCTION DAY_OF_DATE : DINT
VAR_INPUT
	idate : DATE;
END_VAR

DAY_OF_DATE := UDINT_TO_DINT(DATE_TO_UDINT(idate) / 86400);
END_FUNCTION

FUNCTION DAY_OF_MONTH : INT
VAR_INPUT
	IDATE : DATE;
END_VAR
VAR
	leap: INT;
END_VAR

(* calculate the day in the year *)
DAY_OF_MONTH := DAY_OF_YEAR(idate);
(* leap will be set to one for a leap year *)
leap := BOOL_TO_INT(LEAP_OF_DATE(idate));
(* if leap year deduct one from the days of the year *)
DAY_OF_MONTH := DAY_OF_MONTH - leap;
(* search if we are in month december to march ? *)
IF DAY_OF_MONTH > setup.MTH_OFS[9] THEN
	IF DAY_OF_MONTH > setup.MTH_OFS[11] THEN
		IF DAY_OF_MONTH > setup.mth_ofs[12] THEN
			DAY_OF_MONTH := DAY_OF_MONTH - setup.MTH_OFS[12];
		ELSE
			DAY_OF_MONTH := DAY_OF_MONTH - setup.MTH_OFS[11];
		END_IF;
	ELSE
		IF DAY_OF_MONTH > setup.mth_ofs[10] THEN
			DAY_OF_MONTH := DAY_OF_MONTH - setup.MTH_OFS[10];
		ELSE
			DAY_OF_MONTH := DAY_OF_MONTH - setup.MTH_OFS[9];
		END_IF;
	END_IF;
ELSIF DAY_OF_MONTH > setup.MTH_OFS[5] THEN
	IF DAY_OF_MONTH > setup.MTH_OFS[7] THEN
		IF DAY_OF_MONTH > setup.mth_ofs[8] THEN
			DAY_OF_MONTH := DAY_OF_MONTH - setup.MTH_OFS[8];
		ELSE
			DAY_OF_MONTH := DAY_OF_MONTH - setup.MTH_OFS[7];
		END_IF;
	ELSE
		IF DAY_OF_MONTH > setup.mth_ofs[6] THEN
			DAY_OF_MONTH := DAY_OF_MONTH - setup.MTH_OFS[6];
		ELSE
			DAY_OF_MONTH := DAY_OF_MONTH - setup.MTH_OFS[5];
		END_IF;
	END_IF;
ELSIF DAY_OF_MONTH > setup.MTH_OFS[3] THEN
	IF DAY_OF_MONTH > setup.MTH_OFS[4] THEN
		DAY_OF_MONTH := DAY_OF_MONTH - setup.MTH_OFS[4];
	ELSE
		DAY_OF_MONTH := DAY_OF_MONTH - setup.MTH_OFS[3];
	END_IF;
ELSE
	(* since now we must be in february or january we need to add leap again *)
	DAY_OF_MONTH := DAY_OF_MONTH + leap;
	IF DAY_OF_MONTH > setup.MTH_OFS[2] THEN DAY_OF_MONTH := DAY_OF_MONTH - setup.mth_ofs[2]; END_IF;
	(* since nothing was true before, day_of_month must already be good *)
END_IF;


END_FUNCTION

FUNCTION DAY_OF_WEEK : INT
VAR_INPUT
	IDATE : DATE;
END_VAR

DAY_OF_WEEK := UDINT_TO_INT((DATE_TO_UDINT(idate) / 86400 + 3) MOD 7) + 1;




END_FUNCTION

FUNCTION DAY_OF_YEAR : INT
VAR_INPUT
	IDATE : DATE;
END_VAR

DAY_OF_YEAR := UDINT_TO_INT((DATE_TO_UDINT(idate) / UDINT#86400) MOD UDINT#1461);
IF DAY_OF_YEAR > 729 THEN
	IF DAY_OF_YEAR > 1095 THEN DAY_OF_YEAR := DAY_OF_YEAR - 1095; ELSE DAY_OF_YEAR := DAY_OF_YEAR - 729; END_IF;
ELSIF DAY_OF_YEAR > 364 THEN
	DAY_OF_YEAR := DAY_OF_YEAR - 364;
ELSE
	DAY_OF_YEAR := DAY_OF_YEAR + 1;
END_IF;


(*
DAY_OF_YEAR := DWORD_TO_INT((DATE_TO_DWORD(idate) - DATE_TO_DWORD(YEAR_BEGIN(YEAR_OF_DATE(idate)))) / 86400) + 1;
*)


(* revivision history
hm	4. aug. 2007		rev 1.0
	original version

hm	1. oct. 2007		rev 1.1
	added compatibility to STEP7

hm	4. jan. 2008		rev 1.2
	changed code for better performance

hm	7. oct. 2008		rev 1.3
	changed name of function year to year_of_date

hm	28. jan. 2011	rev 1.4
	improved code
*)
END_FUNCTION

FUNCTION DAY_TO_TIME : TIME
VAR_INPUT
	IN : REAL;
END_VAR

DAY_TO_TIME := DWORD_TO_TIME(REAL_TO_DWORD(IN * 86400000.0));
END_FUNCTION

FUNCTION DAYS_DELTA : DINT
VAR_INPUT
	date_1 : DATE;
	date_2 : DATE;
END_VAR

IF DATE_1 > DATE_2 THEN
	DAYS_DELTA := - UDINT_TO_DINT((DATE_TO_UDINT(date_1) - DATE_TO_UDINT(date_2)) / 86400);
ELSE
	DAYS_DELTA := UDINT_TO_DINT((DATE_TO_UDINT(date_2) - DATE_TO_UDINT(date_1)) / 86400);
END_IF;
END_FUNCTION

FUNCTION DAYS_IN_MONTH : INT
VAR_INPUT
	IDATE : DATE;
END_VAR


DAYS_IN_MONTH := DAY_OF_YEAR(IDATE);
IF LEAP_OF_DATE(IDATE) THEN
	CASE DAYS_IN_MONTH OF
		32..60	:	DAYS_IN_MONTH := 29;
		92..121 :	DAYS_IN_MONTH := 30;
		153..182:	DAYS_IN_MONTH := 30;
		245..274:	DAYS_IN_MONTH := 30;
		306..335:	DAYS_IN_MONTH := 30;
	ELSE
		DAYS_IN_MONTH := 31;
	END_CASE;
ELSE
	CASE DAYS_IN_MONTH OF
		32..59	:	DAYS_IN_MONTH := 28;
		91..120 :	DAYS_IN_MONTH := 30;
		152..181:	DAYS_IN_MONTH := 30;
		244..273:	DAYS_IN_MONTH := 30;
		305..334:	DAYS_IN_MONTH := 30;
	ELSE
		DAYS_IN_MONTH := 31;
	END_CASE;
END_IF;
END_FUNCTION

FUNCTION DAYS_IN_YEAR : INT
VAR_INPUT
	IDATE : DATE;
END_VAR


IF LEAP_OF_DATE(IDATE) THEN
	DAYS_IN_YEAR := 366;
ELSE
	DAYS_IN_YEAR := 365;
END_IF;
END_FUNCTION

FUNCTION_BLOCK DCF77
VAR_INPUT
	REC : BOOL;
	SET : BOOL;
	SDT : DT;
	DSI : BOOL;
END_VAR
VAR_INPUT CONSTANT
	SYNC_TIMEOUT : TIME := t#2m;
	TIME_OFFSET : INT := 1;
	DST_EN : BOOL := TRUE;
END_VAR
VAR_OUTPUT
	TP : BOOL;
	DS : BOOL;
	WDAY : INT;
	ERROR : BOOL := TRUE;
	RTC : DT;
	RTC1 : DT;
	MSEC : INT;
	SYNC : BOOL;
END_VAR
VAR
	mez : DT;
	utc : DT;
	state : INT;
	edge: BOOL;
	tx : TIME;
	ty: TIME;
	last : TIME;
	bits : ARRAY[0..58] OF BOOL;
	cnt : INT;
	i : INT;
	old_time : DT;
	minute : INT;
	hour : INT;
	day : INT;
	month : INT;
	year : INT;
	last_sync: TIME;
	t1: TIME;
	tz: TIME;
	init: BOOL;
END_VAR


(* if tp was set it should only be active for 1 cycle *)
TP := FALSE;

(* read system time *)
t1 := DWORD_TO_TIME(T_PLC_MS());
tx := t1 - last;

IF rec XOR edge THEN
	edge := rec;
	IF NOT rec AND tx > t#1700ms AND tx < t#2000ms THEN
		(* start condition reached *)
		state := 0;
		tp := NOT error;
	ELSIF NOT rec AND tx > t#700ms AND tx < t#1000ms THEN
		(* second switch detected *)
		IF state < 58 THEN state := state +1; ELSE state := 0; END_IF;
	ELSIF rec AND tx  < t#120ms THEN
		(* bit 0 detected  *)
		bits[state] := 0;
	ELSIF rec AND tx > t#120ms AND tx < t#250ms THEN
		(* bit 1 detected *)
		bits[state] := 1;
	ELSE
		(* error condition received signal is not valid *)
		error := TRUE;
		state := 0;
	END_IF;
	last := last + tx;
	IF rec AND state = 58 THEN
		error := FALSE;

		(* decode the bits and check for possible errors *)
		IF bits[0] OR NOT (bits[17] XOR bits[18]) OR NOT bits[20] THEN error := TRUE; END_IF;
		(* decode minute *)
		MINUTE := 0;
		MINUTE.0 := bits[21];
		MINUTE.1 := bits[22];
		MINUTE.2 := bits[23];
		MINUTE.3 := bits[24];
		IF bits[25] THEN MINUTE := MINUTE + 10; END_IF;
		IF bits[26] THEN MINUTE := MINUTE + 20; END_IF;
		IF bits[27] THEN MINUTE := MINUTE + 40; END_IF;
		IF MINUTE > 59 OR (bits[21] XOR bits[22] XOR bits[23] XOR bits[24] XOR bits[25] XOR bits[26] XOR bits[27] XOR bits[28]) THEN error := TRUE; END_IF;

		(* decode hour *)
		HOUR := 0;
		HOUR.0 := bits[29];
		HOUR.1 := bits[30];
		HOUR.2 := bits[31];
		HOUR.3 := bits[32];
		IF bits[33] THEN HOUR := HOUR + 10; END_IF;
		IF bits[34] THEN HOUR := HOUR +20; END_IF;
		IF HOUR > 23 OR (bits[29] XOR bits[30] XOR bits[31] XOR bits[32] XOR bits[33] XOR bits[34] XOR bits[35]) THEN error := TRUE; END_IF;

		(* decode day of month *)
		day := 0;
		day.0 := bits[36];
		day.1 := bits[37];
		day.2 := bits[38];
		day.3 := bits[39];
		IF bits[40] THEN day := day + 10; END_IF;
		IF bits[41] THEN day := day + 20; END_IF;
		IF day > 31 THEN error := TRUE; END_IF;

		(* decode day of week *)
		wday := 0;
		wday.0 := bits[42];
		wday.1 := bits[43];
		wday.2 := bits[44];
		IF wday > 7 OR wday < 1 THEN error := TRUE; END_IF;

		(* decode month *)
		MONTH := 0;
		MONTH.0 := bits[45];
		MONTH.1 := bits[46];
		MONTH.2 := bits[47];
		MONTH.3 := bits[48];
		IF bits[49] THEN MONTH := MONTH +10; END_IF;
		IF MONTH > 12 THEN error := TRUE; END_IF;

		(* decode year *)
		YEAR := 0;
		YEAR.0 := bits[50];
		YEAR.1 := bits[51];
		YEAR.2 := bits[52];
		YEAR.3 := bits[53];
		IF bits[54] THEN YEAR := YEAR + 10; END_IF;
		IF bits[55] THEN YEAR := YEAR + 20; END_IF;
		IF bits[56] THEN YEAR := YEAR + 40; END_IF;
		IF bits[57] THEN YEAR := YEAR + 80; END_IF;

		(* check parity for bits 36 to 58 *)
		cnt := 0;
		FOR i := 36 TO 58 DO IF bits[i] THEN cnt := cnt + 1; END_IF; END_FOR;
		IF NOT EVEN(cnt) THEN error := TRUE; END_IF;

		(* time must be valid for two cycles to clear error flag *)
		IF NOT error THEN
			(* set outputs *)
			old_time := mez;
			IF YEAR >= 70 THEN YEAR := YEAR + 1900; ELSE YEAR := YEAR + 2000; END_IF;
			mez := SET_DT(YEAR,MONTH,day,HOUR,MINUTE,0);
			DS := bits[17];
			IF DS THEN
				UTC := UDINT_TO_DT(DT_TO_UDINT(mez) - 7200);
			ELSE
				UTC := UDINT_TO_DT(DT_TO_UDINT(mez) - 3600);
			END_IF;

			(* set trigger signal only if the receiver has received 2 successive minutes *)
			IF mez <> old_time + t#1m THEN error := TRUE ; END_IF;
		END_IF;
	END_IF;
END_IF;

(* this portion implements a free running clock which is triggered by the dcf77 signal *)
tz := UDINT_TO_TIME(INT_TO_UDINT(ABS(time_offset)) * 3600000);

(* input sdt is copied to utc at first power up *)
IF NOT init OR SET THEN
	init := TRUE;
	utc := sdt;
	tp := TRUE;
	DS := DSI;
END_IF;

IF tp THEN
	rtc := utc;
	IF DS AND dst_en THEN
		IF time_offset < 0 THEN	rtc1 := rtc - tz + t#1h; ELSE rtc1 := rtc + tz + t#1h; END_IF;
	ELSE
		IF time_offset < 0 THEN rtc1 := rtc - tz; ELSE rtc1 := rtc + tz; END_IF;
	END_IF;
	sync := TRUE;
	last_sync := last;
	ty := last;
ELSIF rtc > DWORD_TO_DT(0) AND T1 - ty >= t#1s THEN
	rtc := rtc + t#1s;
	rtc1 := rtc1 + t#1s;
	ty := ty + t#1s;
	sync := ty - last_sync < sync_timeout AND last_sync > DWORD_TO_TIME(0);
	wday := DAY_OF_WEEK(DT_TO_DATE(rtc1));
	DS := dst_en AND DST(utc);
END_IF;
msec := TIME_TO_INT(t1 - ty);



(* decode information
bits		content
0			Bitwert immer 0
1 bis 14	reserviert f�r Betriebsinformationen (nicht f�r DCF77-Nutzer bestimmt)
15			Rufbit f�r Alarmierung der PTB-Mitarbeiter
			(bis Mai 2003: Bitwert = 0 falls normale Antenne in Verwendung; 1 = Backupantenne)
16			Bitwert = 1 falls ein Wechsel von MEZ nach MESZ oder umgekehrt bevorsteht; Dauer der Anzeige: 1 Stunde
17 + 18	g�ltige Zeit = MEZ, falls Bit 17=0 und Bit 18=1
			g�ltige Zeit = MESZ, falls Bit 17=1 und Bit 18=0
19			Bitwert = 1 falls innerhalb den n�chsten 59 Minuten eine Schaltsekunde angeordnet ist. Beim Einf�gen einer Schaltsekunde wird anstelle der 59. die 60. Sekundenmarke weggelassen und in der 58. erfolgt ausnahmsweise ein Tr�gerabfall.
20			Startbit f�r Zeitinformation (immer 1)
21 - 27		1, 2, 4, 8, 10, 20, 40 Minuten (bitweise Addition)
28			Pr�fbit (gerade Parit�t) f�r die Bits 21-27
29 - 34		1, 2, 4, 8, 10, 20 Stunden (bitweise Addition)
35			Pr�fbit (gerade Parit�t) f�r die Bits 29-34
36 - 41		Tagesnummer im aktuellen Monat: 1, 2, 4, 8, 10, 20 (bitweise Addition)
42 - 44		Tagesnummer in der aktuellen Woche: 1, 2, 4 (bitweise Addition)
45 - 49		Monatsnummer: 1, 2, 4, 8, 10 (bitweise Addition)
50 - 57		Jahr (zweistellig): 1, 2, 4, 8, 10, 20, 40, 80 (bitweise Addition)
58			Pr�fbit (gerade Parit�t) f�R die Bits 36-57

*)



END_FUNCTION_BLOCK

FUNCTION DST : BOOL
VAR_INPUT
	UTC : DT;
END_VAR
VAR
	yr : INT;
	yr4 : DWORD;
	ltc: DWORD;
	idate : DWORD;
END_VAR

yr := YEAR_OF_DATE(DT_TO_DATE(UTC));
ltc := DT_TO_DWORD(UTC);
idate := DT_TO_DWORD(SET_DT(yr, 3, 31, 1, 0, 0));
yr4 := ULINT_TO_DWORD(INT_TO_ULINT(yr) * 5 / 4 + 1);
DST := (DWORD_TO_ULINT(idate) - ((DWORD_TO_ULINT(yr4) + 3) MOD 7) * 86400 <= DWORD_TO_ULINT(ltc)) AND (DWORD_TO_ULINT(idate) + (214 - DWORD_TO_ULINT(yr4) MOD 7) * 86400 > DWORD_TO_ULINT(ltc));


(*
Equation used TO calculate the beginning OF European Summer TIME:
Sunday (31 - (5*y/4 + 4) mod 7) March at 01.00 UTC
(valid through 2099, courtesy of Robert H. van Gent, EC).

European Summer Time ends (clocks go back) at 01.00 UTC on

    * 29 October 2006
    * 28 October 2007
    * 26 October 2008

Equation used to calculate the end of European Summer Time:
Sunday (31 - (5*y/4 + 1) mod 7) October at 01.00 UTC
(validity AND credits as above).

*)


END_FUNCTION

FUNCTION DT2_TO_SDT : SDT
VAR_INPUT
	DI : DATE;
	TI : TOD;
END_VAR


DT2_TO_SDT.YEAR := YEAR_OF_DATE(di);
DT2_TO_SDT.MONTH := MONTH_OF_DATE(di);
DT2_TO_SDT.DAY := DAY_OF_MONTH(di);
DT2_TO_SDT.WEEKDAY := DAY_OF_WEEK(di);
DT2_TO_SDT.MS := UDINT_TO_INT(TOD_TO_UDINT(ti) MOD 1000);
DT2_TO_SDT.SECOND := UDINT_TO_INT((TOD_TO_UDINT(ti) / 1000) MOD 60);
DT2_TO_SDT.MINUTE := UDINT_TO_INT((TOD_TO_UDINT(ti) / 60000) MOD 60);
DT2_TO_SDT.HOUR := UDINT_TO_INT(TOD_TO_UDINT(ti) / 3600000);
END_FUNCTION

FUNCTION DT_TO_SDT : SDT
VAR_INPUT
	DTI : DT;
END_VAR
VAR
	tmp : DATE;
	tdt : DWORD;
END_VAR


tmp := DT_TO_DATE(dti);
tdt := ULINT_TO_DWORD(DT_TO_ULINT(dti) - DATE_TO_ULINT(tmp));
DT_TO_SDT.YEAR := YEAR_OF_DATE(tmp);
DT_TO_SDT.MONTH := MONTH_OF_DATE(tmp);
DT_TO_SDT.DAY := DAY_OF_MONTH(tmp);
DT_TO_SDT.WEEKDAY := DAY_OF_WEEK(tmp);
DT_TO_SDT.SECOND := ULINT_TO_INT(DWORD_TO_ULINT(tdt) MOD 60);
DT_TO_SDT.MINUTE := ULINT_TO_INT((DWORD_TO_ULINT(tdt) / 60) MOD 60);
DT_TO_SDT.HOUR := ULINT_TO_INT(DWORD_TO_ULINT(tdt) / 3600);
END_FUNCTION

FUNCTION EASTER : DATE
VAR_INPUT
	year : INT;
END_VAR
VAR
	b,c: INT;
	oday: INT;
END_VAR

b := (204 - 11 * (YEAR MOD 19)) MOD 30;
IF b > 27 THEN b := b - 1; END_IF;
c := (year + year / 4 + b - 13) MOD 7;
oday := 28 + b - c;
IF oday > 33 THEN
	EASTER := SET_DATE(year, 4, oday - 31);
ELSE
	EASTER := SET_DATE(year, 3, oday);
END_IF;




(* alternativer algorithmus ueber das pasah fest
execution time roughly 200us
Der sog. Passah-Vollmond wird berechnet, in dem das Jahr durch 19 ge-
teilt wird und der Rest mit der folgenden Tabelle verglichen wird:
 
    0: Apr 14       5: Apr 18      10: Mrz 25      15: Mrz 30
    1: Apr 03       6: Apr 08      11: Apr 13      16: Apr 17
    2: Mrz 23       7: Mrz 28      12: Apr 02      17: Apr 07
    3: Apr 11       8: Apr 16      13: Mrz 22      18: Mrz 27
    4: Mrz 31       9: Apr 05      14: Apr 10
 
Faellt dieses Datum auf einen Sonntag, ist Ostern der naechste Sonntag!
 
Beispiel: 1992 MOD 19 = 16, daraus folgt 17.04., der naechste Sonntag
          ist dann der 19. April (Ostersonntag)
*)


(* this algorithm is 180 us
a := year MOD 19;
b := year / 100;
c := year MOD 100;
d := b / 4;
e := b MOD 4;
f := (b + 8) / 25;
g := (b - f + 1) / 3;
h := (19 * a + b - d -g + 15) MOD 30;
i := C / 4;
k := c MOD 4;
l := (32 + 2*e + 2*i - h - k) MOD 7;
m := (a + 11 * h + 22 * l) / 451;
x := h + l - 7 * m + 114;
n := X / 31;
o := x MOD 31 + 1;

easter := set_Date(year,n,o);
*)
(*
Im Magazin "Nature" vom 20. April 1876 ver�ffentlichte ein anonymer Autor eine Tabelle mit Regeln zur Berechnung des (Gregorianischen) Ostersonntages des Jahres J. In Formeln ausgedr�ckt erh�lt man das Folgende:
a	=	J mod 19
b	=	int(J / 100)
c	=	J mod 100
d	=	int(b / 4)
e	=	b mod 4
f	=	int((b + 8) / 25)
g	=	int((b - f + 1) / 3
h	=	(19 � a + b - d - g + 15) mod 30
i	=	int(c / 4)
k	=	c mod 4
l	=	(32 + 2 � e + 2 � i - h - k) mod 7
m	=	int((a + 11 � h + 22 � l) / 451)
n	=	int((h + l - 7 � m + 114) / 31)
o	=	(h + l - 7 � m + 114) mod 31

n ist hierbei die Nummer des Monats, o + 1 die Nummer des Tages auf welchen der Ostersonntag im Jahr J f�llt. Dieser Algorithmus kommt ohne Hilfszahlen aus.

*)
END_FUNCTION

FUNCTION_BLOCK EVENTS
VAR_INPUT
	DATE_IN : DATE;
	ENA : BOOL;
END_VAR
VAR_OUTPUT
	Y : BOOL;
	NAME : STRING(30);
END_VAR
VAR
	i : INT;
	last_active : DATE;
	size :INT := 49;
	day_in: DINT;
	cyr: INT;
	lday: DINT;
	check : HOLIDAY_DATA;
	y_int : BOOL;
	name_int : STRING(30);
END_VAR
VAR_IN_OUT
	ELIST : ARRAY[0..49] OF HOLIDAY_DATA;
END_VAR

(* for performance reasons only activate once a day *)
IF last_active <> date_in THEN
	last_active := DATE_IN;
	Y_int := FALSE;
	name_int := '';
	day_in := DAY_OF_DATE(DATE_IN);
	cyr := YEAR_OF_DATE(DATE_IN);

	(* search list for events *)
	FOR i := 0 TO size DO
		check := elist[i];
		lday := DAY_OF_DATE(SET_DATE(cyr,check.month, check.day));
		IF day_in >= lday AND day_in <= lday + check.use - 1 THEN
			y_int := TRUE;
			name_int := check.name;
			EXIT;
		END_IF;
	END_FOR;
END_IF;

IF ENA THEN
	Y := y_int;
	NAME := name_int;
ELSE
	Y := FALSE;
	NAME := '';
END_IF;


END_FUNCTION_BLOCK

FUNCTION_BLOCK HOLIDAY
VAR_INPUT
	DATE_IN : DATE;
	LANGU : INT;
	FRIDAY : BOOL;
	SATURDAY : BOOL;
	SUNDAY : BOOL;
END_VAR
VAR_IN_OUT
	HOLIDAYS : ARRAY[0..29] OF HOLIDAY_DATA;
END_VAR
VAR CONSTANT
	SIZE : INT := 29;
END_VAR
VAR_OUTPUT
	Y : BOOL;
	NAME : STRING(30);
END_VAR
VAR
	last_active : DATE;
	ostern: DATE;
	i: INT;
	jahr: INT;
	x_date: DATE;
	lx: INT;
	wdx: INT;
END_VAR

(* for performance reasons only activate once a day *)
IF last_active = date_in THEN RETURN; END_IF;
last_active := DATE_IN;

(* determine language *)
IF LANGU = 0 THEN
	lx := language.DEFAULT;
ELSE
	lx := MIN(language.LMAX, LANGU);
END_IF;

(* berechnung von ostern f�r das aktuelle jahr *)
jahr := YEAR_OF_DATE(date_in);
ostern := EASTER(jahr);
wdx := DAY_OF_WEEK(DATE_IN);
Y := FALSE;

(* check for holidays *)
FOR i := 0 TO size DO
	x_date := SET_DATE(jahr, HOLIDAYS[i].MONTH , HOLIDAYS[i].DAY);
	IF HOLIDAYS[i].USE = 1 AND HOLIDAYS[i].MONTH > 0 THEN
		(* check for fixed date holiday *)
		IF x_date = date_in THEN
			Y := TRUE;
			NAME := HOLIDAYS[i].NAME;
			RETURN;
		END_IF;
	ELSIF HOLIDAYS[i].USE = 1 AND HOLIDAYS[i].MONTH = 0 THEN
		(* check for holiday in reference to easter *)
		IF DATE_ADD(ostern, HOLIDAYS[i].DAY ,0,0,0) = date_in THEN
			Y := TRUE;
			NAME := HOLIDAYS[i].NAME;
			RETURN;
		END_IF;
	ELSIF HOLIDAYS[i].USE < 0 THEN
		(* check for holiday on a weekday before date *)
		IF DAY_OF_WEEK(date_in) = ABS(HOLIDAYS[i].USE) AND date_in < x_date AND date_in >= DATE_ADD(x_date,-7,0,0,0) THEN
			Y := TRUE;
			NAME := HOLIDAYS[i].NAME;
			RETURN;
		END_IF;
	END_IF;
END_FOR;

(* check array if today is weekend *)
IF NOT Y AND (wdx = 5 AND FRIDAY OR wdx = 6 AND SATURDAY OR wdx = 7 AND SUNDAY) THEN
	Y := TRUE;
	NAME := LANGUAGE.WEEKDAYS[LOCATION.LANGUAGE[lx],wdx];
ELSE
	NAME := '';
END_IF;

END_FUNCTION_BLOCK

FUNCTION HOUR : INT
VAR_INPUT
	itod : TOD;
END_VAR

HOUR := UDINT_TO_INT(TOD_TO_UDINT(itod) / 3600000);


(* change history
hm 4. aug 2006	rev 1.0
	original version

hm 2.10.2006 	rev 1.1
	changed name of input to itod

*)
END_FUNCTION

FUNCTION HOUR_OF_DT : INT
VAR_INPUT
	XDT : DT;
END_VAR

HOUR_OF_DT := UDINT_TO_INT((DT_TO_UDINT(XDT) MOD 86400) / 3600);
END_FUNCTION

FUNCTION HOUR_TO_TIME : TIME
VAR_INPUT
	IN : REAL;
END_VAR

HOUR_TO_TIME := DWORD_TO_TIME(REAL_TO_DWORD(IN * 3600000));
END_FUNCTION

FUNCTION HOUR_TO_TOD : TOD
VAR_INPUT
	IN : REAL;
END_VAR

HOUR_TO_TOD := DWORD_TO_TOD(REAL_TO_DWORD(IN * 3600000));
END_FUNCTION

FUNCTION JD2000 : REAL
VAR_INPUT
	DTI : DT;
END_VAR

JD2000 := UDINT_TO_REAL(DT_TO_UDINT(DTI) - 946728000) / 86400.0;

(* revision histroy
hm	15. jul. 2008	rev 1.0
	original release


*)	

END_FUNCTION

FUNCTION LEAP_DAY : BOOL
VAR_INPUT
	IDATE : DATE;
END_VAR
VAR
END_VAR


LEAP_DAY := DATE_TO_UDINT(IDATE) MOD 126230400 = 68169600;


(* change history

hm 	15. jun. 2008	rev 1.0
	original version

hm	7. oct. 2008	rev 1.1
	changed function month to month_of_date

hm	24. jan. 2011	rev 1.2
	improved performance
*)
END_FUNCTION

FUNCTION LEAP_OF_DATE : BOOL
VAR_INPUT
	idate : DATE;
END_VAR


LEAP_OF_DATE := SHL(UDINT_TO_DWORD((DATE_TO_UDINT(idate) + 43200) / 31557600), 30) = 16#80000000;


(* change history

2.10.2006		rev 1.1
the function now calls leap_year to accomodate further accuracy.
the function now works for any year from 1970 to 2100

8. jan 2008		rev 1.2
	improved code for better performance

28. jan. 2011	rev 1.3
	improved performance
*)
END_FUNCTION

FUNCTION LEAP_YEAR : BOOL
VAR_INPUT
	yr : INT;
END_VAR

LEAP_YEAR := yr MOD 4 = 0;

(* this code was used prior to rev 1.1

IF yr MOD 400 = 0 THEN leap_year := TRUE;
ELSIF yr MOD 100 = 0 THEN leap_year := FALSE;
ELSIF yr MOD 4 =0 THEN leap_year := TRUE;
ELSE leap_year := FALSE;
END_IF;

*)

(* change history

hm 	2.10.2006		rev 1.1
	the function now works for any year from 1970 up to 2100

hm	1. oct 2007		rev 1.2
	chaged code for higher performance
*)
END_FUNCTION

FUNCTION LTIME_TO_UTC : DT
VAR_INPUT
	LTIME : DT;
	DST : BOOL;
	TIME_ZONE_OFFSET : INT;
END_VAR

LTIME_TO_UTC := UDINT_TO_DT(DT_TO_UDINT(LTIME) - INT_TO_UDINT(TIME_ZONE_OFFSET) * 60);
IF DST THEN LTIME_TO_UTC := LTIME_TO_UTC - T#1h; END_IF;
END_FUNCTION

FUNCTION MINUTE : INT
VAR_INPUT
	itod : TOD;
END_VAR

MINUTE := UDINT_TO_INT(TOD_TO_UDINT(itod) / 60000 - TOD_TO_UDINT(itod) / 3600000 * 60);


(* change history

2.10.2006 changes name of input to itod

*)
END_FUNCTION

FUNCTION MINUTE_OF_DT : INT
VAR_INPUT
	XDT : DT;
END_VAR

MINUTE_OF_DT := UDINT_TO_INT(DT_TO_UDINT(XDT) MOD 3600) / 60;
END_FUNCTION

FUNCTION MINUTE_TO_TIME : TIME
VAR_INPUT
	IN : REAL;
END_VAR

MINUTE_TO_TIME := DWORD_TO_TIME(REAL_TO_DWORD(IN * 60000.0));
END_FUNCTION

FUNCTION MONTH_BEGIN : DATE
VAR_INPUT
	idate : DATE;
END_VAR

MONTH_BEGIN := UDINT_TO_DATE(DATE_TO_UDINT(idate) - INT_TO_UDINT(DAY_OF_MONTH(idate) - 1) * 86400);
END_FUNCTION

FUNCTION MONTH_END : DATE
VAR_INPUT
	IDATE : DATE;
END_VAR

MONTH_END := UDINT_TO_DATE(DATE_TO_UDINT(SET_DATE(YEAR_OF_DATE(idate),MONTH_OF_DATE(idate)+1,1)) - 86400);
END_FUNCTION

FUNCTION MONTH_OF_DATE : INT
VAR_INPUT
	IDATE : DATE;
END_VAR

MONTH_OF_DATE := DAY_OF_YEAR(idate);
IF MONTH_OF_DATE < 32 THEN
	MONTH_OF_DATE := 1;
ELSIF LEAP_OF_DATE(IDATE) THEN
	MONTH_OF_DATE := (MONTH_OF_DATE * 53 + 1668) / 1623;
ELSE
	MONTH_OF_DATE := (MONTH_OF_DATE * 53 + 1700) / 1620;
END_IF;


(* code for rev 1.2
MONTH_OF_DATE := DAY_OF_YEAR(IDATE);
IF LEAP_OF_DATE(IDATE) THEN
	CASE MONTH_OF_DATE OF
		1..31 	:	MONTH_OF_DATE := 1;
		32..60	:	MONTH_OF_DATE := 2;
		61..91	:	MONTH_OF_DATE := 3;
		92..121 :	MONTH_OF_DATE := 4;
		122..152:	MONTH_OF_DATE := 5;
		153..182:	MONTH_OF_DATE := 6;
		183..213:	MONTH_OF_DATE := 7;
		214..244:	MONTH_OF_DATE := 8;
		245..274:	MONTH_OF_DATE := 9;
		275..305:	MONTH_OF_DATE := 10;
		306..335:	MONTH_OF_DATE := 11;
		336..366:	MONTH_OF_DATE := 12;
	END_CASE;
ELSE
	CASE MONTH_OF_DATE OF
		1..31 	:	MONTH_OF_DATE := 1;
		32..59	:	MONTH_OF_DATE := 2;
		60..90	:	MONTH_OF_DATE := 3;
		91..120 :	MONTH_OF_DATE := 4;
		121..151:	MONTH_OF_DATE := 5;
		152..181:	MONTH_OF_DATE := 6;
		182..212:	MONTH_OF_DATE := 7;
		213..243:	MONTH_OF_DATE := 8;
		244..273:	MONTH_OF_DATE := 9;
		274..304:	MONTH_OF_DATE := 10;
		305..334:	MONTH_OF_DATE := 11;
		335..365:	MONTH_OF_DATE := 12;
	END_CASE;
END_IF;
*)
END_FUNCTION

FUNCTION MULTIME : TIME
VAR_INPUT
	t : TIME;
	M : REAL;
END_VAR

MULTIME := DWORD_TO_TIME(REAL_TO_DWORD(DWORD_TO_REAL(TIME_TO_DWORD(t))*M));
END_FUNCTION

FUNCTION PERIOD : BOOL
VAR_INPUT
	d1 : DATE;
	dx : DATE;
	d2 : DATE;
END_VAR
VAR
	day1, day2, dayx : INT;
END_VAR

day1 := DAY_OF_YEAR(d1);
day2 := DAY_OF_YEAR(d2);
dayx := DAY_OF_YEAR(dx);
IF NOT LEAP_OF_DATE(dx) AND dayx > 58 THEN dayx := dayx + 1; END_IF;
IF NOT LEAP_OF_DATE(d1) AND day1 > 58 THEN day1 := day1 + 1; END_IF;
IF NOT LEAP_OF_DATE(d2) AND day2 > 58 THEN day2 := day2 + 1; END_IF;

IF day2 < day1 THEN
	(* the period spans over the new year *)
	PERIOD := dayx <= day2 OR dayx >= day1;
ELSE
	PERIOD := dayx >= day1 AND dayx <= day2;
END_IF;

(* code before rev 1.2
yx := year(dx);
p1 := date_add(d1,0,0,0,yx - year(d1));
p2 := date_add(d2,0,0,0,yx - year(d2));

IF p2 >= p1 THEN
	period := dx <= p2  AND dx >= p1;
ELSE
	period := dx <= p2 OR dx >= p1;
END_IF;
*)
END_FUNCTION

FUNCTION PERIOD2 : BOOL
VAR_INPUT
	DP : ARRAY[0..3,0..1] OF DATE;
	DX : DATE;
END_VAR

PERIOD2 := 	(DX >= DP[0,0] AND DX <= DP[0,1]) OR
			(DX >= DP[1,0] AND DX <= DP[1,1]) OR
			(DX >= DP[2,0] AND DX <= DP[2,1]) OR
			(DX >= DP[3,0] AND DX <= DP[3,1]);
END_FUNCTION

FUNCTION REFRACTION : REAL
VAR_INPUT
	ELEV : REAL;
END_VAR
VAR
	ELEV_work : REAL;
END_VAR
(* IEC 61131-3 does not let a POU assign to its inputs, so the body works on copies. *)
ELEV_work := ELEV;

ELEV_work := LIMIT(-1.9, ELEV_work, 80.0);
REFRACTION := 0.0174532925199433 / TAN(0.0174532925199433 * (ELEV_work + 10.3 / (ELEV_work + 5.11)));


(* revision histroy
hm	14. jul. 2008	rev 1.0
	original release

hm	7. mar. 2009	rev 1.1
	using new formula

*)	

END_FUNCTION

FUNCTION_BLOCK RTC_2
VAR_INPUT
	SET : BOOL;
	SDT : DT;
	SMS : INT;
	DEN : BOOL;
	OFS : INT;
END_VAR
VAR_OUTPUT
	UDT : DT;
	LOCAL_DT : DT;
	DSO : BOOL;
	XMS : INT;
END_VAR
VAR
	RT : RTC_MS;
END_VAR


(* call rtc *)
RT(SET := SET, SDT := SDT, SMS := SMS);
UDT := rt.xdt;
XMS := rt.XMS;

(* check for daylight savings time and set dso output *)
DSO := DST(udt) AND DEN;

(* calculate time offset and set ldt output *)
LOCAL_DT := UDINT_TO_DT(DT_TO_UDINT(UDT) + INT_TO_DINT(ofs + BOOL_TO_INT(DSO)*60) * 60);
END_FUNCTION_BLOCK

FUNCTION_BLOCK RTC_MS
VAR_INPUT
	SET : BOOL;
	SDT : DT;
	SMS : INT;
END_VAR
VAR_OUTPUT
	XDT : DT;
	XMS : INT;
END_VAR
VAR
	init: BOOL;
	last: DWORD;
	Tx: DWORD;
END_VAR


tx := T_PLC_MS();
IF set OR NOT init THEN
	(* clock needs to be set when set is true or after power up *)
	init := TRUE;
	xdt := SDT;
	XMS := SMS;
ELSE
	XMS := XMS + DWORD_TO_INT(ULINT_TO_DWORD((DWORD_TO_ULINT(tx) + 4294967296 - DWORD_TO_ULINT(last)) MOD 4294967296));
	(* check if one second has expired *)
	IF XMS > 999 THEN
		XDT := XDT + T#1s;
		XMS := XMS - 1000;
	END_IF;
END_IF;
last := tx;
END_FUNCTION_BLOCK

FUNCTION SDT_TO_DATE : DATE
VAR_INPUT
	DTI : SDT;
END_VAR


SDT_TO_DATE := SET_DATE(DTI.YEAR, DTI.MONTH, DTI.DAY);
END_FUNCTION

FUNCTION SDT_TO_DT : DT
VAR_INPUT
	DTI : SDT;
END_VAR


SDT_TO_DT := SET_DT(DTI.YEAR, DTI.MONTH, DTI.DAY, DTI.HOUR, DTI.MINUTE, DTI.SECOND);
END_FUNCTION

FUNCTION SDT_TO_TOD : TOD
VAR_INPUT
	DTI : SDT;
END_VAR


SDT_TO_TOD := UDINT_TO_TOD(INT_TO_UDINT(DTI.HOUR) * 3600000 + INT_TO_UDINT(DTI.MINUTE) * 60000 + INT_TO_UDINT(DTI.SECOND) * 1000 + INT_TO_UDINT(DTI.MS));
END_FUNCTION

FUNCTION SECOND : REAL
VAR_INPUT
	itod : TOD;
END_VAR

SECOND := UDINT_TO_REAL(TOD_TO_UDINT(itod) - TOD_TO_UDINT(itod)/60000 * 60000) / 1000.0;



(* change history

hm	2. oct. 2006 rev 1.1 
	changed name of input to itod

*)
END_FUNCTION

FUNCTION SECOND_OF_DT : INT
VAR_INPUT
	XDT : DT;
END_VAR

SECOND_OF_DT := UDINT_TO_INT(DT_TO_UDINT(XDT) MOD 60);
END_FUNCTION

FUNCTION SECOND_TO_TIME : TIME
VAR_INPUT
	IN : REAL;
END_VAR

SECOND_TO_TIME := DWORD_TO_TIME(REAL_TO_DWORD(IN * 1000.0));
END_FUNCTION

FUNCTION SET_DATE : DATE
VAR_INPUT
	YEAR : INT;
	MONTH : INT;
	DAY : INT;
END_VAR
VAR
	count : INT;
	ofs: ARRAY[1..12] OF INT := 0, 31, 59, 90, 120, 151, 181, 212, 243, 273, 304, 334;

	ENDIF: BOOL;
END_VAR

IF (MONTH > 2) AND (year MOD 4 = 0)
THEN (* we add one day for leap year *)
	SET_DATE := UDINT_TO_DATE((INT_TO_UDINT(ofs[MONTH] + day ) + (INT_TO_UDINT(year) * 1461 - 2878169) / 4)*86400);
ELSE
	SET_DATE := UDINT_TO_DATE((INT_TO_UDINT(ofs[MONTH] + day - 1 ) + (INT_TO_UDINT(year) * 1461 - 2878169) / 4)*86400);
END_IF;


// old code
// IF month > 2 THEN
// 	count := (month - 1) * 30;
// 	IF month > 7 THEN count := count + SHR(month - 3,1); ELSE count := count + SHR(month - 4,1); END_IF;
// 	(* chech for leap year and add one day if true *)
// 	IF year MOD 4 = 0 THEN count := count + 1; END_IF;
// ELSE
// 	count := (month - 1) * 31;
// END_IF;
//
// SET_DATE := UDINT_TO_DATE((INT_TO_UDINT(count + day - 1) + (INT_TO_UDINT(year) * 1461 - 2878169) / 4) * 86400);
END_FUNCTION

FUNCTION SET_DT : DT
VAR_INPUT
	year : INT;
	month : INT;
	day : INT;
	hour : INT;
	minute : INT;
	second : INT;
END_VAR

SET_DT := UDINT_TO_DT(DATE_TO_UDINT(SET_DATE(YEAR, MONTH, day)) + INT_TO_UDINT(SECOND) + INT_TO_UDINT(MINUTE) * 60 + INT_TO_UDINT(HOUR) * 3600);


END_FUNCTION

FUNCTION SET_TOD : TOD
VAR_INPUT
	hour : INT;
	minute : INT;
	second : REAL;
END_VAR

SET_TOD := UDINT_TO_TOD(REAL_TO_UDINT(SECOND * 1000.0) + INT_TO_UDINT(MINUTE) * 60000 + INT_TO_UDINT(HOUR) * 3600000);
END_FUNCTION

FUNCTION SUN_MIDDAY : TOD
VAR_INPUT
	LON : REAL;
	UTC : DATE;
END_VAR
VAR
	T : REAL;
	OFFSET : REAL;
END_VAR

T := INT_TO_REAL(DAY_OF_YEAR(utc));
OFFSET := -0.1752 * SIN(0.033430 * T + 0.5474) - 0.1340 * SIN(0.018234 * T - 0.1939);
SUN_MIDDAY := HOUR_TO_TOD(12.0 - OFFSET - lon * 0.0666666666666);
END_FUNCTION

FUNCTION_BLOCK SUN_POS
VAR_INPUT
	latitude : REAL;				(* latitude of geographical position  *)
	longitude : REAL;				(* longitude of geographical position  *)
	utc : DT;						(*	world time				*)
END_VAR
VAR_OUTPUT
	B: REAL;
	H : REAL;
	HR: REAL;
END_VAR
VAR_TEMP
	g: REAL;
	a: REAL;
	d: REAL;
	t1: REAL;
	n: REAL;
	e: REAL;
	c: REAL;
	tau: REAL;
	sin_d: REAL;
	rlat: REAL;
	sin_lat: REAL;
	cos_lat: REAL;
	cos_tau: REAL;
	cos_d: REAL;
END_VAR


(* n is the julian date and the number of days since 1.1.2000-12:00 midday *)
(* be careful for step7 date startes 1.1.1990 instead of 1.1.1970 *)
(* for step7 this line must change *)
n := UDINT_TO_REAL(DT_TO_UDINT(UTC) - 946728000) * 0.000011574074074074;
g :=MODR(6.240040768 + 0.01720197 * n, math.PI2);
d := MODR(4.89495042 + 0.017202792 * n, math.PI2) + 0.033423055 * SIN(g) + 0.000349066 * SIN(2.0*g);
e := 0.409087723 - 0.000000006981317008 * n;
cos_d := COS(d);
sin_d := SIN(d);
a := ATAN(COS(e) * sin_d / cos_d);
IF cos_d < 0.0 THEN a := a + math.PI; END_IF;
c := ASIN(SIN(e) * sin_d);

(* also here we must be very careful utc is from 1.1.1970 for step7 the formula must change *)
tau := RAD(MODR(6.697376 + (n - 0.25) * 0.0657098245037645 + DWORD_TO_REAL(TOD_TO_DWORD(DT_TO_TOD(utc))) * 0.0000002785383333, 24.0) * 15.0 + longitude) - a;
rlat := RAD(latitude);
sin_lat := SIN(rlat);
cos_lat := COS(rlat);
cos_tau := COS(tau);
t1 := cos_tau * sin_lat - TAN(c) * cos_lat;
B := ATAN(SIN(tau) / t1);
IF t1< 0.0 THEN B := B + math.PI2; ELSE B := B + math.PI; END_IF;
B := DEG(MODR(B, math.PI2));
h := DEG(ASIN(COS(C) * cos_tau * cos_lat +SIN(c) * sin_lat));
IF h > 180.0 THEN h := h - 360.0; END_IF;
(* consider refraction *)
HR := h + REFRACTION(h);
END_FUNCTION_BLOCK

FUNCTION_BLOCK SUN_TIME
VAR_INPUT
	LATITUDE : REAL;				(* latitude of geographical position  *)
	LONGITUDE : REAL;				(* longitude of geographical position  *)
	UTC : DATE;						(*	world time	*)
	H : REAL := -0.83333333333;	(* heighth above horizon for sunrise *)
END_VAR
VAR_OUTPUT
	MIDDAY : TOD;			(*	astrological midday in hours when sun stands at south direction	*)
	SUN_RISE : TOD;				(*	sun rise for current day in local time *)
	SUN_SET : TOD;				(*	sun set for current day in local time *)
	SUN_DECLINATION : REAL;		(*	sun declination above horizon at midday in degrees	*)
END_VAR
VAR
	dk: REAL;						(* sun declination at midday *)
	delta: TIME;					(* delta from midday for sunrise and sunset  *)
	b: REAL;
END_VAR

B := latitude * 0.0174532925199433;
MIDDAY := SUN_MIDDAY(longitude, utc);
DK := 0.40954 * SIN(0.0172 * (INT_TO_REAL(DAY_OF_YEAR(utc)) - 79.35));
sun_declination := DEG(DK);
IF sun_declination > 180.0 THEN sun_declination := sun_declination - 360.0; END_IF;
sun_declination := 90.0 - LATITUDE + sun_declination;
delta := HOUR_TO_TIME(ACOS((SIN(RAD(H)) - SIN(B) * SIN(DK)) / (COS(B) * COS(DK))) * 3.819718632);
sun_rise := MIDDAY - delta;
sun_set := MIDDAY + delta;
END_FUNCTION_BLOCK

FUNCTION TIMECHECK : BOOL
VAR_INPUT
	TD : TOD;
	START : TOD;
	STOP : TOD;
END_VAR

IF stop < start THEN
	TIMECHECK := start <= TD OR TD < stop;
ELSE
	TIMECHECK := start <= TD AND TD < stop;
END_IF;
END_FUNCTION

FUNCTION UTC_TO_LTIME : DT
VAR_INPUT
	UTC : DT;
	DST_ENABLE : BOOL;
	TIME_ZONE_OFFSET : INT;
END_VAR
VAR
	tmp: INT;
END_VAR


tmp := TIME_ZONE_OFFSET * 60 + BOOL_TO_INT(DST_ENABLE AND DST(UTC)) * 3600;
IF tmp < 0 THEN
	tmp := ABS(tmp);
	UTC_TO_LTIME := UDINT_TO_DT(DT_TO_UDINT(UTC) -  INT_TO_UDINT(tmp));
ELSE
	UTC_TO_LTIME := UDINT_TO_DT(DT_TO_UDINT(UTC) +  INT_TO_UDINT(tmp));
END_IF;
END_FUNCTION

FUNCTION WORK_WEEK : INT
VAR_INPUT
	idate : DATE;
END_VAR
VAR
	d1 : DATE;
	w1 : INT;
	ds: DWORD;
	yr: INT;
	w31: INT;
	w01: INT;
	wm: INT;
END_VAR

(* berechne den 1.1 des jahres von idate. *)
yr := YEAR_OF_DATE(idate);
d1 := YEAR_BEGIN(yr);
(* wochentag von d1 *)
w1 := DAY_OF_WEEK(d1);
(* offset des montags der eletzten KW des vorjahres *)
(* wenn der erste tag des jahres gr��er als donnerstag ist dann beginnt die letzte kw am montag davor *)
(* wenn der erste tag des jahres ein donnerstag oder kleiner ist beginnt die erste kw 2 montage davor *)
IF w1 < 5 THEN
	ds := ULINT_TO_DWORD(DATE_TO_ULINT(d1) - INT_TO_ULINT(w1+6) * 86400);
ELSE
	ds := ULINT_TO_DWORD(DATE_TO_ULINT(d1) - INT_TO_ULINT(w1-1) * 86400);
END_IF;

(* kalenderwoche des eingangsdatums *)
WORK_WEEK := ULINT_TO_INT((DATE_TO_ULINT(idate) - DWORD_TO_ULINT(ds)) / 604800);

(* korrektur wenn work_week = 0 *)
IF work_week = 0 THEN
	(* work_week needs to be 53 when 1.jan of the year before is thursday or dec 31. is thursday. *)
	(* first and last weekday of a year is equal and one more day for a leap_year. *)
	IF w1 > 1 THEN w31 := w1 - 1; ELSE W31 := 7; END_IF;
	IF LEAP_YEAR(yr - 1) AND w31 > 1 THEN w01 := W31 - 1; ELSE w1 := 7; END_IF;
	(* if first or last day of a year is a thursday, the year has 53 weeks *)
	WORK_WEEK := 52 + BOOL_TO_INT(w31 = 4 OR w01 = 4);
ELSE
	(* end of year calculation *)
	(* calculated the first and last weekday *)
	IF leap_year(yr) THEN
		IF w1 < 7 THEN w31 := w1 + 1; ELSE w31 := 1; END_IF;
	ELSE
		w31 := w1;
	END_IF;
	(* if first or last day is thursday then the year has 53 weeks otherwise only 52 *)
	wm := 52 + BOOL_TO_INT(w31 = 4 OR w1 = 4);
	IF WORK_WEEK > wm THEN WORK_WEEK := 1; END_IF;
END_IF;
END_FUNCTION

FUNCTION YEAR_BEGIN : DATE
VAR_INPUT
	y : INT;
END_VAR

YEAR_BEGIN := UDINT_TO_DATE((INT_TO_UDINT(y) * 1461 - 2878169) / 4 * 86400);

END_FUNCTION

FUNCTION YEAR_END : DATE
VAR_INPUT
	y : INT;
END_VAR

YEAR_END := UDINT_TO_DATE((INT_TO_UDINT(y) * 1461 - 2876712) / 4 * 86400);

END_FUNCTION

FUNCTION YEAR_OF_DATE : INT
VAR_INPUT
	IDATE : DATE;
END_VAR


YEAR_OF_DATE := UDINT_TO_INT((DATE_TO_UDINT(idate) + 43200) / 31557600 + 1970);
END_FUNCTION

TYPE CALENDAR :
STRUCT
	UTC : DT;					(* world time UTC *)
	LOCAL_DT : DT;					(* local time *)
	LOCAL_DATE : DATE;				(* local date *)
	LOCAL_TOD : TOD;					(* local time of day *)
	YEAR : INT;					(* year of LDATE *)
	MONTH : INT;				(* month of LDATE *)
	DAY : INT;					(* day of LDATE *)
	WEEKDAY : INT;				(* weekday of LDATE *)
	OFFSET : INT;				(* Time Zone Offset for Local time in minutes *)
	DST_EN : BOOL;				(* daylight savings time enable *)
	DST_ON : BOOL;				(* true when daylight savings time os on *)
	NAME : STRING(5);			(* name of time zone *)
	LANGUAGE : INT;			(* location number pls see location setup *)
	LONGITUDE : REAL;			(* longitude of current location *)
	LATITUDE : REAL;			(* latitude of current location *)
	SUN_RISE : TOD;				(* sun_rise for current location *)
	SUN_SET : TOD;				(* sun_set for current location *)
	SUN_MIDDAY : TOD;			(* worldtime when sun stands at south position *)
	SUN_HEIGTH : REAL	;		(* suns heigth at midday, south position *)
	SUN_HOR : REAL;			(* sun angle horizontal 0 = north in degrees *)
	SUN_VER : REAL;			(* sun angle vertical above horizon in degrees *)
	NIGHT : BOOL;				(* true between sun_set and sun_rise *)
	HOLIDAY : BOOL;			(* true when holiday *)
	HOLY_NAME : STRING(30);	(* name of holiday *)
	WORK_WEEK : INT;			(* current work week *)
END_STRUCT
END_TYPE

TYPE COMPLEX :
STRUCT
	re : REAL;
	im : REAL;
END_STRUCT
END_TYPE

TYPE CONSTANTS_LANGUAGE :
STRUCT
	(* Language Setup *)
	DEFAULT : INT := 1; (* 1=english, 2=german 3=french *)
	LMAX : INT := 3;
	WEEKDAYS : ARRAY[1..3, 1..7] OF STRING(10) :=	'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday',
													'Montag', 'Dienstag', 'Mittwoch', 'Donnerstag', 'Freitag', 'Samstag', 'Sonntag',
													'Lundi', 'Mardi', 'Mercredi', 'Jeudi', 'Vendredi', 'Samedi', 'Dimanche';
	WEEKDAYS2 : ARRAY[1..3, 1..7] OF STRING(2) :=	'Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa', 'Su',
													'Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa', 'So',
													'Lu', 'Ma', 'Me', 'Je', 'Ve', 'Sa', 'Di';
	MONTHS : ARRAY[1..3, 1..12] OF STRING(10) :=	'January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December',
													'Januar', 'Februar', 'M�rz', 'April', 'Mai', 'Juni', 'Juli', 'August', 'September', 'Oktober', 'November', 'Dezember',
													'Janvier', 'F�vrier', 'mars', 'Avril', 'Mai', 'Juin', 'Juillet', 'Ao�t', 'Septembre', 'Octobre', 'Novembre', 'Decembre';
	MONTHS3 : ARRAY[1..3, 1..12] OF STRING(3) :=	'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec',
													'Jan', 'Feb', 'Mrz', 'Apr', 'Mai', 'Jun', 'Jul', 'Aug', 'Sep', 'Okt', 'Nov', 'Dez',
													'Jan', 'Fev', 'Mar', 'Avr', 'Mai', 'Jun', 'Jul', 'Aou', 'Sep', 'Oct', 'Nov', 'Dec';
	DIRS : ARRAY[1..3,0..15] OF STRING(3) :=		'N', 'NNE', 'NE', 'ENE', 'E', 'ESE', 'SE', 'SSE', 'S', 'SSW', 'SW', 'WSW', 'W', 'WNW', 'NW', 'NNW',
													'N', 'NNO', 'NO', 'ONO', 'O', 'OSO', 'SO', 'SSO', 'S', 'SSW', 'SW', 'WSW', 'W', 'WNW', 'NW', 'NNW',
													'N', 'NNO', 'NO', 'ONO', 'O', 'OSO', 'SO', 'SSO', 'S', 'SSW', 'SW', 'WSW', 'W', 'WNW', 'NW', 'NNW';
END_STRUCT
END_TYPE

TYPE CONSTANTS_LOCATION :
STRUCT
	(* location setup *)
	DEFAULT : INT := 1; (* 1=germany, 2=austria 3=france 4=belgium-german 5= italien-S�dtirol *)
	LMAX : INT := 5;

	(* language spoken in the location *)
	LANGUAGE : ARRAY[1..5] OF INT := 2, 2, 3, 2, 2;

END_STRUCT
END_TYPE

TYPE CONSTANTS_MATH :
STRUCT
	PI : REAL := 	3.14159265358979323846264338327950288;		(* Kreiszahl PI *)
	PI2 : REAL := 	6.28318530717958647692528676655900576;	(* PI * 2 *)
	PI4 : REAL :=  12.56637061435917295385057353311801152;	(* PI * 4 *)
	PI05 : REAL :=  1.5707963267949;							(* PI / 2 *)
	PI025 : REAL := 0.785398163397448;							(* PI / 4 *)
	PI_INV : REAL := 0.318309886183791;						(* 1 / PI *)
	E : REAL := 	2.71828182845904523536028747135266249;		(* Euler constant e *)
	E_INV : REAL := 0.367879441171442;							(* 1 / e *)
	SQ2 : REAL :=	1.4142135623731;							(* Wurzel von 2 *)
	FACTS : ARRAY[0..12] OF DINT := 1,1,2,6,24,120,720,5040,40320,362880,3628800,39916800,479001600;
END_STRUCT
END_TYPE

TYPE CONSTANTS_PHYS :
STRUCT
	C : REAL := 299792458.0;				(* Lichtgeschwindigkeit in m/s *)
	E : REAL := 1.60217653E-19;				(* elementarladung in Colomb = A * s *)
	G : REAL := 9.80665; 						(* Erdbeschleunigung in m / s� *)
	T0 : REAL := -273.15;						(* absoluter Nullpunkt in �C *)
	RU : REAL := 8.314472; 					(* Universelle Gaskonstante in J / (mol � K) *)
	PN : REAL := 101325.0;					(* NormalDruck in Pa *)
END_STRUCT
END_TYPE

TYPE CONSTANTS_SETUP :
STRUCT
	(* setup Parameters *)
	EXTENDED_ASCII : BOOL := TRUE;
	CHARNAMES : ARRAY[1..4] OF STRING(253) := ';"&quot;&&amp;<&lt;>&gt;�&euro;�&nbsp;�&iexcl;�&cent;�&pound;�&curren;�&yen;�&brvbar;�&sect;�&uml;�&copy;�&ordf;�&laquo;�&not;�&shy;�&reg;�&macr;�&deg;�&plusmn;�&sup2;�&sup3;�&acute;�&micro;�&para;�&middot;�&cedil;�&sup1;�&ordm;�&raquo;�&frac14;�&Ucirc;',
		';�&frac34;�&iquest;�&Agrave;�&Aacute;�&Acirc;�&Atilde;�&Auml;�&Aring;�&AElig;�&Ccedil;�&Egrave;�&Eacute;�&Ecirc;�&Euml;�&Igrave;�&Iacute;�&Icirc;�&Iuml;�&ETH;�&Ntilde;�&Ograve;�&Oacute;�&Ocirc;�&Otilde;�&Ouml;�&times;�&Oslash;�&Ugrave;�&Uacute;�&frac12;',
		';�&Uuml;�&Yacute;�&THORN;�&szlig;�&agrave;�&aacute;�&acirc;�&atilde;�&auml;�&aring;�&aelig;�&ccedil;�&egrave;�&eacute;�&ecirc;�&euml;�&igrave;�&iacute;�&icirc;�&iuml;�&eth;�&ntilde;�&ograve;�&oacute;�&ocirc;�&otilde;�&ouml;�&divide;�&oslash;�&ugrave;',
		';�&uacute;�&ucirc;�&uuml;�&yacute;�&thorn;�&yuml;';
	MTH_OFS : ARRAY[1..12] OF INT := 0,31,59,90,120,151,181,212,243,273,304,334;
	DECADES : ARRAY[0..8] OF REAL := 1.0,10.0,100.0,1000.0,10000.0,100000.0,1000000.0,10000000.0,100000000.0;
END_STRUCT
END_TYPE

TYPE ESR_DATA :
STRUCT
	TYP : BYTE;
	ADRESS : STRING(10);
	DS : DT;
	TS : TIME;
	DATA : ARRAY[0..7] OF BYTE;
END_STRUCT
END_TYPE

TYPE FRACTION :
STRUCT
	NUMERATOR : INT;
	DENOMINATOR : INT;
END_STRUCT
END_TYPE

TYPE HOLIDAY_DATA :
STRUCT
	NAME : STRING(30);
	DAY : SINT;
	MONTH : SINT;
	USE : SINT;
END_STRUCT
END_TYPE

TYPE REAL2 :
STRUCT
	R1 : REAL;	(* small value *)
	RX : REAL;	(* big value *)
END_STRUCT
END_TYPE

TYPE SDT :
STRUCT
	YEAR : INT;
	MONTH : INT;
	DAY : INT;
	WEEKDAY : INT;
	HOUR : INT;
	MINUTE : INT;
	SECOND : INT;
	MS : INT;
END_STRUCT
END_TYPE

TYPE TIMER_EVENT :
STRUCT
	TYP : BYTE;
	CHANNEL : BYTE;
	DAY : BYTE;
	START : TOD;
	DURATION : TIME;
	LAND : BYTE;
	LOR : BYTE;
	LAST : DT;
END_STRUCT
END_TYPE

TYPE VECTOR_3 :
STRUCT
	X : REAL;
	Y : REAL;
	Z : REAL;
END_STRUCT
END_TYPE

VAR_GLOBAL CONSTANT
	STRING_LENGTH : INT := 250;
	LIST_LENGTH : INT := 250;
END_VAR

VAR_GLOBAL
	MATH : CONSTANTS_MATH;
	PHYS : CONSTANTS_PHYS;
	LANGUAGE : CONSTANTS_LANGUAGE;
	SETUP : CONSTANTS_SETUP;
	LOCATION : CONSTANTS_LOCATION;
END_VAR

VAR_GLOBAL RETAIN

END_VAR
