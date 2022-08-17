package flag

import (
	"fmt"
	"io"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
)

type Flag[T any] struct {
	Name   string
	Value  T
	Secure bool
}

type CreateFlags struct {
	Name         *Flag[string]
	Description  *Flag[string]
	AccessKey    *Flag[string]
	SecretKey    *Flag[string]
	Environments *Flag[[]string]
}

type CreateOptions struct {
	CreateFlags
	Writer   io.Writer
	Octopus  *client.Client
	Ask      question.Asker
	Spinner  factory.Spinner
	Space    string
	Host     string
	NoPrompt bool
	CmdPath  string
}

type Generatable interface {
	GetName() string
	GetValue() any
	IsSecure() bool
}

func (f *Flag[T]) GetName() string {
	return f.Name
}

func (f *Flag[T]) GetValue() any {
	return f.Value
}

func (f *Flag[T]) IsSecure() bool {
	return f.Secure
}

func New[T any](name string, secure bool) *Flag[T] {
	return &Flag[T]{
		Name:   name,
		Secure: secure,
	}
}

// GenerateAutomationCmd generates the command that can be used to achive
// the same results with the CLI in automation mode.
func GenerateAutomationCmd(cmdPath string, flags ...Generatable) string {
	autoCmd := fmt.Sprintf("%s --no-prompt", cmdPath)
	for _, flag := range flags {
		switch value := flag.GetValue().(type) {
		case string:
			if value != "" {
				if flag.IsSecure() {
					autoCmd += fmt.Sprintf(" --%s '***'", flag.GetName())
					continue
				}
				autoCmd += fmt.Sprintf(" --%s '%s'", flag.GetName(), strings.ReplaceAll(value, "'", "'\\''"))
			}
		case []string:
			for _, val := range value {
				if flag.IsSecure() {
					autoCmd += fmt.Sprintf(" --%s '***'", flag.GetName())
					continue
				}
				autoCmd += fmt.Sprintf(" --%s '%s'", flag.GetName(), strings.ReplaceAll(val, "'", "'\\''"))
			}
		case bool:
			if value {
				autoCmd += fmt.Sprintf(" --%s", flag.GetName())
			}
		default:
			err := fmt.Errorf("can not generate automation cmd for unsupported flag type: %T", flag)
			panic(err)
		}
	}
	return autoCmd
}
