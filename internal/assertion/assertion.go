package assertion

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/DevrajJain04/reqres/internal/utils"
)

func Evaluate(check any, statusCode int, headers http.Header, body any) error {
	if check == nil {
		return assertStatus(200, statusCode)
	}

	switch t := check.(type) {
	case int:
		return assertStatus(t, statusCode)
	case int64:
		return assertStatus(int(t), statusCode)
	case float64:
		return assertStatus(int(t), statusCode)
	case string:
		raw := strings.TrimSpace(t)
		expected, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("check string must be a status code or map, got %q", t)
		}
		return assertStatus(expected, statusCode)
	case map[string]any:
		return evaluateMapCheck(t, statusCode, headers, body)
	default:
		return fmt.Errorf("unsupported check type %T", check)
	}
}

func evaluateMapCheck(check map[string]any, statusCode int, headers http.Header, body any) error {
	expectedStatus := 200
	if rawStatus, ok := check["status"]; ok {
		expectedStatus = utils.ToInt(rawStatus, expectedStatus)
	}
	if err := assertStatus(expectedStatus, statusCode); err != nil {
		return err
	}

	if rawHeaders, ok := check["headers"]; ok {
		headerChecks := utils.ToStringMap(rawHeaders)
		for key, expected := range headerChecks {
			actual := headerValue(headers, key)
			if err := evaluateExpectation(fmt.Sprintf("header[%s]", key), actual, expected, true); err != nil {
				return err
			}
		}
	}

	if rawBody, ok := check["body"]; ok {
		if err := evaluateBodyBlock(rawBody, body); err != nil {
			return err
		}
	}

	for key, expected := range check {
		if strings.HasPrefix(key, "$") {
			if err := assertPath(key, expected, body); err != nil {
				return err
			}
		}
	}

	return nil
}

func evaluateBodyBlock(raw any, body any) error {
	switch t := raw.(type) {
	case map[string]any:
		for path, expected := range t {
			if err := assertPath(path, expected, body); err != nil {
				return err
			}
		}
	case []any:
		for _, row := range t {
			item := utils.ToStringMap(row)
			path := utils.ToString(item["path"])
			if strings.TrimSpace(path) == "" {
				return fmt.Errorf("body check list item requires path")
			}
			expected := item["value"]
			if op := utils.ToString(item["operator"]); strings.TrimSpace(op) != "" {
				// Lightweight operator support for compatibility with verbose formats.
				switch strings.ToLower(op) {
				case "eq":
				default:
					return fmt.Errorf("unsupported body operator %q", op)
				}
			}
			if err := assertPath(path, expected, body); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("body checks must be map or list, got %T", raw)
	}
	return nil
}

func assertPath(path string, expected any, body any) error {
	actual, found, err := evalJSONPath(path, body)
	if err != nil {
		return err
	}
	return evaluateExpectation(path, actual, expected, found)
}

func evaluateExpectation(label string, actual any, expected any, found bool) error {
	expectedString, expectedIsString := expected.(string)
	if expectedIsString {
		switch strings.TrimSpace(expectedString) {
		case "exists":
			if !found {
				return fmt.Errorf("%s expected to exist", label)
			}
			return nil
		case "!empty":
			if !found || isEmpty(actual) {
				return fmt.Errorf("%s expected non-empty value", label)
			}
			return nil
		}

		trimmed := strings.TrimSpace(expectedString)
		if strings.HasPrefix(trimmed, "/") && strings.HasSuffix(trimmed, "/") && len(trimmed) > 2 {
			pattern := trimmed[1 : len(trimmed)-1]
			re, err := regexp.Compile(pattern)
			if err != nil {
				return fmt.Errorf("%s has invalid regex %q: %w", label, pattern, err)
			}
			if !re.MatchString(utils.ToString(actual)) {
				return fmt.Errorf("%s regex %q did not match %q", label, pattern, utils.ToString(actual))
			}
			return nil
		}

		if expr, ok := parseLenExpr(trimmed); ok {
			size := valueLen(actual)
			if !expr.eval(size) {
				return fmt.Errorf("%s length assertion failed: got %d, expected %s %d", label, size, expr.op, expr.expected)
			}
			return nil
		}
	}

	if !found {
		return fmt.Errorf("%s not found", label)
	}
	if !valuesEqual(actual, expected) {
		return fmt.Errorf("%s mismatch: expected %v (%T), got %v (%T)", label, expected, expected, actual, actual)
	}
	return nil
}

func assertStatus(expected int, actual int) error {
	if expected != actual {
		return fmt.Errorf("status mismatch: expected %d, got %d", expected, actual)
	}
	return nil
}

func headerValue(headers http.Header, name string) string {
	if value := headers.Get(name); value != "" {
		return value
	}
	for key, values := range headers {
		if strings.EqualFold(key, name) {
			return strings.Join(values, ",")
		}
	}
	return ""
}

func isEmpty(value any) bool {
	switch t := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	default:
		return false
	}
}

type lenExpr struct {
	op       string
	expected int
}

func (l lenExpr) eval(actual int) bool {
	switch l.op {
	case "==":
		return actual == l.expected
	case "!=":
		return actual != l.expected
	case ">":
		return actual > l.expected
	case ">=":
		return actual >= l.expected
	case "<":
		return actual < l.expected
	case "<=":
		return actual <= l.expected
	default:
		return false
	}
}

func parseLenExpr(raw string) (lenExpr, bool) {
	parts := strings.Fields(strings.ToLower(raw))
	if len(parts) != 3 || parts[0] != "len" {
		return lenExpr{}, false
	}
	if parts[1] != "==" && parts[1] != "!=" && parts[1] != ">" && parts[1] != ">=" && parts[1] != "<" && parts[1] != "<=" {
		return lenExpr{}, false
	}
	value, err := strconv.Atoi(parts[2])
	if err != nil {
		return lenExpr{}, false
	}
	return lenExpr{
		op:       parts[1],
		expected: value,
	}, true
}

func valueLen(value any) int {
	switch t := value.(type) {
	case nil:
		return 0
	case string:
		return len(t)
	case []any:
		return len(t)
	case map[string]any:
		return len(t)
	default:
		return 0
	}
}

func valuesEqual(actual any, expected any) bool {
	if nums, ok := bothNumber(actual, expected); ok {
		return nums[0] == nums[1]
	}
	return reflect.DeepEqual(actual, expected)
}

func bothNumber(a any, b any) ([2]float64, bool) {
	af, okA := toFloat(a)
	bf, okB := toFloat(b)
	if okA && okB {
		return [2]float64{af, bf}, true
	}
	return [2]float64{}, false
}

func toFloat(value any) (float64, bool) {
	switch t := value.(type) {
	case int:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	case float32:
		return float64(t), true
	case float64:
		return t, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}
