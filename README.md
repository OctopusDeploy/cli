<div align="center">
  <img alt="go-octopusdeploy Logo" src="https://user-images.githubusercontent.com/71493/133961475-fd4d769f-dc32-4723-a9bd-5529c5b12faf.png" height="140" />
  <h3 align="center">cli</h3>
  <p align="center">Command Line Interface for <a href="https://octopus.com/">Octopus Deploy</a> 🐙</p>
  <p align="center">
    <a href="https://github.com/OctopusDeploy/cli/releases/latest"><img alt="GitHub release" src="https://img.shields.io/github/v/release/OctopusDeploy/cli.svg?logo=github&style=flat-square"></a>
    <a href="https://goreportcard.com/badge/github.com/OctopusDeploy/cli"><img src="https://goreportcard.com/badge/github.com/OctopusDeploy/cli" alt="Go Report"></a>
  </p>
</div>

---

## Overview

This project aims to create a new CLI for communicating with the Octopus Deploy Server, written in Go.

It does **not** seek to be a drop-in replacement for the existing CLI which is written in C# using .NET.
https://github.com/OctopusDeploy/OctopusCLI

### Differences from the .NET CLI

The new CLI will not initially contain all the features of the existing .NET CLI. 
Over time we plan to add features and may eventually reach parity, but our intent is that both the 
.NET and Go CLI's will co-exist for a significant period of time.

The New CLI restructures the command line to be more consistent, and fit with convention
across other popular CLI apps. It is built on the popular and widely-used [Cobra](https://github.com/spf13/cobra) 
command line processing library.

#### Examples:

**.NET CLI**

    octo list-releases 
    octo create-release

**Go CLI**

    octopus release list 
    octopus release create

The new CLI supports an "interactive" mode, where it will prompt for input where
parameters are not fully specified on the command line.

## Documentation

End-user documentation will be provided via the octopus documentation site at a future date.

## 🤝 Contributions

Contributions are welcome! :heart: Please read our [Contributing Guide](CONTRIBUTING.md) for information about how to get involved in this project.

# Developer Guide

## Getting Started

First, ensure that you have [Go](https://go.dev/) installed, and available in your `PATH`.
To verify this, open a new terminal window and type `go version`. You should see something similar to `go version go1.18.4 windows/amd64`

Next, clone this git repository

Next, open the directory you cloned into, navigate into the `cmd/octopus` directory, and type `go build .`

```shell
cd <your-local-development-dir>
git clone https://github.com/OctopusDeploy/cli
cd cli
cd cmd/octopus
go build .
```

If successful, the go compiler does not output anything. You should now have an `octopus` binary 
(`octopus.exe` on windows) in your current directory.

## Running the CLI

The CLI needs to authenticate with the octopus server.
This is currently managed using environment variables which you must set before launching it.

**macOS/Linux:**

```shell
export OCTOPUS_HOST="http://localhost:8050" # replace with your octopus URL
export OCTOPUS_API_KEY: "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX" # replace with your API key
./octopus space list # should list all the spaces
```

**Windows (powershell):**

```shell
$env:OCTOPUS_HOST="http://localhost:8050" # replace with your octopus URL
$env:OCTOPUS_API_KEY: "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX" # replace with your API key
./octopus.exe space list # should list all the spaces
```

**Windows (cmd):**

```shell
set OCTOPUS_HOST="http://localhost:8050" # replace with your octopus URL
set OCTOPUS_API_KEY: "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX" # replace with your API key
octopus.exe space list # should list all the spaces
```

### go-octopusdeploy library

The CLI depends heavily on the [go-octopusdeploy](https://github.com/OctopusDeploy/go-octopusdeploy) library, which manages
communication with the Octopus Server via its JSON API.

## Code structure

The CLI follows standard go language conventions for packages, and fits around the package structures set out by the
Cobra library for commands.

A rough overview is as follows:

```shell
cmd/
   octopus/  # Contains the octopus binary
   
pkg/
   apiclient/ # Utility code used to manage authentication/connection to the octopus server
   cmd/ # contains sub-packages for each cobra command
      account/ # contains commands related to accounts
      environment/ # contains commands related to environments
      ... # more commands   
  constants/ # constant values to avoid duplicated strings, ints, etc
  errors/ # internal error objects
  executor/ # See 'architecture' below
  factory/ # "service locator" object used by commands to locate shared services
  output/ # internal utilities which help formatting output
  question/ # See 'architecture' below 

testutil/ # internal utility code used by both unit and integration tests
integrationtest/ # Contains integration tests  
```

## Testing

The most important thing the CLI does is communicate with the Octopus server.

We place high importance on compatibility with the server, and detection of breakages caused by server changes.
As such, Integration tests form the most important part of our testing strategy for the CLI.

If you are writing a new command, or extending an existing one, you should ensure that you have in place integration
tests, which verify against a running instance of the server, that the command behaves correctly.

Unit tests are used to fill gaps that integration testing cannot effectively cover, such as the
workflow of prompting for user input, or highly algorithmic/mathematical code.

### Unit Tests

Unit tests for packages follow go language conventions, and is located next to the code it is testing.

```shell
pkg/
  question/ 
    input.go
    input_test.go # unit tests for the code contained in input.go  
```

The easiest way to run the tests is to `cd pkg` and run `go test ./...`.  
We find `gotestsum` provides a nice wrapper around the underlying go test functionality, which you may also prefer. 

### Integration Tests

Integration tests live outside the pkg structure and operate outside the app.
They launch the CLI as a seperate process, and interact with it using stdout and stderr.

**Important:** Integration tests assume that an Octopus Deploy server is running and accessible. 
Before running the integration tests you must set the following environment variables, or the tests will fail.

```shell
OCTOPUS_TEST_URL="http://localhost:8050" # replace with your octopus URL
OCTOPUS_TEST_APIKEY: "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX" # replace with your API key
```

**Important:** Integration tests require an admin-level API key.

**Important:** Integration tests assume an empty Octopus Server database.
If your server contains existing data, the tests may fail, and they may modify or delete any existing data.

The easiest way to run the tests is to `cd integrationtest` and run `go test ./...` or `gotestsum`  

### Architecture to enable Testing of interactive mode

While we aim for most functionality to be covered by integration tests, the question/answer flow when running
in interactive mode is not amenable to integration tests. The test runner process would need a lot of highly complex
and code parsing the CLI's stdout commands and emulating a terminal buffer. This is not a productive use of time.

Rather, we architect the application so that the question/answer flows are contained within simple functions,
that we can test using Unit Tests, supplying a mocked wrapper which impersonates the `Survey` library.

## Guidance and Example of how to create and test new commands

Imagine that the CLI did not contain an "account create" command, and we wished to add one.

We would go about it as follows:

#### 1. Create packages and files for the command, linking in with Cobra.

We would make a `/cmd/account/create` directory, and within it put `create.go`

We would implement a `func NewCmdCreate(f factory.Factory) *cobra.Command` function which set up
the command structure, parameters, flags, etc, and link it in with the parent code in `account.go`

Example:
```go
func NewCmdCreate(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates an account in an instance of Octopus Deploy",
		Long:  "Creates an account in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s account create"
		`), constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil // TODO
		},
	}
	cmd.Flags().StringP("name", "n", "", "name for the item")
	cmd.Flags().StringP("description", "d", "", "description for the item")
	return cmd
}
```


#### 2. Create a `Task` which encapsulates the command arguments

in the `executor` package, create a new string constant, and struct to carry the options for your command
```go
const TaskTypeCreateAccount = "createAccount"

