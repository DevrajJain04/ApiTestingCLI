package yamlmini

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func Marshal(value any) string {
	var b strings.Builder
	writeYAML(&b, normalize(value), 0)
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func writeYAML(b *strings.Builder, value any, indent int) {
	switch t := normalize(value).(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			writeIndent(b, indent)
			b.WriteString(key)
			b.WriteString(":")
			writeAfterKey(b, t[key], indent)
		}
	case []any:
		for _, item := range t {
			writeIndent(b, indent)
			b.WriteString("-")
			writeAfterListMarker(b, item, indent)
		}
	default:
		writeIndent(b, indent)
		b.WriteString(formatScalar(t))
		b.WriteString("\n")
	}
}

func writeAfterKey(b *strings.Builder, value any, indent int) {
	value = normalize(value)
	switch t := value.(type) {
	case map[string]any:
		if len(t) == 0 {
			b.WriteString(" {}\n")
			return
		}
		b.WriteString("\n")
		writeYAML(b, t, indent+2)
	case []any:
		if len(t) == 0 {
			b.WriteString(" []\n")
			return
		}
		b.WriteString("\n")
		writeYAML(b, t, indent+2)
	default:
		b.WriteString(" ")
		b.WriteString(formatScalar(value))
		b.WriteString("\n")
	}
}

func writeAfterListMarker(b *strings.Builder, value any, indent int) {
	value = normalize(value)
	switch t := value.(type) {
	case map[string]any:
		if len(t) == 0 {
			b.WriteString(" {}\n")
			return
		}
		b.WriteString("\n")
		writeYAML(b, t, indent+2)
	case []any:
		if len(t) == 0 {
			b.WriteString(" []\n")
			return
		}
		b.WriteString("\n")
		writeYAML(b, t, indent+2)
	default:
		b.WriteString(" ")
		b.WriteString(formatScalar(value))
		b.WriteString("\n")
	}
}

func formatScalar(value any) string {
	switch t := value.(type) {
	case nil:
		return "null"
	case string:
		if t == "" {
			return `""`
		}
		if needsQuote(t) {
			return strconv.Quote(t)
		}
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case int, int32, int64, float64, float32:
		return fmt.Sprintf("%v", t)
	default:
		return strconv.Quote(fmt.Sprintf("%v", t))
	}
}

func normalize(value any) any {
	switch t := value.(type) {
	case map[string]string:
		out := map[string]any{}
		for k, v := range t {
			out[k] = normalize(v)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for k, v := range t {
			out[k] = normalize(v)
		}
		return out
	case []string:
		out := make([]any, 0, len(t))
		for _, item := range t {
			out = append(out, normalize(item))
		}
		return out
	case []any:
		out := make([]any, 0, len(t))
		for _, item := range t {
			out = append(out, normalize(item))
		}
		return out
	default:
		return value
	}
}

func needsQuote(value string) bool {
	if strings.ContainsAny(value, ":\n#[]{}") {
		return true
	}
	return strings.TrimSpace(value) != value
}

func writeIndent(b *strings.Builder, indent int) {
	for i := 0; i < indent; i++ {
		b.WriteByte(' ')
	}
}
