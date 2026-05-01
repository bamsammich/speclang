# Writing specs that prove something

SpecLang makes it trivial to generate 10,000 inputs against your system and watch them all pass. That is not the same as proving your system works. This guide is about the difference.

## The SLO analogy

Writing a spec is like writing an SLO or an SLA. It's easy to write an SLO nobody can violate — "our service has uptime greater than 0%". It looks like a commitment. It imposes no discipline. It measures nothing anyone cares about.

It's hard to write an SLO that forces a real quality bar: "p99 latency under 100ms during business hours, measured from the edge, excluding client-side timeouts". Every word is load-bearing. Every clause excludes a class of cheating. You had to think about what "good enough" actually means for your users, then translate that into something a monitor can evaluate.

Specs have exactly the same shape. The question is never "does my spec pass". It's **"does the fact that my spec passes mean anything"**. A spec that passes 100 iterations while asserting nothing of consequence is worse than no spec — it gives you false confidence.

## The core trap: tautology

In a unit test, the tautology is usually visible:

```go
assert_eq!(1 + 1, 2)
```

You can see the constants. You can see there's no system under test. The tautology is local.

In a spec, the tautology is indirect. The invariant references the output of an action, over generated inputs, through an adapter, across many iterations. The mechanics look busy. If the invariant is weak — if the generator cannot produce an input that could falsify it, or if the invariant refers back to its own input through the action in a way that makes it self-consistent rather than system-testing — you get a green check that proves nothing.

You will not notice this by running the spec. **Green is the failure mode.** The spec has to be wrong on paper, before it runs.

## Anti-pattern gallery

Each anti-pattern below has a compilable, v4-syntax example of the bad version and a better version. The bad examples are syntactically valid — the problem is always semantic.

### 1. The invariant that can't fail

**Bad**:

```
contract Transfer(
  from: Account,
  to: Account,
  amount: int { 0 < amount <= from.balance },
) -> TransferResult {
  action {
    return http.post("/transfer", { from: from, to: to, amount: amount })
  }

  invariant responds {
    output != null
  }
}
```

`output != null` is true for any response the server produces, as long as the adapter parsed it. A server that returns a 500 with an empty JSON body still passes this. A server that returns the wrong account balances still passes this. An invariant that a broken server satisfies is not an invariant — it's decoration.

**Better** — relate input to output through the system's actual promise:

```
invariant conservation {
  output.from.balance + output.to.balance == from.balance + to.balance
}
```

Now a server that forgets to debit the source account fails. A server that double-credits fails. The invariant **bites** on the wrong implementations.

### 2. Asserting on the input, not the output

**Bad**:

```
scenario small_transfer {
  when { amount < 10 }
  then {
    from.balance >= 0
    amount > 0
  }
}
```

`amount > 0` was already on the field constraint (`0 < amount <= from.balance`). `from.balance >= 0` is on the `Account` model or implied by the constraint. This `then` block is asserting that the generator respected its own constraints — which it will always do. The system under test is never consulted.

**Better** — assert on `output.<something>`:

```
scenario small_transfer {
  when { amount < 10 }
  then {
    output.error == null
    output.from.balance == from.balance - amount
  }
}
```

### 3. The scenario with no `when`

**Bad**:

```
scenario overdraft {
  then {
    output.error == "insufficient_funds"
  }
}
```

No `when` means the generator picks any valid input matching the contract's field constraints. Most of those inputs will have `amount <= from.balance` and the transfer will succeed, so `output.error` will be `null`. The scenario fails on most iterations, confusingly, because you didn't narrow to the case the assertion is about.

**Better** — narrow with `when`:

```
scenario overdraft {
  when { amount > from.balance }
  then { output.error == "insufficient_funds" }
}
```

Now the generator only produces inputs in the overdraft class, and the assertion is specific to that class.

### 4. The action that re-implements the check

**Bad**:

```
action {
  let expected_from = from.balance - amount
  let expected_to = to.balance + amount
  return { from: { balance: expected_from }, to: { balance: expected_to }, error: null }
}

invariant conservation {
  output.from.balance + output.to.balance == from.balance + to.balance
}
```

The `action` is computing the correct answer locally and returning it. The invariant is checking arithmetic you just performed in the action. The system under test is nowhere in this spec. It's a green check on the spec's internal consistency.

**Better** — the `action` invokes the system through an adapter; the invariant checks a property of what the **system** returned:

```
action {
  return http.post("/api/v1/accounts/transfer", {
    from: from, to: to, amount: amount
  })
}

invariant conservation {
  output.from.balance + output.to.balance == from.balance + to.balance
}
```

### 5. The invariant that only fires on the happy path

**Bad**:

```
invariant conservation {
  output.error == null implies
    output.from.balance + output.to.balance == from.balance + to.balance
}
```

This is a valid invariant — it correctly uses `implies` to guard the assertion on the success case. But it's **incomplete**. The generator will produce inputs where the system returns an error. When it does, the invariant trivially holds (the antecedent is false), and your spec says nothing about what must be true on the error path.

What should be true on the error path? Probably: nothing mutated. Possibly: a specific error code. Maybe: a specific audit event. The spec should have a dual invariant.

**Better** — pair every happy-path law with an error-path law:

```
invariant conservation {
  output.error == null implies
    output.from.balance + output.to.balance == from.balance + to.balance
}

invariant no_mutation_on_error {
  output.error != null implies
    output.from.balance == from.balance
    and output.to.balance == to.balance
}
```

### 6. The scope with no verification

**Bad**:

```
scope transfer {
  before {
    login("admin", "password")
  }

  after {
    cleanup()
  }
}
```

The scope sets up state and tears it down. It contains no contracts, no invariants, no scenarios. `specrun verify` reports "pass" because there's nothing to fail. You have elaborate plumbing producing a green check that reflects nothing about the system.

**Better** — if a scope has no verification, delete it. Every scope exists to enforce something. Use `specrun verify --json` and check that `scenarios_run + invariants_checked > 0` per scope in CI.

### 7. The redundant scenario

**Bad**:

```
scenario success {
  given {
    from: { id: "alice", balance: 100 }
    to: { id: "bob", balance: 50 }
    amount: 30
  }
  then {
    output != null
  }
}
```

The `given` block picks a specific interesting point. The `then` block asserts something that would be true for **any** valid input, not something specific to this point. The scenario is a worse version of an invariant — if `output != null` should hold universally, move it to an `invariant` block. But it shouldn't hold, because (see anti-pattern 1) that invariant proves nothing either.

**Better** — scenarios with `given` are documentation and smoke-tests for **specific interesting points**. The `then` should assert something specific to that point:

```
scenario success {
  given {
    from: { id: "alice", balance: 100 }
    to: { id: "bob", balance: 50 }
    amount: 30
  }
  then {
    output.from.balance == 70
    output.to.balance == 80
    output.error == null
  }
}
```

Concrete, falsifiable, serves as executable documentation of the expected behavior at a canonical input.

## Positive patterns — what "good" looks like

### Every invariant references `output`

Grep your spec. If an invariant block has no `output.<something>` reference, that invariant is either restating a generator constraint or asserting something on the input. It cannot be checking system behavior.

### Every invariant would fail on a sketched-wrong implementation

Before committing a spec, stop and sketch one or two buggy implementations in your head:

- "What if the server swaps from and to?"
- "What if the server returns the old balances instead of the new ones?"
- "What if it returns success on overdraft but doesn't move money?"

For each: does at least one invariant or `when`-scenario fail? If the answer is "no" for any of them, you're missing a law. Add it.

### Use both `when` and `given`

- **`scenario when { predicate } then { assertion }`** tells you "across the full variation of inputs matching `predicate`, this assertion holds". It's a property over a class.
- **`scenario given { concrete values } then { assertion }`** tells you "at this named point, this assertion holds". It's documentation.

