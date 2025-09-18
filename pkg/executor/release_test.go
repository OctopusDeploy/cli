package executor

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Test that when CustomFields are supplied on TaskOptionsCreateRelease they appear in marshalled CreateReleaseCommandV1
func TestCreateReleaseCommand_CustomFieldsMarshal(t *testing.T) {
	cmd := struct {
		SpaceID         string            `json:"spaceId"`
		ProjectIDOrName string            `json:"projectName"`
		CustomFields    map[string]string `json:"customFields,omitempty"`
		SpaceIDOrName   string            `json:"spaceIdOrName"`
	}{
		SpaceID:         "Spaces-1",
		ProjectIDOrName: "MyProject",
		CustomFields:    map[string]string{"Build": "123", "Commit": "abc"},
		SpaceIDOrName:   "Spaces-1",
	}
	data, err := json.Marshal(cmd)
	require.NoError(t, err)
	jsonStr := string(data)
	require.True(t, strings.Contains(jsonStr, "\"customFields\""))
	require.True(t, strings.Contains(jsonStr, "\"Build\":\"123\""))
	require.True(t, strings.Contains(jsonStr, "\"Commit\":\"abc\""))
}
