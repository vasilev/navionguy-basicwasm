package token

import "strings"

type TokenType string

const (
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"
	EOL     = "EOL"

	// Identifiers + literals
	BSTR    = "BSTR"  //string of bytes
	IDENT   = "IDENT" // add, foobar, x, y, ...
	LINENUM = "####"  // 10, 15, 20, ...
	INT     = "INT"   // -32768 to 32767
	INTD    = "INTD"
	STRING  = "STRING" // "A string literal"
	FLOAT   = "FLOAT"  // 2.539999E+01
	FIXED   = "FIXED"  // 42.14

	// Operators
	ASSIGN   = "="
	PLUS     = "+"
	MINUS    = "-"
	BANG     = "!"
	ASTERISK = "*"
	SLASH    = "/"
	BSLASH   = "\\"

	LT = "<"
	GT = ">"

	EQ     = "="
	NOT_EQ = "<>"
	GTE    = ">="
	LTE    = "<="

	// Type designators
	TYPE_STR = "$"
	TYPE_INT = "%"
	TYPE_SGL = "!"
	TYPE_DBL = "#"

	// Delimiters
	PERIOD    = "."
	COMMA     = ","
	SEMICOLON = ";"
	COLON     = ":"
	AMPERSAND = "&"

	LPAREN   = "("
	RPAREN   = ")"
	LBRACE   = "{"
	RBRACE   = "}"
	LBRACKET = "["
	RBRACKET = "]"

	HEX   = "&H"
	OCTAL = "&O"

	// Keywords
	ALL     = "ALL"
	AUTO    = "AUTO"
	BEEP    = "BEEP"
	CHAIN   = "CHAIN"
	CLEAR   = "CLEAR"
	CLS     = "CLS"
	COLOR   = "COLOR"
	COMMON  = "COMMON"
	CONT    = "CONT"
	CSRLIN  = "CSRLIN"
	DATA    = "DATA"
	DEF     = "DEF"
	DIM     = "DIM"
	ELSE    = "ELSE"
	END     = "END"
	FALSE   = "FALSE"
	FILES   = "FILES"
	GOSUB   = "GOSUB"
	GOTO    = "GOTO"
	IF      = "IF"
	LET     = "LET"
	LIST    = "LIST"
	LOAD    = "LOAD"
	LOCATE  = "LOCATE"
	MERGE   = "MERGE"
	MOD     = "MOD"
	NEW     = "NEW"
	PALETTE = "PALETTE"
	PRINT   = "PRINT"
	READ    = "READ"
	REM     = "REM"
	RESTORE = "RESTORE"
	RETURN  = "RETURN"
	RUN     = "RUN"
	SCREEN  = "SCREEN"
	STOP    = "STOP"
	THEN    = "THEN"
	TRON    = "TRON"
	TROFF   = "TROFF"
	TRUE    = "TRUE"
	USING   = "USING"
)

type Token struct {
	Type    TokenType
	Literal string
}

var keywords = map[string]TokenType{
	"all":     ALL,
	"auto":    AUTO,
	"beep":    BEEP,
	"chain":   CHAIN,
	"clear":   CLEAR,
	"cls":     CLS,
	"color":   COLOR,
	"common":  COMMON,
	"cont":    CONT,
	"csrlin":  CSRLIN,
	"data":    DATA,
	"def":     DEF,
	"dim":     DIM,
	"else":    ELSE,
	"end":     END,
	"false":   FALSE,
	"files":   FILES,
	"gosub":   GOSUB,
	"goto":    GOTO,
	"if":      IF,
	"let":     LET,
	"list":    LIST,
	"load":    LOAD,
	"locate":  LOCATE,
	"merge":   MERGE,
	"mod":     MOD,
	"new":     NEW,
	"palette": PALETTE,
	"print":   PRINT,
	"read":    READ,
	"rem":     REM,
	"restore": RESTORE,
	"return":  RETURN,
	"run":     RUN,
	"screen":  SCREEN,
	"stop":    STOP,
	"then":    THEN,
	"tron":    TRON,
	"troff":   TROFF,
	"true":    TRUE,
	"using":   USING,
}

// LookupIdent returns a TokenType object
func LookupIdent(ident string) TokenType {

	if tok, ok := keywords[strings.ToLower(ident)]; ok {
		return tok
	}
	return IDENT
}
