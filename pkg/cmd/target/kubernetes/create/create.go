package create

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/executionscommon"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/certificates"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/feeds"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
)

const (
	FlagName               = "name"
	FlagAuthenticationType = "auth-type"
	FlagAccount            = "account"

	// Azure Service Principal
	FlagAKSClusterName       = "aks-cluster-name"
	FlagAKSResourceGroupName = "aks-resource-group-name"
	FlagUseAdminCredentials  = "aks-use-admin-credentials"

	// AWS Account
	FlagUseServiceRole             = "eks-use-service-role"
	FlagAssumeServiceRole          = "eks-assume-service-role"
	FlagAssumedRoleARN             = "eks-assumed-role-arn"
	FlagAssumedRoleSessionName     = "eks-assumed-role-session-name"
	FlagAssumedRoleSessionDuration = "eks-assumed-role-session-duration"
	FlagAssumedRoleExternalID      = "eks-assumed-role-external-id"
	FlagEKSClusterName             = "eks-cluster-name"

	// Google Cloud Account
	FlagUseVMServiceAccount       = "gke-use-vm-service-account"
	FlagImpersonateServiceAccount = "gke-impersonate-service-account"
	FlagServiceAccountEmails      = "gke-service-account-emails"
	FlagGKEClusterName            = "gke-cluster-name"
	FlagProject                   = "gke-project"
	FlagClusterType               = "gke-cluster-type"
	FlagZone                      = "gke-zone"
	FlagRegion                    = "gke-region"

	FlagClientCertificate = "client-certificate"

	// Pod Service Account
	FlagTokenFilePath = "pod-token-path"

	FlagSkipTLSVerification  = "skip-tls-verification"
	FlagKubernetesClusterURL = "cluster-url"
	FlagKubernetesNamespace  = "namespace"
	FlagCertificate          = "certificate"
	FlagCertificateFilePath  = "certificate-path"

	FlagHealthCheckContainerRegistry = "docker-container-registry"
	FlagHealthCheckTags              = "docker-image-flags"
)

const (
	AuthTypeDisplayUsernameAndPassword   = "Username and Password"
	AuthTypeDisplayToken                 = "Token"
	AuthTypeDisplayAzureServicePrincipal = "Azure Service Principal"
	AuthTypeDisplayAWSAccount            = "AWS Account"
	AuthTypeDisplayGoogleCloud           = "Google Cloud Account"
	AuthTypeDisplayClientCertificate     = "Client Certificate"
	AuthTypeDisplayPodServiceAccount     = "Pod Service Account"

	AuthTypeUsernameAndPassword   = "UsernamePassword"
	AuthTypeToken                 = "Token"
	AuthTypeAzureServicePrincipal = "AzureServicePrincipal"
	AuthTypeAWSAccount            = "AmazonWebServicesAccount"
	AuthTypeGoogleCloud           = "GoogleCloudAccount"
	AuthTypeClientCertificate     = "KubernetesCertificate"
	AuthTypePodServiceAccount     = "KubernetesPodService"
)

const (
	ClusterTypeRegional = "Regional"
	ClusterTypeZonal    = "Zonal"
)

var AuthenticationTypesDisplay = []string{
	AuthTypeDisplayUsernameAndPassword,
	AuthTypeDisplayToken,
	AuthTypeDisplayAzureServicePrincipal,
	AuthTypeDisplayAWSAccount,
	AuthTypeDisplayGoogleCloud,
	AuthTypeDisplayClientCertificate,
	AuthTypeDisplayPodServiceAccount,
}

type CreateFlags struct {
	Name               *flag.Flag[string]
	AuthenticationType *flag.Flag[string]
	Account            *flag.Flag[string]

	// Azure Service Principal
	AKSClusterName       *flag.Flag[string]
	AKSResourceGroupName *flag.Flag[string]
	UseAdminCredentials  *flag.Flag[bool]

	// AWS Account
	UseServiceRole             *flag.Flag[bool]
	AssumeServiceRole          *flag.Flag[bool]
	AssumedRoleARN             *flag.Flag[string]
	AssumedRoleSessionName     *flag.Flag[string]
	AssumedRoleSessionDuration *flag.Flag[int]
	AssumedRoleExternalID      *flag.Flag[string]
	EKSClusterName             *flag.Flag[string]

	// Google Cloud Account
	UseVMServiceAccount       *flag.Flag[bool]
	ImpersonateServiceAccount *flag.Flag[bool]
	ServiceAccountEmails      *flag.Flag[string]
	GKEClusterName            *flag.Flag[string]
	Project                   *flag.Flag[string]
	Zone                      *flag.Flag[string]
	Region                    *flag.Flag[string]

	ClientCertificate *flag.Flag[string]

	// Pod Service Account
	TokenFilePath *flag.Flag[string]

	SkipTLSVerification  *flag.Flag[bool]
	KubernetesClusterURL *flag.Flag[string]
	KubernetesNamespace  *flag.Flag[string]
	Certificate          *flag.Flag[string]
	CertificateFilePath  *flag.Flag[string]

	ContainerRegistry *flag.Flag[string]
	ImageFlags        *flag.Flag[string]

	*shared.CreateTargetEnvironmentFlags
	*shared.CreateTargetRoleFlags
	*machinescommon.CreateTargetMachinePolicyFlags
	*shared.WorkerPoolFlags
	*shared.CreateTargetTenantFlags
	*machinescommon.WebFlags
}

