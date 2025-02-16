package feed_message_events

import (
	"context"
	"log"
	"net/http"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	pb "github.com/fjlanasa/tpm-go/api/v1/events"
	"github.com/fjlanasa/tpm-go/internal/config"
	"github.com/fjlanasa/tpm-go/internal/sources"
	"github.com/reugn/go-streams"
	"github.com/reugn/go-streams/flow"
)

type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

type FeedMessageSource struct {
	streams.Source
	config     config.SourceConfig
	feedSource sources.Source
	in         chan any
	out        chan any
	ctx        context.Context
}

func NewFeedMessageSource(ctx context.Context, cfg config.SourceConfig) *FeedMessageSource {
	feedSource, err := sources.NewSource(ctx, cfg, func() *gtfs.FeedMessage {
		return &gtfs.FeedMessage{}
	}, make(map[config.ID]chan any))
	if err != nil {
		log.Fatal(err)
	}
	source := &FeedMessageSource{ctx: ctx, config: cfg, feedSource: feedSource, out: make(chan any)}
	go source.init()
	return source
}

func (v *FeedMessageSource) init() {
	feedSourceOutlet := v.feedSource.Out()
	for {
		select {
		case <-v.ctx.Done():
			return
		case feed := <-feedSourceOutlet:
			feedMessage := feed.(*gtfs.FeedMessage)
			v.out <- &pb.FeedMessageEvent{
				AgencyId:    v.config.AgencyID,
				FeedMessage: feedMessage,
			}
		}
	}
}

func (v *FeedMessageSource) Via(operator streams.Flow) streams.Flow {
	flow.DoStream(v, operator)
	return operator
}

func (v *FeedMessageSource) In() chan<- any {
	return v.in
}

func (v *FeedMessageSource) Out() <-chan any {
	return v.out
}
