/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under:
 * - GPL v2.0
 * - Commercial
 *
 * You may choose to use this software under the terms of either license.
 * See the LICENSE files in the project root for full license text.
 */

package transpiler

import (
	. "beedance/token"
)

// ClampSINT returns v clamped to the range [min, max].
func ClampSINT(v, min, max SINT) SINT {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ClampINT returns v clamped to the range [min, max].
func ClampINT(v, min, max INT) INT {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ClampDINT returns v clamped to the range [min, max].
func ClampDINT(v, min, max DINT) DINT {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ClampLINT returns v clamped to the range [min, max].
func ClampLINT(v, min, max LINT) LINT {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// Note: Unsigned integer subranges are less common, but could be added here
// following the same pattern (e.g., ClampUSINT, ClampUINT, etc.) if needed.
