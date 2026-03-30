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
	imports ImportRegistry
	fileDir string
	tokens  []Token
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
		TokenScope, TokenConfig,
		TokenBefore,
		TokenLet, TokenReturn:
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

	// Spec-level "use" is no longer valid — plugins are declared at scope level.
	if p.peek().Type == TokenUse {
		tok := p.peek()
		return nil, p.errAt(tok, "'use' directive must be inside a scope block, not at spec level")
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

	// Validate: every scope must declare a plugin via 'use'.
	for _, scope := range spec.Scopes {
		if scope.Use == "" {
			return nil, fmt.Errorf("scope %q is missing a 'use <plugin>' directive", scope.Name)
		}
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
		// Convert v2 Action to v3 ActionDef
		ad := &ActionDef{Name: v.Name, Params: v.Params}
		for _, s := range v.Steps {
			ad.Body = append(ad.Body, s)
		}
		spec.Actions = append(spec.Actions, ad)
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

		if key.Value == "services" {
			if err := p.parseServices(t); err != nil {
				return nil, err
			}
			continue
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

// parseServices parses the services block inside target.
// Supports either compose shorthand or named service blocks.
func (p *parser) parseServices(t *Target) error {
	if _, err := p.expect(TokenLBrace); err != nil {
		return err
	}

	for p.peek().Type != TokenRBrace {
		key, err := p.expect(TokenIdent)
		if err != nil {
			return err
		}

		if key.Value == "compose" {
			if _, err := p.expect(TokenColon); err != nil {
				return err
			}
			val, err := p.expect(TokenString)
			if err != nil {
				return err
			}
			t.Compose = val.Value
			continue
		}

		svc, err := p.parseService(key.Value)
		if err != nil {
			return err
		}
		t.Services = append(t.Services, svc)
	}

	if _, err := p.expect(TokenRBrace); err != nil {
		return err
	}
	return nil
}

// parseService parses a named service block: name { build: "...", port: N, ... }
func (p *parser) parseService(name string) (*Service, error) {
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}

	svc := &Service{Name: name}
	for p.peek().Type != TokenRBrace {
		if err := p.parseServiceEntry(svc); err != nil {
			return nil, err
		}
	}

	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return svc, nil
}

// parseServiceEntry parses a single key-value or sub-block inside a service.
func (p *parser) parseServiceEntry(svc *Service) error {
	key := p.advance()
	if !isIdentLike(key.Type) && key.Type != TokenEnv {
		return fmt.Errorf("%d:%d: expected identifier, got %s (%q)",
			key.Line, key.Col, key.Type, key.Value)
	}

	switch key.Value {
	case "env":
		m, err := p.parseStringMap()
		if err != nil {
			return err
		}
		svc.Env = m
	case "volumes":
		m, err := p.parseStringMap()
		if err != nil {
			return err
		}
		svc.Volumes = m
	default:
		if _, err := p.expect(TokenColon); err != nil {
			return err
		}
		return p.parseServiceField(svc, key)
	}
	return nil
}

// parseServiceField parses a scalar field inside a service block.
func (p *parser) parseServiceField(svc *Service, key Token) error {
	switch key.Value {
	case "build":
		val, err := p.expect(TokenString)
		if err != nil {
			return err
		}
		svc.Build = val.Value
	case "image":
		val, err := p.expect(TokenString)
		if err != nil {
			return err
		}
		svc.Image = val.Value
	case "port":
		val, err := p.expect(TokenInt)
		if err != nil {
			return err
		}
		v, err := strconv.Atoi(val.Value)
		if err != nil {
			return p.errAt(val, fmt.Sprintf("invalid port: %s", val.Value))
		}
		svc.Port = v
	case "health":
		val, err := p.expect(TokenString)
		if err != nil {
			return err
		}
		svc.Health = val.Value
	default:
		return p.errAt(key, fmt.Sprintf("unknown service field %q", key.Value))
	}
	return nil
}

// parseStringMap parses: { key: "value", ... }
func (p *parser) parseStringMap() (map[string]string, error) {
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}

	m := make(map[string]string)
	for p.peek().Type != TokenRBrace {
		var keyVal string
		tok := p.peek()
		if tok.Type == TokenString {
			p.advance()
			keyVal = tok.Value
		} else {
			key, err := p.expectIdent()
			if err != nil {
				return nil, err
			}
			keyVal = key.Value
		}
		if _, err := p.expect(TokenColon); err != nil {
			return nil, err
		}
		val, err := p.expect(TokenString)
		if err != nil {
			return nil, err
		}
		m[keyVal] = val.Value
	}

	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return m, nil
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
	case TokenUse:
		p.advance() // consume "use"
		name, err := p.expect(TokenIdent)
		if err != nil {
			return err
		}
		if scope.Use != "" {
			return p.errAt(tok, fmt.Sprintf("scope %q has multiple 'use' directives", scope.Name))
		}
		scope.Use = name.Value
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
	case TokenBefore:
		if scope.Before != nil {
			return p.errAt(tok, fmt.Sprintf("scope %q has multiple 'before' blocks", scope.Name))
		}
		p.advance() // consume "before"
		block, err := p.parseGivenBlock()
		if err != nil {
			return err
		}
		scope.Before = block
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
	name, err := p.expectIdent()
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

// parseTypeExpr parses a type expression. The trailing ? binds to the
// outermost type: []int? means "optional array of int", not "array of optional int".
func (p *parser) parseTypeExpr() (TypeExpr, error) {
	te, err := p.parseTypeExprInner()
	if err != nil {
		return TypeExpr{}, err
	}
	if p.peek().Type == TokenQuestion {
		p.advance()
		te.Optional = true
	}
	return te, nil
}

// parseTypeExprInner parses the type without consuming a trailing ?.
func (p *parser) parseTypeExprInner() (TypeExpr, error) {
	// Array type: []T
	if p.peek().Type == TokenLBracket {
		p.advance() // consume [
		if _, err := p.expect(TokenRBracket); err != nil {
			return TypeExpr{}, err
		}
		elemType, err := p.parseTypeExprInner()
		if err != nil {
			return TypeExpr{}, err
		}
		return TypeExpr{Name: "array", ElemType: &elemType}, nil
	}

	name, err := p.expectIdent()
	if err != nil {
		return TypeExpr{}, err
	}

	// Map type: map[K, V]
	if name.Value == typeMap && p.peek().Type == TokenLBracket {
		return p.parseMapType()
	}

	// Enum type: enum("val1", "val2", ...)
	if name.Value == typeEnum && p.peek().Type == TokenLParen {
		return p.parseEnumType(name)
	}

	return TypeExpr{Name: name.Value}, nil
}

const (
	typeMap  = "map"
	typeEnum = "enum"
)

func (p *parser) parseMapType() (TypeExpr, error) {
	p.advance() // consume [
	keyType, err := p.parseTypeExprInner()
	if err != nil {
		return TypeExpr{}, err
	}
	if _, err := p.expect(TokenComma); err != nil {
		return TypeExpr{}, err
	}
	valType, err := p.parseTypeExprInner()
	if err != nil {
		return TypeExpr{}, err
	}
	if _, err := p.expect(TokenRBracket); err != nil {
		return TypeExpr{}, err
	}
	return TypeExpr{Name: typeMap, KeyType: &keyType, ValType: &valType}, nil
}

func (p *parser) parseEnumType(name Token) (TypeExpr, error) {
	p.advance() // consume (
	var variants []string
	for p.peek().Type != TokenRParen {
		if len(variants) > 0 {
			if _, err := p.expect(TokenComma); err != nil {
				return TypeExpr{}, err
			}
			// Allow trailing comma
			if p.peek().Type == TokenRParen {
				break
			}
		}
		tok, err := p.expect(TokenString)
		if err != nil {
			return TypeExpr{}, p.errAt(p.peek(), "enum variants must be string literals")
		}
		variants = append(variants, tok.Value)
	}
	if _, err := p.expect(TokenRParen); err != nil {
		return TypeExpr{}, err
	}
	if len(variants) == 0 {
		return TypeExpr{}, p.errAt(name, "enum type requires at least one variant")
	}
	return TypeExpr{Name: typeEnum, Variants: variants}, nil
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

// parseGivenBlock parses: { (assignments | calls)... }
// Distinguishes calls from assignments by lookahead:
//   - ident.ident( → namespaced call
//   - ident(       → local call
//   - ident:       → assignment
//   - ident.ident: → dotted-path assignment
func (p *parser) parseGivenBlock() (*Block, error) {
	if _, err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}

	block := &Block{}
	for p.peek().Type != TokenRBrace {
		step, err := p.parseGivenStep()
		if err != nil {
			return nil, err
		}
		block.Steps = append(block.Steps, step)
	}

	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return block, nil
}

// parseGivenStep parses a single step in a given block: either a call or an assignment.
func (p *parser) parseGivenStep() (GivenStep, error) {
	if p.isGivenCall() {
		return p.parseCall()
	}
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
	return &Assignment{Path: path, Value: val}, nil
}

// isGivenCall returns true if the current position starts a call (not an assignment).
// Patterns: ident( or ident.ident(
func (p *parser) isGivenCall() bool {
	if p.pos >= len(p.tokens) {
		return false
	}
	// ident( → local call
	if p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == TokenLParen {
		return true
	}
	// ident.ident( → namespaced call
	if p.pos+3 < len(p.tokens) &&
		p.tokens[p.pos+1].Type == TokenDot &&
		p.tokens[p.pos+3].Type == TokenLParen {
		return true
	}
	return false
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
		a, err := p.parseAssertion()
		if err != nil {
			return nil, err
		}
		block.Assertions = append(block.Assertions, a)
	}

	if _, err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	return block, nil
}

// parseAssertion parses a single then-block assertion:
// path: expected  OR  path@plugin.property: expected
func (p *parser) parseAssertion() (*Assertion, error) {
	path, err := p.parseFieldPath()
	if err != nil {
		return nil, err
	}

	a := &Assertion{Target: path}

	// Check for @plugin.property syntax: target@plugin.property: expected
	if p.peek().Type == TokenAt {
		p.advance() // consume @
		plugin, err := p.expectIdent()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenDot); err != nil {
			return nil, err
		}
		property, err := p.expectIdent()
		if err != nil {
			return nil, err
		}
		a.Plugin = plugin.Value
		a.Property = property.Value
	}

	// Accept ':' (sugar for ==) or a comparison operator.
	op := "=="
	tok := p.peek()
	switch tok.Type {
	case TokenColon:
		p.advance()
	case TokenGt, TokenGte, TokenLt, TokenLte, TokenEq, TokenNeq:
		p.advance()
		op = tok.Value
	default:
		return nil, p.errAt(tok, "expected ':' or comparison operator in assertion")
	}
	a.Operator = op

	val, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	a.Expected = val
	return a, nil
}

