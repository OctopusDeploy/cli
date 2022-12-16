package create_test

import (
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	k8s "github.com/OctopusDeploy/cli/pkg/cmd/target/kubernetes/create"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/certificates"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/stretchr/testify/assert"
)

var (
	testAccountNames = []string{
		"Account1",
		"Account2",
	}

	testCertNames = []string{
		"Cert1",
		"Cert2",
	}
)

func newTestOpts(ask question.Asker, flags *k8s.CreateFlags) *k8s.CreateOptions {
	return &k8s.CreateOptions{
		Dependencies:                        &cmd.Dependencies{Ask: ask},
		CreateFlags:                         flags,
		GetUsernamePasswordAccountsCallback: getAccount,
		GetTokenAccountsCallback:            getAccount,
		GetAzureServiceAccountsCallback:     getAccount,
		GetGCPAccountsCallback:              getAccount,
		GetAWSAccountsCallback:              getAccount,
		GetCertificatesCallback:             getCerts,
	}
}

func TestAuthType_UsernamePassword(t *testing.T) {
	authType := k8s.AuthTypeUsernameAndPassword
	pa := append([]*testutil.PA{
		testutil.NewSelectPrompt("Select Account", "", testAccountNames, testAccountNames[0]),
	}, getK8sStandaredDetailsPA()...)

	flags := k8s.NewCreateFlags()
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	opts := newTestOpts(asker, flags)
	opts.AuthenticationType.Value = authType

	k8s.PromptForAuthTypeInfo(opts)

	checkRemainingPrompts()

	assert.Equal(t, testAccountNames[0], opts.Account.Value)
	assertK8sStandaredDetials(t, flags)
}

func TestAuthType_Token(t *testing.T) {
	authType := k8s.AuthTypeToken
	pa := append([]*testutil.PA{
		testutil.NewSelectPrompt("Select Account", "", testAccountNames, testAccountNames[0]),
	}, getK8sStandaredDetailsPA()...)

	flags := k8s.NewCreateFlags()
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	opts := newTestOpts(asker, flags)
	opts.AuthenticationType.Value = authType

	k8s.PromptForAuthTypeInfo(opts)

	checkRemainingPrompts()
	assert.Equal(t, testAccountNames[0], opts.Account.Value)
	assertK8sStandaredDetials(t, flags)
}

func TestAuthType_Azure(t *testing.T) {
	authType := k8s.AuthTypeAzureServicePrincipal
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select Account", "", testAccountNames, testAccountNames[0]),
		testutil.NewInputPrompt("AKS Cluster Name", "", "cluster name"),
		testutil.NewInputPrompt("AKS Resource Group Name", "", "group name"),
		testutil.NewConfirmPrompt("Login with administrator credentials?", "", true),
		testutil.NewInputPrompt("Kubernetes Namespace", "", "space"),
	}

	flags := k8s.NewCreateFlags()
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	opts := newTestOpts(asker, flags)
	opts.AuthenticationType.Value = authType

	k8s.PromptForAuthTypeInfo(opts)

	checkRemainingPrompts()
	assert.Equal(t, testAccountNames[0], opts.Account.Value)
	assert.Equal(t, "cluster name", opts.AKSClusterName.Value)
	assert.Equal(t, "group name", opts.AKSResourceGroupName.Value)
	assert.Equal(t, true, opts.UseAdminCredentials.Value)
	assert.Equal(t, "space", opts.KubernetesNamespace.Value)
}

func TestAuthType_GCP(t *testing.T) {
	authType := k8s.AuthTypeGoogleCloud
	pa := []*testutil.PA{
		testutil.NewConfirmPrompt("When running in a Compute Engine virtual machine, use an associated VM service account?", "", true),
		testutil.NewConfirmPrompt("Impersonate service accounts?", "", false),
		testutil.NewInputPrompt("GKE Cluster Name", "", "cluster"),
		testutil.NewInputPrompt("Project", "", "project"),
		{
			Prompt: &survey.Select{
				Message: "Cluster Type",
				Options: []string{
					k8s.ClusterTypeRegional,
					k8s.ClusterTypeZonal,
				},
				Default: k8s.ClusterTypeRegional,
			},
			Answer: k8s.ClusterTypeZonal,
		},
		testutil.NewInputPrompt("Zone", "", "zone"),
		testutil.NewInputPrompt("Kubernetes Namespace", "", "space"),
	}

	flags := k8s.NewCreateFlags()
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	opts := newTestOpts(asker, flags)
	opts.AuthenticationType.Value = authType

	k8s.PromptForAuthTypeInfo(opts)

	checkRemainingPrompts()
	assert.Equal(t, true, opts.UseVMServiceAccount.Value)
	assert.Equal(t, false, opts.ImpersonateServiceAccount.Value)
	assert.Equal(t, "cluster", opts.GKEClusterName.Value)
	assert.Equal(t, "project", opts.Project.Value)
	assert.Equal(t, "zone", opts.Zone.Value)
	assert.Equal(t, "space", opts.KubernetesNamespace.Value)
}

