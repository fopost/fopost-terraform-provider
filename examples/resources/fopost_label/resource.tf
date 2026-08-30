resource "fopost_workspace" "acme" {
  name = "Acme Social"
  slug = "acme-social"
}

resource "fopost_label" "launch" {
  workspace_id = fopost_workspace.acme.id
  name         = "Launch Week"
  color        = "#2563eb"
}
