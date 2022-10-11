package cmd

type NestedOpts interface {
	Commit() error
	GenerateAutomationCmd()
}
