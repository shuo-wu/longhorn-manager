package client

const (
	DISK_TYPE = "disk"
)

type Disk struct {
	Resource `yaml:"-"`

	AllowScheduling bool `json:"allowScheduling,omitempty" yaml:"allow_scheduling,omitempty"`

	Conditions map[string]interface{} `json:"conditions,omitempty" yaml:"conditions,omitempty"`

	DiskUUID string `json:"diskUUID,omitempty" yaml:"disk_uuid,omitempty"`

	EvictionRequested bool `json:"evictionRequested,omitempty" yaml:"eviction_requested,omitempty"`

	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	NodeID string `json:"nodeID,omitempty" yaml:"node_id,omitempty"`

	Path string `json:"path,omitempty" yaml:"path,omitempty"`

	ScheduledReplica map[string]string `json:"scheduledReplica,omitempty" yaml:"scheduled_replica,omitempty"`

	State string `json:"state,omitempty" yaml:"state,omitempty"`

	StorageAvailable int64 `json:"storageAvailable,omitempty" yaml:"storage_available,omitempty"`

	StorageMaximum int64 `json:"storageMaximum,omitempty" yaml:"storage_maximum,omitempty"`

	StorageReserved int64 `json:"storageReserved,omitempty" yaml:"storage_reserved,omitempty"`

	StorageScheduled int64 `json:"storageScheduled,omitempty" yaml:"storage_scheduled,omitempty"`

	Tags []string `json:"tags,omitempty" yaml:"tags,omitempty"`

	Timestamp string `json:"timestamp,omitempty" yaml:"timestamp,omitempty"`
}

type DiskCollection struct {
	Collection
	Data   []Disk `json:"data,omitempty"`
	client *DiskClient
}

type DiskClient struct {
	rancherClient *RancherClient
}

type DiskOperations interface {
	List(opts *ListOpts) (*DiskCollection, error)
	Create(opts *Disk) (*Disk, error)
	Update(existing *Disk, updates interface{}) (*Disk, error)
	ById(id string) (*Disk, error)
	Delete(container *Disk) error
}

func newDiskClient(rancherClient *RancherClient) *DiskClient {
	return &DiskClient{
		rancherClient: rancherClient,
	}
}

func (c *DiskClient) Create(container *Disk) (*Disk, error) {
	resp := &Disk{}
	err := c.rancherClient.doCreate(DISK_TYPE, container, resp)
	return resp, err
}

func (c *DiskClient) Update(existing *Disk, updates interface{}) (*Disk, error) {
	resp := &Disk{}
	err := c.rancherClient.doUpdate(DISK_TYPE, &existing.Resource, updates, resp)
	return resp, err
}

func (c *DiskClient) List(opts *ListOpts) (*DiskCollection, error) {
	resp := &DiskCollection{}
	err := c.rancherClient.doList(DISK_TYPE, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *DiskCollection) Next() (*DiskCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &DiskCollection{}
		err := cc.client.rancherClient.doNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *DiskClient) ById(id string) (*Disk, error) {
	resp := &Disk{}
	err := c.rancherClient.doById(DISK_TYPE, id, resp)
	if apiError, ok := err.(*ApiError); ok {
		if apiError.StatusCode == 404 {
			return nil, nil
		}
	}
	return resp, err
}

func (c *DiskClient) Delete(container *Disk) error {
	return c.rancherClient.doResourceDelete(DISK_TYPE, &container.Resource)
}
