#!/bin/bash
#
# chroma_init.sh - Initialize ChromaDB collections for IRon proxy
#
# Usage:
#   ./chroma_init.sh           Create collections if they don't exist
#   ./chroma_init.sh --reset  Delete and recreate both collections
#   ./chroma_init.sh --help   Show this help message
#
# Collections created:
#   - iron_sem_cache  : Semantic cache collection
#   - iron_context     : RAG context collection
#
# Requires ChromaDB HTTP server running at http://localhost:8000
#

set -e

CHROMA_HOST="http://localhost:8000"
COLLECTIONS=("iron_sem_cache" "iron_context")

show_help() {
    head -20 "$0" | tail -17
    exit 0
}

reset_collections() {
    echo "Resetting ChromaDB collections..."
    for collection in "${COLLECTIONS[@]}"; do
        echo "Deleting collection: $collection"
        response=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "${CHROMA_HOST}/api/v1/collections/${collection}")
        if [ "$response" = "200" ]; then
            echo "  Deleted: $collection"
        elif [ "$response" = "404" ]; then
            echo "  Not found (skipping): $collection"
        else
            echo "  Warning: unexpected response code $response for $collection"
        fi
    done
    create_collections
}

create_collections() {
    echo "Creating ChromaDB collections..."
    for collection in "${COLLECTIONS[@]}"; do
        echo "Creating collection: $collection"
        response=$(curl -s -w "\n%{http_code}" -X POST "${CHROMA_HOST}/api/v1/collections" \
            -H "Content-Type: application/json" \
            -d "{\"name\":\"${collection}\",\"get_or_create\":true}")
        
        http_code=$(echo "$response" | tail -1)
        body=$(echo "$response" | sed '$d')
        
        if [ "$http_code" = "200" ] || [ "$http_code" = "201" ]; then
            echo "  Created: $collection"
        elif echo "$body" | grep -q "already exists"; then
            echo "  Already exists (skipped): $collection"
        else
            echo "  Error: $body"
            exit 1
        fi
    done
    echo "Done!"
}

# Parse arguments
case "${1:-}" in
    --help|-h|help)
        show_help
        ;;
    --reset)
        reset_collections
        ;;
    "")
        create_collections
        ;;
    *)
        echo "Unknown option: $1"
        echo "Use --help for usage information"
        exit 1
        ;;
esac
