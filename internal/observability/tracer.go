package observability

import (
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationBaseScope = "github.com/xueyulinn/cicd-system"

// Tracer returns a tracer using a repository-scoped instrumentation name.
// Examples:
//   - Tracer("internal/mq")
//   - Tracer("internal/services/worker")
func Tracer(scope string) trace.Tracer {
	normalized := strings.TrimSpace(scope)
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.TrimPrefix(normalized, "/")

	if normalized == "" {
		return otel.Tracer(instrumentationBaseScope)
	}
	if strings.HasPrefix(normalized, instrumentationBaseScope) {
		return otel.Tracer(normalized)
	}
	return otel.Tracer(instrumentationBaseScope + "/" + normalized)
}
