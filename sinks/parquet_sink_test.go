package sinks

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/parquet-go/parquet-go"

	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// readParquetRows deserializes Parquet bytes back into EventRow slices.
func readParquetRows(t *testing.T, data []byte) []EventRow {
	t.Helper()
	r := bytes.NewReader(data)
	f, err := parquet.OpenFile(r, int64(len(data)))
	if err != nil {
		t.Fatalf("parquet.OpenFile: %v", err)
	}
	reader := parquet.NewGenericReader[EventRow](f)
	defer func() { _ = reader.Close() }()

	rows := make([]EventRow, reader.NumRows())
	n, err := reader.Read(rows)
	if err != nil && err != io.EOF {
		t.Fatalf("parquet Read: %v", err)
	}
	return rows[:n]
}

func TestSerializeRowsStopEvent(t *testing.T) {
	event := &pb.StopEvent{
		Attributes:    makeAttrs("MBTA", "v1", "Red", "S1"),
		StopEventType: pb.StopEvent_ARRIVAL,
	}
	buf, err := serializeRows([]EventRow{toEventRow(event)})
	if err != nil {
		t.Fatalf("serializeRows: %v", err)
	}
	if len(buf) == 0 {
		t.Fatal("expected non-empty output")
	}

	rows := readParquetRows(t, buf)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	row := rows[0]

	if row.AgencyID != "MBTA" {
		t.Errorf("agency_id: want MBTA, got %q", row.AgencyID)
	}
	if row.VehicleID != "v1" {
		t.Errorf("vehicle_id: want v1, got %q", row.VehicleID)
	}
	if row.RouteID != "Red" {
		t.Errorf("route_id: want Red, got %q", row.RouteID)
	}
	if row.StopID != "S1" {
		t.Errorf("stop_id: want S1, got %q", row.StopID)
	}
	if row.EventType != "StopEvent" {
		t.Errorf("event_type: want StopEvent, got %q", row.EventType)
	}
	if row.StopEventType != "ARRIVAL" {
		t.Errorf("stop_event_type: want ARRIVAL, got %q", row.StopEventType)
	}
}

func TestSerializeRowsDwellTimeEvent(t *testing.T) {
	now := timestamppb.Now()
	event := &pb.DwellTimeEvent{
		Attributes:       makeAttrs("MBTA", "v2", "Green", "S2"),
		ArrivalTime:      now,
		DepartureTime:    timestamppb.New(now.AsTime().Add(45 * time.Second)),
		DwellTimeSeconds: 45,
	}
	buf, err := serializeRows([]EventRow{toEventRow(event)})
	if err != nil {
		t.Fatalf("serializeRows: %v", err)
	}

	rows := readParquetRows(t, buf)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].DwellTimeSeconds != 45 {
		t.Errorf("dwell_time_seconds: want 45, got %d", rows[0].DwellTimeSeconds)
	}
}

func TestSerializeRowsTravelTimeEvent(t *testing.T) {
	now := timestamppb.Now()
	event := &pb.TravelTimeEvent{
		Attributes:        makeAttrs("MBTA", "v3", "Blue", "S3"),
		StartTime:         now,
		EndTime:           timestamppb.New(now.AsTime().Add(5 * time.Minute)),
		TravelTimeSeconds: 300,
	}
	buf, err := serializeRows([]EventRow{toEventRow(event)})
	if err != nil {
		t.Fatalf("serializeRows: %v", err)
	}

	rows := readParquetRows(t, buf)
	if rows[0].TravelTimeSeconds != 300 {
		t.Errorf("travel_time_seconds: want 300, got %d", rows[0].TravelTimeSeconds)
	}
}

func TestSerializeRowsHeadwayTimeEvent(t *testing.T) {
	event := &pb.HeadwayTimeEvent{
		Attributes:         makeAttrs("MBTA", "v4", "Orange", "S4"),
		LeadingVehicleId:   "v3",
		FollowingVehicleId: "v4",
		HeadwaySeconds:     120,
	}
	buf, err := serializeRows([]EventRow{toEventRow(event)})
	if err != nil {
		t.Fatalf("serializeRows: %v", err)
	}

	rows := readParquetRows(t, buf)
	row := rows[0]
	if row.HeadwaySeconds != 120 {
		t.Errorf("headway_seconds: want 120, got %d", row.HeadwaySeconds)
	}
	if row.LeadingVehicleID != "v3" {
		t.Errorf("leading_vehicle_id: want v3, got %q", row.LeadingVehicleID)
	}
	if row.FollowingVehicleID != "v4" {
		t.Errorf("following_vehicle_id: want v4, got %q", row.FollowingVehicleID)
	}
}

func TestSerializeRowsMultipleRows(t *testing.T) {
	const n = 50
	input := make([]EventRow, n)
	for i := range n {
		input[i] = toEventRow(&pb.StopEvent{
			Attributes:    makeAttrs("MBTA", fmt.Sprintf("v%d", i), "Red", "S1"),
			StopEventType: pb.StopEvent_DEPARTURE,
		})
	}

	buf, err := serializeRows(input)
	if err != nil {
		t.Fatalf("serializeRows: %v", err)
	}

	rows := readParquetRows(t, buf)
	if len(rows) != n {
		t.Errorf("expected %d rows, got %d", n, len(rows))
	}
}

func TestObjectKey(t *testing.T) {
	sink := &ParquetSink{
		prefix:    "transit/events",
		eventType: "stop_events",
	}
	ts, _ := time.Parse(time.RFC3339, "2024-03-15T14:30:00Z")
	key := sink.objectKey(ts)
	want := "transit/events/stop_events/year=2024/month=03/day=15/20240315_143000.parquet"
	if key != want {
		t.Errorf("objectKey:\n want %q\n  got %q", want, key)
	}
}

func TestObjectKeyNoPrefix(t *testing.T) {
	sink := &ParquetSink{eventType: "dwell_time_events"}
	ts, _ := time.Parse(time.RFC3339, "2024-01-05T00:00:00Z")
	key := sink.objectKey(ts)
	want := "dwell_time_events/year=2024/month=01/day=05/20240105_000000.parquet"
	if key != want {
		t.Errorf("objectKey:\n want %q\n  got %q", want, key)
	}
}
