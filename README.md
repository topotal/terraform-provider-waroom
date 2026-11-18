# Terraform Provider for Waroom

- Website: [waroom.com](https://waroom.com/)
- Documentation: [`docs/`](docs/)
- Changelog: [CHANGELOG.md](CHANGELOG.md)

The Waroom Provider enables [Terraform](https://www.terraform.io) to manage [Waroom](https://waroom.com/) resources.

> [!NOTE]
> This provider is under active development and has not been released yet. Resources and their schemas may change without notice.

## Usage Example

```terraform
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
  api_key  = var.waroom_api_key
  endpoint = "https://api.waroom.com/api/v0"
}

# Create a service label
resource "waroom_service_label" "example" {
  service_name = "my-service"
  name         = "production"
  color        = "ff5733"
}
```

See [`examples/`](examples/) for more examples.

## Authentication

The provider needs a Waroom API key and API endpoint. Each can be configured either in the provider block or via an environment variable.

| Provider argument | Environment variable  | Required | Description          |
| ----------------- | --------------------- | -------- | -------------------- |
| `api_key`         | `WAROOM_API_KEY`      | Yes      | Waroom API key.      |
| `endpoint`        | `WAROOM_API_ENDPOINT` | Yes      | Waroom API endpoint. |

```shell
export WAROOM_API_KEY="<your-api-key>"
export WAROOM_API_ENDPOINT="https://api.waroom.com/api/v0"
terraform plan
```

## Resources and Data Sources

| Name                   | Resource                                          | Data Source                                          |
| ---------------------- | ------------------------------------------------- | ---------------------------------------------------- |
| `waroom_service_label` | [docs](docs/resources/service_label.md)           | [docs](docs/data-sources/service_label.md)           |

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.16

## Development

See [DEVELOPMENT.md](DEVELOPMENT.md) for building, testing, and developing the provider.

## License

[MIT](LICENSE)
