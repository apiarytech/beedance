/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package object

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// durationPart matches one number and unit of a duration, such as 1.5s.
var durationPart = regexp.MustCompile(`(\d*\.?\d+)([a-z]+)`)

// ParseDuration parses an IEC 61131-3 duration, such as 1d_12h_30m_5s_10ms
// or -1.5s: numbers with the units d, h, m, s, ms, us and ns, and
// underscores for readability.
func ParseDuration(s string) (time.Duration, error) {
	originalString := s
	isNegative := false
	if strings.HasPrefix(s, "-") {
		isNegative = true
		s = s[1:]
	}

	// Per the standard, underscores are for readability and can be ignored.
	s = strings.ReplaceAll(s, "_", "")
	// Work with lowercase for unit matching.
	s = strings.ToLower(s)

	if s == "" {
		// An empty string after the prefix (e.g., T#) is not a valid duration.
		return 0, fmt.Errorf("invalid duration string: empty")
	}

	totalDuration := time.Duration(0)
	remaining := s

	// Each part is a number (integer or real) followed by its unit.
	matches := durationPart.FindAllStringSubmatch(remaining, -1)

	if len(matches) == 0 && remaining != "" {
		return 0, fmt.Errorf("invalid duration format in %q", originalString)
	}

	parsedStr := ""
	for _, match := range matches {
		numPart := match[1]
		unitPart := match[2]
		parsedStr += numPart + unitPart

		val, err := strconv.ParseFloat(numPart, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid number %q in duration string %q", numPart, originalString)
		}

		var unitDuration time.Duration
		switch unitPart {
		case "d":
			unitDuration = 24 * time.Hour
		case "h":
			unitDuration = time.Hour
		case "m":
			unitDuration = time.Minute
		case "s":
			unitDuration = time.Second
		case "ms":
			unitDuration = time.Millisecond
		case "us":
			unitDuration = time.Microsecond
		case "ns":
			unitDuration = time.Nanosecond
		default:
			return 0, fmt.Errorf("unknown duration unit %q in string %q", unitPart, originalString)
		}
		totalDuration += time.Duration(val * float64(unitDuration))
	}

	// Check if the entire string was parsed by the regex.
	if parsedStr != s {
		return 0, fmt.Errorf("unparsed characters in duration string %q", originalString)
	}

	if isNegative {
		totalDuration = -totalDuration
	}
	return totalDuration, nil
}
