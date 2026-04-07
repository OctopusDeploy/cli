package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

// OsExit is a variable so tests can stub it to avoid terminating the process.
var OsExit = os.Exit

func NewCmdAPI(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api <url>",
		Short: "Execute a raw API GET request",
		Long:  "Execute an authenticated GET request against the Octopus Server API and print the JSON response.",
		Example: heredoc.Docf(`
			$ %[1]s api /api
			$ %[1]s api /api/spaces
			$ %[1]s api /api/Spaces-1/projects
		`, constants.ExecutableName),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return apiRun(cmd, f, args[0])
		},
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	return cmd
}

func apiRun(cmd *cobra.Command, f factory.Factory, path string) error {
	client, err := f.GetSystemClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return err
	}

	resp, err := client.HttpSession().DoRawRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Pretty-print if valid JSON, otherwise output raw
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, body, "", "  "); err == nil {
		cmd.Println(prettyJSON.String())
	} else {
		cmd.Print(string(body))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		OsExit(resp.StatusCode)
	}

	return nil
}
