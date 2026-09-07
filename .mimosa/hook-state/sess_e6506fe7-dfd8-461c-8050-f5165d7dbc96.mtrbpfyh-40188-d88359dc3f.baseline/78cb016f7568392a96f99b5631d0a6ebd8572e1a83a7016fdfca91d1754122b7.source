#!/usr/bin/env bash
# 同步官方(upstream)更新到 main 分支
# 用法: ./sync-upstream.sh
set -e

BRANCH="${1:-main}"

cd "$(git rev-parse --show-toplevel)"

echo "📥 拉取 upstream 最新代码..."
git fetch upstream

echo "🔄 切到 ${BRANCH} 并合并 upstream/${BRANCH}..."
git checkout "$BRANCH"
git merge --ff-only "upstream/${BRANCH}"

echo "⬆️  推送到你自己的 origin..."
git push origin "$BRANCH"

echo "✅ 同步完成: ${BRANCH} 已与 upstream/${BRANCH} 保持一致"
