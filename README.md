<picture>
  <img align="right" height="54" src="assets/keep-logo.png">
</picture>

# terraform-provider-keep

[![docs](https://img.shields.io/static/v1?label=docs&message=terraform&color=informational&style=for-the-badge&logo=terraform)](https://registry.terraform.io/providers/justtrackio/keep/latest/docs)
![downloads](https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fregistry.terraform.io%2Fv2%2Fproviders%2Fjusttrackio%2Fkeep%3Finclude%3Dcategories%2Cmoved-to%2Cpotential-fork-of%2Cprovider-versions%2Ctop-modules%26include%3Dcategories%252Cmoved-to%252Cpotential-fork-of%252Cprovider-versions%252Ctop-modules%26name%3Dkeep%26namespace%3Djusttrackio&query=data.attributes.downloads&style=for-the-badge&logo=terraform&label=downloads&color=brightgreen)
![latest version](https://img.shields.io/github/v/release/justtrackio/terraform-provider-keep?style=for-the-badge&label=latest%20version&color=orange)
![license](https://img.shields.io/github/license/justtrackio/terraform-provider-keep?style=for-the-badge)

This is a terraform provider for managing your [keep](https://github.com/keephq/keep) instance.

> **Note:** This provider is not official terraform provider for keep.

### Installation

Add the following to your terraform configuration

```tf
terraform {
  required_providers {
    keep = {
      source  = "justtrackio/keep"
      version = "~> 0.0.1"
    }
  }
}
```

### Example

```hcl
provider "keep" {
  backend_url = "http://localhost:8080" # or use environment variable KEEP_BACKEND_URL
  api_key = "your apikey" # or use environment variable KEEP_API_KEY
}

resource "keep_workflow" "example_workflow" {
  workflow_file_path = "path/to/workflow.yml"
}

resource "keep_mapping" "example_mapping" {
  name = "example_mapping"
  mapping_file_path = "path/to/mapping.yml"
  matchers = [
    "your unique matcher",
  ]
  #priority = 1 (optional)
}

resource "keep_provider" "example_provider" {
  name = "example_provider"
  type = "supported_provider_type"
  auth_config = {
    //...
    // Add your provider specific configuration
    //...
  }
  #install_webhook = true (optional)
}

data "keep_workflow" "example_workflow_data" {
  id = keep_workflow.example_workflow.id
}

data "keep_mapping" "example_mapping_data" {
  id = keep_mapping.example_mapping.id
}
```

## Testing

To run the acceptance tests for this provider, you'll need to set the following environment variables:

```bash
# Keep API Configuration
export KEEP_BACKEND_URL="your-keep-backend-url"
export KEEP_API_KEY="your-keep-api-key"
export KEEP_TIMEOUT="30s"  # Optional, defaults to 30s

# AKS Provider Test Configuration
export AKS_SUBSCRIPTION_ID="your-subscription-id"
export AKS_CLIENT_ID="your-client-id"
export AKS_CLIENT_SECRET="your-client-secret"
export AKS_TENANT_ID="your-tenant-id"
export AKS_RESOURCE_GROUP_NAME="your-resource-group"
export AKS_RESOURCE_NAME="your-resource-name"
```

Then run the tests using:

```bash
# Run all tests
TF_ACC=1 go test ./keep -v

# Run specific tests
TF_ACC=1 go test ./keep -v -run "TestAccProvider|TestAccResourceProvider"
```

Note: These are acceptance tests that create and destroy real resources. Make sure you're using test credentials and resources.

### Running Tests with Docker Compose

You can also run the tests using Docker Compose, which will automatically set up a local Keep backend instance.

Then run the tests with:

```bash
# Start the Keep backend
docker compose up -d

# Run tests with test credentials
export AKS_SUBSCRIPTION_ID=test-subscription-id
export AKS_CLIENT_ID=test-client-id
export AKS_CLIENT_SECRET=test-client-secret
export AKS_TENANT_ID=test-tenant-id
export AKS_RESOURCE_GROUP_NAME=test-resource-group
export AKS_RESOURCE_NAME=test-resource-name
TF_ACC=1 KEEP_BACKEND_URL=http://localhost:8080 KEEP_API_KEY=keepappkey go test ./keep -v

# Clean up
docker compose down
```

For more information, please refer to the [documentation](https://registry.terraform.io/providers/justtrackio/keep/latest/docs).

You can also find some hands-on examples in the [examples](./examples) directory.

You feel overwhelmed with these bunch of information? Don't worry, we got you covered. Just join keep slack workspace and throw your questions.

[![Slack](https://img.shields.io/badge/Slack-4A154B?style=for-the-badge&logo=slack&logoColor=white)](https://slack.keephq.dev)
