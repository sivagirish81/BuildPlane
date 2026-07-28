# BuildPlane GKE Terraform

This directory is a first GKE infrastructure scaffold for Phase 12. It creates:

- Required GCP service APIs
- A VPC network and regional subnet
- Secondary ranges for GKE Pods and Services
- A regional GKE cluster
- A managed node pool
- An Artifact Registry Docker repository

It does not create PostgreSQL, Redis persistence, DNS, TLS, or secret-manager
integrations yet. PostgreSQL remains an external dependency provided to the
Helm release as a Kubernetes Secret.

## Usage

```bash
cp terraform.tfvars.example terraform.tfvars
terraform init
terraform plan
terraform apply
```

After apply:

```bash
terraform output get_credentials_command
```

Run the printed `gcloud container clusters get-credentials ...` command before
installing the Helm chart.

## State

This scaffold intentionally leaves Terraform backend configuration unset. For
real shared use, configure a remote backend such as a GCS bucket before
running `terraform apply`.
