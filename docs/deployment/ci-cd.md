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

## Optional Auto-Deploy

The build workflow only publishes images. To roll them out automatically, add a deploy job that SSHes to the VPS and re-runs `docker stack deploy`.

Store SSH details as repo secrets:

- `VPS_HOST`
- `VPS_USER`
- `VPS_SSH_KEY`

Example:

```yaml
name: Deploy
on:
  workflow_run:
    workflows: ["Build images"]
    types: [completed]
    branches: [main]

jobs:
  deploy:
    if: ${{ github.event.workflow_run.conclusion == 'success' }}
    runs-on: ubuntu-latest
    steps:
      - uses: appleboy/ssh-action@v1
        with:
          host: ${{ secrets.VPS_HOST }}
          username: ${{ secrets.VPS_USER }}
          key: ${{ secrets.VPS_SSH_KEY }}
          script: |
            cd /opt/7spade
            docker stack deploy --with-registry-auth -c stack.yml 7spade
```

Pin `/opt/7spade/stack.yml` to immutable image tags if you need deterministic rollbacks. With `latest`, the deploy pulls whatever the tag currently points to.
