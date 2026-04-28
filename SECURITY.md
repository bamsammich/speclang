# Security Policy

## Threat Model

speclang is a runtime that executes code described in `.spec` files. A spec file is **not** configuration — it is executable input. When you run `specrun verify foo.spec`, the tool may, depending on what the spec declares:

- Execute arbitrary subprocesses (`process { command: ... }`)
- Build and run Docker containers (`services { build: "..." }`)
- Drive a `docker-compose` stack against any compose file the spec points at
- Mount host paths into containers (`volumes { ... }`)
- Issue arbitrary HTTP requests (`http.post(url, ...)`)
- Read environment variables (`env(...)`)
- Drive a real browser (`playwright` adapter)
- Read any file the invoking user can read (via `include` or `import`)

These are intentional capabilities. They make speclang useful for verifying real systems. They also mean **you must treat a spec file the way you would treat a shell script**: do not run `specrun` against specs you did not author or have not reviewed.

This is especially important in LLM-driven workflows (Claude Code and other agents): an agent that reads a spec from a GitHub issue, PR comment, gist, or shared directory and runs `specrun verify` is executing attacker-controlled code if that spec is attacker-controlled. Treat spec ingestion the same as `curl | bash`.

## What speclang DOES guard against

- No shell interpolation in process or Docker command construction — args are always passed as arrays, never as a single shell-evaluated string.
- No SQL, no template engines, no `eval` of user strings beyond the documented expression evaluator (which has no escape hatches to host execution).
- OpenAPI imports reject external `$ref` values (no SSRF via spec import).
- All structured output is JSON-encoded; no string concatenation into JSON output.

## What speclang does NOT guard against

- A trusted spec author who writes a hostile spec.
- File reads via `include` or `import` — a spec can include any file the running user can read. There is no path sandbox; this is intentional, since path containment would not reduce attacker capability (the same spec could simply use `process.exec("cat", "...")`) and would break legitimate cross-directory reuse (monorepos, shared libraries at a known path).
- Docker images, containers, and volume mounts the spec chooses to start.
- DNS or HTTP exfiltration via spec-chosen URLs or environment-variable interpolation.
- Browser-driven exposure when the playwright adapter visits attacker-controlled pages.

## Reporting a Vulnerability

If you discover a security issue in speclang itself (not in a spec or in a system being tested), please email travis@huddlehaus.com or open a private security advisory at https://github.com/bamsammich/speclang/security/advisories/new. Please do not open a public issue for security vulnerabilities.

We aim to acknowledge reports within five business days.
