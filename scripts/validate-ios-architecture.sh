#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=${1:-$(dirname -- "$script_dir")}
source_root="$repo_root/ios/ThenApp"
project_file="$repo_root/ios/ThenApp.xcodeproj/project.pbxproj"
package_file="$repo_root/ios/ThenApp.xcodeproj/project.xcworkspace/xcshareddata/swiftpm/Package.resolved"
app_entry="$source_root/App/ThenApp.swift"

fail() {
    printf 'iOS architecture validation failed: %s\n' "$1" >&2
    exit 1
}

[ -d "$source_root" ] || fail "missing ios/ThenApp"
[ -f "$project_file" ] || fail "missing project.pbxproj"
[ -f "$package_file" ] || fail "missing Package.resolved"
[ -f "$app_entry" ] || fail "missing SwiftUI app entry"

grep -q '^import SwiftUI$' "$app_entry" || fail "ThenApp entry must import SwiftUI"
grep -q '^@main$' "$app_entry" || fail "ThenApp entry must use @main"
grep -Eq '^struct[[:space:]]+ThenApp:[[:space:]]+App[[:space:]]*\{' "$app_entry" \
    || fail "ThenApp entry must conform to SwiftUI App"

prohibited_source_pattern='^import[[:space:]]+(SwiftData|CoreData|RealmSwift|RxSwift|ReactiveSwift|Combine)$|(^|[^[:alnum:]_])(ObservableObject|@Published|@StateObject|@ObservedObject)([^[:alnum:]_]|$)'
if find "$source_root" -type f -name '*.swift' -exec grep -En "$prohibited_source_pattern" {} +; then
    fail "prohibited persistence or state-management framework detected"
fi

prohibited_package_pattern='alamofire|moya|realm|rxswift|reactiveswift|swinject|needle|the-composable-architecture'
if grep -Ein "$prohibited_package_pattern" "$package_file"; then
    fail "prohibited Swift package detected"
fi

grep -q '"identity" : "grdb.swift"' "$package_file" \
    || fail "GRDB must remain locked in Package.resolved"
grep -q '"identity" : "swift-openapi-generator"' "$package_file" \
    || fail "Swift OpenAPI Generator must remain locked"
grep -q '"identity" : "swift-openapi-runtime"' "$package_file" \
    || fail "Swift OpenAPI Runtime must remain locked"
grep -q '"identity" : "swift-openapi-urlsession"' "$package_file" \
    || fail "Swift OpenAPI URLSession must remain locked"

grep -q 'IPHONEOS_DEPLOYMENT_TARGET = 18.0;' "$project_file" \
    || fail "minimum deployment target must remain iOS 18"
grep -q 'SWIFT_VERSION = 6.0;' "$project_file" \
    || fail "Swift 6 language mode is required"
grep -q 'SWIFT_STRICT_CONCURRENCY = complete;' "$project_file" \
    || fail "Complete Strict Concurrency is required"
grep -q 'SWIFT_DEFAULT_ACTOR_ISOLATION = MainActor;' "$project_file" \
    || fail "ThenApp target must default to MainActor isolation"

uikit_imports=$(find "$source_root" -type f -name '*.swift' -exec grep -l '^import UIKit$' {} + || true)
if [ -n "$uikit_imports" ]; then
    old_ifs=$IFS
    IFS='
'
    for file in $uikit_imports; do
        case "$file" in
            */Services/*|*Picker.swift|*ShareSheet.swift)
                ;;
            *)
                printf '%s\n' "$file" >&2
                fail "UIKit import must stay inside an explicit system bridge"
                ;;
        esac
    done
    IFS=$old_ifs
fi

printf '%s\n' \
    'iOS architecture verified:' \
    '- SwiftUI App lifecycle is the only product UI entry' \
    '- Observation/Swift Concurrency baseline has no prohibited alternative framework' \
    '- iOS 18, Swift 6, strict concurrency and MainActor settings are present' \
    '- GRDB and Swift OpenAPI packages are locked' \
    '- UIKit imports remain limited to explicit system bridges'
