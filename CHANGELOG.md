## [v0.0.2](https://github.com/topotal/terraform-provider-waroom/compare/v0.0.1...v0.0.2) - 2026-11-27

- ci: pin the GoReleaser version by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/105
- docs: fix the endpoint example and stale README / llms.txt / DEVELOPMENT.md by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/107
- chore: add CODEOWNERS and gate release PRs on a VERSION file by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/109
- docs: add a security policy by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/108
- fix: send the real versions in the User-Agent by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/110
- ci: drop download-modules now that waroom-client-go is public by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/111
- chore: drop the default code owner for now by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/112
- chore: require a releaser for everything under .github/ by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/114
- ci: run govulncheck and update grpc to v1.83.2 by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/113
- ci: require GPG signing for releases by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/115

## [v0.0.1](https://github.com/topotal/terraform-provider-waroom/commits/v0.0.1) - 2026-11-25

- feat: implement `waroom_service_label` import by @shiimaxx in https://github.com/topotal/terraform-provider-waroom/pull/9
- fix(ServiceLabelResource): properly handle missing service label in Read operation by @shiimaxx in https://github.com/topotal/terraform-provider-waroom/pull/10
- docs: generate initial documentation with terraform-plugin-docs by @shiimaxx in https://github.com/topotal/terraform-provider-waroom/pull/12
- Update hashicorp/setup-terraform action to v4 by @renovate[bot] in https://github.com/topotal/terraform-provider-waroom/pull/23
- Update actions/setup-go action to v7 by @renovate[bot] in https://github.com/topotal/terraform-provider-waroom/pull/22
- Update module github.com/hashicorp/terraform-plugin-docs to v0.25.0 by @renovate[bot] in https://github.com/topotal/terraform-provider-waroom/pull/13
- Update module github.com/hashicorp/terraform-plugin-framework-validators to v0.19.0 by @renovate[bot] in https://github.com/topotal/terraform-provider-waroom/pull/16
- Update actions/checkout action to v7 by @renovate[bot] in https://github.com/topotal/terraform-provider-waroom/pull/21
- Update hashicorp terraform-plugin by @renovate[bot] in https://github.com/topotal/terraform-provider-waroom/pull/24
- chore: Renovate の設定を共通 preset に移行 by @koresou in https://github.com/topotal/terraform-provider-waroom/pull/20
- Pin dependencies by @renovate[bot] in https://github.com/topotal/terraform-provider-waroom/pull/25
- Pin dorny/paths-filter action to 0e4a8c6 by @renovate[bot] in https://github.com/topotal/terraform-provider-waroom/pull/26
- Update dorny/paths-filter action to v4 by @renovate[bot] in https://github.com/topotal/terraform-provider-waroom/pull/27
- Pin actions/setup-go action to d8cbb80 by @renovate[bot] in https://github.com/topotal/terraform-provider-waroom/pull/30
- doc-preview: スクショを画像ごとの artifact にして PR にリンクをコメントする by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/29
- Pin dependencies by @renovate[bot] in https://github.com/topotal/terraform-provider-waroom/pull/31
- Update dependency node to v26 by @renovate[bot] in https://github.com/topotal/terraform-provider-waroom/pull/32
- chore: add client-coverage and implement-candidate skills by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/35
- ci: fetch waroom-client-go with the waroom ecosystem App by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/36
- ci: install Task with go-task/setup-task by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/37
- feat(tfgen): support data source scaffolding with task gen:d by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/52
- chore: keep implement-candidate PR bodies short by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/53
- chore(task): use only the checkout's own go.work by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/55
- chore(task): add task playground to try the provider with terraform by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/56
- ci(doc-preview): bump the preview action to playwright 1.65.1 by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/59
- chore(task): make task playground quieter and clarify the direnv step by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/58
- feat: add waroom_service resource and data source by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/54
- chore(deps): bump waroom-client-go from v0.0.4 to v0.0.6 by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/66
- ci: cache the Go build cache and run go vet in the unit-test job by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/67
- feat: add waroom_postmortem_template data source by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/69
- feat: add waroom_on_call_team data source by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/73
- feat: add waroom_on_call_trigger data source by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/75
- feat: add waroom_on_call_escalation_policy data source by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/74
- docs: add import block examples by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/81
- feat: add waroom_service_architecture_context resource by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/77
- feat: add waroom_postmortem_template resource by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/78
- feat: add waroom_runbook data source by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/70
- feat: add waroom_service_incident_document_template resource by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/79
- feat: add waroom_incident_description_template resource by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/80
- docs: file API and client bugs in waroom-client-go first by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/68
- feat: add waroom_service_runbook_rule resource by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/85
- feat: add waroom_on_call_team resource by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/87
- feat: add waroom_on_call_escalation_policy resource by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/86
- test: create the escalation policy data source prerequisites with resources by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/92
- feat: add waroom_on_call_trigger resource by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/93
- feat: add waroom_service_webhook_configuration data source by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/76
- feat: add waroom_service_notification_slack_channel resource by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/95
- feat: add waroom_user data source by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/72
- fix: accept UUIDs of any version in the UUID validator by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/90
- docs: explain how to unmanage singleton templates and fix postmortem example title by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/97
- fix: reject blank (Unicode whitespace) template content by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/98
- feat: validate IANA time zone names in timezone attributes by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/99
- ci: release with tagpr and GoReleaser by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/100
- ci: run tagpr with the ecosystem App token by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/102
- ci: keep the App key away from jobs that run repository code by @Sho2010 in https://github.com/topotal/terraform-provider-waroom/pull/103

## 0.1.0 (Unreleased)

FEATURES:

* **New Resource:** `waroom_service_label`
* **New Data Source:** `waroom_service_label`