Both are useful. They serve different purposes. Use `when` for generative coverage of a class (overdraft, zero-amount, same-account, empty-string id). Use `given` for canonical examples a reader can run to understand the system.

### Error paths are first-class

For every promise you make about the happy path, answer: **what must hold when the error path is taken**? Usually at least one of:

- Nothing mutated. (`output.from.balance == from.balance`)
- A specific error code. (`output.error == TransferError.insufficient_funds`)
- A bounded error set. (`output.error in (TransferError.insufficient_funds, TransferError.invalid_amount)`)

A spec with no error-path invariants is a spec that doesn't care what happens when the system fails — which almost nothing you ship is actually true about.

### Invariants compose a system of laws

Most real systems satisfy a small set of named properties. If you can name the law, you can usually write the invariant:

- **Conservation** — total of X across before and after is equal. (accounting, inventory, quota)
- **Monotonicity** — Y never decreases (or never increases) over the operation. (sequence numbers, timestamps, audit counters)
- **Idempotence** — applying the operation twice yields the same result as applying it once. (PUTs, upserts, retries)
- **Bounds** — some Z stays within a declared range regardless of input. (balances non-negative, counters below quota)
- **No-op under no-change** — when inputs imply no work, nothing is mutated. (self-transfer, zero-amount)
- **Determinism under fixed input** — same input produces same output across retries.

If your spec has none of these shapes, ask whether you're verifying a system or describing a data shape.

### Named enums for error codes

Inline string comparisons for errors drift:

```
# Fragile: a typo here is a silent failure
then { output.error == "insufficent_funds" }
```

Named enums give you compile-time validation — the parser rejects variants that aren't declared:

```
enum TransferError { insufficient_funds, invalid_amount, same_account }

then { output.error == TransferError.insufficient_funds }
```

Now your typos become parse errors instead of silently-passing scenarios.

## Spec structure: standalone beats composed

A spec should be self-contained where possible. One `.spec` file, one responsibility, runnable on its own. When you have multiple specs that share data types, factor the shared models into a library file and `include` it from each spec — then run all specs with `specrun verify specs/*.spec`. Each spec is its own verification unit; failures in one do not affect others.

The alternative — assembling specs into a super-spec via `include` — does not work: every declaration in the include graph must be unique, so two runnable specs that each define the same model cannot both be included into a root file. If you hit a `duplicate declaration` error, that is the system telling you to use the glob pattern instead of the super-spec pattern.

See [Includes vs. glob execution](language-reference.md#includes-vs-glob-execution) for the full explanation.

## A pre-commit checklist

Run through this before you commit a spec. Any "no" is a problem to fix, not a comment to defer.

- [ ] Does every invariant reference `output.<something>`?
- [ ] Can I sketch an implementation that passes all my invariants but is wrong? If yes, I'm missing an invariant.
- [ ] Is there at least one invariant for the error path?
- [ ] Does `specrun verify --json` show non-zero `scenarios_run` and `invariants_checked` per scope?
- [ ] For every `when`, does the `then` say something **specific to that predicate's class**, not something that would be true everywhere?
- [ ] If I deleted a scenario, does the spec still make sense as a system of laws? If yes, the scenario is either redundant or a useful named edge case — decide which and either delete it or justify why it stays.
- [ ] Do error-case assertions reference named enum variants (`TransferError.insufficient_funds`), not bare strings?
- [ ] Does each contract's `action` block invoke the system (through an adapter) rather than compute the expected answer locally?

## Closing thought

SpecLang's generator makes it feel like you've tested a lot. One invariant run with 100 iterations looks like 100 tests in a dashboard.

Don't confuse coverage of generated inputs with coverage of the system's contract. The contract is what **you wrote**. If what you wrote is true by construction, 10,000 iterations is 10,000 confirmations of nothing.

The work is in the invariants. Spend time there.
