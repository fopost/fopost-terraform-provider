data "fopost_workspaces" "all" {}

output "workspace_slugs" {
  value = [for workspace in data.fopost_workspaces.all.workspaces : workspace.slug]
}
