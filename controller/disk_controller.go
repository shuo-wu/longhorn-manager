package controller

import (
	"fmt"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/controller"

	"github.com/longhorn/longhorn-manager/datastore"
	"github.com/longhorn/longhorn-manager/scheduler"
	"github.com/longhorn/longhorn-manager/types"
	"github.com/longhorn/longhorn-manager/util"

	longhorn "github.com/longhorn/longhorn-manager/k8s/pkg/apis/longhorn/v1beta1"
	lhinformers "github.com/longhorn/longhorn-manager/k8s/pkg/client/informers/externalversions/longhorn/v1beta1"
)

type DiskController struct {
	*baseController

	namespace    string
	controllerID string

	kubeClient    clientset.Interface
	eventRecorder record.EventRecorder

	ds *datastore.DataStore

	dStoreSynced cache.InformerSynced
	nStoreSynced cache.InformerSynced
	rStoreSynced cache.InformerSynced
	sStoreSynced cache.InformerSynced

	getDiskInfoHandler util.GetDiskInfoHandler
	getDiskConfig      util.DiskConfigHandler

	scheduler *scheduler.ReplicaScheduler
}

func NewDiskController(
	logger logrus.FieldLogger,
	ds *datastore.DataStore,
	scheme *runtime.Scheme,
	diskInformer lhinformers.DiskInformer,
	nodeInformer lhinformers.NodeInformer,
	replicaInformer lhinformers.ReplicaInformer,
	settingInformer lhinformers.SettingInformer,
	kubeClient clientset.Interface,
	namespace, controllerID string) *DiskController {

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logrus.Infof)
	// TODO: remove the wrapper when every clients have moved to use the clientset.
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: typedcorev1.New(kubeClient.CoreV1().RESTClient()).Events("")})

	dc := &DiskController{
		baseController: newBaseController("longhorn-disk", logger),

		ds: ds,

		namespace:    namespace,
		controllerID: controllerID,

		kubeClient:    kubeClient,
		eventRecorder: eventBroadcaster.NewRecorder(scheme, v1.EventSource{Component: "longhorn-disk-controller"}),

		dStoreSynced: diskInformer.Informer().HasSynced,
		nStoreSynced: nodeInformer.Informer().HasSynced,
		rStoreSynced: replicaInformer.Informer().HasSynced,
		sStoreSynced: settingInformer.Informer().HasSynced,

		getDiskInfoHandler: util.GetDiskInfo,
		getDiskConfig:      util.GetDiskConfig,
	}

	dc.scheduler = scheduler.NewReplicaScheduler(ds)

	diskInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			d := obj.(*longhorn.Disk)
			dc.enqueueDisk(d)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			cur := newObj.(*longhorn.Disk)
			dc.enqueueDisk(cur)
		},
		DeleteFunc: func(obj interface{}) {
			d := obj.(*longhorn.Disk)
			dc.enqueueDisk(d)
		},
	})

	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			n := obj.(*longhorn.Node)
			dc.enqueueLonghornNodeChange(n)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			cur := newObj.(*longhorn.Node)
			dc.enqueueLonghornNodeChange(cur)
		},
		DeleteFunc: func(obj interface{}) {
			n := obj.(*longhorn.Node)
			dc.enqueueLonghornNodeChange(n)
		},
	})

	replicaInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			r := obj.(*longhorn.Replica)
			dc.enqueueReplicaChange(r)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			cur := newObj.(*longhorn.Replica)
			dc.enqueueReplicaChange(cur)
		},
		DeleteFunc: func(obj interface{}) {
			r := obj.(*longhorn.Replica)
			dc.enqueueReplicaChange(r)
		},
	})

	settingInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *longhorn.Setting:
					return dc.filterSettings(t)
				default:
					utilruntime.HandleError(fmt.Errorf("unable to handle object in %T: %T", dc, obj))
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					s := obj.(*longhorn.Setting)
					dc.enqueueSettingChange(s)
				},
				UpdateFunc: func(oldObj, newObj interface{}) {
					cur := newObj.(*longhorn.Setting)
					dc.enqueueSettingChange(cur)
				},
			},
		},
	)

	return dc
}

