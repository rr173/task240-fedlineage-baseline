#!/usr/bin/env bash
# 用法：bash build_benzhi_docker.sh <镜像名> <平台>
# 平台示例：linux/amd64 或 linux/arm64
set -euo pipefail

IMAGE_NAME="${1:?usage: build_benzhi_docker.sh <镜像名> <平台>}"
PLATFORM="${2:?usage: build_benzhi_docker.sh <镜像名> <平台>}"

cd "$(dirname "$0")"

echo "==> building ${IMAGE_NAME} for ${PLATFORM} (benzhi.Dockerfile)"
docker buildx build \
  --platform "${PLATFORM}" \
  -f benzhi.Dockerfile \
  -t "${IMAGE_NAME}" \
  --load \
  .

echo "==> running --smoke-test on ${IMAGE_NAME}"
docker run --rm --platform "${PLATFORM}" "${IMAGE_NAME}" --smoke-test

echo "==> done: ${IMAGE_NAME} (${PLATFORM})"
