package integration

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"runtime"

	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
)

// This file contains utilities to help with integration testing

// GetApiClient returns a "back door" connection to the Octopus Server
// that integration tests can use to create fixture data, cleanup, etc
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
		err = errors.New("can't get runtime.caller(1)")
	}
	return
}

var ensureCliHasRun = false

// Returns string(stdout), string(stderr), error
// It is a particularly bad idea to run this on something that outputs loads and loads
// of std output over time as we'll consume heaps of memory, but we aren't doing that in
// our integration tests.
// NOTE: the standard out
func runExecutable(executable string, args []string, workingDirectory string, environment []string) (stdout string, stderr string, err error) {
	stdoutBytes, stderrBytes, err := runExecutableRawOutput(executable, args, workingDirectory, environment)
	if stdoutBytes != nil {
		stdout = string(stdoutBytes)
	} else {
		stdout = ""
	}
	if stderrBytes != nil {
		stderr = string(stderrBytes)
	} else {
		stderr = ""
	}
	return
}

// Runs the CLI but returns raw byte output for stdout and stderr. Typically you want to call runExecutable which returns strings instead
func runExecutableRawOutput(executable string, args []string, workingDirectory string, environment []string) (stdout []byte, stderr []byte, err error) {
	cmd := exec.Command(executable, args...)
	cmd.Dir = workingDirectory
	// don't hook stdin, go isn't going to ask us for anything
	stdIn, _ := cmd.StdinPipe()
	stdOut, _ := cmd.StdoutPipe()
	stdErr, _ := cmd.StderrPipe()

	if environment != nil {
		cmd.Env = environment
	}

	err = cmd.Start()
	if err != nil {
		return
	}
	err = stdIn.Close()
	if err != nil {
		return
	}

	stdout, _ = io.ReadAll(stdOut)
	stderr, _ = io.ReadAll(stdErr)

	err = cmd.Wait() // wait for exit

	// note! If the process returned an exit code, then err will be an exec.ExitError
	// but stdout and stderr strings may have data in them that may be interesting.
	return
}

// EnsureCli builds the CLI using 'go build' if it does not already exist.
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
			err = nil // not really an error as we're going to recover it by compiling the app
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

func mapEnv(space string) []string {
	// we know that OCTOPUS_TEST_URL is already available.
	// TODO pass this through rather than re-looking it up
	return []string{
		fmt.Sprintf("OCTOPUS_HOST=%s", os.Getenv("OCTOPUS_TEST_URL")),
		fmt.Sprintf("OCTOPUS_API_KEY=%s", os.Getenv("OCTOPUS_TEST_APIKEY")),
		fmt.Sprintf("OCTOPUS_SPACE=%s", space),
	}
}

func RunCli(space string, args ...string) (string, string, error) {
	cliPath, cliDir, err := EnsureCli()
	if err != nil { // failed!
		return "", "", err
	}

	return runExecutable(cliPath, args, cliDir, mapEnv(space))
}

func RunCliRawOutput(space string, args ...string) ([]byte, []byte, error) {
	cliPath, cliDir, err := EnsureCli()
	if err != nil { // failed!
		return nil, nil, err
	}

	return runExecutableRawOutput(cliPath, args, cliDir, mapEnv(space))
}
