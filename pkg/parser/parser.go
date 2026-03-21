package parser

import (
	"fmt"
	"path/filepath"
	"strconv"
)

// ParseFile reads a spec file, resolves includes, and returns the AST.
func ParseFile(path string) (*Spec, error) {
	return ParseFileWithImports(path, nil)
}

// ParseFileWithImports reads a spec file, resolves includes, and returns the AST.
// The imports registry maps adapter names to import resolvers for the import directive.
func ParseFileWithImports(path string, imports ImportRegistry) (*Spec, error) {
	absRoot, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	tokens, err := lexFile(absRoot)
	if err != nil {
		return nil, err
	}

	resolved, err := resolveIncludes(tokens, filepath.Dir(absRoot), absRoot, nil)
	if err != nil {
		return nil, err
	}

	p := &parser{
		tokens:  resolved,
		imports: imports,
		fileDir: filepath.Dir(absRoot),
	}
	spec, err := p.parse()
	if err != nil {
		return nil, err
	}

	if err := validateNoDuplicates(spec); err != nil {
		return nil, err
	}

	return spec, nil
}

// Parse parses spec source text into an AST.
func Parse(source string) (*Spec, error) {
	tokens, err := Lex(source)
	if err != nil {
		return nil, fmt.Errorf("lexing: %w", err)
	}
	p := &parser{tokens: tokens}
	return p.parse()
}

type parser struct {
	tokens  []Token
	imports ImportRegistry
	fileDir string // directory of the spec file, for relative path resolution
	pos     int
}

// peek returns the current token without consuming it.
func (p *parser) peek() Token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return Token{Type: TokenEOF}
}

// advance consumes and returns the current token.
func (p *parser) advance() Token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

// expect consumes a token of the given type, or returns an error.
func (p *parser) expect(typ TokenType) (Token, error) {
	tok := p.advance()
	if tok.Type != typ {
		return tok, fmt.Errorf("%d:%d: expected %s, got %s (%q)",
			tok.Line, tok.Col, typ, tok.Type, tok.Value)
	}
	return tok, nil
}

// errAt returns a formatted error at the given token's position.
func (*parser) errAt(tok Token, msg string) error {
	return fmt.Errorf("%d:%d: %s", tok.Line, tok.Col, msg)
}

// isIdentLike returns true if the token can be used as an identifier in
// expression context. Keywords like "input", "output", "error" commonly
// appear as field names in expressions.
func isIdentLike(typ TokenType) bool {
	switch typ {
	case TokenIdent,
		TokenInput, TokenOutput,
		TokenModel, TokenAction,
		TokenTarget, TokenLocators,
		TokenGiven, TokenThen,
		TokenScope, TokenConfig:
		return true
	default:
		return false
	}
}

// expectIdent consumes a token that can serve as an identifier (including
// keywords that are valid field names).
func (p *parser) expectIdent() (Token, error) {
	tok := p.advance()
	if !isIdentLike(tok.Type) {
		return tok, fmt.Errorf("%d:%d: expected identifier, got %s (%q)",
			tok.Line, tok.Col, tok.Type, tok.Value)
	}
	return tok, nil
}

// parse is the top-level entry point.
func (p *parser) parse() (*Spec, error) {
	spec := &Spec{}

	// Parse top-level "use" directives.
	for p.peek().Type == TokenUse {
		p.advance() // consume "use"
		name, err := p.expect(TokenIdent)
		if err != nil {
			return nil, fmt.Errorf("parsing use: %w", err)
		}
		spec.Uses = append(spec.Uses, name.Value)
	}

	// Parse "spec Name { ... }"
	if _, err := p.expect(TokenSpec); err != nil {
		return nil, err
	}
	name, err := p.expect(TokenIdent)
	if err != nil {
		return nil, err
	}
	spec.Name = name.Value
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}

	// Parse spec body members until closing brace.
	for p.peek().Type != TokenRBrace && p.peek().Type != TokenEOF {
		if err := p.parseSpecMember(spec); err != nil {
			return nil, err
		}
	}

	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}

	return spec, nil
}

// specMemberParsers maps keyword tokens to their parse functions.
// Each returns the parsed value as an any for dispatch by parseSpecMember.
func (p *parser) specMemberParser(typ TokenType) func() (any, error) {
	switch typ {
	case TokenTarget:
		return wrap(p.parseTarget)
	case TokenModel:
		return wrap(p.parseModel)
	case TokenAction:
		return wrap(p.parseAction)
	case TokenLocators:
		return wrap(p.parseLocators)
	case TokenScope:
		return wrap(p.parseScope)
	case TokenImport:
		return wrap(p.parseImport)
	default:
		return nil
	}
}

