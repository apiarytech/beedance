package transpiler

import (
	"testing"
)

func TestBitwiseOperationsTranspilation(t *testing.T) {
	input := `
PROGRAM BitwiseTest
	VAR
		w1 : WORD := 16#FF00;
		w2 : WORD := 16#00FF;
		w_and : WORD;
		w_or : WORD;
		w_xor : WORD;
		w_not : WORD;
		b_true : BOOL := TRUE;
		b_false : BOOL := FALSE;
		b_and : BOOL;
		b_xor : BOOL;
	END_VAR

	w_and := w1 AND w2;
	w_or  := w1 OR w2;
	w_xor := w1 XOR w2;
	w_not := NOT w1;
	b_and := b_true AND b_false;
	b_xor := b_true XOR b_false;
END_PROGRAM
`
	expected := `
type BitwiseTest struct {
	w1      iec.WORD
	w2      iec.WORD
	w_and   iec.WORD
	w_or    iec.WORD
	w_xor   iec.WORD
	w_not   iec.WORD
	b_true  iec.BOOL
	b_false iec.BOOL
	b_and   iec.BOOL
	b_xor   iec.BOOL
}

// NewBitwiseTestFactory creates a new instance of the BitwiseTest program.
func NewBitwiseTestFactory(params map[string]string) (func(time.Time), error) {
	instance := &BitwiseTest{}
	instance.w1 = 65280
	instance.w2 = 255
	instance.b_true = true
	instance.b_false = false
	return instance.Logic, nil
}

// Link connects the program's located variables to the runtime's I/O manager.
func (p *BitwiseTest) Link(linker config.IOLinker) error {
	return nil
}

func (p *BitwiseTest) Logic(now time.Time) {
	p.w_and = (p.w1 & p.w2)
	p.w_or = (p.w1 | p.w2)
	p.w_xor = (p.w1 ^ p.w2)
	p.w_not = ^(p.w1)
	p.b_and = (p.b_true && p.b_false)
	p.b_xor = (p.b_true != p.b_false)
}
`
	transpileAndCheck(t, "TestBitwiseOperationsTranspilation", input, expected)
}
