package argocd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
)

// DefaultAccountName is a local account: Argo CD has no service-account
// concept. With only the apiKey capability it cannot log in to the web UI.
const DefaultAccountName = "octopus"

const accountEnabledSuffix = "enabled"

type AccountSpec struct {
	Name string
	// AllowSync lets Octopus deploy through Argo CD rather than only observe it.
	AllowSync bool
}

func (s AccountSpec) RequiredPolicies() []string {
	policies := []string{
		fmt.Sprintf("p, %s, applications, get, *, allow", s.Name),
		fmt.Sprintf("p, %s, clusters, get, *, allow", s.Name),
		fmt.Sprintf("p, %s, logs, get, */*, allow", s.Name),
	}
	if s.AllowSync {
		policies = append(policies, fmt.Sprintf("p, %s, applications, sync, *, allow", s.Name))
	}
	sort.Strings(policies)
	return policies
}

type AccountStatus struct {
	Spec                AccountSpec
	HasAPIKeyCapability bool
	Disabled            bool
	MissingPolicies     []string
	// Operator is set when an operator generates Argo CD's ConfigMaps, in which
	// case the changes belong on its resource instead.
	Operator *OperatorInstance
}

func (s AccountStatus) IsComplete() bool {
	return s.HasAPIKeyCapability && !s.Disabled && len(s.MissingPolicies) == 0
}

func (s AccountStatus) Summary() string {
	if s.IsComplete() {
		return fmt.Sprintf("Argo CD account %q exists with the permissions Octopus needs", s.Spec.Name)
	}

	var missing []string
	if !s.HasAPIKeyCapability {
		missing = append(missing, fmt.Sprintf("the %q account with the apiKey capability", s.Spec.Name))
	} else if s.Disabled {
		missing = append(missing, fmt.Sprintf("the %q account is disabled", s.Spec.Name))
	}
	if len(s.MissingPolicies) > 0 {
		missing = append(missing, fmt.Sprintf("%d RBAC %s", len(s.MissingPolicies), octoK8s.Pluralise("policy", "policies", len(s.MissingPolicies))))
	}
	return "Argo CD is missing " + strings.Join(missing, " and ")
}

func InspectAccount(ctx context.Context, c *octoK8s.Cluster, instance Instance, spec AccountSpec) (AccountStatus, error) {
	namespace := instance.Namespace
	status := AccountStatus{Spec: spec, Operator: instance.Operator}

	cm, found, err := c.GetConfigMap(ctx, namespace, ConfigMapName)
	if err != nil {
		return AccountStatus{}, err
	}
	if found {
		capabilities := cm.Data["accounts."+spec.Name]
		status.HasAPIKeyCapability = containsCapability(capabilities, capabilityAPIKey)
		status.Disabled = strings.EqualFold(strings.TrimSpace(cm.Data["accounts."+spec.Name+".enabled"]), "false")
	}

	rbac, found, err := c.GetConfigMap(ctx, namespace, RBACConfigMapName)
	if err != nil {
		return AccountStatus{}, err
	}
	existing := ""
	if found {
		existing = rbac.Data["policy.csv"]
	}
	status.MissingPolicies = missingPolicies(existing, spec.RequiredPolicies())

	return status, nil
}

// AccountPatchPlan shows exactly what ConfigureAccount would write, for the
// user to agree to first.
func AccountPatchPlan(namespace string, status AccountStatus) string {
	var b strings.Builder

	if !status.HasAPIKeyCapability || status.Disabled {
		fmt.Fprintf(&b, "  %s/%s\n", namespace, ConfigMapName)
		fmt.Fprintf(&b, "    + accounts.%s: apiKey\n", status.Spec.Name)
		fmt.Fprintf(&b, "    + accounts.%s.enabled: \"true\"\n", status.Spec.Name)
	}

	if len(status.MissingPolicies) > 0 {
		fmt.Fprintf(&b, "  %s/%s\n", namespace, RBACConfigMapName)
		for _, p := range status.MissingPolicies {
			fmt.Fprintf(&b, "    + %s\n", p)
		}
	}

	return b.String()
}

