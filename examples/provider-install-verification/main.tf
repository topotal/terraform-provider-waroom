terraform {
  required_providers {
    waroom = {
      source = "registry.terraform.io/topotal/waroom"
    }
  }
}

provider "waroom" {}
data "runbook" "example" {}

