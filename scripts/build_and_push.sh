#!/bin/bash
# Build and push eBPF Agent Docker image
set -e

# Configuration
REGISTRY=${REGISTRY:-"gcr.io/testnet-473109"}
IMAGE_NAME="ebpf-agent"
VERSION=${VERSION:-$(date +%Y%m%d-%H%M%S)}

echo "Building and pushing ${REGISTRY}/${IMAGE_NAME}:${VERSION}"
echo ""

# Build for amd64
echo "🔨 Building for linux/amd64..."
docker build --platform linux/amd64 -t ${IMAGE_NAME}:${VERSION} .

# Tag
echo "🏷️  Tagging images..."
docker tag ${IMAGE_NAME}:${VERSION} ${REGISTRY}/${IMAGE_NAME}:${VERSION}
docker tag ${IMAGE_NAME}:${VERSION} ${REGISTRY}/${IMAGE_NAME}:latest

# Push
echo "📤 Pushing to registry..."
docker push ${REGISTRY}/${IMAGE_NAME}:${VERSION}
docker push ${REGISTRY}/${IMAGE_NAME}:latest

echo ""
echo "✅ Done!"
echo ""
echo "Image: ${REGISTRY}/${IMAGE_NAME}:${VERSION}"
echo "Latest: ${REGISTRY}/${IMAGE_NAME}:latest"
