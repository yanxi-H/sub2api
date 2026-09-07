#!/usr/bin/env bash
# 同步官方(upstream)更新 + 推送到你的两个仓库(自动触发镜像编译)
# 用法: ./sync-official.sh
set -e

cd "$(git rev-parse --show-toplevel)"

BRANCH="main"

echo "📥 1. 拉取官方最新代码..."
git fetch upstream

echo ""
echo "🔍 2. 官方新增的提交:"
git log --oneline HEAD..upstream/main | head -20

echo ""
echo "🔀 3. 合并到 ${BRANCH}..."
git checkout "$BRANCH"
git merge upstream/main

echo ""
echo "⬆️  4. 推送到 origin (yanxi-H/sub2api)..."
git push origin "$BRANCH"

echo ""
echo "⬆️  5. 推送到 self (yanxi-H/sub2api-self, 触发镜像编译)..."
git push self "$BRANCH"

echo ""
echo "✅ 同步完成!"
echo ""
echo "接下来:"
echo "  1. 等 GitHub 编译完成: https://github.com/yanxi-H/sub2api-self/actions"
echo "  2. 服务器更新命令:"
echo "     sudo docker compose -f /root/sub2api/deploy/docker-compose.lowmem.yml pull"
echo "     sudo docker compose -f /root/sub2api/deploy/docker-compose.lowmem.yml up -d"