// ConfigureAccount is safe against a partly configured Argo CD: existing
// accounts and policies are left alone.
func ConfigureAccount(ctx context.Context, c *octoK8s.Cluster, instance Instance, status AccountStatus) error {
	if status.Operator != nil {
		return configureViaOperator(ctx, c, instance, status)
	}

	namespace := instance.Namespace
	if !status.HasAPIKeyCapability || status.Disabled {
		if err := grantAPIKeyCapability(ctx, c, namespace, status.Spec.Name); err != nil {
			return err
		}
	}
	if len(status.MissingPolicies) > 0 {
		if err := addPolicies(ctx, c, namespace, status.MissingPolicies); err != nil {
			return err
		}
	}
	return nil
}

// configureViaOperator writes to the ArgoCD resource rather than the ConfigMaps
// it generates. Anything written straight to argocd-cm or argocd-rbac-cm is
// reverted the next time the operator reconciles.
func configureViaOperator(ctx context.Context, c *octoK8s.Cluster, instance Instance, status AccountStatus) error {
	resource := c.Dynamic.Resource(status.Operator.Resource).Namespace(instance.Namespace)

	argoCD, err := resource.Get(ctx, status.Operator.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("could not read the %s ArgoCD resource: %w", status.Operator.Name, err)
	}

	if !status.HasAPIKeyCapability || status.Disabled {
		extraConfig, _, err := unstructured.NestedStringMap(argoCD.Object, "spec", "extraConfig")
		if err != nil {
			return fmt.Errorf("could not read spec.extraConfig from the %s ArgoCD resource: %w", status.Operator.Name, err)
		}
		if extraConfig == nil {
			extraConfig = map[string]string{}
		}

		key := accountsKeyPrefix + status.Spec.Name
		extraConfig[key] = addCapability(extraConfig[key], capabilityAPIKey)
		extraConfig[key+"."+accountEnabledSuffix] = "true"

		if err := unstructured.SetNestedStringMap(argoCD.Object, extraConfig, "spec", "extraConfig"); err != nil {
			return err
		}
	}

	if len(status.MissingPolicies) > 0 {
		existing, _, err := unstructured.NestedString(argoCD.Object, "spec", "rbac", "policy")
		if err != nil {
			return fmt.Errorf("could not read spec.rbac.policy from the %s ArgoCD resource: %w", status.Operator.Name, err)
		}
		if err := unstructured.SetNestedField(argoCD.Object,
			appendPolicies(existing, status.MissingPolicies), "spec", "rbac", "policy"); err != nil {
			return err
		}
	}

	if _, err := resource.Update(ctx, argoCD, metav1.UpdateOptions{}); err != nil {
		if apierrors.IsConflict(err) {
			return fmt.Errorf("the %s ArgoCD resource changed while it was being updated; try again", status.Operator.Name)
		}
		return fmt.Errorf("could not update the %s ArgoCD resource: %w", status.Operator.Name, err)
	}
	return nil
}

// grantAPIKeyCapability sends only the two keys Octopus owns, so nothing else
// in the ConfigMap can be disturbed.
func grantAPIKeyCapability(ctx context.Context, c *octoK8s.Cluster, namespace, accountName string) error {
	cm, found, err := c.GetConfigMap(ctx, namespace, ConfigMapName)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("ConfigMap %s/%s does not exist, so this does not look like a complete Argo CD installation", namespace, ConfigMapName)
	}

	updated := cm.DeepCopy()
	if updated.Data == nil {
		updated.Data = map[string]string{}
	}

	capabilities := updated.Data["accounts."+accountName]
	if !containsCapability(capabilities, capabilityAPIKey) {
		updated.Data["accounts."+accountName] = addCapability(capabilities, capabilityAPIKey)
	}
	updated.Data["accounts."+accountName+".enabled"] = "true"

	if err := updateConfigMap(ctx, c, updated); err != nil {
		return fmt.Errorf("could not add the %q account to %s/%s: %w", accountName, namespace, ConfigMapName, err)
	}
	return nil
}

