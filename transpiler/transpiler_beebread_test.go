/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package transpiler

import (
	"bytes"
	"strings"
	"testing"
)

// The programs below were checked by compiling the generated Go against
// beebread and royaljelly and running it; these tests pin the generated
// code.

// A call of an OSCAT function converts its arguments, passes a VAR_IN_OUT
// by its address and a buffer as a slice, and fills in the inputs it leaves
// out.
func TestBeebreadFunctions(t *testing.T) {
	checkContains(t, `PROGRAM Main
VAR
  band : REAL; easter : DATE; day : INT; c : COMPLEX;
  buf : ARRAY[0..7] OF BYTE; ok : BOOL; lst : STRING := ',a'; x : REAL;
END_VAR
band := DEAD_BAND(5.0, 2.0);
easter := EASTER(2025);
day := DAY_OF_MONTH(easter);
c := CMUL(CSET(1.0, 2.0), c);
ok := _BUFFER_CLEAR(ADR(buf), SIZEOF(buf));
ok := LIST_ADD(44, 'b', lst);
x := AIN(16#0FFF, 12);
END_PROGRAM`,
		"c oscat.COMPLEX",
		"p.band = oscateng.DEAD_BAND(iec.REAL(5.0), iec.REAL(2.0))",
		"p.easter = oscattime.EASTER(iec.INT(2025))",
		"p.day = oscattime.DAY_OF_MONTH(iec.DATE(p.easter))",
		"p.c = oscatmath.CMUL(oscatmath.CSET(iec.REAL(1.0), iec.REAL(2.0)), p.c)",
		"p.ok = oscatbuffer.BUFFER_CLEAR_((p.buf)[:], iec.UINT(unsafe.Sizeof(p.buf)))",
		"p.ok = oscatlist.LIST_ADD(iec.BYTE(44), iec.STRING(\"b\"), &p.lst)",
		// The inputs AIN leaves out get their initial values.
		"p.x = oscateng.AIN(iec.DWORD(4095), iec.BYTE(12), iec.BYTE(255), iec.REAL(0), iec.REAL(10.0))",
	)
	if _, err := transpileSource(t, "PROGRAM P VAR x : REAL; END_VAR x := DEAD_BAND(1.0, 2.0, 3.0); END_PROGRAM"); err == nil {
		t.Error("expected an error for too many arguments")
	}
}

// An OSCAT function block has upper case fields, INIT, Execute(now) and no
// EN; its inputs and outputs can be written in any case.
func TestBeebreadFunctionBlocks(t *testing.T) {
	checkContains(t, `FUNCTION_BLOCK Smooth
VAR_INPUT x : REAL; END_VAR
VAR_OUTPUT y : REAL; END_VAR
VAR f : FT_PT1; END_VAR
f(in := x, T := T#1s);
y := f.out;
END_FUNCTION_BLOCK

PROGRAM Main
VAR tn : TUNE; tick : TICKER; txt : STRING := 'hello'; shown : STRING; y : REAL; q : BOOL; END_VAR
tn(SU := TRUE, Y => y);
tick(N := 3, PT := T#1s, TEXT := txt);
shown := tick.display;
q := tn.su;
END_PROGRAM`,
		"f oscateng.FT_PT1",
		"s.f.INIT()",
		"s.f.IN = s.x",
		"s.f.Execute(now)",
		"s.y = s.f.OUT",
		"instance.tn.INIT()",
		"instance.tick.INIT()",
		"p.tn.SU = true",
		"p.tn.Execute(now)",
		"p.y = p.tn.Y",
		"p.tick.TEXT = &p.txt",
		"p.shown = p.tick.DISPLAY",
		"p.q = p.tn.SU",
	)
	got, err := transpileSource(t, "PROGRAM P VAR tn : TUNE; END_VAR tn(SU := TRUE); END_PROGRAM")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "tn.EN") {
		t.Errorf("an OSCAT function block has no EN:\n%s", got)
	}
}

// A name that royaljelly also defines is royaljelly's unless PreferOSCAT is
// set, and a name the program declares is the program's.
func TestBeebreadNames(t *testing.T) {
	checkContains(t, "PROGRAM P VAR x : REAL; END_VAR x := CEIL(2.5); END_PROGRAM", "iecmath.CEIL")

	var out bytes.Buffer
	tr := New(&out)
	tr.PreferOSCAT = true
	if err := tr.Transpile(parseForTest(t, "PROGRAM P VAR x : REAL; END_VAR x := ROUND(3.14159, 2); END_PROGRAM")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "oscatmath.ROUND(iec.REAL(3.14159), iec.INT(2))") {
		t.Errorf("expected OSCAT's ROUND in:\n%s", out.String())
	}

	checkContains(t, `FUNCTION DEAD_BAND : REAL VAR_INPUT x : REAL; END_VAR DEAD_BAND := x; END_FUNCTION
PROGRAM P VAR x : REAL; END_VAR x := DEAD_BAND(1.0); END_PROGRAM`, "DEAD_BAND(")
	got, err := transpileSource(t, `FUNCTION DEAD_BAND : REAL VAR_INPUT x : REAL; END_VAR DEAD_BAND := x; END_FUNCTION
PROGRAM P VAR x : REAL; END_VAR x := DEAD_BAND(1.0); END_PROGRAM`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "oscateng.DEAD_BAND") {
		t.Errorf("the program's DEAD_BAND is OSCAT's:\n%s", got)
	}
}

