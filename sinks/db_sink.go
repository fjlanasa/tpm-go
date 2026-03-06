package sinks

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/config"
	"github.com/reugn/go-streams"

	// Register the pgx driver for PostgreSQL (database/sql interface).
	// SQLite: callers must import their preferred driver, e.g.:
	//   _ "modernc.org/sqlite"        (pure Go, no CGO)
	//   _ "github.com/mattn/go-sqlite3" (CGO, widely used)
	_ "github.com/jackc/pgx/v5/stdlib"
)

// DatabaseSink writes transit metric events to a relational database in batches.
// Supported driver values in config:
//   - "postgres" — uses pgx stdlib driver (registered by this package)
//   - "sqlite"   — caller must import a SQLite driver, e.g. modernc.org/sqlite
//   - any other string is passed directly to database/sql as the driver name
//
// Tables must be created beforehand using the migration files in db/migrations/.
type DatabaseSink struct {
	streams.Sink
	db            *sql.DB
	driver        string
	in            chan any
	maxBatchSize  int
	flushInterval time.Duration
	ctx           context.Context
}

func NewDatabaseSink(ctx context.Context, cfg config.SinkConfig) (*DatabaseSink, error) {
	driverName := cfg.Database.Driver
	if driverName == "" {
		return nil, fmt.Errorf("db sink: driver is required (\"postgres\" or \"sqlite\")")
	}
	if cfg.Database.DSN == "" {
		return nil, fmt.Errorf("db sink: dsn is required")
	}

	// Map friendly driver names to the registered database/sql driver names.
	sqlDriverName := driverName
	if driverName == "postgres" {
		sqlDriverName = "pgx"
	}
	// "sqlite" and other names are passed through unchanged.

	db, err := sql.Open(sqlDriverName, cfg.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("db sink: open db: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("db sink: ping db: %w", err)
	}

	maxBatchSize := cfg.MaxBatchSize
	if maxBatchSize <= 0 {
		maxBatchSize = 100
	}
	flushInterval := cfg.FlushInterval
	if flushInterval <= 0 {
		flushInterval = 10 * time.Second
	}

	sink := &DatabaseSink{
		db:            db,
		driver:        driverName,
		in:            make(chan any),
		maxBatchSize:  maxBatchSize,
		flushInterval: flushInterval,
		ctx:           ctx,
	}
	go sink.doSink(ctx)
	return sink, nil
}

func (s *DatabaseSink) In() chan<- any {
	return s.in
}

func (s *DatabaseSink) doSink(ctx context.Context) {
	batch := make([]events.Event, 0, s.maxBatchSize)
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()
	defer s.db.Close()

	for {
		select {
		case <-ctx.Done():
			if len(batch) > 0 {
				// Use a fresh context so the final INSERT isn't immediately cancelled.
				s.flush(context.Background(), batch)
			}
			return
		case <-ticker.C:
			if len(batch) > 0 {
				s.flush(ctx, batch)
				batch = batch[:0]
			}
		case msg, ok := <-s.in:
			if !ok {
				return
			}
			event, ok := msg.(events.Event)
			if !ok {
				slog.Warn("db sink: unexpected message type", "type", fmt.Sprintf("%T", msg))
				continue
			}
			batch = append(batch, event)
			if len(batch) >= s.maxBatchSize {
				s.flush(ctx, batch)
				batch = batch[:0]
			}
		}
	}
}

func (s *DatabaseSink) flush(ctx context.Context, batch []events.Event) {
	var stopEvents []*events.StopEvent
	var dwellEvents []*events.DwellTimeEvent
	var travelEvents []*events.TravelTimeEvent
	var headwayEvents []*events.HeadwayTimeEvent

	for _, e := range batch {
		switch ev := e.(type) {
		case *events.StopEvent:
			stopEvents = append(stopEvents, ev)
		case *events.DwellTimeEvent:
			dwellEvents = append(dwellEvents, ev)
		case *events.TravelTimeEvent:
			travelEvents = append(travelEvents, ev)
		case *events.HeadwayTimeEvent:
			headwayEvents = append(headwayEvents, ev)
		default:
			slog.Warn("db sink: unsupported event type, skipping", "type", fmt.Sprintf("%T", e))
		}
	}

	if len(stopEvents) > 0 {
		if err := s.insertStopEvents(ctx, stopEvents); err != nil {
			slog.Error("db sink: insert stop_events", "err", err)
		}
	}
	if len(dwellEvents) > 0 {
		if err := s.insertDwellTimeEvents(ctx, dwellEvents); err != nil {
			slog.Error("db sink: insert dwell_time_events", "err", err)
		}
	}
	if len(travelEvents) > 0 {
		if err := s.insertTravelTimeEvents(ctx, travelEvents); err != nil {
			slog.Error("db sink: insert travel_time_events", "err", err)
		}
	}
	if len(headwayEvents) > 0 {
		if err := s.insertHeadwayTimeEvents(ctx, headwayEvents); err != nil {
			slog.Error("db sink: insert headway_time_events", "err", err)
		}
	}
}

// placeholder returns a SQL positional placeholder for the given 1-based index.
// PostgreSQL uses $N; SQLite uses ?.
func (s *DatabaseSink) placeholder(n int) string {
	if s.driver == "postgres" {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

func (s *DatabaseSink) buildInsert(table string, cols []string, nRows int) string {
	nCols := len(cols)
	rowPH := make([]string, nRows)
	for i := range nRows {
		ph := make([]string, nCols)
		for j := range nCols {
			ph[j] = s.placeholder(i*nCols + j + 1)
		}
		rowPH[i] = "(" + strings.Join(ph, ", ") + ")"
	}
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES %s",
		table,
		strings.Join(cols, ", "),
		strings.Join(rowPH, ", "),
	)
}

func (s *DatabaseSink) insertStopEvents(ctx context.Context, batch []*events.StopEvent) error {
	cols := []string{
		"agency_id", "vehicle_id", "route_id", "stop_id",
		"origin_stop_id", "destination_stop_id", "direction_id",
		"direction", "direction_destination", "parent_station",
		"stop_sequence", "stop_status", "trip_id", "service_date",
		"event_timestamp", "stop_event_type",
	}
	args := make([]any, 0, len(batch)*len(cols))
	for _, e := range batch {
		a := e.GetAttributes()
		args = append(args,
			a.GetAgencyId(), a.GetVehicleId(), a.GetRouteId(), a.GetStopId(),
			a.GetOriginStopId(), a.GetDestinationStopId(), a.GetDirectionId(),
			a.GetDirection(), a.GetDirectionDestination(), a.GetParentStation(),
			a.GetStopSequence(), int32(a.GetStopStatus()), a.GetTripId(), a.GetServiceDate(),
			nullableTime(a),
			e.GetStopEventType().String(),
		)
	}
	_, err := s.db.ExecContext(ctx, s.buildInsert("stop_events", cols, len(batch)), args...)
	return err
}

func (s *DatabaseSink) insertDwellTimeEvents(ctx context.Context, batch []*events.DwellTimeEvent) error {
	cols := []string{
		"agency_id", "vehicle_id", "route_id", "stop_id",
		"origin_stop_id", "destination_stop_id", "direction_id",
		"direction", "direction_destination", "parent_station",
		"stop_sequence", "stop_status", "trip_id", "service_date",
		"event_timestamp", "arrival_time", "departure_time", "dwell_time_seconds",
	}
	args := make([]any, 0, len(batch)*len(cols))
	for _, e := range batch {
		a := e.GetAttributes()
		args = append(args,
			a.GetAgencyId(), a.GetVehicleId(), a.GetRouteId(), a.GetStopId(),
			a.GetOriginStopId(), a.GetDestinationStopId(), a.GetDirectionId(),
			a.GetDirection(), a.GetDirectionDestination(), a.GetParentStation(),
			a.GetStopSequence(), int32(a.GetStopStatus()), a.GetTripId(), a.GetServiceDate(),
			nullableTime(a),
			nullableTimestamp(e.GetArrivalTime()),
			nullableTimestamp(e.GetDepartureTime()),
			e.GetDwellTimeSeconds(),
		)
	}
	_, err := s.db.ExecContext(ctx, s.buildInsert("dwell_time_events", cols, len(batch)), args...)
	return err
}

func (s *DatabaseSink) insertTravelTimeEvents(ctx context.Context, batch []*events.TravelTimeEvent) error {
	cols := []string{
		"agency_id", "vehicle_id", "route_id", "stop_id",
		"origin_stop_id", "destination_stop_id", "direction_id",
		"direction", "direction_destination", "parent_station",
		"stop_sequence", "stop_status", "trip_id", "service_date",
		"event_timestamp", "start_time", "end_time", "travel_time_seconds",
	}
	args := make([]any, 0, len(batch)*len(cols))
	for _, e := range batch {
		a := e.GetAttributes()
		args = append(args,
			a.GetAgencyId(), a.GetVehicleId(), a.GetRouteId(), a.GetStopId(),
			a.GetOriginStopId(), a.GetDestinationStopId(), a.GetDirectionId(),
			a.GetDirection(), a.GetDirectionDestination(), a.GetParentStation(),
			a.GetStopSequence(), int32(a.GetStopStatus()), a.GetTripId(), a.GetServiceDate(),
			nullableTime(a),
			nullableTimestamp(e.GetStartTime()),
			nullableTimestamp(e.GetEndTime()),
			e.GetTravelTimeSeconds(),
		)
	}
	_, err := s.db.ExecContext(ctx, s.buildInsert("travel_time_events", cols, len(batch)), args...)
	return err
}

func (s *DatabaseSink) insertHeadwayTimeEvents(ctx context.Context, batch []*events.HeadwayTimeEvent) error {
	cols := []string{
		"agency_id", "vehicle_id", "route_id", "stop_id",
		"origin_stop_id", "destination_stop_id", "direction_id",
		"direction", "direction_destination", "parent_station",
		"stop_sequence", "stop_status", "trip_id", "service_date",
		"event_timestamp", "leading_vehicle_id", "following_vehicle_id", "headway_seconds",
	}
	args := make([]any, 0, len(batch)*len(cols))
	for _, e := range batch {
		a := e.GetAttributes()
		args = append(args,
			a.GetAgencyId(), a.GetVehicleId(), a.GetRouteId(), a.GetStopId(),
			a.GetOriginStopId(), a.GetDestinationStopId(), a.GetDirectionId(),
			a.GetDirection(), a.GetDirectionDestination(), a.GetParentStation(),
			a.GetStopSequence(), int32(a.GetStopStatus()), a.GetTripId(), a.GetServiceDate(),
			nullableTime(a),
			e.GetLeadingVehicleId(),
			e.GetFollowingVehicleId(),
			e.GetHeadwaySeconds(),
		)
	}
	_, err := s.db.ExecContext(ctx, s.buildInsert("headway_time_events", cols, len(batch)), args...)
	return err
}

// nullableTime returns a *time.Time for the event_timestamp field, or nil if unset.
func nullableTime(a *events.EventAttributes) *time.Time {
	if a == nil || a.GetTimestamp() == nil {
		return nil
	}
	t := a.GetTimestamp().AsTime()
	return &t
}

// nullableTimestamp returns a *time.Time for a google.protobuf.Timestamp field, or nil if unset.
func nullableTimestamp(ts interface{ AsTime() time.Time }) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	if t.IsZero() {
		return nil
	}
	return &t
}