// parseFieldPath consumes a dotted identifier path like "from.balance" or
// "scopes.0.checks.3.inputs_run" (integer segments for array index access).
func (p *parser) parseFieldPath() (string, error) {
	first, err := p.expectIdent()
	if err != nil {
		return "", err
	}
	path := first.Value
	for p.peek().Type == TokenDot {
		p.advance() // consume .
		// Accept integer tokens as array index segments (e.g., "scopes.0.checks.3").
		if p.peek().Type == TokenInt {
			seg := p.advance()
			path += "." + seg.Value
			continue
		}
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
	case TokenStar, TokenSlash, TokenPercent:
		return precMultiply
	default:
		return precNone
	}
}

var opStrings = map[TokenType]string{
	TokenEq:      "==",
	TokenNeq:     "!=",
	TokenGt:      ">",
	TokenLt:      "<",
	TokenGte:     ">=",
	TokenLte:     "<=",
	TokenPlus:    "+",
	TokenMinus:   "-",
	TokenStar:    "*",
	TokenSlash:   "/",
	TokenPercent: "%",
	TokenAnd:     "&&",
	TokenOr:      "||",
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

	if expr, err := p.parseLiteralAtom(tok); expr != nil || err != nil {
		return expr, err
	}

	switch tok.Type {
	case TokenEnv:
		return p.parseEnvRef()

	case TokenService:
		return p.parseServiceRef()

	case TokenLBrace:
		return p.parseObjectLiteral()

	case TokenLBracket:
		return p.parseArrayLiteral()

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

	case TokenIf:
		return p.parseIfExpr()

	default:
		return p.parseAtomDefault(tok)
	}
}