// updateConfigMap relies on the resourceVersion that was read, so a concurrent
// change conflicts rather than being silently overwritten.
func updateConfigMap(ctx context.Context, c *octoK8s.Cluster, cm *corev1.ConfigMap) error {
	_, err := c.Clientset.CoreV1().ConfigMaps(cm.Namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) {
		return fmt.Errorf("%s/%s changed while it was being updated; try again", cm.Namespace, cm.Name)
	}
	return err
}

// addPolicies is a read-modify-write rather than a patch because policy.csv is
// one multi-line value: a patch would replace every rule in it, including rules
// Octopus did not write.
func addPolicies(ctx context.Context, c *octoK8s.Cluster, namespace string, policies []string) error {
	configMaps := c.Clientset.CoreV1().ConfigMaps(namespace)

	cm, found, err := c.GetConfigMap(ctx, namespace, RBACConfigMapName)
	if err != nil {
		return err
	}

	if !found {
		created := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: RBACConfigMapName, Namespace: namespace},
			Data:       map[string]string{"policy.csv": strings.Join(policies, "\n") + "\n"},
		}
		if _, err := configMaps.Create(ctx, created, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("could not create %s/%s: %w", namespace, RBACConfigMapName, err)
		}
		return nil
	}

	updated := cm.DeepCopy()
	if updated.Data == nil {
		updated.Data = map[string]string{}
	}
	updated.Data["policy.csv"] = appendPolicies(updated.Data["policy.csv"], policies)

	// The resourceVersion that was read makes a concurrent edit conflict rather
	// than silently discard someone else's rules.
	if _, err := configMaps.Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
		if apierrors.IsConflict(err) {
			return fmt.Errorf("%s/%s changed while it was being updated; re-run the install to try again", namespace, RBACConfigMapName)
		}
		return fmt.Errorf("could not add RBAC policies to %s/%s: %w", namespace, RBACConfigMapName, err)
	}
	return nil
}

func appendPolicies(existing string, policies []string) string {
	trimmed := strings.TrimRight(existing, "\n")
	if trimmed == "" {
		return strings.Join(policies, "\n") + "\n"
	}
	return trimmed + "\n" + strings.Join(policies, "\n") + "\n"
}

// missingPolicies compares on normalised whitespace, so formatting differences
// do not produce duplicate rules.
func missingPolicies(policyCSV string, required []string) []string {
	existing := map[string]bool{}
	for _, line := range strings.Split(policyCSV, "\n") {
		if normalised := normalisePolicy(line); normalised != "" {
			existing[normalised] = true
		}
	}

	var missing []string
	for _, r := range required {
		if !existing[normalisePolicy(r)] {
			missing = append(missing, r)
		}
	}
	return missing
}

func normalisePolicy(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return ""
	}
	fields := strings.Split(line, ",")
	for i, f := range fields {
		fields[i] = strings.TrimSpace(f)
	}
	return strings.Join(fields, ",")
}

func containsCapability(capabilities, wanted string) bool {
	for _, c := range strings.Split(capabilities, ",") {
		if strings.EqualFold(strings.TrimSpace(c), wanted) {
			return true
		}
	}
	return false
}

func addCapability(capabilities, wanted string) string {
	if strings.TrimSpace(capabilities) == "" {
		return wanted
	}
	return strings.TrimSpace(capabilities) + ", " + wanted
}

func removeCapability(capabilities, unwanted string) string {
	var kept []string
	for _, c := range strings.Split(capabilities, ",") {
		if trimmed := strings.TrimSpace(c); trimmed != "" && !strings.EqualFold(trimmed, unwanted) {
			kept = append(kept, trimmed)
		}
	}
	return strings.Join(kept, ", ")
}
