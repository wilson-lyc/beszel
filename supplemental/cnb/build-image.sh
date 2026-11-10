#!/bin/sh
set -eu

# Called from the repository root by .cnb.yml. Authentication is supplied by CNB.
: "${CNB_DOCKER_REGISTRY:?CNB registry is required}"
: "${CNB_REPO_SLUG_LOWERCASE:?CNB repository is required}"
: "${CNB_COMMIT:?CNB commit is required}"

variant=${1:?Expected hub, agent, alpine, nvidia, nvidia-slim, or intel}
registry_path="${CNB_DOCKER_REGISTRY}/${CNB_REPO_SLUG_LOWERCASE}"
case "$variant" in
  hub)
    dockerfile=internal/dockerfile_hub
    image=$registry_path
    tag=latest
    version="sha-${CNB_COMMIT}"
    ;;
  agent|alpine)
    image="${registry_path}/beszel-agent"
    if [ "$variant" = alpine ]; then
      dockerfile=internal/dockerfile_agent_alpine
      tag=alpine
      version="sha-${CNB_COMMIT}-alpine"
    else
      dockerfile=internal/dockerfile_agent
      tag=latest
      version="sha-${CNB_COMMIT}"
    fi
    ;;
  nvidia|nvidia-slim)
    image="${registry_path}/beszel-agent-nvidia"
    if [ "$variant" = nvidia-slim ]; then
      dockerfile=internal/dockerfile_agent_nvidia_slim
      tag=slim
      version="sha-${CNB_COMMIT}-slim"
    else
      dockerfile=internal/dockerfile_agent_nvidia
      tag=latest
      version="sha-${CNB_COMMIT}"
    fi
    ;;
  intel)
    dockerfile=internal/dockerfile_agent_intel
    image="${registry_path}/beszel-agent-intel"
    tag=latest
    version="sha-${CNB_COMMIT}"
    ;;
  *) echo "Unknown image variant: $variant" >&2; exit 1 ;;
esac

# Dist must be built before invoking the Hub Dockerfile (it embeds the UI).
if [ "$variant" = hub ] && [ ! -f internal/site/dist/index.html ]; then
  echo "Build the web UI before building the Hub image." >&2
  exit 1
fi

docker buildx build \
  --platform linux/amd64 \
  --file "$dockerfile" \
  --tag "${image}:${tag}" \
  --tag "${image}:${version}" \
  --label "org.opencontainers.image.source=https://cnb.cool/${CNB_REPO_SLUG_LOWERCASE}" \
  --label "org.opencontainers.image.revision=${CNB_COMMIT}" \
  --progress plain \
  --push .

echo "Published ${image}:${tag}"
echo "Pinned version: ${image}:${version}"
