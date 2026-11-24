resource "waroom_service" "example" {
  name = "my-service"
}

resource "waroom_service_incident_document_template" "example" {
  service_name = waroom_service.example.name
  blob         = <<-EOT
    # Incident document

    ## Impact

    ## Timeline

    ## Mitigation
  EOT
}
