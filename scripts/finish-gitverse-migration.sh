#!/usr/bin/env bash
# Finish GitHub → GitVerse cutover after SSH keys are unlocked.
# Usage:
#   ssh-add --apple-use-keychain ~/.ssh/id_ed25519_gitverse
#   ssh-add --apple-use-keychain ~/.ssh/id_ed25519   # optional, for github backup push
#   ./scripts/finish-gitverse-migration.sh
set -euo pipefail

export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:$PATH"

root="$(git rev-parse --show-toplevel)"
cd "$root"

GV_URL="ssh://git@git.eaisto.at-consulting.ru:32222/tmukhortova/gitverse-notifier.git"
VPS_HOST="${VPS_HOST:-13.140.85.130}"
VPS_USER="${VPS_USER:-katiadeo}"
VPS_KEY="${VPS_KEY:-$HOME/.ssh/gitverse_notifier_github_deploy}"

echo "==> 1. Verify GitVerse SSH"
ssh -T git@git.eaisto.at-consulting.ru 2>&1 | head -5 || true
git ls-remote --heads origin main >/dev/null

echo "==> 2. Push main + tags to GitVerse (origin)"
git push -u origin main
git push origin --tags || true

echo "==> 3. Backup push to GitHub remote (optional)"
if git remote get-url github >/dev/null 2>&1; then
  if git push github main; then
    echo "GitHub backup updated."
  else
    echo "GitHub backup push skipped/failed (non-fatal)."
  fi
fi

tip="$(git rev-parse --short HEAD)"
echo "==> Confirm on GitVerse UI that main tip is $tip"

echo "==> 4. Retarget VPS origin to GitVerse"
if [[ ! -f "$VPS_KEY" ]]; then
  echo "Missing VPS deploy key: $VPS_KEY" >&2
  exit 1
fi
ssh -i "$VPS_KEY" -o BatchMode=yes -o IdentitiesOnly=yes -o ConnectTimeout=20 \
  "${VPS_USER}@${VPS_HOST}" bash -s <<EOF
set -euo pipefail
cd /opt/gitverse-notifier
git remote set-url origin '$GV_URL'
git remote -v
# Prefer fetch over corporate GitVerse; may require VPN/route or a deploy key on GitVerse.
if git fetch origin; then
  git checkout main
  git reset --hard origin/main
  git log -1 --oneline
else
  echo "WARN: git fetch origin failed on VPS — add a deploy key for this server on GitVerse," >&2
  echo "      or ensure 10.125.29.74:32222 is reachable from the VPS." >&2
  exit 2
fi
EOF

echo
echo "==> 5. Manual CI secrets on GitVerse (UI)"
echo "Set secrets: NOTIFIER_CONFIG (paste local config.yml), VPS_HOST=$VPS_HOST,"
echo "             VPS_USER=$VPS_USER, VPS_SSH_KEY=(contents of $VPS_KEY)"
echo "Set variable: ENABLE_VPS_DEPLOY=true"
echo "Then run workflow 'build and deploy' once."
echo
echo "==> 6. After one successful GitVerse deploy, archive GitHub:"
echo "  gh repo archive katiadeo/gitverse-notifier -y"
echo
echo "Done with automated steps."
