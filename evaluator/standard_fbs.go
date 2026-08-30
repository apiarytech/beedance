package evaluator

import (
	"beedance/object"
	"time"
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
		if t, ok := startTimeObj.(*object.TimeOfDay); ok {
			startTime = t.Value
		}
	}
	timerActive := timerActiveObj == TRUE

	var et time.Duration
	q := FALSE

	if inBool.Value {
		if !timerActive {
			// Rising edge of IN: start the timer
			instanceEnv.Set("__startTime", &object.TimeOfDay{Value: nowFunc()})
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
		instanceEnv.Set("__timerActive", FALSE)
		instanceEnv.Set("__startTime", nil)
		et = 0
		q = FALSE
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
		if t, ok := stopTimeObj.(*object.TimeOfDay); ok {
			stopTime = t.Value
		}
	}

	var et time.Duration
	q := FALSE

	if inBool.Value {
		instanceEnv.Set("__stopTime", nil)
		q = TRUE
		et = 0
	} else {
		// Falling edge of IN
		if stopTime.IsZero() {
			stopTime = nowFunc()
			instanceEnv.Set("__stopTime", &object.TimeOfDay{Value: stopTime})
		}

		et = nowFunc().Sub(stopTime)
		if et < ptDuration.Value {
			q = TRUE
		} else {
			et = ptDuration.Value
			q = FALSE
		}
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
		if t, ok := startTimeObj.(*object.TimeOfDay); ok {
			startTime = t.Value
		}
	}
	pulseActive := pulseActiveObj == TRUE
	lastINBool := lastIN == TRUE

	var et time.Duration
	q := FALSE

	if inBool.Value && !lastINBool { // Rising edge
		pulseActive = true
		startTime = nowFunc()
		instanceEnv.Set("__pulseActive", TRUE)
		instanceEnv.Set("__startTime", &object.TimeOfDay{Value: startTime})
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
		}
	} else {
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

	lastCUBool := lastCU == TRUE
	var cv int64
	if cvInt, ok := cvObj.(*object.LInt); ok {
		cv = cvInt.Value
	}

	if rBool.Value {
		cv = 0
	} else if cuBool.Value && !lastCUBool { // Rising edge on CU
		if cv < pvInt {
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

	lastCDBool := lastCD == TRUE
	var cv int64
	if cvInt, ok := cvObj.(*object.LInt); ok {
		cv = cvInt.Value
	}

	if ldBool.Value {
		cv = pvInt
	} else if cdBool.Value && !lastCDBool { // Rising edge on CD
		if cv > 0 {
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

	lastCUBool := lastCU == TRUE
	lastCDBool := lastCD == TRUE
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
			if cv < pvInt {
				cv++
			}
		} else if cdRising && !cuRising {
			if cv > 0 {
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

	edgeMem := edgeMemObj == TRUE
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

	edgeMem := edgeMemObj == TRUE
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

	q1 := q1Obj == TRUE

	if rBool.Value {
		q1 = false
	} else if s1Bool.Value {
		q1 = true
	}

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

	q1 := q1Obj == TRUE

	if sBool.Value {
		q1 = true
	} else if r1Bool.Value {
		q1 = false
	}

	q1Result := nativeBoolToBooleanObject(q1)
	instanceEnv.Set("Q1", q1Result)

	return q1Result
}
