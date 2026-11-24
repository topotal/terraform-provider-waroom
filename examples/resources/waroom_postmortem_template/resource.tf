# Write the Markdown content inline with a heredoc.
resource "waroom_postmortem_template" "inline" {
  title = "Standard postmortem"
  blob  = <<-EOT
    # Postmortem

    ## Summary

    ## Impact

    ## Timeline

    ## Root cause

    ## Action items
  EOT
}

# Or read it from a Markdown file placed next to this configuration.
resource "waroom_postmortem_template" "from_file" {
  title = "Security incident postmortem"
  blob  = file("${path.module}/postmortem.md")
}
