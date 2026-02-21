package assertion

import (
	"fmt"
	"strconv"
	"strings"
)

type pathToken struct {
	field   string
	index   *int
	isField bool
}

func Extract(path string, body any) (any, bool, error) {
	return evalJSONPath(path, body)
}

func evalJSONPath(path string, root any) (any, bool, error) {
	tokens, err := parseJSONPath(path)
	if err != nil {
		return nil, false, err
	}
	current := root
	for _, token := range tokens {
		if token.isField {
			m, ok := current.(map[string]any)
			if !ok {
				return nil, false, nil
			}
			value, ok := m[token.field]
			if !ok {
				return nil, false, nil
			}
			current = value
			continue
		}
		arr, ok := current.([]any)
		if !ok {
			return nil, false, nil
		}
		idx := *token.index
		if idx < 0 || idx >= len(arr) {
			return nil, false, nil
		}
		current = arr[idx]
	}
	return current, true, nil
}

func parseJSONPath(path string) ([]pathToken, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "$" {
		return nil, nil
	}
	if !strings.HasPrefix(trimmed, "$") {
		return nil, fmt.Errorf("jsonpath must start with '$': %s", path)
	}

	tokens := []pathToken{}
	i := 1
	for i < len(trimmed) {
		switch trimmed[i] {
		case '.':
			i++
			start := i
			for i < len(trimmed) && trimmed[i] != '.' && trimmed[i] != '[' {
				i++
			}
			if start == i {
				return nil, fmt.Errorf("invalid jsonpath segment in %q", path)
			}
			tokens = append(tokens, pathToken{
				field:   trimmed[start:i],
				isField: true,
			})
		case '[':
			i++
			if i >= len(trimmed) {
				return nil, fmt.Errorf("invalid jsonpath index in %q", path)
			}
			if trimmed[i] == '\'' || trimmed[i] == '"' {
				quote := trimmed[i]
				i++
				start := i
				for i < len(trimmed) && trimmed[i] != quote {
					i++
				}
				if i >= len(trimmed) {
					return nil, fmt.Errorf("unterminated jsonpath key in %q", path)
				}
				key := trimmed[start:i]
				i++
				if i >= len(trimmed) || trimmed[i] != ']' {
					return nil, fmt.Errorf("invalid jsonpath key bracket in %q", path)
				}
				i++
				tokens = append(tokens, pathToken{
					field:   key,
					isField: true,
				})
				continue
			}
			start := i
			for i < len(trimmed) && trimmed[i] != ']' {
				i++
			}
			if i >= len(trimmed) {
				return nil, fmt.Errorf("unterminated jsonpath index in %q", path)
			}
			rawIdx := strings.TrimSpace(trimmed[start:i])
			i++
			idx, err := strconv.Atoi(rawIdx)
			if err != nil {
				return nil, fmt.Errorf("invalid jsonpath index %q", rawIdx)
			}
			idxCopy := idx
			tokens = append(tokens, pathToken{
				index: &idxCopy,
			})
		default:
			return nil, fmt.Errorf("unexpected token %q in jsonpath %q", string(trimmed[i]), path)
		}
	}
	return tokens, nil
}