func wrap[T any](fn func() (T, error)) func() (any, error) {
	return func() (any, error) { return fn() }
}

// parseSpecMember parses a single member inside a spec body.
func (p *parser) parseSpecMember(spec *Spec) error {
	tok := p.peek()

	// Handle description as an identifier, not a keyword.
	if tok.Type == TokenIdent && tok.Value == "description" {
		p.advance() // consume "description"
		if _, err := p.expect(TokenColon); err != nil {
			return err
		}
		val, err := p.expect(TokenString)
		if err != nil {
			return err
		}
		spec.Description = val.Value
		return nil
	}

	parse := p.specMemberParser(tok.Type)
	if parse == nil {
		return p.errAt(tok, fmt.Sprintf("unexpected token %s in spec body", tok.Type))
	}

	result, err := parse()
	if err != nil {
		return err
	}

	switch v := result.(type) {
	case *Target:
		spec.Target = v
	case *Model:
		spec.Models = append(spec.Models, v)
	case *Action:
		spec.Actions = append(spec.Actions, v)
	case *Scope:
		spec.Scopes = append(spec.Scopes, v)
	case map[string]string:
		spec.Locators = v
	case *importResult:
		spec.Models = append(spec.Models, v.Models...)
		spec.Scopes = append(spec.Scopes, v.Scopes...)
	}
	return nil
}

// parseTarget parses: target { key: value ... }
func (p *parser) parseTarget() (*Target, error) {
	p.advance() // consume "target"
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}

	t := &Target{Fields: make(map[string]Expr)}
	for p.peek().Type != TokenRBrace {
		key, err := p.expect(TokenIdent)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenColon); err != nil {
			return nil, err
		}
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		t.Fields[key.Value] = val
	}

	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return t, nil
}

// parseScope parses: scope name { config/contract/invariant/scenario ... }
func (p *parser) parseScope() (*Scope, error) {
	p.advance() // consume "scope"
	name, err := p.expectIdent()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}

	scope := &Scope{Name: name.Value}
	for p.peek().Type != TokenRBrace && p.peek().Type != TokenEOF {
		if err := p.parseScopeMember(scope); err != nil {
			return nil, err
		}
	}

	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return scope, nil
}

// parseScopeMember parses a single member inside a scope body.
func (p *parser) parseScopeMember(scope *Scope) error {
	tok := p.peek()
	switch tok.Type {
	case TokenConfig:
		config, err := p.parseScopeConfig()
		if err != nil {
			return err
		}
		scope.Config = config
	case TokenContract:
		c, err := p.parseContract()
		if err != nil {
			return err
		}
		scope.Contract = c
	case TokenInvariant:
		inv, err := p.parseInvariant()
		if err != nil {
			return err
		}
		scope.Invariants = append(scope.Invariants, inv)
	case TokenScenario:
		sc, err := p.parseScenario()
		if err != nil {
			return err
		}
		scope.Scenarios = append(scope.Scenarios, sc)
	default:
		return p.errAt(tok, fmt.Sprintf("unexpected token %s in scope body", tok.Type))
	}
	return nil
}

// parseScopeConfig parses: config { key: expr ... }
// The parser is agnostic to config key semantics — they're passed through to the adapter.
func (p *parser) parseScopeConfig() (map[string]Expr, error) {
	p.advance() // consume "config"
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}

	config := make(map[string]Expr)
	for p.peek().Type != TokenRBrace {
		key, err := p.expectIdent()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenColon); err != nil {
			return nil, err
		}
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		config[key.Value] = val
	}

	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return config, nil
}

// parseLocators parses: locators { name: [selector] ... }
func (p *parser) parseLocators() (map[string]string, error) {
	p.advance() // consume "locators"
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}

	locs := make(map[string]string)
	for p.peek().Type != TokenRBrace {
		key, err := p.expect(TokenIdent)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenColon); err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenLBracket); err != nil {
			return nil, err
		}
		sel, err := p.parseLocatorSelector()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenRBracket); err != nil {
			return nil, err
		}
		locs[key.Value] = sel
	}

	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return locs, nil
}

// parseLocatorSelector reads tokens between [ and ] as a raw selector string.
func (p *parser) parseLocatorSelector() (string, error) {
	// For now, expect a single string or ident inside brackets.
	tok := p.peek()
	if tok.Type == TokenString {
		p.advance()
		return tok.Value, nil
	}
	// Consume tokens until ] and concatenate them as a selector.
	var sel string
	for p.peek().Type != TokenRBracket && p.peek().Type != TokenEOF {
		t := p.advance()
		sel += t.Value
	}
	return sel, nil
}

