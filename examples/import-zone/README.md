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
TF_CLI_CONFIG_FILE=terraform.rc terraform plan -generate-config-out="generated_resources.tf"
TF_CLI_CONFIG_FILE=terraform.rc terraform apply
```

`generated_resources.tf` will contain one `subreg_dns_record` per record in the zone.

The placeholder `subreg_dns_record.imported` block in `main.tf` is only used as a target for import blocks. It ignores changes and prevents destroy, so applying after import will not change DNS records. You can delete it after config generation.

## Notes
- The import uses `domain:id` format and requires Terraform >= 1.5.
- You can remove `main.tf` and keep `generated_resources.tf` after import if desired.
