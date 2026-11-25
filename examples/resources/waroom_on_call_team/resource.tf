# participant_user_subs takes user subs, which can be looked up with GET /api/v0/users.
# Only users who have verified their phone number can join a rotation.
locals {
  alice = "auth0|alice"
  bob   = "auth0|bob"
}

resource "waroom_on_call_team" "example" {
  name  = "SRE"
  color = "1E90FF" # hex format (without #)

  rotations = [
    # Weekdays: hand off every Monday at 10:00, active 09:00-18:00 on Monday to Friday.
    {
      timezone              = "Asia/Tokyo"
      started_at            = "2026-01-05T10:00:00+09:00"
      shift_handoff_time    = "10:00"
      shift_handoff_wday    = 1 # Monday
      participant_user_subs = [local.alice, local.bob]
      time_slots = [
        for day in [1, 2, 3, 4, 5] : { day_of_week = day, start_time = "09:00", end_time = "18:00" }
      ]
    },
    # Off hours: hand off every day at 18:00, always active.
    # Rotations earlier in the list take priority where their time slots overlap.
    {
      timezone              = "Asia/Tokyo"
      started_at            = "2026-01-05T18:00:00+09:00"
      shift_handoff_time    = "18:00"
      interval_days         = 1 # shift_handoff_wday must be omitted unless interval_days is a multiple of 7
      participant_user_subs = [local.bob, local.alice]
    },
  ]
}