type CreateOptions struct {
	GetUsernamePasswordAccountsCallback GetAccountsCallback
	GetTokenAccountsCallback            GetAccountsCallback
	GetAzureServiceAccountsCallback     GetAccountsCallback
	GetGCPAccountsCallback              GetAccountsCallback
	GetAWSAccountsCallback              GetAccountsCallback
	GetCertificatesCallback             GetCertificatesCallback
	GetFeedsCallback                    GetFeedsCallback
	*CreateFlags
	*shared.CreateTargetEnvironmentOptions
	*shared.CreateTargetRoleOptions
	*shared.WorkerPoolOptions
	*shared.CreateTargetTenantOptions
	*machinescommon.CreateTargetMachinePolicyOptions
	*cmd.Dependencies
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Name:               flag.New[string](FlagName, false),
		AuthenticationType: flag.New[string](FlagAuthenticationType, false),
		Account:            flag.New[string](FlagAccount, false),

		// Azure Service Principal
		AKSClusterName:       flag.New[string](FlagAKSClusterName, false),
		AKSResourceGroupName: flag.New[string](FlagAKSResourceGroupName, false),
		UseAdminCredentials:  flag.New[bool](FlagUseAdminCredentials, false),

		// AWS Account
		UseServiceRole:             flag.New[bool](FlagUseServiceRole, false),
		AssumeServiceRole:          flag.New[bool](FlagAssumeServiceRole, false),
		AssumedRoleARN:             flag.New[string](FlagAssumedRoleARN, false),
		AssumedRoleSessionName:     flag.New[string](FlagAssumedRoleSessionName, false),
		AssumedRoleSessionDuration: flag.New[int](FlagAssumedRoleSessionDuration, false),
		AssumedRoleExternalID:      flag.New[string](FlagAssumedRoleExternalID, false),
		EKSClusterName:             flag.New[string](FlagEKSClusterName, false),

		// Google Cloud Account
		UseVMServiceAccount:       flag.New[bool](FlagUseVMServiceAccount, false),
		ImpersonateServiceAccount: flag.New[bool](FlagImpersonateServiceAccount, false),
		ServiceAccountEmails:      flag.New[string](FlagServiceAccountEmails, false),
		GKEClusterName:            flag.New[string](FlagGKEClusterName, false),
		Project:                   flag.New[string](FlagProject, false),
		Zone:                      flag.New[string](FlagZone, false),
		Region:                    flag.New[string](FlagRegion, false),

		ClientCertificate: flag.New[string](FlagClientCertificate, false),

		// Pod Service Account
		TokenFilePath: flag.New[string](FlagTokenFilePath, false),

		SkipTLSVerification:  flag.New[bool](FlagSkipTLSVerification, false),
		KubernetesClusterURL: flag.New[string](FlagKubernetesClusterURL, false),
		KubernetesNamespace:  flag.New[string](FlagKubernetesNamespace, false),
		Certificate:          flag.New[string](FlagCertificate, false),
		CertificateFilePath:  flag.New[string](FlagCertificateFilePath, false),

		ContainerRegistry: flag.New[string](FlagHealthCheckContainerRegistry, false),
		ImageFlags:        flag.New[string](FlagHealthCheckTags, false),

		CreateTargetRoleFlags:          shared.NewCreateTargetRoleFlags(),
		CreateTargetEnvironmentFlags:   shared.NewCreateTargetEnvironmentFlags(),
		WebFlags:                       machinescommon.NewWebFlags(),
		WorkerPoolFlags:                shared.NewWorkerPoolFlags(),
		CreateTargetTenantFlags:        shared.NewCreateTargetTenantFlags(),
		CreateTargetMachinePolicyFlags: machinescommon.NewCreateTargetMachinePolicyFlags(),
	}
}

