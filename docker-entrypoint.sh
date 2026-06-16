#!/bin/sh
set -eu

args=""

if [ "${LISTEN:-}" != "" ]; then
  args="$args -listen ${LISTEN}"
fi
if [ "${API_TOKEN:-}" != "" ]; then
  args="$args -api-token ${API_TOKEN}"
fi
if [ "${DEFAULT_VOICE:-}" != "" ]; then
  args="$args -default-voice ${DEFAULT_VOICE}"
fi
if [ "${PROXY:-}" != "" ]; then
  args="$args -proxy ${PROXY}"
fi
if [ "${UPSTREAM_TIMEOUT:-}" != "" ]; then
  args="$args -upstream-timeout ${UPSTREAM_TIMEOUT}"
fi
if [ "${UPSTREAM_CONCURRENCY:-}" != "" ]; then
  args="$args -upstream-concurrency ${UPSTREAM_CONCURRENCY}"
fi
if [ "${UPSTREAM_INTERVAL_MS:-}" != "" ]; then
  args="$args -upstream-interval-ms ${UPSTREAM_INTERVAL_MS}"
fi
if [ "${READ_HEADER_TIMEOUT:-}" != "" ]; then
  args="$args -read-header-timeout ${READ_HEADER_TIMEOUT}"
fi
if [ "${IDLE_TIMEOUT:-}" != "" ]; then
  args="$args -idle-timeout ${IDLE_TIMEOUT}"
fi

# shellcheck disable=SC2086
exec edge-tts-compatible $args "$@"
