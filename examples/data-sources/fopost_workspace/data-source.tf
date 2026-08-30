data "fopost_workspace" "acme" {
  id = "ws_01hzy8example"
}

output "connected_platforms" {
  value = [for account in data.fopost_workspace.acme.accounts : account.platform]
}
