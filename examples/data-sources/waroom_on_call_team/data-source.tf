# Look up an on-call team by name
data "waroom_on_call_team" "example" {
  name = "SRE"
}

# Or by UUID
data "waroom_on_call_team" "by_uuid" {
  uuid = "00000000-0000-0000-0000-000000000000"
}
