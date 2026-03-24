package parser

import (
	"fmt"
	"strconv"
	"unicode"
)

// Token represents a lexical token.
type Token struct {
	Value string
	Type  TokenType
	File  string
	Line  int
	Col   int
}

func (t Token) String() string {
	if t.File != "" {
		return fmt.Sprintf("%s(%q)@%s:%d:%d", t.Type, t.Value, t.File, t.Line, t.Col)
	}
	return fmt.Sprintf("%s(%q)@%d:%d", t.Type, t.Value, t.Line, t.Col)
}

type TokenType int

const (
	// Literals
	TokenIdent TokenType = iota
	TokenInt
	TokenFloat
	TokenString
	TokenBool

	// Keywords
	TokenUse
	TokenSpec
	TokenModel
	TokenContract
	TokenInput
	TokenOutput
	TokenAction
	TokenInvariant
	TokenScenario
	TokenGiven
	TokenWhen
	TokenThen
	TokenTarget
	TokenLocators
	TokenPlugin
	TokenNull
	TokenScope
	TokenConfig
	TokenEnv
	TokenInclude
	TokenImport

	// Symbols
	TokenLBrace   // {
	TokenRBrace   // }
	TokenLParen   // (
	TokenRParen   // )
	TokenLBracket // [
	TokenRBracket // ]
	TokenColon    // :
	TokenComma    // ,
	TokenDot      // .
	TokenAt       // @
	TokenQuestion // ?

	// Operators
	TokenEq     // ==
	TokenNeq    // !=
	TokenGt     // >
	TokenLt     // <
	TokenGte    // >=
	TokenLte    // <=
	TokenPlus   // +
	TokenMinus  // -
	TokenStar    // *
	TokenSlash   // /
	TokenPercent // %
	TokenAnd     // &&
	TokenOr     // ||
	TokenNot    // !
	TokenAssign // =

	// Special
	TokenComment
	TokenEOF
)

var tokenNames = map[TokenType]string{
	TokenIdent:     "Ident",
	TokenInt:       "Int",
	TokenFloat:     "Float",
	TokenString:    "String",
	TokenBool:      "Bool",
	TokenUse:       "Use",
	TokenSpec:      "Spec",
	TokenModel:     "Model",
	TokenContract:  "Contract",
	TokenInput:     "Input",
	TokenOutput:    "Output",
	TokenAction:    "Action",
	TokenInvariant: "Invariant",
	TokenScenario:  "Scenario",
	TokenGiven:     "Given",
	TokenWhen:      "When",
	TokenThen:      "Then",
	TokenTarget:    "Target",
	TokenLocators:  "Locators",
	TokenPlugin:    "Plugin",
	TokenNull:      "Null",
	TokenScope:     "Scope",
	TokenConfig:    "Config",
	TokenEnv:       "Env",
	TokenInclude:   "Include",
	TokenImport:    "Import",
	TokenLBrace:    "LBrace",
	TokenRBrace:    "RBrace",
	TokenLParen:    "LParen",
	TokenRParen:    "RParen",
	TokenLBracket:  "LBracket",
	TokenRBracket:  "RBracket",
	TokenColon:     "Colon",
	TokenComma:     "Comma",
	TokenDot:       "Dot",
	TokenAt:        "At",
	TokenQuestion:  "Question",
	TokenEq:        "Eq",
	TokenNeq:       "Neq",
	TokenGt:        "Gt",
	TokenLt:        "Lt",
	TokenGte:       "Gte",
	TokenLte:       "Lte",
	TokenPlus:      "Plus",
	TokenMinus:     "Minus",
	TokenStar:      "Star",
	TokenSlash:     "Slash",
	TokenPercent:   "Percent",
	TokenAnd:       "And",
	TokenOr:        "Or",
	TokenNot:       "Not",
	TokenAssign:    "Assign",
	TokenComment:   "Comment",
	TokenEOF:       "EOF",
}

