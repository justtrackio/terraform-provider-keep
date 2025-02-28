provider "keep" {
  # Keep API backend URL
  # Can be overridden with KEEP_BACKEND_URL environment variable
  backend_url = "http://localhost:8080"
  
  # API key for authentication with the Keep backend
  # Can be overridden with KEEP_API_KEY environment variable
  api_key = "keepappkey"
  
  # HTTP client timeout duration
  # Can be overridden with KEEP_TIMEOUT environment variable
  timeout = "30s"
  
  # Uncomment and configure these when using Azure Kubernetes Service (AKS)
  # aks_subscription_id     = "your-subscription-id"
  # aks_client_id           = "your-client-id"
  # aks_client_secret       = "your-client-secret"
  # aks_tenant_id           = "your-tenant-id"
  # aks_resource_group_name = "your-resource-group"
  # aks_resource_name       = "your-aks-cluster-name"
}
