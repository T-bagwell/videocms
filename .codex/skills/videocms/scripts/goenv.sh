#!/usr/bin/env bash
# Fixed Go toolchain environment for the VideoCMS development machine.
#
# Why this exists: this machine's ambient Go environment is polluted
# (GOPATH contains a shell metacharacter entry like "~/.dotnet/tools",
# GOARCH may be overridden, and GOPROXY may be empty). Running `go`
# directly can fail with "GOPATH entry cannot start with shell
# metacharacter" or try the default proxy and time out.
#
# Usage:
#   source scripts/goenv.sh            # fix env in the current shell
#   scripts/goenv.sh go version        # run one command with a fixed env
#   scripts/goenv.sh go test ./...     # run tests with a fixed env
#   scripts/goenv.sh --in backend go test ./...   # run inside a subdir (Go module lives in backend/)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# scripts/ -> videocms/ -> skills/ -> .codex/ -> repo root
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../../.." && pwd)"

fix_env() {
  # Drop any inherited values that break the toolchain.
  unset GOROOT GOPATH GOARCH GOOS GOFLAGS GONOSUMDB GONOSUMCHECK GOSUMDB 2>/dev/null || true
  # Deterministic, machine-local settings.
  export GOPATH="${HOME}/go"
  export GOARCH="amd64"
  export GOPROXY="https://goproxy.cn,direct"
  export CGO_ENABLED="1"
  export PATH="${GOPATH}/bin:${PATH}"
}

fix_env

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  # Executed as a script: run the given command with the fixed environment.
  TARGET_DIR="${REPO_ROOT}"
  if [[ "${1:-}" == "--in" ]]; then
    TARGET_DIR="${REPO_ROOT}/${2:?usage: --in <subdir>}"
    shift 2
  fi
  if [[ $# -eq 0 ]]; then
    echo "usage: $0 <go|make|...> [args...]" >&2
    exit 2
  fi
  cd "${TARGET_DIR}"
  exec "$@"
fi
