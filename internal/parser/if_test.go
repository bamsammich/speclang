package parser_test

import (
	"testing"

	"github.com/bamsammich/speclang/v2/internal/parser"
)

func TestParseIfExpr_Simple(t *testing.T) {
	t.Parallel()

	spec, err := parser.Parse(`spec T {
		scope s {
			invariant i {
				if x > 0 then x else 0
			}
		}
	}`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	inv := spec.Scopes[0].Invariants[0]
	if len(inv.Assertions) != 1 {
		t.Fatalf("expected 1 assertion, got %d", len(inv.Assertions))
	}

	ifExpr, ok := inv.Assertions[0].Expr.(parser.IfExpr)
	if !ok {
		t.Fatalf("expected IfExpr, got %T", inv.Assertions[0].Expr)
	}

	// Condition: x > 0
	binOp, ok := ifExpr.Condition.(parser.BinaryOp)
	if !ok {
		t.Fatalf("expected BinaryOp condition, got %T", ifExpr.Condition)
	}
	if binOp.Op != ">" {
		t.Errorf("expected op >, got %q", binOp.Op)
	}

	// Then branch: x (FieldRef)
	thenRef, ok := ifExpr.Then.(parser.FieldRef)
	if !ok {
		t.Fatalf("expected FieldRef in then branch, got %T", ifExpr.Then)
	}
	if thenRef.Path != "x" {
		t.Errorf("expected then=x, got %q", thenRef.Path)
	}

	// Else branch: 0
	elseLit, ok := ifExpr.Else.(parser.LiteralInt)
	if !ok {
		t.Fatalf("expected LiteralInt in else branch, got %T", ifExpr.Else)
	}
	if elseLit.Value != 0 {
		t.Errorf("expected else=0, got %d", elseLit.Value)
	}
}

func TestParseIfExpr_Nested(t *testing.T) {
	t.Parallel()

	spec, err := parser.Parse(`spec T {
		scope s {
			invariant i {
				if a then (if b then x else y) else z
			}
		}
	}`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	inv := spec.Scopes[0].Invariants[0]
	ifExpr, ok := inv.Assertions[0].Expr.(parser.IfExpr)
	if !ok {
		t.Fatalf("expected IfExpr, got %T", inv.Assertions[0].Expr)
	}

	// Condition: a
	condRef, ok := ifExpr.Condition.(parser.FieldRef)
	if !ok {
		t.Fatalf("expected FieldRef condition, got %T", ifExpr.Condition)
	}
	if condRef.Path != "a" {
		t.Errorf("expected condition=a, got %q", condRef.Path)
	}

	// Then branch: nested if
	innerIf, ok := ifExpr.Then.(parser.IfExpr)
	if !ok {
		t.Fatalf("expected nested IfExpr in then branch, got %T", ifExpr.Then)
	}
	innerCond, ok := innerIf.Condition.(parser.FieldRef)
	if !ok || innerCond.Path != "b" {
		t.Errorf("expected inner condition=b, got %v", innerIf.Condition)
	}
	innerThen, ok := innerIf.Then.(parser.FieldRef)
	if !ok || innerThen.Path != "x" {
		t.Errorf("expected inner then=x, got %v", innerIf.Then)
	}
	innerElse, ok := innerIf.Else.(parser.FieldRef)
	if !ok || innerElse.Path != "y" {
		t.Errorf("expected inner else=y, got %v", innerIf.Else)
	}

	// Else branch: z
	elseRef, ok := ifExpr.Else.(parser.FieldRef)
	if !ok || elseRef.Path != "z" {
		t.Errorf("expected else=z, got %v", ifExpr.Else)
	}
}

func TestParseIfExpr_WithOperators(t *testing.T) {
	t.Parallel()

	// if/then/else with boolean operators in condition and arithmetic in branches
	spec, err := parser.Parse(`spec T {
		scope s {
			invariant i {
				if error == null then output.balance - input.amount else input.balance
			}
		}
	}`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	inv := spec.Scopes[0].Invariants[0]
	ifExpr, ok := inv.Assertions[0].Expr.(parser.IfExpr)
	if !ok {
		t.Fatalf("expected IfExpr, got %T", inv.Assertions[0].Expr)
	}

	// Condition: error == null
	binOp, ok := ifExpr.Condition.(parser.BinaryOp)
	if !ok {
		t.Fatalf("expected BinaryOp condition, got %T", ifExpr.Condition)
	}
	if binOp.Op != "==" {
		t.Errorf("expected op ==, got %q", binOp.Op)
	}

	// Then branch: output.balance - input.amount (BinaryOp)
	thenOp, ok := ifExpr.Then.(parser.BinaryOp)
	if !ok {
		t.Fatalf("expected BinaryOp in then branch, got %T", ifExpr.Then)
	}
	if thenOp.Op != "-" {
		t.Errorf("expected then op -, got %q", thenOp.Op)
	}
}

func TestParseIfExpr_MissingElse(t *testing.T) {
	t.Parallel()

	_, err := parser.Parse(`spec T {
		scope s {
			invariant i {
				if x > 0 then x
			}
		}
	}`)
	if err == nil {
		t.Fatal("expected parse error for if without else, got nil")
	}
}

func TestParseIfExpr_MissingThen(t *testing.T) {
	t.Parallel()

	_, err := parser.Parse(`spec T {
		scope s {
			invariant i {
				if x > 0 x else 0
			}
		}
	}`)
	if err == nil {
		t.Fatal("expected parse error for if without then keyword, got nil")
	}
}

func TestLexIfElseKeywords(t *testing.T) {
	t.Parallel()

	tokens, err := parser.Lex("if then else")
	if err != nil {
		t.Fatalf("lex failed: %v", err)
	}

	// Expect: If, Then, Else, EOF
	expected := []parser.TokenType{
		parser.TokenIf,
		parser.TokenThen,
		parser.TokenElse,
		parser.TokenEOF,
	}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d", len(expected), len(tokens))
	}
	for i, want := range expected {
		if tokens[i].Type != want {
			t.Errorf("token %d: expected %s, got %s", i, want, tokens[i].Type)
		}
	}
}
