# Basic Example

This example reads a zone and optionally creates a test record.

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

3) Run Terraform:

```bash
TF_CLI_CONFIG_FILE=terraform.rc terraform init
TF_CLI_CONFIG_FILE=terraform.rc terraform plan
```

## Optional: create a test record
Set `create_example_record = true` in `terraform.tfvars` and run:

```bash
TF_CLI_CONFIG_FILE=terraform.rc terraform apply
```

## Notes
- Use `SUBREG_WSDL_URL=https://demoreg.net/wsdl` for the test environment.
- The data source `subreg_dns_zone` only reads records.
