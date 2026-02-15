package sources

import (
	"context"
	"fmt"
	"io"
	"log/slog"
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
	ctx      context.Context
	cfg      config.HTTPSourceConfig
	client   HTTPClient
	out      chan any
	interval time.Duration
}

func NewHttpSource(ctx context.Context, cfg config.HTTPSourceConfig, client ...HTTPClient) (*HttpSource, error) {
	var c HTTPClient = http.DefaultClient
	if len(client) > 0 && client[0] != nil {
		c = client[0]
	}
	duration, err := time.ParseDuration(cfg.Interval)
	if err != nil {
		return nil, fmt.Errorf("invalid polling interval %q: %w", cfg.Interval, err)
	}
	source := &HttpSource{ctx: ctx, cfg: cfg, client: c, out: make(chan any), interval: duration}
	go source.init()
	return source, nil
}

func (s *HttpSource) init() {
	ticker := time.NewTicker(s.interval)

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			resp, err := s.client.Get(s.cfg.URL)
			if err != nil {
				slog.Error("http source: request failed", "url", s.cfg.URL, "error", err)
				continue
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				slog.Error("http source: failed to read response body", "url", s.cfg.URL, "error", err)
				continue
			}
			s.out <- body
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

