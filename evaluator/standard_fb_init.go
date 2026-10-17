package evaluator

import (
	"github.com/apiarytech/beedance/ast"
	"github.com/apiarytech/beedance/object"
)

// standardFBInitial are the inputs and outputs of the standard function
// blocks with their IEC 61131-3 initial values. An instance has them from
// its declaration, so a program may read an output before the block's first
// call (common in SFC and in code generated from LD and FBD), and a call
// may leave inputs out (an unwired pin in a diagram): an input not given
// keeps its value, as on a PLC.
var standardFBInitial = map[string]func() map[string]object.Object{
	"TON": timerInitial,
	"TOF": timerInitial,
	"TP":  timerInitial,
	"CTU": func() map[string]object.Object {
		return withCounter(map[string]object.Object{"CU": FALSE, "R": FALSE, "Q": FALSE})
	},
	"CTD": func() map[string]object.Object {
		return withCounter(map[string]object.Object{"CD": FALSE, "LD": FALSE, "Q": FALSE})
	},
	"CTUD": func() map[string]object.Object {
		return withCounter(map[string]object.Object{"CU": FALSE, "CD": FALSE, "R": FALSE, "LD": FALSE, "QU": FALSE, "QD": FALSE})
	},
	"R_TRIG": func() map[string]object.Object { return map[string]object.Object{"CLK": FALSE, "Q": FALSE} },
	"F_TRIG": func() map[string]object.Object { return map[string]object.Object{"CLK": FALSE, "Q": FALSE} },
	"SR":     func() map[string]object.Object { return map[string]object.Object{"S1": FALSE, "R": FALSE, "Q1": FALSE} },
	"RS":     func() map[string]object.Object { return map[string]object.Object{"S": FALSE, "R1": FALSE, "Q1": FALSE} },
}

func timerInitial() map[string]object.Object {
	return map[string]object.Object{"IN": FALSE, "PT": &object.Time{Value: 0}, "Q": FALSE, "ET": &object.Time{Value: 0}}
}

// withCounter adds a counter's preset and count, both 0.
func withCounter(m map[string]object.Object) map[string]object.Object {
	m["PV"] = &object.LInt{Value: 0}
	m["CV"] = &object.LInt{Value: 0}
	return m
}

// initStandardFBOutputs gives a new instance of a standard function block
// its inputs' and outputs' initial values.
func initStandardFBOutputs(fb *object.BuiltinFunctionBlock, env *object.Environment) {
	for name, std := range standardFBs {
		if std != fb {
			continue
		}
		if initial, ok := standardFBInitial[name]; ok {
			for k, v := range initial() {
				env.Set(k, v)
			}
		}
		return
	}
}

// fbInstanceName names an instance's type for messages; a standard block's
// instance has no user definition.
func fbInstanceName(fb *object.FunctionBlockInstance) string {
	if fb.Definition != nil && fb.Definition.Name != nil {
		return fb.Definition.Name.Value
	}
	if logic, ok := fb.Env.Get("__fb_logic__"); ok {
		for name, std := range standardFBs {
			if std == logic {
				return name
			}
		}
	}
	return "function block"
}

// isBoolTrue reports whether o is a BOOL that is TRUE. A BOOL is not always
// the TRUE object: one read from a variable or computed may be a copy, so
// state kept by a block (an edge memory, a last input) is compared by value.
func isBoolTrue(o object.Object) bool {
	b, ok := o.(*object.Boolean)
	return ok && b.Value
}

// isBoolFalse reports whether o is a BOOL that is FALSE.
func isBoolFalse(o object.Object) bool {
	b, ok := o.(*object.Boolean)
	return ok && !b.Value
}

// standardFBInputs are the inputs of each standard function block, in order.
var standardFBInputs = map[string][]string{
	"TON": {"IN", "PT"}, "TOF": {"IN", "PT"}, "TP": {"IN", "PT"},
	"CTU": {"CU", "R", "PV"}, "CTD": {"CD", "LD", "PV"}, "CTUD": {"CU", "CD", "R", "LD", "PV"},
	"R_TRIG": {"CLK"}, "F_TRIG": {"CLK"}, "SR": {"S1", "R"}, "RS": {"S", "R1"},
}

// namedStandardFBInputs returns the arguments of a call of a standard
// function block with each positional one named by its input, as the
// block's inputs are not declarations; or args unchanged.
func namedStandardFBInputs(def object.Object, args []ast.Expression) []ast.Expression {
	fb, ok := def.(*object.BuiltinFunctionBlock)
	if !ok {
		return args
	}
	var inputs []string
	for name, std := range standardFBs {
		if std == fb {
			inputs = standardFBInputs[name]
			break
		}
	}
	if inputs == nil {
		return args
	}
	named := make([]ast.Expression, len(args))
	position := 0
	for i, arg := range args {
		switch arg.(type) {
		case *ast.NamedArgument, *ast.OutputArgument:
			named[i] = arg
		default:
			if position >= len(inputs) {
				return args // too many: left to report
			}
			name := &ast.Identifier{Value: inputs[position]}
			named[i] = &ast.NamedArgument{Name: name, Value: arg}
			position++
		}
	}
	return named
}

// integerAsFloat returns an integer value n, of val, as a float64; a ULINT
// above the int64 range keeps its value.
func integerAsFloat(val object.Object, n int64) float64 {
	if u, ok := val.(*object.ULInt); ok {
		return float64(u.Value)
	}
	return float64(n)
}
