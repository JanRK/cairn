# Glossary

**Report**: one thing an agent tells cairn it came across while working: a fact about a skill, a broken tool, a blocked permission, an outage, an idea. The unit cairn collects and the human reviews.
_Avoid_: finding, ticket, issue

**Kind**: the category of a report. One of `field-note`, `tool-failure`, `permission`, `outage`, `idea`, `other`.

**Field note**: a report of kind `field-note`: a skill statement that is wrong, outdated or worth adding. Formerly written straight into skill source; now sent to cairn.

**Subject**: what a report is about, named by a subject type (`skill`, `mcp-server`, `tool`, `agent`, `environment`, `other`) and a free-text name such as `mcp-scrapling`.

**Reporter**: the agent that sent a report, described by its agent name, harness, model and workspace.

**Review**: the human (usually through an agent session) reading reports and deciding what to apply to skills.

**Open report**: a report with status `new` (untriaged) or `accepted` (worth applying, not yet fully handled).

**Handled report**: a report with status `applied`, `rejected` or `duplicate`. Deleted 7 days after being handled; cairn keeps no history.
