package parser

import (
	"os"
	"testing"
)

func TestLexTransferSpec(t *testing.T) {
	data, err := os.ReadFile("../../examples/transfer.spec")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	tokens, err := Lex(string(data))
	if err != nil {
		t.Fatalf("lex error: %v", err)
	}

	// Verify we got tokens and the last one is EOF
	if len(tokens) == 0 {
		t.Fatal("expected tokens, got none")
	}
	if tokens[len(tokens)-1].Type != TokenEOF {
		t.Fatalf("expected EOF as last token, got %s", tokens[len(tokens)-1].Type)
	}

	// Spot-check key token sequences
	types := make([]TokenType, len(tokens))
	for i, tok := range tokens {
		types[i] = tok.Type
	}

	// "use http" should be first two tokens
	assertToken(t, tokens, 0, TokenUse, "use")
	assertToken(t, tokens, 1, TokenIdent, "http")

	// "spec Transfer {" next
	assertToken(t, tokens, 2, TokenSpec, "spec")
	assertToken(t, tokens, 3, TokenIdent, "AccountAPI")
	assertToken(t, tokens, 4, TokenLBrace, "{")

	// Find "model Account {" sequence
	idx := findTokenSeq(tokens, TokenModel, TokenIdent)
	if idx < 0 {
		t.Fatal("could not find 'model Account'")
	}
	assertToken(t, tokens, idx+1, TokenIdent, "Account")

	// Check that "env" is lexed as a keyword
	envIdx := findToken(tokens, TokenEnv)
	if envIdx < 0 {
		t.Fatal("could not find env keyword")
	}

	// Check string literal "http://localhost:8080"
	strIdx := findTokenValue(tokens, TokenString, "http://localhost:8080")
	if strIdx < 0 {
		t.Fatal("could not find string literal 'http://localhost:8080'")
	}

	// Check that "null" is lexed as TokenNull
	nullIdx := findToken(tokens, TokenNull)
	if nullIdx < 0 {
		t.Fatal("could not find null keyword")
	}

	// Check operators: ==, !=, >, <=, >=, +
	for _, op := range []TokenType{TokenEq, TokenNeq, TokenGt, TokenLte, TokenPlus} {
		if findToken(tokens, op) < 0 {
			t.Errorf("expected to find operator %s", op)
		}
	}

	// Check ? token (from string?)
	if findToken(tokens, TokenQuestion) < 0 {
		t.Fatal("expected to find ? token")
	}

	// Check int literal 100
	intIdx := findTokenValue(tokens, TokenInt, "100")
	if intIdx < 0 {
		t.Fatal("could not find int literal 100")
	}

	// Verify line tracking: "use" should be line 1
	if tokens[0].Line != 1 {
		t.Errorf("expected 'use' on line 1, got line %d", tokens[0].Line)
	}
	// "spec" should be line 3
	if tokens[2].Line != 3 {
		t.Errorf("expected 'spec' on line 3, got line %d", tokens[2].Line)
	}
}

func TestLexOperators(t *testing.T) {
	tokens, err := Lex("== != > < >= <= + - * && ||")
	if err != nil {
		t.Fatal(err)
	}
	expected := []TokenType{
		TokenEq, TokenNeq, TokenGt, TokenLt, TokenGte, TokenLte,
		TokenPlus, TokenMinus, TokenStar, TokenAnd, TokenOr, TokenEOF,
	}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d", len(expected), len(tokens))
	}
	for i, exp := range expected {
		if tokens[i].Type != exp {
			t.Errorf("token %d: expected %s, got %s", i, exp, tokens[i].Type)
		}
	}
}

func TestLexStringEscape(t *testing.T) {
	tokens, err := Lex(`"hello \"world\""`)
	if err != nil {
		t.Fatal(err)
	}
	assertToken(t, tokens, 0, TokenString, `hello "world"`)
}

func TestLexUnterminatedString(t *testing.T) {
	_, err := Lex(`"unterminated`)
	if err == nil {
		t.Fatal("expected error for unterminated string")
	}
}

func TestLexComments(t *testing.T) {
	tokens, err := Lex("foo # this is a comment\nbar")
	if err != nil {
		t.Fatal(err)
	}
	assertToken(t, tokens, 0, TokenIdent, "foo")
	assertToken(t, tokens, 1, TokenIdent, "bar")
	if len(tokens) != 3 { // foo, bar, EOF
		t.Errorf("expected 3 tokens, got %d", len(tokens))
	}
}

// Helpers

func assertToken(t *testing.T, tokens []Token, idx int, typ TokenType, value string) {
	t.Helper()
	if idx >= len(tokens) {
		t.Fatalf("token index %d out of range (len=%d)", idx, len(tokens))
	}
	tok := tokens[idx]
	if tok.Type != typ {
		t.Errorf("token[%d]: expected type %s, got %s (value=%q)", idx, typ, tok.Type, tok.Value)
	}
	if tok.Value != value {
		t.Errorf("token[%d]: expected value %q, got %q", idx, value, tok.Value)
	}
}

func findToken(tokens []Token, typ TokenType) int {
	for i, tok := range tokens {
		if tok.Type == typ {
			return i
		}
	}
	return -1
}

func findTokenValue(tokens []Token, typ TokenType, value string) int {
	for i, tok := range tokens {
		if tok.Type == typ && tok.Value == value {
			return i
		}
	}
	return -1
}

func findTokenSeq(tokens []Token, types ...TokenType) int {
	for i := 0; i <= len(tokens)-len(types); i++ {
		match := true
		for j, typ := range types {
			if tokens[i+j].Type != typ {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
