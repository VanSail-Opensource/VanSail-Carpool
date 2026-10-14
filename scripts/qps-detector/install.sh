#!/usr/bin/env bash
set -euo pipefail

config_path=/etc/tx-carpool-qps-detector.conf
reporter_path=/usr/local/sbin/vansail-carpool-qps-detector-report
cron_path=/etc/cron.d/tx-carpool-qps-detector

require_root() { [ "${EUID}" -eq 0 ] || { printf '%s\n' 'Run as root.' >&2; exit 1; }; }
remove() { rm -f "$cron_path" "$reporter_path" "$config_path"; rm -rf /var/lib/tx-carpool-qps-detector; printf '%s\n' 'QPS detector reporter removed.'; }
stream_xlogs() {
  if command -v remnanode >/dev/null; then
    remnanode xlogs
    return
  fi
  command -v docker >/dev/null || { printf '%s\n' 'Remnanode or Docker is required.' >&2; exit 1; }
  docker exec remnanode xlogs
}
print_logs() {
  local -a statuses
  command -v head >/dev/null || { printf '%s\n' 'head is required.' >&2; exit 1; }
  set +e
  stream_xlogs | head -n 1000
  statuses=("${PIPESTATUS[@]}")
  set -e
  [ "${statuses[1]}" -eq 0 ] || return "${statuses[1]}"
  [ "${statuses[0]}" -eq 0 ] || [ "${statuses[0]}" -eq 141 ] || return "${statuses[0]}"
}
require_debian() { [ -r /etc/debian_version ] && grep -q '^13' /etc/debian_version || { printf '%s\n' 'Debian 13 is required.' >&2; exit 1; }; command -v docker >/dev/null; command -v curl >/dev/null; command -v flock >/dev/null; command -v split >/dev/null; command -v stat >/dev/null; command -v tail >/dev/null; command -v timeout >/dev/null; command -v truncate >/dev/null; command -v wc >/dev/null; systemctl is-active --quiet cron || { printf '%s\n' 'The cron service must be active.' >&2; exit 1; }; }
install_reporter() {
  install -d -m 700 /var/lib/tx-carpool-qps-detector
  install -m 600 /dev/null "$config_path"
  printf 'API_URL=%q\nNODE_TOKEN=%q\nINTERVAL_MINUTES=%q\n' "$api_url" "$node_token" "$interval" > "$config_path"
  cat > "$reporter_path" <<'REPORTER'
#!/usr/bin/env bash
set -euo pipefail
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
config_path=/etc/tx-carpool-qps-detector.conf
state_path=/var/lib/tx-carpool-qps-detector/report-state-v3
lock_path=/var/lib/tx-carpool-qps-detector/lock
umask 077
exec 9>"$lock_path"
flock -n 9 || { [ "${REPORTER_REQUIRE_RUN:-0}" = 1 ] && exit 75; exit 0; }
source "$config_path"
started_at=$(date +%s)
last=0
if [ -r "$state_path" ]; then read -r last < "$state_path" || true; fi
[[ "$last" =~ ^[0-9]+$ ]] || last=0
[ "$started_at" -ge "$((last + INTERVAL_MINUTES * 60))" ] || exit 0
payload=$(mktemp)
batch_dir=$(mktemp -d)
state_tmp="${state_path}.tmp"
trap 'rm -rf "$batch_dir"; rm -f "$payload" "$state_tmp"' EXIT
max_report_bytes=$((15 * 1024 * 1024))
capture_command() {
  local -a statuses command
  command=("$@")
  : > "$payload"
  set +e
  timeout --signal=INT --kill-after=5s 5s "${command[@]}" | tail -n 1000 > "$payload"
  statuses=("${PIPESTATUS[@]}")
  set -e
  [ "${statuses[1]}" -eq 0 ] || return "${statuses[1]}"
  case "${statuses[0]}" in 0|124|137) return 0 ;; *) return "${statuses[0]}" ;; esac
}
capture() {
  if command -v remnanode >/dev/null && capture_command remnanode xlogs; then return; fi
  command -v docker >/dev/null || { printf '%s\n' 'Remnanode or Docker is required.' >&2; return 1; }
  capture_command docker exec remnanode xlogs
}
upload() {
  curl --fail --silent --show-error --connect-timeout 10 --max-time 120 --retry 2 --retry-all-errors --config - --data-binary "@$1" "$API_URL/api/v1/agents/qps-reports" > /dev/null <<CURL
header = "Authorization: Bearer $NODE_TOKEN"
request = "POST"
header = "Content-Type: text/plain"
CURL
}
capture
if [ -s "$payload" ] && [ "$(tail -c 1 "$payload" | wc -l)" -eq 0 ]; then
  captured_bytes=$(stat -c '%s' "$payload")
  partial_bytes=$(tail -n 1 "$payload" | wc -c)
  truncate -s "$((captured_bytes - partial_bytes))" "$payload"
fi
[ -s "$payload" ] || printf '\n' > "$payload"
split -C "$max_report_bytes" --numeric-suffixes=0 --suffix-length=4 "$payload" "$batch_dir/part-"
for part in "$batch_dir"/part-*; do upload "$part"; done
printf '%s\n' "$started_at" > "$state_tmp"
mv "$state_tmp" "$state_path"
REPORTER
  chmod 700 "$reporter_path"
  printf '* * * * * root %s\n' "$reporter_path" > "$cron_path"
  chmod 600 "$cron_path"
  if ! REPORTER_REQUIRE_RUN=1 "$reporter_path"; then
    printf '%s\n' 'Reporter installed, but the initial report was not accepted. Cron will retry.' >&2
    return 1
  fi
}
main() {
  require_root
  if [ "${1:-install}" = "uninstall" ]; then remove; return; fi
  if [ "${1:-install}" = "test" ]; then print_logs; return; fi
  require_debian
  read -r -p 'API domain (https://panel.example): ' api_url
  read -r -s -p 'Node token: ' node_token; printf '\n'
  read -r -p 'Report interval in minutes [30]: ' interval
  interval=${interval:-30}
  [[ "$api_url" =~ ^https?://[^[:space:]]+$ ]] && [[ "$node_token" =~ ^[A-Za-z0-9_-]{20,}$ ]] && [[ "$interval" =~ ^[1-9][0-9]{0,3}$ ]] || { printf '%s\n' 'Invalid configuration.' >&2; exit 1; }
  api_url=${api_url%/}
  install_reporter
  printf '%s\n' 'QPS detector reporter installed and the initial report was accepted.'
}
main "$@"
