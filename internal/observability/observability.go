package observability

import (
	"context"
	"log/slog"
)

// Bootstrap sets up logging, metrics, and tracing for a service.
// Returns a shutdown function that flushes pending traces.
func Bootstrap(ctx context.Context, serviceName string) (shutdown func(context.Context) error, err error) {
	RegisterMetrics()

	shutdown, err = setupOTel(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	slog.Info("observability initialized",
		slog.String("service", serviceName),
	)

	return shutdown, nil
}
