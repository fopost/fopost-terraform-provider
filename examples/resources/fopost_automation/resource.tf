resource "fopost_workspace" "acme" {
  name = "Acme Social"
  slug = "acme-social"
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
