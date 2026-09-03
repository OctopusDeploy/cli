package kubernetes

import (
	"context"
	"fmt"
	"sort"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StorageClass is what an installer needs to know to choose where a component's
// volume comes from.
type StorageClass struct {
	Name        string
	Provisioner string
	IsDefault   bool
}

// readWriteManyProvisioners serve a shared filesystem, so many pods on many
// nodes can mount the same volume. Everything else provisions a block device
// that only one node can mount at a time.
//
// The Kubernetes API does not report which access modes a storage class
// supports, and this is the only signal it does give. An unrecognised
// provisioner is treated as one node at a time, because that costs nothing but
// node affinity, where guessing the other way leaves a volume that never binds.
var readWriteManyProvisioners = map[string]bool{
	"efs.csi.aws.com":                             true, // AWS EFS
	"filestore.csi.storage.gke.io":                true, // Google Filestore
	"file.csi.azure.com":                          true, // Azure Files
	"kubernetes.io/azure-file":                    true,
	"nfs.csi.k8s.io":                              true,
	"smb.csi.k8s.io":                              true,
	"cephfs.csi.ceph.com":                         true,
	"rook-ceph.cephfs.csi.ceph.com":               true,
	"openebs.io/nfsrwx":                           true,
	"nfs.openebs.io":                              true,
	"k8s-sigs.io/nfs-subdir-external-provisioner": true,
}

// SupportsReadWriteMany reports whether a volume from this class can be mounted
// by pods on more than one node.
func (s StorageClass) SupportsReadWriteMany() bool {
	return readWriteManyProvisioners[s.Provisioner]
}

func (s StorageClass) Display() string {
	if s.IsDefault {
		return fmt.Sprintf("%s (cluster default, %s)", s.Name, s.Provisioner)
	}
	return fmt.Sprintf("%s (%s)", s.Name, s.Provisioner)
}

// StorageClasses is advisory, so a cluster that will not let the caller list
// them reports none rather than failing.
func (c *Cluster) StorageClasses(ctx context.Context) ([]StorageClass, error) {
	list, err := c.Clientset.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsForbidden(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("could not list the cluster's storage classes: %w", err)
	}

	classes := make([]StorageClass, 0, len(list.Items))
	for _, item := range list.Items {
		classes = append(classes, StorageClass{
			Name:        item.Name,
			Provisioner: item.Provisioner,
			IsDefault:   item.Annotations["storageclass.kubernetes.io/is-default-class"] == "true",
		})
	}

	sort.Slice(classes, func(i, j int) bool {
		if classes[i].IsDefault != classes[j].IsDefault {
			return classes[i].IsDefault
		}
		return classes[i].Name < classes[j].Name
	})
	return classes, nil
}
