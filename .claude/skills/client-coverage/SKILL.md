---
name: client-coverage
description: Compare the waroom-client-go API (the version pinned in go.mod) against the resources / data sources implemented in terraform-provider-waroom, and list what is unimplemented, not planned, and implemented. After confirmation, create one GitHub issue per unimplemented candidate. Use when asked to find unimplemented resources, check client-go coverage, turn the diff into issues after bumping the client, or "client-coverage".
---

# client-coverage

Find the APIs provided by waroom-client-go (the version pinned in go.mod) that are not yet exposed as resources / data sources in this provider, and file an issue per candidate.

## Source of truth

| Status | How it is determined |
| --- | --- |
| Implemented | Registered in `Resources()` / `DataSources()` in `internal/provider/provider.go` with a matching `TypeName` |
| Unimplemented (has issue) | An issue with the marker is **open** |
| Not planned | An issue with the marker is **closed (state_reason = not_planned)** |
| Unimplemented (no issue) | None of the above → candidate for a new issue |

The marker is an HTML comment embedded in the issue body, so matching still works if the title changes:

```
<!-- client-coverage: kind=resource name=waroom_runbook -->
```

- `kind` is `resource` or `data_source`
- `name` is the Terraform type name (with the `waroom_` prefix)

To decide not to support something, comment the reason and then **Close as not planned**. The skill excludes it from new-issue candidates, but **always lists it with its reason in every report** (so it does not get silently forgotten; the section is shown even when empty). If an issue is closed as completed but no implementation exists in the code, report it as an inconsistency.

## Steps

### 1. Collect the client API and the provider's implementation status

```sh
"${CLAUDE_SKILL_DIR}/scripts/list-client-api.sh"
```

- `go.work` points to `../waroom-client-go` (a local checkout), so the script uses `GOWORK=off` to read **the version pinned in go.mod**. Unreleased APIs in the local checkout are out of scope.
- For method signatures and type details, read `waroom/*.go` and `waroom/types.go` under the `source` directory printed by the script.

### 2. Map to Terraform candidates

Build candidates from each Service's methods. This cannot be done mechanically; use the following as guidelines.

- **resource**: has Create (or an upsert via Put/Update) and Delete, and its state can be read back via Get or List
  - Things that live under a parent ID (e.g. `RunbookRuleService` under a service) take the parent ID as a required attribute
  - Settings attached once per parent, such as `*NotificationSlackChannel` / `*IncidentDocumentTemplate` / `*ArchitectureContext` on `ServiceService`, become standalone resources (e.g. `waroom_service_notification_slack_channel`)
  - Organization-wide singletons such as `IncidentDescriptionTemplateService` become singleton resources
- **data_source**: has Get or List. If only List exists, make a data source that filters by name or similar
- **Record API constraints in notes**: for each candidate, note missing operations (no `Delete`, no `Get`, no `Update`) and singleton shape. These go into the report's notes column and the issue's "Implementation notes" section
- **Follow existing naming**: reflect the API's parent/child relationship in the name, as in `LabelService` → `waroom_service_label`. Check existing `TypeName`s before choosing a name
- **Operational data** (Incidents / Postmortems / ActionItems and their Update* methods) is likely a poor fit for Terraform management. Do not drop them; list them as "Needs decision" (if already closed as not planned, that takes precedence)

### 3. Match against existing issues

```sh
gh issue list --state all --limit 200 --search 'in:body "client-coverage:"' \
  --json number,title,state,stateReason,closedAt,url,body
```

Match the body marker by `kind` + `name` and classify each candidate according to the "Source of truth" table. The marker, not the label, is authoritative; if a marked issue lacks the `resource-candidate` label, list it under Inconsistencies.

For each issue classified as not planned, check its reason and whether it needs reconsideration:

```sh
gh issue view <number> --json closedAt,comments \
  -q '.closedAt as $c | [.comments[] | select(.createdAt <= $c)] | last | .body'
```

- **Reason**: summarize the last comment up to the close time in one line. If there is no comment, write "No reason given" (a not-planned close without a reason should be revisited)
- **Reconsider flag**: if the version in the issue body's `client:` line differs from the currently pinned version, and the relevant Service's methods were added or removed in between, mark it "Reconsider". Get both versions' sources with `GOWORK=off go mod download -json <mod>@<ver>` and compare

Also confirm that the parent tracking issue (currently #8, titled 「実装予定Resource/Data Source」) is open. If it cannot be found or is closed, ask the user.

### 4. Report

Present the following to the user.

```
client: github.com/topotal/waroom-client-go@<version>
To create N / Needs decision N / Has issue N / Not planned N (Reconsider N, No reason N) / Implemented N

## Unimplemented (no issue) — candidates
| kind | name | client methods | notes |

## Needs decision (operational data, etc.)
| kind | name | client methods | notes |

## Unimplemented (has issue)
| kind | name | issue |

## Not planned (always shown)
| kind | name | issue | closed | reason | reconsider |

## Implemented
| kind | name | file |

## Inconsistencies
(closed as completed but not implemented, registered but the client method no longer exists, etc.)
```

### 5. Create issues

After showing the report, **ask the user which candidates to file before creating anything** (Needs decision items only when explicitly selected). Even when creating in bulk, present the list of titles and get approval first.

Title: `[resource] waroom_runbook` / `[data source] waroom_runbook`

Labels: `resource-candidate` (marks issues managed by this skill; used for data sources too, no separate label), plus `platform-squad`, `software-squad`, `enhancement` (following existing issues). Create without labels that don't exist, and tell the user which ones were missing.

Body template:

```markdown
<!-- client-coverage: kind=<resource|data_source> name=<waroom_xxx> -->

## Summary

Implement the `<waroom_xxx>` <resource|data source>.

- client: `github.com/topotal/waroom-client-go@<version>`
- Client methods: `<Service>.<Method>`, ...
- Parent ID: <if any, e.g. service_uuid>

## Implementation notes

<API constraints the implementer must work around, taken from the report's notes column. Omit this section if there are none. Examples:>
- No `Delete` API: decide whether destroy is a no-op (remove from state only) or resets the value via `Update`, and document it.
- No `Get` API: Read must find the object via `List` (by ID for resources, by name or other filter for data sources); treat "not found in List" as 404.
- Singleton (one per organization / per parent): Create is an upsert, and import uses the parent ID (or a fixed ID) instead of an object ID.
- No `Update` API: mark changeable attributes `RequiresReplace`.

## TODO

- [ ] Scaffold: `task gen:<r|d> name=<xxx>`
- [ ] Register in <Resources|DataSources> in provider.go
- [ ] Implement CRUD (Read for data sources); remove from state on 404 in Read
- [ ] Import (resources only)
- [ ] Unit tests / acceptance tests
- [ ] Add `examples/`
- [ ] Generate docs with `task gen-docs`

## If we decide not to support this

Comment the reason and **Close as not planned**. The client-coverage skill treats it as "Not planned", stops proposing it, and keeps listing it with the reason in every report.
```

Create:

```sh
gh issue create --title '<title>' --body-file <scratchpad>/<name>.md \
  --label resource-candidate --label platform-squad --label software-squad --label enhancement
```

### 6. Link as sub-issues of the parent issue

Only when the tracking issue was confirmed. `sub_issue_id` is the REST `id`, not the issue number.

```sh
child_id=$(gh api repos/{owner}/{repo}/issues/<child-number> -q .id)
gh api -X POST repos/{owner}/{repo}/issues/<parent-number>/sub_issues -F sub_issue_id="$child_id"
```

Finally, report the URLs of the created issues.