// parseModel parses: model Name { field: type ... }
func (p *parser) parseModel() (*Model, error) {
	p.advance() // consume "model"
	name, err := p.expect(TokenIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}

	m := &Model{Name: name.Value}
	for p.peek().Type != TokenRBrace {
		field, err := p.parseField()
		if err != nil {
			return nil, err
		}
		m.Fields = append(m.Fields, field)
	}

	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return m, nil
}

// parseField parses: name: type {constraint}?
func (p *parser) parseField() (*Field, error) {
	name, err := p.expect(TokenIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenColon); err != nil {
		return nil, err
	}

	typeExpr, err := p.parseTypeExpr()
	if err != nil {
		return nil, err
	}

	f := &Field{Name: name.Value, Type: typeExpr}

	// Optional constraint block: { expr }
	if p.peek().Type == TokenLBrace {
		p.advance() // consume {
		constraint, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		f.Constraint = constraint
		if _, err := p.expect(TokenRBrace); err != nil {
			return nil, err
		}
	}

	return f, nil
}

// parseTypeExpr parses a type: ident or ident?
func (p *parser) parseTypeExpr() (TypeExpr, error) {
	name, err := p.expect(TokenIdent)
	if err != nil {
		return TypeExpr{}, err
	}
	te := TypeExpr{Name: name.Value}
	if p.peek().Type == TokenQuestion {
		p.advance()
		te.Optional = true
	}
	return te, nil
}

// parseContract parses: contract { input { ... } output { ... } }
func (p *parser) parseContract() (*Contract, error) {
	p.advance() // consume "contract"
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}

	c := &Contract{}
	for p.peek().Type != TokenRBrace {
		tok := p.peek()
		switch tok.Type {
		case TokenInput:
			p.advance()
			fields, err := p.parseFieldBlock()
			if err != nil {
				return nil, err
			}
			c.Input = fields
		case TokenOutput:
			p.advance()
			fields, err := p.parseFieldBlock()
			if err != nil {
				return nil, err
			}
			c.Output = fields
		default:
			return nil, p.errAt(
				tok,
				fmt.Sprintf("expected 'input' or 'output' in contract, got %s", tok.Type),
			)
		}
	}

	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return c, nil
}

// parseFieldBlock parses: { field: type ... }
func (p *parser) parseFieldBlock() ([]*Field, error) {
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}
	var fields []*Field
	for p.peek().Type != TokenRBrace {
		f, err := p.parseField()
		if err != nil {
			return nil, err
		}
		fields = append(fields, f)
	}
	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return fields, nil
}

// parseAction parses: action name(params) { steps }
func (p *parser) parseAction() (*Action, error) {
	p.advance() // consume "action"
	name, err := p.expect(TokenIdent)
	if err != nil {
		return nil, err
	}

	a := &Action{Name: name.Value}

	// Parse parameter list.
	if _, err := p.expect(TokenLParen); err != nil {
		return nil, err
	}
	for p.peek().Type != TokenRParen {
		param, err := p.parseParam()
		if err != nil {
			return nil, err
		}
		a.Params = append(a.Params, param)
		if p.peek().Type == TokenComma {
			p.advance()
		}
	}
	if _, err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	// Parse body.
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}
	for p.peek().Type != TokenRBrace {
		call, err := p.parseCall()
		if err != nil {
			return nil, err
		}
		a.Steps = append(a.Steps, call)
	}
	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}

	return a, nil
}

// parseParam parses: name: type
func (p *parser) parseParam() (*Param, error) {
	name, err := p.expect(TokenIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenColon); err != nil {
		return nil, err
	}
	typeExpr, err := p.parseTypeExpr()
	if err != nil {
		return nil, err
	}
	return &Param{Name: name.Value, Type: typeExpr}, nil
}

// parseCall parses: namespace.method(args) or method(args)
func (p *parser) parseCall() (*Call, error) {
	name, err := p.expect(TokenIdent)
	if err != nil {
		return nil, err
	}

	c := &Call{}
	if p.peek().Type == TokenDot {
		p.advance() // consume .
		method, err := p.expect(TokenIdent)
		if err != nil {
			return nil, err
		}
		c.Namespace = name.Value
		c.Method = method.Value
	} else {
		c.Method = name.Value
	}

	if _, err := p.expect(TokenLParen); err != nil {
		return nil, err
	}
	for p.peek().Type != TokenRParen {
		arg, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		c.Args = append(c.Args, arg)
		if p.peek().Type == TokenComma {
			p.advance()
		}
	}
	if _, err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	return c, nil
}