func NewCreateOptions(createFlags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:                         createFlags,
		Dependencies:                        dependencies,
		CreateTargetRoleOptions:             shared.NewCreateTargetRoleOptions(dependencies),
		CreateTargetEnvironmentOptions:      shared.NewCreateTargetEnvironmentOptions(dependencies),
		GetUsernamePasswordAccountsCallback: CreateGetAccountsCallback(dependencies.Client, accounts.AccountTypeUsernamePassword),
		GetTokenAccountsCallback:            CreateGetAccountsCallback(dependencies.Client, accounts.AccountTypeToken),
		GetAzureServiceAccountsCallback:     CreateGetAccountsCallback(dependencies.Client, accounts.AccountTypeAzureServicePrincipal),
		GetGCPAccountsCallback:              CreateGetAccountsCallback(dependencies.Client, accounts.AccountTypeGoogleCloudPlatformAccount),
		GetAWSAccountsCallback:              CreateGetAccountsCallback(dependencies.Client, accounts.AccountTypeAmazonWebServicesAccount),
		GetCertificatesCallback:             CreateGetCertificatesCallback(dependencies.Client),
		GetFeedsCallback:                    CreateGetFeedsCallback(dependencies.Client),
		CreateTargetTenantOptions:           shared.NewCreateTargetTenantOptions(dependencies),
		CreateTargetMachinePolicyOptions:    machinescommon.NewCreateTargetMachinePolicyOptions(dependencies),
		WorkerPoolOptions:                   shared.NewWorkerPoolOptionsForCreateTarget(dependencies),
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a Kubernetes deployment target",
		Long:    "Create a Kubernetes deployment target in Octopus Deploy",
		Example: heredoc.Docf("$ %s deployment-target kubernetes create", constants.ExecutableName),
		Aliases: []string{"new"},
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c))

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "A short, memorable, unique name for this deployment target.")
	flags.StringVar(&createFlags.AuthenticationType.Value, createFlags.AuthenticationType.Name, "", "The authentication type to use.")
	flags.StringVar(&createFlags.Account.Value, createFlags.Account.Name, "", "The name of the account to use for authentication.")

	// Azure Service Principal
	flags.StringVar(&createFlags.AKSClusterName.Value, createFlags.AKSClusterName.Name, "", "The AKS cluster name.")
	flags.StringVar(&createFlags.AKSResourceGroupName.Value, createFlags.AKSResourceGroupName.Name, "", "The AKS resource group name.")
	flags.BoolVar(&createFlags.UseAdminCredentials.Value, createFlags.UseAdminCredentials.Name, false, "Enabling this option passes the --admin flag to az aks get-credentials. This is useful for AKS clusters with Azure Active Directory integration.")

	// AWS Account
	flags.BoolVar(&createFlags.UseServiceRole.Value, createFlags.UseServiceRole.Name, false, "Execute using the AWS service role for an EC2 instance.")
	flags.BoolVar(&createFlags.AssumeServiceRole.Value, createFlags.AssumeServiceRole.Name, false, "Assume a different AWS service role.")
	flags.StringVar(&createFlags.AssumedRoleARN.Value, createFlags.AssumedRoleARN.Name, "", "ARN of assumed AWS service role.")
	flags.StringVar(&createFlags.AssumedRoleSessionName.Value, createFlags.AssumedRoleSessionName.Name, "", "Session name of assumed AWS service role.")
	// Durations default is set on the struct sent to server, not here.
	// This is to prevent the auto cmd generator from showing this flag when not explicitly set.
	flags.IntVar(&createFlags.AssumedRoleSessionDuration.Value, createFlags.AssumedRoleSessionDuration.Name, 0, "AWS assumed role session duration in seconds. (defaults to 3600 seconds, 1 hour)")
	flags.StringVar(&createFlags.AssumedRoleExternalID.Value, createFlags.AssumedRoleExternalID.Name, "", "AWS assumed role external ID.")
	flags.StringVar(&createFlags.EKSClusterName.Value, createFlags.EKSClusterName.Name, "", "AWS EKS Cluster Name")

	// Google Cloud Account
	flags.BoolVar(&createFlags.UseVMServiceAccount.Value, createFlags.UseVMServiceAccount.Name, false, "When running in a Compute Engine virtual machine, use the associated VM service account.")
	flags.BoolVar(&createFlags.ImpersonateServiceAccount.Value, createFlags.ImpersonateServiceAccount.Name, false, "Impersonate service accounts.")
	flags.StringVar(&createFlags.ServiceAccountEmails.Value, createFlags.ServiceAccountEmails.Name, "", "Service Account Email.")
	flags.StringVar(&createFlags.GKEClusterName.Value, createFlags.GKEClusterName.Name, "", "GKE Cluster Name.")
	flags.StringVar(&createFlags.Project.Value, createFlags.Project.Name, "", "GKE Project.")
	flags.StringVar(&createFlags.Zone.Value, createFlags.Zone.Name, "", "GKE Zone.")
	flags.StringVar(&createFlags.Region.Value, createFlags.Region.Name, "", "GKE Region.")

	// Pod Service Account
	flags.StringVar(&createFlags.TokenFilePath.Value, createFlags.TokenFilePath.Name, "", "The path to the token of the pod service account. The default value usually is: /var/run/secrets/kubernetes.io/serviceaccount/token")

	flags.StringVar(&createFlags.ClientCertificate.Value, createFlags.ClientCertificate.Name, "", "Name of client certificate in Octopus Deploy")

	flags.BoolVar(&createFlags.SkipTLSVerification.Value, createFlags.SkipTLSVerification.Name, false, "Skip the verification of the cluster certificate. This can only be provided if no cluster certificate is specified.")
	flags.StringVar(&createFlags.KubernetesClusterURL.Value, createFlags.KubernetesClusterURL.Name, "", "Kubernetes cluster URL. Must be an absolute URL. e.g. https://kubernetes.example.com")
	flags.StringVar(&createFlags.KubernetesNamespace.Value, createFlags.KubernetesNamespace.Name, "", "Kubernetes Namespace.")
	flags.StringVar(&createFlags.Certificate.Value, createFlags.Certificate.Name, "", "Name of Certificate in Octopus Deploy.")
	flags.StringVar(&createFlags.CertificateFilePath.Value, createFlags.CertificateFilePath.Name, "", "The path to the CA certificate of the cluster. The default value usually is: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt")

	flags.StringVar(&createFlags.ContainerRegistry.Value, createFlags.ContainerRegistry.Name, "", "The feed of the docker container registery to use if running health check in a container on the worker")

	flags.StringVar(&createFlags.ImageFlags.Value, createFlags.ImageFlags.Name, "", "The image (including the tag) to use from the container registery")

	shared.RegisterCreateTargetEnvironmentFlags(cmd, createFlags.CreateTargetEnvironmentFlags)
	shared.RegisterCreateTargetWorkerPoolFlags(cmd, createFlags.WorkerPoolFlags)
	shared.RegisterCreateTargetTenantFlags(cmd, createFlags.CreateTargetTenantFlags)
	shared.RegisterCreateTargetRoleFlags(cmd, createFlags.CreateTargetRoleFlags)
	machinescommon.RegisterWebFlag(cmd, createFlags.WebFlags)

	return cmd
}

