package api

type VirtualCluster struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	AgentPoolID   string `json:"agent_pool_id"`
	AgentPoolName string `json:"agent_pool_name"`
	CreatedAt     string `json:"created_at"`
}
