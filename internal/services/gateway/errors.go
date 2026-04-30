package gateway

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	errUpstreamTimeout = errors.New("upstream timeout")
)

func upstreamServiceTimeout(message string) error {
	return fmt.Errorf("%w: %s", errUpstreamTimeout, message)
}

func classifyError(err error) (int, string, string) {
	switch {
	case errors.Is(err, errUpstreamTimeout):
		return http.StatusGatewayTimeout, "upstream_timeout", err.Error()
	default:
		return http.StatusBadGateway, "upstream_unavailable", err.Error()
	}
}
