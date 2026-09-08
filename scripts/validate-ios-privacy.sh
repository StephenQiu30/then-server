#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(dirname -- "$script_dir")
manifest="$repo_root/app/ThenApp/App/PrivacyInfo.xcprivacy"
source_root="$repo_root/app/ThenApp"

fail() {
    printf 'iOS privacy verification failed: %s\n' "$1" >&2
    exit 1
}

[ -f "$manifest" ] || fail "missing ThenApp privacy manifest"
[ "$(find "$source_root" -type f -name 'PrivacyInfo.xcprivacy' | wc -l | tr -d ' ')" = 1 ] \
    || fail "ThenApp source must contain exactly one privacy manifest"

plutil -lint "$manifest" >/dev/null || fail "invalid privacy manifest plist"
[ "$(plutil -extract NSPrivacyTracking raw -o - "$manifest")" = false ] \
    || fail "tracking must remain disabled for P0"

for key in NSPrivacyTrackingDomains NSPrivacyCollectedDataTypes NSPrivacyAccessedAPITypes; do
    [ "$(plutil -extract "$key" json -o - "$manifest")" = '[]' ] \
        || fail "$key must remain empty for the approved P0 data flow"
done

required_reason_pattern='UserDefaults|activeInputModes|systemUptime|mach_absolute_time|attributesOfFileSystem|volumeAvailableCapacity|contentModificationDate|creationDate|NSFileModificationDate|NSFileCreationDate|(^|[^[:alnum:]_])(f?stat)[[:space:]]*\('
if find "$source_root" -type f -name '*.swift' -exec grep -En "$required_reason_pattern" {} +; then
    fail "source uses a Required Reason API without an approved manifest reason"
fi

network_pattern='(^|[^[:alnum:]_])(URLSession|URLRequest|NWConnection|NWPathMonitor)([^[:alnum:]_]|$)|^import Network$|Firebase|Sentry|PostHog|Analytics'
if find "$source_root" -type f -name '*.swift' -exec grep -En "$network_pattern" {} +; then
    fail "P0 source contains an unapproved network, analytics, crash, or tracking entry point"
fi

production_log_pattern='(^|[^[:alnum:]_])(print|debugPrint|dump|NSLog|NSLogv|CFShow|os_log|os_signpost|Logger|OSLog|OSSignposter|OSSignpostID)([^[:alnum:]_]|$)|^import[[:space:]]+OSLog$|FileHandle\.standard(Output|Error)|STDOUT_FILENO|STDERR_FILENO'
if find "$source_root" -type f -name '*.swift' -exec grep -En "$production_log_pattern" {} +; then
    fail "P0 production source contains an unapproved logging or diagnostic output entry point"
fi

if [ "$#" -gt 1 ]; then
    fail "usage: scripts/validate-ios-privacy.sh [ThenApp.app]"
fi

if [ "$#" -eq 1 ]; then
    app_bundle=$1
    [ -d "$app_bundle" ] || fail "app bundle does not exist: $app_bundle"
    bundled_manifest="$app_bundle/PrivacyInfo.xcprivacy"
    [ -f "$bundled_manifest" ] || fail "app bundle root is missing PrivacyInfo.xcprivacy"
    [ "$(find "$app_bundle" -maxdepth 1 -type f -name 'PrivacyInfo.xcprivacy' | wc -l | tr -d ' ')" = 1 ] \
        || fail "app bundle root must contain exactly one privacy manifest"
    plutil -lint "$bundled_manifest" >/dev/null \
        || fail "bundled privacy manifest is invalid"
    cmp -s "$manifest" "$bundled_manifest" \
        || fail "bundled privacy manifest differs from the reviewed source"

    embedded_manifests=$(find "$app_bundle" -type f -name 'PrivacyInfo.xcprivacy' -print | LC_ALL=C sort)
    [ -n "$embedded_manifests" ] || fail "app bundle contains no privacy manifests"
    embedded_manifest_count=0
    old_ifs=$IFS
    IFS='
'
    for embedded_manifest in $embedded_manifests; do
        embedded_manifest_count=$((embedded_manifest_count + 1))
        plutil -lint "$embedded_manifest" >/dev/null \
            || fail "invalid embedded privacy manifest: ${embedded_manifest#"$app_bundle"/}"

        tracking=false
        if plutil -extract NSPrivacyTracking raw -o - "$embedded_manifest" >/dev/null 2>&1; then
            tracking=$(plutil -extract NSPrivacyTracking raw -o - "$embedded_manifest")
        fi
        [ "$tracking" = false ] \
            || fail "embedded manifest enables tracking: ${embedded_manifest#"$app_bundle"/}"

        for key in NSPrivacyTrackingDomains NSPrivacyCollectedDataTypes NSPrivacyAccessedAPITypes; do
            value='[]'
            if plutil -extract "$key" json -o - "$embedded_manifest" >/dev/null 2>&1; then
                value=$(plutil -extract "$key" json -o - "$embedded_manifest")
            fi
            [ "$value" = '[]' ] \
                || fail "embedded manifest changes the approved P0 $key declaration: ${embedded_manifest#"$app_bundle"/}"
        done

        printf 'embedded privacy manifest verified: %s\n' \
            "${embedded_manifest#"$app_bundle"/}"
    done
    IFS=$old_ifs
    printf 'embedded privacy inventory verified: %s manifest(s)\n' "$embedded_manifest_count"
fi

printf 'iOS privacy verified: no tracking, off-device collection, Required Reason API, P0 network entry point, or production logging output\n'
