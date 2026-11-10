#!/usr/bin/env bash
set -euo pipefail

export PATH="/opt/homebrew/bin:/usr/local/bin:$PATH"

root="$(git rev-parse --show-toplevel)"
file="$root/config.yml"
stamp="$root/.git/gitverse-notifier-config.sha256"

if_changed=0
do_deploy=0
for arg in "$@"; do
  case "$arg" in
    --if-changed) if_changed=1 ;;
    --deploy) do_deploy=1 ;;
    -h|--help)
      echo "usage: $0 [--if-changed] [--deploy]"
      echo "  uploads local config.yml to CI secret NOTIFIER_CONFIG when origin is GitHub"
      echo "  for GitVerse origin: prints manual UI instructions (does not block push)"
      echo "  --if-changed  skip if the file has not changed since last upload"
      echo "  --deploy      also trigger the VPS deploy workflow (GitHub only)"
      exit 0
      ;;
    *)
      echo "unknown argument: $arg" >&2
      exit 1
      ;;
  esac
done

current_hash() {
  git hash-object -- "$file"
}

config_changed() {
  [[ -f "$file" ]] || return 1
  local now last=""
  now="$(current_hash)"
  if [[ -f "$stamp" ]]; then
    last="$(tr -d '[:space:]' < "$stamp")"
  fi
  [[ "$now" != "$last" ]]
}

origin_kind() {
  local url
  url="$(git remote get-url origin 2>/dev/null || true)"
  url="${url%.git}"
  case "$url" in
    git@github.com:*|ssh://git@github.com/*|https://github.com/*)
      echo github
      ;;
    *git.eaisto.at-consulting.ru*|*gitverse*)
      echo gitverse
      ;;
    "")
      echo missing
      ;;
    *)
      echo other
      ;;
  esac
}

github_repo() {
  local url
  url="$(git remote get-url origin 2>/dev/null || true)"
  url="${url%.git}"
  case "$url" in
    git@github.com:*) echo "${url#git@github.com:}" ;;
    ssh://git@github.com/*) echo "${url#ssh://git@github.com/}" ;;
    https://github.com/*) echo "${url#https://github.com/}" ;;
    *)
      echo "remote origin не указывает на GitHub: ${url:-<пусто>}" >&2
      return 1
      ;;
  esac
}

remind_gitverse_manual() {
  local url
  url="$(git remote get-url origin 2>/dev/null || true)"
  echo "origin указывает на GitVerse (${url:-<пусто>}), автоматическая заливка секрета через gh недоступна." >&2
  echo "Обновите CI secret NOTIFIER_CONFIG вручную в UI репозитория (вставьте содержимое локального config.yml)." >&2
  echo "Также нужны secrets VPS_HOST, VPS_USER, VPS_SSH_KEY и variable ENABLE_VPS_DEPLOY=true." >&2
  if [[ "$do_deploy" -eq 1 ]]; then
    echo "Запустите workflow build and deploy вручную в UI GitVerse после обновления секрета." >&2
  fi
}

if [[ ! -f "$file" ]]; then
  if [[ "$if_changed" -eq 1 ]]; then
    exit 0
  fi
  echo "Нет $file — скопируйте config.yml.example в config.yml и заполните секреты." >&2
  exit 1
fi

if [[ "$if_changed" -eq 1 ]] && ! config_changed; then
  echo "config.yml не менялся с прошлой синхронизации."
  exit 0
fi

if grep -Eq '^GITVERSE_API_URL:[[:space:]]*$' "$file" || grep -Eq '^TELEGRAM_BOT_TOKEN:[[:space:]]*$' "$file"; then
  echo "config.yml выглядит пустым. Заполните секреты перед синхронизацией." >&2
  exit 1
fi

kind="$(origin_kind)"
case "$kind" in
  gitverse|other|missing)
    remind_gitverse_manual
    # Never block pre-push / make sync-config when origin is not GitHub.
    exit 0
    ;;
esac

if ! command -v gh >/dev/null 2>&1; then
  echo "Нет GitHub CLI (gh). Поставьте: brew install gh && gh auth login" >&2
  if [[ "$if_changed" -eq 1 ]]; then
    echo "Пуш продолжается без обновления NOTIFIER_CONFIG." >&2
    exit 0
  fi
  exit 1
fi

if ! gh auth status >/dev/null 2>&1; then
  echo "gh не залогинен (gh auth login). Секрет NOTIFIER_CONFIG не обновлён." >&2
  if [[ "$if_changed" -eq 1 ]]; then
    echo "Пуш продолжается без обновления NOTIFIER_CONFIG." >&2
    exit 0
  fi
  exit 1
fi

# gh refuses to guess when origin and upstream both point at GitHub.
repo="$(github_repo)"

gh secret set NOTIFIER_CONFIG --repo "$repo" --app actions < "$file"
current_hash > "$stamp"
echo "GitHub secret NOTIFIER_CONFIG обновлён в $repo."

if [[ "$do_deploy" -eq 1 ]]; then
  gh workflow run "build and deploy" --repo "$repo"
  echo "Запущен workflow build and deploy в $repo — он перезапишет config.yml на VPS и пересоберёт контейнер."
fi
