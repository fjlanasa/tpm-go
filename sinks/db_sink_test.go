package sinks

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/config"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ─── helpers shared with parquet_sink_test.go ────────────────────────────────

func makeAttrs(agencyID, vehicleID, routeID, stopID string) *pb.EventAttributes {
	return &pb.EventAttributes{
		AgencyId:    agencyID,
		VehicleId:   vehicleID,
		RouteId:     routeID,
		StopId:      stopID,
		ServiceDate: "2024-01-15",
		Timestamp:   timestamppb.Now(),
	}
}

// ─── test fixtures ────────────────────────────────────────────────────────────

// createTestSchema runs the table DDL against db.
// With the testmemdb driver, CREATE TABLE statements are silently accepted.
func createTestSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS stop_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			agency_id TEXT NOT NULL DEFAULT '',
			vehicle_id TEXT NOT NULL DEFAULT '',
			route_id TEXT NOT NULL DEFAULT '',
			stop_id TEXT NOT NULL DEFAULT '',
			origin_stop_id TEXT NOT NULL DEFAULT '',
			destination_stop_id TEXT NOT NULL DEFAULT '',
			direction_id TEXT NOT NULL DEFAULT '',
			direction TEXT NOT NULL DEFAULT '',
			direction_destination TEXT NOT NULL DEFAULT '',
			parent_station TEXT NOT NULL DEFAULT '',
			stop_sequence INTEGER NOT NULL DEFAULT 0,
			stop_status INTEGER NOT NULL DEFAULT 0,
			trip_id TEXT NOT NULL DEFAULT '',
			service_date TEXT NOT NULL DEFAULT '',
			event_timestamp TEXT,
			stop_event_type TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS dwell_time_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			agency_id TEXT NOT NULL DEFAULT '',
			vehicle_id TEXT NOT NULL DEFAULT '',
			route_id TEXT NOT NULL DEFAULT '',
			stop_id TEXT NOT NULL DEFAULT '',
			origin_stop_id TEXT NOT NULL DEFAULT '',
			destination_stop_id TEXT NOT NULL DEFAULT '',
			direction_id TEXT NOT NULL DEFAULT '',
			direction TEXT NOT NULL DEFAULT '',
			direction_destination TEXT NOT NULL DEFAULT '',
			parent_station TEXT NOT NULL DEFAULT '',
			stop_sequence INTEGER NOT NULL DEFAULT 0,
			stop_status INTEGER NOT NULL DEFAULT 0,
			trip_id TEXT NOT NULL DEFAULT '',
			service_date TEXT NOT NULL DEFAULT '',
			event_timestamp TEXT,
			arrival_time TEXT,
			departure_time TEXT,
			dwell_time_seconds INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS travel_time_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			agency_id TEXT NOT NULL DEFAULT '',
			vehicle_id TEXT NOT NULL DEFAULT '',
			route_id TEXT NOT NULL DEFAULT '',
			stop_id TEXT NOT NULL DEFAULT '',
			origin_stop_id TEXT NOT NULL DEFAULT '',
			destination_stop_id TEXT NOT NULL DEFAULT '',
			direction_id TEXT NOT NULL DEFAULT '',
			direction TEXT NOT NULL DEFAULT '',
			direction_destination TEXT NOT NULL DEFAULT '',
			parent_station TEXT NOT NULL DEFAULT '',
			stop_sequence INTEGER NOT NULL DEFAULT 0,
			stop_status INTEGER NOT NULL DEFAULT 0,
			trip_id TEXT NOT NULL DEFAULT '',
			service_date TEXT NOT NULL DEFAULT '',
			event_timestamp TEXT,
			start_time TEXT,
			end_time TEXT,
			travel_time_seconds INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS headway_time_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			agency_id TEXT NOT NULL DEFAULT '',
			vehicle_id TEXT NOT NULL DEFAULT '',
			route_id TEXT NOT NULL DEFAULT '',
			stop_id TEXT NOT NULL DEFAULT '',
			origin_stop_id TEXT NOT NULL DEFAULT '',
			destination_stop_id TEXT NOT NULL DEFAULT '',
			direction_id TEXT NOT NULL DEFAULT '',
			direction TEXT NOT NULL DEFAULT '',
			direction_destination TEXT NOT NULL DEFAULT '',
			parent_station TEXT NOT NULL DEFAULT '',
			stop_sequence INTEGER NOT NULL DEFAULT 0,
			stop_status INTEGER NOT NULL DEFAULT 0,
			trip_id TEXT NOT NULL DEFAULT '',
			service_date TEXT NOT NULL DEFAULT '',
			event_timestamp TEXT,
			leading_vehicle_id TEXT NOT NULL DEFAULT '',
			following_vehicle_id TEXT NOT NULL DEFAULT '',
			headway_seconds INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("createTestSchema: %v", err)
		}
	}
}

