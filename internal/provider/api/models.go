package api

type ClusterParameters struct {
	Type   string
	Region string
	Cloud  string
}

type AccessGrant struct {
	PrincipalKind string `json:"principal_kind"`
	ResourceKind  string `json:"resource_kind"`
	ResourceID    string `json:"resource_id"`
}

type APIKey struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Key          string        `json:"key"`
	AccessGrants []AccessGrant `json:"access_grants"`
	CreatedAt    string        `json:"created_at"`
}

type VirtualCluster struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	AgentKeys     *[]APIKey `json:"agent_keys"`
	AgentPoolID   string    `json:"agent_pool_id"`
	AgentPoolName string    `json:"agent_pool_name"`
	CreatedAt     string    `json:"created_at"`
	Region        string    `json:"region"`
	CloudProvider string    `json:"cloud_provider"`
	BootstrapURL  *string   `json:"bootstrap_url"`
}

type VirtualClusterCredentials struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	UserName         string `json:"username"`
	Password         string `json:"password"`
	CreatedAt        string `json:"created_at"`
	AgentPoolID      string `json:"agent_pool_id"`
	AgentPoolName    string `json:"agent_pool_name"`
	ClusterSuperuser bool   `json:"is_cluster_superuser"`
}

type VirtualClusterConfiguration struct {
	AclsEnabled            bool  `json:"are_acls_enabled"`
	AutoCreateTopic        bool  `json:"is_auto_create_topic_enabled"`
	DefaultNumPartitions   int64 `json:"default_num_partitions"`
	DefaultRetentionMillis int64 `json:"default_retention_millis"`
}