func createRun(opts *CreateOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	return opts.Commit()
}

func (opts *CreateOptions) Commit() error {
	envs, err := executionscommon.FindEnvironments(opts.Client, opts.Environments.Value)
	if err != nil {
		return err
	}
	environmentIds := util.SliceTransform(envs, func(e *environments.Environment) string { return e.ID })

	kubernetesUrl, err := url.Parse(opts.KubernetesClusterURL.Value)
	if err != nil {
		return err
	}

	endpoint := machines.NewKubernetesEndpoint(kubernetesUrl)
	endpoint.Namespace = opts.KubernetesNamespace.Value
	endpoint.SkipTLSVerification = opts.SkipTLSVerification.Value

	if opts.WorkerPool.Value != "" {
		workerPoolId, err := shared.FindWorkerPoolId(opts.GetAllWorkerPoolsCallback, opts.WorkerPool.Value)
		if err != nil {
			return err
		}
		endpoint.DefaultWorkerPoolID = workerPoolId
	}

	if opts.Certificate.Value != "" {
		certID, err := QualifyCertificate(opts.Client, opts.Certificate.Value)
		if err != nil {
			return err
		}
		endpoint.ClusterCertificate = certID
	}

	if opts.CertificateFilePath.Value != "" {
		endpoint.ClusterCertificatePath = opts.CertificateFilePath.Value
	}

	if opts.AuthenticationType.Value == AuthTypeUsernameAndPassword ||
		opts.AuthenticationType.Value == AuthTypeToken {
		auth := machines.NewKubernetesStandardAuthentication("")
		accountID, err := QualifyAccount(opts.Client, opts.Account.Value)
		if err != nil {
			return err
		}
		auth.AccountID = accountID
		endpoint.Authentication = auth
	}

	if opts.AuthenticationType.Value == AuthTypeAzureServicePrincipal {
		auth := machines.NewKubernetesAzureAuthentication()
		accountID, err := QualifyAccount(opts.Client, opts.Account.Value)
		if err != nil {
			return err
		}
		auth.AccountID = accountID
		auth.ClusterName = opts.AKSClusterName.Value
		auth.ClusterResourceGroup = opts.AKSResourceGroupName.Value
		auth.AdminLogin = strconv.FormatBool(opts.UseAdminCredentials.Value)
		endpoint.Authentication = auth
	}

	if opts.AuthenticationType.Value == AuthTypeAWSAccount {
		auth := machines.NewKubernetesAwsAuthentication()
		if !opts.UseServiceRole.Value {
			accountID, err := QualifyAccount(opts.Client, opts.Account.Value)
			if err != nil {
				return err
			}
			auth.AccountID = accountID
		}
		if opts.AssumeServiceRole.Value {
			auth.AssumeRole = opts.AssumeServiceRole.Value
			auth.AssumedRoleARN = opts.AssumedRoleARN.Value
			auth.AssumedRoleSession = opts.AssumedRoleSessionName.Value
			if opts.AssumedRoleSessionDuration.Value == 0 {
				opts.AssumedRoleSessionDuration.Value = 3600
			}
			auth.AssumeRoleSessionDuration = opts.AssumedRoleSessionDuration.Value
			auth.AssumeRoleExternalID = opts.AssumedRoleExternalID.Value
		}
		auth.ClusterName = opts.EKSClusterName.Value
		endpoint.Authentication = auth
	}

	if opts.AuthenticationType.Value == AuthTypeGoogleCloud {
		auth := machines.NewKubernetesGcpAuthentication()
		if !opts.UseVMServiceAccount.Value {
			auth.UseVmServiceAccount = opts.UseVMServiceAccount.Value
			accountID, err := QualifyAccount(opts.Client, opts.Account.Value)
			if err != nil {
				return err
			}
			auth.AccountID = accountID
		}
		if opts.ImpersonateServiceAccount.Value {
			auth.ImpersonateServiceAccount = opts.ImpersonateServiceAccount.Value
			auth.ServiceAccountEmails = opts.ServiceAccountEmails.Value
		}
		auth.ClusterName = opts.GKEClusterName.Value
		auth.Project = opts.Project.Value
		if opts.Region.Value != "" {
			auth.Region = opts.Region.Value
		}
		if opts.Zone.Value != "" {
			auth.Zone = opts.Zone.Value
		}
		endpoint.Authentication = auth
	}

	if opts.AuthenticationType.Value == AuthTypeClientCertificate {
		auth := machines.NewKubernetesCertificateAuthentication()
		certificateID, err := QualifyCertificate(opts.Client, opts.ClientCertificate.Value)
		if err != nil {
			return err
		}
		auth.ClientCertificate = certificateID
		endpoint.Authentication = auth
	}

	if opts.AuthenticationType.Value == AuthTypePodServiceAccount {
		auth := machines.NewKubernetesPodAuthentication()
		auth.TokenPath = opts.TokenFilePath.Value
		endpoint.Authentication = auth
	}

	deploymentTarget := machines.NewDeploymentTarget(opts.Name.Value, endpoint, environmentIds, util.SliceDistinct(opts.Roles.Value))

	machinePolicy, err := machinescommon.FindDefaultMachinePolicy(opts.GetAllMachinePoliciesCallback)
	if err != nil {
		return err
	}
	deploymentTarget.MachinePolicyID = machinePolicy.GetID()

	err = shared.ConfigureTenant(deploymentTarget, opts.CreateTargetTenantFlags, opts.CreateTargetTenantOptions)
	if err != nil {
		return err
	}

	createdTarget, err := opts.Client.Machines.Add(deploymentTarget)
	if err != nil {
		return err
	}

	fmt.Fprintf(opts.Out, "Successfully created Kubernetes deployment target '%s'.\n", deploymentTarget.Name)
	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(
			opts.CmdPath,
			opts.Name,
			opts.AuthenticationType,
			opts.Account,

			// Azure Service Principal
			opts.AKSClusterName,
			opts.AKSResourceGroupName,
			opts.UseAdminCredentials,

			// AWS Account
			opts.UseServiceRole,
			opts.AssumeServiceRole,
			opts.AssumedRoleARN,
			opts.AssumedRoleSessionName,
			opts.AssumedRoleSessionDuration,
			opts.AssumedRoleExternalID,
			opts.EKSClusterName,

			// Google Cloud Account
			opts.UseVMServiceAccount,
			opts.ImpersonateServiceAccount,
			opts.ServiceAccountEmails,
			opts.GKEClusterName,
			opts.Project,
			opts.Zone,
			opts.Region,

			opts.ClientCertificate,

			// Pod Service Account
			opts.TokenFilePath,

			opts.SkipTLSVerification,
			opts.KubernetesClusterURL,
			opts.KubernetesNamespace,
			opts.Certificate,
			opts.CertificateFilePath,

			opts.ContainerRegistry,
			opts.ImageFlags,

			opts.Environments,
			opts.Roles,
			opts.TenantedDeploymentMode,
			opts.Tenants,
			opts.TenantTags,
			opts.WorkerPool,
		)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	machinescommon.DoWebForTargets(createdTarget, opts.Dependencies, opts.WebFlags, "kubernetes")
	return nil
}

