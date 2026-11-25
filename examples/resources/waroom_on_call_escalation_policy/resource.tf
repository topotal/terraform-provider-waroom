data "waroom_on_call_team" "sre" {
  name = "SRE"
}

data "waroom_on_call_team" "backend" {
  name = "Backend"
}

# Call up to two people in the SRE team first, then everyone in the Backend team at once,
# on weekdays during business hours.
resource "waroom_on_call_escalation_policy" "example" {
  name           = "Primary escalation"
  timezone       = "Asia/Tokyo"
  call_depth     = 2
  call_traversal = "depth_first"

  steps = [
    {
      team_uuid = data.waroom_on_call_team.sre.uuid
    },
    {
      team_uuid         = data.waroom_on_call_team.backend.uuid
      step_type         = "fallback"
      simultaneous_call = true
    },
  ]

  time_slots = [
    for day in [1, 2, 3, 4, 5] : {
      day_of_week = day
      start_time  = "09:00"
      end_time    = "18:00"
    }
  ]
}

# A slot whose end_time is earlier than its start_time continues into the next day.
resource "waroom_on_call_escalation_policy" "night" {
  name = "Night shift"

  steps = [
    { team_uuid = data.waroom_on_call_team.sre.uuid },
  ]

  time_slots = [
    { day_of_week = 5, start_time = "22:00", end_time = "06:00" },
  ]
}
