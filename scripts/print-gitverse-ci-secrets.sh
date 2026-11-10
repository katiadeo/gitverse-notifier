#!/usr/bin/env bash
# Print values to paste into GitVerse → CI/CD (Secrets / Variables).
# Does not upload anything; run locally only.
set -euo pipefail

export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:$PATH"

root="$(git rev-parse --show-toplevel)"
file="$root/config.yml"
vps_key="${VPS_KEY:-$HOME/.ssh/gitverse_notifier_github_deploy}"

: "${VPS_HOST:=13.140.85.130}"
: "${VPS_USER:=katiadeo}"

if [[ ! -f "$file" ]]; then
  echo "Missing $file" >&2
  exit 1
fi
if [[ ! -f "$vps_key" ]]; then
  echo "Missing $vps_key" >&2
  exit 1
fi

echo "GitVerse CI — скопируйте в UI репозитория:"
echo
echo "Secret NOTIFIER_CONFIG = (весь файл config.yml):"
echo "----- begin config.yml -----"
cat "$file"
echo "----- end config.yml -----"
echo
echo "Secret VPS_HOST = $VPS_HOST"
echo "Secret VPS_USER = $VPS_USER"
echo "Secret VPS_SSH_KEY = (приватный ключ ниже):"
echo "----- begin VPS_SSH_KEY -----"
cat "$vps_key"
echo "----- end VPS_SSH_KEY -----"
echo
echo "Variable ENABLE_VPS_DEPLOY = true"
echo
echo "Deploy key репозитория (VPS git fetch): добавьте в GitVerse → Settings repo → Deploy keys:"
echo "----- VPS public key (read-only) -----"
# shellcheck disable=SC2029
ssh -i "$vps_key" -o BatchMode=yes -o IdentitiesOnly=yes -o ConnectTimeout=15 \
  "${VPS_USER}@${VPS_HOST}" 'cat ~/.ssh/id_ed25519.pub' 2>/dev/null || \
  echo "(не удалось снять с VPS — зайдите на сервер и: cat ~/.ssh/id_ed25519.pub)"
