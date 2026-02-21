package yamlmini

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type line struct {
	indent int
	text   string
	raw    string
	no     int
}

// Parse reads a practical YAML subset used by ReqRes configs.
// It supports nested maps/lists and inline [] / {} collections.
func Parse(data []byte) (any, error) {
	lines, err := tokenize(string(data))
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return map[string]any{}, nil
	}

	startIndent := lines[0].indent
	value, next, err := parseNode(lines, 0, startIndent)
	if err != nil {
		return nil, err
	}
	if next != len(lines) {
		return nil, fmt.Errorf("yaml: trailing content after line %d", lines[next].no)
	}
	return value, nil
}

func tokenize(input string) ([]line, error) {
	normalized := strings.ReplaceAll(input, "\r\n", "\n")
	normalized = strings.TrimPrefix(normalized, "\uFEFF")
	rows := strings.Split(normalized, "\n")

	out := make([]line, 0, len(rows))
	for i, row := range rows {
		if strings.ContainsRune(row, '\t') {
			return nil, fmt.Errorf("yaml: tabs are not supported (line %d)", i+1)
		}
		clean := stripComment(row)
		clean = strings.TrimRight(clean, " ")
		if strings.TrimSpace(clean) == "" {
			continue
		}
		indent := leadingSpaces(clean)
		out = append(out, line{
			indent: indent,
			text:   strings.TrimSpace(clean),
			raw:    row,
			no:     i + 1,
		})
	}
	return out, nil
}

func parseNode(lines []line, idx int, indent int) (any, int, error) {
	if idx >= len(lines) {
		return nil, idx, nil
	}
	if lines[idx].indent < indent {
		return nil, idx, nil
	}
	if lines[idx].indent > indent {
		indent = lines[idx].indent
	}

	if isListItem(lines[idx].text) {
		return parseList(lines, idx, indent)
	}
	return parseMap(lines, idx, indent)
}

func parseMap(lines []line, idx int, indent int) (map[string]any, int, error) {
	result := map[string]any{}
	for idx < len(lines) {
		current := lines[idx]
		if current.indent < indent {
			break
		}
		if current.indent > indent {
			return nil, idx, fmt.Errorf("yaml: unexpected indent at line %d", current.no)
		}
		if isListItem(current.text) {
			break
		}

		key, valuePart, ok := splitKeyValue(current.text)
		if !ok {
			return nil, idx, fmt.Errorf("yaml: expected key/value at line %d", current.no)
		}
		idx++

		if valuePart == "" {
			if idx < len(lines) && lines[idx].indent > indent {
				child, next, err := parseNode(lines, idx, lines[idx].indent)
				if err != nil {
					return nil, idx, err
				}
				result[key] = child
				idx = next
			} else {
				result[key] = nil
			}
			continue
		}

		parsed, err := parseValue(valuePart)
		if err != nil {
			return nil, idx, fmt.Errorf("yaml: %w at line %d", err, current.no)
		}
		result[key] = parsed
	}
	return result, idx, nil
}

func parseList(lines []line, idx int, indent int) ([]any, int, error) {
	result := []any{}
	for idx < len(lines) {
		current := lines[idx]
		if current.indent < indent {
			break
		}
		if current.indent > indent {
			return nil, idx, fmt.Errorf("yaml: unexpected indent in list at line %d", current.no)
		}
		if !isListItem(current.text) {
			break
		}

		itemText := listItemText(current.text)
		idx++
		if itemText == "" {
			if idx < len(lines) && lines[idx].indent > indent {
				child, next, err := parseNode(lines, idx, lines[idx].indent)
				if err != nil {
					return nil, idx, err
				}
				result = append(result, child)
				idx = next
			} else {
				result = append(result, nil)
			}
			continue
		}

		if key, valuePart, ok := splitKeyValue(itemText); ok && !strings.HasPrefix(itemText, "{") {
			item := map[string]any{}
			if valuePart == "" {
				if idx < len(lines) && lines[idx].indent > indent+2 {
					child, next, err := parseNode(lines, idx, lines[idx].indent)
					if err != nil {
						return nil, idx, err
					}
					item[key] = child
					idx = next
				} else {
					item[key] = nil
				}
			} else {
				parsed, err := parseValue(valuePart)
				if err != nil {
					return nil, idx, fmt.Errorf("yaml: %w at line %d", err, current.no)
				}
				item[key] = parsed
			}

			for idx < len(lines) {
				nextLine := lines[idx]
				if nextLine.indent <= indent {
					break
				}
				if nextLine.indent != indent+2 {
					break
				}
				if isListItem(nextLine.text) {
					break
				}
				subKey, subValue, ok := splitKeyValue(nextLine.text)
				if !ok {
					return nil, idx, fmt.Errorf("yaml: expected key/value at line %d", nextLine.no)
				}
				idx++
				if subValue == "" {
					if idx < len(lines) && lines[idx].indent > nextLine.indent {
						child, next, err := parseNode(lines, idx, lines[idx].indent)
						if err != nil {
							return nil, idx, err
						}
						item[subKey] = child
						idx = next
					} else {
						item[subKey] = nil
					}
				} else {
					parsed, err := parseValue(subValue)
					if err != nil {
						return nil, idx, fmt.Errorf("yaml: %w at line %d", err, nextLine.no)
					}
					item[subKey] = parsed
				}
			}
			result = append(result, item)
			continue
		}

		parsed, err := parseValue(itemText)
		if err != nil {
			return nil, idx, fmt.Errorf("yaml: %w at line %d", err, current.no)
		}
		result = append(result, parsed)
	}
	return result, idx, nil
}

