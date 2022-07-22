//go:build integration

package integrationtest

import (
	"errors"
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"runtime"
)

// Integration tests should call this to get a "back door" connection to the Octopus Server
// so they can create fixture data, cleanup, etc
func GetApiClient(spaceId string) (*client.Client, error) {
	apiUrl, err := url.Parse(os.Getenv("OCTOPUS_TEST_URL"))
	apiKey := os.Getenv("OCTOPUS_TEST_APIKEY")

	if err != nil || apiUrl == nil || apiKey == "" {
		fmt.Println("IntegrationTest GetApiClient cannot launch; OCTOPUS_TEST_URL and OCTOPUS_TEST_APIKEY environment variables must be set")
		os.Exit(999)
	}

	return client.NewClient(nil, apiUrl, apiKey, spaceId)
}

func GetCliPath() (cliPath string, cliDir string, err error) {
	_, fileName, _, ok := runtime.Caller(1)
	if ok {
		// we expect to be in <base>\integrationtest
		// we expect the CLI executable to be in <base>\cmd\octopus
		myDir := path.Dir(fileName)
		cliDir = path.Join(myDir, "..", "cmd", "octopus")
		if _, err = os.Stat(cliDir); os.IsNotExist(err) {
			err = errors.New(fmt.Sprintf("expected directory %s not found!", cliDir))
			return
		}

		//goland:noinspection GoBoolExpressions
		if runtime.GOOS == "windows" {
			cliPath = path.Join(cliDir, "octopus.exe")
		} else {
			cliPath = path.Join(cliDir, "octopus")
		}
	} else {
		err = errors.New("Can't get runtime.caller(1)")
	}
	return
}

var ensureCliHasRun = false

// Returns string(stdout), string(stderr), error
// It is a particularly bad idea to run this on something that outputs loads and loads
// of std output over time as we'll consume heaps of memory, but we aren't doing that in
// our integration tests.
func runExecutable(executable string, args []string, workingDirectory string, environment []string) (string, string, error) {
	cmd := exec.Command(executable, args...)
	cmd.Dir = workingDirectory
	// don't hook stdin, go isn't going to ask us for anything
	stdIn, _ := cmd.StdinPipe()
	stdOut, _ := cmd.StdoutPipe()
	stdErr, _ := cmd.StderrPipe()

	if environment != nil {
		cmd.Env = environment
	}

	err := cmd.Start()
	if err != nil {
		return "", "", err
	}
	err = stdIn.Close()
	if err != nil {
		return "", "", err
	}

	outBytes, _ := io.ReadAll(stdOut)
	errBytes, _ := io.ReadAll(stdErr)

	err = cmd.Wait() // wait for exit

	// note! If the process returned an exit code, then err will be an exec.ExitError
	// but stdout and stderr strings may have data in them that may be interesting.
	return string(outBytes), string(errBytes), err
}

// builds the CLI using 'go build' if it does not already exist.
// note: this will always deliberately build the CLI the first time you invoke it, just in case
// you have an existing out-of-date binary lying around from some prior thing
func EnsureCli() (cliPath string, cliDir string, err error) {
	cliPath, cliDir, err = GetCliPath()
	if err != nil {
		return
	}

	shouldCompile := !ensureCliHasRun
	_, err = os.Stat(cliPath)
	if err != nil {
		if os.IsNotExist(err) {
			shouldCompile = true
		} else {
			return
		}
	}

	if shouldCompile {
		capturedStdout, capturedStderr, runErr := runExecutable("go", []string{"build", "."}, cliDir, os.Environ())

		// typically the go compiler doesn't emit any output, so don't expect anything here
		// always print stdout/stderr even if the process failed
		fmt.Println(capturedStdout)
		if len(capturedStderr) > 0 {
			fmt.Println(output.Red(capturedStderr))
		}

		if runErr != nil {
			if exiterr, ok := runErr.(*exec.ExitError); ok {
				err = errors.New(fmt.Sprintf("go build failed with exit code %d", exiterr.ExitCode()))
			}
			return // fail
		}
	}

	ensureCliHasRun = true
	return
}

func RunCli(space string, args ...string) (string, string, error) {
	cliPath, cliDir, err := EnsureCli()
	if err != nil { // failed!
		return "", "", err
	}

	// we know that OCTOPUS_TEST_URL is already available.
	// TODO pass this through rather than re-looking it up
	env := []string{
		fmt.Sprintf("OCTOPUS_HOST=%s", os.Getenv("OCTOPUS_TEST_URL")),
		fmt.Sprintf("OCTOPUS_API_KEY=%s", os.Getenv("OCTOPUS_TEST_APIKEY")),
		fmt.Sprintf("OCTOPUS_SPACE=%s", space),
	}

	return runExecutable(cliPath, args, cliDir, env)
}
