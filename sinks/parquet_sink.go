// Package sinks contains sink implementations for the TPM-Go pipeline.
//
// # Parquet Sink
//
// ParquetSink buffers transit events and periodically writes Apache Parquet
// files to S3. The EventRow struct's "parquet" tags define the schema.
package sinks

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/parquet-go/parquet-go"

	"github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/config"
	"github.com/reugn/go-streams"
)

// EventRow is a flat struct representing any transit metric event.
// Columns that are not relevant to a given event type are left as zero values.
// This layout is directly queryable by Athena, DuckDB, BigQuery, etc.
type EventRow struct {
	// Common
	EventType            string `parquet:"event_type"`
	AgencyID             string `parquet:"agency_id"`
	VehicleID            string `parquet:"vehicle_id"`
	RouteID              string `parquet:"route_id"`
	StopID               string `parquet:"stop_id"`
	OriginStopID         string `parquet:"origin_stop_id"`
	DestinationStopID    string `parquet:"destination_stop_id"`
	DirectionID          string `parquet:"direction_id"`
	Direction            string `parquet:"direction"`
	DirectionDestination string `parquet:"direction_destination"`
	ParentStation        string `parquet:"parent_station"`
	StopSequence         int32  `parquet:"stop_sequence"`
	StopStatus           int32  `parquet:"stop_status"`
	TripID               string `parquet:"trip_id"`
	ServiceDate          string `parquet:"service_date"`
	Timestamp            int64  `parquet:"timestamp"` // Unix seconds; 0 if unset

	// StopEvent
	StopEventType string `parquet:"stop_event_type"`

	// DwellTimeEvent
	ArrivalTime      int64 `parquet:"arrival_time"`   // Unix seconds
	DepartureTime    int64 `parquet:"departure_time"` // Unix seconds
	DwellTimeSeconds int32 `parquet:"dwell_time_seconds"`

	// TravelTimeEvent
	StartTime         int64 `parquet:"start_time"` // Unix seconds
	EndTime           int64 `parquet:"end_time"`   // Unix seconds
	TravelTimeSeconds int32 `parquet:"travel_time_seconds"`

	// HeadwayTimeEvent
	LeadingVehicleID   string `parquet:"leading_vehicle_id"`
	FollowingVehicleID string `parquet:"following_vehicle_id"`
	HeadwaySeconds     int32  `parquet:"headway_seconds"`
}

// ParquetSink buffers transit events and periodically writes files to S3.
// One instance handles one event type; configure event_type in YAML to set the
// partition path segment (e.g. "stop_events").
//
// S3 key pattern:
//
//	{prefix}/{event_type}/year={Y}/month={MM}/day={DD}/{timestamp}.parquet
type ParquetSink struct {
	streams.Sink
	s3Client      *s3.Client
	bucketName    string
	prefix        string
	eventType     string
	in            chan any
	maxBatchSize  int
	flushInterval time.Duration
	ctx           context.Context
}

func NewParquetSink(ctx context.Context, cfg config.SinkConfig) (*ParquetSink, error) {
	pc := cfg.Parquet
	if pc.BucketName == "" {
		return nil, fmt.Errorf("parquet sink: bucket_name is required")
	}

	s3Client, err := buildS3Client(ctx, pc)
	if err != nil {
		return nil, fmt.Errorf("parquet sink: build s3 client: %w", err)
	}

	maxBatchSize := cfg.MaxBatchSize
	if maxBatchSize <= 0 {
		maxBatchSize = 10_000
	}
	flushInterval := cfg.FlushInterval
	if flushInterval <= 0 {
		flushInterval = time.Hour
	}

	sink := &ParquetSink{
		s3Client:      s3Client,
		bucketName:    pc.BucketName,
		prefix:        strings.TrimRight(pc.Prefix, "/"),
		eventType:     pc.EventType,
		in:            make(chan any),
		maxBatchSize:  maxBatchSize,
		flushInterval: flushInterval,
		ctx:           ctx,
	}
	go sink.doSink(ctx)
	return sink, nil
}

func (s *ParquetSink) In() chan<- any {
	return s.in
}

func (s *ParquetSink) doSink(ctx context.Context) {
	batch := make([]EventRow, 0, s.maxBatchSize)
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if len(batch) > 0 {
				s.flush(ctx, batch)
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
				slog.Warn("parquet sink: unexpected message type", "type", fmt.Sprintf("%T", msg))
				continue
			}
			batch = append(batch, toEventRow(event))
			if len(batch) >= s.maxBatchSize {
				s.flush(ctx, batch)
				batch = batch[:0]
			}
		}
	}
}

