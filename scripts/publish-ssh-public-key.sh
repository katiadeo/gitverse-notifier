#!/usr/bin/env bash
set -euo pipefail

key="${1:-$HOME/.ssh/id_ed25519_gitverse.pub}"
if [[ ! -f "$key" ]]; then
  echo "Нет файла: $key" >&2
  exit 1
fi
/usr/bin/pbcopy < "$key"
echo "Скопировано в буфер: $key"
/usr/bin/open "https://git.eaisto.at-consulting.ru/user/settings/keys" 2>/dev/null || \
  echo "Откройте вручную: https://git.eaisto.at-consulting.ru/user/settings/keys"
echo
cat "$key"
