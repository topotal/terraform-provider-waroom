# Look up a webhook configuration by name
data "waroom_service_webhook_configuration" "by_name" {
  service_name = "my-service"
  name         = "my-webhook"
}

# Look up a webhook configuration by UUID
data "waroom_service_webhook_configuration" "by_uuid" {
  service_name = "my-service"
  uuid         = "7dcf90f0-de30-4bd3-a797-3d53cef7ec7d"
}

output "webhook_url" {
  value = data.waroom_service_webhook_configuration.by_name.url
}
