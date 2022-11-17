package create

import (
	"errors"
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/executionscommon"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts/azure"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
	"strings"
)

type GetAllAzureAccounts func() ([]*accounts.AzureServicePrincipalAccount, error)
type GetAllAzureWebApps func(account *accounts.AzureServicePrincipalAccount) ([]*azure.AzureWebApp, error)
type GetAllAzureWebAppSlots func(account *accounts.AzureServicePrincipalAccount, app *azure.AzureWebApp) ([]*azure.AzureWebAppSlot, error)

const (
	FlagName          = "name"
	FlagAccount       = "account"
	FlagWebApp        = "web-app"
	FlagResourceGroup = "resource-group"
	FlagWebAppSlot    = "web-app-slot"
)

type CreateFlags struct {
	Name          *flag.Flag[string]
	Account       *flag.Flag[string]
	WebApp        *flag.Flag[string]
	ResourceGroup *flag.Flag[string]
	Slot          *flag.Flag[string]
	*shared.CreateTargetEnvironmentFlags
	*shared.CreateTargetRoleFlags
	*shared.CreateTargetTenantFlags
	*shared.WorkerPoolFlags
	*shared.WebFlags
}

type CreateOptions struct {
	*CreateFlags
	*shared.CreateTargetEnvironmentOptions
	*shared.CreateTargetRoleOptions
	*shared.CreateTargetTenantOptions
	*shared.WorkerPoolOptions
	*cmd.Dependencies

	GetAllAzureAccounts
	GetAllAzureWebApps
	GetAllAzureWebAppSlots
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Name:                         flag.New[string](FlagName, false),
		Account:                      flag.New[string](FlagAccount, false),
		WebApp:                       flag.New[string](FlagWebApp, false),
		ResourceGroup:                flag.New[string](FlagResourceGroup, false),
		Slot:                         flag.New[string](FlagWebAppSlot, false),
		CreateTargetRoleFlags:        shared.NewCreateTargetRoleFlags(),
		CreateTargetEnvironmentFlags: shared.NewCreateTargetEnvironmentFlags(),
		CreateTargetTenantFlags:      shared.NewCreateTargetTenantFlags(),
		WorkerPoolFlags:              shared.NewWorkerPoolFlags(),
		WebFlags:                     shared.NewWebFlags(),
	}
}

