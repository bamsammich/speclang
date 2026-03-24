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

	// "spec AccountAPI {" should be first tokens
	assertToken(t, tokens, 0, TokenSpec, "spec")
	assertToken(t, tokens, 1, TokenIdent, "AccountAPI")
	assertToken(t, tokens, 2, TokenLBrace, "{")

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

	// Check include directives with string paths
	inclIdx := findToken(tokens, TokenInclude)
	if inclIdx < 0 {
		t.Fatal("could not find include keyword")
	}
	assertToken(t, tokens, inclIdx+1, TokenString, "models/account.spec")

	// Find second include
	inclIdx2 := findTokenValue(tokens, TokenString, "scopes/transfer.spec")
	if inclIdx2 < 0 {
		t.Fatal("could not find include path 'scopes/transfer.spec'")
	}

	// model and scope tokens should NOT appear in root file (they are in included files)
	if findToken(tokens, TokenModel) >= 0 {
		t.Fatal("root file should not contain model token (moved to included file)")
	}
	if findToken(tokens, TokenScope) >= 0 {
		t.Fatal("root file should not contain scope token (moved to included file)")
	}

	// Verify line tracking: "spec" should be line 1
	if tokens[0].Line != 1 {
		t.Errorf("expected 'spec' on line 1, got line %d", tokens[0].Line)
	}
}

func TestLexOperators(t *testing.T) {
	tokens, err := Lex("== != > < >= <= + - * / % && ||")
	if err != nil {
		t.Fatal(err)
	}
	expected := []TokenType{
		TokenEq, TokenNeq, TokenGt, TokenLt, TokenGte, TokenLte,
		TokenPlus, TokenMinus, TokenStar, TokenSlash, TokenPercent, TokenAnd, TokenOr, TokenEOF,
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

func TestLexInclude(t *testing.T) {
	tokens, err := Lex(`include "models.spec"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 3 { // include, string, EOF
		t.Fatalf("expected 3 tokens, got %d", len(tokens))
	}
	assertToken(t, tokens, 0, TokenInclude, "include")
	assertToken(t, tokens, 1, TokenString, "models.spec")
}

func TestTokenFileField(t *testing.T) {
	tok := Token{Type: TokenIdent, Value: "foo", File: "test.spec", Line: 1, Col: 1}
	if tok.File != "test.spec" {
		t.Fatalf("expected File=test.spec, got %q", tok.File)
	}
	expected := `Ident("foo")@test.spec:1:1`
	if tok.String() != expected {
		t.Fatalf("expected %q, got %q", expected, tok.String())
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
