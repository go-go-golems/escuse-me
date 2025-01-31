---
Title: Managing Elasticsearch Documents with escuse-me
Slug: document-management
Short: Learn how to index, bulk-index, retrieve, update, and delete documents using escuse-me's command-line interface
Topics:
- elasticsearch
- documents
- indexing
- bulk operations
Commands:
- documents index
- documents bulk-index
- documents get
- documents mget
- documents update
- documents delete
- documents delete-by-query
Flags:
- index
- id
- op_type
- pipeline
- refresh
- routing
- version
- version_type
- wait_for_active_shards
- require_alias
- document
- preference
- realtime
- _source_includes
- _source_excludes
- stored_fields
- script
- lang
- retry_on_conflict
- if_seq_no
- if_primary_term
- conflicts
- max_docs
- requests_per_second
- slices
- scroll_size
- wait_for_completion
- timeout
- flatten_source
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

# Document Management in escuse-me

This document describes how to manage Elasticsearch documents using escuse-me's command-line interface.

## Indexing Documents

### Single Document Indexing

Use the `index` command to index a single document. The command supports various indexing options and document metadata.

```bash
# Index a document with automatic ID generation (JSON)
escuse-me documents index --index my-index --document '{"field": "value"}'

# Index a document from a YAML file
escuse-me documents index --index my-index --document document.yaml

# Index a document with a specific ID (JSON or YAML)
escuse-me documents index --index my-index --id doc1 --document '{"field": "value"}'

# Index with additional options
escuse-me documents index --index my-index \
  --id doc1 \
  --routing shard1 \
  --pipeline my-pipeline \
  --refresh wait_for \
  --document document.yaml
```

### Options for index command:

- `--index` (required): Name of the data stream or index to target
- `--id`: Unique identifier for the document (auto-generated if not specified)
- `--op_type`: Operation type (index or create)
- `--pipeline`: Pipeline to process the document
- `--refresh`: Refresh policy (true, false, wait_for)
- `--routing`: Custom routing value
- `--version`: Version number for optimistic concurrency control
- `--version_type`: Version type (internal, external, external_gte)
- `--wait_for_active_shards`: Number of active shards required before proceeding
- `--require_alias`: Whether the target must be an alias
- `--document`: The document content in JSON or YAML format

### Bulk Indexing

Use the `bulk-index` command to index multiple documents in a single request, which is more efficient than indexing documents individually.

```bash
# Bulk index documents from a JSON file
escuse-me documents bulk-index --index my-index --files documents.json

# Bulk index documents from a YAML file
escuse-me documents bulk-index --index my-index --files documents.yaml

# Bulk index with specific options
escuse-me documents bulk-index --index my-index \
  --pipeline my-pipeline \
  --refresh wait_for \
  --routing shard1 \
  --files documents.yaml
```

### Options for bulk-index command:

- `--index` (required): Target index for the documents
- `--pipeline`: Pipeline to process the documents
- `--refresh`: Refresh policy (true, false, wait_for)
- `--routing`: Custom routing value
- `--source`: Fields to include in the response
- `--full-source`: Whether to return the full source
- `--source_excludes`: Fields to exclude from the response
- `--source_includes`: Fields to include in the response
- `--wait_for_active_shards`: Number of active shards required before proceeding
- `--require_alias`: Whether the target must be an alias
- `--files`: List of documents to index (in JSON or YAML format)

## Retrieving Documents

### Single Document Retrieval

Use the `get` command to retrieve a single document by its ID. The command supports various options to control what data is returned and how it is retrieved.

```bash
# Retrieve a document by ID
escuse-me documents get --index my-index --id doc1

# Retrieve specific fields from a document
escuse-me documents get --index my-index --id doc1 --_source_includes "field1,field2"

# Retrieve a document excluding certain fields
escuse-me documents get --index my-index --id doc1 --_source_excludes "field3,field4"

# Retrieve a document with additional options
escuse-me documents get --index my-index \
  --id doc1 \
  --routing shard1 \
  --realtime true \
  --refresh true

# Retrieve a document and flatten its _source fields
escuse-me documents get --index "my-index" --id "my-document-id" --flatten_source
```

### Options for get command:

- `--index` (required): Name of the index to retrieve from
- `--id` (required): Document ID to retrieve
- `--preference`: Specify which shard replicas to execute the get request on
- `--realtime`: Whether to perform a realtime get or wait for a refresh
- `--refresh`: Whether to refresh the shard before getting the document
- `--routing`: Custom routing value
- `--_source_includes`: List of source fields to include
- `--_source_excludes`: List of source fields to exclude
- `--version`: Version number for optimistic concurrency control
- `--version_type`: Version type (internal, external, external_gte)
- `--flatten_source`: When set to true, flattens the _source fields into the root of the response instead of keeping them nested under _source

### Multi-Document Retrieval

Use the `mget` command to retrieve multiple documents in a single request. This is more efficient than making multiple individual get requests.

```bash
# Retrieve multiple documents by ID
escuse-me documents mget --index my-index --ids "doc1,doc2,doc3"

# Retrieve specific fields from multiple documents
escuse-me documents mget --index my-index \
  --ids "doc1,doc2,doc3" \
  --stored_fields "field1,field2"

# Retrieve multiple documents with field filtering
escuse-me documents mget --index my-index \
  --ids "doc1,doc2,doc3" \
  --_source_includes "field1,field2" \
  --_source_excludes "field3,field4"

# Retrieve multiple documents and flatten their _source fields
escuse-me documents mget --index "my-index" --ids "id1,id2,id3" --flatten_source
```

### Options for mget command:

