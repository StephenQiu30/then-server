#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(dirname -- "$script_dir")
source_root="$repo_root/app/ThenApp"

fail() {
    printf 'iOS motion verification failed: %s\n' "$1" >&2
    exit 1
}

[ -d "$source_root" ] || fail "missing ThenApp production source"

custom_motion_pattern='(^|[^[:alnum:]_])(withAnimation|withTransaction|Animation|AnyTransition|matchedGeometryEffect|matchedTransitionSource|navigationTransition|phaseAnimator|keyframeAnimator|symbolEffect|contentTransition|scrollTransition|TimelineView|UIViewPropertyAnimator|CATransaction|CAAnimation|CABasicAnimation|CAKeyframeAnimation|CASpringAnimation|CAAnimationGroup|CADisplayLink)([^[:alnum:]_]|$)|\.animation[[:space:]]*\(|\.transition[[:space:]]*\(|UIView\.animate[[:space:]]*\(|^import[[:space:]]+(QuartzCore|CoreAnimation)$'
if find "$source_root" -type f -name '*.swift' -exec grep -En "$custom_motion_pattern" {} +; then
    fail "P0 production source contains a custom animation entry point"
fi

printf 'iOS motion verified: P0 production source uses no custom SwiftUI, UIKit, or Core Animation entry point\n'