func parseValue(raw string) (any, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	if strings.HasPrefix(value, "{") {
		return parseInlineMap(value)
	}
	if strings.HasPrefix(value, "[") {
		return parseInlineList(value)
	}
	if strings.HasPrefix(value, "\"") || strings.HasPrefix(value, "'") {
		return parseQuoted(value)
	}

	switch strings.ToLower(value) {
	case "null", "~":
		return nil, nil
	case "true":
		return true, nil
	case "false":
		return false, nil
	}

	if n, err := strconv.ParseInt(value, 10, 64); err == nil {
		return int(n), nil
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f, nil
	}
	return value, nil
}

func parseInlineMap(raw string) (map[string]any, error) {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasSuffix(trimmed, "}") {
		return nil, fmt.Errorf("invalid inline map %q", raw)
	}
	body := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	out := map[string]any{}
	if body == "" {
		return out, nil
	}
	parts, err := splitTopLevel(body, ',')
	if err != nil {
		return nil, err
	}
	for _, part := range parts {
		key, valuePart, ok := splitKeyValue(part)
		if !ok {
			return nil, fmt.Errorf("invalid inline map item %q", part)
		}
		value, err := parseValue(valuePart)
		if err != nil {
			return nil, err
		}
		out[key] = value
	}
	return out, nil
}

func parseInlineList(raw string) ([]any, error) {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasSuffix(trimmed, "]") {
		return nil, fmt.Errorf("invalid inline list %q", raw)
	}
	body := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	if body == "" {
		return []any{}, nil
	}
	parts, err := splitTopLevel(body, ',')
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(parts))
	for _, part := range parts {
		value, err := parseValue(part)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

func splitTopLevel(raw string, sep rune) ([]string, error) {
	parts := []string{}
	start := 0
	depthBraces := 0
	depthBrackets := 0
	inSingle := false
	inDouble := false
	escaped := false

	for i, r := range raw {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			if inDouble {
				escaped = true
			}
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case !inSingle && !inDouble && r == '{':
			depthBraces++
		case !inSingle && !inDouble && r == '}':
			depthBraces--
		case !inSingle && !inDouble && r == '[':
			depthBrackets++
		case !inSingle && !inDouble && r == ']':
			depthBrackets--
		case !inSingle && !inDouble && depthBraces == 0 && depthBrackets == 0 && r == sep:
			parts = append(parts, strings.TrimSpace(raw[start:i]))
			start = i + 1
		}
	}

	if inSingle || inDouble || depthBraces != 0 || depthBrackets != 0 {
		return nil, fmt.Errorf("malformed inline yaml expression")
	}
	parts = append(parts, strings.TrimSpace(raw[start:]))
	return parts, nil
}

func splitKeyValue(raw string) (string, string, bool) {
	depthBraces := 0
	depthBrackets := 0
	inSingle := false
	inDouble := false
	escaped := false

	for i, r := range raw {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			if inDouble {
				escaped = true
			}
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case !inSingle && !inDouble && r == '{':
			depthBraces++
		case !inSingle && !inDouble && r == '}':
			depthBraces--
		case !inSingle && !inDouble && r == '[':
			depthBrackets++
		case !inSingle && !inDouble && r == ']':
			depthBrackets--
		case !inSingle && !inDouble && depthBraces == 0 && depthBrackets == 0 && r == ':':
			key := strings.TrimSpace(raw[:i])
			value := strings.TrimSpace(raw[i+1:])
			if key == "" {
				return "", "", false
			}
			key = unquoteKey(key)
			return key, value, true
		}
	}
	return "", "", false
}

func parseQuoted(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "'") && strings.HasSuffix(trimmed, "'") {
		return strings.ReplaceAll(trimmed[1:len(trimmed)-1], "''", "'"), nil
	}
	value, err := strconv.Unquote(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid quoted string %q", raw)
	}
	return value, nil
}

func stripComment(raw string) string {
	inSingle := false
	inDouble := false
	escaped := false
	for i, r := range raw {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			if inDouble {
				escaped = true
			}
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case r == '#' && !inSingle && !inDouble:
			if i == 0 || unicode.IsSpace(rune(raw[i-1])) {
				return raw[:i]
			}
		}
	}
	return raw
}

func leadingSpaces(raw string) int {
	count := 0
	for _, r := range raw {
		if r != ' ' {
			break
		}
		count++
	}
	return count
}

func unquoteKey(key string) string {
	if len(key) >= 2 {
		if strings.HasPrefix(key, "\"") && strings.HasSuffix(key, "\"") {
			if parsed, err := strconv.Unquote(key); err == nil {
				return parsed
			}
		}
		if strings.HasPrefix(key, "'") && strings.HasSuffix(key, "'") {
			return strings.ReplaceAll(key[1:len(key)-1], "''", "'")
		}
	}
	return key
}

func isListItem(text string) bool {
	return text == "-" || strings.HasPrefix(text, "- ")
}

func listItemText(text string) string {
	if text == "-" {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(text, "- "))
}
