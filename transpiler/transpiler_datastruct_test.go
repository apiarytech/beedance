package transpiler

import (
	"testing"
)

func TestAdvancedDataStructuresTranspilation(t *testing.T) {
	input := `
TYPE
	MY_STRUCT : STRUCT
		A : INT;
		B : BOOL;
	END_STRUCT;
END_TYPE

PROGRAM DataStructTest
	VAR
		matrix : ARRAY[1..2, 1..3] OF INT;
		s1 : MY_STRUCT;
		s2 : MY_STRUCT := (A := 10, B := TRUE);
	END_VAR

	matrix[1][2] := 5;
	s1 := s2;
END_PROGRAM
`
	expected := `
// MY_STRUCT is the transpiled struct for the user-defined type.
type MY_STRUCT struct {
	A iec.INT
	B iec.BOOL
}

type DataStructTest struct {
	matrix [2][3]iec.INT
	s1     MY_STRUCT
	s2     MY_STRUCT
}

// NewDataStructTestFactory creates a new instance of the DataStructTest program.
func NewDataStructTestFactory(params map[string]string) (func(time.Time), error) {
	instance := &DataStructTest{}
	instance.matrix = [2][3]iec.INT{}
	instance.s2 = MY_STRUCT{A: 10, B: true}
	return instance.Logic, nil
}

func (p *DataStructTest) Logic(now time.Time) {
	p.matrix[0][1] = 5
	p.s1 = p.s2
}
`
	transpileAndCheck(t, "TestAdvancedDataStructuresTranspilation", input, expected)
}
