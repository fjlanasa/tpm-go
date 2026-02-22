package processors

import (
	"context"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/config"
	"github.com/reugn/go-streams"
	"google.golang.org/protobuf/proto"
)

type FeedMessageProcessor struct {
	agencyID config.ID
	in       chan any
	out      chan any
}

func NewFeedMessageProcessor(ctx context.Context, agencyID config.ID) *FeedMessageProcessor {
	flow := &FeedMessageProcessor{agencyID: agencyID, in: make(chan any), out: make(chan any)}
	go flow.doStream(ctx)
	return flow
}

func (f *FeedMessageProcessor) doStream(ctx context.Context) {
	defer close(f.out)

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-f.in:
			if !ok {
				return
			}

			var feedMessage *gtfs.FeedMessage
			switch v := event.(type) {
			case []uint8:
				msg := &gtfs.FeedMessage{}
				if err := proto.Unmarshal(v, msg); err != nil {
					continue
				}
				feedMessage = msg
			case *gtfs.FeedMessage:
				feedMessage = v
			default:
				continue
			}

			f.out <- &pb.FeedMessageEvent{
				AgencyId:    string(f.agencyID),
				FeedMessage: feedMessage,
			}
		}
	}
}

func (f *FeedMessageProcessor) In() chan<- any {
	return f.in
}

func (f *FeedMessageProcessor) Out() <-chan any {
	return f.out
}

func (f *FeedMessageProcessor) Via(flow streams.Flow) streams.Flow {
	go f.transmit(flow)
	return flow
}

func (f *FeedMessageProcessor) To(sink streams.Sink) {
	go f.transmit(sink)
}

func (f *FeedMessageProcessor) transmit(inlet streams.Inlet) {
	for element := range f.Out() {
		inlet.In() <- element
	}
	close(inlet.In())
}
