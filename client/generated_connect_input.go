package client

const (
	CONNECT_INPUT_TYPE = "connectInput"
)

type ConnectInput struct {
	Resource `yaml:"-"`

	NodeID string `json:"nodeID,omitempty" yaml:"node_id,omitempty"`

	Path string `json:"path,omitempty" yaml:"path,omitempty"`
}

type ConnectInputCollection struct {
	Collection
	Data   []ConnectInput `json:"data,omitempty"`
	client *ConnectInputClient
}

type ConnectInputClient struct {
	rancherClient *RancherClient
}

type ConnectInputOperations interface {
	List(opts *ListOpts) (*ConnectInputCollection, error)
	Create(opts *ConnectInput) (*ConnectInput, error)
	Update(existing *ConnectInput, updates interface{}) (*ConnectInput, error)
	ById(id string) (*ConnectInput, error)
	Delete(container *ConnectInput) error
}

func newConnectInputClient(rancherClient *RancherClient) *ConnectInputClient {
	return &ConnectInputClient{
		rancherClient: rancherClient,
	}
}

func (c *ConnectInputClient) Create(container *ConnectInput) (*ConnectInput, error) {
	resp := &ConnectInput{}
	err := c.rancherClient.doCreate(CONNECT_INPUT_TYPE, container, resp)
	return resp, err
}

func (c *ConnectInputClient) Update(existing *ConnectInput, updates interface{}) (*ConnectInput, error) {
	resp := &ConnectInput{}
	err := c.rancherClient.doUpdate(CONNECT_INPUT_TYPE, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ConnectInputClient) List(opts *ListOpts) (*ConnectInputCollection, error) {
	resp := &ConnectInputCollection{}
	err := c.rancherClient.doList(CONNECT_INPUT_TYPE, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ConnectInputCollection) Next() (*ConnectInputCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ConnectInputCollection{}
		err := cc.client.rancherClient.doNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ConnectInputClient) ById(id string) (*ConnectInput, error) {
	resp := &ConnectInput{}
	err := c.rancherClient.doById(CONNECT_INPUT_TYPE, id, resp)
	if apiError, ok := err.(*ApiError); ok {
		if apiError.StatusCode == 404 {
			return nil, nil
		}
	}
	return resp, err
}

func (c *ConnectInputClient) Delete(container *ConnectInput) error {
	return c.rancherClient.doResourceDelete(CONNECT_INPUT_TYPE, &container.Resource)
}