func TestAuthType_AWS(t *testing.T) {
	authType := k8s.AuthTypeAWSAccount
	pa := append([]*testutil.PA{
		testutil.NewConfirmPrompt("Execute using the AWS service role for an EC2 instance?", "", false),
		testutil.NewSelectPrompt("Select Account", "", testAccountNames, testAccountNames[0]),
		testutil.NewConfirmPrompt("Assume a different AWS service role?", "", true),
		testutil.NewInputPrompt("Assumed Role ARN", "", "arn"),
		testutil.NewInputPrompt("Assumed Role Session Name", "", "role"),
		{
			Prompt: &survey.Input{
				Message: "Assumed Role Session Duration (In Seconds)",
				Default: "3600",
			},
			Answer: "3600",
		},
		testutil.NewInputPrompt("Assumed Role External ID", "", "extern id"),
		testutil.NewInputPrompt("EKS Cluster Name", "", "eks cluster"),
	}, getK8sStandaredDetailsPA()...)

	flags := k8s.NewCreateFlags()
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	opts := newTestOpts(asker, flags)
	opts.AuthenticationType.Value = authType

	k8s.PromptForAuthTypeInfo(opts)

	checkRemainingPrompts()
	assert.Equal(t, false, opts.UseServiceRole.Value)
	assert.Equal(t, testAccountNames[0], opts.Account.Value)
	assert.Equal(t, true, opts.AssumeServiceRole.Value)
	assert.Equal(t, "arn", opts.AssumedRoleARN.Value)
	assert.Equal(t, "role", opts.AssumedRoleSessionName.Value)
	assert.Equal(t, 3600, opts.AssumedRoleSessionDuration.Value)
	assert.Equal(t, "extern id", opts.AssumedRoleExternalID.Value)
	assert.Equal(t, "eks cluster", opts.EKSClusterName.Value)
	assertK8sStandaredDetials(t, flags)
}

func getAccount() ([]accounts.IAccount, error) {
	return []accounts.IAccount{
		accounts.NewAccountResource(testAccountNames[0], accounts.AccountTypeNone),
		accounts.NewAccountResource(testAccountNames[1], accounts.AccountTypeNone),
	}, nil
}

func getCerts() ([]*certificates.CertificateResource, error) {
	return []*certificates.CertificateResource{
		certificates.NewCertificateResource(testCertNames[0], core.NewSensitiveValue(""), core.NewSensitiveValue("")),
		certificates.NewCertificateResource(testCertNames[1], core.NewSensitiveValue(""), core.NewSensitiveValue("")),
	}, nil
}

func getK8sStandaredDetailsPA() []*testutil.PA {
	return []*testutil.PA{
		testutil.NewInputPrompt("Kubernetes cluster URL", "Must be an absolute URL. e.g. https://kubernetes.example.com", "https://octopus.com"),
		testutil.NewSelectPrompt("Select Certificate", "", testCertNames, testCertNames[1]),
		testutil.NewConfirmPrompt("Skip TLS Verification", "Enable this option to skip the verification of the cluster certificate. This can only be selected if no cluster certificate is specified.", false),
		testutil.NewInputPrompt("Kubernetes Namespace", "", "space"),
	}
}

func assertK8sStandaredDetials(t *testing.T, flags *k8s.CreateFlags) {
	assert.Equal(t, "https://octopus.com", flags.KubernetesClusterURL.Value)
	assert.Equal(t, testCertNames[1], flags.Certificate.Value)
	assert.Equal(t, false, flags.SkipTLSVerification.Value)
	assert.Equal(t, "space", flags.KubernetesNamespace.Value)
}
