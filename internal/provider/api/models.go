package api

type VirtualCluster struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	AgentPoolID   string `json:"agent_pool_id"`
	AgentPoolName string `json:"agent_pool_name"`
	CreatedAt     string `json:"created_at"`
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
