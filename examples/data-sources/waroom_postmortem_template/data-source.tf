# Look up a postmortem template by title.
data "waroom_postmortem_template" "example" {
  title = "Default postmortem"
}

# Or by UUID.
data "waroom_postmortem_template" "by_uuid" {
  uuid = "00000000-0000-0000-0000-000000000000"
}
