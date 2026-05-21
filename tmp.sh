#!/bin/sh
set -eu

state_file="${LOCALCI_STATE_FILE:-$HOME/.localci/daemon/state.json}"

if [ ! -f "$state_file" ]; then
  echo "state file not found: $state_file" >&2
  exit 1
fi

json_field() {
  python3 -c 'import json, sys; print(json.load(open(sys.argv[1]))[sys.argv[2]])' "$state_file" "$1"
}

base_url=$(json_field http_base_url)
if [ -z "$base_url" ]; then
  echo "http_base_url not found in $state_file" >&2
  exit 1
fi

usage() {
  cat <<'EOF'
Usage:
  ./tmp.sh state
  ./tmp.sh home
  ./tmp.sh commit <repo> <commit>
  ./tmp.sh task <repo> <commit> <task>
  ./tmp.sh artifact <repo> <commit> <task> <path>
EOF
}

urlencode() {
  python3 -c 'import sys, urllib.parse; print(urllib.parse.quote(sys.argv[1], safe=""))' "$1"
}

request() {
  url=$1
  curl --retry 20 --retry-delay 1 --retry-connrefused -sS "$url"
}

case "${1:-}" in
  state)
    cat "$state_file"
    ;;
  home)
    request "$base_url"
    ;;
  commit)
    [ "$#" -eq 3 ] || { usage >&2; exit 1; }
    repo=$(urlencode "$2")
    commit=$(urlencode "$3")
    request "$base_url/commit?repo=$repo&commit=$commit"
    ;;
  task)
    [ "$#" -eq 4 ] || { usage >&2; exit 1; }
    repo=$(urlencode "$2")
    commit=$(urlencode "$3")
    task=$(urlencode "$4")
    request "$base_url/task?repo=$repo&commit=$commit&task=$task"
    ;;
  artifact)
    [ "$#" -eq 5 ] || { usage >&2; exit 1; }
    repo=$(urlencode "$2")
    commit=$(urlencode "$3")
    task=$(urlencode "$4")
    path=$(urlencode "$5")
    request "$base_url/artifact?repo=$repo&commit=$commit&task=$task&path=$path"
    ;;
  *)
    usage >&2
    exit 1
    ;;
esac
