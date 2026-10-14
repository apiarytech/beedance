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
	"strings"
	"testing"
)

const compactImageSource = `
PROGRAM Main
VAR
    start AT %IX0.3 : BOOL;
    lamp AT %QX1.2 : BOOL;
    speed AT %QW4 : INT;
END_VAR
lamp := start;
speed := 7;
END_PROGRAM
`

// CompactImage declares only the entries the located variables use, and
// the code reads and writes them as it does a vars.Image.
func TestCompactImage(t *testing.T) {
	out, err := GoFileWith(parseForTest(t, compactImageSource), Options{CompactImage: true})
	if err != nil {
		t.Fatal(err)
	}
	code := string(out)
	for _, want := range []string{
		"B [11]iec.BOOL", // %QX1.2 is entry 10
		"W [5]iec.WORD",  // %QW4
		"processImage.Read(func(img *processImageData) {",
		"defer processImage.Write(func(img *processImageData) {",
		"img.Q.B[10] = iec.BOOL(p.lamp)",
		"var processImage processImageGuard",
		`"sync"`,
	} {
		if !strings.Contains(code, want) {
			t.Errorf("missing %q in:\n%s", want, code)
		}
	}
	for _, unwanted := range []string{"processImage vars.ProcessImage", "img *vars.Image", "royaljelly/vars"} {
		if strings.Contains(code, unwanted) {
			t.Errorf("%q in:\n%s", unwanted, code)
		}
	}
}

// Without CompactImage the file keeps royaljelly's vars.ProcessImage.
func TestDefaultImageIsRoyaljellys(t *testing.T) {
	out, err := GoFileWith(parseForTest(t, compactImageSource), Options{})
	if err != nil {
		t.Fatal(err)
	}
	code := string(out)
	if !strings.Contains(code, "var processImage vars.ProcessImage") || strings.Contains(code, "processImageData") {
		t.Errorf("default output changed:\n%s", code)
	}
}
