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
