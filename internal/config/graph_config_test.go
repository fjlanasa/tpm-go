package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadGraphConfig(t *testing.T) {
	// Create a temporary test file
	testConfig := `
sources:
  source1:
    type: "test_source"
state_stores:
  store1:
    type: "test_store"
connectors:
  connector1:
sinks:
  sink1:
    type: "test_sink"
pipelines:
  vp_pipeline:
    type: "vehicle_position"
    sources: ["source1"]
    state_store: "store1"
    sinks: ["sink1"]
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.yaml")
	err := os.WriteFile(configPath, []byte(testConfig), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Test successful config reading
	config, err := ReadGraphConfig(configPath)
	if err != nil {
		t.Fatalf("ReadGraphConfig() error = %v", err)
	}

	// Verify config contents
	if len(config.Connectors) != 1 {
		t.Errorf("expected 1 connector, got %d", len(config.Connectors))
	}

	// Verify pipeline config
	pipeline := config.Pipelines[0]
	if pipeline.Type != PipelineTypeVehiclePosition {
		t.Errorf("expected pipeline type %v, got %v", PipelineTypeVehiclePosition, pipeline.Type)
	}
	expectedSources := []SourceConfig{
		{
			ID:   "source1",
			Type: "test_source",
		},
	}
	if !reflect.DeepEqual(pipeline.Sources, expectedSources) {
		t.Errorf("expected sources %v, got %v", expectedSources, pipeline.Sources)
	}
	expectedStateStore := StateStoreConfig{
		ID:   "store1",
		Type: "test_store",
	}
	if !reflect.DeepEqual(pipeline.StateStore, expectedStateStore) {
		t.Errorf("expected state store %v, got %v", expectedStateStore, pipeline.StateStore)
	}
	expectedSinks := []SinkConfig{
		{
			ID:   "sink1",
			Type: "test_sink",
		},
	}
	if !reflect.DeepEqual(pipeline.Sinks, expectedSinks) {
		t.Errorf("expected sinks %v, got %v", expectedSinks, pipeline.Sinks)
	}

	// Test reading non-existent file
	_, err = ReadGraphConfig("non_existent_file.yaml")
	if err == nil {
		t.Error("expected error when reading non-existent file, got nil")
	}

	// Test invalid YAML
	invalidConfig := "invalid: yaml: :"
	invalidPath := filepath.Join(tmpDir, "invalid_config.yaml")
	err = os.WriteFile(invalidPath, []byte(invalidConfig), 0644)
	if err != nil {
		t.Fatalf("failed to write invalid config: %v", err)
	}

	_, err = ReadGraphConfig(invalidPath)
	if err == nil {
		t.Error("expected error when reading invalid YAML, got nil")
	}
}

func TestReadGraphConfigInvalidPath(t *testing.T) {
	_, err := ReadGraphConfig("nonexistent.yaml")
	if err == nil {
		t.Error("ReadConfig() error = nil, want error")
	}
}

func TestReadConfigInvalidYAML(t *testing.T) {
	// Create a temporary file with invalid YAML
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	invalidYAML := `
pipelines:
  - type: invalid
    sources:
      - type: [invalid yaml]
`
	if _, err := tmpfile.Write([]byte(invalidYAML)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = ReadGraphConfig(tmpfile.Name())
	if err == nil {
		t.Error("ReadConfig() error = nil, want error")
	}
}
