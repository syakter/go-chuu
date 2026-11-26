# Auto-deploy

Every merge into `master` runs `.github/workflows/deploy.yml`:

1. **test**: `go vet` and `go test` on a GitHub-hosted runner.
2. **deploy**: on the self-hosted runner in the Fedora LXC, builds with `make build`, then runs
   `/usr/local/sbin/go-chuu-deploy`. That script installs the binary to `/opt/go-chuu/go-chuu`
   (keeping the previous one as `go-chuu-old`) and restarts `go-chuu.service`. If the service
   is not up 15 seconds later, it rolls back.

The LXC connects out to GitHub, so it needs no open ports. You can also start a deploy by hand
from the Actions tab (**Deploy → Run workflow**).

## One-time setup (on the LXC)

```bash
cd ~/Projects/go-chuu
sudo deploy/setup-runner.sh "$(gh api -X POST repos/syakter/go-chuu/actions/runners/registration-token -q .token)"
```

This sets up:

- a `ghrunner` user that runs the GitHub runner as a systemd service
- `/usr/local/sbin/go-chuu-deploy`, plus a sudoers rule that lets `ghrunner` run only that script

## Public repo safeguard

A self-hosted runner on a public repo would run code from a fork's pull request if that PR adds
a workflow which targets the runner. Require approval before any outside contributor's workflows run:

```bash
gh api -X PUT repos/syakter/go-chuu/actions/permissions/fork-pr-contributor-approval \
  -f approval_policy=all_external_contributors
```

Never approve workflow runs on fork PRs that touch `.github/workflows/`.

## Updating the deploy script

Edit `deploy/go-chuu-deploy`, then reinstall it:

```bash
sudo install -o root -g root -m 0755 deploy/go-chuu-deploy /usr/local/sbin/go-chuu-deploy
```
