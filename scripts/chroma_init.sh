#!/bin/bash
#
# chroma_init.sh - Initialize ChromaDB collections for IRon proxy
#
# Usage:
#   ./chroma_init.sh              # Create collections if they don't exist
#   ./chroma_init.sh --reset      # Delete and recreate collections
#   ./chroma_init.sh --check      # Check if collections exist
#
# Requirements:
#   - ChromaDB server running at http://localhost:8000 (or set CHROMA_URL)
#   - curl installed
#

set -e

CHROMA_URL="${CHROMA_URL:-http://localhost:8000}"
CACHE_COLLECTION="iron_sem_cache"
RAG_COLLECTION="iron_context"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

error() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $1" >&2
}

check_chroma() {
    if ! curl -s "${CHROMA_URL}/api/v1/heartbeat" > /dev/null 2>&1; then
        error "ChromaDB is not available at ${CHROMA_URL}"
        error "Please start ChromaDB first: docker run -p 8000:8000 chromadb/chroma"
        exit 1
    fi
    log "ChromaDB is available at ${CHROMA_URL}"
}

collection_exists() {
    local name="$1"
    local response
    response=$(curl -s "${CHROMA_URL}/api/v1/collections/${name}")
    if echo "$response" | grep -q "error"; then
        return 1
    fi
    return 0
}

delete_collection() {
    local name="$1"
    log "Deleting collection: ${name}"
    curl -X DELETE "${CHROMA_URL}/api/v1/collections/${name}" 2>/dev/null
    log "Deleted collection: ${name}"
}

create_collection() {
    local name="$1"
    local metadata="$2"
    
    if collection_exists "$name"; then
        log "Collection '${name}' already exists, skipping creation"
        return 0
    fi
    
    log "Creating collection: ${name}"
    local response
    response=$(curl -s -X POST "${CHROMA_URL}/api/v1/collections" \
        -H "Content-Type: application/json" \
        -d "{\"name\":\"${name}\", \"metadata\": ${metadata}}")
    
    if echo "$response" | grep -q "error"; then
        error "Failed to create collection '${name}': ${response}"
        return 1
    fi
    
    log "Created collection: ${name}"
}

create_collections() {
    log "Creating ChromaDB collections..."
    
    # iron_sem_cache: for semantic caching of responses
    create_collection "$CACHE_COLLECTION" '{"description": "Semantic cache for IRon proxy responses", "type": "cache"}'
    
    # iron_context: for RAG context retrieval
    create_collection "$RAG_COLLECTION" '{"description": "RAG context collection for IRon proxy", "type": "rag"}'
    
    log "All collections created successfully"
}

reset_collections() {
    log "Resetting ChromaDB collections..."
    
    if collection_exists "$CACHE_COLLECTION"; then
        delete_collection "$CACHE_COLLECTION"
    else
        log "Collection '${CACHE_COLLECTION}' does not exist, skipping deletion"
    fi
    
    if collection_exists "$RAG_COLLECTION"; then
        delete_collection "$RAG_COLLECTION"
    else
        log "Collection '${RAG_COLLECTION}' does not exist, skipping deletion"
    fi
    
    create_collections
}

check_collections() {
    log "Checking ChromaDB collections..."
    
    if collection_exists "$CACHE_COLLECTION"; then
        log "✓ Collection '${CACHE_COLLECTION}' exists"
    else
        log "✗ Collection '${CACHE_COLLECTION}' does not exist"
    fi
    
    if collection_exists "$RAG_COLLECTION"; then
        log "✓ Collection '${RAG_COLLECTION}' exists"
    else
        log "✗ Collection '${RAG_COLLECTION}' does not exist"
    fi
}

usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --reset   Delete and recreate all collections"
    echo "  --check   Check if collections exist"
    echo "  --help    Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  CHROMA_URL    ChromaDB server URL (default: http://localhost:8000)"
}

main() {
    case "${1:-}" in
        --reset)
            check_chroma
            reset_collections
            ;;
        --check)
            check_chroma
            check_collections
            ;;
        --help)
            usage
            exit 0
            ;;
        "")
            check_chroma
            create_collections
            ;;
        *)
            error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
}

main "$@"
