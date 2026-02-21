package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DevrajJain04/reqres/internal/config"
	"github.com/DevrajJain04/reqres/internal/gha"
	"github.com/DevrajJain04/reqres/internal/loadtest"
	"github.com/DevrajJain04/reqres/internal/mockserver"
	"github.com/DevrajJain04/reqres/internal/model"
	"github.com/DevrajJain04/reqres/internal/openapi"
	"github.com/DevrajJain04/reqres/internal/report"
	"github.com/DevrajJain04/reqres/internal/runner"
	"github.com/DevrajJain04/reqres/internal/snapshot"
	"github.com/DevrajJain04/reqres/internal/utils"
)

func Run(args []string) int {
	if len(args) == 0 {
		printUsage()
		return 1
	}
	switch args[0] {
	case "run":
		return runCommand(args[1:])
	case "validate":
		return validateCommand(args[1:])
	case "mock":
		return mockCommand(args[1:])
	case "generate":
		return generateCommand(args[1:])
	case "gha-init":
		return ghaInitCommand(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		printUsage()
		return 1
	}
}

func runCommand(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	tagsRaw := fs.String("tags", "", "comma-separated tags to include")
	env := fs.String("env", "", "environment override name")
	parallel := fs.Int("parallel", max(1, runtime.NumCPU()), "parallel workers")
	reportJSON := fs.String("report-json", "", "write JSON report to this path")
	reportHTML := fs.String("report-html", "", "write HTML report to this path")
	ghaFlag := fs.Bool("github-actions", false, "emit GitHub Actions annotations")
	flakyRuns := fs.Int("detect-flaky", 1, "rerun suites to detect flaky tests")
	updateSnapshots := fs.Bool("update-snapshots", false, "rewrite snapshot baselines")
	noLoad := fs.Bool("no-load", false, "skip load block execution")

	normalizedArgs := reorderArgs(args, map[string]bool{
		"--tags":             true,
		"--env":              true,
		"--parallel":         true,
		"--report-json":      true,
		"--report-html":      true,
		"--detect-flaky":     true,
		"--github-actions":   false,
		"--update-snapshots": false,
		"--no-load":          false,
	})
	normalizedArgs = fillDefaultForBareFlag(normalizedArgs, "--parallel", strconv.Itoa(max(1, runtime.NumCPU())))
	if err := fs.Parse(normalizedArgs); err != nil {
		return 1
	}
	files := fs.Args()
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "run requires at least one yaml file")
		return 1
	}

	opts := model.RunOptions{
		Env:             strings.TrimSpace(*env),
		Tags:            parseCSV(*tagsRaw),
		Parallel:        max(1, *parallel),
		ReportJSONPath:  strings.TrimSpace(*reportJSON),
		ReportHTMLPath:  strings.TrimSpace(*reportHTML),
		GitHubActions:   gha.Enabled(*ghaFlag),
		DetectFlakyRuns: max(1, *flakyRuns),
		UpdateSnapshots: *updateSnapshots,
		RunLoad:         !*noLoad,
	}

	reportData, err := runFiles(files, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, utils.Red("Error: "+err.Error()))
		return 1
	}

	if opts.GitHubActions {
		for _, failure := range reportData.Failures {
			fmt.Println(gha.FailureAnnotation(failure.File, failure.Test, failure.Why))
		}
	}

	if opts.ReportJSONPath != "" {
		path := config.ResolveOutputPath(files[0], opts.ReportJSONPath)
		if err := report.WriteJSON(path, reportData); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write JSON report: %v\n", err)
		}
	}
	if opts.ReportHTMLPath != "" {
		path := config.ResolveOutputPath(files[0], opts.ReportHTMLPath)
		if err := report.WriteHTML(path, reportData); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write HTML report: %v\n", err)
		}
	}

	printRunSummary(reportData)
	if reportData.Failed > 0 || len(reportData.Flaky) > 0 {
		return 1
	}
	return 0
}