func (t TokenType) String() string {
	if name, ok := tokenNames[t]; ok {
		return name
	}
	return fmt.Sprintf("TokenType(%d)", int(t))
}

var keywords = map[string]TokenType{
	"use":       TokenUse,
	"spec":      TokenSpec,
	"model":     TokenModel,
	"contract":  TokenContract,
	"input":     TokenInput,
	"output":    TokenOutput,
	"action":    TokenAction,
	"invariant": TokenInvariant,
	"scenario":  TokenScenario,
	"given":     TokenGiven,
	"when":      TokenWhen,
	"then":      TokenThen,
	"target":    TokenTarget,
	"locators":  TokenLocators,
	"plugin":    TokenPlugin,
	"null":      TokenNull,
	"scope":     TokenScope,
	"config":    TokenConfig,
	"env":       TokenEnv,
	"include":   TokenInclude,
	"import":    TokenImport,
	"true":      TokenBool,
	"false":     TokenBool,
}

// singleCharTokens maps single characters to their token types.
var singleCharTokens = map[rune]TokenType{
	'{': TokenLBrace,
	'}': TokenRBrace,
	'(': TokenLParen,
	')': TokenRParen,
	'[': TokenLBracket,
	']': TokenRBracket,
	':': TokenColon,
	',': TokenComma,
	'.': TokenDot,
	'@': TokenAt,
	'?': TokenQuestion,
	'+': TokenPlus,
	'-': TokenMinus,
	'*': TokenStar,
	'/': TokenSlash,
	'%': TokenPercent,
}

type lexer struct {
	src    []rune
	tokens []Token
	pos    int
	line   int
	col    int
}

// Lex tokenizes spec source text.
func Lex(source string) ([]Token, error) {
	l := &lexer{
		src:  []rune(source),
		line: 1,
		col:  1,
	}
	if err := l.lex(); err != nil {
		return nil, err
	}
	return l.tokens, nil
}

func (l *lexer) lex() error {
	for l.pos < len(l.src) {
		if err := l.lexOne(); err != nil {
			return err
		}
	}
	l.tokens = append(l.tokens, Token{Type: TokenEOF, Line: l.line, Col: l.col})
	return nil
}

func (l *lexer) lexOne() error {
	ch := l.src[l.pos]

	switch {
	case ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n':
		l.advance()
	case ch == '#':
		l.skipComment()
	case ch == '"':
		return l.lexString()
	case unicode.IsDigit(ch):
		l.lexNumber()
	case unicode.IsLetter(ch) || ch == '_':
		l.lexIdent()
	default:
		return l.lexSymbol()
	}
	return nil
}

func (l *lexer) advance() {
	if l.pos < len(l.src) {
		if l.src[l.pos] == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
		l.pos++
	}
}

func (l *lexer) emit(typ TokenType, value string, line, col int) {
	l.tokens = append(l.tokens, Token{Type: typ, Value: value, Line: line, Col: col})
}

func (l *lexer) skipComment() {
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		l.advance()
	}
}

func (l *lexer) lexString() error {
	startLine, startCol := l.line, l.col
	l.advance() // consume opening "

	s, err := l.lexStringBody(startLine, startCol)
	if err != nil {
		return err
	}

	if l.pos >= len(l.src) {
		return fmt.Errorf("%d:%d: unterminated string literal", startLine, startCol)
	}
	l.advance() // consume closing "
	l.emit(TokenString, string(s), startLine, startCol)
	return nil
}

func (l *lexer) lexStringBody(startLine, startCol int) ([]rune, error) {
	var s []rune
	for l.pos < len(l.src) && l.src[l.pos] != '"' {
		if l.src[l.pos] != '\\' {
			s = append(s, l.src[l.pos])
			l.advance()
			continue
		}
		l.advance() // consume backslash
		if l.pos >= len(l.src) {
			return nil, fmt.Errorf("%d:%d: unterminated string escape", startLine, startCol)
		}
		s = append(s, escapeChar(l.src[l.pos])...)
		l.advance()
	}
	return s, nil
}