// The OSCAT BUILDING and NETWORK libraries are called like BASIC: a block of
// each, a function, a global variable of NETWORK and a constant, and the
// TwinCAT TCP/IP blocks NETWORK uses, whose Go names are mixed case.
func TestBeebreadBuildingAndNetwork(t *testing.T) {
	src := `PROGRAM Main
VAR coil : ACTUATOR_COIL; conn : FB_SocketConnect; on : BOOL; ip : STRING; lvl : BYTE; n : UINT; END_VAR
coil(IN := TRUE);
on := coil.out;
ip := IP4_TO_STRING(16#C0A80001);
conn(sRemoteHost := ip, nRemotePort := 502, bExecute := on);
lvl := LOG_CL.LEVEL;
n := NETWORK_BUFFER_SHORT_SIZE;
END_PROGRAM`
	checkContains(t, src,
		"coil oscatactuators.ACTUATOR_COIL",
		"conn oscattcpip.FB_SocketConnect",
		"p.coil.IN = true",
		"p.on = p.coil.OUT",
		"p.ip = oscatencoding.IP4_TO_STRING(iec.DWORD(3232235521))",
		"p.conn.SREMOTEHOST = p.ip",
		"p.conn.Execute(now)",
		"p.lvl = oscatnet.LOG_CL.LEVEL",
		"p.n = oscatnet.NETWORK_BUFFER_SHORT_SIZE")
	out, err := GoFile(parseForTest(t, src))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`oscatactuators "github.com/apiarytech/beebread/building/actuators"`,
		`oscatnet "github.com/apiarytech/beebread/network"`,
		`oscatencoding "github.com/apiarytech/beebread/network/encoding"`,
		`oscattcpip "github.com/apiarytech/beebread/network/tcpip"`,
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("expected %s in:\n%s", want, out)
		}
	}
}

// GoFile imports the beebread packages the code uses.
func TestBeebreadImports(t *testing.T) {
	out, err := GoFile(parseForTest(t, "PROGRAM P VAR f : FT_PT1; b : ARRAY[0..3] OF BYTE; x : BOOL; END_VAR f(IN := 1.0); x := _BUFFER_CLEAR(ADR(b), SIZEOF(b)); END_PROGRAM"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`oscateng "github.com/apiarytech/beebread/basic/engineering"`,
		`oscatbuffer "github.com/apiarytech/beebread/basic/buffer"`,
		`"unsafe"`,
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("expected %s in:\n%s", want, out)
		}
	}
}

// Names ignore case: a use is spelled as its declaration, as OSCAT's
// sources need.
func TestNameCase(t *testing.T) {
	checkContains(t, `FUNCTION MODR : REAL
VAR_INPUT IN : REAL; DIVI : REAL; END_VAR
IF divi = 0.0 THEN modr := 0.0; ELSE MODR := in - divi; END_IF;
END_FUNCTION
FUNCTION_BLOCK Acc VAR_INPUT amount : INT; END_VAR VAR_OUTPUT Done : BOOL; END_VAR done := AMOUNT > 0; END_FUNCTION_BLOCK
PROGRAM P VAR acc : ACC; x : REAL; ok : BOOL; END_VAR
x := modr(3.0, 2.0); ACC(AMOUNT := 1); ok := acc.done;
END_PROGRAM`,
		"if (DIVI == 0.0) {", "MODR = 0.0", "MODR = (IN - DIVI)",
		"a.Done = (a.amount > 0)", "acc Acc", "p.x = MODR(3.0, 2.0)",
		"p.acc.amount = 1", "p.ok = p.acc.Done")
}

// OSCAT's global variables are beebread's, and an index into an array OSCAT
// declares from 1 counts from 1.
func TestBeebreadGlobals(t *testing.T) {
	checkContains(t, `FUNCTION F : REAL VAR_INPUT x : REAL; END_VAR F := x * math.pi2; END_FUNCTION
PROGRAM P VAR s : STRING; d : DRIVER_4C; b : BYTE; n : INT; END_VAR
s := LANGUAGE.WEEKDAYS[2, 7]; n := STRING_LENGTH; b := d.SX[1]; d.sx[2] := 3;
END_PROGRAM`,
		"F = (x * oscat.MATH.PI2)",
		"p.s = oscat.LANGUAGE.WEEKDAYS[1][6]",
		"p.n = oscat.STRING_LENGTH",
		"p.b = p.d.SX[0]",
		"p.d.SX[1] = 3")
	// A program's own MATH is the program's.
	checkContains(t, "PROGRAM P VAR math : INT; END_VAR math := 1; END_PROGRAM", "p.math = 1")
}

// A real literal keeps all its digits, and values of different numeric
// types meet at the wider type, as OSCAT's sources need.
func TestLiteralsAndWidening(t *testing.T) {
	checkContains(t, `FUNCTION F : REAL
VAR_INPUT r : REAL; i : INT; d : DINT; b : BYTE; w : WORD; t1, t2 : DATE; END_VAR
VAR unused : INT; l : DINT; END_VAR
l := i;
F := 1.7453293E-2 * r * i;
IF t1 < t2 THEN F := 2.805E-9; END_IF;
b := b AND 16#0F;
w := SHL(w, 2) OR b;
END_FUNCTION`,
		"_ = unused",
		"l = iec.DINT(i)",
		"F = ((0.017453293 * r) * iec.REAL(i))",
		"if (iec.BOOL(time.Time(t1).Before(time.Time(t2)))) {",
		"F = 2.805e-09",
		"b = (b & 15)",
		"w = (bitwise.SHL(w, uint(2)) | iec.WORD(b))")
}

// An assignment to a STRING(n) keeps the first n characters.
func TestStringLength(t *testing.T) {
	checkContains(t, "PROGRAM P VAR s : STRING(2); u : STRING; END_VAR s := 'abc'; u := 'abc'; END_PROGRAM",
		`p.s = stringCut(iec.STRING("abc"), 2)`, "func stringCut(s iec.STRING, n int) iec.STRING", `p.u = "abc"`)
}
