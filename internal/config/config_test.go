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
    name: "test_connector"
sinks:
  sink1:
    type: "test_sink"
pipelines:
  - type: "vehicle_position"
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
	if config == nil {
		t.Fatal("ReadGraphConfig() returned nil config")
	}

	// Verify config contents
	if len(config.Sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(config.Sources))
	}
	if len(config.StateStores) != 1 {
		t.Errorf("expected 1 state store, got %d", len(config.StateStores))
	}
	if len(config.Connectors) != 1 {
		t.Errorf("expected 1 connector, got %d", len(config.Connectors))
	}
	if len(config.Sinks) != 1 {
		t.Errorf("expected 1 sink, got %d", len(config.Sinks))
	}
	if len(config.Pipelines) != 1 {
		t.Errorf("expected 1 pipeline, got %d", len(config.Pipelines))
	}

	// Verify pipeline config
	pipeline := config.Pipelines[0]
	if pipeline.Type != PipelineTypeVehiclePosition {
		t.Errorf("expected pipeline type %v, got %v", PipelineTypeVehiclePosition, pipeline.Type)
	}
	expectedSources := []ID{"source1"}
	if !reflect.DeepEqual(pipeline.Sources, expectedSources) {
		t.Errorf("expected sources %v, got %v", expectedSources, pipeline.Sources)
	}
	if pipeline.StateStore != "store1" {
		t.Errorf("expected state store %v, got %v", "store1", pipeline.StateStore)
	}
	expectedSinks := []ID{"sink1"}
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
	config, err := ReadGraphConfig("nonexistent.yaml")
	if err == nil {
		t.Error("ReadConfig() error = nil, want error")
	}
	if config != nil {
		t.Errorf("ReadConfig() config = %v, want nil", config)
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

	config, err := ReadGraphConfig(tmpfile.Name())
	if err == nil {
		t.Error("ReadConfig() error = nil, want error")
	}
	if config != nil {
		t.Errorf("ReadConfig() config = %v, want nil", config)
	}
}
