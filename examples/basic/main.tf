terraform {
  required_providers {
    subreg = {
      source = "davidhrbac/subreg"
    }
  }
}

provider "subreg" {
  login    = var.subreg_login
  password = var.subreg_password
  wsdl_url = var.subreg_wsdl_url
}

data "subreg_dns_zone" "zone" {
  domain = var.subreg_domain
}

resource "subreg_dns_record" "example" {
  count = var.create_example_record ? 1 : 0

  domain  = var.subreg_domain
  name    = var.example_record_name
  type    = var.example_record_type
  content = var.example_record_content
  prio    = var.example_record_prio
  ttl     = var.example_record_ttl
}
