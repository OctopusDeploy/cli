package apiclient_test

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

const octopusUrl = "https://octopus.example.com/api/"

// clearProxyEnvironment stops whatever the machine running the tests has configured
// from leaking into the expectations.
func clearProxyEnvironment(t *testing.T) {
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("http_proxy", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("https_proxy", "")
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")
}

func TestProxySettings_ProxyFunc(t *testing.T) {
	tests := []struct {
		name       string
		settings   apiclient.ProxySettings
		env        map[string]string
		requestUrl string
		wantProxy  string
	}{
		{
			name:       "no proxy configured at all",
			requestUrl: octopusUrl,
		},
		{
			name:       "HTTPS_PROXY is honoured with no explicit configuration",
			env:        map[string]string{"HTTPS_PROXY": "http://envproxy:3128"},
			requestUrl: octopusUrl,
			wantProxy:  "http://envproxy:3128",
		},
		{
			name:       "HTTP_PROXY is honoured for plain http requests",
			env:        map[string]string{"HTTP_PROXY": "http://envproxy:3128"},
			requestUrl: "http://octopus.example.com/api/",
			wantProxy:  "http://envproxy:3128",
		},
		{
			name:       "HTTPS_PROXY does not apply to plain http requests",
			env:        map[string]string{"HTTPS_PROXY": "http://envproxy:3128"},
			requestUrl: "http://octopus.example.com/api/",
		},
		{
			name:       "the configured proxy url wins over HTTPS_PROXY",
			settings:   apiclient.ProxySettings{Url: "http://configured:3128"},
			env:        map[string]string{"HTTPS_PROXY": "http://envproxy:3128"},
			requestUrl: octopusUrl,
			wantProxy:  "http://configured:3128",
		},
		{
			name:       "the configured proxy url applies to plain http requests too",
			settings:   apiclient.ProxySettings{Url: "http://configured:3128"},
			requestUrl: "http://octopus.example.com/api/",
			wantProxy:  "http://configured:3128",
		},
		{
			name:       "a proxy url without a scheme is assumed to be http",
			settings:   apiclient.ProxySettings{Url: "configured:3128"},
			requestUrl: octopusUrl,
			wantProxy:  "http://configured:3128",
		},
		{
			name:       "socks5 proxies are passed through to net/http",
			settings:   apiclient.ProxySettings{Url: "socks5://configured:1080"},
			requestUrl: octopusUrl,
			wantProxy:  "socks5://configured:1080",
		},
		{
			name:       "NO_PROXY excludes the host from the configured proxy",
			settings:   apiclient.ProxySettings{Url: "http://configured:3128"},
			env:        map[string]string{"NO_PROXY": "octopus.example.com"},
			requestUrl: octopusUrl,
		},
		{
			name:       "NO_PROXY excludes the host from HTTPS_PROXY",
			env:        map[string]string{"HTTPS_PROXY": "http://envproxy:3128", "NO_PROXY": "octopus.example.com"},
			requestUrl: octopusUrl,
		},
		{
			name:       "NO_PROXY leaves other hosts proxied",
			settings:   apiclient.ProxySettings{Url: "http://configured:3128"},
			env:        map[string]string{"NO_PROXY": "internal.example.com"},
			requestUrl: octopusUrl,
			wantProxy:  "http://configured:3128",
		},
		{
			name:       "loopback servers are never proxied",
			settings:   apiclient.ProxySettings{Url: "http://configured:3128"},
			requestUrl: "http://localhost:8065/api/",
		},
		{
			name:       "credentials are added to the configured proxy url",
			settings:   apiclient.ProxySettings{Url: "http://configured:3128", Username: "octo", Password: "s3cret"},
			requestUrl: octopusUrl,
			wantProxy:  "http://octo:s3cret@configured:3128",
		},
		{
			name:       "credentials are added to a proxy url taken from the environment",
			settings:   apiclient.ProxySettings{Username: "octo", Password: "s3cret"},
			env:        map[string]string{"HTTPS_PROXY": "http://envproxy:3128"},
			requestUrl: octopusUrl,
			wantProxy:  "http://octo:s3cret@envproxy:3128",
		},
		{
			name:       "credentials in the proxy url win over the environment",
			settings:   apiclient.ProxySettings{Url: "http://inurl:inurlpassword@configured:3128", Username: "octo", Password: "s3cret"},
			requestUrl: octopusUrl,
			wantProxy:  "http://inurl:inurlpassword@configured:3128",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearProxyEnvironment(t)
			for key, value := range test.env {
				t.Setenv(key, value)
			}

			proxyFunc, err := test.settings.ProxyFunc()
			if !assert.NoError(t, err) {
				return
			}

			request, err := http.NewRequest(http.MethodGet, test.requestUrl, nil)
			if !assert.NoError(t, err) {
				return
			}

			proxyUrl, err := proxyFunc(request)
			assert.NoError(t, err)

			if test.wantProxy == "" {
				assert.Nil(t, proxyUrl)
				return
			}
			if assert.NotNil(t, proxyUrl) {
				assert.Equal(t, test.wantProxy, proxyUrl.String())
			}
		})
	}
}

