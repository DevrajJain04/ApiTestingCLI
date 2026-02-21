package runner

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/DevrajJain04/reqres/internal/assertion"
	"github.com/DevrajJain04/reqres/internal/httpx"
	"github.com/DevrajJain04/reqres/internal/model"
	"github.com/DevrajJain04/reqres/internal/snapshot"
	"github.com/DevrajJain04/reqres/internal/utils"
)

type FileRunOptions struct {
	FilePath        string
	Config          model.Config
	RunOptions      model.RunOptions
	SnapshotManager *snapshot.Manager
}

func RunFile(opts FileRunOptions) (model.FileReport, int) {
	started := time.Now()
	cfg := opts.Config
	runOpts := opts.RunOptions

	tests := filterByTags(cfg.Tests, runOpts.Tags)
	report := model.FileReport{
		File:  opts.FilePath,
		Tests: []model.TestResult{},
	}
	if len(tests) == 0 {
		report.Duration = time.Since(started).Milliseconds()
		return report, 0
	}

	testOrder := make([]string, 0, len(tests))
	testByName := map[string]model.TestCase{}
	unresolved := map[string]bool{}
	for _, test := range tests {
		testOrder = append(testOrder, test.Name)
		testByName[test.Name] = test
		unresolved[test.Name] = true
	}

	varsMu := sync.RWMutex{}
	vars := map[string]any{}
	for k, v := range cfg.Vars {
		vars[k] = v
	}

	resultsByName := map[string]model.TestResult{}
	snapshotsSaved := 0

	for len(unresolved) > 0 {
		// Resolve a ready batch where all dependency links (`after`) are satisfied.
		ready := []model.TestCase{}
		progress := false

		for _, name := range testOrder {
			if !unresolved[name] {
				continue
			}
			test := testByName[name]
			if strings.TrimSpace(test.After) == "" {
				ready = append(ready, test)
				continue
			}

			depResult, ok := resultsByName[test.After]
			if !ok {
				if _, depSelected := unresolved[test.After]; depSelected {
					continue
				}
				resultsByName[test.Name] = model.TestResult{
					Name:    test.Name,
					Method:  effectiveMethod(test.Method),
					Path:    test.Path,
					Status:  model.StatusSkip,
					Message: fmt.Sprintf("dependency %q is not selected in this run", test.After),
				}
				delete(unresolved, test.Name)
				progress = true
				continue
			}

			if depResult.Status == model.StatusPass {
				ready = append(ready, test)
			} else {
				resultsByName[test.Name] = model.TestResult{
					Name:    test.Name,
					Method:  effectiveMethod(test.Method),
					Path:    test.Path,
					Status:  model.StatusSkip,
					Message: fmt.Sprintf("dependency %q did not pass", test.After),
				}
				delete(unresolved, test.Name)
				progress = true
			}
		}

		if len(ready) == 0 {
			if !progress {
				for _, name := range testOrder {
					if !unresolved[name] {
						continue
					}
					test := testByName[name]
					resultsByName[test.Name] = model.TestResult{
						Name:    test.Name,
						Method:  effectiveMethod(test.Method),
						Path:    test.Path,
						Status:  model.StatusFail,
						Message: "dependency cycle detected",
					}
					delete(unresolved, name)
				}
			}
			continue
		}

		batchResults, saved := runBatch(ready, max(1, runOpts.Parallel), func(test model.TestCase) model.TestResult {
			return executeTest(test, opts.FilePath, cfg, runOpts, &varsMu, vars, opts.SnapshotManager)
		})
		snapshotsSaved += saved
		for _, result := range batchResults {
			resultsByName[result.Name] = result
			delete(unresolved, result.Name)
		}
	}

	for _, name := range testOrder {
		result := resultsByName[name]
		report.Tests = append(report.Tests, result)
		switch result.Status {
		case model.StatusPass:
			report.Passed++
		case model.StatusFail:
			report.Failed++
		case model.StatusSkip:
			report.Skipped++
		}
	}
	report.Total = len(report.Tests)
	report.Duration = time.Since(started).Milliseconds()
	return report, snapshotsSaved
}

