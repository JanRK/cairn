---
name: cairn-review
description: Triage the reports agents filed with cairn. Use when the user wants to review cairn reports, go through the cairn inbox, or apply field notes to skills.
---

Review is a loop over open reports: read each one, decide, and for accepted ones edit the skill (or tool, or setup) it names. Reports you mark handled are deleted 7 days later, and cairn keeps no history, so a decision is final once the week is out.

## Connect

The review tools live on a separate port that is never exposed to agents: `http://<host>:8081/mcp/review` (streamable HTTP MCP, no auth).

- Docker compose: the port is bound to `127.0.0.1:8081`, so use `http://127.0.0.1:8081/mcp/review` on the host.
- Kubernetes: `kubectl port-forward deploy/cairn 8081:8081`, then the same URL.

Tools: `list_reports`, `get_report`, `set_status`. If they are missing, the MCP client is not configured for that URL; ask the user to add it rather than guessing another endpoint.

## Triage loop

1. `list_reports` with `status: "new"`. Follow `next` until it is empty. Done when every `new` report has been read.
2. For each report, `get_report` and decide:
   - **accepted**: true and worth acting on.
   - **rejected**: wrong, noise, or not worth acting on. Put the reason in `note`.
   - **duplicate**: same finding as an earlier report. Set `duplicate_of` to the original's id.
   Batch ids with the same outcome into one `set_status` call.
3. `list_reports` with `status: "accepted"`. For each, make the change in the named subject (for a field note: edit that skill's source), confirm with the user where the change is not obvious, then `set_status` to `applied` with a `note` saying what changed. Done when no `accepted` report remains, or the user has chosen to leave some for later.
4. Tell the user: how many reports were accepted, applied, rejected and marked duplicate, and which ones are still open.

## Statuses

Open: `new`, `accepted`. Handled: `applied`, `rejected`, `duplicate`. Setting a handled status starts the 7-day deletion clock; setting an open status again stops it. Leave a report `accepted` when it is right but not yet applied, so it stays.
