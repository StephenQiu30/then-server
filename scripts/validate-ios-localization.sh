#!/bin/bash

set -euo pipefail

export LC_ALL=en_US.UTF-8

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPOSITORY_ROOT=${1:-"$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"}
APP_BUNDLE=${2:-}

fail() {
  printf 'iOS localization validation failed: %s\n' "$1" >&2
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

LOCALIZABLE_CATALOG="$REPOSITORY_ROOT/app/ThenApp/Localizable.xcstrings"
INFO_PLIST_CATALOG="$REPOSITORY_ROOT/app/ThenApp/InfoPlist.xcstrings"
INFO_PLIST="$REPOSITORY_ROOT/app/ThenApp/Info.plist"
PROJECT_FILE="$REPOSITORY_ROOT/app/ThenApp.xcodeproj/project.pbxproj"
OPENAPI_FILE="$REPOSITORY_ROOT/backend/openapi.yaml"

require_file "$LOCALIZABLE_CATALOG"
require_file "$INFO_PLIST_CATALOG"
require_file "$INFO_PLIST"
require_file "$PROJECT_FILE"
require_file "$OPENAPI_FILE"

if [[ $(find "$REPOSITORY_ROOT/app/ThenApp" -type f -name '*.xcstrings' | wc -l | tr -d ' ') != 2 ]]; then
  fail "ThenApp production source must contain exactly two string catalogs"
fi

if find "$REPOSITORY_ROOT/app/ThenApp" -type f -name '*.strings' -print -quit | grep -q .; then
  fail "handwritten .strings files are not allowed beside the approved catalogs"
fi

command -v ruby >/dev/null 2>&1 || fail "ruby is required"
command -v xcodebuild >/dev/null 2>&1 || fail "xcodebuild is required"
xcrun --find xcstringstool >/dev/null 2>&1 || fail "xcstringstool is required"

ruby -rjson - "$LOCALIZABLE_CATALOG" "$INFO_PLIST_CATALOG" <<'RUBY'
# encoding: UTF-8
localizable_path, info_path = ARGV
localizable = JSON.parse(File.read(localizable_path))
info = JSON.parse(File.read(info_path))

def fail_validation(message)
  warn "iOS localization validation failed: #{message}"
  exit 1
end

def format_tokens(value)
  value.scan(/%(?:\d+\$)?(?:lld|llu|ld|lu|zd|zu|d|u|f|g|@)/).map do |token|
    token.sub(/%\d+\$/, "%")
  end
end

unless localizable["sourceLanguage"] == "zh-Hans" && info["sourceLanguage"] == "zh-Hans"
  fail_validation("both catalogs must use zh-Hans as sourceLanguage")
end

localizable_strings = localizable.fetch("strings")
fail_validation("Localizable.xcstrings contains an empty key") if localizable_strings.key?("")
fail_validation("Localizable.xcstrings is empty") if localizable_strings.empty?

localizable_strings.each do |key, entry|
  unit = entry.dig("localizations", "zh-Hans", "stringUnit")
  fail_validation("missing reviewed zh-Hans value for #{key.inspect}") unless unit
  fail_validation("unreviewed zh-Hans value for #{key.inspect}") unless unit["state"] == "translated"
  value = unit.fetch("value", "")
  fail_validation("blank zh-Hans value for #{key.inspect}") if value.empty?
  unless format_tokens(key) == format_tokens(value)
    fail_validation("format placeholders differ for #{key.inspect}")
  end
end

expected_info = {
  "CFBundleDisplayName" => "于是",
  "CFBundleName" => "ThenApp",
  "NSCalendarsFullAccessUsageDescription" => "用于读取你明确选择的系统日历，将时间和地点转为本地出行计划；于是不会修改系统事件。",
  "NSCameraUsageDescription" => "用于拍摄票据并在设备内识别记账候选，不保存票据原图。",
  "NSLocationWhenInUseUsageDescription" => "只在你主动开始行程后记录本次位置，用于生成距离和完整性摘要；确认摘要后删除原始轨迹。"
}
info_strings = info.fetch("strings")
unless info_strings.keys.sort == expected_info.keys.sort
  fail_validation("InfoPlist.xcstrings keys differ from the five approved system values")
end

expected_info.each do |key, expected_value|
  entry = info_strings.fetch(key)
  unit = entry.dig("localizations", "zh-Hans", "stringUnit")
  fail_validation("missing reviewed InfoPlist value for #{key}") unless unit
  unless unit["state"] == "translated" && unit["value"] == expected_value
    fail_validation("InfoPlist value differs from the approved copy for #{key}")
  end
end

puts "catalog content verified: #{localizable_strings.length} Localizable keys and #{info_strings.length} InfoPlist keys"
RUBY

for permission_key in \
  NSCalendarsFullAccessUsageDescription \
  NSCameraUsageDescription \
  NSLocationWhenInUseUsageDescription; do
  plist_value=$(plutil -extract "$permission_key" raw -o - "$INFO_PLIST")
  catalog_value=$(ruby -rjson -e \
    'data = JSON.parse(File.read(ARGV.fetch(0))); puts data.fetch("strings").fetch(ARGV.fetch(1)).dig("localizations", "zh-Hans", "stringUnit", "value")' \
    "$INFO_PLIST_CATALOG" "$permission_key")
  [[ "$plist_value" == "$catalog_value" ]] \
    || fail "Info.plist and InfoPlist.xcstrings differ for $permission_key"
done

ruby - "$PROJECT_FILE" <<'RUBY'
project_path = ARGV.fetch(0)
project = File.read(project_path)

def fail_validation(message)
  warn "iOS localization validation failed: #{message}"
  exit 1
end

target_section = project[%r{/\* Begin PBXNativeTarget section \*/(.*?)/\* End PBXNativeTarget section \*/}m, 1]
fail_validation("PBXNativeTarget section was not found") unless target_section
target_match = target_section.scan(%r{\t\t[A-F0-9]{24} /\* .*? \*/ = \{\n(.*?)\n\t\t\};}m).find do |match|
  match.first.include?("\n\t\t\tname = ThenApp;\n")
end
fail_validation("ThenApp native target was not found") unless target_match
resources_id = target_match.first[%r{([A-F0-9]{24}) /\* Resources \*/}, 1]
fail_validation("ThenApp Resources build phase was not found") unless resources_id
resources_section = project[%r{/\* Begin PBXResourcesBuildPhase section \*/(.*?)/\* End PBXResourcesBuildPhase section \*/}m, 1]
fail_validation("PBXResourcesBuildPhase section was not found") unless resources_section
phase_match = resources_section.match(%r{#{resources_id} /\* Resources \*/ = \{.*?files = \((.*?)\);.*?\};}m)
fail_validation("ThenApp Resources build phase contents were not found") unless phase_match
resources = phase_match[1]

["Localizable.xcstrings", "InfoPlist.xcstrings"].each do |name|
  path_count = project.scan(%r{path = #{Regexp.escape(name)};}).length
  fail_validation("#{name} must have exactly one file reference") unless path_count == 1
  resource_count = resources.scan(%r{/\* #{Regexp.escape(name)} in Resources \*/}).length
  fail_validation("#{name} must appear exactly once in ThenApp Resources") unless resource_count == 1
end
RUBY

TEMPORARY_ROOT=$(mktemp -d /tmp/then-ios-localization.XXXXXX)
trap 'rm -rf "$TEMPORARY_ROOT"' EXIT
COMPILED_ROOT="$TEMPORARY_ROOT/compiled"
mkdir -p "$COMPILED_ROOT"

xcrun xcstringstool compile "$LOCALIZABLE_CATALOG" \
  --output-directory "$COMPILED_ROOT" --language zh-Hans
xcrun xcstringstool compile "$INFO_PLIST_CATALOG" \
  --output-directory "$COMPILED_ROOT" --language zh-Hans

[[ -s "$COMPILED_ROOT/zh-Hans.lproj/Localizable.strings" ]] \
  || fail "Localizable.xcstrings did not compile a non-empty zh-Hans resource"
[[ -s "$COMPILED_ROOT/zh-Hans.lproj/InfoPlist.strings" ]] \
  || fail "InfoPlist.xcstrings did not compile a non-empty zh-Hans resource"

WORKING_ROOT="$TEMPORARY_ROOT/repository"
mkdir -p "$WORKING_ROOT/backend"
/usr/bin/ditto "$REPOSITORY_ROOT/ios" "$WORKING_ROOT/ios"
cp "$OPENAPI_FILE" "$WORKING_ROOT/backend/openapi.yaml"

EXPORT_ROOT="$TEMPORARY_ROOT/export"
DERIVED_DATA_ROOT="$TEMPORARY_ROOT/DerivedData"
mkdir -p "$EXPORT_ROOT"
if ! xcodebuild -exportLocalizations \
  -project "$WORKING_ROOT/app/ThenApp.xcodeproj" \
  -scheme ThenApp \
  -localizationPath "$EXPORT_ROOT" \
  -exportLanguage zh-Hans \
  -derivedDataPath "$DERIVED_DATA_ROOT" \
  -skipPackagePluginValidation \
  -onlyUsePackageVersionsFromResolvedFile \
  CODE_SIGNING_ALLOWED=NO \
  >"$TEMPORARY_ROOT/xcodebuild-export.log" 2>&1; then
  tail -n 60 "$TEMPORARY_ROOT/xcodebuild-export.log" >&2
  fail "Xcode could not export production localizations"
fi

SYNCED_LOCALIZABLE="$WORKING_ROOT/app/ThenApp/Localizable.xcstrings"
SYNCED_INFO="$WORKING_ROOT/app/ThenApp/InfoPlist.xcstrings"
require_file "$SYNCED_LOCALIZABLE"
require_file "$SYNCED_INFO"

ruby -rjson - "$LOCALIZABLE_CATALOG" "$SYNCED_LOCALIZABLE" "$INFO_PLIST_CATALOG" "$SYNCED_INFO" <<'RUBY'
committed_localizable_path, synced_localizable_path, committed_info_path, synced_info_path = ARGV
committed_localizable = JSON.parse(File.read(committed_localizable_path)).fetch("strings")
synced_localizable = JSON.parse(File.read(synced_localizable_path)).fetch("strings")
committed_info = JSON.parse(File.read(committed_info_path)).fetch("strings")
synced_info = JSON.parse(File.read(synced_info_path)).fetch("strings")

def fail_validation(message)
  warn "iOS localization validation failed: #{message}"
  exit 1
end

if synced_localizable.key?("")
  fail_validation("Xcode extracted an empty production localization key")
end

stale_keys = synced_localizable.filter_map do |key, entry|
  key if entry["extractionState"] == "stale"
end
fail_validation("stale Localizable keys: #{stale_keys.join(", ")}") unless stale_keys.empty?

missing = synced_localizable.keys - committed_localizable.keys
obsolete = committed_localizable.keys - synced_localizable.keys
unless missing.empty? && obsolete.empty?
  fail_validation("catalog drift; missing=#{missing.inspect}, obsolete=#{obsolete.inspect}")
end

unless synced_info.keys.sort == committed_info.keys.sort
  fail_validation("InfoPlist catalog drift after Xcode extraction")
end

puts "Xcode extraction verified: #{synced_localizable.length} Localizable keys and #{synced_info.length} InfoPlist keys"
RUBY

if [[ -n "$APP_BUNDLE" ]]; then
  [[ -d "$APP_BUNDLE" ]] || fail "app bundle does not exist: $APP_BUNDLE"
  [[ -s "$APP_BUNDLE/zh-Hans.lproj/Localizable.strings" ]] \
    || fail "app bundle is missing zh-Hans.lproj/Localizable.strings"
  [[ -s "$APP_BUNDLE/zh-Hans.lproj/InfoPlist.strings" ]] \
    || fail "app bundle is missing zh-Hans.lproj/InfoPlist.strings"
  [[ $(plutil -extract CFBundleDisplayName raw -o - "$APP_BUNDLE/Info.plist") == "于是" ]] \
    || fail "app bundle display name is not 于是"
  printf 'app localization resources verified: %s\n' "$APP_BUNDLE/zh-Hans.lproj"
fi

printf '%s\n' \
  'iOS localization verified:' \
  '- exactly two reviewed zh-Hans string catalogs' \
  '- compiler extraction and committed keys are synchronized' \
  '- Info.plist permission copy matches the reviewed catalog' \
  '- both catalogs compile and belong to ThenApp Resources'
