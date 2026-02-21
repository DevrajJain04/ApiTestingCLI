package loadtest

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DevrajJain04/reqres/internal/assertion"
	"github.com/DevrajJain04/reqres/internal/httpx"
	"github.com/DevrajJain04/reqres/internal/model"
	"github.com/DevrajJain04/reqres/internal/utils"
)

type Options struct {
	BaseURL   string
	Headers   map[string]string
	Auth      string
	TimeoutMS int
	Retries   int
}

func Run(loadCfg model.LoadConfig, opts Options) (*model.LoadSummary, error) {
	method := strings.ToUpper(strings.TrimSpace(loadCfg.Method))
	if method == "" {
		method = "GET"
	}
	duration, err := time.ParseDuration(strings.TrimSpace(loadCfg.Duration))
	if err != nil {
		return nil, fmt.Errorf("invalid load.duration %q: %w", loadCfg.Duration, err)
	}
	rampUp := time.Duration(0)
	if strings.TrimSpace(loadCfg.RampUp) != "" {
		if parsed, err := time.ParseDuration(strings.TrimSpace(loadCfg.RampUp)); err == nil {
			rampUp = parsed
		}
	}

	start := time.Now()
	stopAt := start.Add(duration)

	var requests int64
	var successes int64
	var failures int64
	latencies := []float64{}
	var latencyMu sync.Mutex

	users := loadCfg.Users
	if users <= 0 {
		users = 1
	}
	var wg sync.WaitGroup
	for i := 0; i < users; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			if rampUp > 0 && users > 1 {
				delay := time.Duration(float64(rampUp) * (float64(workerID) / float64(users-1)))
				time.Sleep(delay)
			}
			for time.Now().Before(stopAt) {
				startReq := time.Now()
				resp, err := withRetries(opts.Retries, func() (httpx.Response, error) {
					return httpx.Do(httpx.RequestOptions{
						Method:  method,
						URL:     joinURL(opts.BaseURL, loadCfg.Path),
						Headers: opts.Headers,
						Query:   loadCfg.Query,
						Body:    loadCfg.Body,
						Auth:    opts.Auth,
						Timeout: time.Duration(opts.TimeoutMS) * time.Millisecond,
					})
				})
				elapsed := float64(time.Since(startReq).Milliseconds())

				latencyMu.Lock()
				latencies = append(latencies, elapsed)
				latencyMu.Unlock()

				atomic.AddInt64(&requests, 1)
				if err != nil {
					atomic.AddInt64(&failures, 1)
					continue
				}
				if err := assertion.Evaluate(defaultCheck(loadCfg.Check), resp.StatusCode, resp.Headers, resp.BodyJSON); err != nil {
					atomic.AddInt64(&failures, 1)
					continue
				}
				atomic.AddInt64(&successes, 1)
			}
		}(i)
	}
	wg.Wait()

	f := summarizeLatencies(latencies)
	summary := &model.LoadSummary{
		Method:     method,
		Path:       loadCfg.Path,
		Users:      users,
		Requests:   requests,
		Successes:  successes,
		Failures:   failures,
		AvgMS:      f.avg,
		P95MS:      f.p95,
		MinMS:      f.min,
		MaxMS:      f.max,
		DurationMS: time.Since(start).Milliseconds(),
	}
	return summary, nil
}

func joinURL(base string, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func withRetries(retries int, action func() (httpx.Response, error)) (httpx.Response, error) {
	if retries < 0 {
		retries = 0
	}
	var lastErr error
	var resp httpx.Response
	for attempt := 0; attempt <= retries; attempt++ {
		resp, lastErr = action()
		if lastErr == nil {
			return resp, nil
		}
	}
	return resp, lastErr
}

func defaultCheck(raw any) any {
	if raw == nil {
		return 200
	}
	if _, ok := raw.(map[string]any); ok {
		return raw
	}
	return utils.ToInt(raw, 200)
}

type latencyStats struct {
	min float64
	max float64
	avg float64
	p95 float64
}

func summarizeLatencies(values []float64) latencyStats {
	if len(values) == 0 {
		return latencyStats{}
	}
	sort.Float64s(values)
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	p95Index := int(float64(len(values)-1) * 0.95)
	return latencyStats{
		min: values[0],
		max: values[len(values)-1],
		avg: sum / float64(len(values)),
		p95: values[p95Index],
	}
}
