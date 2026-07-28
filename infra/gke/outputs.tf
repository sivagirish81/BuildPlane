output "cluster_name" {
  description = "Created GKE cluster name."
  value       = google_container_cluster.buildplane.name
}

output "cluster_location" {
  description = "Created GKE cluster location."
  value       = google_container_cluster.buildplane.location
}

output "artifact_registry_repository_url" {
  description = "Docker repository host/path for image tags."
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.buildplane.repository_id}"
}

output "get_credentials_command" {
  description = "Command to configure kubectl for this cluster."
  value       = "gcloud container clusters get-credentials ${google_container_cluster.buildplane.name} --region ${google_container_cluster.buildplane.location} --project ${var.project_id}"
}
