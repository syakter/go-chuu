#!/usr/bin/env bash
# One-time setup of the self-hosted GitHub Actions runner on the Fedora LXC.
#
# Run from the repo root as root, passing a runner registration token:
#   sudo deploy/setup-runner.sh "$(gh api -X POST repos/syakter/go-chuu/actions/runners/registration-token -q .token)"
set -euo pipefail

TOKEN="${1:?usage: setup-runner.sh <registration-token>}"
REPO_URL="https://github.com/syakter/go-chuu"
RUNNER_USER="ghrunner"
RUNNER_DIR="/home/$RUNNER_USER/actions-runner"
LABELS="go-chuu-deploy"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

[[ $EUID -eq 0 ]] || { echo "run as root (sudo)" >&2; exit 1; }

# 1. Dedicated unprivileged user for the runner.
if ! id "$RUNNER_USER" &>/dev/null; then
    useradd --create-home --shell /bin/bash "$RUNNER_USER"
fi

# 2. Deploy script + a sudoers rule that allows only that script.
install -o root -g root -m 0755 "$SCRIPT_DIR/go-chuu-deploy" /usr/local/sbin/go-chuu-deploy
sudoers=/etc/sudoers.d/go-chuu-deploy
echo "$RUNNER_USER ALL=(root) NOPASSWD: /usr/local/sbin/go-chuu-deploy" >"$sudoers.tmp"
chmod 0440 "$sudoers.tmp"
visudo -cf "$sudoers.tmp"
mv "$sudoers.tmp" "$sudoers"

# 3. Download the latest runner release.
if [[ ! -x "$RUNNER_DIR/config.sh" ]]; then
    version="$(curl -fsSL https://api.github.com/repos/actions/runner/releases/latest | jq -r .tag_name | sed 's/^v//')"
    case "$(uname -m)" in
        x86_64) arch=x64 ;;
        aarch64) arch=arm64 ;;
        *) echo "unsupported arch $(uname -m)" >&2; exit 1 ;;
    esac
    tarball="actions-runner-linux-${arch}-${version}.tar.gz"
    runuser -u "$RUNNER_USER" -- mkdir -p "$RUNNER_DIR"
    runuser -u "$RUNNER_USER" -- bash -c "cd '$RUNNER_DIR' && curl -fsSLO 'https://github.com/actions/runner/releases/download/v${version}/${tarball}' && tar xzf '$tarball' && rm '$tarball'"
    "$RUNNER_DIR/bin/installdependencies.sh"
fi

# 4. Register with the repo and install as a systemd service.
runuser -u "$RUNNER_USER" -- bash -c "cd '$RUNNER_DIR' && ./config.sh --unattended --replace \
    --url '$REPO_URL' --token '$TOKEN' \
    --name '$(hostname)-go-chuu' --labels '$LABELS'"

cd "$RUNNER_DIR"
./svc.sh install "$RUNNER_USER"
./svc.sh start

echo "Runner installed. Check it at $REPO_URL/settings/actions/runners"
