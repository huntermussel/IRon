#!/bin/bash
# chroma_init.sh — Initialize ChromaDB collections for IRon
#
# Usage:
#   ./scripts/chroma_init.sh        # create collections
#   ./scripts/chroma_init.sh --reset # delete and recreate collections
#   ./scripts/chroma_init.sh --help # show this help
#
# Collections created:
#   - iron_sem_cache  (semantic cache)
#   - iron_context    (RAG context)

set -euo pipefail

CHROMA_URL="${CHROMA_URL:-http://localhost:8000}"
API_VERSION="api/v1"

usage() {
    head -12 "$0" | tail -9
    exit 0
}

reset() {
    echo "Deleting collection: iron_sem_cache"
    curl -s -X DELETE "${CHROMA_URL}/${API_VERSION}/collections/iron_sem_cache" || true

    echo "Deleting collection: iron_context"
    curl -s -X DELETE "${CHROMA_URL}/${API_VERSION}/collections/iron_context" || true

    echo "Reset complete — recreating collections"
    create_collections
}

create_collections() {
    echo "Creating collection: iron_sem_cache"
    curl -s -X POST "${CHROMA_URL}/${API_VERSION}/collections" \
        -H "Content-Type: application/json" \
        -d '{"name":"iron_sem_cache","get_or_create":true}'

    echo "Creating collection: iron_context"
    curl -s -X POST "${CHROMA_URL}/${API_VERSION}/collections" \
        -H "Content-Type: application/json" \
        -d '{"name":"iron_context","get_or_create":true}'
}

main() {
    case "${1:-}" in
        --reset)  reset ;;
        --help)   usage ;;
        *)        create_collections ;;
    esac
}

main "$@"
