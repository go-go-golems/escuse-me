---
Title: Managing Elasticsearch Indices with escuse-me
Slug: index-management
Short: Learn how to create indices, update mappings, and manage index settings using escuse-me's command-line interface
Topics:
- elasticsearch
- indices
- mappings
- index management
Commands:
- indices create
- indices update-mapping
- indices mappings
Flags:
- index
- mappings
- settings
- aliases
- write_index_only
- allow_no_indices
- expand_wildcards
- ignore_unavailable
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

# Index Management in escuse-me

This document describes how to manage Elasticsearch indices using escuse-me's command-line interface.

## Creating an Index

Use the `create` command to create a new index. The command supports setting up the index with custom settings, mappings, and aliases.

```bash
# Create a simple index
escuse-me indices create --index my-index

# Create an index with custom mappings from a JSON or YAML file
escuse-me indices create --index my-index --mappings mappings.json

# Create an index with both settings and mappings (both JSON and YAML formats are supported)
escuse-me indices create --index my-index --settings settings.yaml --mappings mappings.json

# Create an index with aliases (JSON or YAML format)
escuse-me indices create --index my-index --aliases aliases.yaml
```

### Options for create command:
- `--index`: (Required) Name of the index to create
- `--settings`: JSON or YAML file containing index settings
- `--mappings`: JSON or YAML file containing index mappings
- `--aliases`: JSON or YAML file containing index aliases
- `--wait_for_active_shards`: Set the number of active shards to wait for before the operation returns

## Updating Mappings

The `update-mapping` command allows you to update the mapping of an existing index.

```bash
# Update mapping for a single index
escuse-me indices update-mapping --index my-index --mappings new-mappings.json

# Update mapping with additional options
escuse-me indices update-mapping \
  --index my-index \
  --mappings new-mappings.json \
  --allow_no_indices \
  --expand_wildcards open,closed

# Update mapping for a single index (JSON or YAML file)
escuse-me indices update-mapping --index my-index --mappings new-mappings.yaml
```

### Options for update-mapping command:
- `--index`: (Required) Name of the index to update mapping for
- `--mappings`: (Required) JSON or YAML file containing updated index mappings
- `--write_index_only`: If true, the mappings are applied only to the current write index
- `--allow_no_indices`: Whether to ignore if a wildcard expression matches no indices (default: true)
- `--expand_wildcards`: Whether to expand wildcard expression to concrete indices that are open, closed or both (default: ["open", "closed"])
- `--ignore_unavailable`: Whether specified concrete indices should be ignored when unavailable

## Getting Mappings

Use the `mappings` command to view the current mappings of one or more indices.

```bash
# Get mappings for a single index
escuse-me indices mappings --index my-index

# Get mappings for multiple indices
escuse-me indices mappings --index "index1,index2"

# Get full mapping response
escuse-me indices mappings --index my-index --full

# Get mappings with wildcard patterns
escuse-me indices mappings --index "my-*" --expand_wildcards open
```

### Options for mappings command:
- `--index`: (Required) The index or indices to get mappings for
- `--full`: Prints the full version response (default: false)
- `--allow_no_indices`: Whether to ignore if a wildcard expression matches no indices (default: true)
- `--expand_wildcards`: Whether to expand wildcard expression to concrete indices (default: ["open", "closed"])
- `--ignore_unavailable`: Whether to ignore unavailable indices
- `--local`: Return local information, do not retrieve the state from master node

## Example Workflow

Here's a complete example of creating an index with custom mappings and then updating them:

1. First, create a mappings file (`mappings.json`):
```json
{
  "properties": {
    "title": {
      "type": "text",
      "fields": {
        "keyword": {
          "type": "keyword"
        }
      }
    },
    "description": {
      "type": "text"
    },
    "created_at": {
      "type": "date"
    }
  }
}
```

2. Create the index with initial mappings:
```bash
escuse-me indices create --index my-index --mappings mappings.json
```

3. Later, create an updated mapping file (`new-mappings.yaml`):
```yml
properties:
  tags:
    type: keyword
```

4. Update the existing index with the new mapping:
```bash
escuse-me indices update-mapping --index my-index --mappings new-mappings.yaml
```

5. Verify the updated mappings:
```bash
escuse-me indices mappings --index my-index
```

Note: When updating mappings, you can only add new fields or modify certain field settings. You cannot remove fields or make incompatible changes to existing field mappings.
