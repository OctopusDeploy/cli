package accesstokens

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpiry_ReadsTheExpClaim(t *testing.T) {
	expires := time.Now().Add(time.Hour).Truncate(time.Second)

	assert.Equal(t, expires, expiry(jwtWith(t, map[string]any{"exp": expires.Unix()})))
}

// A token that cannot be read is still a usable token, so an unreadable one
// reports no expiry rather than an error.
func TestExpiry_UnreadableTokensReportNothing(t *testing.T) {
	for name, token := range map[string]string{
		"not a JWT":             "API-XXXXXXXX",
		"no payload":            "a.b",
		"payload is not base64": "a.!!!.c",
		"payload is not JSON":   "a." + base64.RawURLEncoding.EncodeToString([]byte("nonsense")) + ".c",
		"no exp claim":          jwtWith(t, map[string]any{"sub": "users-1"}),
		"an exp of zero":        jwtWith(t, map[string]any{"exp": 0}),
	} {
		t.Run(name, func(t *testing.T) {
			assert.True(t, expiry(token).IsZero())
		})
	}
}

func TestDescribe_NeverIncludesTheToken(t *testing.T) {
	expires := time.Unix(1750000000, 0)

	described := Token{Value: "eyJhbGciOiJIUzI1NiJ9.secret", Expires: expires}.Describe()
	assert.NotContains(t, described, "secret")
	assert.Contains(t, described, expires.Local().Format("15:04"))

	assert.Equal(t, "short-lived access token for your Octopus user", Token{Value: "x"}.Describe())
}

func jwtWith(t *testing.T, claims map[string]any) string {
	t.Helper()

	payload, err := json.Marshal(claims)
	require.NoError(t, err)

	encode := base64.RawURLEncoding.EncodeToString
	return encode([]byte(`{"alg":"HS256"}`)) + "." + encode(payload) + ".signature"
}
