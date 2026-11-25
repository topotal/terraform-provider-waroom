# Link the /api-server/recovery runbook to incidents of my-service whose title mentions a database outage
resource "waroom_service_runbook_rule" "example" {
  service_name      = "my-service"
  pattern           = "(?i)database.*down"
  target            = "title" # "title" or "description"
  priority          = 10      # lower value = higher priority
  runbook_namespace = "/api-server/recovery"
}
