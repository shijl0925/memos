package getter

import (
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/url"
	"time"
)

var defaultHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
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
		return nil, fmt.Errorf("unsupported outbound url scheme")
	}
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return nil, fmt.Errorf("missing outbound url host")
	}

	if ip := net.ParseIP(hostname); ip != nil {
		if !isPublicIP(ip) {
			return nil, fmt.Errorf("outbound url host is not public")
		}
		return parsedURL, nil
	}

	ips, err := net.LookupIP(hostname)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("outbound url host resolution returned no address")
	}
	for _, ip := range ips {
		if !isPublicIP(ip) {
			return nil, fmt.Errorf("outbound url host resolved to a non-public address")
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
