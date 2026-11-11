package v1

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Resolve and pin each connection to a checked address. Rechecking at dial time
// prevents DNS rebinding between URL validation and the actual connection.
func newSubscriptionHTTPClient() *http.Client {
	transport := &http.Transport{
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			if isLocalHostname(host) {
				return nil, fmt.Errorf("source URL host is not allowed")
			}
			addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			if len(addresses) == 0 {
				return nil, fmt.Errorf("source URL host has no addresses")
			}
			for _, address := range addresses {
				if isPrivateIP(address.IP) {
					return nil, fmt.Errorf("source URL host is not allowed")
				}
			}
			dialer := &net.Dialer{Timeout: 10 * time.Second}
			var dialErr error
			for _, address := range addresses {
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
				if err == nil {
					return conn, nil
				}
				dialErr = err
			}
			return nil, dialErr
		},
	}
	return &http.Client{
		Transport: transport,
		Timeout:   20 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many subscription redirects")
			}
			_, err := validateSubscriptionSourceURL(request.Context(), request.URL.String())
			return err
		},
	}
}
