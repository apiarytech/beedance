package evaluator

import (
	"math"
	"time"

	"github.com/apiarytech/beedance/object"
)

// evalTON implements the logic for the TON (Timer On-Delay) standard function block.
func evalTON(instanceEnv, callEnv *object.Environment) object.Object {
	// 1. Get inputs from the instance environment (set by applyFunction)
	in, _ := instanceEnv.Get("IN")
	pt, _ := instanceEnv.Get("PT")

	// 2. Get internal state variables from the instance environment
	startTimeObj, _ := instanceEnv.Get("__startTime")
	timerActiveObj, _ := instanceEnv.Get("__timerActive")

	// 3. Type assertions and defaults
	inBool, _ := in.(*object.Boolean)
	ptDuration, _ := pt.(*object.Time)
	if inBool == nil || ptDuration == nil {
		return object.NewBuiltinError("TON requires IN (BOOL) and PT (TIME) inputs")
	}

	var startTime time.Time
	if startTimeObj != nil {
		if t, ok := startTimeObj.(*object.DateAndTime); ok {
			startTime = t.Value
		}
	}
	timerActive := isBoolTrue(timerActiveObj)

	var et time.Duration
	q := FALSE

	if inBool.Value {
		if !timerActive {
			// Rising edge of IN: start the timer
			instanceEnv.Set("__startTime", &object.DateAndTime{Value: nowFunc()})
			instanceEnv.Set("__timerActive", TRUE)
			startTime = nowFunc()
		}

		et = nowFunc().Sub(startTime)
		if et >= ptDuration.Value {
			et = ptDuration.Value
			q = TRUE
		}
	} else {
		// IN is FALSE: reset the timer
		if timerActive { // Only reset if it was previously active
			instanceEnv.Set("__timerActive", FALSE)
			instanceEnv.Set("__startTime", nil)
			et = 0
			q = FALSE
		}
	}

	// 4. Set outputs in the instance environment
	instanceEnv.Set("Q", q)
	instanceEnv.Set("ET", &object.Time{Value: et})

	return q // The primary output of TON is Q
}

// evalTOF implements the logic for the TOF (Timer Off-Delay) standard function block.
func evalTOF(instanceEnv, callEnv *object.Environment) object.Object {
	in, _ := instanceEnv.Get("IN")
	pt, _ := instanceEnv.Get("PT")
	stopTimeObj, _ := instanceEnv.Get("__stopTime")

	inBool, _ := in.(*object.Boolean)
	ptDuration, _ := pt.(*object.Time)
	if inBool == nil || ptDuration == nil {
		return object.NewBuiltinError("TOF requires IN (BOOL) and PT (TIME) inputs")
	}

	var stopTime time.Time
	if stopTimeObj != nil {
		if t, ok := stopTimeObj.(*object.DateAndTime); ok {
			stopTime = t.Value
		}
	}

	// The delay runs only while Q is on, as in the compiled TOF: a TOF whose
	// IN has not been on keeps Q off, and with PT = T#0s Q goes off in the
	// scan IN falls.
	prevQ, _ := instanceEnv.Get("Q")
	if inBool.Value {
		instanceEnv.Set("__stopTime", nil)
		instanceEnv.Set("Q", TRUE)
		instanceEnv.Set("ET", &object.Time{Value: 0})
		return TRUE
	}
	if b, ok := prevQ.(*object.Boolean); !ok || !b.Value {
		return FALSE // Q and ET stay as they are
	}

	// IN is off and Q on: IN has fallen, or the delay is running.
	if stopTime.IsZero() {
		stopTime = nowFunc()
		instanceEnv.Set("__stopTime", &object.DateAndTime{Value: stopTime})
	}
	et := nowFunc().Sub(stopTime)
	q := TRUE
	if et >= ptDuration.Value {
		et = ptDuration.Value
		q = FALSE
		instanceEnv.Set("__stopTime", nil)
	}

	instanceEnv.Set("Q", q)
	instanceEnv.Set("ET", &object.Time{Value: et})

	return q
}

