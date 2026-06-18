# CI/CD

Image builds run in CI, not on the server. The repo ships [`.github/workflows/build-images.yml`](../../.github/workflows/build-images.yml), which builds the `api`, `ws`, and `web` images and publishes them to GitHub Container Registry.

Builds run on:

- pushes to `main`
- tags matching `v*`
- manual workflow dispatch
- pull requests, build only with no image push

## Required GitHub Configuration

| Type | Name | Purpose |
|---|---|---|
| Repo variable | `VITE_API_URL` | Baked into the web image |
| Repo variable | `VITE_WS_URL` | Baked into the web image |
| Repo variable | `VITE_WS_HEALTH_URL` | Baked into the web image |
| Setting | Workflow permissions | Read and write, so the job can push to GHCR |

No custom secret is needed for the build. The workflow authenticates to GHCR with the built-in `GITHUB_TOKEN`.

## Pulling Private Images

By default, GitHub packages may be private. The VPS has two options:

1. Make the packages public. The server then needs no registry login.
2. Keep them private and log in with a classic PAT that has only `read:packages`:

```bash
echo "<GHCR_READ_TOKEN>" | docker login ghcr.io -u <github-user> --password-stdin
```

Deploy with `--with-registry-auth` so Swarm forwards the credentials to each node's image pull.

## Auto-Deploy on Release

The repo ships [`.github/workflows/deploy.yml`](../../.github/workflows/deploy.yml), which runs on **published GitHub releases**. Cutting a release tagged `vX.Y.Z`:

1. Builds the three images and pushes them to GHCR with two tags: the literal release tag (`vX.Y.Z`) and `latest`.
2. SSHes to the Swarm manager and runs `docker service update --image ghcr.io/<owner>/7spade/<service>:vX.Y.Z` for `7spade_api`, `7spade_ws`, `7spade_web`.

The Swarm services are named after the stack (`docker stack deploy ... 7spade`), so service names are `7spade_<service>`. Images are public on GHCR, so the manager needs no `docker login` and the workflow does not pass `--with-registry-auth`.

### Required GitHub Configuration

In addition to the build-images variables above, create a `production` environment with these secrets:

| Type | Name | Purpose |
|---|---|---|
| Environment | `production` | Gates the build + deploy jobs |
| Environment secret | `VPS_HOST` | SSH target hostname |
| Environment secret | `VPS_USER` | SSH username |
| Environment secret | `VPS_SSH_KEY` | SSH private key |

The `VITE_*` repository variables are read by both workflows and may be defined at the repo level or scoped to the `production` environment.

### Cutting a Release

```bash
git tag v1.2.3
git push origin v1.2.3
gh release create v1.2.3 --generate-notes
```

The release-create step triggers `deploy.yml`. Watch the run in the Actions tab; the deploy step blocks until each `service update` converges.
