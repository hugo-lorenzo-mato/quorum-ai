#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/run-monitored.sh [options] -- "prompt text"

Options:
  -p, --prompt TEXT        Prompt text (preferred). Use "--" to pass as positional.
  -f, --file PATH          Read prompt from file.
  -c, --config PATH        Config file path to pass to quorum.
  --max-idle SECONDS       Stop if no new log output for this long. Default: 90.
  --max-runtime SECONDS    Stop after this total runtime (0 disables). Default: 0.
  --max-retries N          Max retries for quorum run. Default: 1.
  --log-dir DIR            Log directory. Default: .quorum/logs.
  --quorum-cmd PATH         Quorum binary (default: ./bin/quorum if present, else quorum).
  -h, --help               Show help.

Examples:
  scripts/run-monitored.sh -- "Write a short plan for updating docs"
  scripts/run-monitored.sh -f prompt.txt --max-idle 120
USAGE
}

prompt=""
prompt_file=""
config_path=""
max_idle=90
max_runtime=0
max_retries=1
log_dir=".quorum/logs"
quorum_cmd=""

if [ -x ./bin/quorum ]; then
  quorum_cmd="./bin/quorum"
else
  quorum_cmd="quorum"
fi

extra_args=()

while [ "$#" -gt 0 ]; do
  case "$1" in
    -p|--prompt)
      prompt="$2"
      shift 2
      ;;
    -f|--file)
      prompt_file="$2"
      shift 2
      ;;
    -c|--config)
      config_path="$2"
      shift 2
      ;;
    --max-idle)
      max_idle="$2"
      shift 2
      ;;
    --max-runtime)
      max_runtime="$2"
      shift 2
      ;;
    --max-retries)
      max_retries="$2"
      shift 2
      ;;
    --log-dir)
      log_dir="$2"
      shift 2
      ;;
    --quorum-cmd)
      quorum_cmd="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      if [ "$#" -gt 0 ] && [ -z "$prompt" ] && [ -z "$prompt_file" ]; then
        prompt="$*"
      else
        extra_args+=("$@")
      fi
      break
      ;;
    *)
      extra_args+=("$1")
      shift
      ;;
  esac
done

if [ -z "$prompt" ] && [ -z "$prompt_file" ]; then
  echo "Error: prompt required. Use --prompt, --file, or -- 'prompt text'." >&2
  usage
  exit 1
fi

mkdir -p "$log_dir"
log_file="$log_dir/run-$(date +%Y%m%d-%H%M%S).log"

cmd=("$quorum_cmd" run --max-retries "$max_retries")
if [ -n "$config_path" ]; then
  cmd+=(--config "$config_path")
fi
if [ -n "$prompt_file" ]; then
  cmd+=(--file "$prompt_file")
else
  cmd+=("$prompt")
fi
if [ "${#extra_args[@]}" -gt 0 ]; then
  cmd+=("${extra_args[@]}")
fi

"${cmd[@]}" >"$log_file" 2>&1 &
pid=$!

start_time=$(date +%s)
last_change=$start_time
last_mtime=0

echo "PID=$pid log=$log_file"

cleanup() {
  if kill -0 "$pid" 2>/dev/null; then
    kill -INT "$pid" || true
  fi
}
trap cleanup EXIT

while kill -0 "$pid" 2>/dev/null; do
  if [ -f "$log_file" ]; then
    mtime=$(stat -c %Y "$log_file" 2>/dev/null || echo 0)
    if [ "$mtime" -ne "$last_mtime" ]; then
      tail -n 5 "$log_file" || true
      last_mtime=$mtime
      last_change=$(date +%s)
    fi
  fi

  now=$(date +%s)
  if [ "$max_idle" -gt 0 ] && [ $((now-last_change)) -gt "$max_idle" ]; then
    echo "No output for ${max_idle}s, sending SIGINT"
    kill -INT "$pid" || true
    break
  fi

  if [ "$max_runtime" -gt 0 ] && [ $((now-start_time)) -gt "$max_runtime" ]; then
    echo "Max runtime ${max_runtime}s exceeded, sending SIGINT"
    kill -INT "$pid" || true
    break
  fi

  sleep 5
done

wait "$pid" || true
echo "run finished"