func (dc *DiskController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer dc.queue.ShutDown()

	logrus.Infof("Start Longhorn disk controller")
	defer logrus.Infof("Shutting down Longhorn disk controller")

	if !controller.WaitForCacheSync("longhorn disk", stopCh,
		dc.dStoreSynced, dc.nStoreSynced, dc.rStoreSynced, dc.sStoreSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(dc.worker, time.Second, stopCh)
	}

	<-stopCh
}

func (dc *DiskController) worker() {
	for dc.processNextWorkItem() {
	}
}

func (dc *DiskController) processNextWorkItem() bool {
	key, quit := dc.queue.Get()

	if quit {
		return false
	}
	defer dc.queue.Done(key)

	err := dc.syncDisk(key.(string))
	dc.handleErr(err, key)

	return true
}

func (dc *DiskController) handleErr(err error, key interface{}) {
	if err == nil {
		dc.queue.Forget(key)
		return
	}

	log := dc.logger.WithField("disk", key)
	if dc.queue.NumRequeues(key) < maxRetries {
		log.WithError(err).Warn("Error syncing Longhorn disk")
		dc.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	log.WithError(err).Warn("Dropping Longhorn disk out of the queue")
	dc.queue.Forget(key)
}

func (dc *DiskController) syncDisk(key string) (err error) {
	defer func() {
		err = errors.Wrapf(err, "fail to sync disk for %v", key)
	}()
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	log := dc.logger.WithField("disk", name)
	disk, err := dc.ds.GetDisk(name)
	if err != nil {
		if datastore.ErrorIsNotFound(err) {
			log.WithError(err).Error("Longhorn disk has been deleted")
			return nil
		}
		return err
	}

	if disk.Status.OwnerID != dc.controllerID {
		if !dc.isResponsibleFor(disk) {
			// Not ours
			return nil
		}
		disk.Status.OwnerID = dc.controllerID
		disk, err = dc.ds.UpdateDiskStatus(disk)
		if err != nil {
			// we don't mind others coming first
			if apierrors.IsConflict(errors.Cause(err)) {
				return nil
			}
			return err
		}
		log.Infof("Disk got new owner %v", dc.controllerID)
	}

	if disk.DeletionTimestamp != nil {
		dc.eventRecorder.Eventf(disk, v1.EventTypeNormal, EventReasonDelete, "Deleting disk %v", disk.Name)
		return dc.ds.RemoveFinalizerForDisk(disk)
	}

	replicaList, err := dc.ds.ListReplicasByDisk(disk.Name)
	if err != nil {
		return err
	}
	existingReplicaMap := make(map[string]*longhorn.Replica, len(replicaList))
	for _, r := range replicaList {
		existingReplicaMap[r.Name] = r.DeepCopy()
	}
	existingDisk := disk.DeepCopy()
	defer func() {
		if err != nil {
			return
		}
		for _, r := range replicaList {
			existingReplica, exists := existingReplicaMap[r.Name]
			if !exists {
				logrus.Errorf("BUG: found unknown replica %v during the update", r.Name)
				return
			}
			if !reflect.DeepEqual(existingReplica.Spec, r.Spec) {
				if _, replicaErr := dc.ds.UpdateReplica(r); replicaErr != nil {
					err = errors.Wrapf(replicaErr, "failed to update replica %v node id after disk state change", r.Name)
					return
				}
			}
		}

		// we're going to update engine assume things changes
		if !reflect.DeepEqual(existingDisk.Status, disk.Status) {
			_, err = dc.ds.UpdateDiskStatus(disk)
		}
		// requeue if it's conflict
		if apierrors.IsConflict(errors.Cause(err)) {
			log.WithError(err).Debug("Requeue disk due to conflict")
			dc.enqueueDisk(disk)
			err = nil
		}
	}()

	// initialize the disk if necessary
	if disk.Status.Conditions == nil {
		disk.Status.Conditions = map[string]types.Condition{}
	}
	if disk.Status.ScheduledReplica == nil {
		disk.Status.ScheduledReplica = map[string]int64{}
	}

	var diskFailureReason, diskFailureMessage string
	defer func() {
		if diskFailureReason != "" {
			disk.Status.State = types.DiskStateDisconnected
			disk.Status.Conditions = types.SetConditionAndRecord(disk.Status.Conditions,
				types.DiskConditionTypeReady, types.ConditionStatusFalse, diskFailureReason, diskFailureMessage,
				dc.eventRecorder, disk, v1.EventTypeWarning)
			disk.Status.Conditions = types.SetConditionAndRecord(disk.Status.Conditions,
				types.DiskConditionTypeSchedulable, types.ConditionStatusFalse,
				string(types.DiskConditionReasonDiskNotReady),
				"Disk is not ready",
				dc.eventRecorder, disk, v1.EventTypeWarning)
		}
		if disk.Status.State == types.DiskStateConnected {
			disk.Status.Conditions = types.SetConditionAndRecord(disk.Status.Conditions,
				types.DiskConditionTypeReady, types.ConditionStatusTrue,
				"", "",
				dc.eventRecorder, disk, v1.EventTypeNormal)
		} else {
			disk.Status.StorageMaximum = 0
			disk.Status.StorageAvailable = 0
			disk.Status.StorageScheduled = 0
			disk.Status.ScheduledReplica = map[string]int64{}
			disk.Status.FSID = ""
			// All running replicas should be marked as failure
			for _, r := range replicaList {
				if r.Status.CurrentState == types.InstanceStateRunning && r.Spec.FailedAt == "" {
					r.Spec.FailedAt = util.Now()
				}
			}
		}
	}()

	if disk.Status.NodeID == "" || disk.Status.State == types.DiskStateDisconnected {
		nodes, err := dc.ds.ListNodes()
		if err != nil {
			return err
		}
		for _, node := range nodes {
			cond := types.GetCondition(node.Status.Conditions, types.NodeConditionTypeReady)
			if cond.Status == types.ConditionStatusFalse &&
				(cond.Reason == string(types.NodeConditionReasonKubernetesNodeGone) ||
					cond.Reason == string(types.NodeConditionReasonKubernetesNodeNotReady)) {
				continue
			}
			for _, diskID := range node.Status.DiskPathIDMap {
				if diskID == disk.Name {
					disk.Status.NodeID = node.Name
					break
				}
			}
			if disk.Status.NodeID != "" {
				break
			}
		}
		if disk.Status.NodeID == "" {
			diskFailureReason = string(types.DiskConditionReasonNodeUnknown)
			diskFailureMessage = "Can not find node ID for the disk"
			return nil
		}
	}

	isNodeDownOrDeleted, err := dc.ds.IsNodeDownOrDeleted(disk.Status.NodeID)
	if err != nil {
		return err
	}
	if isNodeDownOrDeleted {
		diskFailureReason = string(types.DiskConditionReasonNodeUnknown)
		diskFailureMessage = fmt.Sprintf("The connected node %v is down or deleted", disk.Status.NodeID)
		return nil
	}

	// Prevent the corner case:
	// The corresponding node is up but the disk ownership hasn't been transferred.
	// In this case, Longhorn cannot verify UUID and FSID for the disk.
	if disk.Status.NodeID != dc.controllerID {
		diskFailureReason = string(types.DiskConditionReasonNodeUnknown)
		diskFailureMessage = "The disk hasn't been taken by the preferred node"
		return nil
	}

	info, err := dc.getDiskInfoHandler(disk.Spec.Path)
	if err != nil {
		diskFailureReason = string(types.DiskConditionReasonNoDiskInfo)
		diskFailureMessage = fmt.Sprintf("Get disk information, error: %v", err)
		return nil
	}
	disk.Status.State = types.DiskStateConnected

	// Check disks in the same filesystem
	// Filesystem ID won't be used to identify a disk. See the doc for more details:
	// https://github.com/longhorn/longhorn/blob/v1.0.2/enhancements/20200331-replace-filesystem-id-key-in-disk-map.md
	isDuplicate, err := dc.isFSIDDuplicatedWithExistingReadyDisk(disk, info.Fsid)
	if err != nil {
		return err
	}
	// Found multiple disks in the same Fsid
	if isDuplicate {
		diskFailureReason = string(types.DiskConditionReasonDiskFilesystemChanged)
		diskFailureMessage = fmt.Sprintf("Disk has same file system ID %v as other disks", info.Fsid)
		return nil
	}
	disk.Status.FSID = info.Fsid

	if diskConfig, err := dc.getDiskConfig(disk.Spec.Path); err != nil {
		if !types.ErrorIsNotFound(err) {
			diskFailureReason = string(types.DiskConditionReasonNoDiskInfo)
			diskFailureMessage = fmt.Sprintf("Failed to get disk config: %v", err)
			return nil
		}
	} else {
		if disk.Name != diskConfig.DiskUUID {
			diskFailureReason = string(types.DiskConditionReasonDiskFilesystemChanged)
			diskFailureMessage = fmt.Sprintf("Disk Name/UUID doesn't match the recorded UUID %v in the meta file", diskConfig.DiskUUID)
			return nil
		}
	}

	disk.Status.StorageMaximum = info.StorageMaximum
	disk.Status.StorageAvailable = info.StorageAvailable

	// update Schedulable condition
	minimalAvailablePercentage, err := dc.ds.GetSettingAsInt(types.SettingNameStorageMinimalAvailablePercentage)
	if err != nil {
		return err
	}

	// calculate storage scheduled
	scheduledReplica := map[string]int64{}
	storageScheduled := int64(0)
	for _, r := range replicaList {
		storageScheduled += r.Spec.VolumeSize
		scheduledReplica[r.Name] = r.Spec.VolumeSize
	}
	disk.Status.StorageScheduled = storageScheduled
	disk.Status.ScheduledReplica = scheduledReplica

	// update schedule condition
	schedulingInfo, err := dc.scheduler.GetDiskSchedulingInfo(disk)
	if err != nil {
		return err
	}
	if !dc.scheduler.IsSchedulableToDisk(0, 0, schedulingInfo) {
		disk.Status.Conditions = types.SetConditionAndRecord(disk.Status.Conditions,
			types.DiskConditionTypeSchedulable, types.ConditionStatusFalse,
			string(types.DiskConditionReasonDiskPressure),
			fmt.Sprintf("The disk has %v available, but requires reserved %v, minimal %v%s to schedule more replicas",
				disk.Status.StorageAvailable, disk.Spec.StorageReserved, minimalAvailablePercentage, "%"),
			dc.eventRecorder, disk, v1.EventTypeWarning)

	} else {
		disk.Status.Conditions = types.SetConditionAndRecord(disk.Status.Conditions,
			types.DiskConditionTypeSchedulable, types.ConditionStatusTrue,
			"", "",
			dc.eventRecorder, disk, v1.EventTypeNormal)
	}

	return nil
}

// Check all disks in the same filesystem ID are in ready status
func (dc *DiskController) isFSIDDuplicatedWithExistingReadyDisk(disk *longhorn.Disk, fsid string) (bool, error) {
	diskList, err := dc.ds.ListDisksByNode(disk.Status.NodeID)
	if err != nil {
		return false, err
	}

	for _, d := range diskList {
		if d.Name == disk.Name {
			continue
		}
		if d.Status.State != types.DiskStateConnected {
			continue
		}
		if d.Status.FSID == fsid {
			return true, nil
		}
	}

	return false, nil
}

func (dc *DiskController) filterSettings(s *longhorn.Setting) bool {
	// filter that only StorageMinimalAvailablePercentage will impact disk status
	if types.SettingName(s.Name) == types.SettingNameStorageMinimalAvailablePercentage {
		return true
	}
	return false
}

func (dc *DiskController) enqueueDisk(disk *longhorn.Disk) {
	key, err := controller.KeyFunc(disk)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %#v: %v", disk, err))
		return
	}

	dc.queue.AddRateLimited(key)
}

func (dc *DiskController) enqueueLonghornNodeChange(node *longhorn.Node) {
	dList, err := dc.ds.ListDisksByNode(node.Name)
	if err != nil {
		logrus.Warnf("Failed to list disks on node %v", node.Name)
	}
	for _, d := range dList {
		dc.enqueueDisk(d)
	}
	return
}

func (dc *DiskController) enqueueReplicaChange(replica *longhorn.Replica) {
	if replica.Spec.DiskID == "" {
		return
	}
	disk, err := dc.ds.GetDisk(replica.Spec.DiskID)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get disk %v for replica %v: %v ",
			replica.Spec.DiskID, replica.Name, err))
		return
	}
	dc.enqueueDisk(disk)
}

func (dc *DiskController) enqueueSettingChange(setting *longhorn.Setting) {
	diskList, err := dc.ds.ListDisks()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get all nodes: %v ", err))
		return
	}

	for _, disk := range diskList {
		dc.enqueueDisk(disk)
	}
}

func (dc *DiskController) isResponsibleFor(disk *longhorn.Disk) bool {
	return isControllerResponsibleFor(dc.controllerID, dc.ds, disk.Name, disk.Status.NodeID, disk.Status.OwnerID)
}
