data "fopost_account" "brand_linkedin" {
  id = "acc_01hzy8example"
}

output "linkedin_handle" {
  value = data.fopost_account.brand_linkedin.username
}
