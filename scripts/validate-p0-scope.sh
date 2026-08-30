#!/bin/bash

set -euo pipefail

export LC_ALL=C

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPOSITORY_ROOT=${1:-"$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"}

fail() {
  printf 'P0 scope validation failed: %s\n' "$1" >&2
  exit 1
}

require_file() {
  if [[ ! -f "$1" ]]; then
    fail "missing required file: ${1#"$REPOSITORY_ROOT"/}"
  fi
}

if [[ ! -d "$REPOSITORY_ROOT" ]]; then
  fail "repository root does not exist: $REPOSITORY_ROOT"
fi

MIGRATIONS_FILE="$REPOSITORY_ROOT/ios/ThenApp/Data/Database/DatabaseMigrations.swift"
OPENAPI_FILE="$REPOSITORY_ROOT/backend/openapi.yaml"
BACKEND_SCHEMA_FILE="$REPOSITORY_ROOT/backend/schema.sql"
PROJECT_FILE="$REPOSITORY_ROOT/ios/ThenApp.xcodeproj/project.pbxproj"
PACKAGE_RESOLVED_FILE="$REPOSITORY_ROOT/ios/ThenApp.xcodeproj/project.xcworkspace/xcshareddata/swiftpm/Package.resolved"
PRODUCTION_SOURCE="$REPOSITORY_ROOT/ios/ThenApp"

require_file "$MIGRATIONS_FILE"
require_file "$OPENAPI_FILE"
require_file "$BACKEND_SCHEMA_FILE"
require_file "$PROJECT_FILE"
require_file "$PACKAGE_RESOLVED_FILE"

if [[ ! -d "$PRODUCTION_SOURCE" ]]; then
  fail "missing production source directory: ios/ThenApp"
fi

PROHIBITED_ENTRY_PATTERN='预算|结转|共享预算|"(保存票据|票据附件|导出轨迹|轨迹导出|保存原始轨迹|原始轨迹保留|轨迹云备份|高德地图|百度地图|腾讯地图|订阅方案|购买订阅|立即订阅|付费方案|立即付费|免费容量|权益分层)|Time Sensitive|Critical Alerts|ReceiptAttachment|ReceiptImageStore|TrackExport|TrackUpload|TrackCloud|SubscriptionPlan|Paywall|PurchaseService|EntitlementTier'
PROHIBITED_ENTRY_MATCHES=$(
  {
    rg -n --glob '*.swift' --glob '*.plist' --glob '*.xcprivacy' \
      "$PROHIBITED_ENTRY_PATTERN" "$PRODUCTION_SOURCE" || true
    rg -n "$PROHIBITED_ENTRY_PATTERN" "$PROJECT_FILE" || true
  }
)
if [[ -n "$PROHIBITED_ENTRY_MATCHES" ]]; then
  printf '%s\n' "$PROHIBITED_ENTRY_MATCHES" >&2
  fail "prohibited P0 product entry text or symbol detected"
fi

PROHIBITED_SDK_PATTERN='AMap|MAMap|Baidu(Map|LBS)|TencentLBS|QMap|Mapbox|RevenueCat|Purchases|Adapty|StoreKit\.framework|Firebase|Sentry|AppsFlyer|Adjust'
PROHIBITED_SDK_MATCHES=$(
  {
    rg -n "$PROHIBITED_SDK_PATTERN" "$PROJECT_FILE" || true
    rg -n "$PROHIBITED_SDK_PATTERN" "$PACKAGE_RESOLVED_FILE" || true
  }
)
if [[ -n "$PROHIBITED_SDK_MATCHES" ]]; then
  printf '%s\n' "$PROHIBITED_SDK_MATCHES" >&2
  fail "prohibited P0 SDK detected"
fi

EXPECTED_TABLES='calendar_occurrence_revisions
calendar_occurrences
calendar_scans
calendar_series
calendar_sources
departure_reminders
event_trip_links
journeys
ledger_accounts
ledger_transactions
local_profiles
location_snapshots
navigation_handoffs
postings
route_estimates
track_points
track_segments
transaction_journey_links
trip_plans'
ACTUAL_TABLES=$(
  sed -nE 's/^[[:space:]]*CREATE TABLE ([a-z_]+).*/\1/p' "$MIGRATIONS_FILE" \
    | sort -u
)
if [[ "$ACTUAL_TABLES" != "$EXPECTED_TABLES" ]]; then
  printf '%s\n' 'Expected P0 tables:' "$EXPECTED_TABLES" '' 'Actual P0 tables:' \
    "$ACTUAL_TABLES" >&2
  fail "local database table set differs from the approved 19-table P0 schema"
fi

if ! grep -qx 'paths: {}' "$OPENAPI_FILE"; then
  fail "backend/openapi.yaml must keep an empty paths object before P1 approval"
fi
if grep -Eq '^(components|webhooks):' "$OPENAPI_FILE"; then
  fail "backend/openapi.yaml contains unapproved component or webhook placeholders"
fi

if grep -Ein '^[[:space:]]*CREATE[[:space:]]+(TABLE|TYPE)([[:space:]]|$)' "$BACKEND_SCHEMA_FILE"; then
  fail "backend/schema.sql contains unapproved P1 business DDL"
fi

if ! command -v ruby >/dev/null 2>&1; then
  fail "ruby is required to validate Package.resolved"
fi

EXPECTED_PACKAGES='grdb.swift
openapikit
swift-algorithms
swift-argument-parser
swift-collections
swift-http-types
swift-numerics
swift-openapi-generator
swift-openapi-runtime
swift-openapi-urlsession
yams'
ACTUAL_PACKAGES=$(
  ruby -rjson -e \
    'document = JSON.parse(File.read(ARGV.fetch(0))); puts document.fetch("pins").map { |pin| pin.fetch("identity") }.sort' \
    "$PACKAGE_RESOLVED_FILE"
)
if [[ "$ACTUAL_PACKAGES" != "$EXPECTED_PACKAGES" ]]; then
  printf '%s\n' 'Expected SwiftPM packages:' "$EXPECTED_PACKAGES" '' \
    'Actual SwiftPM packages:' "$ACTUAL_PACKAGES" >&2
  fail "SwiftPM package set differs from the approved P0 dependency baseline"
fi

printf '%s\n' \
  'P0 scope verified:' \
  '- no prohibited product entry text or SDK references' \
  '- local database table set matches 19 approved tables' \
  '- backend OpenAPI paths remain empty' \
  '- backend schema contains no P1 business DDL' \
  '- SwiftPM dependency set matches 11 locked packages'
