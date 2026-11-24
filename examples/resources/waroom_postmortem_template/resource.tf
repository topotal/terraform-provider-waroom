# postmortem.md is a Markdown file placed next to this configuration.
resource "waroom_postmortem_template" "example" {
  title = "Standard postmortem"
  blob  = file("${path.module}/postmortem.md")
}