func PromptMissing(opts *CreateOptions) error {
	err := question.AskName(opts.Ask, "", "Kubernetes", &opts.Name.Value)
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

	opts.AuthenticationType.Value, err = PromptForAuthenticationType(opts.Ask)
	if err != nil {
		return err
	}

	err = PromptForAuthTypeInfo(opts)
	if err != nil {
		return err
	}

	err = shared.PromptForWorkerPool(opts.WorkerPoolOptions, opts.WorkerPoolFlags)
	if err != nil {
		return err
	}

	err = PromptForHealthCheck(opts)
	if err != nil {
		return err
	}

	err = shared.PromptForTenant(opts.CreateTargetTenantOptions, opts.CreateTargetTenantFlags)
	if err != nil {
		return err
	}

	return nil
}

func PromptForAuthenticationType(ask question.Asker) (string, error) {
	authType := ""
	err := ask(&survey.Select{
		Message: "Select an authentication type.",
		Options: AuthenticationTypesDisplay,
	}, &authType)
	if err != nil {
		return "", err
	}
	return QualifyAuthType(authType)
}

func PromptForAuthTypeInfo(opts *CreateOptions) error {
	switch opts.AuthenticationType.Value {
	case AuthTypeUsernameAndPassword:
		return PromptUsernamePassword(opts)
	case AuthTypeToken:
		return PromptToken(opts)
	case AuthTypeAzureServicePrincipal:
		return PromptAzureService(opts)
	case AuthTypeGoogleCloud:
		return PromptGCP(opts)
	case AuthTypeAWSAccount:
		return PromptAWS(opts)
	case AuthTypeClientCertificate:
		return PromptClientCert(opts)
	case AuthTypePodServiceAccount:
		return PromptPodService(opts)
	}
	return fmt.Errorf("auth type '%s' is not supported", opts.AuthenticationType.Value)
}

func PromptUsernamePassword(opts *CreateOptions) error {
	account, err := PromptAccount(opts.Ask, opts.Account.Value, opts.GetUsernamePasswordAccountsCallback)
	if err != nil {
		return err
	}
	opts.Account.Value = account

	return PromptKubernetesDetails(opts)
}

