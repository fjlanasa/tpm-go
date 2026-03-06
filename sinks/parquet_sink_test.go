package sinks

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"io"
	"testing"
	"time"

	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// readCSVRows decompresses and parses a gzipped CSV produced by serializeRows.
func readCSVRows(t *testing.T, data []byte) (header []string, records [][]string) {
	t.Helper()
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gr.Close()
	raw, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("read gzip: %v", err)
	}
	r := csv.NewReader(bytes.NewReader(raw))
	all, err := r.ReadAll()
	if err != nil {
		t.Fatalf("csv.ReadAll: %v", err)
	}
	if len(all) == 0 {
		t.Fatal("empty CSV")
	}
	return all[0], all[1:]
}

// colIndex returns the index of col in header, or -1 if not found.
func colIndex(header []string, col string) int {
	for i, h := range header {
		if h == col {
			return i
		}
	}
	return -1
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

	header, records := readCSVRows(t, buf)
	if len(records) != 1 {
		t.Fatalf("expected 1 data row, got %d", len(records))
	}
	row := records[0]

	check := func(col, want string) {
		t.Helper()
		i := colIndex(header, col)
		if i < 0 {
			t.Errorf("column %q not found in header", col)
			return
		}
		if row[i] != want {
			t.Errorf("col %q: want %q, got %q", col, want, row[i])
		}
	}

	check("agency_id", "MBTA")
	check("vehicle_id", "v1")
	check("route_id", "Red")
	check("stop_id", "S1")
	check("event_type", "StopEvent")
	check("stop_event_type", "ARRIVAL")
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

	header, records := readCSVRows(t, buf)
	if len(records) != 1 {
		t.Fatalf("expected 1 data row, got %d", len(records))
	}
	i := colIndex(header, "dwell_time_seconds")
	if i < 0 {
		t.Fatal("column dwell_time_seconds not found")
	}
	if records[0][i] != "45" {
		t.Errorf("dwell_time_seconds: want 45, got %q", records[0][i])
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

	header, records := readCSVRows(t, buf)
	i := colIndex(header, "travel_time_seconds")
	if i < 0 {
		t.Fatal("column travel_time_seconds not found")
	}
	if records[0][i] != "300" {
		t.Errorf("travel_time_seconds: want 300, got %q", records[0][i])
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

	header, records := readCSVRows(t, buf)
	checkCol := func(col, want string) {
		t.Helper()
		i := colIndex(header, col)
		if i < 0 {
			t.Errorf("column %q not found", col)
			return
		}
		if records[0][i] != want {
			t.Errorf("%s: want %q, got %q", col, want, records[0][i])
		}
	}
	checkCol("headway_seconds", "120")
	checkCol("leading_vehicle_id", "v3")
	checkCol("following_vehicle_id", "v4")
}

func TestSerializeRowsMultipleRows(t *testing.T) {
	const n = 50
	rows := make([]EventRow, n)
	for i := range n {
		rows[i] = toEventRow(&pb.StopEvent{
			Attributes:    makeAttrs("MBTA", fmt.Sprintf("v%d", i), "Red", "S1"),
			StopEventType: pb.StopEvent_DEPARTURE,
		})
	}

	buf, err := serializeRows(rows)
	if err != nil {
		t.Fatalf("serializeRows: %v", err)
	}

	_, records := readCSVRows(t, buf)
	if len(records) != n {
		t.Errorf("expected %d rows, got %d", n, len(records))
	}
}

func TestEventRowColumns(t *testing.T) {
	cols := eventRowColumns()
	if len(cols) == 0 {
		t.Fatal("no columns returned")
	}
	// Verify a few expected column names.
	expected := []string{"event_type", "agency_id", "vehicle_id", "stop_event_type", "headway_seconds"}
	colSet := make(map[string]bool, len(cols))
	for _, c := range cols {
		colSet[c] = true
	}
	for _, e := range expected {
		if !colSet[e] {
			t.Errorf("expected column %q not found in %v", e, cols)
		}
	}
}

func TestObjectKey(t *testing.T) {
	sink := &ParquetSink{
		prefix:    "transit/events",
		eventType: "stop_events",
	}
	ts, _ := time.Parse(time.RFC3339, "2024-03-15T14:30:00Z")
	key := sink.objectKey(ts)
	want := "transit/events/stop_events/year=2024/month=03/day=15/20240315_143000.csv.gz"
	if key != want {
		t.Errorf("objectKey:\n want %q\n  got %q", want, key)
	}
}

func TestObjectKeyNoPrefix(t *testing.T) {
	sink := &ParquetSink{eventType: "dwell_time_events"}
	ts, _ := time.Parse(time.RFC3339, "2024-01-05T00:00:00Z")
	key := sink.objectKey(ts)
	want := "dwell_time_events/year=2024/month=01/day=05/20240105_000000.csv.gz"
	if key != want {
		t.Errorf("objectKey:\n want %q\n  got %q", want, key)
	}
}
