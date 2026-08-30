data "fopost_labels" "acme" {
  workspace_id = "ws_01hzy8example"
}

output "label_names" {
  value = [for label in data.fopost_labels.acme.labels : label.name]
}
