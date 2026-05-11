/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */
package token

import "strings"

type TokenType string

type Token struct {
	Type    TokenType
	Literal string
	Row     int
	Column  int
}

const (
	// Special types
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"

	// Identifiers + literals
	IDENT           = "IDENT" // add, foobar, x, y, ...
	INT             = "INT"
	REAL            = "REAL"
	STRING_LITERAL  = "STRING_LITERAL"
	WSTRING_LITERAL = "WSTRING_LITERAL"
	TIME            = "TIME"

	UNTERMINATED_STRING  = "UNTERMINATED_STRING"
	UNTERMINATED_COMMENT = "UNTERMINATED_COMMENT"

	// Operators
	ASSIGN    = ":="
	PLUS      = "+"
	MINUS     = "-"
	ASTERISK  = "*"
	SLASH     = "/"
	LT        = "<"
	LE        = "<="
	GT        = ">"
	GE        = ">="
	EQ        = "="
	NEQ       = "<>"
	EXPONENT  = "**"
	ARROW     = "=>"
	AMPERSAND = "&"
	COLON     = ":"
	RANGE     = ".."

	// Delimiters
	COMMA     = ","
	SEMICOLON = ";"
	LPAREN    = "("
	RPAREN    = ")"
	LBRACE    = "{"
	RBRACE    = "}"
	DOT       = "."
	LBRACKET  = "["
	PERCENT   = "%"
	RBRACKET  = "]"
	HASH      = "#"

	// Keywords
	TRUE  = "TRUE"
	FALSE = "FALSE"
	NOT   = "NOT"
	AND   = "AND"
	OR    = "OR"
	XOR   = "XOR"
	NAND  = "NAND"
	NOR   = "NOR"
	MOD   = "MOD"
	MACRO = "MACRO"

	// New IEC 61131-3 Keywords
	VAR               = "VAR"
	END_VAR           = "END_VAR"
	VAR_INPUT         = "VAR_INPUT"
	VAR_OUTPUT        = "VAR_OUTPUT"
	VAR_IN_OUT        = "VAR_IN_OUT"
	VAR_EXTERNAL      = "VAR_EXTERNAL"
	VAR_GLOBAL        = "VAR_GLOBAL"
	VAR_ACCESS        = "VAR_ACCESS"
	VAR_TEMP          = "VAR_TEMP"
	VAR_CONFIG        = "VAR_CONFIG"
	DIRECT_VAR        = "VAR_DIRECT" // e.g. %IX1.0
	RETAIN            = "RETAIN"
	NON_RETAIN        = "NON_RETAIN"
	CONSTANT          = "CONSTANT"
	IF                = "IF"
	THEN              = "THEN"
	ELSE              = "ELSE"
	ELSIF             = "ELSIF"
	END_IF            = "END_IF"
	FOR               = "FOR"
	TO                = "TO"
	BY                = "BY"
	DO                = "DO"
	END_FOR           = "END_FOR"
	WHILE             = "WHILE"
	END_WHILE         = "END_WHILE"
	REPEAT            = "REPEAT"
	UNTIL             = "UNTIL"
	END_REPEAT        = "END_REPEAT"
	EXIT              = "EXIT"
	CASE              = "CASE"
	OF                = "OF"
	END_CASE          = "END_CASE"
	R_EDGE            = "R_EDGE"
	CAL               = "CAL"
	F_EDGE            = "F_EDGE"
	TYPE              = "TYPE"
	END_TYPE          = "END_TYPE"
	STRUCT            = "STRUCT"
	END_STRUCT        = "END_STRUCT"
	AT                = "AT"
	CONFIGURATION     = "CONFIGURATION"
	END_CONFIGURATION = "END_CONFIGURATION"
	RESOURCE          = "RESOURCE"
	END_RESOURCE      = "END_RESOURCE"
	ON                = "ON"
	WITH              = "WITH"

	// Program Organization Unit Keywords
	PROGRAM            = "PROGRAM"
	END_PROGRAM        = "END_PROGRAM"
	FROM               = "FROM"
	FUNCTION_BLOCK     = "FUNCTION_BLOCK"
	END_FUNCTION_BLOCK = "END_FUNCTION_BLOCK"
	FUNCTION           = "FUNCTION"
	END_FUNCTION       = "END_FUNCTION"
	ACTION             = "ACTION"
	END_ACTION         = "END_ACTION"
	TRANSITION         = "TRANSITION"
	END_TRANSITION     = "END_TRANSITION"
	STEP               = "STEP" // Also used in SFC
	END_STEP           = "END_STEP"
	INITIAL_STEP       = "INITIAL_STEP"
	RETURN             = "RETURN"

	// Data Type Keywords
	BOOL = "BOOL"
	SINT = "SINT"

	DINT          = "DINT"
	LINT          = "LINT"
	USINT         = "USINT"
	UINT          = "UINT"
	UDINT         = "UDINT"
	ULINT         = "ULINT"
	LREAL         = "LREAL"
	D             = "DATE"
	DATE          = "DATE"
	DATE_AND_TIME = "DATE_AND_TIME"
	DT            = "DT"
	TOD           = "TOD"
	TIME_OF_DAY   = "TIME_OF_DAY"
	WSTRING       = "WSTRING"
	BYTE          = "BYTE"
	WORD          = "WORD"
	DWORD         = "DWORD"
	LWORD         = "LWORD"
	ARRAY         = "ARRAY"
	STRING        = "STRING"
)

