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

data "waroom_service_label" "naitooo" {
  service_name = "ないとぅー"
  name         = "hoge"
}