func (s *ParquetSink) flush(ctx context.Context, batch []EventRow) {
	if len(batch) == 0 {
		return
	}

	buf, err := serializeRows(batch)
	if err != nil {
		slog.Error("parquet sink: serialize rows", "err", err)
		return
	}

	key := s.objectKey(time.Now().UTC())
	if _, err := s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(buf),
		ContentType: aws.String("application/vnd.apache.parquet"),
	}); err != nil {
		slog.Error("parquet sink: put object", "key", key, "err", err)
		return
	}
	slog.Info("parquet sink: flushed", "key", key, "rows", len(batch), "bytes", len(buf))
}

func (s *ParquetSink) objectKey(t time.Time) string {
	name := fmt.Sprintf("%s.parquet", t.Format("20060102_150405"))
	parts := []string{}
	if s.prefix != "" {
		parts = append(parts, s.prefix)
	}
	if s.eventType != "" {
		parts = append(parts, s.eventType)
	}
	parts = append(parts,
		fmt.Sprintf("year=%d", t.Year()),
		fmt.Sprintf("month=%02d", t.Month()),
		fmt.Sprintf("day=%02d", t.Day()),
		name,
	)
	return strings.Join(parts, "/")
}

// serializeRows writes rows as an Apache Parquet file.
func serializeRows(rows []EventRow) ([]byte, error) {
	var buf bytes.Buffer
	w := parquet.NewGenericWriter[EventRow](&buf)
	if _, err := w.Write(rows); err != nil {
		_ = w.Close()
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// toEventRow converts any events.Event into a flat EventRow.
func toEventRow(e events.Event) EventRow {
	a := e.GetAttributes()
	row := EventRow{
		EventType:            strings.TrimPrefix(fmt.Sprintf("%T", e), "*events."),
		AgencyID:             a.GetAgencyId(),
		VehicleID:            a.GetVehicleId(),
		RouteID:              a.GetRouteId(),
		StopID:               a.GetStopId(),
		OriginStopID:         a.GetOriginStopId(),
		DestinationStopID:    a.GetDestinationStopId(),
		DirectionID:          a.GetDirectionId(),
		Direction:            a.GetDirection(),
		DirectionDestination: a.GetDirectionDestination(),
		ParentStation:        a.GetParentStation(),
		StopSequence:         a.GetStopSequence(),
		StopStatus:           int32(a.GetStopStatus()),
		TripID:               a.GetTripId(),
		ServiceDate:          a.GetServiceDate(),
	}
	if a.GetTimestamp() != nil {
		row.Timestamp = a.GetTimestamp().AsTime().Unix()
	}

	switch ev := e.(type) {
	case *events.StopEvent:
		row.StopEventType = ev.GetStopEventType().String()

	case *events.DwellTimeEvent:
		if ev.GetArrivalTime() != nil {
			row.ArrivalTime = ev.GetArrivalTime().AsTime().Unix()
		}
		if ev.GetDepartureTime() != nil {
			row.DepartureTime = ev.GetDepartureTime().AsTime().Unix()
		}
		row.DwellTimeSeconds = ev.GetDwellTimeSeconds()

	case *events.TravelTimeEvent:
		if ev.GetStartTime() != nil {
			row.StartTime = ev.GetStartTime().AsTime().Unix()
		}
		if ev.GetEndTime() != nil {
			row.EndTime = ev.GetEndTime().AsTime().Unix()
		}
		row.TravelTimeSeconds = ev.GetTravelTimeSeconds()

	case *events.HeadwayTimeEvent:
		row.LeadingVehicleID = ev.GetLeadingVehicleId()
		row.FollowingVehicleID = ev.GetFollowingVehicleId()
		row.HeadwaySeconds = ev.GetHeadwaySeconds()
	}
	return row
}

// buildS3Client creates an AWS S3 client from the sink config.
// If AWSAccessKey is set, static credentials are used; otherwise the default
// credential chain (env vars, ~/.aws/credentials, IAM role, etc.) is used.
func buildS3Client(ctx context.Context, pc config.ParquetSinkConfig) (*s3.Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{}

	if pc.AWSAccessKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(pc.AWSAccessKey, pc.AWSSecretKey, ""),
		))
	}
	if pc.AWSRegion != "" {
		opts = append(opts, awsconfig.WithRegion(pc.AWSRegion))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}

	s3Opts := []func(*s3.Options){}
	if pc.AWSEndpointURL != "" {
		endpointURL := pc.AWSEndpointURL
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpointURL)
			o.UsePathStyle = true // required for MinIO / LocalStack
		})
	}

	return s3.NewFromConfig(awsCfg, s3Opts...), nil
}
