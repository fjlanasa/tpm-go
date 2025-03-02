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
)

type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

type HttpSource struct {
	ctx    context.Context
	cfg    config.HTTPSourceConfig
	client HTTPClient
	out    chan any
}

func NewHttpSource(ctx context.Context, cfg config.HTTPSourceConfig) *HttpSource {
	source := &HttpSource{ctx: ctx, cfg: cfg, client: http.DefaultClient, out: make(chan any)}
	go source.init()
	return source
}

func (s *HttpSource) init() {
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
			s.out <- body
			resp.Body.Close()
		}
	}

}

func (s *HttpSource) Via(operator streams.Flow) streams.Flow {
	flow.DoStream(s, operator)
	return operator
}

func (s *HttpSource) Out() <-chan any {
	return s.out
}

func (s *HttpSource) SetHTTPClient(client HTTPClient) {
	s.client = client
}