// evalTP implements the logic for the TP (Pulse Timer) standard function block.
func evalTP(instanceEnv, callEnv *object.Environment) object.Object {
	in, _ := instanceEnv.Get("IN")
	pt, _ := instanceEnv.Get("PT")
	startTimeObj, _ := instanceEnv.Get("__startTime")
	pulseActiveObj, _ := instanceEnv.Get("__pulseActive")
	lastIN, _ := instanceEnv.Get("__lastIN")

	inBool, _ := in.(*object.Boolean)
	ptDuration, _ := pt.(*object.Time)
	if inBool == nil || ptDuration == nil {
		return object.NewBuiltinError("TP requires IN (BOOL) and PT (TIME) inputs")
	}

	var startTime time.Time
	if startTimeObj != nil {
		if t, ok := startTimeObj.(*object.DateAndTime); ok {
			startTime = t.Value
		}
	}
	pulseActive := isBoolTrue(pulseActiveObj)
	lastINBool := isBoolTrue(lastIN)

	var et time.Duration
	q := FALSE

	// A rising edge starts a pulse only when none is running: TP is not
	// retriggerable (IEC 61131-3), as the VM's and royaljelly's TP.
	if inBool.Value && !lastINBool && !pulseActive {
		pulseActive = true
		startTime = nowFunc()
		instanceEnv.Set("__pulseActive", TRUE)
		instanceEnv.Set("__startTime", &object.DateAndTime{Value: startTime})
	}

	if pulseActive {
		et = nowFunc().Sub(startTime)
		if et < ptDuration.Value {
			q = TRUE
		} else {
			et = ptDuration.Value
			q = FALSE
			pulseActive = false
			instanceEnv.Set("__pulseActive", FALSE)
			instanceEnv.Set("__startTime", nil)
			instanceEnv.Set("__pulseDone", TRUE)
		}
	} else if done, _ := instanceEnv.Get("__pulseDone"); inBool.Value && isBoolTrue(done) {
		et = ptDuration.Value // the pulse is over; ET stays PT while IN is TRUE
		q = FALSE
	} else {
		instanceEnv.Set("__pulseDone", FALSE)
		et = 0
		q = FALSE
	}

	instanceEnv.Set("__lastIN", inBool)
	instanceEnv.Set("Q", q)
	instanceEnv.Set("ET", &object.Time{Value: et})

	return q
}

// evalCTU implements the logic for the CTU (Counter Up) standard function block.
func evalCTU(instanceEnv, callEnv *object.Environment) object.Object {
	cu, _ := instanceEnv.Get("CU")
	r, _ := instanceEnv.Get("R")
	pv, _ := instanceEnv.Get("PV")
	lastCU, _ := instanceEnv.GetRaw("__lastCU")
	cvObj, _ := instanceEnv.Get("CV")

	cuBool, _ := cu.(*object.Boolean)
	rBool, _ := r.(*object.Boolean)
	pvInt, _, ok := object.GetIntegerObjectValue(pv)
	if cuBool == nil || rBool == nil || !ok {
		return object.NewBuiltinError("CTU requires CU (BOOL), R (BOOL), and PV (any INT type) inputs")
	}

	lastCUBool := isBoolTrue(lastCU)
	var cv int64
	if cvInt, ok := cvObj.(*object.LInt); ok {
		cv = cvInt.Value
	}

	if rBool.Value {
		cv = 0
	} else if cuBool.Value && !lastCUBool { // Rising edge on CU
		// CV counts past PV up to PVmax, the INT maximum (IEC 61131-3).
		if cv < math.MaxInt16 {
			cv++
		}
	}

	q := nativeBoolToBooleanObject(cv >= pvInt)

	instanceEnv.Set("__lastCU", cu)
	instanceEnv.Set("Q", q)
	instanceEnv.Set("CV", &object.LInt{Value: cv})

	return q
}

// evalCTD implements the logic for the CTD (Counter Down) standard function block.
func evalCTD(instanceEnv, callEnv *object.Environment) object.Object {
	cdObj, _ := instanceEnv.Get("CD")
	ldObj, _ := instanceEnv.Get("LD")
	pv, _ := instanceEnv.Get("PV")
	lastCD, _ := instanceEnv.GetRaw("__lastCD")
	cvObj, _ := instanceEnv.Get("CV")

	cdBool := nativeBoolToBooleanObject(object.IsTruthy(cdObj))
	ldBool := nativeBoolToBooleanObject(object.IsTruthy(ldObj))
	pvInt, _, ok := object.GetIntegerObjectValue(pv)
	if !ok {
		return object.NewBuiltinError("CTD requires a PV (Preset Value) input of an integer type")
	}

	lastCDBool := isBoolTrue(lastCD)
	var cv int64
	if cvInt, ok := cvObj.(*object.LInt); ok {
		cv = cvInt.Value
	}

	if ldBool.Value {
		cv = pvInt
	} else if cdBool.Value && !lastCDBool { // Rising edge on CD
		if cv > math.MinInt16 { // down to PVmin, the INT minimum
			cv--
		}
	}

	q := nativeBoolToBooleanObject(cv <= 0)

	instanceEnv.Set("__lastCD", cdBool)
	instanceEnv.Set("Q", q)
	instanceEnv.Set("CV", &object.LInt{Value: cv})

	return q
}

