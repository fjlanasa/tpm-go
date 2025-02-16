package events

type Event interface {
	GetAgencyId() string
	GetRouteId() string
	GetStopId() string
	GetEventType() string
	GetAttributes() map[string]string
}
