terraform {
  required_providers {
    oncallalert = {
      source = "OnCallAlert/oncallalert"
    }
  }
}

# Point the provider at your own server. The token needs Admin scope
# (profile page -> API Tokens). Both can also come from ONCALLALERT_URL / ONCALLALERT_TOKEN.
provider "oncallalert" {
  url   = "https://oncall.example.com"
  token = var.oncallalert_token
}

variable "oncallalert_token" {
  type      = string
  sensitive = true
}