func PromptToken(opts *CreateOptions) error {
	acc, err := PromptAccount(opts.Ask, opts.Account.Value, opts.GetTokenAccountsCallback)
	if err != nil {
		return err
	}
	opts.Account.Value = acc

	return PromptKubernetesDetails(opts)
}

func PromptAzureService(opts *CreateOptions) error {
	acc, err := PromptAccount(opts.Ask, opts.Account.Value, opts.GetAzureServiceAccountsCallback)
	if err != nil {
		return err
	}
	opts.Account.Value = acc

	if opts.AKSClusterName.Value == "" {
		err = opts.Ask(&survey.Input{
			Message: "AKS Cluster Name",
		}, &opts.AKSClusterName.Value)
		if err != nil {
			return err
		}
	}

	if opts.AKSResourceGroupName.Value == "" {
		err = opts.Ask(&survey.Input{
			Message: "AKS Resource Group Name",
		}, &opts.AKSResourceGroupName.Value)
		if err != nil {
			return err
		}
	}

	if !opts.UseAdminCredentials.Value {
		err = opts.Ask(&survey.Confirm{
			Message: "Login with administrator credentials?",
			Default: false,
		}, &opts.UseAdminCredentials.Value)
		if err != nil {
			return err
		}
	}

	namespace, err := PromptKubernetesNamespace(opts.Ask, opts.KubernetesNamespace.Value)
	if err != nil {
		return err
	}
	opts.KubernetesNamespace.Value = namespace

	return nil
}

func PromptGCP(opts *CreateOptions) error {
	if !opts.UseVMServiceAccount.Value {
		err := opts.Ask(&survey.Confirm{
			Message: "When running in a Compute Engine virtual machine, use an associated VM service account?",
			Default: false,
		}, &opts.UseVMServiceAccount.Value)
		if err != nil {
			return err
		}
	}

	if !opts.UseVMServiceAccount.Value {
		acc, err := PromptAccount(opts.Ask, opts.Account.Value, opts.GetGCPAccountsCallback)
		if err != nil {
			return err
		}
		opts.Account.Value = acc
	}

	if !opts.ImpersonateServiceAccount.Value {
		err := opts.Ask(&survey.Confirm{
			Message: "Impersonate service accounts?",
			Default: false,
		}, &opts.ImpersonateServiceAccount.Value)
		if err != nil {
			return err
		}
	}

	if opts.ImpersonateServiceAccount.Value {
		if opts.ServiceAccountEmails.Value == "" {
			err := opts.Ask(&survey.Input{
				Message: "Service Account Emails",
			}, &opts.ServiceAccountEmails.Value)
			if err != nil {
				return err
			}
		}
	}

	if opts.GKEClusterName.Value == "" {
		err := opts.Ask(&survey.Input{
			Message: "GKE Cluster Name",
		}, &opts.GKEClusterName.Value)
		if err != nil {
			return err
		}
	}

	if opts.Project.Value == "" {
		err := opts.Ask(&survey.Input{
			Message: "Project",
		}, &opts.Project.Value)
		if err != nil {
			return err
		}
	}

	clusterType := ""
	err := opts.Ask(&survey.Select{
		Message: "Cluster Type",
		Options: []string{
			ClusterTypeRegional,
			ClusterTypeZonal,
		},
		Default: ClusterTypeRegional,
	}, &clusterType)
	if err != nil {
		return err
	}

	if clusterType == ClusterTypeRegional {
		if opts.Region.Value == "" {
			err := opts.Ask(&survey.Input{
				Message: "Region",
			}, &opts.Region.Value)
			if err != nil {
				return err
			}
		}
	}

	if clusterType == ClusterTypeZonal {
		if opts.Zone.Value == "" {
			err := opts.Ask(&survey.Input{
				Message: "Zone",
			}, &opts.Zone.Value)
			if err != nil {
				return err
			}
		}
	}

	namespace, err := PromptKubernetesNamespace(opts.Ask, opts.KubernetesNamespace.Value)
	if err != nil {
		return err
	}
	opts.KubernetesNamespace.Value = namespace

	return nil
}

