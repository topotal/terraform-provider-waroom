terraform {
  required_providers {
    waroom = {
      source = "registry.terraform.io/topotal/waroom"
    }
  }
}

provider "waroom" {
  endpoint = "https://api.staging.waroom.com/api/v0"
}

resource "waroom_service_label" "naito" {
  service_name = "ないとぅー"
  name         = "created-from-terraform03"
  color        = "40ff20"
}