// parseLiteralAtom handles literal tokens (int, float, string, bool, null).
// Returns (nil, nil) if the current token is not a literal.
func (p *parser) parseLiteralAtom(tok Token) (Expr, error) {
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
	default:
		return nil, nil
	}
}

// parseAtomDefault handles the default branch of parseAtom: built-in function
// calls and field references.
func (p *parser) parseAtomDefault(tok Token) (Expr, error) {
	if tok.Type == TokenIdent {
		switch tok.Value {
		case "len":
			return p.parseLenExpr()
		case "all":
			return p.parseQuantifierExpr("all")
		case "any":
			return p.parseQuantifierExpr("any")
		case "contains":
			return p.parseContainsExpr()
		case "exists":
			return p.parseExistsExpr()
		case "has_key":
			return p.parseHasKeyExpr()
		}
	}
	if isIdentLike(tok.Type) {
		return p.parseFieldRefExpr()
	}
	return nil, p.errAt(tok, fmt.Sprintf("unexpected token %s in expression", tok.Type))
}

// parseFieldRefExpr parses a dotted identifier path as a FieldRef expression.
func (p *parser) parseFieldRefExpr() (Expr, error) {
	first := p.advance() // already confirmed isIdentLike
	path := first.Value
	for p.peek().Type == TokenDot {
		p.advance() // consume .
		// Accept integer tokens as array index segments (e.g., "output.items.0").
		if p.peek().Type == TokenInt {
			seg := p.advance()
			path += "." + seg.Value
			continue
		}
		next, err := p.expectIdent()
		if err != nil {
			return nil, err
		}
		path += "." + next.Value
	}
	return FieldRef{Path: path}, nil
}