// newTestDatabaseSink creates a DatabaseSink backed by an isolated in-memory
// testmemdb instance and returns both the sink and a direct DB handle for
// assertions.
func newTestDatabaseSink(t *testing.T) (*DatabaseSink, *sql.DB) {
	t.Helper()
	// Use test name as DSN so each test gets its own in-memory store.
	dsn := t.Name()
	db, err := sql.Open("testmemdb", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	createTestSchema(t, db)

	cfg := config.SinkConfig{
		MaxBatchSize:  100,
		FlushInterval: 50 * time.Millisecond,
		Database: config.DatabaseSinkConfig{
			Driver: "testmemdb", // test-only driver
			DSN:    dsn,
		},
	}
	sink, err := NewDatabaseSink(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewDatabaseSink: %v", err)
	}
	return sink, db
}

// ─── tests ───────────────────────────────────────────────────────────────────

func TestDatabaseSinkStopEvent(t *testing.T) {
	sink, db := newTestDatabaseSink(t)

	event := &pb.StopEvent{
		Attributes:    makeAttrs("MBTA", "v1", "Red", "S1"),
		StopEventType: pb.StopEvent_ARRIVAL,
	}
	sink.In() <- event

	time.Sleep(200 * time.Millisecond)

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM stop_events WHERE agency_id = 'MBTA'").Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 stop_event row, got %d", count)
	}

	var stopEventType string
	if err := db.QueryRow("SELECT stop_event_type FROM stop_events WHERE agency_id = 'MBTA'").Scan(&stopEventType); err != nil {
		t.Fatalf("query stop_event_type: %v", err)
	}
	if stopEventType != "ARRIVAL" {
		t.Errorf("expected stop_event_type=ARRIVAL, got %q", stopEventType)
	}
}

func TestDatabaseSinkDwellTimeEvent(t *testing.T) {
	sink, db := newTestDatabaseSink(t)

	now := timestamppb.Now()
	later := timestamppb.New(now.AsTime().Add(30 * time.Second))
	event := &pb.DwellTimeEvent{
		Attributes:       makeAttrs("MBTA", "v2", "Green", "S2"),
		ArrivalTime:      now,
		DepartureTime:    later,
		DwellTimeSeconds: 30,
	}
	sink.In() <- event

	time.Sleep(200 * time.Millisecond)

	var dwell int64
	if err := db.QueryRow("SELECT dwell_time_seconds FROM dwell_time_events WHERE vehicle_id = 'v2'").Scan(&dwell); err != nil {
		t.Fatalf("query: %v", err)
	}
	if dwell != 30 {
		t.Errorf("expected dwell_time_seconds=30, got %d", dwell)
	}
}

func TestDatabaseSinkTravelTimeEvent(t *testing.T) {
	sink, db := newTestDatabaseSink(t)

	now := timestamppb.Now()
	later := timestamppb.New(now.AsTime().Add(5 * time.Minute))
	event := &pb.TravelTimeEvent{
		Attributes:        makeAttrs("MBTA", "v3", "Blue", "S3"),
		StartTime:         now,
		EndTime:           later,
		TravelTimeSeconds: 300,
	}
	sink.In() <- event

	time.Sleep(200 * time.Millisecond)

	var travel int64
	if err := db.QueryRow("SELECT travel_time_seconds FROM travel_time_events WHERE vehicle_id = 'v3'").Scan(&travel); err != nil {
		t.Fatalf("query: %v", err)
	}
	if travel != 300 {
		t.Errorf("expected travel_time_seconds=300, got %d", travel)
	}
}