- `--index`: Name of the index to retrieve from
- `--ids` (required): Comma-separated list of document IDs to retrieve
- `--preference`: Specify which shard replicas to execute the get request on
- `--realtime`: Whether to perform a realtime get or wait for a refresh
- `--refresh`: Whether to refresh the shard before getting the documents
- `--routing`: Custom routing value
- `--stored_fields`: List of stored fields to retrieve
- `--_source`: List of source fields to return
- `--_source_includes`: List of source fields to include
- `--_source_excludes`: List of source fields to exclude
- `--flatten_source`: When set to true, flattens the _source fields into the root of the response instead of keeping them nested under _source

Note: Both commands support documents in either JSON or YAML format. When using the commands in scripts, you may need to properly escape the strings if providing inline document content.

Note: The mget command is particularly useful when you need to retrieve multiple documents efficiently, as it reduces network overhead by combining multiple get requests into a single request.

## Updating Documents

Use the `update` command to modify existing documents. The command supports script-based updates and provides various options for handling conflicts and controlling the update process.

```bash
# Update a document using a script
escuse-me documents update \
  --index my-index \
  --id doc1 \
  --script 'ctx._source.counter += 1' \
  --lang painless

# Update with retry on conflict
escuse-me documents update \
  --index my-index \
  --id doc1 \
  --script 'ctx._source.timestamp = params.now' \
  --lang painless \
  --retry_on_conflict 3

# Update with optimistic concurrency control
escuse-me documents update \
  --index my-index \
  --id doc1 \
  --script 'ctx._source.field = "new value"' \
  --if_seq_no 123 \
  --if_primary_term 456

# Update with refresh control
escuse-me documents update \
  --index my-index \
  --id doc1 \
  --script 'ctx._source.field = "new value"' \
  --refresh wait_for
```

### Options for update command:

- `--index` (required): Name of the target index
- `--id` (required): Document ID to update
- `--script`: Update script to execute
- `--lang`: Script language (default is painless)
- `--retry_on_conflict`: Number of times to retry the update in case of version conflicts
- `--refresh`: When to make the update visible (true, false, wait_for)
- `--wait_for_active_shards`: Number of active shards that must acknowledge the update
- `--if_seq_no`: Only update if the document has the specified sequence number
- `--if_primary_term`: Only update if the document has the specified primary term
- `--require_alias`: Whether the target index must be an alias
- `--_source`: List of source fields to return in the response
- `--_source_excludes`: List of source fields to exclude from the response
- `--_source_includes`: List of source fields to include in the response

Note: When using scripts, make sure they are properly escaped in your shell. The script language defaults to 'painless', which is Elasticsearch's built-in scripting language. For complex updates, consider storing your scripts in files and using script parameters.

## Deleting Documents

### Single Document Deletion

Use the `delete` command to remove a specific document by its ID. The command supports various options for controlling the deletion process and ensuring consistency.

```bash
# Delete a document by ID
escuse-me documents delete --index my-index --id doc1

# Delete with routing
escuse-me documents delete \
  --index my-index \
  --id doc1 \
  --routing shard1

# Delete with optimistic concurrency control
escuse-me documents delete \
  --index my-index \
  --id doc1 \
  --if_seq_no 123 \
  --if_primary_term 456

# Delete with refresh control
escuse-me documents delete \
  --index my-index \
  --id doc1 \
  --refresh wait_for
```

### Options for delete command:

- `--index` (required): Name of the index
- `--id` (required): Document ID to delete
- `--routing`: Custom routing value
- `--refresh`: When to make the deletion visible (true, false, wait_for)
- `--if_seq_no`: Only delete if the document has the specified sequence number
- `--if_primary_term`: Only delete if the document has the specified primary term
- `--wait_for_active_shards`: Number of active shards that must acknowledge the deletion

### Bulk Deletion by Query

Use the `delete-by-query` command to delete multiple documents that match a specific query. This is useful for bulk deletions based on document content rather than IDs.

```bash
# Delete all documents matching a query
escuse-me documents delete-by-query \
  --index my-index \
  --query '{"match": {"status": "expired"}}'

# Delete with conflict handling
escuse-me documents delete-by-query \
  --index my-index \
  --query '{"range": {"age": {"lt": 18}}}' \
  --conflicts proceed

# Delete with rate limiting and size control
escuse-me documents delete-by-query \
  --index my-index \
  --query '{"term": {"processed": false}}' \
  --requests_per_second 100 \
  --max_docs 1000

# Delete from multiple indices
escuse-me documents delete-by-query \
  --index "index1,index2" \
  --query '{"match_all": {}}' \
  --routing "shard1,shard2" \
  --refresh true
```

### Options for delete-by-query command:

- `--index` (required): Comma-separated list of indices to delete from
- `--query` (required): Query to match documents for deletion (JSON format)
- `--conflicts`: How to handle version conflicts (abort or proceed)
- `--max_docs`: Maximum number of documents to delete
- `--requests_per_second`: Rate limit for delete operations
- `--slices`: Number of slices to divide the deletion into for parallelism
- `--scroll_size`: Number of documents to process per batch
- `--wait_for_completion`: Whether to wait for the deletion to complete
- `--refresh`: Whether to refresh the affected shards
- `--routing`: Custom routing values
- `--timeout`: Operation timeout
- `--wait_for_active_shards`: Number of active shards required

Note: The delete-by-query command is particularly useful for bulk deletions based on document content. It supports complex queries and can be configured to handle large-scale deletions efficiently. Use the `--conflicts proceed` option if you want the operation to continue even when version conflicts are encountered.
