# Write the Markdown content inline with a heredoc.
resource "waroom_runbook" "inline" {
  namespace = "/api-server/recovery"
  blob      = <<-EOT
    # API server recovery

    1. Check the error rate on the dashboard.
    2. Restart the API server.
    3. Confirm the error rate is back to normal.
  EOT
}

# Or read it from a Markdown file placed next to this configuration.
resource "waroom_runbook" "from_file" {
  namespace = "/database/failover"
  blob      = file("${path.module}/runbook.md")
}
