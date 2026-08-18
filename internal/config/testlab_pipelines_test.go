package config_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cannectors/runtime/internal/config"
)

// TestTestLabPipelinesAreSchemaCompliant guards the test-lab pipelines the same
// way TestExampleConfigsAreSchemaCompliant guards examples: a schema change that
// invalidates one of them must fail here rather than at the next manual E2E run.
// Story 25.2 tightened the append contract and silently broke
// http-call-header-append.yaml because nothing covered this directory.
//
// Only parsing and schema validation are checked — instantiating these pipelines
// would need the local WireMock and database the test-lab scripts bring up.
func TestTestLabPipelinesAreSchemaCompliant(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join("..", "..", "test-lab", "pipelines", "*"))
	if err != nil {
		t.Fatalf("glob test-lab pipelines: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("no test-lab pipelines found — did the fixtures move?")
	}

	for _, path := range matches {
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" && ext != ".json" {
			continue
		}
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			result := config.ParseConfig(path)
			if len(result.ParseErrors) > 0 {
				t.Fatalf("parse errors: %+v", result.ParseErrors)
			}
			if len(result.ValidationErrors) > 0 {
				t.Fatalf("schema validation errors: %+v", result.ValidationErrors)
			}
		})
	}
}
