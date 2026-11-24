# Claude Code Context

This file provides context for Claude Code when working with this repository.
Project context is maintained in [llms.txt](llms.txt) and imported below.

@llms.txt

## Instructions for LLM Assistants

1. Follow the HashiCorp Terraform Plugin Framework documentation linked in llms.txt
2. Review existing code in `internal/provider/` and `examples/` for implementation patterns
3. Ensure all implementations comply with Terraform provider best practices

## Reporting Bugs and Checking API Behavior

[waroom-client-go](https://github.com/topotal/waroom-client-go) is the entry point for bug reports and API behavior questions across the Waroom ecosystem. When a bug or unexpected behavior may come from the API or the client rather than from this repository's own code (an unexpected status code or error message, a missing or wrong field, a mismatch with the spec, behavior you need to confirm before implementing):

1. **Search existing issues first** in waroom-client-go and [waroom-backend](https://github.com/topotal/waroom-backend), including closed ones:
   `gh search issues --repo topotal/waroom-client-go --repo topotal/waroom-backend "<keywords>"`.
   If one already covers it, link it instead of filing a duplicate.
2. **File new issues in waroom-client-go**, not in waroom-backend or [waroom](https://github.com/topotal/waroom), even when the cause looks like the backend. Include the Terraform configuration, the error, expected vs actual behavior, the request and response if you can capture them (the provider does not dump API traffic; reproduce with `waroomctl --debug` or the client's `WithRequestDump()` / `WithResponseDump()`), and the provider version and the waroom-client-go version pinned in `go.mod`. Use placeholders for tenant data and never paste API keys.
3. **Leave the upstream decision to waroom-client-go.** Whether a waroom-backend or waroom issue is needed is decided from the waroom-client-go issue; do not file there directly from this repository.

Bugs that are clearly in this repository's own code (schema, plan/state handling, import) are filed here as usual.
