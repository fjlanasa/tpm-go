package sources

import (
	"context"
	"fmt"

	"github.com/fjlanasa/tpm-go/config"
	"github.com/reugn/go-streams"
	"google.golang.org/protobuf/proto"
)

type Source interface {
	streams.Source
}

func NewSource[T proto.Message](ctx context.Context, cfg config.SourceConfig, new func() T, connectors map[config.ID]chan any) (Source, error) {
	switch cfg.Type {
	case config.SourceTypeHTTP:
		source := NewHttpSource[T](ctx, cfg.HTTP, new)
		return source, nil
	case config.SourceTypeConnector:
		source := NewConnectorSource(ctx, cfg.Connector, connectors)
		return source, nil
	}
	return nil, fmt.Errorf("invalid source type: %s", cfg.Type)
}