func escapeChar(ch rune) []rune {
	switch ch {
	case '"', '\\':
		return []rune{ch}
	case 'n':
		return []rune{'\n'}
	case 't':
		return []rune{'\t'}
	default:
		return []rune{'\\', ch}
	}
}

func (l *lexer) lexNumber() {
	startLine, startCol := l.line, l.col
	start := l.pos
	for l.pos < len(l.src) && unicode.IsDigit(l.src[l.pos]) {
		l.advance()
	}
	// Check for decimal point followed by digit → float literal
	if l.pos < len(l.src)-1 && l.src[l.pos] == '.' && unicode.IsDigit(l.src[l.pos+1]) {
		l.advance() // consume '.'
		for l.pos < len(l.src) && unicode.IsDigit(l.src[l.pos]) {
			l.advance()
		}
		l.emit(TokenFloat, string(l.src[start:l.pos]), startLine, startCol)
		return
	}
	l.emit(TokenInt, string(l.src[start:l.pos]), startLine, startCol)
}

func (l *lexer) lexIdent() {
	startLine, startCol := l.line, l.col
	start := l.pos
	for l.pos < len(l.src) && (unicode.IsLetter(l.src[l.pos]) || unicode.IsDigit(l.src[l.pos]) || l.src[l.pos] == '_') {
		l.advance()
	}
	word := string(l.src[start:l.pos])
	if typ, ok := keywords[word]; ok {
		l.emit(typ, word, startLine, startCol)
		return
	}
	l.emit(TokenIdent, word, startLine, startCol)
}

func (l *lexer) lexSymbol() error {
	line, col := l.line, l.col
	ch := l.src[l.pos]

	// Single-character tokens.
	if typ, ok := singleCharTokens[ch]; ok {
		l.emit(typ, string(ch), line, col)
		l.advance()
		return nil
	}

	// Two-character and compound operators.
	return l.lexOperator(line, col, ch)
}

func (l *lexer) lexOperator(line, col int, ch rune) error {
	l.advance() // consume first character

	switch ch {
	case '=':
		l.emitCompound(line, col, '=', TokenEq, "==", TokenAssign, "=")
	case '!':
		l.emitCompound(line, col, '=', TokenNeq, "!=", TokenNot, "!")
	case '>':
		l.emitCompound(line, col, '=', TokenGte, ">=", TokenGt, ">")
	case '<':
		l.emitCompound(line, col, '=', TokenLte, "<=", TokenLt, "<")
	case '&':
		return l.expectDouble(line, col, '&', TokenAnd, "&&")
	case '|':
		return l.expectDouble(line, col, '|', TokenOr, "||")
	default:
		return fmt.Errorf("%d:%d: unexpected character %s", line, col, strconv.QuoteRune(ch))
	}
	return nil
}

// emitCompound handles operators that may be one or two characters (e.g., = vs ==).
func (l *lexer) emitCompound(
	line, col int,
	next rune,
	doubleTyp TokenType,
	doubleVal string,
	singleTyp TokenType,
	singleVal string,
) {
	if l.pos < len(l.src) && l.src[l.pos] == next {
		l.emit(doubleTyp, doubleVal, line, col)
		l.advance()
		return
	}
	l.emit(singleTyp, singleVal, line, col)
}

// expectDouble handles operators that require two identical characters (e.g., && ||).
func (l *lexer) expectDouble(line, col int, ch rune, typ TokenType, val string) error {
	if l.pos >= len(l.src) || l.src[l.pos] != ch {
		return fmt.Errorf("%d:%d: unexpected character '%c' (expected '%s')", line, col, ch, val)
	}
	l.emit(typ, val, line, col)
	l.advance()
	return nil
}
