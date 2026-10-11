#!/usr/bin/env bash
set -euo pipefail
# Paths arrive from Node as Windows paths. Git Bash converts the tool directory.
export PATH="$(cygpath -u "$2"):$PATH"
cd "$1"
shift 2
./configure "$@"
# Native Windows make cannot strip the nested quiet-command prefixes used by
# FFmpeg. Disable those prefixes and let make suppress command echo instead.
make -s V=1 -j "${MEDIANINJA_BUILD_JOBS:-4}"
