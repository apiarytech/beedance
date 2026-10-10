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
	"strconv"
	"strings"
	"time"
)

// durationParts returns the numbers and units of a duration, such as 1.5s:
// the leftmost matches, in order, of (\d*\.?\d+)([a-z]+), found as Go's
// regexp finds them, so text between or around them is caught by the
// caller. (Not a regexp: a microcontroller build would carry the regexp
// package and its Unicode tables in RAM.)
func durationParts(s string) [][2]string {
	isDigit := func(i int) bool { return i < len(s) && s[i] >= '0' && s[i] <= '9' }
	isLower := func(i int) bool { return i < len(s) && s[i] >= 'a' && s[i] <= 'z' }
	digits := func(i int) int { // the end of the digits from i
		for isDigit(i) {
			i++
		}
		return i
	}
	// match returns the end of the number and of the unit of a match at p,
	// or -1. The number is greedy: digits, then '.' and digits if there
	// are any, else at least one digit; the unit follows at once.
	match := func(p int) (num, end int) {
		num = digits(p)
		if num < len(s) && s[num] == '.' && isDigit(num+1) {
			num = digits(num + 1)
		} else if num == p {
			return -1, -1
		}
		if !isLower(num) {
			return -1, -1
		}
		end = num
		for isLower(end) {
			end++
		}
		return num, end
	}
	var out [][2]string
	for p := 0; p < len(s); {
		num, end := match(p)
		if num < 0 {
			p++
			continue
		}
		out = append(out, [2]string{s[p:num], s[num:end]})
		p = end
	}
	return out
}

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
	matches := durationParts(remaining)

	if len(matches) == 0 && remaining != "" {
		return 0, fmt.Errorf("invalid duration format in %q", originalString)
	}

	parsedStr := ""
	for _, match := range matches {
		numPart := match[0]
		unitPart := match[1]
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
