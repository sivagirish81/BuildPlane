#!/usr/bin/env ruby
# frozen_string_literal: true

require "yaml"

rendered_path = ARGV.fetch(0)
expected_secret_name = ARGV.fetch(1, "buildplane-postgres")
expected_secret_key = ARGV.fetch(2, "database-url")
expect_rendered_secret = ENV["EXPECT_RENDERED_DATABASE_SECRET"] == "1"
expect_postgres_statefulset = ENV["EXPECT_POSTGRES_STATEFULSET"] == "1"

documents = YAML.load_stream(File.read(rendered_path)).compact

database_secret = documents.find do |document|
  document["kind"] == "Secret" &&
    document.dig("metadata", "name") == expected_secret_name
end

if database_secret && !expect_rendered_secret
  abort "Helm rendered #{expected_secret_name}; the default chart must reference an externally managed database Secret"
end

if expect_rendered_secret
  abort "Missing Helm-rendered #{expected_secret_name} Secret" unless database_secret

  unless database_secret.dig("stringData", expected_secret_key)
    abort "Helm-rendered #{expected_secret_name} Secret is missing #{expected_secret_key}"
  end
end

if expect_postgres_statefulset
  postgres_service = documents.find do |document|
    document["kind"] == "Service" &&
      document.dig("metadata", "name") == expected_secret_name
  end

  postgres_statefulset = documents.find do |document|
    document["kind"] == "StatefulSet" &&
      document.dig("metadata", "name") == expected_secret_name
  end

  abort "Missing local PostgreSQL Service #{expected_secret_name}" unless postgres_service
  abort "Missing local PostgreSQL StatefulSet #{expected_secret_name}" unless postgres_statefulset
end

required_deployments = {
  "buildplane-control-plane" => "control-plane",
  "buildplane-scheduler" => "scheduler",
  "buildplane-worker-general" => "worker",
  "buildplane-worker-ai" => "worker"
}

required_deployments.each do |deployment_name, container_name|
  deployment = documents.find do |document|
    document["kind"] == "Deployment" &&
      document.dig("metadata", "name") == deployment_name
  end

  abort "Missing Deployment #{deployment_name}" unless deployment

  containers = deployment.dig("spec", "template", "spec", "containers") || []
  container = containers.find { |candidate| candidate["name"] == container_name }

  abort "Missing container #{container_name} in #{deployment_name}" unless container

  env = container["env"] || []
  database_env = env.find { |entry| entry["name"] == "BUILDPLANE_DATABASE_URL" }

  abort "Missing BUILDPLANE_DATABASE_URL in #{deployment_name}" unless database_env

  secret_ref = database_env.dig("valueFrom", "secretKeyRef")

  unless secret_ref&.fetch("name", nil) == expected_secret_name &&
         secret_ref&.fetch("key", nil) == expected_secret_key
    abort "#{deployment_name} has wrong database Secret reference: #{secret_ref.inspect}"
  end
end

puts "database Secret references verified"
