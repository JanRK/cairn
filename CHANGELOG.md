# Changelog

## 1.0.0 (2026-11-14)


### Features

* accept reports over POST /reports with validation, redaction and SQLite storage ([7e609dd](https://github.com/JanRK/cairn/commit/7e609dd55e8cd387c52d6533d7469996938217b5))
* add k8s and LiteLLM example setups with kubeconform CI ([557ba98](https://github.com/JanRK/cairn/commit/557ba982918e8dc7e0286f5106b5325f57f3dc11))
* delete handled reports after retention window ([eeb2421](https://github.com/JanRK/cairn/commit/eeb242121393903b9672c6e76556795d2cf4cfd5))
* serve review MCP tools on /mcp/review on the review listener ([5eaa056](https://github.com/JanRK/cairn/commit/5eaa05629b0237bcc4b473832f2fa8df829bb027))
* serve submit_report over the agent MCP endpoint ([7ba50b5](https://github.com/JanRK/cairn/commit/7ba50b56c6d9301800170bdbcb406f3b68e5711f))


### Bug Fixes

* **deps:** bump actions/checkout from 6.1.0 to 7.0.1 ([#2](https://github.com/JanRK/cairn/issues/2)) ([52f81b2](https://github.com/JanRK/cairn/commit/52f81b292f8c07ef64d517e677905ce60407b7cf))
* **deps:** bump actions/setup-go from 6.5.0 to 7.0.0 ([67f9a63](https://github.com/JanRK/cairn/commit/67f9a63b5f4e2d15070ed0803bbe91e72c7828d2))
* **deps:** bump actions/setup-go from 6.5.0 to 7.0.0 ([2c8124a](https://github.com/JanRK/cairn/commit/2c8124aec461fe332d2bddeac522399f68272782))
* log review MCP protocol errors like the agent endpoint ([c1912cd](https://github.com/JanRK/cairn/commit/c1912cd220eedaf50fca30d68829c0895010baef))
* reject trailing data after the report JSON ([6c7e029](https://github.com/JanRK/cairn/commit/6c7e0290dadc44eca5f27390ca396fe4a43d8bb9))