func runFiles(files []string, opts model.RunOptions) (model.RunReport, error) {
	started := time.Now()
	snapshots := snapshot.NewManager(".reqres_snapshots")

	fileReports, loadResults, err := runRound(files, opts, snapshots, true)
	if err != nil {
		return model.RunReport{}, err
	}

	flakyMap := map[string]bool{}
	if opts.DetectFlakyRuns > 1 {
		history := map[string]map[model.TestStatus]int{}
		recordHistory(history, fileReports)
		for round := 2; round <= opts.DetectFlakyRuns; round++ {
			roundReports, _, err := runRound(files, opts, snapshots, false)
			if err != nil {
				return model.RunReport{}, err
			}
			recordHistory(history, roundReports)
		}
		for key, counters := range history {
			_, hadPass := counters[model.StatusPass]
			_, hadFail := counters[model.StatusFail]
			if hadPass && hadFail {
				flakyMap[key] = true
			}
		}
	}

	flakyNames := []string{}
	for i := range fileReports {
		for j := range fileReports[i].Tests {
			key := flakyKey(fileReports[i].File, fileReports[i].Tests[j].Name)
			if !flakyMap[key] {
				continue
			}
			fileReports[i].Tests[j].Status = model.StatusFlaky
			fileReports[i].Tests[j].Message = "flaky: mixed pass/fail across reruns"
			flakyNames = append(flakyNames, key)
		}
	}
	sort.Strings(flakyNames)

	out := model.RunReport{
		StartedAt:   started,
		FinishedAt:  time.Now(),
		GeneratedBy: "reqres",
		Files:       fileReports,
		Flaky:       flakyNames,
	}
	for _, file := range out.Files {
		out.Total += file.Total
		for _, test := range file.Tests {
			switch test.Status {
			case model.StatusPass:
				out.Passed++
			case model.StatusFail, model.StatusFlaky:
				out.Failed++
				out.Failures = append(out.Failures, model.FailureEntry{
					File: file.File,
					Test: test.Name,
					Why:  test.Message,
				})
			case model.StatusSkip:
				out.Skipped++
			}
		}
	}
	out.DurationMS = out.FinishedAt.Sub(out.StartedAt).Milliseconds()

	if len(loadResults) == 1 {
		out.Load = loadResults[0]
	}
	return out, nil
}

func runRound(files []string, opts model.RunOptions, snapshots *snapshot.Manager, includeLoad bool) ([]model.FileReport, []*model.LoadSummary, error) {
	type roundResult struct {
		file   string
		report model.FileReport
		load   *model.LoadSummary
		err    error
	}
	results := make([]roundResult, len(files))
	sem := make(chan struct{}, max(1, opts.Parallel))
	var wg sync.WaitGroup

	for i, file := range files {
		wg.Add(1)
		go func(i int, file string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			cfg, err := config.LoadFromFile(file, opts.Env)
			if err != nil {
				results[i] = roundResult{file: file, err: err}
				return
			}
			if errs := config.Validate(cfg); len(errs) > 0 {
				messages := make([]string, 0, len(errs))
				for _, e := range errs {
					messages = append(messages, e.Error())
				}
				results[i] = roundResult{file: file, err: errors.New(strings.Join(messages, "; "))}
				return
			}

			fileReport, _ := runner.RunFile(runner.FileRunOptions{
				FilePath:        file,
				Config:          cfg,
				RunOptions:      opts,
				SnapshotManager: snapshots,
			})

			var loadSummary *model.LoadSummary
			if includeLoad && opts.RunLoad && cfg.Load != nil && allowByTags(cfg.Load.Tags, opts.Tags) {
				expandedLoad, err := expandLoadConfig(*cfg.Load, cfg.Vars)
				if err != nil {
					results[i] = roundResult{file: file, err: err}
					return
				}
				expandedLoad.Path, err = utils.ExpandString(expandedLoad.Path, cfg.Vars)
				if err != nil {
					results[i] = roundResult{file: file, err: err}
					return
				}
				headers := mergeHeaders(cfg.Defaults.Headers, expandedLoad.Headers)
				expandedHeaders := map[string]string{}
				for key, value := range headers {
					expanded, err := utils.ExpandString(value, cfg.Vars)
					if err != nil {
						results[i] = roundResult{file: file, err: err}
						return
					}
					expandedHeaders[key] = expanded
				}
				loadSummary, err = loadtest.Run(expandedLoad, loadtest.Options{
					BaseURL:   cfg.Base,
					Headers:   expandedHeaders,
					Auth:      cfg.Defaults.Auth,
					TimeoutMS: cfg.Timeout,
					Retries:   cfg.Retries,
				})
				if err != nil {
					results[i] = roundResult{file: file, err: err}
					return
				}
			}

			results[i] = roundResult{
				file:   file,
				report: fileReport,
				load:   loadSummary,
			}
		}(i, file)
	}
	wg.Wait()

	fileReports := make([]model.FileReport, 0, len(files))
	loads := []*model.LoadSummary{}
	for _, item := range results {
		if item.err != nil {
			return nil, nil, fmt.Errorf("%s: %w", item.file, item.err)
		}
		fileReports = append(fileReports, item.report)
		if item.load != nil {
			loads = append(loads, item.load)
		}
	}
	return fileReports, loads, nil
}

