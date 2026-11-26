terraform {
  required_providers {
    waroom = {
      source = "topotal/waroom"
    }
  }
}

variable "waroom_api_key" {
  type      = string
  sensitive = true
}

# Configure the Waroom Provider
provider "waroom" {
  # api_key can also be set via the WAROOM_API_KEY environment variable
  api_key = var.waroom_api_key

  # endpoint can also be set via the WAROOM_API_ENDPOINT environment variable
  endpoint = "https://api.waroom.com" # host only; do not add /api/v0
}
