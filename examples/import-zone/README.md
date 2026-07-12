# Import Zone Example

Use this example to import all records for a domain and generate Terraform configuration.

## Setup
1) Build a local provider binary and CLI config:

```bash
chmod +x setup-local-provider.sh
./setup-local-provider.sh
```

2) Provide variables (pick one):

Option A: `terraform.tfvars`

```bash
cp terraform.tfvars.example terraform.tfvars
```

Option B: environment variables

```bash
cp env.example env.sh
source env.sh
```

## Generate config from imports

```bash
TF_CLI_CONFIG_FILE=terraform.rc terraform init
chmod +x generate-config.sh
./generate-config.sh
TF_CLI_CONFIG_FILE=terraform.rc terraform apply
```

`imports.tf` and `domains/` are generated locally and ignored by git.
`domains/<first-char>/<domain>.tf` will contain one `subreg_domain`, one `subreg_dns_zone`, and one `subreg_dns_record` per record in the zone.
Default `prio = 0` values are omitted from the generated config.
`subreg_dns_zone` includes the desired `dnssec` state.
Resources are sorted as domain, DNSSEC zone, then records.

Templates in this directory:
- `imports.example.tf` shows the import block shape.
- `generated_resources.example.tf` shows the Terraform output produced after import.

## Notes
- The import uses `domain:id` format and requires Terraform >= 1.5.
- `main.tf` only wires the provider; you can keep the generated config in a separate file or merge it into your own layout.
- Use `generate-config.sh` to write config into `domains/<first-char>/<domain>.tf`.
