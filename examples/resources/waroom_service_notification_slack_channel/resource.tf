resource "waroom_service" "api" {
  name = "api"
}

# Send the incident notifications of the service to a Slack channel.
# The channel must exist in the Slack workspace connected to Waroom.
resource "waroom_service_notification_slack_channel" "api" {
  service_name = waroom_service.api.name
  channel_id   = "C1234567890"
}
