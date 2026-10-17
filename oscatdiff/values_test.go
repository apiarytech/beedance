/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package oscatdiff

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/apiarytech/beedance/object"
)

// goTypeName is the name of the royaljelly iec type a port uses for an IEC
// type, by IEC name.
var goTypeName = map[string]string{
	"BOOL": "BOOL", "BYTE": "BYTE", "WORD": "WORD", "DWORD": "DWORD", "LWORD": "LWORD",
	"SINT": "SINT", "INT": "INT", "DINT": "DINT", "LINT": "LINT",
	"USINT": "USINT", "UINT": "UINT", "UDINT": "UDINT", "ULINT": "ULINT",
	"REAL": "REAL", "LREAL": "LREAL", "STRING": "STRING",
	"TIME": "TIME", "DATE": "DATE", "TOD": "TIME_OF_DAY", "TIME_OF_DAY": "TIME_OF_DAY",
	"DT": "DT", "DATE_AND_TIME": "DT",
}

const iecPkg = "github.com/apiarytech/royaljelly/iec"

// iecType returns the IEC elementary type of an ST type name, such as
// STRING for STRING(20), and whether the harness supports it.
func iecType(st string) (string, bool) {
	st = strings.ToUpper(strings.TrimSpace(st))
	if i := strings.IndexAny(st, "(["); i >= 0 {
		st = strings.TrimSpace(st[:i])
	}
	_, ok := goTypeName[st]
	return st, ok
}

// matches reports whether the Go type of a port's parameter or result is
// the one for the IEC type st.
func matches(st string, t reflect.Type) bool {
	return t.PkgPath() == iecPkg && t.Name() == goTypeName[st]
}

// seed returns the seed of a POU's random cases, so that a run repeats.
func seed(name string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(strings.ToUpper(name)))
	return h.Sum64()
}

// value is an argument of a call: the Go value for the port and the ST
// literal for the evaluator.
type value struct {
	goV reflect.Value
	lit string
}

var (
	realEdges = []float64{0, 1, -1, 0.5, -0.5, 2, 3.14159, 10, -10, 100, -100, 1000, 1e6, -1e6, 1e-3}
	// No string has a $ or a ': the evaluator does not decode $$ and $'.
	stringEdges = []string{"", "a", "A", "0", "123", "-12.5", "Hello World", " x ", "abc,def", "ABCdef 123 xyz",
		strings.Repeat("0123456789", 8)}
	timeEdges = []time.Duration{0, time.Millisecond, 999 * time.Millisecond, time.Second, time.Minute, time.Hour, 24 * time.Hour}
	dateEdges = []string{"1970-01-01", "2000-02-29", "2024-03-31", "2024-12-31", "2099-12-31", "2026-10-13"}
)

const stringChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 ,.;:-_+/=()"

// intRange is the range of an IEC integer type.
func intRange(st string) (lo, hi int64, unsigned bool) {
	switch st {
	case "SINT":
		return math.MinInt8, math.MaxInt8, false
	case "INT":
		return math.MinInt16, math.MaxInt16, false
	case "DINT":
		return math.MinInt32, math.MaxInt32, false
	case "LINT":
		return math.MinInt64, math.MaxInt64, false
	case "BYTE", "USINT":
		return 0, math.MaxUint8, true
	case "WORD", "UINT":
		return 0, math.MaxUint16, true
	case "DWORD", "UDINT":
		return 0, math.MaxUint32, true
	}
	return 0, math.MaxInt64, true // LWORD, ULINT: the harness stays below 2^63
}

