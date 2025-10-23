#!/bin/bash
# Deploy eBPF Agent to Reya Testnet GKE
set -e

COLOR_GREEN='\033[0;32m'
COLOR_BLUE='\033[0;34m'
COLOR_RED='\033[0;31m'
COLOR_YELLOW='\033[1;33m'
COLOR_RESET='\033[0m'

info() {
    echo -e "${COLOR_BLUE}ℹ ${1}${COLOR_RESET}"
}

success() {
    echo -e "${COLOR_GREEN}✓ ${1}${COLOR_RESET}"
}

error() {
    echo -e "${COLOR_RED}✗ ${1}${COLOR_RESET}"
}

warning() {
    echo -e "${COLOR_YELLOW}⚠ ${1}${COLOR_RESET}"
}

# Configuration
REGISTRY=${REGISTRY:-"gcr.io/testnet-473109"}
IMAGE_NAME="ebpf-agent"
VERSION=${VERSION:-$(date +%Y%m%d-%H%M%S)}
FULL_IMAGE="${REGISTRY}/${IMAGE_NAME}:${VERSION}"
LATEST_IMAGE="${REGISTRY}/${IMAGE_NAME}:latest"
CLUSTER_NAME="gke-de-test"
CLUSTER_ZONE="europe-west3"
PROJECT_ID="testnet-473109"

echo "========================================"
echo "  eBPF Agent Deployment to Testnet"
echo "========================================"
echo ""
info "Configuration:"
echo "  Registry: ${REGISTRY}"
echo "  Image: ${IMAGE_NAME}"
echo "  Version: ${VERSION}"
echo "  Cluster: ${CLUSTER_NAME}"
echo "  Project: ${PROJECT_ID}"
echo ""

# Step 1: Build Docker image for amd64
info "Step 1/5: Building Docker image for linux/amd64..."
docker build --platform linux/amd64 -t ${IMAGE_NAME}:${VERSION} .
docker tag ${IMAGE_NAME}:${VERSION} ${IMAGE_NAME}:latest
success "Image built successfully"
echo ""

# Step 2: Tag for GCR
info "Step 2/5: Tagging image for Google Container Registry..."
docker tag ${IMAGE_NAME}:${VERSION} ${FULL_IMAGE}
docker tag ${IMAGE_NAME}:${VERSION} ${LATEST_IMAGE}
success "Image tagged: ${FULL_IMAGE}"
success "Image tagged: ${LATEST_IMAGE}"
echo ""

# Step 3: Push to GCR
info "Step 3/5: Pushing image to GCR..."
docker push ${FULL_IMAGE}
docker push ${LATEST_IMAGE}
success "Image pushed to registry"
echo ""

# Step 4: Switch to testnet context
info "Step 4/5: Switching to testnet Kubernetes context..."
gcloud container clusters get-credentials ${CLUSTER_NAME} \
    --zone=${CLUSTER_ZONE} \
    --project=${PROJECT_ID}
success "Connected to testnet cluster"
echo ""

# Step 5: Update deployment with new image
info "Step 5/5: Deploying to Kubernetes..."

# Create temporary deployment file with the correct image
cat k8s-deployment.yaml | \
    sed "s|image: ebpf-agent:latest|image: ${FULL_IMAGE}|g" | \
    kubectl apply -f -

success "Deployment applied"
echo ""

# Wait for rollout
info "Waiting for rollout to complete..."
kubectl rollout status daemonset/ebpf-agent -n nats --timeout=120s || true
echo ""

# Show status
info "Deployment status:"
kubectl get daemonset ebpf-agent -n nats
echo ""
kubectl get pods -n nats -l app=ebpf-agent
echo ""

success "Deployment complete!"
echo ""
info "To view logs:"
echo "  kubectl logs -f daemonset/ebpf-agent -n nats"
echo ""
info "To check NATS connectivity:"
echo "  kubectl exec -it deployment/nats-0 -n nats -- nats sub \">\""
echo ""
info "Image details:"
echo "  ${FULL_IMAGE}"
echo "  ${LATEST_IMAGE}"
