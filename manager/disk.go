package manager

import (
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/longhorn/longhorn-manager/types"
	"github.com/longhorn/longhorn-manager/util"

	longhorn "github.com/longhorn/longhorn-manager/k8s/pkg/apis/longhorn/v1beta1"
)

func (m *VolumeManager) GetDisk(name string) (*longhorn.Disk, error) {
	return m.ds.GetDisk(name)
}

func (m *VolumeManager) ListDisks() (map[string]*longhorn.Disk, error) {
	diskMap, err := m.ds.ListDisks()
	if err != nil {
		return nil, err
	}
	return diskMap, nil
}

func (m *VolumeManager) ListDisksSorted() ([]*longhorn.Disk, error) {
	diskMap, err := m.ListDisks()
	if err != nil {
		return []*longhorn.Disk{}, err
	}

	disks := []*longhorn.Disk{}
	nodeDiskMap := map[string]map[string]*longhorn.Disk{}
	for _, disk := range diskMap {
		if nodeDiskMap[disk.Status.NodeID] == nil {
			nodeDiskMap[disk.Status.NodeID] = map[string]*longhorn.Disk{}
		}
		nodeDiskMap[disk.Status.NodeID][disk.Name] = disk
	}
	sortedNodeNameList, err := sortKeys(nodeDiskMap)
	if err != nil {
		return []*longhorn.Disk{}, err
	}
	for _, nodeName := range sortedNodeNameList {
		sortedDiskNameList, err := sortKeys(nodeDiskMap[nodeName])
		if err != nil {
			return []*longhorn.Disk{}, err
		}
		for _, diskName := range sortedDiskNameList {
			disks = append(disks, diskMap[diskName])
		}
	}

	return disks, nil
}

func (m *VolumeManager) CreateDisk(nodeID string, diskSpec *types.DiskSpec) (*longhorn.Disk, error) {
	if err := m.validateDiskSpec(diskSpec); err != nil {
		return nil, err
	}

	if diskSpec.Path == "" {
		return nil, fmt.Errorf("invalid paramater: empty disk path")
	}
	if path, err := filepath.Abs(diskSpec.Path); err != nil {
		return nil, errors.Wrapf(err, "failed to format disk path before creation")
	} else {
		diskSpec.Path = path
	}

	if nodeID == "" {
		return nil, fmt.Errorf("invalid paramater: empty node ID")
	}
	node, err := m.GetNode(nodeID)
	if err != nil {
		return nil, err
	}
	readyCondition := types.GetCondition(node.Status.Conditions, types.NodeConditionTypeReady)
	if readyCondition.Status != types.ConditionStatusTrue {
		return nil, fmt.Errorf("node %v is not ready, couldn't add disks for it", node.Name)
	}
	if _, exists := node.Spec.DiskPathMap[diskSpec.Path]; exists {
		return nil, fmt.Errorf("Disk path %v is already on node %v", diskSpec.Path, node.Name)
	}

	node.Spec.DiskPathMap[diskSpec.Path] = struct{}{}
	if _, err := m.ds.UpdateNode(node); err != nil {
		return nil, errors.Wrapf(err, "failed to update node during the disk creation")
	}

	disk, err := m.ds.WaitForDiskCreation(nodeID, diskSpec.Path)
	if err != nil {
		return nil, err
	}

	disk.Spec = *diskSpec
	if disk, err = m.ds.UpdateDisk(disk); err != nil {
		return nil, errors.Wrapf(err, "failed to apply all parameters during the disk creation")
	}

	defer logrus.Infof("Create a new disk %v for node %v", disk.Name, nodeID)
	return disk, nil
}

func (m *VolumeManager) validateDiskSpec(diskSpec *types.DiskSpec) error {
	if diskSpec.StorageReserved < 0 {
		return fmt.Errorf("the reserved storage %v for disk path %v is not valid, should be positive", diskSpec.StorageReserved, diskSpec.Path)
	}

	tags, err := util.ValidateTags(diskSpec.Tags)
	if err != nil {
		return err
	}
	diskSpec.Tags = tags

	return nil
}

func (m *VolumeManager) DeleteDisk(name string) error {
	// only remove node from longhorn without any volumes on it
	disk, err := m.ds.GetDisk(name)
	if err != nil {
		return err
	}
	if disk.Status.State == types.DiskStateConnected && disk.Spec.AllowScheduling {
		return fmt.Errorf("need to disable the scheduling before deleting the connected disk")
	}

	defer logrus.Debugf("Deleted disk %v", name)
	if disk.Status.NodeID == "" {
		return m.ds.DeleteDisk(name)
	}

	node, err := m.ds.GetNode(disk.Status.NodeID)
	if err != nil {
		return errors.Wrapf(err, "failed to get node %v during the disk %v deletion", disk.Status.NodeID, disk.Name)
	}
	if _, exists := node.Spec.DiskPathMap[disk.Spec.Path]; !exists {
		return fmt.Errorf("the disk %v is removed/being removed from node %v", disk.Name, node.Name)
	}

	delete(node.Spec.DiskPathMap, disk.Spec.Path)
	if _, err := m.ds.UpdateNode(node); err != nil {
		return err
	}

	return nil
}

func (m *VolumeManager) UpdateDisk(d *longhorn.Disk) (*longhorn.Disk, error) {
	if err := m.validateDiskSpec(&d.Spec); err != nil {
		return nil, err
	}
	disk, err := m.ds.UpdateDisk(d)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("Updated disk %v to %+v", disk.Name, disk.Spec)
	return disk, nil
}

func (m *VolumeManager) GetDiskTags() ([]string, error) {
	foundTags := make(map[string]struct{})
	var tags []string

	diskList, err := m.ds.ListDisks()
	if err != nil {
		return nil, errors.Wrapf(err, "fail to list disks")
	}
	for _, disk := range diskList {
		for _, tag := range disk.Spec.Tags {
			if _, ok := foundTags[tag]; !ok {
				foundTags[tag] = struct{}{}
				tags = append(tags, tag)
			}
		}
	}

	return tags, nil
}
