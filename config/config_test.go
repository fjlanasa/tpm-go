package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadConfig(t *testing.T) {
	// Create a temporary test file
	testConfig := `
graph:
  sources:
    source1:
      type: "test_source"
  state_stores:
    store1:
      type: "test_store"
  connectors:
    connector1:
      type: "test_connector"
  sinks:
    sink1:
      type: "test_sink"
  pipelines:
    vp_pipeline:
      type: "vehicle_position"
      sources: ["source1"]
      state_store: "store1"
      sinks: ["connector1", "sink1"]
    se_pipeline:
      type: "stop_event"
      sources: ["connector1"]
      state_store: "store1"
      sinks: ["sink1"]
event_server:
  port: "8080"
  path: "/events"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.yaml")
	err := os.WriteFile(configPath, []byte(testConfig), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Test successful config reading
	config, err := ReadConfig(configPath)
	if err != nil {
		t.Fatalf("ReadConfig() error = %v", err)
	}

	// Verify config contents
	if len(config.Graph.Connectors) != 1 {
		t.Errorf("expected 1 connector, got %d", len(config.Graph.Connectors))
	}

	// Verify pipeline config
	pipeline := config.Graph.Pipelines["vp_pipeline"]
	if pipeline.Type != PipelineTypeVehiclePosition {
		t.Errorf("expected pipeline type %v, got %v", PipelineTypeVehiclePosition, pipeline.Type)
	}
	expectedSources := []ID{
		"source1",
	}
	if !reflect.DeepEqual(pipeline.Sources, expectedSources) {
		t.Errorf("expected sources %v, got %v", expectedSources, pipeline.Sources)
	}
	expectedStateStore := ID("store1")
	if !reflect.DeepEqual(pipeline.StateStore, expectedStateStore) {
		t.Errorf("expected state store %v, got %v", expectedStateStore, pipeline.StateStore)
	}
	expectedSinks := []ID{
		"connector1",
		"sink1",
	}
	for i, sink := range pipeline.Sinks {
		if !reflect.DeepEqual(sink, expectedSinks[i]) {
			t.Errorf("expected sink %v, got %v", expectedSinks[i], sink)
		}
	}

	pipeline = config.Graph.Pipelines["se_pipeline"]
	if pipeline.Type != PipelineTypeStopEvent {
		t.Errorf("expected pipeline type %v, got %v", PipelineTypeStopEvent, pipeline.Type)
	}
	expectedSources = []ID{
		"connector1",
	}
	if !reflect.DeepEqual(pipeline.Sources, expectedSources) {
		t.Errorf("expected sources %v, got %v", expectedSources, pipeline.Sources)
	}
	expectedStateStore = ID("store1")
	if !reflect.DeepEqual(pipeline.StateStore, expectedStateStore) {
		t.Errorf("expected state store %v, got %v", expectedStateStore, pipeline.StateStore)
	}
	expectedSinks = []ID{
		"sink1",
	}
	if !reflect.DeepEqual(pipeline.Sinks, expectedSinks) {
		t.Errorf("expected sinks %v, got %v", expectedSinks, pipeline.Sinks)
	}
}

func TestReadGraphConfigInvalidPath(t *testing.T) {
	_, err := ReadConfig("nonexistent.yaml")
	if err == nil {
		t.Error("ReadConfig() error = nil, want error")
	}
}

// --- Validate() unit tests ---

func makeValidGraphConfig() *GraphConfig {
	return &GraphConfig{
		Sources: map[ID]SourceConfig{
			"src1": {
				Type: SourceTypeHTTP,
				HTTP: HTTPSourceConfig{URL: "http://example.com/feed", Interval: "10s"},
			},
		},
		Sinks: map[ID]SinkConfig{
			"sink1": {Type: SinkTypeConsole},
		},
		Connectors:  map[ID]interface{}{},
		StateStores: map[ID]StateStoreConfig{},
		Pipelines: map[ID]PipelineConfig{
			"pipe1": {
				ID:      "pipe1",
				Type:    PipelineTypeVehiclePosition,
				Sources: []ID{"src1"},
				Sinks:   []ID{"sink1"},
			},
		},
	}
}

func TestValidateValidConfig(t *testing.T) {
	cfg := makeValidGraphConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() returned unexpected error: %v", err)
	}
}

func TestValidateHTTPSourceMissingURL(t *testing.T) {
	cfg := makeValidGraphConfig()
	cfg.Sources["src1"] = SourceConfig{
		Type: SourceTypeHTTP,
		HTTP: HTTPSourceConfig{URL: "", Interval: "10s"},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() expected error for missing HTTP URL, got nil")
	}
}

func TestValidateHTTPSourceMissingInterval(t *testing.T) {
	cfg := makeValidGraphConfig()
	cfg.Sources["src1"] = SourceConfig{
		Type: SourceTypeHTTP,
		HTTP: HTTPSourceConfig{URL: "http://example.com", Interval: ""},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() expected error for missing HTTP interval, got nil")
	}
}

func TestValidateHTTPSourceInvalidInterval(t *testing.T) {
	cfg := makeValidGraphConfig()
	cfg.Sources["src1"] = SourceConfig{
		Type: SourceTypeHTTP,
		HTTP: HTTPSourceConfig{URL: "http://example.com", Interval: "not-a-duration"},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() expected error for invalid HTTP interval, got nil")
	}
}

func TestValidatePipelineInvalidType(t *testing.T) {
	cfg := makeValidGraphConfig()
	cfg.Pipelines["pipe1"] = PipelineConfig{
		ID:      "pipe1",
		Type:    "invalid_type",
		Sources: []ID{"src1"},
		Sinks:   []ID{"sink1"},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() expected error for invalid pipeline type, got nil")
	}
}

func TestValidatePipelineNoSources(t *testing.T) {
	cfg := makeValidGraphConfig()
	cfg.Pipelines["pipe1"] = PipelineConfig{
		ID:      "pipe1",
		Type:    PipelineTypeVehiclePosition,
		Sources: []ID{},
		Sinks:   []ID{"sink1"},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() expected error for pipeline with no sources, got nil")
	}
}

func TestValidatePipelineNoSinks(t *testing.T) {
	cfg := makeValidGraphConfig()
	cfg.Pipelines["pipe1"] = PipelineConfig{
		ID:      "pipe1",
		Type:    PipelineTypeVehiclePosition,
		Sources: []ID{"src1"},
		Sinks:   []ID{},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() expected error for pipeline with no sinks, got nil")
	}
}

func TestValidatePipelineUnknownSource(t *testing.T) {
	cfg := makeValidGraphConfig()
	cfg.Pipelines["pipe1"] = PipelineConfig{
		ID:      "pipe1",
		Type:    PipelineTypeVehiclePosition,
		Sources: []ID{"nonexistent-source"},
		Sinks:   []ID{"sink1"},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() expected error for pipeline referencing unknown source, got nil")
	}
}

func TestValidatePipelineUnknownSink(t *testing.T) {
	cfg := makeValidGraphConfig()
	cfg.Pipelines["pipe1"] = PipelineConfig{
		ID:      "pipe1",
		Type:    PipelineTypeVehiclePosition,
		Sources: []ID{"src1"},
		Sinks:   []ID{"nonexistent-sink"},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() expected error for pipeline referencing unknown sink, got nil")
	}
}

func TestValidateConnectorCountsAsSourceAndSink(t *testing.T) {
	cfg := &GraphConfig{
		Sources:     map[ID]SourceConfig{},
		Sinks:       map[ID]SinkConfig{},
		Connectors:  map[ID]interface{}{"connector1": nil},
		StateStores: map[ID]StateStoreConfig{},
		Pipelines: map[ID]PipelineConfig{
			"pipe1": {
				ID:      "pipe1",
				Type:    PipelineTypeFeedMessage,
				Sources: []ID{"connector1"},
				Sinks:   []ID{"connector1"},
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() unexpected error when using connector as source and sink: %v", err)
	}
}

func TestReadConfigInvalidYAML(t *testing.T) {
	// Create a temporary file with invalid YAML
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpfile.Name()) }()

	invalidYAML := `
graph:
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

	_, err = ReadConfig(tmpfile.Name())
	if err == nil {
		t.Error("ReadConfig() error = nil, want error")
	}
}
