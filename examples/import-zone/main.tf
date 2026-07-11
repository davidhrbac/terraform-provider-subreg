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
