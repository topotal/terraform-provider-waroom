---
name: implement-candidate
description: Implement a Terraform resource / data source from a `resource-candidate` issue created by the client-coverage skill — scaffold, CRUD, import, examples, docs, unit tests and acceptance (E2E) tests. Use when asked to implement a resource-candidate issue, "implement #34", "waroom_runbook を実装して", or "implement-candidate".
argument-hint: "<issue-number>"
arguments: [issue]
---

# implement-candidate

Implement the resource / data source described by issue #$issue, following the patterns in `internal/provider/`.

Read `${CLAUDE_SKILL_DIR}/../client-coverage/SKILL.md` for the issue marker format. Always refer to skill files through `${CLAUDE_SKILL_DIR}`: the work happens in a worktree checked out from `origin/main`, which may not contain these skill files. Invoking this skill authorizes the commit, push, and PR in step 10; do not do any other outward-facing action (closing the issue, merging) unless asked.

## 0. Preflight

```sh
gh issue view $issue --json number,title,state,labels,body,url
```

- The issue must be open, have the `resource-candidate` label, and contain the marker `<!-- client-coverage: kind=<kind> name=<name> -->`. Otherwise stop and tell the user.
- From the marker, derive:
  - `kind`: `resource` or `data_source`
  - `name`: Terraform type name, e.g. `waroom_runbook`
  - `snake`: `name` without the `waroom_` prefix, e.g. `runbook`
  - `Pascal`: e.g. `Runbook`
- Check that it is not already implemented: `internal/provider/<kind>_<snake>.go` must not exist and no `TypeName` ends with `_<snake>` for the same kind.
- Work in a dedicated git worktree so the user's current checkout (its branch, uncommitted changes, `go.work`) is left untouched:
  ```sh
  git fetch origin main
  git worktree list                       # reuse it if .claude/worktrees/<kind>-<snake> already exists
  git worktree add -b feat/<kind>-<snake> .claude/worktrees/<kind>-<snake> origin/main
  cd .claude/worktrees/<kind>-<snake>
  ```
  - If the branch already exists without a worktree, use `git worktree add .claude/worktrees/<kind>-<snake> feat/<kind>-<snake>` instead.
  - Make sure the worktrees do not show up as untracked files in the main checkout: if `git check-ignore -q .claude/worktrees/x` fails, append `.claude/worktrees/` to `$(git rev-parse --git-common-dir)/info/exclude` (local only; do not edit `.gitignore` in this PR).
  - All later steps run inside the worktree. It has no `go.work` (gitignored), so builds use the client version pinned in go.mod, the same as CI.
- Confirm the client version in the issue's `client:` line matches the version pinned in go.mod. If not, re-check the client methods in step 2 and mention the difference to the user.

### Waroom connectivity

Acceptance tests (step 7) need a running Waroom that the API key can access. Check it up front so a missing server is found before the implementation, not at the end:

```sh
GOWORK=off go -C "${CLAUDE_SKILL_DIR}/scripts/ping" run .
```

It calls the read-only list-users API once with `E2E_WAROOM_API_ENDPOINT` / `E2E_WAROOM_API_KEY`, the same variables `task test:acc` uses. `E2E_WAROOM_API_ENDPOINT` is the host only (e.g. `http://localhost:3000`); the client adds `/api/v0`.

- `OK` → ask once, now, whether acceptance tests may be run against this endpoint (they create and delete real objects). Ask here rather than in step 7 so a `/goal` run is not blocked midway. Then continue.
- `NG` → show the message and ask the user to either start Waroom / fix the env vars and rerun the check, or continue without running acceptance tests (they are still written in step 7). Do not start Waroom yourself.

Never print the API key.

## 1. Print the goal condition

Before implementing, print a `/goal` command tailored to this issue for the user to paste (a skill cannot set a goal itself). The goal covers steps 2–7 only, ending when the acceptance tests pass; examples, docs, and the PR (steps 8–10) come after the goal is met. Keep it verifiable from the transcript:

```
/goal <kind> <name> from issue #<issue> is implemented and its acceptance tests pass: internal/provider/<kind>_<snake>.go and internal/provider/<kind>_<snake>_test.go exist, it is registered in provider.go, `task build`, `task test` and `task lint` have each been shown exiting 0 after the last code change, and `task test:acc -- -run 'TestAcc<Pascal>' ./internal/provider/` has been shown passing (not skipped) after the last code change. Stop after 40 turns.
```

Print it only when the connectivity check was `OK` and the user agreed to run acceptance tests (step 0). Otherwise the goal cannot be met; print the same goal without the `task test:acc` clause and say the acceptance tests will be written but not run.

Then continue without waiting.

## 2. Read the API

```sh
"${CLAUDE_SKILL_DIR}/../client-coverage/scripts/list-client-api.sh"
```

(Run it from inside the worktree; it inspects the repository of the current directory.)

- Read the issue's "Client methods" and "Implementation notes".
- In the `source` directory the script prints, read `waroom/<service>.go` and the related types in `waroom/types.go` (request/response structs, params, `Opt*` types) for every method used. Follow `gen.*` aliases into `internal/ogen` when field details are needed.
- Decide the schema from the response type: which attributes are Required / Optional / Computed, which identify the parent (Required + `RequiresReplace`), and which cannot be updated (`RequiresReplace`).
- If the schema has a real design choice (e.g. how a missing `Delete` should behave, which field a data source looks up by), state your choice and its reason to the user before writing code. Ask only when the issue's notes and the API leave it genuinely open.

## 3. Scaffold

- resource: `task gen:r name=<snake>` (the template has no import support; add it in step 4).
- data_source: `cmd/tfgen` supports resources only. Create `internal/provider/data_source_<snake>.go` by hand, modeled on `data_source_service_label.go` (type `<Pascal>Source`, constructor `New<Pascal>Source`).

Register the constructor in `Resources()` / `DataSources()` in `internal/provider/provider.go`.

## 4. Implement

Follow `resource_service_label.go` / `data_source_service_label.go`:

- Interface assertions at the top (`resource.Resource`, `ResourceWithConfigure`, `ResourceWithImportState`).
- `Configure` casts `req.ProviderData` to `*waroom.Client` with the same error message.
- Every attribute has a `MarkdownDescription` (these become the docs). The resource description starts with "Manages a Waroom ...", the data source with "Retrieves information about a Waroom ...".
- `id` is a Computed alias of `uuid`; both use `stringplanmodifier.UseStateForUnknown()`.
- Validators from `stringvalidator` / `internal/validator` where the API constrains values.
- Errors: `resp.Diagnostics.AddError("Client Error", fmt.Sprintf("<Operation>, got error: %s", err))`.
- 404 in Read: remove from state instead of erroring.
  ```go
  var apiErr *waroom.APIError
  if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
      resp.State.RemoveResource(ctx)
      return
  }
  ```
  When Read uses `List` (no `Get` API), "not found in the list" is the 404.
- `tflog.Info` after Create / Update with the identifying fields.
- Import:
  - top-level object: `resource.ImportStatePassthroughID` on `uuid`, and set `id` too
  - under a parent: `<parent>,<uuid>` like `ImportState` in `resource_service_label.go`
  - singleton: the parent ID (or a fixed ID) and document it
  - Write the format in the method's doc comment as `resource_service_label.go` does.
- Apply each item in the issue's "Implementation notes" (no Delete / no Get / singleton / no Update).

## 5. Unit tests

Add unit tests for pure logic you introduced (import ID parsing, model mapping, validators) as `*_test.go` next to the code, in the style of `internal/validator/*_test.go`. Skip if there is no such logic.

## 6. Build, test, lint

```sh
task fmt
task build
task test
task lint
```

Fix everything until all four pass. `task test` also compiles the acceptance tests from step 7 once they exist (they are skipped without `TF_ACC`), so rerun it after step 7.

## 7. Acceptance (E2E) tests

