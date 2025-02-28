package sources

import (
	"context"

	"github.com/fjlanasa/tpm-go/config"
	streams "github.com/reugn/go-streams/extension"
)

func NewConnectorSource(ctx context.Context, cfg config.ConnectorConfig, connectors map[config.ID]chan any) Source {
	in := connectors[cfg.ID]
	return streams.NewChanSource(in)
}
