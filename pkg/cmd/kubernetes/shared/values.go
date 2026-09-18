package shared

import (
	"fmt"
	"io"
	"os"

	"github.com/OctopusDeploy/cli/pkg/output"
	"sigs.k8s.io/yaml"
)

// WriteValuesFile writes the resolved Helm values where --output-values asked
// for them. secretsWarning, when set, is printed after the file is written,
// because writing credentials to disk should never happen silently.
func WriteValuesFile(out io.Writer, path string, values map[string]any, secretsWarning string) error {
	if path == "" {
		return nil
	}

	encoded, err := yaml.Marshal(values)
	if err != nil {
		return fmt.Errorf("could not encode the Helm values: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return fmt.Errorf("could not write %s: %w", path, err)
	}

	fmt.Fprintf(out, "Wrote Helm values to %s\n", output.Cyan(path))
	if secretsWarning != "" {
		fmt.Fprintf(out, "%s %s\n", output.Yellow("!"), secretsWarning)
	}
	return nil
}
