package httpx

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/DevrajJain04/reqres/internal/utils"
)

type RequestOptions struct {
	Method  string
	URL     string
	Headers map[string]string
	Query   map[string]any
	Body    any
	Auth    string
	Timeout time.Duration
}

type Response struct {
	StatusCode int
	Headers    http.Header
	BodyBytes  []byte
	BodyJSON   any
	BodyText   string
}

func Do(opts RequestOptions) (Response, error) {
	reqURL, err := addQuery(opts.URL, opts.Query)
	if err != nil {
		return Response{}, err
	}

	bodyReader, bodyContentType, err := requestBody(opts.Body)
	if err != nil {
		return Response{}, err
	}

	ctx := context.Background()
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(opts.Method), reqURL, bodyReader)
	if err != nil {
		return Response{}, err
	}

	for key, value := range opts.Headers {
		req.Header.Set(key, value)
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	if bodyContentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", bodyContentType)
	}

	if err := applyAuth(req, opts.Auth); err != nil {
		return Response{}, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, err
	}
	bodyText := string(respBytes)
	bodyJSON := decodeJSON(respBytes)

	return Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header.Clone(),
		BodyBytes:  respBytes,
		BodyText:   bodyText,
		BodyJSON:   bodyJSON,
	}, nil
}

func addQuery(rawURL string, query map[string]any) (string, error) {
	if len(query) == 0 {
		return rawURL, nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	values := parsed.Query()
	for key, raw := range query {
		switch t := raw.(type) {
		case []any:
			for _, item := range t {
				values.Add(key, utils.ToString(item))
			}
		default:
			values.Set(key, utils.ToString(raw))
		}
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func requestBody(body any) (io.Reader, string, error) {
	if body == nil {
		return nil, "", nil
	}
	switch t := body.(type) {
	case string:
		return strings.NewReader(t), "application/json", nil
	case []byte:
		return bytes.NewReader(t), "application/json", nil
	default:
		raw, err := json.Marshal(t)
		if err != nil {
			return nil, "", fmt.Errorf("marshal request body: %w", err)
		}
		return bytes.NewReader(raw), "application/json", nil
	}
}

func applyAuth(req *http.Request, auth string) error {
	value := strings.TrimSpace(auth)
	if value == "" {
		return nil
	}
	parts := strings.SplitN(value, " ", 2)
	if len(parts) < 2 {
		return fmt.Errorf("invalid auth format %q", auth)
	}
	switch strings.ToLower(strings.TrimSpace(parts[0])) {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(parts[1]))
	case "basic":
		credentials := strings.TrimSpace(parts[1])
		encoded := base64.StdEncoding.EncodeToString([]byte(credentials))
		req.Header.Set("Authorization", "Basic "+encoded)
	default:
		return fmt.Errorf("unsupported auth scheme %q", parts[0])
	}
	return nil
}

func decodeJSON(body []byte) any {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return map[string]any{}
	}
	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		return map[string]any{
			"text": trimmed,
		}
	}
	return data
}
