package getter

import (
	"context"
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/url"
	"time"
)

var defaultHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			resolvedIPs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			if len(resolvedIPs) == 0 {
				return nil, fmt.Errorf("outbound URL host resolution returned no address")
			}
			for _, resolvedIP := range resolvedIPs {
				if !isPublicIP(resolvedIP.IP) {
					return nil, fmt.Errorf("outbound URL host resolved to a non-public address")
				}
			}
			dialer := &net.Dialer{}
			return dialer.DialContext(ctx, network, net.JoinHostPort(resolvedIPs[0].IP.String(), port))
		},
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after too many redirects")
		}
		_, err := validateOutboundURL(req.URL.String())
		return err
	},
}

func getMediatype(response *http.Response) (string, error) {
	contentType := response.Header.Get("content-type")
	mediatype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", err
	}
	return mediatype, nil
}

func validateOutboundURL(urlStr string) (*url.URL, error) {
	parsedURL, err := url.ParseRequestURI(urlStr)
	if err != nil {
		return nil, err
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported outbound URL scheme")
	}
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return nil, fmt.Errorf("missing outbound URL host")
	}

	if ip := net.ParseIP(hostname); ip != nil {
		if !isPublicIP(ip) {
			return nil, fmt.Errorf("outbound URL host is not public")
		}
	}
	return parsedURL, nil
}

func isPublicIP(ip net.IP) bool {
	return !(ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified())
}