func executeTest(
	test model.TestCase,
	filePath string,
	cfg model.Config,
	runOpts model.RunOptions,
	varsMu *sync.RWMutex,
	vars map[string]any,
	snapshots *snapshot.Manager,
) model.TestResult {
	started := time.Now()
	result := model.TestResult{
		Name:   test.Name,
		Method: effectiveMethod(test.Method),
		Path:   test.Path,
		Status: model.StatusFail,
	}

	mergedHeaders := mergeHeaders(cfg.Defaults.Headers, test.Headers)
	auth := strings.TrimSpace(test.Auth)
	if auth == "" {
		auth = cfg.Defaults.Auth
	}

	timeoutMS := cfg.Timeout
	if test.TimeoutMS != nil {
		timeoutMS = *test.TimeoutMS
	}
	retries := cfg.Retries
	if test.Retries != nil {
		retries = *test.Retries
	}

	varsSnapshot := map[string]any{}
	varsMu.RLock()
	for k, v := range vars {
		varsSnapshot[k] = v
	}
	varsMu.RUnlock()

	path, err := utils.ExpandString(test.Path, varsSnapshot)
	if err != nil {
		result.Message = err.Error()
		result.DurationMS = time.Since(started).Milliseconds()
		return result
	}
	result.Path = path

	auth, err = utils.ExpandString(auth, varsSnapshot)
	if err != nil {
		result.Message = err.Error()
		result.DurationMS = time.Since(started).Milliseconds()
		return result
	}

	expandedHeaders := map[string]string{}
	for key, value := range mergedHeaders {
		expanded, err := utils.ExpandString(value, varsSnapshot)
		if err != nil {
			result.Message = err.Error()
			result.DurationMS = time.Since(started).Milliseconds()
			return result
		}
		expandedHeaders[key] = expanded
	}

	queryAny, err := utils.ExpandAny(test.Query, varsSnapshot)
	if err != nil {
		result.Message = err.Error()
		result.DurationMS = time.Since(started).Milliseconds()
		return result
	}
	query := utils.ToStringMap(queryAny)

	bodyAny, err := utils.ExpandAny(test.Body, varsSnapshot)
	if err != nil {
		result.Message = err.Error()
		result.DurationMS = time.Since(started).Milliseconds()
		return result
	}

	expandedCheck, err := utils.ExpandAny(test.Check, varsSnapshot)
	if err != nil {
		result.Message = err.Error()
		result.DurationMS = time.Since(started).Milliseconds()
		return result
	}

	url := joinURL(cfg.Base, path)
	attempts := 0
	var lastErr error
	var lastResp httpx.Response
	// Retry wraps both transport and assertion failures so flaky network/status paths can recover.
	for attempt := 0; attempt <= max(0, retries); attempt++ {
		attempts++
		resp, reqErr := httpx.Do(httpx.RequestOptions{
			Method:  result.Method,
			URL:     url,
			Headers: expandedHeaders,
			Query:   query,
			Body:    bodyAny,
			Auth:    auth,
			Timeout: time.Duration(timeoutMS) * time.Millisecond,
		})
		lastResp = resp
		if reqErr != nil {
			lastErr = reqErr
			continue
		}
		if assertErr := assertion.Evaluate(expandedCheck, resp.StatusCode, resp.Headers, resp.BodyJSON); assertErr != nil {
			lastErr = assertErr
			continue
		}
		lastErr = nil
		break
	}

	result.Attempts = attempts
	result.StatusCode = lastResp.StatusCode

	if lastErr != nil {
		result.Status = model.StatusFail
		result.Message = lastErr.Error()
		result.DurationMS = time.Since(started).Milliseconds()
		return result
	}

	if len(test.Capture) > 0 {
		result.Captures = map[string]string{}
		for key, pathExpr := range test.Capture {
			value, found, err := assertion.Extract(pathExpr, lastResp.BodyJSON)
			if err != nil {
				result.Status = model.StatusFail
				result.Message = fmt.Sprintf("capture %s: %v", key, err)
				result.DurationMS = time.Since(started).Milliseconds()
				return result
			}
			if !found {
				result.Status = model.StatusFail
				result.Message = fmt.Sprintf("capture %s path not found: %s", key, pathExpr)
				result.DurationMS = time.Since(started).Milliseconds()
				return result
			}

			varsMu.Lock()
			vars[key] = value
			varsMu.Unlock()
			result.Captures[key] = utils.ToString(value)
		}
	}

	if snapshots != nil {
		if _, err := snapshots.Evaluate(filePath, test.Name, test.Snapshot, lastResp.BodyJSON, runOpts.UpdateSnapshots); err != nil {
			result.Status = model.StatusFail
			result.Message = err.Error()
			result.DurationMS = time.Since(started).Milliseconds()
			return result
		}
	}

	result.Status = model.StatusPass
	result.Message = "ok"
	result.DurationMS = time.Since(started).Milliseconds()
	return result
}

func runBatch(tests []model.TestCase, parallel int, run func(model.TestCase) model.TestResult) ([]model.TestResult, int) {
	if parallel <= 1 || len(tests) <= 1 {
		out := make([]model.TestResult, 0, len(tests))
		for _, test := range tests {
			out = append(out, run(test))
		}
		return out, 0
	}

	type item struct {
		index  int
		result model.TestResult
	}
	in := make(chan struct {
		index int
		test  model.TestCase
	})
	out := make(chan item, len(tests))
	var wg sync.WaitGroup
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range in {
				out <- item{
					index:  task.index,
					result: run(task.test),
				}
			}
		}()
	}

	go func() {
		for i, test := range tests {
			in <- struct {
				index int
				test  model.TestCase
			}{
				index: i,
				test:  test,
			}
		}
		close(in)
		wg.Wait()
		close(out)
	}()

	ordered := make([]model.TestResult, len(tests))
	for item := range out {
		ordered[item.index] = item.result
	}
	return ordered, 0
}

func filterByTags(tests []model.TestCase, selected []string) []model.TestCase {
	if len(selected) == 0 {
		return tests
	}
	need := map[string]struct{}{}
	for _, tag := range selected {
		trimmed := strings.TrimSpace(tag)
		if trimmed != "" {
			need[trimmed] = struct{}{}
		}
	}
	if len(need) == 0 {
		return tests
	}
	out := []model.TestCase{}
	for _, test := range tests {
		if len(test.Tags) == 0 {
			continue
		}
		matched := false
		for _, tag := range test.Tags {
			if _, ok := need[tag]; ok {
				matched = true
				break
			}
		}
		if matched {
			out = append(out, test)
		}
	}
	return out
}

func mergeHeaders(a, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func joinURL(base string, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func effectiveMethod(method string) string {
	if strings.TrimSpace(method) == "" {
		return "GET"
	}
	return strings.ToUpper(strings.TrimSpace(method))
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
