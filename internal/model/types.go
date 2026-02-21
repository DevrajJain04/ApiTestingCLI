package model

import "time"

type Config struct {
	Base     string
	Timeout  int
	Retries  int
	Vars     map[string]any
	Defaults Defaults
	Envs     map[string]EnvOverride
	Load     *LoadConfig
	Mock     *MockConfig
	Tests    []TestCase
}

type Defaults struct {
	Headers map[string]string
	Auth    string
}

type EnvOverride struct {
	Base     string
	Timeout  *int
	Retries  *int
	Vars     map[string]any
	Defaults *Defaults
}

type TestCase struct {
	Name      string
	Method    string
	Path      string
	Headers   map[string]string
	Query     map[string]any
	Body      any
	Auth      string
	Tags      []string
	Check     any
	Capture   map[string]string
	After     string
	Snapshot  any
	Mock      *MockRoute
	Retries   *int
	TimeoutMS *int
}

type LoadConfig struct {
	Users    int
	Duration string
	RampUp   string
	Method   string
	Path     string
	Query    map[string]any
	Headers  map[string]string
	Body     any
	Check    any
	Tags     []string
}

type MockConfig struct {
	Routes []MockRoute
	Delay  string
}

type MockRoute struct {
	Name    string
	Method  string
	Path    string
	Status  int
	Headers map[string]string
	Body    any
	Query   map[string]any
	Delay   string
}

type RunOptions struct {
	Env             string
	Tags            []string
	Parallel        int
	ReportJSONPath  string
	ReportHTMLPath  string
	GitHubActions   bool
	DetectFlakyRuns int
	UpdateSnapshots bool
	RunLoad         bool
}

type RunReport struct {
	StartedAt      time.Time      `json:"started_at"`
	FinishedAt     time.Time      `json:"finished_at"`
	DurationMS     int64          `json:"duration_ms"`
	Total          int            `json:"total"`
	Passed         int            `json:"passed"`
	Failed         int            `json:"failed"`
	Skipped        int            `json:"skipped"`
	Flaky          []string       `json:"flaky,omitempty"`
	Files          []FileReport   `json:"files"`
	Load           *LoadSummary   `json:"load,omitempty"`
	Failures       []FailureEntry `json:"failures,omitempty"`
	GeneratedBy    string         `json:"generated_by"`
	SnapshotsSaved int            `json:"snapshots_saved,omitempty"`
}

type FileReport struct {
	File     string       `json:"file"`
	Total    int          `json:"total"`
	Passed   int          `json:"passed"`
	Failed   int          `json:"failed"`
	Skipped  int          `json:"skipped"`
	Duration int64        `json:"duration_ms"`
	Tests    []TestResult `json:"tests"`
}

type TestStatus string

const (
	StatusPass    TestStatus = "pass"
	StatusFail    TestStatus = "fail"
	StatusSkip    TestStatus = "skip"
	StatusFlaky   TestStatus = "flaky"
	StatusUnknown TestStatus = "unknown"
)

type TestResult struct {
	Name       string            `json:"name"`
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Status     TestStatus        `json:"status"`
	Message    string            `json:"message,omitempty"`
	DurationMS int64             `json:"duration_ms"`
	Attempts   int               `json:"attempts"`
	StatusCode int               `json:"status_code,omitempty"`
	Captures   map[string]string `json:"captures,omitempty"`
}

type FailureEntry struct {
	File string `json:"file"`
	Test string `json:"test"`
	Why  string `json:"why"`
}

type LoadSummary struct {
	Method     string  `json:"method"`
	Path       string  `json:"path"`
	Users      int     `json:"users"`
	Requests   int64   `json:"requests"`
	Successes  int64   `json:"successes"`
	Failures   int64   `json:"failures"`
	AvgMS      float64 `json:"avg_ms"`
	P95MS      float64 `json:"p95_ms"`
	MinMS      float64 `json:"min_ms"`
	MaxMS      float64 `json:"max_ms"`
	DurationMS int64   `json:"duration_ms"`
}
