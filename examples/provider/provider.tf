terraform {
  required_providers {
    waroom = {
      source = "topotal/waroom"
    }
  }
}

# Configure the Waroom Provider
provider "waroom" {
  # API key can be set via WAROOM_API_KEY environment variable
  api_key = var.waroom_api_key

  # Optional: Custom API endpoint
  # endpoint can be set via WAROOM_API_ENDPOINT environment variable
  # endpoint = "https://api.waroom.com/api/v0"
}
