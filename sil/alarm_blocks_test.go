/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package sil_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/apiarytech/beedance/sil"
)

// alarmBlocksTest drives beeguard's reference alarm blocks, unchanged
// (IEC-68 and IEC-69: they jump in ST and clear their own one-shot
// requests): a high alarm on BG_ALM_ANA acknowledged by an operator, and a
// 2 s on-delay on BG_ALM_DIG, 300 scans 10 ms apart.
const alarmBlocksTest = `
PROGRAM TEST_AlarmBlocks
VAR_OUTPUT failures : INT; done : BOOL; END_VAR
VAR ana : BG_ALM_ANA; dig : BG_ALM_DIG; n : INT; END_VAR
n := n + 1;
(* the analog alarm: high at 80, the value 90 from scan 10 *)
IF n = 50 THEN ana.CMD_OP_ACK_ALL := TRUE; END_IF;
ana(IN_VALUE := SEL(n >= 10, 50.0, 90.0), CFG_H_EN := TRUE, CFG_H_SP := 80.0);
IF n = 20 AND (NOT ana.STS_ALM OR NOT ana.STS_UNACKED) THEN failures := failures + 1; END_IF;
IF n = 51 AND (NOT ana.STS_ALM OR ana.STS_UNACKED OR ana.CMD_OP_ACK_ALL) THEN failures := failures + 1; END_IF;
(* the digital alarm: in alarm from scan 1, announced after its 2 s on-delay *)
IF n = 250 THEN dig.CMD_OP_ACK := TRUE; END_IF;
dig(IN_SIGNAL := TRUE, CFG_ACK_REQ := TRUE, CFG_ON_DELAY_MS := 2000);
IF n = 150 AND dig.STS_ALM THEN failures := failures + 1; END_IF;
IF n = 240 AND (NOT dig.STS_ALM OR NOT dig.STS_UNACKED) THEN failures := failures + 1; END_IF;
IF n = 251 AND (NOT dig.STS_ALM OR dig.STS_UNACKED OR dig.CMD_OP_ACK) THEN failures := failures + 1; END_IF;
IF n = 300 THEN done := TRUE; END_IF;
END_PROGRAM
`

// TestBeeguardAlarmBlocks is IEC-69's done-when: the two blocks, as beeguard
// ships them, compile and pass on every engine with input writes allowed.
func TestBeeguardAlarmBlocks(t *testing.T) {
	var src []byte
	for _, f := range []string{"digital_alarm.st", "analog_alarm.st"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "beeguard", "reference", f))
		if err != nil {
			t.Skipf("beeguard's reference blocks are not beside beedance: %v", err)
		}
		src = append(append(src, b...), '\n')
	}
	src = append(src, alarmBlocksTest...)
	engines := inputWriteEngines()
	results, err := sil.Run(context.Background(), string(src), sil.Options{Engines: engines, Interval: 10 * time.Millisecond,
		MaxScans: 400, GoTimeout: 2 * time.Minute, AllowInputWrites: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if len(r.Engines) != len(engines) || !r.Passed {
			t.Errorf("%s: passed %v on %d engines, mismatch %q", r.Name, r.Passed, len(r.Engines), r.Mismatch)
			for _, e := range r.Engines {
				t.Logf("    %s: %v %s", e.Engine, e.Problems(), e.Err)
			}
		}
	}
}
