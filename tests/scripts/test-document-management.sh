#!/bin/bash

# Exit on error and error if undefined variable is used
set -eux

# Error handling function
handle_error() {
    echo "Error occurred in test script at line $1"
    exit 1
}

# Set up error handling
trap 'handle_error $LINENO' ERR

# Function to pause and ask for user confirmation
pause_for_user() {
    echo
    echo "==> $1"
    read -p "Press Enter to continue or Ctrl+C to abort..."
    echo
}

# Test index name
INDEX_NAME="test-docs-$(date +%s)"

echo "Starting document management tests..."
pause_for_user "Will create test documents and start indexing tests"

echo "Creating test document (JSON)..."
cat > test-document.json << EOL
{
  "title": "Test Document",
  "description": "This is a test document",
  "created_at": "2023-01-01T00:00:00Z"
}
EOL

echo "Creating test document (YAML)..."
cat > test-document.yaml << EOL
title: Test Document YAML
description: This is a test document in YAML format
created_at: 2023-01-01T00:00:00Z
EOL

echo "Creating test documents for bulk indexing (JSON)..."
cat > test-documents.json << EOL
{
  "title": "Test Document 1",
  "description": "This is test document 1",
  "created_at": "2023-01-01T00:00:00Z"
}
{
  "title": "Test Document 2",
  "description": "This is test document 2",
  "created_at": "2023-01-01T00:00:00Z"
}
EOL

echo "Creating test documents for bulk indexing (YAML)..."
cat > test-documents.yaml << EOL
---
title: Test Document 3
description: This is test document 3 in YAML
created_at: 2023-01-01T00:00:00Z
---
title: Test Document 4
description: This is test document 4 in YAML
created_at: 2023-01-01T00:00:00Z
EOL

pause_for_user "Will start single document indexing tests"

echo "Testing single document indexing (JSON)..."
escuse-me documents index \
  --index "$INDEX_NAME" \
  --document test-document.json

echo "Testing single document indexing (YAML)..."
escuse-me documents index \
  --index "$INDEX_NAME" \
  --document test-document.yaml

echo "Testing single document indexing with ID (JSON)..."
escuse-me documents index \
  --index "$INDEX_NAME" \
  --id "doc1" \
  --document test-document.json

echo "Testing single document indexing with ID (YAML)..."
escuse-me documents index \
  --index "$INDEX_NAME" \
  --id "doc2" \
  --document test-document.yaml

pause_for_user "Will start bulk indexing tests"

echo "Testing bulk document indexing (JSON)..."
escuse-me documents bulk-index \
  --index "$INDEX_NAME" \
  test-documents.json

echo "Testing bulk document indexing (YAML)..."
escuse-me documents bulk-index \
  --index "$INDEX_NAME" \
  test-documents.yaml

echo "Testing bulk document indexing with options (YAML)..."
escuse-me documents bulk-index \
  --index "$INDEX_NAME" \
  --refresh wait_for \
  test-documents.yaml

pause_for_user "Will start document retrieval tests"

echo "Testing single document retrieval..."
escuse-me documents get \
  --index "$INDEX_NAME" \
  --id "doc1"

echo "Testing single document retrieval with field filtering..."
escuse-me documents get \
  --index "$INDEX_NAME" \
  --id "doc1" \
  --source-includes "title,description"

echo "Testing multi-document retrieval..."
escuse-me documents mget \
  --index "$INDEX_NAME" \
  --ids "doc1,doc2"

echo "Testing multi-document retrieval with field filtering..."
escuse-me documents mget \
  --index "$INDEX_NAME" \
  --ids "doc1,doc2" \
  --source-includes "title" \
  --source-excludes "created_at"

pause_for_user "Will start document update tests"

echo "Testing document update with script..."
escuse-me documents update \
  --index "$INDEX_NAME" \
  --id "doc1" \
  --script 'ctx._source.description = "Updated description"' \
  --lang painless

echo "Testing document update with retry on conflict..."
escuse-me documents update \
  --index "$INDEX_NAME" \
  --id "doc1" \
  --script 'ctx._source.updated_at = params.now' \
  --lang painless \
  --retry-on-conflict 3

echo "Testing document update with refresh..."
escuse-me documents update \
  --index "$INDEX_NAME" \
  --id "doc2" \
  --script 'ctx._source.description = "Another update"' \
  --refresh wait_for

pause_for_user "Will start document deletion tests"

echo "Testing single document deletion..."
escuse-me documents delete \
  --index "$INDEX_NAME" \
  --id "doc2" \
  --refresh true

echo "Testing document deletion with routing..."
escuse-me documents delete \
  --index "$INDEX_NAME" \
  --id "doc1" \
  --routing shard1 \
  --refresh wait_for

pause_for_user "Will start delete-by-query tests"

# Create some test documents for delete-by-query
echo "Creating test documents for delete-by-query..."
cat > test-documents-delete.json << EOL
{
  "title": "Delete Test 1",
  "status": "expired",
  "created_at": "2023-01-01T00:00:00Z"
}
{
  "title": "Delete Test 2",
  "status": "expired",
  "created_at": "2023-01-01T00:00:00Z"
}
{
  "title": "Keep Test",
  "status": "active",
  "created_at": "2023-01-01T00:00:00Z"
}
EOL

echo "Indexing test documents for delete-by-query..."
escuse-me documents bulk-index \
  --index "$INDEX_NAME" \
  test-documents-delete.json \
  --refresh true

echo "Testing delete-by-query..."
escuse-me documents delete-by-query \
  --index "$INDEX_NAME" \
  --query '{"match": {"status": "expired"}}' \
  --conflicts proceed \
  --refresh true

echo "Testing delete-by-query with max docs..."
escuse-me documents delete-by-query \
  --index "$INDEX_NAME" \
  --query '{"match_all": {}}' \
  --max-docs 2 \
  --requests-per-second 100 \
  --refresh

pause_for_user "Tests completed. Will clean up test files"

echo "Cleanup..."
rm -f test-document.json test-document.yaml test-documents.json test-documents.yaml test-documents-delete.json

echo "All document management tests completed successfully!"
