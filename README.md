# cairn
Field-notes inbox for AI agents: agents post findings over MCP from anywhere, including containers; you review and keep the good ones.

## Releases

Merging the release-please PR on `main` tags `vX.Y.Z`, publishes a GitHub Release with `CHANGELOG.md` and binaries, and pushes `ghcr.io/janrk/cairn:vX.Y.Z` and `:latest` (linux/amd64, linux/arm64).

```sh
docker run --rm ghcr.io/janrk/cairn:latest --version
```

### Owner setup checklist

- [ ] Branch protection on `main`: require the `ci` status check, allow auto-merge, allow squash merges.
- [ ] Settings → Actions → General: allow GitHub Actions to create and approve pull requests (release-please).
- [ ] Create a fine-grained PAT (or GitHub App token) for this repo with Contents and Pull requests read/write, and store it as the Actions secret `RELEASE_PLEASE_TOKEN`. Without it, release-please falls back to `GITHUB_TOKEN`, whose PRs do not trigger `ci`; trigger it by hand (for example close and reopen the release PR).
- [ ] Keep immutable releases off: goreleaser uploads binaries to the release after release-please has published it.
- [ ] GHCR package `cairn`: set visibility to public once the repo is public.