func NewCreateOptions(createFlags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:                    createFlags,
		Dependencies:                   dependencies,
		CreateTargetRoleOptions:        shared.NewCreateTargetRoleOptions(dependencies),
		CreateTargetEnvironmentOptions: shared.NewCreateTargetEnvironmentOptions(dependencies),
		CreateTargetTenantOptions:      shared.NewCreateTargetTenantOptions(dependencies),
		WorkerPoolOptions:              shared.NewWorkerPoolOptionsForCreateTarget(dependencies),
		GetAllAzureAccounts: func() ([]*accounts.AzureServicePrincipalAccount, error) {
			return getAllAzureAccounts(*dependencies.Client)
		},
		GetAllAzureWebApps: func(account *accounts.AzureServicePrincipalAccount) ([]*azure.AzureWebApp, error) {
			return getAllAzureWebapps(*dependencies.Client, account)
		},
		GetAllAzureWebAppSlots: func(account *accounts.AzureServicePrincipalAccount, webapp *azure.AzureWebApp) ([]*azure.AzureWebAppSlot, error) {
			return getAllAzureWebAppSlots(*dependencies.Client, account, webapp)
		},
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an Azure Web App deployment target",
		Long:  "Create an Azure Web App deployment target in Octopus Deploy",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s deployment-target azure-web-app create
	`), constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c))

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "A short, memorable, unique name for this Azure Web App.")
	flags.StringVar(&createFlags.Account.Value, createFlags.Account.Name, "", "The name or ID of the Azure Service Principal account")
	flags.StringVar(&createFlags.ResourceGroup.Value, createFlags.ResourceGroup.Name, "", "The resource group of the Azure Web App")
	flags.StringVar(&createFlags.WebApp.Value, createFlags.WebApp.Name, "", "The name of the Azure Web App for this deployment target")
	flags.StringVar(&createFlags.Slot.Value, createFlags.Slot.Name, "", "The name of the Azure Web App Slot for this deployment target")
	shared.RegisterCreateTargetEnvironmentFlags(cmd, createFlags.CreateTargetEnvironmentFlags)
	shared.RegisterCreateTargetRoleFlags(cmd, createFlags.CreateTargetRoleFlags)
	shared.RegisterCreateTargetTenantFlags(cmd, createFlags.CreateTargetTenantFlags)
	shared.RegisterCreateTargetWorkerPoolFlags(cmd, createFlags.WorkerPoolFlags)
	shared.RegisterWebFlag(cmd, createFlags.WebFlags)
	return cmd
}

func createRun(opts *CreateOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	envs, err := executionscommon.FindEnvironments(opts.Client, opts.Environments.Value)
	if err != nil {
		return err
	}
	environmentIds := util.SliceTransform(envs, func(e *environments.Environment) string { return e.ID })

	account, err := getAzureAccount(opts)
	if err != nil {
		return err
	}

	endpoint := machines.NewAzureWebAppEndpoint()
	endpoint.AccountID = account.GetID()
	endpoint.WebAppName = opts.WebApp.Value
	endpoint.ResourceGroupName = opts.ResourceGroup.Value
	endpoint.WebAppSlotName = opts.Slot.Value
	deploymentTarget := machines.NewDeploymentTarget(opts.Name.Value, endpoint, environmentIds, shared.DistinctRoles(opts.Roles.Value))

	err = shared.ConfigureTenant(deploymentTarget, opts.CreateTargetTenantFlags, opts.CreateTargetTenantOptions)
	if err != nil {
		return err
	}

	createdTarget, err := opts.Client.Machines.Add(deploymentTarget)
	if err != nil {
		return err
	}

	fmt.Fprintf(opts.Out, "Successfully created Azure web app '%s'.\n", deploymentTarget.Name)
	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Name, opts.Account, opts.WebApp, opts.ResourceGroup, opts.Slot, opts.Environments, opts.Roles, opts.TenantedDeploymentMode, opts.Tenants, opts.TenantTags)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	shared.DoWebForTargets(createdTarget, opts.Dependencies, opts.WebFlags, "Azure web app")

	return nil
}

func PromptMissing(opts *CreateOptions) error {
	err := question.AskName(opts.Ask, "", "Azure Web App", &opts.Name.Value)
	if err != nil {
		return err
	}

	err = shared.PromptForEnvironments(opts.CreateTargetEnvironmentOptions, opts.CreateTargetEnvironmentFlags)
	if err != nil {
		return err
	}

	err = shared.PromptForRoles(opts.CreateTargetRoleOptions, opts.CreateTargetRoleFlags)
	if err != nil {
		return err
	}

	account, err := PromptForAccount(opts)
	if err != nil {
		return err
	}

	err = PromptForWebApp(opts, account)
	if err != nil {
		return err
	}

	err = shared.PromptForWorkerPool(opts.WorkerPoolOptions, opts.WorkerPoolFlags)
	if err != nil {
		return err
	}

	err = shared.PromptForTenant(opts.CreateTargetTenantOptions, opts.CreateTargetTenantFlags)
	if err != nil {
		return err
	}

	return nil
}

func PromptForAccount(opts *CreateOptions) (*accounts.AzureServicePrincipalAccount, error) {
	var account *accounts.AzureServicePrincipalAccount
	if opts.Account.Value == "" {
		selectedAccount, err := selectors.Select(
			opts.Ask,
			"Select the Azure Account to use\n",
			opts.GetAllAzureAccounts,
			func(p *accounts.AzureServicePrincipalAccount) string {
				return (*p).GetName()
			})
		if err != nil {
			return nil, err
		}
		account = selectedAccount
	} else {
		a, err := getAzureAccount(opts)
		if err != nil {
			return nil, err
		}
		account = a
	}

	opts.Account.Value = account.Name
	return account, nil
}

func PromptForWebApp(opts *CreateOptions, account *accounts.AzureServicePrincipalAccount) error {
	webapps, err := opts.GetAllAzureWebApps(account)
	if err != nil {
		return err
	}

	var webapp *azure.AzureWebApp
	if opts.WebApp.Value == "" || opts.ResourceGroup.Value == "" {
		if account == nil {
			var err error
			account, err = getAzureAccount(opts)
			if err != nil {
				return err
			}
		}

		if opts.ResourceGroup.Value != "" {
			webapps = util.SliceFilter(webapps, func(a *azure.AzureWebApp) bool {
				return strings.EqualFold(a.ResourceGroup, opts.ResourceGroup.Value)
			})
		}
		if opts.WebApp.Value != "" {
			webapps = util.SliceFilter(webapps, func(a *azure.AzureWebApp) bool {
				return strings.EqualFold(a.Name, opts.WebApp.Value)
			})
		}

		selectedWebApp, err := selectors.Select(
			opts.Ask,
			"Select the Azure Web App\n",
			func() ([]*azure.AzureWebApp, error) { return webapps, nil },
			func(a *azure.AzureWebApp) string { return a.Name })
		if err != nil {
			return err
		}

		webapp = selectedWebApp
	} else {
		matchedApps := util.SliceFilter(webapps, func(a *azure.AzureWebApp) bool {
			return strings.EqualFold(a.Name, opts.WebApp.Value) && strings.EqualFold(a.ResourceGroup, opts.ResourceGroup.Value)
		})

		if len(matchedApps) != 1 {
			return errors.New("could not find matching Azure Web App")
		}

		webapp = matchedApps[0]
	}

	opts.WebApp.Value = webapp.Name
	opts.ResourceGroup.Value = webapp.ResourceGroup

	if opts.Slot.Value == "" {
		slots, err := opts.GetAllAzureWebAppSlots(account, webapp)
		if err != nil {
			return err
		}

		if util.Any(slots) {
			selectedSlot, err := selectors.Select(opts.Ask, "Select the Azure Web App slot\n", func() ([]*azure.AzureWebAppSlot, error) { return slots, nil }, func(slot *azure.AzureWebAppSlot) string { return slot.Name })
			if err != nil {
				return err
			}

			opts.Slot.Value = selectedSlot.Name
		}
	}

	return nil
}

func getAllAzureWebAppSlots(client client.Client, spAccount *accounts.AzureServicePrincipalAccount, webapp *azure.AzureWebApp) ([]*azure.AzureWebAppSlot, error) {
	slots, err := azure.GetWebSiteSlots(client, spAccount, webapp)
	if err != nil {
		return nil, err
	}

	return slots, nil
}

func getAllAzureWebapps(client client.Client, account *accounts.AzureServicePrincipalAccount) ([]*azure.AzureWebApp, error) {
	sites, err := azure.GetWebSites(client, account)
	if err != nil {
		return nil, err
	}

	return sites, nil
}

func getAllAzureAccounts(client client.Client) ([]*accounts.AzureServicePrincipalAccount, error) {
	allAccounts, err := client.Accounts.GetAll()
	if err != nil {
		return nil, err
	}

	var spAccounts []*accounts.AzureServicePrincipalAccount
	for _, a := range allAccounts {
		if s, ok := a.(*accounts.AzureServicePrincipalAccount); ok {
			spAccounts = append(spAccounts, s)
		}
	}

	return spAccounts, nil
}

func getAzureAccount(opts *CreateOptions) (*accounts.AzureServicePrincipalAccount, error) {
	idOrName := opts.Account.Value
	allAccounts, err := opts.GetAllAzureAccounts()
	if err != nil {
		return nil, err
	}

	for _, a := range allAccounts {
		if strings.EqualFold(a.GetID(), idOrName) || strings.EqualFold(a.GetName(), idOrName) {
			return a, nil
		}
	}

	return nil, fmt.Errorf("cannot find account %s", idOrName)
}
