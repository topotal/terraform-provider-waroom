resource "waroom_service_architecture_context" "example" {
  service_name = "my-service"
  blob         = <<-EOT
    # Architecture

    - Web: Ruby on Rails on ECS
    - Database: Aurora PostgreSQL
    - Cache: ElastiCache for Redis
  EOT
}

# The content can also be loaded from a Markdown file.
resource "waroom_service_architecture_context" "from_file" {
  service_name = "my-other-service"
  blob         = file("${path.module}/architecture.md")
}
