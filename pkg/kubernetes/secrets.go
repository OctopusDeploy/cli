package kubernetes

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Cluster) GetSecret(ctx context.Context, namespace, name string) (*corev1.Secret, bool, error) {
	s, err := c.Clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	switch {
	case err == nil:
		return s, true, nil
	case apierrors.IsNotFound(err):
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("could not read Secret %s/%s: %w", namespace, name, err)
	}
}

// UpsertSecret replaces a Secret's contents wholesale, dropping any key it was
// not given. Only use it for Secrets Octopus owns; for anything else use
// MergeSecretKeys.
func (c *Cluster) UpsertSecret(ctx context.Context, namespace, name string, data map[string]string) error {
	secrets := c.Clientset.CoreV1().Secrets(namespace)

	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"app.kubernetes.io/managed-by": "octopus-cli"},
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: data,
	}

	existing, found, err := c.GetSecret(ctx, namespace, name)
	if err != nil {
		return err
	}

	if !found {
		if _, err := secrets.Create(ctx, desired, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("could not create Secret %s/%s: %w", namespace, name, err)
		}
		return nil
	}

	desired.ResourceVersion = existing.ResourceVersion
	if _, err := secrets.Update(ctx, desired, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("could not update Secret %s/%s: %w", namespace, name, err)
	}
	return nil
}

// MergeSecretKeys is the only safe way to edit a Secret Octopus does not own.
// argocd-secret holds Argo CD's TLS and signing keys alongside anything Octopus
// puts there, and replacing it wholesale would destroy the installation.
func (c *Cluster) MergeSecretKeys(ctx context.Context, namespace, name string, set map[string]string, remove []string) error {
	secrets := c.Clientset.CoreV1().Secrets(namespace)

	existing, found, err := c.GetSecret(ctx, namespace, name)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("Secret %s/%s does not exist", namespace, name)
	}

	updated := existing.DeepCopy()
	if updated.Data == nil {
		updated.Data = map[string][]byte{}
	}
	for key, value := range set {
		updated.Data[key] = []byte(value)
	}
	for _, key := range remove {
		delete(updated.Data, key)
	}

	// The resourceVersion that was read makes a concurrent change conflict
	// rather than be silently overwritten.
	if _, err := secrets.Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
		if apierrors.IsConflict(err) {
			return fmt.Errorf("%s/%s changed while it was being updated; try again", namespace, name)
		}
		return fmt.Errorf("could not update Secret %s/%s: %w", namespace, name, err)
	}
	return nil
}

func (c *Cluster) SecretKey(ctx context.Context, namespace, name, key string) (string, bool, error) {
	secret, found, err := c.GetSecret(ctx, namespace, name)
	if err != nil || !found {
		return "", false, err
	}
	value, ok := secret.Data[key]
	if !ok {
		return "", false, nil
	}
	return string(value), true, nil
}
