# Look up a trigger by UUID.
data "waroom_on_call_trigger" "by_uuid" {
  uuid = "770e8400-e29b-41d4-a716-446655440001"
}

# Look up a trigger by name within a service.
data "waroom_on_call_trigger" "critical" {
  service_name = "example-service"
  name         = "critical incidents"
}

output "critical_trigger_escalation_policy_uuids" {
  value = data.waroom_on_call_trigger.critical.escalation_policies[*].uuid
}
