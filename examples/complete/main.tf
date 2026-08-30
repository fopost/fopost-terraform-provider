# A complete FoPost setup: one workspace, the labels its campaigns report
# against, a webhook that pushes delivery events, and an automation that turns
# the blog feed into scheduled posts.

terraform {
  required_providers {
    fopost = {
      source  = "fopost/fopost"
      version = "~> 0.1"
    }
  }
}

# Reads FOPOST_API_KEY from the environment.
provider "fopost" {}

resource "fopost_workspace" "acme" {
  name        = "Acme Social"
  slug        = "acme-social"
  type        = "TEAM"
  timezone    = "Europe/Berlin"
  language    = "en"
  description = "Everything Acme publishes."
}

resource "fopost_label" "campaign" {
  for_each = {
    launch    = "#2563eb"
    evergreen = "#16a34a"
    support   = "#f59e0b"
  }

  workspace_id = fopost_workspace.acme.id
  name         = title(each.key)
  color        = each.value
}

resource "fopost_webhook" "delivery" {
  workspace_id = fopost_workspace.acme.id
  url          = "https://hooks.acme.example.com/fopost"

  events = [
    "post.published",
    "post.failed",
    "delivery.failed",
    "account.health_changed",
  ]
}

resource "fopost_automation" "blog_to_social" {
  workspace_id = fopost_workspace.acme.id
  name         = "Blog to social"
  trigger_type = "rss_feed"

  trigger_config = jsonencode({
    feed_url = "https://acme.example.com/blog/feed.xml"
  })

  step {
    action_type = "delay"

    action_config = jsonencode({
      minutes = 15
    })
  }

  step {
    action_type = "publish"
  }
}

# Accounts are connected in the FoPost dashboard through each platform's OAuth
# flow, so Terraform reads them rather than creating them.
data "fopost_accounts" "acme" {
  workspace_id = fopost_workspace.acme.id
}

output "connected_accounts" {
  value = [
    for account in data.fopost_accounts.acme.accounts :
    "${account.platform}/${account.username}"
  ]
}

output "webhook_signing_secret" {
  value     = fopost_webhook.delivery.secret
  sensitive = true
}
