package logout

import (
	"bytes"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestLogout_SetsConfigCorrectly(t *testing.T) {
	api := testutil.NewMockHttpServer()
	fac := testutil.NewMockFactory(api)
	logoutCmd := NewCmdLogout(fac)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	logoutCmd.SetOut(stdout)
	logoutCmd.SetErr(stderr)
	cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
		return logoutCmd.ExecuteC()
	})

	_, err := testutil.ReceivePair(cmdReceiver)
	assert.Nil(t, err)
	assert.Empty(t, viper.GetString(constants.ConfigUrl))
	assert.Empty(t, viper.GetString(constants.ConfigApiKey))
	assert.Empty(t, viper.GetString(constants.ConfigAccessToken))
}
