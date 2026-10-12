/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package esp32hw_test

import (
	"os/exec"
	"testing"
)

// plc/program/program.go is what program.st gives: regenerate it with
// go generate ./embedded/esp32hw/plc/ after changing either, or the
// generator.
func TestGeneratedProgramUpToDate(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no Go toolchain")
	}
	out, err := exec.Command(goTool, "run", "./plc/gen", "-dir", "plc", "-check").CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}
