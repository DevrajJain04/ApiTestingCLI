package snapshot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DevrajJain04/reqres/internal/utils"
)

type Manager struct {
	BaseDir string
}

func NewManager(baseDir string) *Manager {
	if strings.TrimSpace(baseDir) == "" {
		baseDir = ".reqres_snapshots"
	}
	return &Manager{BaseDir: baseDir}
}

func (m *Manager) Evaluate(filePath string, testName string, snapshotSpec any, body any, update bool) (bool, error) {
	name, enabled := snapshotName(testName, snapshotSpec)
	if !enabled {
		return false, nil
	}

	suite := utils.SanitizeFileName(strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath)))
	targetDir := filepath.Join(m.BaseDir, suite)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return false, fmt.Errorf("create snapshot dir: %w", err)
	}

	target := filepath.Join(targetDir, utils.SanitizeFileName(name)+".json")
	current, err := normalize(body)
	if err != nil {
		return false, fmt.Errorf("normalize snapshot for %s: %w", testName, err)
	}

	existing, readErr := os.ReadFile(target)
	if readErr != nil || update {
		if err := os.WriteFile(target, current, 0o644); err != nil {
			return false, fmt.Errorf("write snapshot: %w", err)
		}
		return true, nil
	}

	if !bytes.Equal(bytes.TrimSpace(existing), bytes.TrimSpace(current)) {
		return false, fmt.Errorf("snapshot mismatch for %s (%s)", testName, target)
	}
	return false, nil
}

func snapshotName(testName string, spec any) (string, bool) {
	if spec == nil {
		return "", false
	}
	switch t := spec.(type) {
	case bool:
		if t {
			return testName, true
		}
		return "", false
	case string:
		value := strings.TrimSpace(t)
		if value == "" {
			return testName, true
		}
		return value, true
	default:
		return testName, true
	}
}

func normalize(value any) ([]byte, error) {
	if value == nil {
		return []byte("null\n"), nil
	}
	switch t := value.(type) {
	case string:
		trimmed := strings.TrimSpace(t)
		var parsed any
		if json.Unmarshal([]byte(trimmed), &parsed) == nil {
			return json.MarshalIndent(parsed, "", "  ")
		}
		return []byte(trimmed + "\n"), nil
	default:
		return json.MarshalIndent(t, "", "  ")
	}
}
