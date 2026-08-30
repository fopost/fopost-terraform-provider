resource "fopost_workspace" "acme" {
  name = "Acme Social"
  slug = "acme-social"
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

# FoPost signs every delivery with this secret and returns it exactly once, at
# creation. Hand it to whatever verifies the signature.
output "fopost_webhook_secret" {
  value     = fopost_webhook.delivery.secret
  sensitive = true
}
