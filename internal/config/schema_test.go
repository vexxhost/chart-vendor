package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/vexxhost/chart-vendor/internal/config"
)

// repoRoot returns the absolute path to the repository root by walking up from
// this test file's location.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file path")
	}
	// filename is .../internal/config/schema_test.go; root is two levels up
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

func TestSchemaIsUpToDate(t *testing.T) {
	schema := config.GenerateSchema()

	generated, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		t.Fatalf("marshaling generated schema: %v", err)
	}
	generated = append(generated, '\n')

	schemaPath := filepath.Join(repoRoot(t), "charts.schema.json")
	committed, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("reading %s: %v", schemaPath, err)
	}

	if string(generated) != string(committed) {
		t.Errorf("charts.schema.json is stale; run `go run ./cmd/generate-schema/` to regenerate it")
	}
}
