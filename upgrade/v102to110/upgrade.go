package v102to110

import (
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"

	longhorn "github.com/longhorn/longhorn-manager/k8s/pkg/apis/longhorn/v1beta1"
	lhclientset "github.com/longhorn/longhorn-manager/k8s/pkg/client/clientset/versioned"
	"github.com/longhorn/longhorn-manager/types"
)

// This upgrade is needed because:
//   1. we add one more field `controller: true` to the ownerReferences of
//   instance manager pods so that `kubectl drain` can work without --force flag.
//   2. we use separate the disk resources from node object (in v1.0.2) and use
//   CRs to track them (in v1.1.0).
// Therefore, we need to update thost existing CRs.
// Link to the original issues:
//   https://github.com/longhorn/longhorn/issues/1286
//   https://github.com/longhorn/longhorn/issues/1269

const (
	upgradeLogPrefix = "upgrade from v1.0.2 to v1.1.0: "
)

func UpgradeInstanceManagerPods(namespace string, lhClient *lhclientset.Clientset, kubeClient *clientset.Clientset) (err error) {
	defer func() {
		err = errors.Wrapf(err, upgradeLogPrefix+"upgrade instance manager pods failed")
	}()

	imPodsList, err := kubeClient.CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", types.GetLonghornLabelComponentKey(), types.LonghornLabelInstanceManager),
	})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, upgradeLogPrefix+"failed to list all existing instance manager pods during the instance managers pods upgrade")
	}

	for _, pod := range imPodsList.Items {
		if err := upgradeInstanceMangerPodOwnerRef(&pod, kubeClient, namespace); err != nil {
			return err
		}
	}
	return nil
}

func UpgradeCRDs(namespace string, lhClient *lhclientset.Clientset) error {
	if err := createDisksAndUpdateNodes(namespace, lhClient); err != nil {
		return err
	}
	if err := doReplicaUpgrade(namespace, lhClient); err != nil {
		return err
	}

	return nil
}

func upgradeInstanceMangerPodOwnerRef(pod *v1.Pod, kubeClient *clientset.Clientset, namespace string) (err error) {
	metadata, err := meta.Accessor(pod)
	if err != nil {
		return err
	}

	podOwnerRefs := metadata.GetOwnerReferences()
	isController := true
	needToUpdate := false
	for ind, ownerRef := range podOwnerRefs {
		if ownerRef.Kind == types.LonghornKindInstanceManager &&
			(ownerRef.Controller == nil || *ownerRef.Controller != true) {
			ownerRef.Controller = &isController
			needToUpdate = true
		}
		podOwnerRefs[ind] = ownerRef
	}

	if !needToUpdate {
		return nil
	}

	metadata.SetOwnerReferences(podOwnerRefs)

	if _, err = kubeClient.CoreV1().Pods(namespace).Update(pod); err != nil {
		return errors.Wrapf(err, upgradeLogPrefix+"failed to update the owner reference for instance manager pod %v during the instance managers pods upgrade", pod.GetName())
	}

	return nil
}

func createDisksAndUpdateNodes(namespace string, lhClient *lhclientset.Clientset) (err error) {
	defer func() {
		err = errors.Wrapf(err, upgradeLogPrefix+"upgrade instance manager failed")
	}()

	nodeList, err := lhClient.LonghornV1beta1().Nodes(namespace).List(metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, upgradeLogPrefix+"failed to list all existing nodes during the upgrade")
	}

	for _, node := range nodeList.Items {
		if node.Spec.DiskPathMap == nil {
			node.Spec.DiskPathMap = map[string]struct{}{}
		}
		if node.Status.DiskPathIDMap == nil {
			node.Status.DiskPathIDMap = map[string]string{}
		}
		if node.Spec.Disks != nil {
			for diskID, diskSpec := range node.Spec.Disks {
				status, exists := node.Status.DiskStatus[diskID]
				if !exists {
					continue
				}
				diskName := status.DiskUUID
				if _, err := lhClient.LonghornV1beta1().Disks(namespace).Get(diskName, metav1.GetOptions{}); err == nil {
					continue
				} else {
					if !apierrors.IsNotFound(err) {
						return errors.Wrapf(err, upgradeLogPrefix+"failed to get disk %v for node %v during the upgrade", diskName, node.Name)
					}
				}
				path, err := filepath.Abs(diskSpec.Path)
				if err != nil {
					return errors.Wrapf(err, upgradeLogPrefix+"failed to format disk path %v for node %v during the upgrade", diskSpec.Path, node.Name)
				}
				disk := &longhorn.Disk{
					ObjectMeta: metav1.ObjectMeta{
						Name:      diskName,
						Namespace: node.Namespace,
						Labels: map[string]string{
							types.LonghornNodeKey: node.Name,
						},
					},
					Spec: types.DiskSpec{
						Path:              path,
						AllowScheduling:   diskSpec.AllowScheduling,
						EvictionRequested: diskSpec.EvictionRequested,
						StorageReserved:   diskSpec.StorageReserved,
						Tags:              diskSpec.Tags,
					},
				}
				if _, err := lhClient.LonghornV1beta1().Disks(namespace).Create(disk); err != nil {
					return errors.Wrapf(err, upgradeLogPrefix+"failed to create disk %v during the upgrade", diskName)
				}
				node.Spec.DiskPathMap[path] = struct{}{}
				node.Status.DiskPathIDMap[path] = diskName
			}
		}

		node.Spec.Disks = nil
		node.Status.DiskStatus = nil
		if _, err := lhClient.LonghornV1beta1().Nodes(namespace).Update(&node); err != nil {
			return errors.Wrapf(err, upgradeLogPrefix+"failed to clean up the deprecated field node.Spec.Disks for node %v during the upgrade", node.Name)
		}
	}
	return nil
}

func doReplicaUpgrade(namespace string, lhClient *lhclientset.Clientset) (err error) {
	defer func() {
		err = errors.Wrapf(err, upgradeLogPrefix+"upgrade instance manager failed")
	}()

	replicaList, err := lhClient.LonghornV1beta1().Replicas(namespace).List(metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, upgradeLogPrefix+"failed to list all existing replicas during the upgrade")
	}

	for _, replica := range replicaList.Items {
		if _, exist := replica.Labels[types.LonghornDiskKey]; exist {
			continue
		}
		if replica.Spec.NodeID == "" || replica.Spec.DiskID == "" {
			continue
		}
		node, err := lhClient.LonghornV1beta1().Nodes(namespace).Get(replica.Spec.NodeID, metav1.GetOptions{})
		if err != nil {
			return err
		}
		diskStatus, exists := node.Status.DiskStatus[replica.Spec.DiskID]
		if !exists {
			continue
		}
		replica.Spec.DiskID = diskStatus.DiskUUID
		replica.Labels[types.LonghornDiskKey] = diskStatus.DiskUUID
		if _, err := lhClient.LonghornV1beta1().Replicas(namespace).Update(&replica); err != nil {
			return err
		}
	}
	return nil
}
