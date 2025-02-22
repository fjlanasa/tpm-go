package sources

import (
	"context"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/fjlanasa/tpm-go/config"
	"github.com/reugn/go-streams"
	"github.com/reugn/go-streams/flow"
	"google.golang.org/protobuf/proto"
)

type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

type HttpSource[T proto.Message] struct {
	ctx         context.Context
	cfg         config.HTTPSourceConfig
	client      HTTPClient
	out         chan any
	newResponse func() T
}

func NewHttpSource[T proto.Message](ctx context.Context, cfg config.HTTPSourceConfig, newResponse func() T) *HttpSource[T] {
	source := &HttpSource[T]{ctx: ctx, cfg: cfg, client: http.DefaultClient, out: make(chan any), newResponse: newResponse}
	go source.init()
	return source
}

func (s *HttpSource[T]) init() {
	duration, err := time.ParseDuration(s.cfg.Interval)
	if err != nil {
		log.Fatal(err)
	}
	ticker := time.NewTicker(duration)

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			resp, err := s.client.Get(s.cfg.URL)
			if err != nil {
				log.Fatal(err)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Fatal(err)
			}
			response := s.newResponse()
			err = proto.Unmarshal(body, response)
			if err != nil {
				log.Fatal(err)
			}
			s.out <- response
			resp.Body.Close()
		}
	}

}

func (s *HttpSource[T]) Via(operator streams.Flow) streams.Flow {
	flow.DoStream(s, operator)
	return operator
}

func (s *HttpSource[T]) Out() <-chan any {
	return s.out
}

func (s *HttpSource[T]) SetHTTPClient(client HTTPClient) {
	s.client = client
}
