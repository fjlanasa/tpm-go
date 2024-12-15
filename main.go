package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	"github.com/fjlanasa/tpm-go/api/v1/events"
	tpmflow "github.com/fjlanasa/tpm-go/flow"
	"github.com/fjlanasa/tpm-go/source"
	"github.com/reugn/go-streams/extension"
	"github.com/reugn/go-streams/flow"
)

type VehiclePosition struct {
	VehiclePosition *gtfs.VehiclePosition
	Source          int
}

func main() {
	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create channel for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create output channel with buffer
	vpOutChan := make(chan any)
	stopEventOutChan := make(chan any)
	headwayOutChan := make(chan any)
	dwellOutChan := make(chan any)
	// Create source
	go func() {
		vpSource := flow.FanOut(source.NewVehiclePositionsSource("MBTA", "https://cdn.mbta.com/realtime/VehiclePositions.pb", 1*time.Second), 2)
		// Send vehicle position to sink
		go vpSource[0].Via(flow.NewPassThrough()).To(extension.NewChanSink(vpOutChan))
		seSource := flow.FanOut(vpSource[1].Via(tpmflow.NewStopEventFlow()), 3)
		go seSource[0].Via(flow.NewPassThrough()).Via(flow.NewFilter(func(stopEvent *events.StopEvent) bool {
			return stopEvent.RouteId == "Red"
		}, 3)).To(extension.NewChanSink(stopEventOutChan))
		go seSource[1].Via(tpmflow.NewHeadwayEventFlow()).To(extension.NewChanSink(headwayOutChan))
		go seSource[2].Via(flow.NewFilter(func(stopEvent *events.StopEvent) bool {
			return stopEvent.RouteId == "Red"
		}, 3)).Via(tpmflow.NewDwellEventFlow()).To(extension.NewChanSink(dwellOutChan))
	}()
	fmt.Println("Waiting for messages...")
	// Process messages
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-vpOutChan:
				if event == nil {
					continue
				}
				vp, ok := event.(*gtfs.VehiclePosition)
				if !ok || vp == nil {
					continue
				}
				// fmt.Printf("Stop Event: Vehicle ID: %s, Route ID: %s, Stop ID: %s, Event Type: %d\n", vp.GetVehicle().GetId(), vp.GetTrip().GetRouteId(), vp.GetStopId(), vp.GetTimestamp())
			case event := <-stopEventOutChan:
				if event == nil {
					continue
				}
				stopEvent, ok := event.(*events.StopEvent)
				if !ok || stopEvent == nil {
					continue
				}
				fmt.Printf("Stop Event: Vehicle ID: %s, Route ID: %s, Stop ID: %s, Event Type: %s\n", stopEvent.VehicleId, stopEvent.RouteId, stopEvent.StopId, stopEvent.EventType)
			case event := <-dwellOutChan:
				if event == nil {
					continue
				}
				dwellEvent, ok := event.(*events.DwellTimeEvent)
				if !ok || dwellEvent == nil {
					continue
				}
				fmt.Printf("Dwell Event: Vehicle ID: %s, Route ID: %s, Stop ID: %s, Seconds: %d\n", dwellEvent.VehicleId, dwellEvent.RouteId, dwellEvent.StopId, dwellEvent.DwellTimeSeconds)
			case event := <-headwayOutChan:
				if event == nil {
					continue
				}
				headwayEvent, ok := event.(*events.HeadwayTimeEvent)
				if !ok || headwayEvent == nil {
					continue
				}
				fmt.Printf("Headway Event: Vehicle ID: %s, Route ID: %s, Stop ID: %s, Seconds: %d\n", headwayEvent.LeadingVehicleId, headwayEvent.RouteId, headwayEvent.StopId, headwayEvent.HeadwayTrunkSeconds)
			}
		}
	}()
	fmt.Println("Waiting for messages here...")

	// Wait for shutdown signal
	<-sigChan
	close(vpOutChan)
	close(stopEventOutChan)
	close(headwayOutChan)
	close(dwellOutChan)
	fmt.Println("\nShutting down...")
}