// parseInvariant parses: invariant name { [when expr:] assertions... }
func (p *parser) parseInvariant() (*Invariant, error) {
	p.advance() // consume "invariant"
	name, err := p.expect(TokenIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}

	inv := &Invariant{Name: name.Value}

	// Check for optional "when expr:" guard.
	if p.peek().Type == TokenWhen {
		p.advance() // consume "when"
		guard, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		inv.When = guard
		if _, err := p.expect(TokenColon); err != nil {
			return nil, err
		}
	}

	// Parse body assertions (boolean expressions) until closing brace.
	for p.peek().Type != TokenRBrace {
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		inv.Assertions = append(inv.Assertions, &Assertion{Expr: expr})
	}

	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return inv, nil
}

// parseScenario parses: scenario name { given/when/then blocks }
func (p *parser) parseScenario() (*Scenario, error) {
	p.advance() // consume "scenario"
	name, err := p.expect(TokenIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}

	sc := &Scenario{Name: name.Value}

	for p.peek().Type != TokenRBrace && p.peek().Type != TokenEOF {
		if err := p.parseScenarioBlock(sc); err != nil {
			return nil, err
		}
	}

	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return sc, nil
}

func (p *parser) parseScenarioBlock(sc *Scenario) error {
	tok := p.peek()
	switch tok.Type {
	case TokenGiven:
		p.advance()
		block, err := p.parseGivenBlock()
		if err != nil {
			return err
		}
		sc.Given = block
	case TokenWhen:
		p.advance()
		block, err := p.parseWhenBlock()
		if err != nil {
			return err
		}
		sc.When = block
	case TokenThen:
		p.advance()
		block, err := p.parseThenBlock()
		if err != nil {
			return err
		}
		sc.Then = block
	default:
		return p.errAt(
			tok,
			fmt.Sprintf("expected 'given', 'when', or 'then' in scenario, got %s", tok.Type),
		)
	}
	return nil
}

// parseGivenBlock parses: { assignments... }
func (p *parser) parseGivenBlock() (*Block, error) {
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}

	block := &Block{}
	for p.peek().Type != TokenRBrace {
		path, err := p.parseFieldPath()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenColon); err != nil {
			return nil, err
		}
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		block.Assignments = append(block.Assignments, &Assignment{
			Path:  path,
			Value: val,
		})
	}

	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return block, nil
}

// parseWhenBlock parses: { predicates... }
func (p *parser) parseWhenBlock() (*Block, error) {
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}

	block := &Block{}
	for p.peek().Type != TokenRBrace {
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		block.Predicates = append(block.Predicates, expr)
	}

	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return block, nil
}

// parseThenBlock parses: { assertions... }
// Assertions are in the form: path: expected
func (p *parser) parseThenBlock() (*Block, error) {
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}

	block := &Block{}
	for p.peek().Type != TokenRBrace {
		path, err := p.parseFieldPath()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenColon); err != nil {
			return nil, err
		}
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		block.Assertions = append(block.Assertions, &Assertion{
			Target:   path,
			Expected: val,
		})
	}

	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return block, nil
}

// parseFieldPath consumes a dotted identifier path like "from.balance".
func (p *parser) parseFieldPath() (string, error) {
	first, err := p.expectIdent()
	if err != nil {
		return "", err
	}
	path := first.Value
	for p.peek().Type == TokenDot {
		p.advance() // consume .
		next, err := p.expectIdent()
		if err != nil {
			return "", err
		}
		path += "." + next.Value
	}
	return path, nil
}

// --- Expression parser (Pratt / precedence climbing) ---

// Precedence levels (ascending).
const (
	precNone       = 0
	precOr         = 1
	precAnd        = 2
	precEquality   = 3
	precComparison = 4
	precAdditive   = 5
	precMultiply   = 6
)

// infixPrec returns the precedence of an infix operator token, or 0 if not infix.
func infixPrec(typ TokenType) int {
	switch typ {
	case TokenOr:
		return precOr
	case TokenAnd:
		return precAnd
	case TokenEq, TokenNeq:
		return precEquality
	case TokenLt, TokenGt, TokenLte, TokenGte:
		return precComparison
	case TokenPlus, TokenMinus:
		return precAdditive
	case TokenStar:
		return precMultiply
	default:
		return precNone
	}
}

var opStrings = map[TokenType]string{
	TokenEq:    "==",
	TokenNeq:   "!=",
	TokenGt:    ">",
	TokenLt:    "<",
	TokenGte:   ">=",
	TokenLte:   "<=",
	TokenPlus:  "+",
	TokenMinus: "-",
	TokenStar:  "*",
	TokenAnd:   "&&",
	TokenOr:    "||",
}

