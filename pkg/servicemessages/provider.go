package servicemessages

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/spf13/viper"
)

type Provider interface {
	ServiceMessage(messageName string, values any)
}

type provider struct {
	printer *OutputPrinter
}

func NewProvider(printer *OutputPrinter) Provider {
	return &provider{
		printer: printer,
	}
}

func (p *provider) ServiceMessage(messageName string, values any) {
	serviceMessageEnabled := viper.GetBool(constants.FlagEnableServiceMessages)
	if !serviceMessageEnabled {
		return
	}

	teamCityEnvVar := os.Getenv("TEAMCITY_VERSION")
	if teamCityEnvVar == "" {
		p.printer.Error("service messages are only supported in TeamCity builds")
		return
	}

	switch t := values.(type) {
	case string:
		p.printer.Info(fmt.Sprintf("##teamcity[%s %s]\n", messageName, t))
	case map[string]string:
		mapMsg := p.mapToStringMsg(t, messageName)
		p.printer.Info(mapMsg)
	default:
		p.printer.Error("Unsupported service message value type")
	}
}

type OutputPrinter struct {
	Out io.Writer
	Err io.Writer
}

func NewOutputPrinter(out io.Writer, err io.Writer) *OutputPrinter {
	return &OutputPrinter{
		Out: out,
		Err: err,
	}
}

func (p *OutputPrinter) Info(msg string) {
	fmt.Fprint(p.Out, msg)
}

func (p *OutputPrinter) Error(msg string) {
	fmt.Fprint(p.Err, msg)
}

func (p *provider) mapToStringMsg(m map[string]string, messageName string) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("##teamcity[%s", messageName))
	for key, value := range m {
		builder.WriteString(fmt.Sprintf(" %s=%s", key, value))
	}
	builder.WriteString("]")
	return builder.String()
}
