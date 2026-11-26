# Terraform Provider for Waroom

- Website: [waroom.com](https://waroom.com/)
- Documentation: [`docs/`](docs/)
- Changelog: [CHANGELOG.md](CHANGELOG.md)

The Waroom Provider enables [Terraform](https://www.terraform.io) to manage [Waroom](https://waroom.com/) resources.

> [!NOTE]
> This provider is in early development (v0.x). Resources and their schemas may change without notice.

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
  endpoint = "https://api.waroom.com"
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
export WAROOM_API_ENDPOINT="https://api.waroom.com"
terraform plan
```

## Resources and Data Sources

| Name                                        | Resource                                                     | Data Source                                                |
| ------------------------------------------- | ------------------------------------------------------------ | ---------------------------------------------------------- |
| `waroom_incident_description_template`      | [docs](docs/resources/incident_description_template.md)      |                                                            |
| `waroom_on_call_escalation_policy`          | [docs](docs/resources/on_call_escalation_policy.md)          | [docs](docs/data-sources/on_call_escalation_policy.md)     |
| `waroom_on_call_team`                       | [docs](docs/resources/on_call_team.md)                       | [docs](docs/data-sources/on_call_team.md)                  |
| `waroom_on_call_trigger`                    | [docs](docs/resources/on_call_trigger.md)                    | [docs](docs/data-sources/on_call_trigger.md)               |
| `waroom_postmortem_template`                | [docs](docs/resources/postmortem_template.md)                | [docs](docs/data-sources/postmortem_template.md)           |
| `waroom_runbook`                            |                                                              | [docs](docs/data-sources/runbook.md)                       |
| `waroom_service`                            | [docs](docs/resources/service.md)                            | [docs](docs/data-sources/service.md)                       |
| `waroom_service_architecture_context`       | [docs](docs/resources/service_architecture_context.md)       |                                                            |
| `waroom_service_incident_document_template` | [docs](docs/resources/service_incident_document_template.md) |                                                            |
| `waroom_service_label`                      | [docs](docs/resources/service_label.md)                      | [docs](docs/data-sources/service_label.md)                 |
| `waroom_service_notification_slack_channel` | [docs](docs/resources/service_notification_slack_channel.md) |                                                            |
| `waroom_service_runbook_rule`               | [docs](docs/resources/service_runbook_rule.md)               |                                                            |
| `waroom_service_webhook_configuration`      |                                                              | [docs](docs/data-sources/service_webhook_configuration.md) |
| `waroom_user`                               |                                                              | [docs](docs/data-sources/user.md)                          |

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.16

## Development

See [DEVELOPMENT.md](DEVELOPMENT.md) for building, testing, and developing the provider.

## License

[MIT](LICENSE)
