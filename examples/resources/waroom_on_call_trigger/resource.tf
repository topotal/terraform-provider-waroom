data "waroom_on_call_escalation_policy" "primary" {
  name = "Primary escalation"
}

# Call the primary escalation policy when a critical or high incident is created
# in my-service from an integration or the web UI, and retry the escalation twice
# when no one responds.
resource "waroom_on_call_trigger" "example" {
  name                    = "critical incidents"
  service_name            = "my-service"
  escalation_policy_uuids = [data.waroom_on_call_escalation_policy.primary.uuid]
  severity_filters        = ["critical", "high"]
  source_filters          = ["integration", "web"]
  retry_count             = 2
}

# A trigger is enabled by default. Set enabled = false to create it without paging anyone.
resource "waroom_on_call_trigger" "disabled" {
  name                    = "low incidents"
  service_name            = "my-service"
  escalation_policy_uuids = [data.waroom_on_call_escalation_policy.primary.uuid]
  severity_filters        = ["low"]
  enabled                 = false
}
