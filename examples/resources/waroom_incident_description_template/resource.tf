# The organization has only one incident description template.
resource "waroom_incident_description_template" "example" {
  blob = <<-EOT
    ## Impact

    ## Current status

    ## Next update
  EOT
}
