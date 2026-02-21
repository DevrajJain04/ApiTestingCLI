package mockserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DevrajJain04/reqres/internal/model"
	"github.com/DevrajJain04/reqres/internal/utils"
)

type Options struct {
	Port int
}

func Serve(cfg model.Config, opts Options) error {
	routes := buildRoutes(cfg)
	if len(routes) == 0 {
		return fmt.Errorf("no mock routes found (add mock.routes or test.mock)")
	}

	port := opts.Port
	if port <= 0 {
		port = 8080
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, ok := matchRoute(routes, r)
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"mock route not found"}`))
			return
		}

		delay := route.Delay
		if delay == "" && cfg.Mock != nil {
			delay = cfg.Mock.Delay
		}
		if strings.TrimSpace(delay) != "" {
			if parsed, err := time.ParseDuration(delay); err == nil {
				time.Sleep(parsed)
			}
		}

		for key, value := range route.Headers {
			w.Header().Set(key, value)
		}
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/json")
		}

		status := route.Status
		if status <= 0 {
			status = 200
		}
		w.WriteHeader(status)

		switch body := route.Body.(type) {
		case nil:
			_, _ = w.Write([]byte(`{}`))
		case string:
			_, _ = w.Write([]byte(body))
		default:
			payload, err := json.Marshal(body)
			if err != nil {
				_, _ = w.Write([]byte(`{"error":"invalid mock body"}`))
				return
			}
			_, _ = w.Write(payload)
		}
	})

	server := &http.Server{
		Addr:    ":" + strconv.Itoa(port),
		Handler: handler,
	}

	fmt.Printf("Mock server listening on http://localhost:%d with %d routes\n", port, len(routes))
	return server.ListenAndServe()
}

func Shutdown(server *http.Server, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return server.Shutdown(ctx)
}

type builtRoute struct {
	Method  string
	Path    string
	Status  int
	Headers map[string]string
	Body    any
	Query   map[string]any
	Delay   string
}

func buildRoutes(cfg model.Config) []builtRoute {
	var out []builtRoute
	if cfg.Mock != nil {
		for _, route := range cfg.Mock.Routes {
			out = append(out, builtRoute{
				Method:  pickMethod(route.Method),
				Path:    route.Path,
				Status:  route.Status,
				Headers: route.Headers,
				Body:    route.Body,
				Query:   route.Query,
				Delay:   route.Delay,
			})
		}
	}

	for _, test := range cfg.Tests {
		if test.Mock == nil {
			continue
		}
		route := *test.Mock
		method := route.Method
		if method == "" {
			method = test.Method
		}
		path := route.Path
		if path == "" {
			path = test.Path
		}
		out = append(out, builtRoute{
			Method:  pickMethod(method),
			Path:    path,
			Status:  route.Status,
			Headers: route.Headers,
			Body:    route.Body,
			Query:   route.Query,
			Delay:   route.Delay,
		})
	}
	return out
}

func matchRoute(routes []builtRoute, req *http.Request) (builtRoute, bool) {
	for _, route := range routes {
		if route.Path != req.URL.Path {
			continue
		}
		if route.Method != "ANY" && !strings.EqualFold(route.Method, req.Method) {
			continue
		}
		if !queryMatches(route.Query, req) {
			continue
		}
		return route, true
	}
	return builtRoute{}, false
}

func queryMatches(expected map[string]any, req *http.Request) bool {
	if len(expected) == 0 {
		return true
	}
	query := req.URL.Query()
	for key, value := range expected {
		if query.Get(key) != utils.ToString(value) {
			return false
		}
	}
	return true
}

func pickMethod(raw string) string {
	method := strings.ToUpper(strings.TrimSpace(raw))
	if method == "" {
		return "ANY"
	}
	return method
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
