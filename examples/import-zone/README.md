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

`imports.tf` and `generated_resources.tf` are generated locally and ignored by git.
`generated_resources.tf` will contain one `subreg_dns_record` per record in the zone.

## Notes
- The import uses `domain:id` format and requires Terraform >= 1.5.
- `main.tf` only wires the provider; you can keep the generated config in a separate file or merge it into your own layout.
