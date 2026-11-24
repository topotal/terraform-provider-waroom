# The organization has only one incident description template, so declare this resource at most once.

# Write the content inline with a heredoc.
resource "waroom_incident_description_template" "example" {
  blob = <<-EOT
    ## Impact

    ## Current status

    ## Next update
  EOT
}

# Or read it from a Markdown file placed next to this configuration:
#
# resource "waroom_incident_description_template" "example" {
#   blob = file("${path.module}/incident_description.md")
# }
