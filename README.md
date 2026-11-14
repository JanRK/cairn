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
- [ ] GHCR package `cairn`: set visibility to public once the repo is public.
