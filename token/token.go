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
	Pos     int
}

const (
	// Special types
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"

	// Identifiers + literals
	IDENT           = "IDENT" // add, foobar, x, y, ...
	STRING_LITERAL  = "STRING_LITERAL"
	WSTRING_LITERAL = "WSTRING_LITERAL"
	TIME            = "TIME"
	//T               = "TIME"
	//D               = "DATE"
	DATE          = "DATE"
	DATE_AND_TIME = "DATE_AND_TIME"
	//DT              = "DT"
	//TOD             = "TOD"
	TIME_OF_DAY = "TIME_OF_DAY"

	UNTERMINATED_STRING  = "UNTERMINATED_STRING"
	UNTERMINATED_COMMENT = "UNTERMINATED_COMMENT"
	COMMENT              = "COMMENT"

	// Operators
	ASSIGN    = ":="
	ATTEMPT   = "?=" // the assignment attempt, r ?= i
	QUESTION  = "?"  // The ? of IL ST?, the assignment attempt
	PLUS      = "+"
	MINUS     = "-"
	ASTERISK  = "*"
	SLASH     = "/"
	LT        = "<"
	LE        = "<="
	GT        = ">"
	GE        = ">="
	EQ        = "="
	NE        = "<>"
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
	CARET     = "^"
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

	// Instruction List (IL) Operators
	LD    = "LD"
	ST    = "ST"
	CAL   = "CAL"
	S     = "S"
	R     = "R"
	JMP   = "JMP"
	RET   = "RET"
	EN    = "EN"
	ENO   = "ENO"
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
	REFERENCE         = "REFERENCE"
	READ_ONLY         = "READ_ONLY"
	READ_WRITE        = "READ_WRITE"
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
	CLASS              = "CLASS"
	END_CLASS          = "END_CLASS"
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
	MOVE               = "MOVE"
	MIN                = "MIN"
	MAX                = "MAX"
	NIL                = "NIL"

	// OOP Keywords
	EXTENDS       = "EXTENDS"
	IMPLEMENTS    = "IMPLEMENTS"
	METHOD        = "METHOD"
	END_METHOD    = "END_METHOD"
	INTERFACE     = "INTERFACE"
	END_INTERFACE = "END_INTERFACE"
	THIS          = "THIS"
	SUPER         = "SUPER"
	PROPERTY      = "PROPERTY"
	END_PROPERTY  = "END_PROPERTY"
	GET           = "GET"
	SET           = "SET"
	END_GET       = "END_GET"
	END_SET       = "END_SET"
	ABSTRACT      = "ABSTRACT"
	NAMESPACE     = "NAMESPACE"
	END_NAMESPACE = "END_NAMESPACE"
	PUBLIC        = "PUBLIC"
	PRIVATE       = "PRIVATE"
	PROTECTED     = "PROTECTED"
	FINAL         = "FINAL"
	INTERNAL      = "INTERNAL"
	VOID          = "VOID"

	// Data Type Keywords
	BOOL    = "BOOL"
	SINT    = "SINT"
	INT     = "INT"
	DINT    = "DINT"
	LINT    = "LINT"
	USINT   = "USINT"
	UINT    = "UINT"
	UDINT   = "UDINT"
	ULINT   = "ULINT"
	REAL    = "REAL"
	LREAL   = "LREAL"
	WSTRING = "WSTRING"
	BYTE    = "BYTE"
	WORD    = "WORD"
	DWORD   = "DWORD"
	LWORD   = "LWORD"
	ARRAY   = "ARRAY"
	STRING  = "STRING"
	SR      = "SR"
	RS      = "RS"
)

// Task-specific keywords
const (
	TASK     = "TASK"
	SINGLE   = "SINGLE"
	INTERVAL = "INTERVAL"
	PRIORITY = "PRIORITY"
)