func PromptAWS(opts *CreateOptions) error {
	if !opts.UseServiceRole.Value {
		err := opts.Ask(&survey.Confirm{
			Message: "Execute using the AWS service role for an EC2 instance?",
			Default: false,
		}, &opts.UseServiceRole.Value)
		if err != nil {
			return err
		}
	}

	if !opts.UseServiceRole.Value {
		acc, err := PromptAccount(opts.Ask, opts.Account.Value, opts.GetAWSAccountsCallback)
		if err != nil {
			return err
		}
		opts.Account.Value = acc
	}

	if !opts.AssumeServiceRole.Value {
		err := opts.Ask(&survey.Confirm{
			Message: "Assume a different AWS service role?",
			Default: false,
		}, &opts.AssumeServiceRole.Value)
		if err != nil {
			return err
		}
	}

	if opts.AssumeServiceRole.Value {
		if opts.AssumedRoleARN.Value == "" {
			err := opts.Ask(&survey.Input{
				Message: "Assumed Role ARN",
			}, &opts.AssumedRoleARN.Value)
			if err != nil {
				return err
			}
		}

		if opts.AssumedRoleSessionName.Value == "" {
			err := opts.Ask(&survey.Input{
				Message: "Assumed Role Session Name",
			}, &opts.AssumedRoleSessionName.Value)
			if err != nil {
				return err
			}
		}

		if opts.AssumedRoleSessionDuration.Value == 0 {
			duration := ""
			// Note: this could provide better UX with custom number validator
			err := opts.Ask(&survey.Input{
				Message: "Assumed Role Session Duration (In Seconds)",
				Default: "3600",
			}, &duration)
			if err != nil {
				return err
			}
			opts.AssumedRoleSessionDuration.Value, err = strconv.Atoi(duration)
			if err != nil {
				return err
			}
		}

		if opts.AssumedRoleExternalID.Value == "" {
			err := opts.Ask(&survey.Input{
				Message: "Assumed Role External ID",
			}, &opts.AssumedRoleExternalID.Value)
			if err != nil {
				return err
			}
		}
	}

	if opts.EKSClusterName.Value == "" {
		err := opts.Ask(&survey.Input{
			Message: "EKS Cluster Name",
		}, &opts.EKSClusterName.Value)
		if err != nil {
			return err
		}
	}

	return PromptKubernetesDetails(opts)
}

func PromptPodService(opts *CreateOptions) error {
	if opts.TokenFilePath.Value == "" {
		err := opts.Ask(&survey.Input{
			Message: "File Token Path",
			Help:    "The path to the token of the pod service account. The default value usually is: /var/run/secrets/kubernetes.io/serviceaccount/token",
		}, &opts.TokenFilePath.Value)
		if err != nil {
			return err
		}
	}

	url, err := PromptClusterURL(opts.Ask, opts.KubernetesClusterURL.Value)
	if err != nil {
		return err
	}
	opts.KubernetesClusterURL.Value = url

	cert, err := PromptCertificatePath(opts.Ask, opts.CertificateFilePath.Value)
	if err != nil {
		return err
	}
	opts.CertificateFilePath.Value = cert

	skipTLS, err := PromptSkipTLS(opts.Ask, opts.SkipTLSVerification.Value)
	if err != nil {
		return err
	}
	opts.SkipTLSVerification.Value = skipTLS

	namespace, err := PromptKubernetesNamespace(opts.Ask, opts.KubernetesNamespace.Value)
	if err != nil {
		return err
	}
	opts.KubernetesNamespace.Value = namespace

	return PromptKubernetesDetails(opts)
}

func PromptClientCert(opts *CreateOptions) error {
	cert, err := PromptCertificate(opts.Ask, opts.ClientCertificate.Value, opts.GetCertificatesCallback)
	if err != nil {
		return err
	}
	opts.ClientCertificate.Value = cert

	return PromptKubernetesDetails(opts)
}

func PromptAccount(ask question.Asker, acc string, GetAccounts GetAccountsCallback) (string, error) {
	if acc != "" {
		return acc, nil
	}
	account, err := selectors.Select(ask, "Select Account", GetAccounts,
		func(item accounts.IAccount) string {
			return item.GetName()
		})
	if err != nil {
		return "", err
	}
	return account.GetName(), nil
}

func PromptKubernetesDetails(opts *CreateOptions) error {
	url, err := PromptClusterURL(opts.Ask, opts.KubernetesClusterURL.Value)
	if err != nil {
		return err
	}
	opts.KubernetesClusterURL.Value = url

	cert, err := PromptCertificate(opts.Ask, opts.Certificate.Value, opts.GetCertificatesCallback)
	if err != nil {
		return err
	}
	opts.Certificate.Value = cert

	skipTLS, err := PromptSkipTLS(opts.Ask, opts.SkipTLSVerification.Value)
	if err != nil {
		return err
	}
	opts.SkipTLSVerification.Value = skipTLS

	namespace, err := PromptKubernetesNamespace(opts.Ask, opts.KubernetesNamespace.Value)
	if err != nil {
		return err
	}
	opts.KubernetesNamespace.Value = namespace

	return nil
}

func PromptClusterURL(ask question.Asker, url string) (string, error) {
	if url != "" {
		return url, nil
	}
	err := ask(&survey.Input{
		Message: "Kubernetes cluster URL",
		Help:    "Must be an absolute URL. e.g. https://kubernetes.example.com",
	}, &url)
	return url, err
}

func PromptSkipTLS(ask question.Asker, skipTLS bool) (bool, error) {
	if skipTLS {
		return skipTLS, nil
	}
	err := ask(&survey.Confirm{
		Message: "Skip TLS Verification",
		Help:    "Enable this option to skip the verification of the cluster certificate. This can only be selected if no cluster certificate is specified.",
		Default: false,
	}, &skipTLS)
	return skipTLS, err
}

func PromptKubernetesNamespace(ask question.Asker, namespace string) (string, error) {
	if namespace != "" {
		return namespace, nil
	}
	err := ask(&survey.Input{
		Message: "Kubernetes Namespace",
	}, &namespace)
	return namespace, err
}

