resource "fopost_workspace" "acme" {
  name        = "Acme Social"
  slug        = "acme-social"
  type        = "TEAM"
  timezone    = "Europe/Berlin"
  language    = "en"
  country     = "DE"
  website     = "https://acme.example.com"
  description = "Everything Acme publishes."
}