// genValue returns an argument of the IEC type st for the Go type t: an
// edge of the type's range on the first cases, a random value later.
func genValue(r *rand.Rand, st string, t reflect.Type, i int) value {
	v := reflect.New(t).Elem()
	edge := i < 24 || r.IntN(4) == 0
	switch st {
	case "BOOL":
		b := r.IntN(2) == 1
		v.SetBool(b)
		return value{v, strings.ToUpper(strconv.FormatBool(b))}
	case "REAL", "LREAL":
		var f float64
		switch {
		case edge:
			f = realEdges[r.IntN(len(realEdges))]
		case r.IntN(3) == 0:
			f = r.Float64()
		case r.IntN(2) == 0:
			f = r.Float64()*200 - 100
		default:
			f = r.Float64()*2e4 - 1e4
		}
		if st == "REAL" {
			f = float64(float32(f))
		}
		v.SetFloat(f)
		return value{v, st + "#" + realLit(f, st)}
	case "STRING":
		var s string
		if edge {
			s = stringEdges[r.IntN(len(stringEdges))]
		} else {
			b := make([]byte, r.IntN(21))
			for k := range b {
				b[k] = stringChars[r.IntN(len(stringChars))]
			}
			s = string(b)
		}
		v.SetString(s)
		return value{v, stringLit(s)}
	case "TIME":
		d := time.Duration(r.Int64N(1e8)) * time.Millisecond
		if edge {
			d = timeEdges[r.IntN(len(timeEdges))]
		}
		v.SetInt(int64(d))
		return value{v, fmt.Sprintf("T#%dms", d.Milliseconds())}
	case "DATE", "DT", "DATE_AND_TIME", "TOD", "TIME_OF_DAY":
		day := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, r.IntN(47482)) // to 2099
		if edge {
			day, _ = time.Parse("2006-01-02", dateEdges[r.IntN(len(dateEdges))])
		}
		switch st {
		case "DATE":
			v.Set(reflect.ValueOf(day).Convert(t))
			return value{v, "D#" + day.Format("2006-01-02")}
		case "DT", "DATE_AND_TIME":
			dt := day.Add(time.Duration(r.IntN(86400)) * time.Second)
			v.Set(reflect.ValueOf(dt).Convert(t))
			return value{v, "DT#" + dt.Format("2006-01-02-15:04:05")}
		}
		ms := r.IntN(86400000)
		if edge {
			ms = []int{0, 43200000, 86399999, 3600000}[r.IntN(4)]
		}
		// The ports count a time of day from the zero time.
		tod := time.Time{}.Add(time.Duration(ms) * time.Millisecond)
		v.Set(reflect.ValueOf(tod).Convert(t))
		return value{v, "TOD#" + tod.Format("15:04:05.000")}
	}
	lo, hi, unsigned := intRange(st)
	var n int64
	switch {
	case edge:
		edges := []int64{0, 1, 2, 10, lo, hi, hi / 2, hi/2 + 1}
		if !unsigned {
			edges = append(edges, -1, -2, -10)
		}
		n = edges[r.IntN(len(edges))]
	case r.IntN(2) == 0:
		n = max(lo, min(hi, r.Int64N(111)-10)) // a small value, as most inputs are
	default:
		n = lo + r.Int64N(hi-lo) // hi-lo fits: the harness stays below 2^63
		if lo == math.MinInt64 {
			n = r.Int64()
		}
	}
	if unsigned {
		v.SetUint(uint64(n))
	} else {
		v.SetInt(n)
	}
	return value{v, fmt.Sprintf("%s#%d", st, n)}
}

// realLit writes a REAL or LREAL as an ST literal, which needs a decimal
// point before its exponent.
func realLit(f float64, st string) string {
	bits := 64
	if st == "REAL" {
		bits = 32
	}
	s := strconv.FormatFloat(f, 'E', -1, bits)
	mant, exp, _ := strings.Cut(s, "E")
	if !strings.Contains(mant, ".") {
		mant += ".0"
	}
	return mant + "E" + exp
}

// stringLit writes s as an ST string literal.
func stringLit(s string) string {
	s = strings.ReplaceAll(s, "$", "$$")
	return "'" + strings.ReplaceAll(s, "'", "$'") + "'"
}

// realTolerance is how far a REAL result of a port may be from the
// evaluator's, relative to the larger of 1 and the result: the ports compute
// in 32 bits, as a PLC does, and the evaluator in 64.
const realTolerance = 1e-4

// precisionTolerance is how far a REAL result may be from the evaluator's
// to count as a difference of precision, not of the calculation: 32 bit
// rounding grows in a long calculation, and more where it cancels.
const precisionTolerance = 1e-2

// verdict is how a port's result compares with the evaluator's.
type verdict int

const (
	agree     verdict = iota
	wide              // agree once the evaluator's integer result is wrapped to its type
	precision         // REAL results apart by less than precisionTolerance
	evalError         // the evaluator fails where the port returns
	bothFail          // the evaluator fails and the port panics
	mismatch
)

