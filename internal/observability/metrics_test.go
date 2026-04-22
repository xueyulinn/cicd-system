package observability

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"testing"
)

func TestSchemeFromRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
		want string
	}{
		{
			name: "nil request defaults to http",
			req:  nil,
			want: "http",
		},
		{
			name: "url scheme takes precedence",
			req: &http.Request{
				URL: &url.URL{Scheme: "http"},
				TLS: &tls.ConnectionState{},
			},
			want: "http",
		},
		{
			name: "url scheme https",
			req: &http.Request{
				URL: &url.URL{Scheme: "https"},
			},
			want: "https",
		},
		{
			name: "tls implies https when scheme missing",
			req: &http.Request{
				URL: &url.URL{},
				TLS: &tls.ConnectionState{},
			},
			want: "https",
		},
		{
			name: "missing scheme and tls defaults to http",
			req: &http.Request{
				URL: &url.URL{},
			},
			want: "http",
		},
		{
			name: "nil url with tls defaults to https",
			req: &http.Request{
				TLS: &tls.ConnectionState{},
			},
			want: "https",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := schemeFromRequest(tt.req); got != tt.want {
				t.Fatalf("schemeFromRequest() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseProto(t *testing.T) {
	tests := []struct {
		name        string
		proto       string
		wantName    string
		wantVersion string
	}{
		{
			name:        "http 1.1",
			proto:       "HTTP/1.1",
			wantName:    "http",
			wantVersion: "1.1",
		},
		{
			name:        "http2",
			proto:       "HTTP/2.0",
			wantName:    "http",
			wantVersion: "2.0",
		},
		{
			name:        "custom protocol",
			proto:       "GRPC/2",
			wantName:    "grpc",
			wantVersion: "2",
		},
		{
			name:        "trims spaces and lowercases",
			proto:       "  HTTP/1.0  ",
			wantName:    "http",
			wantVersion: "1.0",
		},
		{
			name:        "missing separator falls back",
			proto:       "HTTP2",
			wantName:    "http",
			wantVersion: "",
		},
		{
			name:        "empty falls back",
			proto:       "",
			wantName:    "http",
			wantVersion: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotVersion := parseProto(tt.proto)
			if gotName != tt.wantName || gotVersion != tt.wantVersion {
				t.Fatalf("parseProto(%q) = (%q, %q), want (%q, %q)", tt.proto, gotName, gotVersion, tt.wantName, tt.wantVersion)
			}
		})
	}
}

func TestServerFromRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
		want string
	}{
		{
			name: "extract hostname from absolute URL",
			req: &http.Request{
				URL: &url.URL{
					Scheme: "https",
					Host:   "api.example.com:8443",
				},
			},
			want: "api.example.com",
		},
		{
			name: "empty when URL missing",
			req:  &http.Request{},
			want: "",
		},
		{
			name: "empty when request is nil",
			req:  nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := serverFromRequest(tt.req)
			if got != tt.want {
				t.Fatalf("serverFromRequest() = %q, want %q", got, tt.want)
			}
		})
	}
}
