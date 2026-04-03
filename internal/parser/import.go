package parser

import (
	"fmt"
	"path/filepath"

	"github.com/bamsammich/speclang/v3/pkg/spec"
)

// Import type aliases — all types are defined in pkg/spec and re-exported here
// for backward compatibility.

type ImportResolver = spec.ImportResolver
type ImportRegistry = spec.ImportRegistry

// importResult wraps models and scopes returned by an import resolver
// for dispatch by parseSpecMember.
type importResult struct {
	Models []*Model
	Scopes []*Scope
}

// parseImport parses: import <adapter>("<path>")
func (p *parser) parseImport() (*importResult, error) {
	importTok := p.peek()
	p.advance() // consume "import"

	// Read adapter name identifier (e.g., "openapi")
	adapterTok := p.peek()
	if adapterTok.Type != TokenIdent {
		return nil, p.errAt(
			adapterTok,
			fmt.Sprintf("expected import adapter name, got %s", adapterTok.Type),
		)
	}
	p.advance()

	// Expect ( "path" )
	if _, err := p.expect(TokenLParen); err != nil {
		return nil, err
	}
	pathTok, err := p.expect(TokenString)
	if err != nil {
		return nil, p.errAt(p.peek(), "expected string path in import")
	}
	if _, err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	// Look up resolver
	if p.imports == nil {
		return nil, p.errAt(
			importTok,
			fmt.Sprintf("no import resolvers registered (for %q)", adapterTok.Value),
		)
	}
	resolver, ok := p.imports[adapterTok.Value]
	if !ok {
		return nil, p.errAt(adapterTok, fmt.Sprintf("unknown import adapter %q", adapterTok.Value))
	}

	// Resolve path relative to spec file directory
	relPath := pathTok.Value
	absPath := relPath
	if p.fileDir != "" {
		absPath = filepath.Join(p.fileDir, relPath)
	}
	resolved, err := filepath.Abs(absPath)
	if err != nil {
		return nil, p.errAt(pathTok, fmt.Sprintf("resolving import path: %v", err))
	}

	models, scopes, err := resolver.Resolve(resolved)
	if err != nil {
		return nil, p.errAt(
			pathTok,
			fmt.Sprintf("import %s(%q): %v", adapterTok.Value, relPath, err),
		)
	}

	return &importResult{Models: models, Scopes: scopes}, nil
}