// compare compares a port's result with the evaluator's for the IEC type
// st, and returns the evaluator's result as text, or its error.
func compare(st string, goV reflect.Value, ev object.Object) (verdict, string) {
	if e, ok := ev.(*object.Error); ok {
		return evalError, e.Message
	}
	switch st {
	case "BOOL":
		if b, ok := ev.(*object.Boolean); ok && b.Value == goV.Bool() {
			return agree, ""
		}
		// The evaluator keeps an integer assigned to a BOOL, such as OSCAT's
		// Q := 0, as the integer.
		if n, ok := intOf(ev); ok && (n == 0 || n == 1) && (n == 1) == goV.Bool() {
			return wide, ""
		}
	case "REAL", "LREAL":
		f, ok := floatOf(ev)
		if !ok {
			break
		}
		if st == "REAL" {
			f = float64(float32(f))
		}
		g := goV.Float()
		if (math.IsNaN(f) && math.IsNaN(g)) || f == g ||
			math.Abs(f-g) <= realTolerance*math.Max(1, math.Max(math.Abs(f), math.Abs(g))) {
			return agree, ""
		}
		if math.Abs(f-g) <= precisionTolerance*math.Max(1, math.Max(math.Abs(f), math.Abs(g))) {
			return precision, ev.Inspect()
		}
	case "STRING":

		if s, ok := ev.(*object.String); ok && s.Value == goV.String() {
			return agree, ""
		}
	case "TIME":
		if d, ok := ev.(*object.Time); ok && int64(d.Value) == goV.Int() {
			return agree, ""
		}
	case "DATE":
		if d, ok := ev.(*object.Date); ok && d.Value.Format("2006-01-02") == timeOf(goV).Format("2006-01-02") {
			return agree, ""
		}
	case "DT", "DATE_AND_TIME":
		if d, ok := ev.(*object.DateAndTime); ok && d.Value.Unix() == timeOf(goV).Unix() {
			return agree, ""
		}
	case "TOD", "TIME_OF_DAY":
		if d, ok := ev.(*object.TimeOfDay); ok && msOfDay(d.Value) == msOfDay(timeOf(goV)) {
			return agree, ""
		}
	default:
		n, ok := intOf(ev)
		if !ok {
			break
		}
		lo, hi, unsigned := intRange(st)
		isWide := n < lo || n > hi
		// Wrap as the PLC does, then compare.
		w := reflect.New(goV.Type()).Elem()
		if unsigned {
			w.SetUint(uint64(n))
			if w.Uint() == goV.Uint() {
				return wideIf(isWide), ""
			}
		} else {
			w.SetInt(n)
			if w.Int() == goV.Int() {
				return wideIf(isWide), ""
			}
		}
	}
	return mismatch, ev.Inspect()
}

// latin1 returns the ISO 8859-1 text s as UTF-8.
func latin1(s string) string {
	r := make([]rune, len(s))
	for i := range len(s) {
		r[i] = rune(s[i])
	}
	return string(r)
}

func wideIf(isWide bool) verdict {
	if isWide {
		return wide
	}
	return agree
}

// intOf returns the value of an integer or bit string object.
func intOf(ev object.Object) (int64, bool) {
	switch o := ev.(type) {
	case *object.BitString:
		return int64(o.Value), true
	case *object.Byte:
		return int64(o.Value), true
	case *object.Word:
		return int64(o.Value), true
	case *object.DWord:
		return int64(o.Value), true
	case *object.LWord:
		return int64(o.Value), true
	case *object.Boolean:
		if o.Value {
			return 1, true
		}
		return 0, true
	}
	n, _, ok := object.GetIntegerObjectValue(ev)
	return n, ok
}

// floatOf returns the value of a numeric object.
func floatOf(ev object.Object) (float64, bool) {
	if f, ok := object.GetFloat64Value(ev); ok {
		return f, true
	}
	n, ok := intOf(ev)
	return float64(n), ok
}

// timeOf returns the time.Time of an iec DATE, DT or TIME_OF_DAY.
func timeOf(v reflect.Value) time.Time {
	return v.Convert(reflect.TypeFor[time.Time]()).Interface().(time.Time)
}

func msOfDay(t time.Time) int {
	return t.Hour()*3600000 + t.Minute()*60000 + t.Second()*1000 + t.Nanosecond()/1e6
}

// goString writes a port's result for a report.
func goString(v reflect.Value) string {
	switch v.Kind() {
	case reflect.String:
		return strconv.Quote(v.String())
	case reflect.Struct:
		return timeOf(v).Format("2006-01-02-15:04:05.000")
	}
	return fmt.Sprint(v.Interface())
}
