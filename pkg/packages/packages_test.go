package packages_test

import (
	"errors"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/packages"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/stretchr/testify/assert"
)

func TestMissingPackageVersionsError_Error(t *testing.T) {
	missing := []releases.ReleaseTemplatePackage{
		{PackageID: "acme.web", ActionName: "Deploy Web", FeedID: "feeds-builtin", FeedName: "Octopus Server (built-in)"},
	}

	t.Run("names the package, the step and the feed", func(t *testing.T) {
		err := packages.NewMissingPackageVersionsError(missing, nil)
		assert.Contains(t, err.Error(), "no version could be found for the following packages")
		assert.Contains(t, err.Error(), "'acme.web' in step 'Deploy Web' (feed 'Octopus Server (built-in)')")
		assert.Contains(t, err.Error(), "push the package(s) to the feed")
	})

	// the diagnosis is inferred from a failure the server doesn't describe, so if we guessed wrong
	// the user still needs to be able to see what actually went wrong
	t.Run("reports what the server said alongside the diagnosis", func(t *testing.T) {
		cause := errors.New("There are no viable release plans in any channels")
		err := packages.NewMissingPackageVersionsError(missing, cause)
		assert.Contains(t, err.Error(), "the server reported: There are no viable release plans in any channels")
		assert.ErrorIs(t, err, cause)
	})

	t.Run("omits the null reference message, which explains nothing", func(t *testing.T) {
		cause := errors.New("Octopus API error: " + packages.ServerNullReferenceMessage + " []")
		err := packages.NewMissingPackageVersionsError(missing, cause)
		assert.NotContains(t, err.Error(), packages.ServerNullReferenceMessage)
		assert.NotContains(t, err.Error(), "the server reported")
		assert.ErrorIs(t, err, cause) // still unwrappable, just not printed
	})
}
