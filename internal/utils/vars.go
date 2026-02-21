package utils

import (
	"fmt"
	"regexp"
	"strings"
)

var placeholderRE = regexp.MustCompile(`\$\{([a-zA-Z0-9_.-]+)\}`)

func ExpandString(input string, vars map[string]any) (string, error) {
	if input == "" {
		return "", nil
	}
	var missing []string
	out := placeholderRE.ReplaceAllStringFunc(input, func(token string) string {
		matches := placeholderRE.FindStringSubmatch(token)
		if len(matches) != 2 {
			return token
		}
		name := matches[1]
		value, ok := vars[name]
		if !ok {
			missing = append(missing, name)
			return token
		}
		return ToString(value)
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("missing vars: %s", strings.Join(uniqueStrings(missing), ", "))
	}
	return out, nil
}

func ExpandAny(value any, vars map[string]any) (any, error) {
	switch t := value.(type) {
	case string:
		return ExpandString(t, vars)
	case []any:
		out := make([]any, 0, len(t))
		for _, item := range t {
			expanded, err := ExpandAny(item, vars)
			if err != nil {
				return nil, err
			}
			out = append(out, expanded)
		}
		return out, nil
	case map[string]any:
		out := map[string]any{}
		for k, v := range t {
			expanded, err := ExpandAny(v, vars)
			if err != nil {
				return nil, err
			}
			out[k] = expanded
		}
		return out, nil
	default:
		return value, nil
	}
}

func uniqueStrings(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
