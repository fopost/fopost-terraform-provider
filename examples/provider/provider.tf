terraform {
  required_providers {
    fopost = {
      source  = "fopost/fopost"
      version = "~> 0.1"
    }
  }
}

# With no arguments the provider reads FOPOST_API_KEY from the environment,
# which keeps the key out of the configuration and out of version control.
provider "fopost" {}
