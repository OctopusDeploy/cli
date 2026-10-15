package view

import (
	"testing"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/credentials"
	"github.com/stretchr/testify/assert"
)

func TestCredentialTypeDisplay(t *testing.T) {
	assert.Equal(t, "None", credentialTypeDisplay(nil))
	assert.Equal(t, "Anonymous", credentialTypeDisplay(credentials.NewAnonymous()))
	assert.Equal(t, "Username/Password", credentialTypeDisplay(credentials.NewUsernamePassword("user", core.NewSensitiveValue("pass"))))
	assert.Equal(t, "GitHub Connection", credentialTypeDisplay(credentials.NewGitHubApp("Connections-1")))
}
