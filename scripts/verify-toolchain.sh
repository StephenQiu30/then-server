#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(dirname -- "$script_dir")

fail() {
    printf 'toolchain verification failed: %s\n' "$1" >&2
    exit 1
}

expected_xcode=$(tr -d '\r\n' < "$repo_root/.xcode-version")
actual_xcode=$(xcodebuild -version | awk 'NR == 1 { print $2 }')
[ "$actual_xcode" = "$expected_xcode" ] || fail "expected Xcode $expected_xcode, got $actual_xcode"

expected_swift=6.3.3
actual_swift=$(swift --version 2>&1 | sed -nE 's/.*Apple Swift version ([0-9.]+).*/\1/p' | head -n 1)
[ "$actual_swift" = "$expected_swift" ] || fail "expected Swift $expected_swift, got $actual_swift"

expected_go=1.26.5
actual_go=$(go version | sed -nE 's/.* go([0-9.]+) .*/\1/p')
[ "$actual_go" = "$expected_go" ] || fail "expected Go $expected_go, got $actual_go"

go_module=$(cd "$repo_root/backend" && go mod edit -json)
printf '%s' "$go_module" | grep -Fq '"Path": "github.com/StephenQiu30/then/backend"' || fail "unexpected Go module path"
printf '%s' "$go_module" | grep -Fq '"Go": "1.26.0"' || fail "expected Go language 1.26.0"
printf '%s' "$go_module" | grep -Fq '"Toolchain": "go1.26.5"' || fail "expected Go toolchain go1.26.5"

printf 'toolchain verified: Xcode %s, Swift %s, Go %s\n' "$actual_xcode" "$actual_swift" "$actual_go"
