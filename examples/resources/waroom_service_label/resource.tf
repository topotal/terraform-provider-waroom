resource "waroom_service_label" "example" {
  service_name = "my-service"
  name         = "production"
  color        = "ff5733" # Red color in hex format (without #)
}
