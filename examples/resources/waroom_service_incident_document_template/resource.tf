resource "waroom_service" "api" {
  name = "api"
}

resource "waroom_service" "web" {
  name = "web"
}

# Write the Markdown content inline with a heredoc.
resource "waroom_service_incident_document_template" "api" {
  service_name = waroom_service.api.name
  blob         = <<-EOT
    # Incident document

    ## Impact

    ## Timeline

    ## Mitigation
  EOT
}

# Or read it from a Markdown file placed next to this configuration.
resource "waroom_service_incident_document_template" "web" {
  service_name = waroom_service.web.name
  blob         = file("${path.module}/incident_document.md")
}