func TestProxySettings_ProxyFuncRejectsAnInvalidProxyUrl(t *testing.T) {
	clearProxyEnvironment(t)

	_, err := apiclient.ProxySettings{Url: "http://%zz:3128"}.ProxyFunc()

	assert.ErrorContains(t, err, "invalid proxy url")
}

// The error goes to the terminal (and CI logs), so it must not repeat the password
// back - neither from the raw string nor from url.Parse's own *url.Error message.
func TestProxySettings_ProxyFuncDoesNotEchoThePasswordOfAnInvalidProxyUrl(t *testing.T) {
	clearProxyEnvironment(t)

	_, err := apiclient.ProxySettings{Url: "http://octo:s3cret@%zz:3128"}.ProxyFunc()

	if assert.ErrorContains(t, err, "invalid proxy url") {
		assert.NotContains(t, err.Error(), "s3cret")
		assert.Contains(t, err.Error(), "octo:xxxxx@")
	}
}

func TestProxySettingsFromConfig(t *testing.T) {
	clearProxyEnvironment(t)
	t.Setenv(constants.EnvOctopusProxyUsername, "octo")
	t.Setenv(constants.EnvOctopusProxyPassword, "s3cret")

	viper.Set(constants.ConfigProxyUrl, "http://configured:3128")
	t.Cleanup(func() { viper.Set(constants.ConfigProxyUrl, "") })

	settings := apiclient.ProxySettingsFromConfig()

	assert.Equal(t, apiclient.ProxySettings{Url: "http://configured:3128", Username: "octo", Password: "s3cret"}, settings)
}

func TestNewHttpTransport_SendsRequestsThroughTheProxy(t *testing.T) {
	clearProxyEnvironment(t)

	var proxiedUrl, proxyAuthorization string
	proxy := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		proxiedUrl = r.URL.String()
		proxyAuthorization = r.Header.Get("Proxy-Authorization")
	}))
	defer proxy.Close()

	transport, err := apiclient.NewHttpTransport(apiclient.ProxySettings{Url: proxy.URL, Username: "octo", Password: "s3cret"}, false)
	if !assert.NoError(t, err) {
		return
	}

	response, err := (&http.Client{Transport: transport}).Get("http://octopus.example.com/api/")
	if !assert.NoError(t, err) {
		return
	}
	defer response.Body.Close()

	assert.Equal(t, "http://octopus.example.com/api/", proxiedUrl)
	assert.Equal(t, "Basic "+base64.StdEncoding.EncodeToString([]byte("octo:s3cret")), proxyAuthorization)
}

// The CLI used to configure TLS by mutating the shared default transport, which
// affects every other user of it in the process.
func TestNewHttpTransport_LeavesTheDefaultTransportAlone(t *testing.T) {
	clearProxyEnvironment(t)

	transport, err := apiclient.NewHttpTransport(apiclient.ProxySettings{}, true)
	if !assert.NoError(t, err) {
		return
	}

	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
	if defaultTlsConfig := http.DefaultTransport.(*http.Transport).TLSClientConfig; defaultTlsConfig != nil {
		assert.False(t, defaultTlsConfig.InsecureSkipVerify, "the shared default transport must keep verifying certificates")
	}
}

func TestRedactProxyUrl(t *testing.T) {
	tests := []struct {
		name   string
		rawUrl string
		want   string
	}{
		{name: "empty", rawUrl: "", want: ""},
		{name: "no credentials", rawUrl: "http://proxy.example.com:3128", want: "http://proxy.example.com:3128"},
		{name: "no scheme", rawUrl: "proxy.example.com:3128", want: "proxy.example.com:3128"},
		{name: "username only", rawUrl: "http://octo@proxy.example.com:3128", want: "http://octo@proxy.example.com:3128"},
		{name: "username and password", rawUrl: "http://octo:s3cret@proxy.example.com:3128", want: "http://octo:xxxxx@proxy.example.com:3128"},
		{name: "unparseable", rawUrl: "http://octo:s3cret@%zz", want: "***"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, apiclient.RedactProxyUrl(test.rawUrl))
			assert.NotContains(t, apiclient.RedactProxyUrl(test.rawUrl), "s3cret")
		})
	}
}
