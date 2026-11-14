# Example: cairn behind a LiteLLM MCP gateway

> **Example only.** cairn does not need LiteLLM. Any MCP client or gateway can use `http://<host>:8080/mcp` directly. This is one setup, checked against the LiteLLM docs on 2026-11-14 ([MCP](https://docs.litellm.ai/docs/mcp), [MCP permissions](https://docs.litellm.ai/docs/mcp_control)).

Register cairn in the LiteLLM proxy `config.yaml`:

```yaml
mcp_servers:
  cairn:
    url: "http://cairn:8080/mcp"   # agent port only; never the review port 8081
    transport: "http"              # streamable HTTP
    server_id: "cairn"             # stable id, so key/team grants survive URL changes
    description: "Report problems with tools, docs, or the environment"
    allowed_tools: ["submit_report"]

general_settings:
  # Keys reach an MCP server only when it is granted to them explicitly.
  require_key_mcp_access_defined: true
```

Then grant the `cairn` server to the agent keys or teams that should report.

What agents see:

- LiteLLM prefixes tool names with the server alias, so agents call **`cairn-submit_report`**, not `submit_report`.
- `allowed_tools` is a catalog allowlist for every caller; other tools cairn may add are hidden.
- Agents authenticate to the gateway with `x-litellm-api-key: Bearer <key>`. cairn itself has no auth, so the gateway key is the only check.

The review side (port 8081, `/mcp/review`) stays off the gateway. Reach it directly, for example with `kubectl port-forward` (see `deploy/k8s/`).
