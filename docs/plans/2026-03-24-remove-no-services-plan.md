# Remove SPECRUN_NO_SERVICES Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove the `SPECRUN_NO_SERVICES` workaround entirely so specrun manages its own containers everywhere — CI, local dev, no escape hatches.

**Architecture:** Remove env var check from runner, remove fallback port resolution, simplify CI to let specrun start containers via Docker, ensure port collision produces a clear error.

**Tech Stack:** Go, Docker (already integrated)

---

### Task 1: Remove SPECRUN_NO_SERVICES from code

**Files:**
- Modify: `cmd/specrun/main.go`

**Step 1: Remove env var check**

Find where `os.Getenv("SPECRUN_NO_SERVICES")` is checked (in `startServices` or the verify action). Remove the check entirely — services always start if declared.

**Step 2: Remove os.Setenv auto-propagation**

Find `os.Setenv("SPECRUN_NO_SERVICES", "1")` in `startServices` (set after starting containers). Remove it.

**Step 3: Remove resolveServiceURL fallback**

Find `resolveServiceURL` — it currently falls back to the declared port when services aren't running. Remove the fallback. If a service isn't in the `runningServices` list, return `""` (adapter Init will fail with a clear error about missing base_url).

```go
func resolveServiceURL(name string, _ *parser.Target, services []infra.RunningService) string {
	for _, svc := range services {
		if svc.Name == name {
			return svc.URL
		}
	}
	return ""
}
```

**Step 4: Verify**

```bash
go build ./cmd/specrun
go test ./... -count=1   # some tests may fail if they relied on the env var — fix in Task 2
golangci-lint run ./...
```

**Step 5: Commit**

```bash
git add cmd/specrun/main.go
git commit -m "refactor: remove SPECRUN_NO_SERVICES env var and port fallback"
```

---

### Task 2: Update tests

**Files:**
- Modify: `cmd/specrun/main_test.go`

**Step 1: Remove SPECRUN_NO_SERVICES from test env**

Find `SPECRUN_NO_SERVICES=1` in the subprocess environment for `TestSelfVerification_Parse`. Remove it.

**Step 2: Handle Docker dependency in self-verification test**

`TestSelfVerification_Parse` runs the full self-verification which includes `specs/speclang.spec` (declares services). This requires Docker.

Add a Docker availability check:
```go
func skipIfNoDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available, skipping self-verification")
	}
}
```

Call it at the top of `TestSelfVerification_Parse`.

Also remove the manual server startup helpers from the test if they were only there for the `SPECRUN_NO_SERVICES` case. Check: `startFixedPortServer`, or any `httptest.Server` started on fixed ports (8080, 8081, 8082) for self-verification. Those were the workaround — with Docker managing containers, they're not needed.

BUT: other tests like `TestVerify_JSON` and `TestVerify_HumanOutput` use `startTransferServer(t)` which returns an `httptest.Server` with a random port. These are fine — they don't use `service()`, they pass `APP_URL` directly.

**Step 3: Verify**

```bash
go test ./... -count=1    # all pass (self-verification skips if no Docker)
golangci-lint run ./...
```

**Step 4: Commit**

```bash
git add cmd/specrun/main_test.go
git commit -m "test: remove SPECRUN_NO_SERVICES, skip self-verification without Docker"
```

---

### Task 3: Simplify CI

**Files:**
- Modify: `.github/workflows/ci.yml`

**Step 1: Remove manual server startup steps**

Remove:
- "Start example server" (`go run ./examples/server &`)
- "Start broken server" (`go run ./testdata/self/broken_server &`)
- "Start HTTP test server" (`go run ./testdata/self/http_server &`)
- "Start services test server" (`PORT=9090 go run ./testdata/self/http_server &`)

**Step 2: Simplify self-verification step**

```yaml
- name: Self-verification
  env:
    SPECRUN_BIN: ./specrun
    ECHO_TOOL_BIN: ./echo_tool
  run: ./specrun verify specs/speclang.spec
```

Remove `SPECRUN_NO_SERVICES`, `APP_URL`, `BROKEN_APP_URL`, `HTTP_TEST_URL`.

GitHub Actions `ubuntu-latest` has Docker pre-installed. specrun will build and start containers from the Dockerfiles in the repo.

**Step 3: Verify CI works**

Push to a branch and check CI. The self-verification step should:
1. Build container images from Dockerfiles
2. Start containers on declared ports
3. Run verification
4. Stop containers

**Step 4: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: let specrun manage containers, remove manual server startup"
```

---

### Task 4: Improve port collision error message

**Files:**
- Modify: `pkg/infra/docker.go`

**Step 1: Check Docker's error on port collision**

When Docker can't bind a port (already in use), it returns an error like `"Bind for 0.0.0.0:8080 failed: port is already allocated"`. Find where container start errors are surfaced in `docker.go` and wrap them with a clearer message:

```go
return nil, fmt.Errorf("starting service %q: %w — is port %d already in use?", def.Name, err, def.Port)
```

**Step 2: Verify**

Start something on port 8080, then try to start a service on the same port. The error should be clear.

**Step 3: Commit**

```bash
git add pkg/infra/docker.go
git commit -m "fix(infra): improve port collision error message"
```

---

### Task 5: Update documentation

**Files:**
- Modify: `CLAUDE.md` — remove all SPECRUN_NO_SERVICES references
- Modify: `docs/services.md` — remove SPECRUN_NO_SERVICES section, update CI example
- Modify: `docs/self-verification.md` — update to reflect Docker-managed containers
- Modify: `skills/verify/SKILL.md` — remove SPECRUN_NO_SERVICES reference
- Modify: `docs/language-reference.md` — remove if referenced

**Step 1: Search and remove all references**

```bash
grep -rn "SPECRUN_NO_SERVICES" --include="*.md" --include="*.go" --include="*.yml" --include="*.spec"
```

Remove every occurrence. Replace CI documentation examples with the simplified version.

**Step 2: Commit**

```bash
git add -A
git commit -m "docs: remove all SPECRUN_NO_SERVICES references"
```

---

### Task 6: Update specs if needed

**Files:**
- Check: `specs/*.spec` — verify no spec references `SPECRUN_NO_SERVICES`

**Step 1: Search**

```bash
grep -rn "NO_SERVICES\|no-services\|no_services" specs/ testdata/
```

If anything is found, remove it. This should already be clean from #71 but verify.

**Step 2: Commit if needed**

---

### Task 7: Final verification and PR

**Step 1: Full checks**

```bash
golangci-lint run ./...    # zero issues
go test ./... -count=1     # all pass
```

**Step 2: Create PR**

```bash
git push -u origin <branch>
gh pr create --title "chore: remove SPECRUN_NO_SERVICES, let specrun manage containers everywhere (#73)" --body "..."
gh pr checks <number> --watch
gh pr merge <number> --squash --delete-branch
```