var keywords = map[string]TokenType{
	"FN":       FUNCTION,
	"TRUE":     TRUE,
	"FALSE":    FALSE,
	"MACRO":    MACRO,
	"ASSIGN":   ASSIGN,
	"PLUS":     PLUS,
	"MINUS":    MINUS,
	"ASTERISK": ASTERISK,
	"SLASH":    SLASH,
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
	"READ_ONLY":          READ_ONLY,
	"READ_WRITE":         READ_WRITE,
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
	"LD":                 LD,
	"ST":                 ST,
	"S":                  S,
	"CAL":                CAL,
	"JMP":                JMP,
	"RET":                RET,
	"R":                  R,
	"EN":                 EN,
	"ENO":                ENO,
	"MOD":                MOD,
	"NOR":                NOR,
	"RETURN":             RETURN,
	"FROM":               FROM,
	"END_PROGRAM":        END_PROGRAM,
	"FUNCTION":           FUNCTION,
	"END_FUNCTION":       END_FUNCTION,
	"FUNCTION_BLOCK":     FUNCTION_BLOCK,
	"FUNCTIONBLOCK":      FUNCTION_BLOCK, // CODESYS 2.3 also accepts this spelling.
	"END_FUNCTION_BLOCK": END_FUNCTION_BLOCK,
	"CLASS":              CLASS,
	"END_CLASS":          END_CLASS,
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
	"REFERENCE":          REFERENCE,
	// OOP VARIABLES
	"EXTENDS":       EXTENDS,
	"IMPLEMENTS":    IMPLEMENTS,
	"INTERFACE":     INTERFACE,
	"END_INTERFACE": END_INTERFACE,
	"METHOD":        METHOD,
	"THIS":          THIS,
	"SUPER":         SUPER,
	"ABSTRACT":      ABSTRACT,
	"PROPERTY":      PROPERTY,
	"END_PROPERTY":  END_PROPERTY,
	"GET":           GET,
	"SET":           SET,
	"END_GET":       END_GET,
	"END_SET":       END_SET,
	"END_METHOD":    END_METHOD,
	"NAMESPACE":     NAMESPACE,
	"END_NAMESPACE": END_NAMESPACE,
	"PUBLIC":        PUBLIC,
	"PRIVATE":       PRIVATE,
	"PROTECTED":     PROTECTED,
	"FINAL":         FINAL,
	"INTERNAL":      INTERNAL,
	"VOID":          VOID,
	//
	"ON":            ON,
	"WITH":          WITH,
	"TASK":          TASK,
	"MIN":           MIN,
	"MAX":           MAX,
	"MOVE":          MOVE,
	"NIL":           NIL,
	"SINGLE":        SINGLE,
	"INTERVAL":      INTERVAL,
	"PRIORITY":      PRIORITY,
	"BOOL":          BOOL,
	"INT":           INT,
	"SINT":          SINT,
	"DINT":          DINT,
	"LINT":          LINT,
	"USINT":         USINT,
	"UINT":          UINT,
	"UDINT":         UDINT,
	"ULINT":         ULINT, // Abbreviation for TIME
	"REAL":          REAL,
	"LREAL":         LREAL,
	"T":             TIME,
	"DT":            DATE_AND_TIME,
	"TIME":          TIME,
	"D":             DATE,
	"DATE":          DATE,
	"DATE_AND_TIME": DATE_AND_TIME,
	"TIME_OF_DAY":   TIME_OF_DAY,
	"TOD":           TIME_OF_DAY, // Alias for TIME_OF_DAY
	"WSTRING":       WSTRING,
	"BYTE":          BYTE,
	"WORD":          WORD,
	"DWORD":         DWORD,
	"LWORD":         LWORD,
	"ARRAY":         ARRAY,
	"STRING":        STRING,
	"SR":            SR,
	"RS":            RS,
	"RANGE":         RANGE,
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

// SameIdent reports whether two names are equal ignoring ASCII case, as
// IEC 61131-3 compares identifiers. Identifiers are ASCII (the lexer reads no
// other letters), so unlike strings.EqualFold it needs no Unicode case
// tables: using both EqualFold and ToUpper puts unicode.CaseRanges (6.7 KB)
// in RAM on TinyGo targets.
func SameIdent(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if x == y {
			continue
		}
		if 'a' <= x && x <= 'z' {
			x -= 'a' - 'A'
		}
		if 'a' <= y && y <= 'z' {
			y -= 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}

// longTimeTypes are the long time types of IEC 61131-3 3rd edition and the
// types beedance holds them as, which already have nanoseconds.
var longTimeTypes = map[string]string{
	"LTIME": "TIME", "LDATE": "DATE", "LTOD": "TOD", "LTIME_OF_DAY": "TIME_OF_DAY",
	"LDT": "DT", "LDATE_AND_TIME": "DATE_AND_TIME",
}

// LongTimeType returns the type a long time type's name stands for, LTIME
// for TIME, and whether ident is one. beforeHash: ident is followed by #,
// where LT and LD, the short prefixes of LTIME and LDATE literals, count.
func LongTimeType(ident string, beforeHash bool) (string, bool) {
	upper := strings.ToUpper(ident)
	if short, ok := longTimeTypes[upper]; ok {
		return short, true
	}
	if beforeHash {
		switch upper {
		case "LT":
			return "T", true
		case "LD":
			return "D", true
		}
	}
	return "", false
}
