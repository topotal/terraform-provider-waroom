# Look up an escalation policy by name.
data "waroom_on_call_escalation_policy" "example" {
  name = "Primary escalation"
}

# Or by UUID.
data "waroom_on_call_escalation_policy" "by_uuid" {
  uuid = "770e8400-e29b-41d4-a716-446655440001"
}