Create `internal/provider/<kind>_<snake>_test.go` using `github.com/hashicorp/terraform-plugin-testing`. There is no earlier acceptance test in this repo yet; these are the conventions:

- Function name `TestAcc<Pascal>Resource` / `TestAcc<Pascal>DataSource`.
- `resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, ...})`.
- Unique names: `acctest.RandomWithPrefix("tf-acc")` from `github.com/hashicorp/terraform-plugin-testing/helper/acctest`.
- Assertions with `ConfigStateChecks` + `statecheck.ExpectKnownValue` / `knownvalue`, not legacy `Check` functions.
- Resource test steps:
  1. Create and check attributes.
  2. `ImportState: true, ImportStateVerify: true` (with `ImportStateIdFunc` when the import ID is composite).
  3. Update every updatable attribute and check it (skip if the API has no Update).
  4. Destroy is implicit; add `CheckDestroy` that calls the client and expects 404 / absence.
- Data source test: create the object with the matching resource in the same config if that resource exists; otherwise read an existing object provided via an env var.
- Prerequisites that cannot be created through the API (e.g. a Waroom service, since there is no create-service API) come from env vars named `E2E_WAROOM_<THING>` (e.g. `E2E_WAROOM_SERVICE_NAME`). Read them in the test and `t.Skip` with a clear message when unset. Add each new variable to the `test:acc` task in `Taskfile.yaml` (mapped like `E2E_WAROOM_API_KEY`) and to DEVELOPMENT.md.
- Configs are built with `fmt.Sprintf` helpers named `testAcc<Pascal><Kind>Config(...)`.

Running them creates real objects in Waroom. Run only if the user agreed in step 0, and rerun the connectivity check first (it must be `OK`):

```sh
task test:acc -- -run 'TestAcc<Pascal>' ./internal/provider/
```

If the check is `NG` or the user declines, say the acceptance tests were written but not run.

## 8. Examples and docs

- `examples/resources/<name>/resource.tf` (+ `import.sh` for resources), or `examples/data-sources/<name>/data-source.tf`, in the style of the `waroom_service_label` examples.
- `task gen-docs` to regenerate `docs/` (also runs `terraform fmt` on examples). Check the generated page reads well.

## 9. Report

Report to the user:

- Files added / changed
- Schema summary (attributes and Required / Optional / Computed)
- Decisions made for the issue's implementation notes
- Results of `task build` / `task test` / `task lint` and whether acceptance tests ran (with output if they failed)
- New `E2E_WAROOM_*` env vars, if any
- Unchecked items in the issue's TODO, if any

## 10. Commit, push, PR

Do this only when `task build` / `task test` / `task lint` all pass. If any still fails, stop at the report and ask.

- Stage explicit paths only: the files this skill added or changed (provider code, tests, `provider.go`, examples, `docs/`, `Taskfile.yaml` / DEVELOPMENT.md if changed). Never `git add -A` / `git add .`, and never stage untracked files this skill did not create (e.g. `temp/`).
- Commit message (Conventional Commits, as in this repo's history):
  ```
  feat: add <name> <resource|data source>

  Closes #<issue>
  ```
- `git push -u origin <branch>`
- Open the PR against `main`:
  ```sh
  gh pr create --base main --title 'feat: add <name> <resource|data source>' --body-file <scratchpad>/pr-<snake>.md
  ```
  Open it as a draft (`--draft`) when the acceptance tests were not run or did not pass; otherwise ready for review.
- PR body:
  ```markdown
  Closes #<issue>

  ## Summary
  <schema summary and how the implementation notes were handled>

  ## Testing
  - task build / test / lint: pass
  - Acceptance tests: <ran against <endpoint host> and passed | not run: <reason>>

  ## Notes
  <new E2E_WAROOM_* env vars, open questions for reviewers>
  ```
  End the body with the attribution lines the session asks for, if any.

Finally, give the user the PR URL and the worktree path. Leave the issue's checklist and state to the PR (it closes the issue on merge). Keep the worktree for review follow-ups; tell the user it can be removed after merge with `git worktree remove .claude/worktrees/<kind>-<snake>`.
