package sources

import (
	"context"
	"fmt"

	"github.com/fjlanasa/tpm-go/config"
	"github.com/reugn/go-streams"
)

type Source interface {
	streams.Source
}

func NewSource(ctx context.Context, cfg config.SourceConfig, connectors map[config.ID]chan any) (Source, error) {
	switch cfg.Type {
	case config.SourceTypeHTTP:
		return NewHTTPSource(ctx, cfg.HTTP)
	case config.SourceTypeConnector:
		source := NewConnectorSource(ctx, cfg.Connector, connectors)
		return source, nil
	}
	return nil, fmt.Errorf("invalid source type: %s", cfg.Type)
}
