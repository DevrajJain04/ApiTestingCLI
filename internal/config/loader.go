package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DevrajJain04/reqres/internal/model"
	"github.com/DevrajJain04/reqres/internal/utils"
	"github.com/DevrajJain04/reqres/internal/yamlmini"
)

func LoadFromFile(path string, env string) (model.Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return model.Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	decoded, err := yamlmini.Parse(content)
	if err != nil {
		return model.Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	root, ok := decoded.(map[string]any)
	if !ok {
		return model.Config{}, fmt.Errorf("config %s: root must be a map", path)
	}

	cfg, err := decodeConfig(root)
	if err != nil {
		return model.Config{}, fmt.Errorf("config %s: %w", path, err)
	}

	if env != "" {
		if err := applyEnv(&cfg, env); err != nil {
			return model.Config{}, fmt.Errorf("config %s: %w", path, err)
		}
	}

	// Keep timeout and retries predictable for runner internals.
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5000
	}
	if cfg.Retries < 0 {
		cfg.Retries = 0
	}

	for i := range cfg.Tests {
		if cfg.Tests[i].Method == "" {
			cfg.Tests[i].Method = "GET"
		}
		cfg.Tests[i].Method = strings.ToUpper(cfg.Tests[i].Method)
		if cfg.Tests[i].Check == nil {
			cfg.Tests[i].Check = 200
		}
		if cfg.Tests[i].Headers == nil {
			cfg.Tests[i].Headers = map[string]string{}
		}
		if cfg.Tests[i].Query == nil {
			cfg.Tests[i].Query = map[string]any{}
		}
	}

	return cfg, nil
}

func decodeConfig(root map[string]any) (model.Config, error) {
	cfg := model.Config{
		Base:     utils.ToString(root["base"]),
		Timeout:  utils.ToInt(root["timeout"], 5000),
		Retries:  utils.ToInt(root["retries"], 0),
		Vars:     utils.ToStringMap(root["vars"]),
		Defaults: decodeDefaults(root["defaults"]),
		Envs:     decodeEnvs(root["envs"]),
		Load:     decodeLoad(root["load"]),
		Mock:     decodeMockConfig(root["mock"]),
	}
	tests, err := decodeTests(root["tests"])
	if err != nil {
		return model.Config{}, err
	}
	cfg.Tests = tests
	return cfg, nil
}

func decodeDefaults(raw any) model.Defaults {
	data := utils.ToStringMap(raw)
	return model.Defaults{
		Headers: utils.ToStringStringMap(data["headers"]),
		Auth:    utils.ToString(data["auth"]),
	}
}

func decodeEnvs(raw any) map[string]model.EnvOverride {
	root := utils.ToStringMap(raw)
	out := map[string]model.EnvOverride{}
	for name, value := range root {
		data := utils.ToStringMap(value)
		override := model.EnvOverride{
			Base: utils.ToString(data["base"]),
			Vars: utils.ToStringMap(data["vars"]),
		}
		if _, ok := data["timeout"]; ok {
			v := utils.ToInt(data["timeout"], 0)
			override.Timeout = &v
		}
		if _, ok := data["retries"]; ok {
			v := utils.ToInt(data["retries"], 0)
			override.Retries = &v
		}
		if _, ok := data["defaults"]; ok {
			d := decodeDefaults(data["defaults"])
			override.Defaults = &d
		}
		out[name] = override
	}
	return out
}

func decodeTests(raw any) ([]model.TestCase, error) {
	rows := utils.ToSlice(raw)
	out := make([]model.TestCase, 0, len(rows))
	for idx, row := range rows {
		testMap := utils.ToStringMap(row)
		if len(testMap) == 0 {
			return nil, fmt.Errorf("tests[%d] must be a map", idx)
		}
		test := model.TestCase{
			Name:     utils.ToString(testMap["name"]),
			Method:   strings.ToUpper(utils.ToString(testMap["method"])),
			Path:     utils.ToString(testMap["path"]),
			Headers:  utils.ToStringStringMap(testMap["headers"]),
			Query:    utils.ToStringMap(testMap["query"]),
			Body:     testMap["body"],
			Auth:     utils.ToString(testMap["auth"]),
			Tags:     decodeTags(testMap["tags"]),
			Check:    testMap["check"],
			Capture:  decodeCapture(testMap["capture"]),
			After:    utils.ToString(testMap["after"]),
			Snapshot: testMap["snapshot"],
			Mock:     decodeMockRoute(testMap["mock"]),
		}
		if _, ok := testMap["retries"]; ok {
			v := utils.ToInt(testMap["retries"], 0)
			test.Retries = &v
		}
		if _, ok := testMap["timeout"]; ok {
			v := utils.ToInt(testMap["timeout"], 0)
			test.TimeoutMS = &v
		}
		out = append(out, test)
	}
	return out, nil
}

