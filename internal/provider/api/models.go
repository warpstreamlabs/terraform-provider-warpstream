package api

type VirtualCluster struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	AgentPoolID   string `json:"agent_pool_id"`
	AgentPoolName string `json:"agent_pool_name"`
	CreatedAt     string `json:"created_at"`
}

type VirtualClusterCredentials struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	UserName      string `json:"username"`
	Password      string `json:"password"`
	CreatedAt     string `json:"created_at"`
	AgentPoolID   string `json:"agent_pool_id"`
	AgentPoolName string `json:"agent_pool_name"`
}
