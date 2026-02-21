# ReqRes - Complete How To Use Guide

## 1. What This Tool Is

ReqRes is a CLI test runner for backend APIs.
You define tests in YAML, then run:

```bash
reqres run tests.yaml
```

It executes HTTP requests, validates responses, prints pass/fail summary, and returns CI-friendly exit codes:

- `0` = all pass
- `1` = one or more fail (or flaky marked as fail)

## 2. Build and Run

From project root:

```bash
go build -o reqres .
```

Or run without building:

```bash
go run . help
```

## 3. CLI Commands

### 3.1 Run tests

```bash
reqres run <file...> [flags]
```

Examples:

```bash
reqres run tests.yaml
reqres run tests.yaml --tags smoke
reqres run tests.yaml --env staging
reqres run tests.yaml --parallel 8
reqres run users.yaml orders.yaml --parallel
reqres run tests.yaml --report-json reports/result.json --report-html reports/result.html
reqres run tests.yaml --detect-flaky 3
reqres run tests.yaml --update-snapshots
reqres run tests.yaml --github-actions
reqres run tests.yaml --no-load
```

Flags:

- `--tags` comma-separated filter (`smoke,regression`)
- `--env` apply `envs.<name>` override
- `--parallel` worker count (bare `--parallel` also works and uses CPU count)
- `--report-json` write JSON report
- `--report-html` write HTML report
- `--detect-flaky` rerun suite N times to detect pass/fail oscillation
- `--update-snapshots` rewrite snapshot baselines
- `--github-actions` emit GHA error annotations on failures
- `--no-load` skip `load:` block execution

### 3.2 Validate config only

```bash
reqres validate <file...> [--env staging]
```

No network calls. Only parse + schema validation.

### 3.3 Start mock server

```bash
reqres mock <file> [--port 8080] [--env staging]
```

Serves responses from `mock.routes` and/or per-test `mock` blocks.

### 3.4 Generate tests from OpenAPI

```bash
reqres generate openapi.json -o tests.yaml
```

Accepts JSON or YAML OpenAPI input.

### 3.5 Generate GitHub Actions workflow

```bash
reqres gha-init
reqres gha-init .github/workflows/reqres-custom.yml
```

## 4. YAML Structure

## 4.1 Top-level keys

- `base` required base URL
- `timeout` default request timeout in ms (default `5000`)
- `retries` default retries (default `0`)
- `vars` reusable variables (`${token}`)
- `defaults.headers` shared headers
- `defaults.auth` shared auth string
- `envs` environment overrides
- `load` optional load test config
- `mock` optional mock server config
- `tests` test list

## 4.2 Test keys

- `name` required
- `path` required
- `method` default `GET`
- `headers`, `query`, `body`
- `auth` (`bearer ...` / `basic user:pass`)
- `tags` list or comma string
- `check` status or assertion map
- `capture` response value extraction
- `after` dependency by test name
- `retries`, `timeout` per-test override
- `snapshot` enable snapshot diffing
- `mock` per-test mock route override

## 4.3 Example file

```yaml
base: https://jsonplaceholder.typicode.com
timeout: 5000
retries: 1

vars:
  token: demo-token

defaults:
  headers:
    X-Client: reqres
  auth: bearer ${token}

envs:
  staging:
    base: https://staging.example.com
    vars:
      token: staging-token

load:
  users: 10
  duration: 20s
  ramp_up: 5s
  method: GET
  path: /posts
  tags: [regression]

mock:
  delay: 50ms
  routes:
    - method: GET
      path: /health
      status: 200
      body: { ok: true }

tests:
  - name: List posts
    path: /posts
    query: { _limit: 3 }
    tags: [smoke, regression]
    check:
      $: "len >= 1"
      $[0].id: exists

  - name: Create post
    method: POST
    path: /posts
    body: { title: "ReqRes", body: "test", userId: 1 }
    check:
      status: 201
      $.id: exists
    capture:
      new_post_id: $.id

  - name: Get created post
    path: /posts/${new_post_id}
    after: Create post
    snapshot: true
    check: 200
```

## 5. Assertions

### 5.1 Status shorthand

```yaml
check: 404
```

### 5.2 Rich checks

```yaml
check:
  status: 200
  headers:
    Content-Type: /json/
  $.id: 1
  $.title: "!empty"
  $.userId: exists
  $: "len >= 1"
```

Supported expectation styles:

- exact equality (`$.id: 1`)
- existence (`exists`)
- non-empty (`!empty`)
- regex (`"/^Jo/"`)
- length expression on current value (`"len >= 1"`)

Also supported:

```yaml
check:
  body:
    - path: $.id
      value: 1
```

## 6. Chaining and Dependencies

Use `capture` + `${var}` + `after` to chain tests:

```yaml
- name: Create user
  method: POST
  path: /users
  body: { name: Alice }
  capture:
    user_id: $.id
  check: 201

- name: Fetch user
  path: /users/${user_id}
  after: Create user
  check: 200
```

If dependency fails, dependent test is skipped.
If dependency graph has a cycle, tests in the cycle fail with cycle message.

## 7. Networking and HTTP Behavior

ReqRes does **not** shell out to `curl`.
It sends HTTP requests directly using Go `net/http` client.

Current request behavior:

- URL = `base + path` unless path is absolute URL
- query params appended from `query`
- body auto-JSON encoded for map/list objects
- `Accept: application/json` default
- `Content-Type: application/json` auto-set when body exists
- auth parsing:
  - `bearer <token>` -> `Authorization: Bearer <token>`
  - `basic user:pass` -> base64 `Authorization: Basic ...`
- timeout uses context deadline (`timeout` ms)
- retries re-run failed request/assertion up to configured count

## 8. Load Testing (`load:`)

If `load` exists and you do not pass `--no-load`, ReqRes runs load phase after test phase for that file.

Config:

- `users` concurrent workers
- `duration` total run time (Go duration, e.g. `30s`)
- `ramp_up` optional spread start time
- request fields: `method`, `path`, `query`, `headers`, `body`, `check`, `tags`

Output includes requests count, success/fail count, avg/p95/min/max latency.

## 9. Snapshot Diffing

Per test:

```yaml
snapshot: true
```

or custom snapshot name:

```yaml
snapshot: user-list-baseline
```

Snapshots are stored under `.reqres_snapshots/<suite>/<name>.json`.

- First run creates snapshot if missing.
- Next runs compare response vs stored file.
- Mismatch fails test.
- `--update-snapshots` refreshes baseline.

## 10. Mock Server

Run:

```bash
reqres mock tests.yaml --port 8080
```

Matching uses method + path (+ optional query checks).
Response supports status, headers, body, and optional delay.

You can define routes either:

- globally under `mock.routes`
- or per test with `test.mock`

## 11. Reports and CI

### 11.1 Reports

```bash
reqres run tests.yaml --report-json reports/result.json --report-html reports/result.html
```

### 11.2 Flaky detection

```bash
reqres run tests.yaml --detect-flaky 5
```

If same test passes in one round and fails in another, it is marked flaky and contributes to failure exit code.

### 11.3 GitHub Actions

```bash
reqres run tests.yaml --github-actions
```

Prints `::error ...` annotations for failed tests.

Generate baseline workflow:

```bash
reqres gha-init
```

## 12. OpenAPI-Assisted Test Generation

Generate starter YAML from OpenAPI:

```bash
reqres generate openapi.yaml -o api.tests.yaml
```

What it generates:

- `base` from first OpenAPI `servers[0].url` (or fallback)
- one test per HTTP operation in `paths`
- test name from `summary`, else `operationId`, else `METHOD /path`
- expected status prefers first `2xx` response

This is a starter file. Add richer assertions/capture/tags manually or via AI.

## 13. Repetition Minimization Patterns (for Devs + AI Agents)

Use these patterns to keep YAML short and maintainable:

- Put shared auth/headers in `defaults`
- Keep environment-specific values in `envs`
- Store secrets/tokens/user IDs in `vars`
- Reuse dynamic IDs with `capture` + `${var}`
- Use tag slices to run only impacted test groups
- Keep one YAML per microservice for parallel runs
- Use OpenAPI generator for initial scaffold
- Use snapshots for large response bodies instead of writing many field assertions

These choices reduce token usage and repeated edits for AI coding agents.

## 14. YAML Parser Notes

This project uses an internal YAML parser (`internal/yamlmini`) to avoid external dependencies.
It supports the practical subset used by ReqRes configs:

- nested maps/lists
- inline maps/lists (`{}` / `[]`)
- quoted and unquoted scalars

Avoid advanced YAML features (anchors, aliases, complex multiline scalars) for best compatibility.

## 15. Troubleshooting

- `missing vars: ...`  
  Add missing key in `vars` or `capture` it in prior test.

- `dependency "..." not selected`  
  Your tag filter excluded the dependency test.

- `snapshot mismatch`  
  Response changed. Inspect API change or run with `--update-snapshots`.

- `env "staging" not found`  
  Ensure exact key exists under `envs`.

- Parsing error with line number  
  Fix YAML indentation/format near reported line.

## 16. Suggested Team Workflow

1. Keep `tests.yaml` per service.
2. On backend changes, run `reqres validate` then `reqres run --tags smoke`.
3. In CI, run full regression with reports.
4. Add flaky detection for unstable suites.
5. Periodically regenerate/update tests from OpenAPI and enrich assertions.


