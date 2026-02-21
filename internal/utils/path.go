package utils

import "strings"

func SanitizeFileName(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "unnamed"
	}
	replacer := strings.NewReplacer(
		"\\", "_",
		"/", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
	)
	return replacer.Replace(trimmed)
}
