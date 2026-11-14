# cairn

Field-notes inbox for AI agents: agents post findings over MCP from anywhere, including containers; you review and keep the good ones.

An agent that finds a wrong skill fact, a broken tool, a blocked permission or an outage files a **report** with cairn instead of editing skill source or forgetting it. Later you (usually through an agent session) review the open reports and apply the good ones. cairn is one container with a SQLite file; it keeps no history, and handled reports are deleted after 7 days.

Two ports:

| Port | Path | Who | Exposure |
|------|------|-----|----------|
| 8080 | `/mcp`, `POST /reports`, `GET /healthz` | agents | reachable by agents |
| 8081 | `/mcp/review`, `GET /healthz` | reviewer | private: localhost or `kubectl port-forward` |

Neither port has auth. Put the agent port behind your network or a gateway, and keep the review port private.

## Quick start

With Docker Compose, from a checkout of this repo:

```sh
docker compose -f deploy/docker-compose.yml up -d
curl -s http://localhost:8080/healthz
```

`ok` means cairn is up. File a test report:

```sh
curl -s -X POST http://localhost:8080/reports \
  -H 'Content-Type: application/json' \
  -d '{"kind":"field-note","summary":"Quick start test","details":"Posted from the README quick start.","subject_type":"skill","subject":"example"}'
```

The reply is `201 Created` with the new report's `id` and `received_at`. A bad report gets `400` with an `errors` list naming each bad field.

Reports live in the `cairn-data` volume. Stop with `docker compose -f deploy/docker-compose.yml down` (add `-v` to delete the data).

## Connecting agents

**MCP.** Point any MCP client at `http://<host>:8080/mcp` (streamable HTTP). It exposes one tool, `submit_report`, whose description tells the agent what to report and what not to.

**HTTP.** `POST /reports` with a JSON body, for scripts, hooks or agents without MCP. Fields:

| Field | Required | Notes |
|-------|----------|-------|
| `kind` | yes | `field-note`, `tool-failure`, `permission`, `outage`, `idea`, `other` |
| `summary` | yes | one line, up to 200 characters |
| `details` | yes | what happened, what was expected, why it matters; up to 16 KiB |
| `subject_type` | no | `skill`, `mcp-server`, `tool`, `agent`, `environment`, `other` |
| `subject` | no | name of the thing, e.g. `mcp-scrapling` |
| `evidence` | no | the few log lines or outputs that matter |
| `suggested_fix` | no | |
| `agent`, `harness`, `model`, `workspace` | no | who is reporting |

Secrets in reports are redacted on intake.

### Snippet for skills

Where a skill used to say "edit this skill's source when you find something wrong", say this instead:

```markdown
If anything in this skill is wrong, outdated or missing, report it to cairn:
call the `submit_report` tool with kind `field-note`, subject_type `skill` and
subject `<this skill's name>`. Keep working; do not edit the skill yourself.
```

## Review

Copy [`skills/cairn-review/`](skills/cairn-review/SKILL.md) into your agent's skills, connect an MCP client to the review port, and ask the agent to review cairn reports.

- Compose: the review port is bound to `127.0.0.1:8081`, so use `http://127.0.0.1:8081/mcp/review` on the host.
- Kubernetes: `kubectl port-forward deploy/cairn 8081:8081`, then the same URL.

Review tools: `list_reports`, `get_report`, `set_status`. Statuses: `new` and `accepted` are open; `applied`, `rejected` and `duplicate` are handled. A handled report is deleted after the retention period (7 days by default); moving it back to open stops the clock.

## Configuration

Environment variables of the container image:

| Variable | Default | Meaning |
|----------|---------|---------|
| `CAIRN_DB_PATH` | `/data/cairn.db` | SQLite database file; mount a volume at `/data` |
| `CAIRN_ADDR` | `:8080` | agent listen address |
| `CAIRN_REVIEW_ADDR` | `:8081` | review listen address |
| `CAIRN_RETENTION` | `168h` | how long handled reports are kept (Go duration) |
| `CAIRN_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

## Examples

- Kubernetes: [`deploy/k8s/cairn.yaml`](deploy/k8s/cairn.yaml)
- LiteLLM MCP gateway: [`docs/examples/litellm.md`](docs/examples/litellm.md)