// opString returns the string representation of an operator token.
func opString(typ TokenType) string {
	if s, ok := opStrings[typ]; ok {
		return s
	}
	return "?"
}

// parseExpr parses an expression using precedence climbing.
func (p *parser) parseExpr() (Expr, error) {
	return p.parseExprPrec(precNone + 1)
}

// parseExprPrec parses an expression at the given minimum precedence.
func (p *parser) parseExprPrec(minPrec int) (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for {
		tok := p.peek()
		prec := infixPrec(tok.Type)
		if prec < minPrec {
			break
		}
		p.advance() // consume operator
		op := opString(tok.Type)

		// Left-associative: require strictly higher precedence on the right.
		right, err := p.parseExprPrec(prec + 1)
		if err != nil {
			return nil, err
		}
		left = BinaryOp{Left: left, Op: op, Right: right}
	}

	return left, nil
}

// parseUnary handles unary operators: !, -
func (p *parser) parseUnary() (Expr, error) {
	tok := p.peek()
	switch tok.Type {
	case TokenNot:
		p.advance()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return UnaryOp{Op: "!", Operand: operand}, nil
	case TokenMinus:
		p.advance()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return UnaryOp{Op: "-", Operand: operand}, nil
	default:
		return p.parseAtom()
	}
}

// parseAtom parses a primary expression: literal, field ref, env(), object, or grouped.
func (p *parser) parseAtom() (Expr, error) {
	tok := p.peek()

	switch tok.Type {
	case TokenInt:
		p.advance()
		v, err := strconv.Atoi(tok.Value)
		if err != nil {
			return nil, p.errAt(tok, fmt.Sprintf("invalid int: %s", tok.Value))
		}
		return LiteralInt{Value: v}, nil

	case TokenFloat:
		p.advance()
		v, err := strconv.ParseFloat(tok.Value, 64)
		if err != nil {
			return nil, p.errAt(tok, fmt.Sprintf("invalid float: %s", tok.Value))
		}
		return LiteralFloat{Value: v}, nil

	case TokenString:
		p.advance()
		return LiteralString{Value: tok.Value}, nil

	case TokenBool:
		p.advance()
		return LiteralBool{Value: tok.Value == "true"}, nil

	case TokenNull:
		p.advance()
		return LiteralNull{}, nil

	case TokenEnv:
		return p.parseEnvRef()

	case TokenLBrace:
		return p.parseObjectLiteral()

	case TokenLParen:
		p.advance() // consume (
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
		return expr, nil

	default:
		if isIdentLike(tok.Type) {
			return p.parseFieldRefExpr()
		}
		return nil, p.errAt(tok, fmt.Sprintf("unexpected token %s in expression", tok.Type))
	}
}

// parseFieldRefExpr parses a dotted identifier path as a FieldRef expression.
func (p *parser) parseFieldRefExpr() (Expr, error) {
	first := p.advance() // already confirmed isIdentLike
	path := first.Value
	for p.peek().Type == TokenDot {
		p.advance() // consume .
		next, err := p.expectIdent()
		if err != nil {
			return nil, err
		}
		path += "." + next.Value
	}
	return FieldRef{Path: path}, nil
}

// parseEnvRef parses: env(VAR) or env(VAR, "default")
func (p *parser) parseEnvRef() (Expr, error) {
	p.advance() // consume "env"
	if _, err := p.expect(TokenLParen); err != nil {
		return nil, err
	}
	varName, err := p.expect(TokenIdent)
	if err != nil {
		return nil, err
	}

	ref := EnvRef{Var: varName.Value}

	if p.peek().Type == TokenComma {
		p.advance() // consume ,
		def, err := p.expect(TokenString)
		if err != nil {
			return nil, err
		}
		ref.Default = def.Value
	}

	if _, err := p.expect(TokenRParen); err != nil {
		return nil, err
	}
	return ref, nil
}

// parseObjectLiteral parses: { key: value, ... }
func (p *parser) parseObjectLiteral() (Expr, error) {
	p.advance() // consume {
	obj := ObjectLiteral{}

	for p.peek().Type != TokenRBrace {
		key, err := p.expect(TokenIdent)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenColon); err != nil {
			return nil, err
		}
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		obj.Fields = append(obj.Fields, &ObjField{
			Key:   key.Value,
			Value: val,
		})
		if p.peek().Type == TokenComma {
			p.advance()
		}
	}

	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return obj, nil
}
