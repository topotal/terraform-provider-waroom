# Look up a user by nickname.
data "waroom_user" "by_nickname" {
  nickname = "alice"
}

# Look up a user by the ID of the linked Slack user.
data "waroom_user" "by_slack_user" {
  slack_user_key = "U0123ABCD"
}

# Look up a user by sub.
data "waroom_user" "by_sub" {
  sub = "auth0|abc123"
}

# Use the looked-up sub in an on-call rotation instead of hard-coding it.
data "waroom_user" "alice" {
  slack_user_key = "U0123ABCD" # or nickname = "alice"
}

resource "waroom_on_call_team" "platform" {
  name  = "Platform"
  color = "1E90FF"

  rotations = [
    {
      timezone              = "Asia/Tokyo"
      started_at            = "2026-01-05T10:00:00+09:00"
      shift_handoff_time    = "10:00"
      shift_handoff_wday    = 1
      participant_user_subs = [data.waroom_user.alice.sub]
    },
  ]

  lifecycle {
    # Only users who have verified their phone number can join a rotation.
    precondition {
      condition     = data.waroom_user.alice.phone_verified
      error_message = "User ${data.waroom_user.alice.nickname} has not verified a phone number."
    }
  }
}

# Build an on-call rotation from Slack user IDs instead of hard-coding subs.
# Only users who have verified their phone number can join a rotation,
# so fail at plan time when one of them has not.
locals {
  sre_slack_user_keys = ["U0123ABCD", "U0456EFGH"]
}

data "waroom_user" "sre" {
  for_each       = toset(local.sre_slack_user_keys)
  slack_user_key = each.value

  lifecycle {
    postcondition {
      condition     = self.phone_verified
      error_message = "User ${self.nickname} has not verified a phone number and cannot join an on-call rotation."
    }
  }
}

resource "waroom_on_call_team" "sre" {
  name  = "SRE"
  color = "1E90FF"

  rotations = [
    {
      timezone              = "Asia/Tokyo"
      started_at            = "2026-01-05T10:00:00+09:00"
      shift_handoff_time    = "10:00"
      shift_handoff_wday    = 1
      participant_user_subs = [for key in local.sre_slack_user_keys : data.waroom_user.sre[key].sub]
    },
  ]
}
