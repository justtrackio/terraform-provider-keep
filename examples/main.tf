# Examples of Keep provider resource configurations

# Example Prometheus provider
resource "keep_provider" "prometheus" {
  name = "prometheus-dev"
  type = "prometheus"
  auth_config = {
    url = "http://prometheus:9090"
    # Optional authentication
    # username = "admin"
    # password = "secure_password"
  }
  # Optionally set up a webhook for this provider
  # install_webhook = true
}

# Example Grafana provider
resource "keep_provider" "grafana" {
  name = "grafana-prod"
  type = "grafana"
  auth_config = {
    url = "http://grafana:3000"
    api_token = "your-grafana-api-token"
  }
}

# Example PagerDuty provider
resource "keep_provider" "pagerduty" {
  name = "pagerduty-main"
  type = "pagerduty"
  auth_config = {
    api_key = "your-pagerduty-api-key"
  }
}

# Example Alert workflow that uses the defined providers
resource "keep_workflow" "alert_workflow" {
  workflow_file_path = "${path.module}/workflows/alert.yml"
  
  # Add dependencies to ensure providers are created first
  depends_on = [
    keep_provider.prometheus,
    keep_provider.pagerduty
  ]
}

# Example Basic workflow 
resource "keep_workflow" "basic_workflow" {
  workflow_file_path = "${path.module}/workflows/basic.yml"
}

# Example Alert mapping
resource "keep_mapping" "alert_mapping" {
  name = "alerts-mapping"
  description = "Mapping for critical alerts"
  mapping_file_path = "${path.module}/mappings/alerts.csv"
  
  # Define matchers that correspond to columns in the CSV
  matchers = [
    "alert_name=~'.*error.*'",
    "severity='critical'"
  ]
  
  priority = 1
}

# Example Log mapping
resource "keep_mapping" "log_mapping" {
  name = "logs-mapping"
  description = "Mapping for application logs"
  mapping_file_path = "${path.module}/mappings/logs.csv"
  
  matchers = [
    "source='application'",
    "level=~'error|warn'"
  ]
  
  priority = 2
}

# Extraction resource for custom log parsing
resource "keep_extraction" "log_extraction" {
  name = "error-logs"
  description = "Extract fields from error logs"
  
  # Define regex patterns to extract fields from logs
  patterns = [
    {
      pattern = "error\\[(\\w+)\\]:\\s+(.*)"
      fields = {
        "error_code" = "$1"
        "message" = "$2"
      }
    },
    {
      pattern = "WARNING:\\s+(.*)"
      fields = {
        "message" = "$1"
        "level" = "warning"
      }
    }
  ]
}

# Outputs for referencing in other configurations
output "prometheus_provider_id" {
  value = keep_provider.prometheus.id
  description = "ID of the Prometheus provider"
}

output "alert_workflow_id" {
  value = keep_workflow.alert_workflow.id
  description = "ID of the alert workflow"
}

output "alert_mapping_id" {
  value = keep_mapping.alert_mapping.id
  description = "ID of the alert mapping"
}