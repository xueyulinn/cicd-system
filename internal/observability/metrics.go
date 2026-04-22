package observability

import (
	"go.opentelemetry.io/otel"
)

var meter = otel.Meter("github.com/xueyulinn/cicd-system/internal/observability")

// HTTP OTel metrics.
// var (
// 	httpServerRequestDuration metric.Float64Histogram
// 	httpClientRequestDuration metric.Float64Histogram
// )

func metricsBootStrap() {
	
}

// func recordHttpServerRequestDuration(rec *statusRecorder, r *http.Request, elapsed float64) {
// 	ctx := context.Background()
// 	code := http.StatusInternalServerError
// 	method := ""
// 	route := ""
// 	protoName, protoVersion := "http", ""
// 	scheme := schemeFromRequest(r)

// 	if r != nil {
// 		ctx = r.Context()
// 		method = strings.TrimSpace(r.Method)
// 		if method == "" {
// 			method = "_OTHER"
// 		}
// 		if r.URL != nil {
// 			route = normalizePath(r.URL.Path)
// 		}
// 		protoName, protoVersion = parseProto(r.Proto)
// 	}
// 	if rec != nil {
// 		code = rec.code
// 	}

// 	attrs := []attribute.KeyValue{
// 		attribute.String("http.request.method", method),
// 		attribute.String("url.scheme", scheme),
// 		attribute.Int("http.response.status_code", code),
// 		attribute.String("http.route", route),
// 		attribute.String("network.protocol.name", protoName),
// 		attribute.String("network.protocol.version", protoVersion),
// 	}
// 	if code >= http.StatusBadRequest {
// 		attrs = append(attrs, attribute.String("error.type", strconv.Itoa(code)))
// 	}

// 	httpServerRequestDuration.Record(ctx, elapsed, metric.WithAttributes(attrs...))
// }

// func recordHttpClientRequestDuration(req *http.Request, resp *http.Response, err error, elapsed float64) {
// 	reqForAttrs := requestForClientAttrs(req, resp)
// 	ctx := context.Background()
// 	if reqForAttrs != nil {
// 		ctx = reqForAttrs.Context()
// 	}

// 	protoName, protoVersion := "http", ""
// 	if resp != nil {
// 		protoName, protoVersion = parseProto(resp.Proto)
// 	} else if reqForAttrs != nil {
// 		protoName, protoVersion = parseProto(reqForAttrs.Proto)
// 	}

// 	method := "_OTHER"
// 	if reqForAttrs != nil && strings.TrimSpace(reqForAttrs.Method) != "" {
// 		method = strings.TrimSpace(reqForAttrs.Method)
// 	}

// 	serverAddr, serverPort := serverAddressPort(reqForAttrs)

// 	attrs := []attribute.KeyValue{
// 		attribute.String("http.request.method", method),
// 		attribute.String("network.protocol.name", protoName),
// 		attribute.String("network.protocol.version", protoVersion),
// 	}
// 	if serverAddr != "" {
// 		attrs = append(attrs, attribute.String("server.address", serverAddr))
// 	}
// 	if serverPort > 0 {
// 		attrs = append(attrs, attribute.Int("server.port", serverPort))
// 	}
// 	if resp != nil {
// 		attrs = append(attrs, attribute.Int("http.response.status_code", resp.StatusCode))
// 		if resp.StatusCode >= http.StatusBadRequest {
// 			attrs = append(attrs, attribute.String("error.type", strconv.Itoa(resp.StatusCode)))
// 		}
// 	} else if err != nil {
// 		attrs = append(attrs, attribute.String("error.type", classifyHTTPClientError(err)))
// 	}

// 	httpClientRequestDuration.Record(ctx, elapsed, metric.WithAttributes(attrs...))
// }

// func requestForClientAttrs(req *http.Request, resp *http.Response) *http.Request {
// 	if req != nil {
// 		return req
// 	}
// 	if resp != nil {
// 		return resp.Request
// 	}
// 	return nil
// }

// func serverFromRequest(r *http.Request) string {
// 	addr, _ := serverAddressPort(r)
// 	return addr
// }

// func serverAddressPort(r *http.Request) (string, int) {
// 	if r == nil || r.URL == nil {
// 		return "", 0
// 	}

// 	address := r.URL.Hostname()
// 	port := 0
// 	if p := r.URL.Port(); p != "" {
// 		if n, err := strconv.Atoi(p); err == nil && n > 0 {
// 			port = n
// 		}
// 	}
// 	if port == 0 {
// 		port = defaultPortForScheme(schemeFromRequest(r))
// 	}
// 	return address, port
// }

// func defaultPortForScheme(scheme string) int {
// 	switch strings.ToLower(strings.TrimSpace(scheme)) {
// 	case "https":
// 		return 443
// 	case "http":
// 		return 80
// 	default:
// 		return 0
// 	}
// }

// func schemeFromRequest(r *http.Request) string {
// 	if r == nil {
// 		return "http"
// 	}
// 	if r.URL != nil && r.URL.Scheme != "" {
// 		return r.URL.Scheme
// 	}
// 	if r.TLS != nil {
// 		return "https"
// 	}
// 	return "http"
// }

// func parseProto(proto string) (string, string) {
// 	protoName := "http"
// 	protoVersion := ""
// 	if parts := strings.SplitN(strings.ToLower(strings.TrimSpace(proto)), "/", 2); len(parts) == 2 {
// 		protoName = parts[0]
// 		protoVersion = parts[1]
// 	}
// 	return protoName, protoVersion
// }

// func classifyHTTPClientError(err error) string {
// 	var timeoutErr interface{ Timeout() bool }
// 	if errors.As(err, &timeoutErr) && timeoutErr.Timeout() {
// 		return "timeout"
// 	}
// 	return "transport_error"
// }

// // normalizePath keeps known paths and collapses the rest to avoid high cardinality.
// func normalizePath(p string) string {
// 	switch p {
// 	case "/health", "/ready", "/metrics",
// 		"/validate", "/dryrun", "/run", "/report",
// 		"/execute", "/services":
// 		return p
// 	default:
// 		return "/other"
// 	}
// }
