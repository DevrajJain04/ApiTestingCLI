package openapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DevrajJain04/reqres/internal/utils"
	"github.com/DevrajJain04/reqres/internal/yamlmini"
)

var validMethods = []string{"get", "post", "put", "patch", "delete", "head", "options"}

func GenerateFromFile(specPath string, outputPath string) (string, error) {
	spec, err := loadSpec(specPath)
	if err != nil {
		return "", err
	}
	document, err := buildReqRes(spec)
	if err != nil {
		return "", err
	}
	yaml := yamlmini.Marshal(document)

	target := outputPath
	if strings.TrimSpace(target) == "" {
		base := strings.TrimSuffix(filepath.Base(specPath), filepath.Ext(specPath))
		target = base + ".tests.yaml"
	}
	if err := os.WriteFile(target, []byte(yaml), 0o644); err != nil {
		return "", err
	}
	return target, nil
}

func loadSpec(path string) (map[string]any, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var asJSON map[string]any
	if json.Unmarshal(content, &asJSON) == nil {
		return asJSON, nil
	}
	parsed, err := yamlmini.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parse openapi spec: %w", err)
	}
	root, ok := parsed.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("openapi root must be a map")
	}
	return root, nil
}

func buildReqRes(spec map[string]any) (map[string]any, error) {
	base := pickBaseURL(spec)
	paths := utils.ToStringMap(spec["paths"])
	if len(paths) == 0 {
		return nil, fmt.Errorf("openapi spec has no paths")
	}

	tests := []any{}
	pathKeys := make([]string, 0, len(paths))
	for path := range paths {
		pathKeys = append(pathKeys, path)
	}
	sort.Strings(pathKeys)

	for _, path := range pathKeys {
		operations := utils.ToStringMap(paths[path])
		for _, method := range validMethods {
			opRaw, ok := operations[method]
			if !ok {
				continue
			}
			op := utils.ToStringMap(opRaw)
			name := utils.ToString(op["summary"])
			if name == "" {
				name = utils.ToString(op["operationId"])
			}
			if name == "" {
				name = strings.ToUpper(method) + " " + path
			}
			status := pickStatusCode(op)
			test := map[string]any{
				"name":   name,
				"method": strings.ToUpper(method),
				"path":   path,
				"tags":   utils.ToStringSlice(op["tags"]),
				"check":  status,
			}
			tests = append(tests, test)
		}
	}

	if len(tests) == 0 {
		return nil, fmt.Errorf("openapi spec had paths but no operations")
	}

	return map[string]any{
		"base":    base,
		"timeout": 5000,
		"retries": 0,
		"tests":   tests,
	}, nil
}

func pickBaseURL(spec map[string]any) string {
	servers := utils.ToSlice(spec["servers"])
	if len(servers) > 0 {
		server := utils.ToStringMap(servers[0])
		if url := utils.ToString(server["url"]); url != "" {
			return url
		}
	}
	return "http://localhost:8080"
}

func pickStatusCode(operation map[string]any) int {
	responses := utils.ToStringMap(operation["responses"])
	if len(responses) == 0 {
		return 200
	}
	keys := make([]string, 0, len(responses))
	for key := range responses {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if strings.HasPrefix(key, "2") {
			return utils.ToInt(key, 200)
		}
	}
	return utils.ToInt(keys[0], 200)
}
