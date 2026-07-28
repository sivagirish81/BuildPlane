variable "project_id" {
  description = "GCP project ID where BuildPlane infrastructure will be created."
  type        = string
}

variable "region" {
  description = "GCP region for the GKE cluster and Artifact Registry repository."
  type        = string
  default     = "us-central1"
}

variable "cluster_name" {
  description = "GKE cluster name."
  type        = string
  default     = "buildplane"
}

variable "network_name" {
  description = "VPC network name."
  type        = string
  default     = "buildplane"
}

variable "subnet_cidr" {
  description = "Primary subnet CIDR for GKE nodes."
  type        = string
  default     = "10.20.0.0/20"
}

variable "pods_cidr" {
  description = "Secondary subnet CIDR for GKE Pods."
  type        = string
  default     = "10.24.0.0/14"
}

variable "services_cidr" {
  description = "Secondary subnet CIDR for Kubernetes Services."
  type        = string
  default     = "10.28.0.0/20"
}

variable "node_machine_type" {
  description = "Machine type for the first BuildPlane node pool."
  type        = string
  default     = "e2-standard-4"
}

variable "node_disk_size_gb" {
  description = "Boot disk size for GKE worker nodes."
  type        = number
  default     = 50
}

variable "node_min_count" {
  description = "Minimum node count for the primary node pool."
  type        = number
  default     = 1
}

variable "node_max_count" {
  description = "Maximum node count for the primary node pool."
  type        = number
  default     = 3
}

variable "release_channel" {
  description = "GKE release channel."
  type        = string
  default     = "REGULAR"
}

variable "artifact_registry_repository" {
  description = "Artifact Registry Docker repository ID for BuildPlane images."
  type        = string
  default     = "buildplane"
}

variable "node_service_account_email" {
  description = "Optional service account email for GKE nodes. Empty uses the Compute Engine default service account."
  type        = string
  default     = null
}

variable "enable_apis" {
  description = "Whether Terraform should enable required GCP service APIs."
  type        = bool
  default     = true
}

variable "deletion_protection" {
  description = "Whether to enable GKE deletion protection."
  type        = bool
  default     = true
}