// evalCTUD implements the logic for the CTUD (Up/Down Counter) standard function block.
func evalCTUD(instanceEnv, callEnv *object.Environment) object.Object {
	cu, _ := instanceEnv.Get("CU")
	cd, _ := instanceEnv.Get("CD")
	r, _ := instanceEnv.Get("R")
	ld, _ := instanceEnv.Get("LD")
	pv, _ := instanceEnv.Get("PV")
	lastCU, _ := instanceEnv.GetRaw("__lastCU")
	lastCD, _ := instanceEnv.GetRaw("__lastCD")
	cvObj, _ := instanceEnv.Get("CV")

	cuBool, _ := cu.(*object.Boolean)
	cdBool, _ := cd.(*object.Boolean)
	rBool, _ := r.(*object.Boolean)
	ldBool, _ := ld.(*object.Boolean)
	pvInt, _, ok := object.GetIntegerObjectValue(pv)
	if cuBool == nil || cdBool == nil || rBool == nil || ldBool == nil || !ok {
		return object.NewBuiltinError("CTUD requires CU, CD, R, LD (BOOL) and PV (INT) inputs")
	}

	lastCUBool := isBoolTrue(lastCU)
	lastCDBool := isBoolTrue(lastCD)
	var cv int64
	if cvInt, ok := cvObj.(*object.LInt); ok {
		cv = cvInt.Value
	}

	if rBool.Value {
		cv = 0
	} else if ldBool.Value {
		cv = pvInt
	} else {
		cuRising := cuBool.Value && !lastCUBool
		cdRising := cdBool.Value && !lastCDBool
		if cuRising && !cdRising {
			if cv < math.MaxInt16 { // up to PVmax, the INT maximum
				cv++
			}
		} else if cdRising && !cuRising {
			if cv > math.MinInt16 { // down to PVmin, the INT minimum
				cv--
			}
		}
	}

	instanceEnv.Set("__lastCU", cu)
	instanceEnv.Set("__lastCD", cd)
	instanceEnv.Set("CV", &object.LInt{Value: cv})
	instanceEnv.Set("QU", nativeBoolToBooleanObject(cv >= pvInt))
	instanceEnv.Set("QD", nativeBoolToBooleanObject(cv <= 0))

	return NULL
}

// evalR_TRIG implements the logic for the R_TRIG (Rising Edge Trigger) standard function block.
func evalR_TRIG(instanceEnv, callEnv *object.Environment) object.Object {
	clkObj, _ := instanceEnv.Get("CLK")
	edgeMemObj, _ := instanceEnv.Get("__edge_mem")

	clk, ok := clkObj.(*object.Boolean)
	if !ok {
		instanceEnv.Set("Q", FALSE)
		instanceEnv.Set("__edge_mem", FALSE)
		return FALSE
	}

	edgeMem := isBoolTrue(edgeMemObj)
	q := nativeBoolToBooleanObject(clk.Value && !edgeMem)

	instanceEnv.Set("__edge_mem", clk)
	instanceEnv.Set("Q", q)

	return q
}

// evalF_TRIG implements the logic for the F_TRIG (Falling Edge Trigger) standard function block.
func evalF_TRIG(instanceEnv, callEnv *object.Environment) object.Object {
	clkObj, _ := instanceEnv.Get("CLK")
	edgeMemObj, _ := instanceEnv.Get("__edge_mem")

	clk, ok := clkObj.(*object.Boolean)
	if !ok {
		instanceEnv.Set("Q", FALSE)
		instanceEnv.Set("__edge_mem", FALSE)
		return FALSE
	}

	edgeMem := isBoolTrue(edgeMemObj)
	q := nativeBoolToBooleanObject(!clk.Value && edgeMem)

	instanceEnv.Set("__edge_mem", clk)
	instanceEnv.Set("Q", q)

	return q
}

// evalSR implements the logic for the SR (Set-Reset) bistable function block.
func evalSR(instanceEnv, callEnv *object.Environment) object.Object {
	s1, _ := instanceEnv.Get("S1")
	r, _ := instanceEnv.Get("R")
	q1Obj, _ := instanceEnv.Get("Q1")

	s1Bool, _ := s1.(*object.Boolean)
	rBool, _ := r.(*object.Boolean)
	if s1Bool == nil || rBool == nil {
		return object.NewBuiltinError("SR requires S1 (BOOL) and R (BOOL) inputs")
	}

	// Set dominant (IEC 61131-3): Q1 := S1 OR (NOT R AND Q1).
	q1 := s1Bool.Value || (!rBool.Value && isBoolTrue(q1Obj))

	q1Result := nativeBoolToBooleanObject(q1)
	instanceEnv.Set("Q1", q1Result)

	return q1Result
}

// evalRS implements the logic for the RS (Reset-Set) bistable function block.
func evalRS(instanceEnv, callEnv *object.Environment) object.Object {
	s, _ := instanceEnv.Get("S")
	r1, _ := instanceEnv.Get("R1")
	q1Obj, _ := instanceEnv.Get("Q1")

	sBool, _ := s.(*object.Boolean)
	r1Bool, _ := r1.(*object.Boolean)
	if sBool == nil || r1Bool == nil {
		return object.NewBuiltinError("RS requires S (BOOL) and R1 (BOOL) inputs")
	}

	// Reset dominant (IEC 61131-3): Q1 := NOT R1 AND (S OR Q1).
	q1 := !r1Bool.Value && (sBool.Value || isBoolTrue(q1Obj))

	q1Result := nativeBoolToBooleanObject(q1)
	instanceEnv.Set("Q1", q1Result)

	return q1Result
}
