package kubernetes_test

import (
	"testing"

	"github.com/OctopusDeploy/cli/pkg/kubernetes"
	"github.com/stretchr/testify/assert"
)

func TestDeriveGRPCURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"cloud instance", "https://my.octopus.app", "grpc://my.octopus.app:8443"},
		{"trailing slash", "https://my.octopus.app/", "grpc://my.octopus.app:8443"},
		{"self hosted with a port", "https://octopus.internal:8080/", "grpc://octopus.internal:8443"},
		{"http", "http://octopus.internal/", "grpc://octopus.internal:8443"},
		{"local", "http://localhost:8065/", "grpc://localhost:8443"},
		{"empty", "", ""},
		{"not a url", "://nope", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, kubernetes.DeriveGRPCURL(tt.input))
		})
	}
}

func TestDerivePollingURL(t *testing.T) {
	tests := []struct {
		name     string
		server   string
		expected string
	}{
		{"self-hosted gets the polling port", "https://octopus.example.com", "https://octopus.example.com:10943"},
		{"a port on the REST address is not the polling port", "https://octopus.example.com:8080", "https://octopus.example.com:10943"},
		{"Octopus Cloud serves polling on its own hostname", "https://myinstance.octopus.app", "https://polling.myinstance.octopus.app"},
		{"Octopus Cloud test instances too", "https://myinstance.testoctopus.app", "https://polling.myinstance.testoctopus.app"},
		{"nothing to derive from", "", ""},
		{"not an address", "not a url", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, kubernetes.DerivePollingURL(tt.server))
		})
	}
}
