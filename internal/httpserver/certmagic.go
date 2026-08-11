package httpserver

import (
	"net/http"

	"github.com/caddyserver/certmagic"
)

func runHTTPS(handler http.Handler, config Config) error {
	if config.ACMEEmail != "" {
		certmagic.DefaultACME.Email = config.ACMEEmail
	}
	certmagic.DefaultACME.Agreed = true
	return certmagic.HTTPS(config.Domains, handler)
}
