#!/bin/sh

set -eu

usage() {
    cat >&2 <<'EOF'
用法：
  scripts/validate-commit-message.sh <提交信息文件>
  scripts/validate-commit-message.sh --message "type(scope): subject"
EOF
}

if [ "$#" -eq 1 ]; then
    commit_message_file=$1
    if [ ! -f "$commit_message_file" ]; then
        echo "提交信息文件不存在：$commit_message_file" >&2
        exit 2
    fi
    subject=$(sed -n '1p' "$commit_message_file")
elif [ "$#" -eq 2 ] && [ "$1" = "--message" ]; then
    subject=$2
else
    usage
    exit 2
fi

allowed_types='feat|fix|docs|refactor|perf|test|build|ci|chore|style|revert'
subject_pattern="^(${allowed_types})\\([a-z0-9]+(-[a-z0-9]+)*\\): [^[:space:]].*$"

if ! printf '%s\n' "$subject" | grep -Eq "$subject_pattern"; then
    cat >&2 <<EOF
提交信息不符合规范：$subject

必须使用：type(scope): subject
允许的 type：feat、fix、docs、refactor、perf、test、build、ci、chore、style、revert
scope：必填，只能使用小写字母、数字和单个连字符分段
冒号后：必须保留一个空格，并填写明确描述

示例：feat(wardrobe): 新增手工录入衣物入口
EOF
    exit 1
fi
