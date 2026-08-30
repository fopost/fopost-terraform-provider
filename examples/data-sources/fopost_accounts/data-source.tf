data "fopost_accounts" "acme" {
  workspace_id = "ws_01hzy8example"
}

# Anything that is not healthy needs reconnecting in the FoPost dashboard.
output "accounts_needing_attention" {
  value = [
    for account in data.fopost_accounts.acme.accounts :
    account.username if account.health_status != "healthy"
  ]
}
