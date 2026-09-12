#!/bin/bash
# Wrapper de compatibilidade. O lifecycle, HMR e rebuild são gerenciados pelo Wails v3.
set -e

exec wails3 dev -config ./build/config.yml -port "${WAILS_VITE_PORT:-5173}"
