package utils

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

func ToStringMap(value any) map[string]any {
	out := map[string]any{}
	if value == nil {
		return out
	}
	raw, ok := value.(map[string]any)
	if !ok {
		return out
	}
	for k, v := range raw {
		out[k] = v
	}
	return out
}

func ToStringStringMap(value any) map[string]string {
	out := map[string]string{}
	if value == nil {
		return out
	}
	raw, ok := value.(map[string]any)
	if !ok {
		return out
	}
	for k, v := range raw {
		out[k] = ToString(v)
	}
	return out
}

func ToSlice(value any) []any {
	if value == nil {
		return nil
	}
	s, ok := value.([]any)
	if !ok {
		return nil
	}
	return s
}

func ToStringSlice(value any) []string {
	raw := ToSlice(value)
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		out = append(out, ToString(item))
	}
	return out
}

func ToString(value any) string {
	switch t := value.(type) {
	case nil:
		return ""
	case string:
		return t
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		if math.Trunc(t) == t {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

func ToInt(value any, fallback int) int {
	switch t := value.(type) {
	case nil:
		return fallback
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case string:
		v, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil {
			return fallback
		}
		return v
	default:
		return fallback
	}
}

func CloneMapStringAny(src map[string]any) map[string]any {
	dst := map[string]any{}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func CloneMapStringString(src map[string]string) map[string]string {
	dst := map[string]string{}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func ParseMaybeJSON(raw string) any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var data any
	if err := json.Unmarshal([]byte(trimmed), &data); err == nil {
		return data
	}
	return raw
}

func JSONString(value any) string {
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(b)
}
