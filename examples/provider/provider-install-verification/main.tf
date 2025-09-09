terraform {
  required_providers {
    waroom = {
      source = "registry.terraform.io/topotal/waroom"
    }
  }
}

provider "waroom" {
  api_key = "key"
}

data "waroom_example" "example" {
  configurable_attribute = "test"
}

data "waroom_service_label" "example" {
  name = "test"
}

