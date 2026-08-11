package httpserver

import (
	"testing"
)

func TestHTTPDisplayURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		addr string
		want string
	}{
		{addr: ":8080", want: "http://localhost:8080"},
		{addr: "0.0.0.0:8080", want: "http://localhost:8080"},
		{addr: "[::]:8080", want: "http://localhost:8080"},
		{addr: "127.0.0.1:3000", want: "http://127.0.0.1:3000"},
		{addr: "localhost:8080", want: "http://localhost:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			t.Parallel()
			if got := httpDisplayURL(tt.addr); got != tt.want {
				t.Fatalf("httpDisplayURL(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}
