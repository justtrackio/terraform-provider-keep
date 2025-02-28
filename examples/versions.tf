terraform {
  required_providers {
    keep = {
      source  = "justtrackio/keep"
      version = "~> 0.2.0"
    }
  }
  
  required_version = ">= 1.0.0"
  
  # Uncomment to use Terraform Cloud backend
  # cloud {
  #   organization = "your-organization"
  #
  #   workspaces {
  #     name = "keep-workspace"
  #   }
  # }
}