type TaskOptionsCreateAccount struct {
    Name           string   // REQUIRED.
    Description    string   // optional
}
```

Back in your `cmd` file, write some code which maps values from the command flags, and puts them into the `Task` structure,
then submit it to the excutor which will do the work when you call `ProcessTasks`

```go
RunE: func(cmd *cobra.Command, args []string) error {
    name := cmd.Flags().GetString("name")
    description := cmd.Flags().GetString("description")

    task := executor.NewTask(executor.TaskTypeCreateAccount, executor.TaskOptionsCreateAccount{
        Name:        name,
        Description: description,
        // etc
    })

    executor.ProcessTasks(f, []executor.Task{ task })
}
```

#### 3. Extend the `executor` to handle your new task

Update the code in ProcessTasks to match your new task identifier string, and write a new helper function
to do the work (sending data to the octopus server, etc.)

At this point you should have a functioning command which works in automation mode.

#### 4. Write some integration tests to ensure your command works as expected when run against a real server

Add a new file under the `integrationtest` directory, and write tests as appropriate

#### 5. Implement interactive mode

Return back to your new command's go file (`account/create.go` in this example)

**TODO!** At this point in the development cycle, the exact patterns and conventions around interactive mode questioning
are not yet well defined. We will update the README to be more prescriptive as the situation changes.

At a high level, you should create a function which encapsulates the interactive question/answer session, and returns
your `TaskOptions` structure, which you then pass to `ProcessTasks` 

You should pass a reference to the `Ask` func, which allows you to mock out the `Survey` library, and then you should
write a series of unit tests which ensure that the question/answer session works correctly.