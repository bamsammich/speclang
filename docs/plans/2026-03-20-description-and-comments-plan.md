# Spec Description Field & Comments Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `description: "..."` field to the `spec` block and add comments to all existing spec files.

**Architecture:** The `description` field is parsed as an identifier check in `parseSpecMember` (not a keyword token). Comments already work at the lexer level (`#` syntax, silently skipped). The only code changes are AST + parser; comments are purely spec file content changes.

**Tech Stack:** Go, existing parser infrastructure

---

### Task 1: Add description field to AST and parser

**Files:**
- Modify: `pkg/parser/ast.go:4-12` (Spec struct)
- Modify: `pkg/parser/parser.go:179-204` (parseSpecMember)
- Modify: `pkg/parser/parser_test.go` (add test)
- Modify: `testdata/self/minimal.spec` (add description for test fixture)

**Step 1: Write the failing test**

Add to `pkg/parser/parser_test.go` after the existing imports:

```go
func TestParse_Description(t *testing.T) {
	t.Parallel()

	spec, err := parser.Parse(`spec Foo {
  description: "A test specification"
}`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if spec.Description != "A test specification" {
		t.Errorf("expected description %q, got %q", "A test specification", spec.Description)
	}
}

func TestParse_DescriptionOptional(t *testing.T) {
	t.Parallel()

	spec, err := parser.Parse(`spec Foo {}`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if spec.Description != "" {
		t.Errorf("expected empty description, got %q", spec.Description)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `mise exec -- go test ./pkg/parser/ -run TestParse_Description -v`
Expected: FAIL — `spec.Description` field does not exist

**Step 3: Add Description field to Spec struct**

In `pkg/parser/ast.go`, add `Description` field to the `Spec` struct:

```go
type Spec struct {
	Uses        []string          `json:"uses,omitempty"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Target      *Target           `json:"target,omitempty"`
	Locators    map[string]string `json:"locators,omitempty"`
	Models      []*Model          `json:"models,omitempty"`
	Actions     []*Action         `json:"actions,omitempty"`
	Scopes      []*Scope          `json:"scopes,omitempty"`
}
```

**Step 4: Add description parsing to parseSpecMember**

In `pkg/parser/parser.go`, modify `parseSpecMember` to check for `description` as an identifier before dispatching to `specMemberParser`:

```go
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
	}
	return nil
}
```

**Step 5: Run tests to verify they pass**

Run: `mise exec -- go test ./pkg/parser/ -v`
Expected: PASS (all existing + new tests)

**Step 6: Run full suite**

Run: `mise exec -- go test ./...`
Expected: PASS

**Step 7: Commit**

```bash
git add pkg/parser/ast.go pkg/parser/parser.go pkg/parser/parser_test.go
git commit -m "feat(parser): add description field to spec block"
```

---

### Task 2: Add description and comments to example and self-verification specs

**Files:**
- Modify: `examples/transfer.spec`
- Modify: `examples/models/account.spec`
- Modify: `examples/scopes/transfer.spec`
- Modify: `specs/speclang.spec`
- Modify: `specs/parse.spec`
- Modify: `specs/generate.spec`
- Modify: `specs/verify.spec`

**Step 1: Update `examples/transfer.spec`**

```
use http

spec AccountAPI {
  description: "REST API for inter-account money transfers with balance tracking"

  target {
    base_url: env(APP_URL, "http://localhost:8080")
  }

  include "models/account.spec"
  include "scopes/transfer.spec"
}
```

**Step 2: Update `examples/models/account.spec`**

```
# Shared account model used across transfer scenarios.
model Account {
  id: string
  balance: int
}
```

**Step 3: Update `examples/scopes/transfer.spec`**

```
scope transfer {
  config {
    path: "/api/v1/accounts/transfer"
    method: "POST"
  }

  contract {
    input {
      from: Account
      to: Account
      amount: int { 0 < amount <= from.balance }
    }
    output {
      from: Account
      to: Account
      error: string?
    }
  }

  # Money is neither created nor destroyed on successful transfers.
  invariant conservation {
    when error == null:
      output.from.balance + output.to.balance
        == input.from.balance + input.to.balance
  }

  # Balances must never go negative, even on error.
  invariant non_negative {
    output.from.balance >= 0
    output.to.balance >= 0
  }

  # Failed transfers must not change any balances.
  invariant no_mutation_on_error {
    when error != null:
      output.from.balance == input.from.balance
      output.to.balance == input.to.balance
  }

  # Smoke test: a concrete successful transfer.
  scenario success {
    given {
      from: { id: "alice", balance: 100 }
      to: { id: "bob", balance: 50 }
      amount: 30
    }
    then {
      from.balance: 70
      to.balance: 80
      error: null
    }
  }

  # Generative: any amount exceeding balance must be rejected.
  scenario overdraft {
    when {
      amount > from.balance
    }
    then {
      error: "insufficient_funds"
    }
  }

  # Generative: zero-amount transfers are invalid.
  scenario zero_transfer {
    when {
      amount == 0
    }
    then {
      error: "invalid_amount"
    }
  }
}
```

**Step 4: Update `specs/speclang.spec`**

```
use process

# Self-verification: speclang verifying its own runtime behavior.
spec Speclang {
  description: "Black-box verification of the specrun CLI: parsing, generation, and end-to-end verify"

  target {
    command: env(SPECRUN_BIN, "./specrun")
  }

  include "parse.spec"
  include "generate.spec"
  include "verify.spec"
}
```

**Step 5: Update `specs/parse.spec`**

```
# Verifies the parser accepts valid specs and produces expected AST structure.
scope parse_valid {
  config {
    args: "parse"
  }

  contract {
    input {
      file: string
    }
    output {
      exit_code: int
      name: string
    }
  }

  scenario minimal_spec {
    given {
      file: "testdata/self/minimal.spec"
    }
    then {
      exit_code: 0
      name: "Minimal"
    }
  }

  scenario transfer_spec {
    given {
      file: "examples/transfer.spec"
    }
    then {
      exit_code: 0
      name: "AccountAPI"
    }
  }
}

# Verifies the parser rejects malformed specs with a non-zero exit code.
scope parse_invalid {
  config {
    args: "parse"
  }

  contract {
    input {
      file: string
    }
    output {
      exit_code: int
    }
  }

  scenario unterminated_spec {
    given {
      file: "testdata/self/invalid_unterminated.spec"
    }
    then {
      exit_code: 1
    }
  }

  scenario circular_include {
    given {
      file: "testdata/include/circular/a.spec"
    }
    then {
      exit_code: 1
    }
  }
}
```

**Step 6: Update `specs/generate.spec`**

```
# Verifies the generator produces constraint-satisfying outputs across seeds.
scope generate {
  config {
    args: "generate examples/transfer.spec --scope transfer --seed"
  }

  contract {
    input {
      seed: int
    }
    output {
      exit_code: int
      amount: int
      from: any
      to: any
    }
  }

  invariant produces_output {
    exit_code == 0
  }

  # Generated amounts must satisfy the declared constraint: 0 < amount <= from.balance.
  invariant constraints_satisfied {
    when exit_code == 0:
      output.amount > 0
      output.amount <= output.from.balance
  }
}
```

**Step 7: Update `specs/verify.spec`**

```
# Verifies that specrun verify passes correct implementations.
scope verify_pass {
  config {
    args: "verify --json"
  }

  contract {
    input {
      file: string
    }
    output {
      exit_code: int
      scenarios_run: int
      scenarios_passed: int
      invariants_checked: int
      invariants_passed: int
      scopes: any
    }
  }

  # End-to-end: the transfer example must pass all checks.
  scenario transfer_spec_passes {
    given {
      file: "examples/transfer.spec"
    }
    then {
      exit_code: 0
      scenarios_run: 3
      scenarios_passed: 3
      invariants_checked: 3
      invariants_passed: 3
    }
  }
}
```

**Step 8: Run full suite**

Run: `mise exec -- go test ./...`
Expected: PASS (comments are ignored by lexer, description is parsed)

**Step 9: Commit**

```bash
git add examples/ specs/
git commit -m "docs(spec): add description and comments to all spec files"
```

---

### Task 3: Add comments to testdata spec fixtures

Only add comments where they clarify the fixture's purpose. Don't add comments to trivially-obvious fixtures (like `minimal.spec`) or error case fixtures where the lack of clarity is intentional.

**Files:**
- Modify: `testdata/self/broken_transfer.spec`
- Modify: `testdata/include/basic/root.spec`
- Modify: `testdata/include/basic/models.spec`
- Modify: `testdata/include/basic/scopes.spec`
- Modify: `testdata/include/nested/root.spec`
- Modify: `testdata/include/nested/mid.spec`
- Modify: `testdata/include/nested/leaf.spec`
- Modify: `testdata/include/duplicate/root.spec`
- Modify: `testdata/include/duplicate_scope/root.spec`

**Step 1: Update `testdata/self/broken_transfer.spec`**

Add a comment at top:

```
# Test fixture: transfer spec targeting a broken server (wrong balances).
use http
```

(rest of file unchanged)

**Step 2: Update `testdata/include/basic/root.spec`**

```
# Test fixture: basic include resolution (root -> models + scopes).
use http

spec TestAPI {
  target {
    base_url: env(APP_URL, "http://localhost:8080")
  }

  include "models.spec"
  include "scopes.spec"
}
```

**Step 3: Update `testdata/include/basic/models.spec`**

```
# Included by basic/root.spec.
model Account {
  id: string
  balance: int
}
```

**Step 4: Update `testdata/include/basic/scopes.spec`**

```
# Included by basic/root.spec.
scope transfer {
  config {
    path: "/transfer"
    method: "POST"
  }

  contract {
    input {
      from: Account
      to: Account
      amount: int
    }
    output {
      from: Account
      to: Account
      error: string?
    }
  }

  scenario success {
    given {
      from: { id: "alice", balance: 100 }
      to: { id: "bob", balance: 50 }
      amount: 30
    }
    then {
      from.balance: 70
      to.balance: 80
      error: null
    }
  }
}
```

**Step 5: Update `testdata/include/nested/root.spec`**

```
# Test fixture: nested include resolution (root -> mid -> leaf).
use http

spec Nested {
  include "mid.spec"
}
```

**Step 6: Update `testdata/include/nested/mid.spec`**

```
# Transitive include: pulls in leaf.spec, then defines its own model.
include "leaf.spec"

model Container {
  count: int
}
```

**Step 7: Update `testdata/include/nested/leaf.spec`**

```
# Leaf of the nested include chain.
model Item {
  name: string
}
```

**Step 8: Update `testdata/include/duplicate/root.spec`**

```
# Test fixture: duplicate model names across includes (should error).
spec Dup {
  include "models_a.spec"
  include "models_b.spec"
}
```

**Step 9: Update `testdata/include/duplicate_scope/root.spec`**

```
# Test fixture: duplicate scope names across includes (should error).
spec DupScope {
  include "scope_a.spec"
  include "scope_b.spec"
}
```

**Step 10: Run full suite**

Run: `mise exec -- go test ./...`
Expected: PASS

**Step 11: Commit**

```bash
git add testdata/
git commit -m "docs(spec): add comments to testdata fixtures"
```

---

### Task 4: Update CLAUDE.md spec syntax documentation

**Files:**
- Modify: `CLAUDE.md` (Spec File Structure section)

**Step 1: Update the Spec File Structure in CLAUDE.md**

In the spec file structure example, add `description` after the spec name:

```
spec <Name> {

  description: "<description>"            # optional, for AI context

  target {
```

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add description field to spec syntax documentation"
```
