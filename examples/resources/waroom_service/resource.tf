# Note: the Waroom API cannot delete a service.
# Destroying this resource only removes it from the Terraform state.
resource "waroom_service" "example" {
  name = "my-service"
}
