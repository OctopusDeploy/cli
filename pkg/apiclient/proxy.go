package apiclient

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/spf13/viper"
	"golang.org/x/net/http/httpproxy"
)

// ProxySettings is the CLI's proxy configuration.
//
// Url takes precedence over the standard HTTP_PROXY/HTTPS_PROXY variables and
// applies to both schemes; when it is empty those variables are used instead.
// NO_PROXY is honoured either way. http, https, socks5 and socks5h proxies are
// supported, all by net/http itself.
type ProxySettings struct {
	Url      string
	Username string
	Password string
}

// ProxySettingsFromConfig reads the proxy settings from the viper config, which
// covers the ProxyUrl config file key and the OCTOPUS_PROXY environment variable.
// The credentials are deliberately read from the environment only, so that a
// proxy password is never written to the config file in plain text.
func ProxySettingsFromConfig() ProxySettings {
	return ProxySettings{
		Url:      viper.GetString(constants.ConfigProxyUrl),
		Username: os.Getenv(constants.EnvOctopusProxyUsername),
		Password: os.Getenv(constants.EnvOctopusProxyPassword),
	}
}

// ProxyFunc returns a function suitable for http.Transport.Proxy.
func (s ProxySettings) ProxyFunc() (func(*http.Request) (*url.URL, error), error) {
	config := httpproxy.FromEnvironment()
	if s.Url != "" {
		// httpproxy silently ignores a proxy address it cannot parse, so validate it here
		// to report a typo rather than quietly connecting directly.
		if _, err := parseProxyUrl(s.Url); err != nil {
			return nil, err
		}
		config.HTTPProxy = s.Url
		config.HTTPSProxy = s.Url
		config.CGI = false
	}

	proxyForUrl := config.ProxyFunc()
	return func(request *http.Request) (*url.URL, error) {
		proxyUrl, err := proxyForUrl(request.URL)
		if err != nil || proxyUrl == nil {
			return nil, err
		}
		return s.applyCredentials(proxyUrl), nil
	}, nil
}

// applyCredentials adds the configured proxy credentials, unless the proxy url
// already carries its own.
func (s ProxySettings) applyCredentials(proxyUrl *url.URL) *url.URL {
	if s.Username == "" || proxyUrl.User != nil {
		return proxyUrl
	}
	withCredentials := *proxyUrl
	withCredentials.User = url.UserPassword(s.Username, s.Password)
	return &withCredentials
}

// NewHttpTransport returns the transport the CLI uses to talk to Octopus. It is
// a clone of http.DefaultTransport so the standard defaults are kept, with the
// proxy resolution replaced by ours.
func NewHttpTransport(settings ProxySettings, insecureSkipVerify bool) (*http.Transport, error) {
	proxyFunc, err := settings.ProxyFunc()
	if err != nil {
		return nil, err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = proxyFunc
	if insecureSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return transport, nil
}

// RedactProxyUrl removes the password from a proxy url so it can be displayed.
func RedactProxyUrl(rawUrl string) string {
	if rawUrl == "" {
		return ""
	}
	parsed, err := parseProxyUrl(rawUrl)
	if err != nil {
		return "***" // can't parse it, so we can't tell whether it holds a password
	}
	if parsed.User == nil {
		return rawUrl
	}
	return parsed.Redacted()
}

// parseProxyUrl mirrors how net/http parses a proxy address: a bare "host:port"
// is treated as http.
func parseProxyUrl(rawUrl string) (*url.URL, error) {
	parsed, err := url.Parse(rawUrl)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		if withScheme, schemeErr := url.Parse("http://" + rawUrl); schemeErr == nil && withScheme.Host != "" {
			return withScheme, nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("invalid proxy url '%s': %w", rawUrl, err)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("invalid proxy url '%s': no host specified", rawUrl)
	}
	return parsed, nil
}
