# __generated__ by Terraform
# Review these resources and move them into your own configuration.

# __generated__ by Terraform from "example.com:1234567"
resource "subreg_dns_record" "root_mx_1234567" {
  content = "smtp.example.net"
  domain  = "example.com"
  name    = "@"
  prio    = 10
  ttl     = 600
  type    = "MX"
}

# __generated__ by Terraform from "example.com:1234568"
resource "subreg_dns_record" "www_cname_1234568" {
  content = "www1.example.net"
  domain  = "example.com"
  name    = "www"
  ttl     = 600
  type    = "CNAME"
}
