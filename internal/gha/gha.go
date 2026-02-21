package gha

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Enabled(flag bool) bool {
	if flag {
		return true
	}
	return strings.EqualFold(os.Getenv("GITHUB_ACTIONS"), "true")
}

func EscapeAnnotation(value string) string {
	value = strings.ReplaceAll(value, "%", "%25")
	value = strings.ReplaceAll(value, "\r", "%0D")
	value = strings.ReplaceAll(value, "\n", "%0A")
	value = strings.ReplaceAll(value, ":", "%3A")
	value = strings.ReplaceAll(value, ",", "%2C")
	return value
}

func FailureAnnotation(file, test, message string) string {
	title := EscapeAnnotation(fmt.Sprintf("ReqRes %s", test))
	msg := EscapeAnnotation(message)
	if strings.TrimSpace(file) != "" {
		return fmt.Sprintf("::error file=%s,title=%s::%s", EscapeAnnotation(file), title, msg)
	}
	return fmt.Sprintf("::error title=%s::%s", title, msg)
}

func WriteWorkflow(path string) error {
	if strings.TrimSpace(path) == "" {
		path = filepath.Join(".github", "workflows", "reqres.yml")
	}
	content := "name: reqres-tests\n" +
		"on:\n" +
		"  pull_request:\n" +
		"  push:\n" +
		"    branches: [main]\n" +
		"jobs:\n" +
		"  api-tests:\n" +
		"    runs-on: ubuntu-latest\n" +
		"    steps:\n" +
		"      - uses: actions/checkout@v4\n" +
		"      - uses: actions/setup-go@v5\n" +
		"        with:\n" +
		"          go-version: '1.25.x'\n" +
		"      - name: Build ReqRes\n" +
		"        run: go build -o reqres ./\n" +
		"      - name: Run API tests\n" +
		"        run: ./reqres run tests.yaml --report-json reports/reqres.json --report-html reports/reqres.html --github-actions\n" +
		"      - name: Upload reports\n" +
		"        uses: actions/upload-artifact@v4\n" +
		"        with:\n" +
		"          name: reqres-report\n" +
		"          path: reports/\n"

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
