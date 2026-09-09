# CI/CD

Image builds run in CI, not on the server. The agreed deployment extends both
[`.github/workflows/build-images.yml`](../../.github/workflows/build-images.yml)
and [`.github/workflows/deploy.yml`](../../.github/workflows/deploy.yml) to build
`api`, `ws`, `web`, `admin-api`, and `admin-web` for GitHub Container Registry.
Verify those changes are included before using this procedure; the
[admin runbook](./admin.md#deployment-contract) defines the required contract,
not confirmation of a live deployment.

Builds run on:

- pushes to `main` matching the workflow's service/frontend paths, including both admin applications in the agreed workflow
- tags matching `v*`
- manual workflow dispatch
- pull requests, build only with no image push

## Required GitHub Configuration

| Type | Name | Purpose |
|---|---|---|
| Repo variable | `VITE_API_URL` | Baked into the web image |
| Repo variable | `VITE_WS_URL` | Baked into the web image |
| Repo variable | `VITE_WS_HEALTH_URL` | Baked into the web image |
| Repo variable | `VITE_SKIN_ASSETS_URL` | Skin CDN/bucket prefix baked into the web image |
| Setting | Workflow permissions | Read and write, so the job can push to GHCR |

No custom secret is needed for the build. The workflow authenticates to GHCR with the built-in `GITHUB_TOKEN`.

Both image workflows pass all four `VITE_*` variables to the web Docker build.
Changing one requires rebuilding and deploying the web image because Vite embeds
these values in the static bundle.

Both workflows must also pass `VITE_ADMIN_API_URL=/admin-api` to the admin-web
build. This same-origin value is compiled in, not a production secret or a
server runtime variable. The admin API image must include `/bootstrap-admin`.
The admin-web CSP asset origin is a separate runtime setting in `admin-web.env`.

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

1. Builds all five images and pushes them to GHCR with two tags: the literal release tag (`vX.Y.Z`) and `latest`.
2. SSHes to the Swarm manager and preflights existence of **all five** services before any image update. Missing services fail the deploy without updating even the player API.
3. Runs `docker service update --image ghcr.io/<owner>/7spade/<service>:vX.Y.Z` in order: `7spade_api`, `7spade_ws`, `7spade_web`, `7spade_admin-api`, `7spade_admin-web`, waiting for each update to converge.

The API goes first because it owns migrations for the shared player/admin
schema. Service existence and convergence are not schema readiness checks;
verify API migration completion and the [admin smoke checklist](./admin.md#smoke-checklist).
A later rollout failure can leave mixed image versions: the five updates are
not atomic, and rolling back images does not roll back the schema.

**First provisioning is manual.** Create both admin services, all runtime env
files, DNS/TLS/proxy/firewall policy, schema, and the first administrator using
[Admin Deployment](./admin.md) before publishing an auto-deployed release. Image
updates cannot create missing services or install that configuration. They also
do not reread env files; use an explicitly tagged stack deploy for those changes.

The agreed stack uses `${IMAGE_TAG:-latest}` for all five application images.
The workflow updates images to the release tag but does not persist a server
`IMAGE_TAG` setting. Record/export the deployed tag before future manual stack
deploys so an env-only change does not unexpectedly switch images to `latest`.

The Swarm services are named after the stack (`docker stack deploy ... 7spade`), so service names are `7spade_<service>`. Images are public on GHCR, so the manager needs no `docker login` and the workflow does not pass `--with-registry-auth`.

### Required GitHub Configuration

In addition to the build-images variables above, create a `production` environment with these secrets:

| Type | Name | Purpose |
|---|---|---|
| Environment | `production` | Gates the build + deploy jobs |
| Environment secret | `VPS_HOST` | SSH target hostname |
| Environment secret | `VPS_PORT` | SSH port used by the release workflow |
| Environment secret | `VPS_USER` | SSH username |
| Environment secret | `VPS_SSH_KEY` | SSH private key |

Define the `VITE_*` values as repository variables because the ordinary
`build-images.yml` workflow does not select the `production` environment. The
release workflow can also read those repository variables while its secrets
remain scoped to `production`.

### Cutting a Release

```bash
tag="v$(tr -d '[:space:]' < VERSION)"
git tag "$tag"
git push origin "$tag"
gh release create "$tag" --generate-notes
```

The tag must exactly equal the root `VERSION` with a `v` prefix; the workflow
rejects mismatches. The release-create step triggers `deploy.yml`. Watch the run
in the Actions tab; the deploy step blocks until each `service update` converges.
