package diagram

import (
	"strings"
	"testing"
)

func lowerText(t *testing.T, lang, src string, declared ...string) *Lowered {
	t.Helper()
	low, err := LowerText("P", lang, src, 10, Options{Declared: declared})
	if err != nil {
		t.Fatal(err)
	}
	return low
}

func TestLDTextLowering(t *testing.T) {
	low := lowerText(t, "LD", `
  RUNG sealin (* start, hold, stop *)
    [ Start | Run ] /Stop ( Run )
  RUNG delay
    Run t1:TON(PT := T#5S, ET => Elapsed) ( Done ) ( S Latched )
  RUNG pulse
    +Button EQ(Mode, 2) ( P Pulse ) ( /NotPulse )
`)
	got := low.Body
	for _, want := range []string{
		"Run := (Start OR Run) AND NOT(Stop);",
		"t1(IN := Run, PT := T#5S);",
		"Elapsed := t1.ET;",
		"Done := t1.Q;",
		"IF t1.Q THEN Latched := TRUE; END_IF;",
		"_R_TRIG1(CLK := Button);",
		"(Mode = 2)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if strings.Join(low.Decls, " ") != "t1 : TON; _R_TRIG1 : R_TRIG; _R_TRIG2 : R_TRIG;" {
		t.Errorf("decls %v", low.Decls)
	}
	if low.Rows[0] != 12 {
		t.Errorf("rows %v", low.Rows)
	}
	t.Log("\n" + got)
}

func TestFBDTextLowering(t *testing.T) {
	low := lowerText(t, "FBD", `
  // TIC: a timer read before its call
  Alarm := OR(t1.Q, Alarm)
  hot = GT(Temp, ADD(Limit, 5.0))
  t1 : TON(IN := hot, PT := T#5S)
  Speed := SEL(Run, 0.0, MUL(Ref, -1.5)); Ok := NOT(hot)
`, "t1")
	want := "t1(IN := (Temp > (Limit + 5.0)), PT := T#5S);\nAlarm := (t1.Q OR Alarm);\nSpeed := SEL(Run, 0.0, (Ref * -1.5));\nOk := NOT((Temp > (Limit + 5.0)));\n"
	if low.Body != want {
		t.Errorf("got\n%s\nwant\n%s", low.Body, want)
	}
	if len(low.Decls) != 0 {
		t.Errorf("t1 is declared, got decls %v", low.Decls)
	}
}

func TestTextErrors(t *testing.T) {
	for _, c := range []struct{ lang, src, want string }{
		{"LD", "RUNG a\n  A B", "line 10, column 1: the rung ends in neither a coil nor a function block"},
		{"LD", "RUNG a\n  A ( B ) C", "coils end a rung"},
		{"LD", "RUNG a\n  [ A | B ( C )", "'[' not closed"},
		{"LD", "RUNG a\n  t1:TON(IN := X) ( Q )", "IN of t1 is driven by the rung"},
		{"LD", "RUNG a\n  TON(X) ( Q )", "TON is a function block"},
		{"LD", "A ( B )", "expected RUNG"},
		{"FBD", "X := LIMIT(MN := 0, 5, 10)", "positional"},
		{"FBD", "X := (* open", "comment not closed"},
		{"FBD", "a = b\nb = a\nX := a", "loop"},
		{"FBD", "X := Y Z", "expected the end of the statement"},
	} {
		_, err := LowerText("P", c.lang, c.src, 10, Options{})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s %q: got %v, want %q", c.lang, c.src, err, c.want)
		}
	}
}
