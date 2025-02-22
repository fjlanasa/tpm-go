package feed_message_events

import (
	"context"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/config"
	"github.com/reugn/go-streams"
)

type FeedMessageFlow struct {
	agencyID config.ID
	in       chan any
	out      chan any
}

func NewFeedMessageFlow(ctx context.Context, agencyID config.ID) *FeedMessageFlow {
	flow := &FeedMessageFlow{agencyID: agencyID, in: make(chan any), out: make(chan any)}
	go flow.doStream(ctx)
	return flow
}

func (f *FeedMessageFlow) doStream(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-f.in:
			if !ok {
				close(f.out)
				return
			}
			f.out <- &pb.FeedMessageEvent{
				AgencyId:    string(f.agencyID),
				FeedMessage: event.(*gtfs.FeedMessage),
			}
		}
	}
}

func (f *FeedMessageFlow) In() chan<- any {
	return f.in
}

func (f *FeedMessageFlow) Out() <-chan any {
	return f.out
}

func (f *FeedMessageFlow) Via(flow streams.Flow) streams.Flow {
	go f.transmit(flow)
	return flow
}

func (f *FeedMessageFlow) To(sink streams.Sink) {
	go f.transmit(sink)
}

func (f *FeedMessageFlow) transmit(inlet streams.Inlet) {
	for element := range f.Out() {
		inlet.In() <- element
	}
	close(inlet.In())
}
