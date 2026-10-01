package transpiler

import (
	"testing"
)

func TestStandardFunctionsTranspilation(t *testing.T) {
	input := `
PROGRAM StdLibTest
	VAR
		i : INT := -10;
		r : REAL;
		s1 : STRING := 'Hello';
		s2 : STRING := 'World';
		s3 : STRING;
		len_s1 : INT;
	END_VAR

	r := SQRT(25.0);
	i := ABS(i);
	s3 := CONCAT(s1, ' ');
	s3 := CONCAT(s3, s2);
	len_s1 := LEN(s1);
END_PROGRAM
`
	expected := `
type StdLibTest struct {
	i      iec.INT
	r      iec.REAL
	s1     iec.STRING
	s2     iec.STRING
	s3     iec.STRING
	len_s1 iec.INT
}

// NewStdLibTestFactory creates a new instance of the StdLibTest program.
func NewStdLibTestFactory(params map[string]string) (func(time.Time), error) {
	instance := &StdLibTest{}
	instance.i = (-10)
	instance.s1 = "Hello"
	instance.s2 = "World"
	return instance.Logic, nil
}

func (p *StdLibTest) Logic(now time.Time) {
	p.r = iec.REAL(numerical.SQRT(iec.REAL(25.0)))
	p.i = iec.INT(numerical.ABS(p.i))
	p.s3 = iecstrings.CONCAT(iec.STRING(p.s1), iec.STRING(" "))
	p.s3 = iecstrings.CONCAT(iec.STRING(p.s3), iec.STRING(p.s2))
	p.len_s1 = iec.INT(iecstrings.LEN(iec.STRING(p.s1)))
}
`
	transpileAndCheck(t, "TestStandardFunctionsTranspilation", input, expected)
}
