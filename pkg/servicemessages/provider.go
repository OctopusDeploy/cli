package servicemessages

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/viper"
)

type Provider interface {
	ServiceMessage(messageName string, values any)
}

type provider struct {
	printer *Printer
}

func NewProvider(printer *Printer) Provider {
	return &provider{
		printer: printer,
	}
}

func (p *provider) ServiceMessage(messageName string, values any) {
	serviceMessageEnabled := viper.GetBool("enable-service-messages")
	teamCityEnvVar := os.Getenv("TEAMCITY_VERSION")

	if serviceMessageEnabled && teamCityEnvVar == "" {
		p.printer.Error("service messages are only supported in TeamCity builds")
		return
	}
	switch t := values.(type) {
	case string:
		p.printer.Println(fmt.Sprintf("##teamcity[%s %s]\n", messageName, t))
	case map[string]string:
		for key, value := range t {
			p.printer.Println(fmt.Sprintf("##teamcity[%s %s=%s]\n", messageName, key, value))
		}
	default:
		p.printer.Error("Unsupported service message value type")
	}
}

type Printer struct {
	Out io.Writer
	Err io.Writer
}

func NewPrinter(out io.Writer, err io.Writer) *Printer {
	return &Printer{
		Out: out,
		Err: err,
	}
}

func (p *Printer) Println(msg string) {
	fmt.Fprintln(p.Out, msg)
}

func (p *Printer) Error(msg string) {
	fmt.Fprintln(p.Err, msg)
}
