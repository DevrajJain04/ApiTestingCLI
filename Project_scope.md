# API Testing CLI Tool

## Overview
A command-line tool for executing automated API tests defined using YAML configuration files. Designed for fast feedback in modern software development workflows.

## Core Features (v1)
- CLI interface for running API test suites
- YAML-based test definitions
- Support for all HTTP methods
- Environment variable substitution
- Parallel test execution
- Configurable retries and timeouts
- Authentication (Basic, Bearer)
- Assertion engine (status, headers, JSON body)
- Colored console output
- Exit codes for CI/CD usage

## CLI Interface
- apitest run <file>
- apitest validate <file>
- apitest run <file> --parallel=N
- apitest run <file> --env=staging

## Non-Goals (v1)
- Web UI
- AI integrations
- Persistent database
- Test recording
- GraphQL support

## Future Roadmap (v2+)
- HTML reports
- Mock server
- GitHub Actions integration
- AI agent integration
- Flaky test detection