func PromptCertificate(ask question.Asker, cert string, GetCerts GetCertificatesCallback) (string, error) {
	if cert != "" {
		return cert, nil
	}
	certificate, err := selectors.Select(ask, "Select Certificate", GetCerts, func(item *certificates.CertificateResource) string {
		return item.Name
	})
	if err != nil {
		return "", err
	}
	return certificate.Name, nil
}

func PromptCertificatePath(ask question.Asker, certificatePath string) (string, error) {
	if certificatePath != "" {
		return certificatePath, nil
	}
	err := ask(&survey.Input{
		Message: "Kubernetes Certificate File Path",
	}, &certificatePath)
	return certificatePath, err
}

func PromptForHealthCheck(opts *CreateOptions) error {
	runOnWorker := false
	err := opts.Ask(&survey.Confirm{
		Message: "Should health check run in a container on the worker?",
		Default: false,
	}, &runOnWorker)
	if err != nil {
		return err
	}
	if !runOnWorker {
		return nil
	}

	feed, err := PromptContainerRegistry(opts.Ask, opts.GetFeedsCallback)
	if err != nil {
		return nil
	}
	opts.ContainerRegistry.Value = feed

	return nil
}

func PromptContainerRegistry(ask question.Asker, getFeedsCallback GetFeedsCallback) (string, error) {
	feed, err := selectors.Select(ask, "Container Registery", getFeedsCallback, func(item feeds.IFeed) string {
		return item.GetName()
	})
	if err != nil {
		return "", err
	}
	return feed.GetName(), nil
}

func QualifyAuthType(authType string) (string, error) {
	switch authType {
	case AuthTypeDisplayUsernameAndPassword:
		return AuthTypeUsernameAndPassword, nil
	case AuthTypeDisplayToken:
		return AuthTypeToken, nil
	case AuthTypeDisplayAzureServicePrincipal:
		return AuthTypeAzureServicePrincipal, nil
	case AuthTypeDisplayGoogleCloud:
		return AuthTypeGoogleCloud, nil
	case AuthTypeDisplayAWSAccount:
		return AuthTypeAWSAccount, nil
	case AuthTypeDisplayClientCertificate:
		return AuthTypeClientCertificate, nil
	case AuthTypeDisplayPodServiceAccount:
		return AuthTypePodServiceAccount, nil
	}
	return "", fmt.Errorf("auth type '%s' is not supported", authType)
}

func QualifyAccount(octopus *client.Client, account string) (string, error) {
	accs, err := octopus.Accounts.Get(accounts.AccountsQuery{
		PartialName: account,
	})
	if err != nil {
		return "", err
	}

	allMatchAccs, err := accs.GetAllPages(octopus.Sling())
	if err != nil {
		return "", err
	}

	accountID := ""
	for i := range allMatchAccs {
		if strings.EqualFold(allMatchAccs[i].GetName(), account) {
			accountID = allMatchAccs[i].GetID()
			break
		}
	}
	if accountID == "" {
		return "", fmt.Errorf("could not qualify ID for the account '%s'", account)
	}

	return accountID, nil
}

func QualifyCertificate(octopus *client.Client, certificate string) (string, error) {
	accs, err := octopus.Certificates.Get(certificates.CertificatesQuery{
		PartialName: certificate,
	})
	if err != nil {
		return "", err
	}

	allMatchCerts, err := accs.GetAllPages(octopus.Sling())
	if err != nil {
		return "", err
	}

	accountID := ""
	for i := range allMatchCerts {
		if strings.EqualFold(allMatchCerts[i].Name, certificate) {
			accountID = allMatchCerts[i].GetID()
			break
		}
	}
	if accountID == "" {
		return "", fmt.Errorf("could not qualify ID for the certificate '%s'", certificate)
	}

	return accountID, nil
}

type GetAccountsCallback = func() ([]accounts.IAccount, error)

func CreateGetAccountsCallback(octopus *client.Client, accountType accounts.AccountType) GetAccountsCallback {
	return func() ([]accounts.IAccount, error) {
		acc, err := octopus.Accounts.Get(accounts.AccountsQuery{
			AccountType: accountType,
		})
		if err != nil {
			return nil, err
		}
		return acc.GetAllPages(octopus.Sling())
	}
}

type GetCertificatesCallback = func() ([]*certificates.CertificateResource, error)

func CreateGetCertificatesCallback(octopus *client.Client) GetCertificatesCallback {
	return func() ([]*certificates.CertificateResource, error) {
		certs, err := octopus.Certificates.Get(certificates.CertificatesQuery{})
		if err != nil {
			return nil, err
		}
		return certs.GetAllPages(octopus.Sling())
	}
}

type GetFeedsCallback = func() ([]feeds.IFeed, error)

func CreateGetFeedsCallback(octopus *client.Client) GetFeedsCallback {
	return func() ([]feeds.IFeed, error) {
		feedsResource, err := octopus.Feeds.Get(feeds.FeedsQuery{
			FeedType: string(feeds.FeedTypeDocker),
			Take:     999,
		})
		if err != nil {
			return nil, err
		}
		return feedsResource.Items, nil
	}
}
