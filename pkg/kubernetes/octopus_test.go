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
