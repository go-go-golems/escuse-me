#!/bin/bash

# Exit on error
set -e

# Test index name
INDEX_NAME="test-index-$(date +%s)"

echo "Creating initial mappings file..."
cat > initial-mappings.yaml << EOL
properties:
  title:
    type: text
    fields:
      keyword:
        type: keyword
  description:
    type: text
  created_at:
    type: date
EOL

echo "Creating updated mappings file..."
cat > updated-mappings.yaml << EOL
properties:
  title:
    type: text
    fields:
      keyword:
        type: keyword
  description:
    type: text
  created_at:
    type: date
  tags:
    type: keyword
  priority:
    type: integer
EOL

echo "Creating index with initial mappings..."
escuse-me indices create --index "$INDEX_NAME" --mappings initial-mappings.yaml

echo "Getting initial mappings..."
escuse-me indices mappings --index "$INDEX_NAME"

echo "Updating mappings..."
escuse-me indices update-mapping --index "$INDEX_NAME" --mappings updated-mappings.yaml

echo "Getting updated mappings..."
escuse-me indices mappings --index "$INDEX_NAME"

# Clean up
rm initial-mappings.yaml updated-mappings.yaml

echo "Test completed successfully!"
