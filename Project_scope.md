# ReqRes — API Testing CLI Tool

## Vision
Make AI-assisted ("vibe coding") development safer. When a developer (or AI agent) changes backend code, running `reqres run tests.yaml` in one command verifies the change **and** regression-tests everything else. The YAML is concise enough that an LLM can update it cheaply (low token cost) and the tool gives instant pass/fail feedback.

## Design Philosophy
- **Smart defaults** — GET, status 200, JSON content-type are assumed. Only specify what differs.
- **Flat assertions** — `$.id: 1` not `{path: "$.id", operator: eq, value: 1}`.
- **One file per service** — microservices get separate YAML files run in parallel.
- **Tags** — `smoke`, `regression`, `negative`, feature names. Run subsets with `--tags`.
- **Token-efficient** — every key, every line costs tokens when an AI reads/writes the file.

## YAML Schema (quick ref)
| Key | Required | Default | Purpose |
|-----|----------|---------|---------|
| `base` | yes | — | Base URL for the service |
| `timeout` | no | 5000 | Request timeout in ms |
| `retries` | no | 0 | Retry count on failure |
| `vars` | no | — | Reusable variables (`${name}`) |
| `defaults.headers` | no | — | Headers applied to every request |
| `envs` | no | — | Per-environment overrides |
| `load` | no | — | Stress test config (v2) |
| **Per test** | | | |
| `name` | yes | — | Human-readable label |
| `method` | no | GET | HTTP method |
| `path` | yes | — | URL path (appended to `base`) |
| `headers` | no | — | Extra/override headers |
| `query` | no | — | Query parameters |
| `body` | no | — | Request body (auto JSON) |
| `auth` | no | — | `bearer ${token}` or `basic user:pass` |
| `tags` | no | — | Labels for selective runs |
| `check` | no | 200 | Status int **or** map of JSONPath assertions |
| `capture` | no | — | Extract response values for chaining |
| `after` | no | — | Run after named test (dependency) |

## Core Features (v1)
- CLI: `reqres run <file>`, `reqres validate <file>`
- All HTTP methods
- Variable substitution (`${var}`)
- `defaults` block (shared headers/auth)
- Tag-based filtering (`--tags smoke`)
- Parallel test execution (`--parallel N`)
- Configurable retries & timeouts
- Auth: Bearer, Basic
- Assertion engine: status, headers, JSONPath body checks
- Chained tests with `capture` / `after`
- Colored console output
- Exit codes for CI/CD (0 = pass, 1 = fail)

## CLI Interface
```
reqres run tests.yaml
reqres run tests.yaml --tags smoke
reqres run tests.yaml --env staging
reqres run tests.yaml --parallel 8
reqres run users.yaml orders.yaml --parallel   # multi-service
reqres validate tests.yaml                      # schema check only
```

## Non-Goals (v1)
- Web UI
- Persistent database
- Test recording / HAR import
- GraphQL support

## Future Roadmap (v2+)
- Stress / load testing (`load:` block)
- HTML & JSON reports
- GitHub Actions integration
- AI agent integration (auto-generate YAML from OpenAPI spec)
- Flaky test detection
- Mock server
- Response snapshot diffing