func TestDatabaseSinkHeadwayTimeEvent(t *testing.T) {
	sink, db := newTestDatabaseSink(t)

	event := &pb.HeadwayTimeEvent{
		Attributes:         makeAttrs("MBTA", "v4", "Orange", "S4"),
		LeadingVehicleId:   "v3",
		FollowingVehicleId: "v4",
		HeadwaySeconds:     120,
	}
	sink.In() <- event

	time.Sleep(200 * time.Millisecond)

	var headway int64
	var leading string
	row := db.QueryRow("SELECT headway_seconds FROM headway_time_events WHERE vehicle_id = 'v4'")
	if err := row.Scan(&headway); err != nil {
		t.Fatalf("query: %v", err)
	}
	if headway != 120 {
		t.Errorf("expected headway_seconds=120, got %d", headway)
	}

	row2 := db.QueryRow("SELECT leading_vehicle_id FROM headway_time_events WHERE vehicle_id = 'v4'")
	if err := row2.Scan(&leading); err != nil {
		t.Fatalf("query leading_vehicle_id: %v", err)
	}
	if leading != "v3" {
		t.Errorf("expected leading_vehicle_id=v3, got %q", leading)
	}
}

func TestDatabaseSinkBatchFlush(t *testing.T) {
	sink, db := newTestDatabaseSink(t)

	const n = 10
	for i := range n {
		sink.In() <- &pb.StopEvent{
			Attributes:    makeAttrs("MBTA", fmt.Sprintf("v%d", i), "Red", "S1"),
			StopEventType: pb.StopEvent_DEPARTURE,
		}
	}

	time.Sleep(200 * time.Millisecond)

	var count int64
	if err := db.QueryRow("SELECT COUNT(*) FROM stop_events").Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != n {
		t.Errorf("expected %d rows, got %d", n, count)
	}
}

func TestDatabaseSinkInvalidTypeIsSkipped(t *testing.T) {
	sink, db := newTestDatabaseSink(t)

	// Non-event message should be silently skipped.
	sink.In() <- "not-an-event"
	sink.In() <- &pb.StopEvent{
		Attributes:    makeAttrs("MBTA", "v1", "Red", "S1"),
		StopEventType: pb.StopEvent_ARRIVAL,
	}

	time.Sleep(200 * time.Millisecond)

	var count int64
	if err := db.QueryRow("SELECT COUNT(*) FROM stop_events").Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 valid row, got %d", count)
	}
}

func TestDatabaseSinkContextCancellationFlushesRemaining(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	dsn := t.Name() + "_cancel"
	db, err := sql.Open("testmemdb", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	createTestSchema(t, db)

	// Very long flush interval so the flush only happens on cancel.
	cfg := config.SinkConfig{
		MaxBatchSize:  1000,
		FlushInterval: time.Hour,
		Database: config.DatabaseSinkConfig{
			Driver: "testmemdb",
			DSN:    dsn,
		},
	}
	sink, err := NewDatabaseSink(ctx, cfg)
	if err != nil {
		t.Fatalf("NewDatabaseSink: %v", err)
	}

	sink.In() <- &pb.StopEvent{
		Attributes:    makeAttrs("MBTA", "v1", "Red", "S1"),
		StopEventType: pb.StopEvent_ARRIVAL,
	}
	time.Sleep(20 * time.Millisecond) // let the event be received

	cancel() // triggers flush of buffered event
	time.Sleep(100 * time.Millisecond)

	var count int64
	if err := db.QueryRow("SELECT COUNT(*) FROM stop_events").Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row after cancel flush, got %d", count)
	}
}
