# ReqRes

ReqRes is a YAML-driven API testing CLI for backend teams and AI coding agents.
Run one command to verify new changes and catch regressions fast.

## Quickstart (60 seconds)

1. Build:

```bash
go build -o reqres .
```

2. Validate your test file:

```bash
./reqres validate tests.yaml
```

3. Run smoke tests:

```bash
./reqres run tests.yaml --tags smoke
```

4. Run full suite with reports:

```bash
./reqres run tests.yaml --report-json reports/result.json --report-html reports/result.html
```

## Common Commands

```bash
./reqres run tests.yaml
./reqres run tests.yaml --env staging --parallel 8
./reqres run users.yaml orders.yaml --parallel
./reqres run tests.yaml --detect-flaky 3
./reqres run tests.yaml --update-snapshots
./reqres mock tests.yaml --port 8080
./reqres generate openapi.json -o generated-tests.yaml
./reqres gha-init
```

## Why it reduces repetition

- Shared defaults (`defaults`) remove repeated headers/auth per test.
- Variables (`vars` + `${name}`) remove copy-paste values.
- Capture chaining (`capture` + `after`) reuses dynamic IDs across tests.
- Tags (`smoke`, `regression`) avoid re-running everything every time.
- OpenAPI generation creates a starter suite quickly for humans and AI agents.

## Full Documentation

See the complete guide: `HOW_TO_USE.md`

