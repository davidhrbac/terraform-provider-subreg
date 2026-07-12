# Terraform Provider Subreg

Manage DNS records in Subreg via the SOAP API.

## Requirements
- Terraform 1.x
- Subreg API credentials

## Provider Configuration
Credentials can be set in the provider block or via environment variables.

Environment variables:
- `SUBREG_LOGIN`
- `SUBREG_PASSWORD`
- `SUBREG_WSDL_URL` (optional; defaults to production WSDL)

Example:

```hcl
terraform {
  required_providers {
    subreg = {
      source = "davidhrbac/subreg"
      # version = "..."
    }
  }
}

provider "subreg" {
  login    = var.subreg_login
  password = var.subreg_password
  # wsdl_url = "https://demoreg.net/wsdl"
}
```

## Resource: subreg_dns_record

Manages a single DNS record in a zone.

Arguments:
- `domain` (Required) Registered domain, e.g. `example.com`.
- `name` (Required) Record name without the domain, e.g. `@` or `www`.
- `type` (Required) Record type, e.g. `A`, `AAAA`, `CNAME`, `MX`, `TXT`.
- `content` (Required) Record value (IP, hostname, or text).
- `prio` (Optional) Priority for MX records.
- `ttl` (Optional) TTL in seconds.

Example:

```hcl
resource "subreg_dns_record" "root_a" {
  domain  = "example.com"
  name    = "@"
  type    = "A"
  content = "203.0.113.10"
  ttl     = 300
}

resource "subreg_dns_record" "mail_mx" {
  domain  = "example.com"
  name    = "@"
  type    = "MX"
  content = "mail.example.com"
  prio    = 10
  ttl     = 3600
}
```

## Import

Use `domain:id` format, where `id` is the Subreg record ID:

```bash
terraform import subreg_dns_record.root_a example.com:123
```

## Data Source: subreg_dns_zone

Reads all records in a DNS zone.

Arguments:
- `domain` (Required) Registered domain, e.g. `example.com`.

Attributes:
- `records` List of records with `id`, `name`, `type`, `content`, `prio`, `ttl`.

Example:

```hcl
data "subreg_dns_zone" "example" {
  domain = "example.com"
}

output "zone_records" {
  value = data.subreg_dns_zone.example.records
}
```

## Notes
- Record `type` is normalized to uppercase.
- `name` changes require delete+add (Subreg Modify API does not accept `name`).
- Use `@` for the root record.

## Examples
- Basic usage: `examples/basic`
- Import entire zone with config generation: `examples/import-zone`

## Workflow: import records of a new zone
Use this when you want to take over an existing zone and generate Terraform config from it.

1) Build local provider and CLI config (example uses local plugin mirror):

```bash
cd examples/import-zone
chmod +x setup-local-provider.sh
./setup-local-provider.sh
```

2) Provide credentials and domain (env vars or tfvars):

```bash
cp env.example env.sh
source env.sh
```

3) Generate import blocks with unique resource names:

```bash
chmod +x generate-imports.sh
./generate-imports.sh
```

This creates `examples/import-zone/imports.tf` with one import block per record ID.
`imports.tf` and `generated_resources.tf` are local artifacts and ignored by git.

4) Generate Terraform config and import state:

```bash
TF_CLI_CONFIG_FILE=terraform.rc terraform init
TF_CLI_CONFIG_FILE=terraform.rc terraform plan -generate-config-out="generated_resources.tf"
TF_CLI_CONFIG_FILE=terraform.rc terraform apply
```

5) Review `generated_resources.tf` and clean up:
- Remove any duplicate records you don't want managed.
- If you keep round-robin records, keep one resource per record.
- You can delete `imports.tf` after import is complete.

Notes:
- Subreg enforces TTL >= 600 (or 0 for default).
- Root records are represented as `name = "@"`.