// parseLenExpr parses: len(expr)
func (p *parser) parseLenExpr() (Expr, error) {
	p.advance() // consume "len"
	if _, err := p.expect(TokenLParen); err != nil {
		return nil, err
	}
	arg, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenRParen); err != nil {
		return nil, err
	}
	return LenExpr{Arg: arg}, nil
}

// parseQuantifierExpr parses: all(expr, ident => expr) or any(expr, ident => expr)
// The "=>" arrow is lexed as TokenAssign followed by TokenGt.
func (p *parser) parseQuantifierExpr(name string) (Expr, error) {
	p.advance() // consume "all" or "any"
	if _, err := p.expect(TokenLParen); err != nil {
		return nil, err
	}
	arrayExpr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenComma); err != nil {
		return nil, err
	}
	boundVar, err := p.expectIdent()
	if err != nil {
		return nil, err
	}
	// Expect "=>" as two tokens: = then >
	if _, err := p.expect(TokenAssign); err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenGt); err != nil {
		return nil, err
	}
	predicate, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	if name == "all" {
		return AllExpr{Array: arrayExpr, BoundVar: boundVar.Value, Predicate: predicate}, nil
	}
	return AnyExpr{Array: arrayExpr, BoundVar: boundVar.Value, Predicate: predicate}, nil
}

// parseContainsExpr parses: contains(haystack, needle)
func (p *parser) parseContainsExpr() (Expr, error) {
	p.advance() // consume "contains"
	if _, err := p.expect(TokenLParen); err != nil {
		return nil, err
	}
	haystack, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenComma); err != nil {
		return nil, err
	}
	needle, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenRParen); err != nil {
		return nil, err
	}
	return ContainsExpr{Haystack: haystack, Needle: needle}, nil
}

// parseExistsExpr parses: exists(expr)
func (p *parser) parseExistsExpr() (Expr, error) {
	p.advance() // consume "exists"
	if _, err := p.expect(TokenLParen); err != nil {
		return nil, err
	}
	arg, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenRParen); err != nil {
		return nil, err
	}
	return ExistsExpr{Arg: arg}, nil
}

// parseHasKeyExpr parses: has_key(expr, key)
func (p *parser) parseHasKeyExpr() (Expr, error) {
	p.advance() // consume "has_key"
	if _, err := p.expect(TokenLParen); err != nil {
		return nil, err
	}
	arg, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenComma); err != nil {
		return nil, err
	}
	key, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenRParen); err != nil {
		return nil, err
	}
	return HasKeyExpr{Arg: arg, Key: key}, nil
}

// parseIfExpr parses: if expr then expr else expr
func (p *parser) parseIfExpr() (Expr, error) {
	p.advance() // consume "if"
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenThen); err != nil {
		return nil, err
	}
	thenExpr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenElse); err != nil {
		return nil, err
	}
	elseExpr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return IfExpr{Condition: cond, Then: thenExpr, Else: elseExpr}, nil
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

// parseServiceRef parses: service(name)
func (p *parser) parseServiceRef() (Expr, error) {
	p.advance() // consume "service"
	if _, err := p.expect(TokenLParen); err != nil {
		return nil, err
	}
	name, err := p.expectIdent()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenRParen); err != nil {
		return nil, err
	}
	return ServiceRef{Name: name.Value}, nil
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

// parseArrayLiteral parses: [ expr, expr, ... ]
func (p *parser) parseArrayLiteral() (Expr, error) {
	p.advance() // consume [
	arr := ArrayLiteral{}

	for p.peek().Type != TokenRBracket && p.peek().Type != TokenEOF {
		elem, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		arr.Elements = append(arr.Elements, elem)
		if p.peek().Type == TokenComma {
			p.advance()
		}
	}

	if _, err := p.expect(TokenRBracket); err != nil {
		return nil, err
	}
	return arr, nil
}