func decodeTags(raw any) []string {
	if raw == nil {
		return nil
	}
	if value, ok := raw.(string); ok {
		if strings.TrimSpace(value) == "" {
			return nil
		}
		parts := strings.Split(value, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			tag := strings.TrimSpace(part)
			if tag != "" {
				out = append(out, tag)
			}
		}
		return out
	}
	return utils.ToStringSlice(raw)
}

func decodeCapture(raw any) map[string]string {
	in := utils.ToStringMap(raw)
	if len(in) == 0 {
		return nil
	}
	out := map[string]string{}
	for k, v := range in {
		out[k] = utils.ToString(v)
	}
	return out
}

func decodeLoad(raw any) *model.LoadConfig {
	if raw == nil {
		return nil
	}
	data := utils.ToStringMap(raw)
	if len(data) == 0 {
		return nil
	}
	return &model.LoadConfig{
		Users:    utils.ToInt(data["users"], 1),
		Duration: utils.ToString(data["duration"]),
		RampUp:   utils.ToString(data["ramp_up"]),
		Method:   strings.ToUpper(utils.ToString(data["method"])),
		Path:     utils.ToString(data["path"]),
		Query:    utils.ToStringMap(data["query"]),
		Headers:  utils.ToStringStringMap(data["headers"]),
		Body:     data["body"],
		Check:    data["check"],
		Tags:     decodeTags(data["tags"]),
	}
}

func decodeMockConfig(raw any) *model.MockConfig {
	if raw == nil {
		return nil
	}
	data := utils.ToStringMap(raw)
	if len(data) == 0 {
		return nil
	}
	rows := utils.ToSlice(data["routes"])
	routes := make([]model.MockRoute, 0, len(rows))
	for _, row := range rows {
		route := decodeMockRoute(row)
		if route != nil {
			routes = append(routes, *route)
		}
	}
	return &model.MockConfig{
		Routes: routes,
		Delay:  utils.ToString(data["delay"]),
	}
}

func decodeMockRoute(raw any) *model.MockRoute {
	data := utils.ToStringMap(raw)
	if len(data) == 0 {
		return nil
	}
	return &model.MockRoute{
		Name:    utils.ToString(data["name"]),
		Method:  strings.ToUpper(utils.ToString(data["method"])),
		Path:    utils.ToString(data["path"]),
		Status:  utils.ToInt(data["status"], 200),
		Headers: utils.ToStringStringMap(data["headers"]),
		Body:    data["body"],
		Query:   utils.ToStringMap(data["query"]),
		Delay:   utils.ToString(data["delay"]),
	}
}

func applyEnv(cfg *model.Config, env string) error {
	override, ok := cfg.Envs[env]
	if !ok {
		available := make([]string, 0, len(cfg.Envs))
		for name := range cfg.Envs {
			available = append(available, name)
		}
		return fmt.Errorf("env %q not found (available: %s)", env, strings.Join(available, ", "))
	}
	if override.Base != "" {
		cfg.Base = override.Base
	}
	if override.Timeout != nil {
		cfg.Timeout = *override.Timeout
	}
	if override.Retries != nil {
		cfg.Retries = *override.Retries
	}
	for k, v := range override.Vars {
		if cfg.Vars == nil {
			cfg.Vars = map[string]any{}
		}
		cfg.Vars[k] = v
	}
	if override.Defaults != nil {
		if cfg.Defaults.Headers == nil {
			cfg.Defaults.Headers = map[string]string{}
		}
		for k, v := range override.Defaults.Headers {
			cfg.Defaults.Headers[k] = v
		}
		if override.Defaults.Auth != "" {
			cfg.Defaults.Auth = override.Defaults.Auth
		}
	}
	return nil
}

func ResolveOutputPath(baseFile string, output string) string {
	if output == "" {
		return ""
	}
	if filepath.IsAbs(output) {
		return output
	}
	baseDir := filepath.Dir(baseFile)
	return filepath.Join(baseDir, output)
}
