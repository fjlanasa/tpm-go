package config

type PipelineType string

const (
	PipelineTypeFeedMessage     PipelineType = "feed_message"
	PipelineTypeVehiclePosition PipelineType = "vehicle_position"
	PipelineTypeStopEvent       PipelineType = "stop_event"
	PipelineTypeDwellEvent      PipelineType = "dwell_event"
	PipelineTypeHeadwayEvent    PipelineType = "headway_event"
	PipelineTypeTravelTime      PipelineType = "travel_time"
)

// Pipeline

type PipelineConfig struct {
	ID         ID           `yaml:"id"`
	AgencyID   ID           `yaml:"agency_id"`
	Type       PipelineType `yaml:"type"`
	Sources    []ID         `yaml:"sources"`
	StateStore ID           `yaml:"state_store"`
	Sinks      []ID         `yaml:"sinks"`
}