func validateCommand(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	env := fs.String("env", "", "environment override name")
	if err := fs.Parse(reorderArgs(args, map[string]bool{"--env": true})); err != nil {
		return 1
	}
	files := fs.Args()
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "validate requires at least one yaml file")
		return 1
	}

	hasErrors := false
	for _, file := range files {
		cfg, err := config.LoadFromFile(file, strings.TrimSpace(*env))
		if err != nil {
			hasErrors = true
			fmt.Printf("%s %s\n", utils.Red("INVALID"), file)
			fmt.Printf("  %v\n", err)
			continue
		}
		errs := config.Validate(cfg)
		if len(errs) == 0 {
			fmt.Printf("%s %s\n", utils.Green("VALID"), file)
			continue
		}
		hasErrors = true
		fmt.Printf("%s %s\n", utils.Red("INVALID"), file)
		for _, err := range errs {
			fmt.Printf("  - %v\n", err)
		}
	}
	if hasErrors {
		return 1
	}
	return 0
}

func mockCommand(args []string) int {
	fs := flag.NewFlagSet("mock", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	env := fs.String("env", "", "environment override name")
	port := fs.Int("port", 8080, "port to listen on")
	if err := fs.Parse(reorderArgs(args, map[string]bool{"--env": true, "--port": true})); err != nil {
		return 1
	}
	files := fs.Args()
	if len(files) != 1 {
		fmt.Fprintln(os.Stderr, "mock requires exactly one yaml file")
		return 1
	}

	cfg, err := config.LoadFromFile(files[0], strings.TrimSpace(*env))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := mockserver.Serve(cfg, mockserver.Options{Port: *port}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func generateCommand(args []string) int {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	output := fs.String("o", "", "output yaml file")
	if err := fs.Parse(reorderArgs(args, map[string]bool{"-o": true})); err != nil {
		return 1
	}
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(os.Stderr, "generate requires one OpenAPI spec file")
		return 1
	}
	out, err := openapi.GenerateFromFile(rest[0], strings.TrimSpace(*output))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("Generated %s\n", out)
	return 0
}

func ghaInitCommand(args []string) int {
	target := ""
	if len(args) > 0 {
		target = strings.TrimSpace(args[0])
	}
	if err := gha.WriteWorkflow(target); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if target == "" {
		target = filepath.Join(".github", "workflows", "reqres.yml")
	}
	fmt.Printf("Generated %s\n", target)
	return 0
}

func printUsage() {
	fmt.Println(`ReqRes - API testing CLI

Usage:
  reqres run <file...> [--tags smoke] [--env staging] [--parallel 8]
  reqres validate <file...>
  reqres mock <file> [--port 8080]
  reqres generate <openapi.json|yaml> [-o tests.yaml]
  reqres gha-init [path]
`)
}

func printRunSummary(data model.RunReport) {
	for _, file := range data.Files {
		fmt.Printf("\n%s (%d ms)\n", utils.Blue(file.File), file.Duration)
		for _, test := range file.Tests {
			label := string(test.Status)
			switch test.Status {
			case model.StatusPass:
				label = utils.Green("PASS")
			case model.StatusFail:
				label = utils.Red("FAIL")
			case model.StatusSkip:
				label = utils.Yellow("SKIP")
			case model.StatusFlaky:
				label = utils.Yellow("FLAKY")
			}
			fmt.Printf("  [%s] %s (%s %s)", label, test.Name, test.Method, test.Path)
			if test.Message != "" && test.Message != "ok" {
				fmt.Printf(" - %s", test.Message)
			}
			fmt.Println()
		}
	}

	fmt.Printf("\nSummary: total=%d pass=%d fail=%d skip=%d duration=%dms\n",
		data.Total, data.Passed, data.Failed, data.Skipped, data.DurationMS)
	if len(data.Flaky) > 0 {
		fmt.Printf("Flaky tests: %s\n", strings.Join(data.Flaky, ", "))
	}
	if data.Load != nil {
		fmt.Printf("Load: %s %s users=%d requests=%d success=%d fail=%d avg=%0.2fms p95=%0.2fms\n",
			data.Load.Method, data.Load.Path, data.Load.Users, data.Load.Requests, data.Load.Successes, data.Load.Failures, data.Load.AvgMS, data.Load.P95MS)
	}
}

func parseCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag != "" {
			out = append(out, tag)
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

func allowByTags(itemTags []string, selected []string) bool {
	if len(selected) == 0 {
		return true
	}
	if len(itemTags) == 0 {
		return true
	}
	set := map[string]struct{}{}
	for _, tag := range selected {
		set[tag] = struct{}{}
	}
	for _, tag := range itemTags {
		if _, ok := set[tag]; ok {
			return true
		}
	}
	return false
}

func expandLoadConfig(loadCfg model.LoadConfig, vars map[string]any) (model.LoadConfig, error) {
	out := loadCfg
	queryAny, err := utils.ExpandAny(loadCfg.Query, vars)
	if err != nil {
		return out, err
	}
	bodyAny, err := utils.ExpandAny(loadCfg.Body, vars)
	if err != nil {
		return out, err
	}
	headersAny, err := utils.ExpandAny(loadCfg.Headers, vars)
	if err != nil {
		return out, err
	}
	checkAny, err := utils.ExpandAny(loadCfg.Check, vars)
	if err != nil {
		return out, err
	}

	out.Query = utils.ToStringMap(queryAny)
	out.Body = bodyAny
	out.Headers = map[string]string{}
	for k, v := range utils.ToStringMap(headersAny) {
		out.Headers[k] = utils.ToString(v)
	}
	out.Check = checkAny
	if out.Method == "" {
		out.Method = "GET"
	}
	return out, nil
}

func recordHistory(history map[string]map[model.TestStatus]int, files []model.FileReport) {
	for _, file := range files {
		for _, test := range file.Tests {
			key := flakyKey(file.File, test.Name)
			if _, ok := history[key]; !ok {
				history[key] = map[model.TestStatus]int{}
			}
			history[key][test.Status]++
		}
	}
}

func flakyKey(file string, test string) string {
	return file + "::" + test
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func fillDefaultForBareFlag(args []string, flagName string, defaultValue string) []string {
	out := make([]string, 0, len(args)+1)
	for i := 0; i < len(args); i++ {
		item := args[i]
		out = append(out, item)
		if item != flagName {
			continue
		}
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			continue
		}
		out = append(out, defaultValue)
	}
	return out
}

func reorderArgs(args []string, takesValue map[string]bool) []string {
	flags := make([]string, 0, len(args))
	positional := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		item := args[i]
		if !strings.HasPrefix(item, "-") {
			positional = append(positional, item)
			continue
		}
		flags = append(flags, item)
		if strings.Contains(item, "=") {
			continue
		}
		if !takesValue[item] {
			continue
		}
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			flags = append(flags, args[i+1])
			i++
		}
	}
	return append(flags, positional...)
}