// Task-specific keywords
const (
	TASK     = "TASK"
	SINGLE   = "SINGLE"
	INTERVAL = "INTERVAL"
	PRIORITY = "PRIORITY"
)

var keywords = map[string]TokenType{
	"FN":    FUNCTION,
	"TRUE":  TRUE,
	"FALSE": FALSE,
	"MACRO": MACRO,
	// Variable declaration keywords
	"VAR":                VAR,
	"END_VAR":            END_VAR,
	"VAR_INPUT":          VAR_INPUT,
	"VAR_OUTPUT":         VAR_OUTPUT,
	"VAR_IN_OUT":         VAR_IN_OUT,
	"VAR_EXTERNAL":       VAR_EXTERNAL,
	"VAR_GLOBAL":         VAR_GLOBAL,
	"VAR_ACCESS":         VAR_ACCESS,
	"VAR_TEMP":           VAR_TEMP,
	"VAR_CONFIG":         VAR_CONFIG,
	"DIRECT_VAR":         DIRECT_VAR,
	"RETAIN":             RETAIN,
	"NON_RETAIN":         NON_RETAIN,
	"CONSTANT":           CONSTANT,
	"IF":                 IF,
	"THEN":               THEN,
	"ELSE":               ELSE,
	"ELSIF":              ELSIF,
	"END_IF":             END_IF,
	"FOR":                FOR,
	"TO":                 TO,
	"BY":                 BY,
	"DO":                 DO,
	"END_FOR":            END_FOR,
	"WHILE":              WHILE,
	"END_WHILE":          END_WHILE,
	"REPEAT":             REPEAT,
	"UNTIL":              UNTIL,
	"END_REPEAT":         END_REPEAT,
	"EXIT":               EXIT,
	"R_EDGE":             R_EDGE,
	"CAL":                CAL,
	"F_EDGE":             F_EDGE,
	"OF":                 OF,
	"TYPE":               TYPE,
	"END_TYPE":           END_TYPE,
	"STRUCT":             STRUCT,
	"END_STRUCT":         END_STRUCT,
	"CASE":               CASE,
	"END_CASE":           END_CASE,
	"PROGRAM":            PROGRAM,
	"NOT":                NOT,
	"AND":                AND,
	"OR":                 OR,
	"XOR":                XOR,
	"NAND":               NAND,
	"MOD":                MOD,
	"NOR":                NOR,
	"RETURN":             RETURN,
	"FROM":               FROM,
	"END_PROGRAM":        END_PROGRAM,
	"FUNCTION":           FUNCTION,
	"END_FUNCTION":       END_FUNCTION,
	"FUNCTION_BLOCK":     FUNCTION_BLOCK,
	"END_FUNCTION_BLOCK": END_FUNCTION_BLOCK,
	"ACTION":             ACTION,
	"END_ACTION":         END_ACTION,
	"TRANSITION":         TRANSITION,
	"END_TRANSITION":     END_TRANSITION,
	"INITIAL_STEP":       INITIAL_STEP,
	"STEP":               STEP,
	"END_STEP":           END_STEP,
	"AT":                 AT,
	"CONFIGURATION":      CONFIGURATION,
	"END_CONFIGURATION":  END_CONFIGURATION,
	"RESOURCE":           RESOURCE,
	"END_RESOURCE":       END_RESOURCE,
	"ON":                 ON,
	"WITH":               WITH,
	"TASK":               TASK,
	"SINGLE":             SINGLE,
	"INTERVAL":           INTERVAL,
	"PRIORITY":           PRIORITY,
	"BOOL":               BOOL,
	"INT":                INT,
	"SINT":               SINT,
	"DINT":               DINT,
	"LINT":               LINT,
	"USINT":              USINT,
	"UINT":               UINT,
	"UDINT":              UDINT,
	"ULINT":              ULINT, // Abbreviation for TIME
	"REAL":               REAL,
	"LREAL":              LREAL,
	"TIME":               TIME,
	"DATE":               DATE,
	"DATE_AND_TIME":      DATE_AND_TIME,
	"TIME_OF_DAY":        TIME_OF_DAY,
	"TOD":                TIME_OF_DAY, // Alias for TIME_OF_DAY
	"WSTRING":            WSTRING,
	"BYTE":               BYTE,
	"WORD":               WORD,
	"DWORD":              DWORD,
	"LWORD":              LWORD,
	"ARRAY":              ARRAY,
	"STRING":             STRING,
	"RANGE":              RANGE,
}

// LookupIdent checks the `keywords` table to see whether the given identifier
// is in fact a keyword.
func LookupIdent(ident string) TokenType {
	upperIdent := strings.ToUpper(ident)
	if tok, ok := keywords[upperIdent]; ok {
		return tok
	}
	return IDENT // Return the canonical (uppercase) version
}
