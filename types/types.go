package types

import (
	"fmt"
	"strconv"

	"github.com/rancher/longhorn-manager/util"
)

const (
	DefaultAPIPort = 9500
)

type ReplicaMode string

const (
	ReplicaModeRW  = ReplicaMode("RW")
	ReplicaModeWO  = ReplicaMode("WO")
	ReplicaModeERR = ReplicaMode("ERR")

	EnvNodeName     = "NODE_NAME"
	EnvPodNamespace = "POD_NAMESPACE"
	EnvPodIP        = "POD_IP"

	OptionFromBackup          = "fromBackup"
	OptionNumberOfReplica     = "numberOfReplicas"
	OptionStaleReplicaTimeout = "staleReplicaTimeout"
)

type NotFoundError struct {
	Name string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%v was not found", e.Name)
}

const (
	engineSuffix    = "-e"
	replicaSuffix   = "-r"
	recurringSuffix = "-c"

	// MaximumJobNameSize is calculated using
	// 1. NameMaximumLength is 40
	// 2. Recurring suffix is 2
	// 3. Maximum kubernetes name length is 63
	// 4. cronjob pod suffix is 11
	// 5. Dash and buffer for 2
	MaximumJobNameSize = 8
)

func GetEngineNameForVolume(vName string) string {
	return vName + engineSuffix
}

func GenerateReplicaNameForVolume(vName string) string {
	return vName + replicaSuffix + "-" + util.RandomID()
}

func GetCronJobNameForVolumeAndJob(vName, job string) string {
	return vName + "-" + job + recurringSuffix
}

func GetAPIServerAddressFromIP(ip string) string {
	return ip + ":" + strconv.Itoa(DefaultAPIPort)
}
