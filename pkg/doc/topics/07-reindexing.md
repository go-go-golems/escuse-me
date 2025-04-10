---
Title: Reindexing Elasticsearch Data with escuse-me
Slug: reindexing
Short: Guide to using the reindex command for copying, transforming, and managing index data.
Topics:
  - elasticsearch
  - indices
  - reindexing
  - tasks
  - aliases
Commands:
  - indices reindex
Flags:
  - source-index
  - target-index
  - query
  - script
  - pipeline
  - batch-size
  - slices
  - requests-per-second
  - wait-for-completion
  - poll-interval
  - create-target
  - target-settings
  - target-mappings
  - swap-alias
  - timeout
  - request-timeout
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

# Reindexing Elasticsearch Data with escuse-me

This document explains how to use the `escuse-me indices reindex` command to copy documents between Elasticsearch indices. This process is crucial for managing your data effectively, especially when you need to apply changes or restructure your indices.

## What is Reindexing?

Reindexing in Elasticsearch is essentially the process of copying documents from an existing index (the "source") into another index (the "destination" or "target"). Think of it like making a processed copy of your data from one table or database to another.

**Why is it necessary?** Many core aspects of an Elasticsearch index, such as its fundamental structure (mappings, like field types) or certain performance settings (like the number of primary shards), cannot be changed after the index is created. If you need to make such changes – perhaps you decided a text field should have been analyzed differently, or you want to optimize shard allocation – you can't modify the existing index directly. Instead, you create a _new_ index with the desired structure and then copy the data from the old index into the new one using reindexing.

Common scenarios requiring reindexing include:

- **Changing Mappings or Settings:** The most frequent reason. You might need to change a field's type (e.g., from `text` to `keyword`), add a new analyzer for better search relevance, or adjust the number of shards (though this requires more advanced techniques usually involving reindexing into a new index with the desired shard count).
- **Upgrading Indices:** When moving to a new major version of Elasticsearch, the underlying data format might change, requiring you to reindex data from the old version's indices into new indices compatible with the upgraded version.
- **Consolidating Indices:** You might have data spread across multiple indices (e.g., daily logs) and want to combine them into a single, larger index for easier querying or retention management.
- **Filtering or Transforming Data:** You can use reindexing to create a new index that contains only a subset of the original data (e.g., only active users) or where the documents have been modified or enriched during the copy process (e.g., adding a new field, renaming an existing one).

The `escuse-me indices reindex` command provides a user-friendly interface to Elasticsearch's built-in `_reindex` API. It simplifies the process by handling API calls, optionally monitoring the long-running reindex task, providing progress updates, and allowing for actions like swapping index aliases upon completion.

## The `reindex` Command

The primary command for this functionality is `escuse-me indices reindex`.

### Core Functionality

At its simplest, the command takes a `--source-index` and a `--target-index` and tells Elasticsearch to start copying documents. A key feature of the `escuse-me` command is its default behavior: it runs the reindex **asynchronously**. This means the command tells Elasticsearch to start the job in the background and then immediately begins monitoring its progress.

Elasticsearch manages long-running operations like reindexing as "Tasks". The `escuse-me` command gets the ID of this background task and periodically asks Elasticsearch (using the Tasks API) for status updates. These updates are then displayed, typically as streaming output (which you can format as tables, JSON, etc., using standard `glazed` flags like `-o table --output-mode stream`). This gives you real-time insight into how the reindex is progressing without requiring the command itself to stay connected for the entire duration (which could be hours for large indices).

```bash
# Basic asynchronous reindex (command monitors progress in the background)
escuse-me indices reindex --source-index old-logs --target-index new-logs

# View progress as a nicely formatted table
# The --output-mode stream is important for seeing updates as they happen
escuse-me indices reindex --source-index old-logs --target-index new-logs -o table --output-mode stream
```

This is different from the `escuse-me indices clone` command, which uses filesystem-level hard links (where possible) for a near-instantaneous copy but _cannot_ change mappings or filter data during the process. Reindexing reads each document and writes it again, making it more resource-intensive but far more flexible.

### Common Use Cases & Examples

Let's explore how to use the `reindex` command for various common tasks.

#### 1. Simple Copy to Existing Index

This is the most basic scenario: you have an existing `target-index` (perhaps created manually beforehand) and want to populate it with data from `source-index`.

```bash
# Assumes 'my-target-backup' index already exists
escuse-me indices reindex \
  --source-index my-source-data \
  --target-index my-target-backup
```

#### 2. Create Target Index During Reindex

Often, you reindex because you need a _new_ index structure. The `--create-target` flag lets you create this new index as part of the reindex command itself. You'll typically provide the desired **settings** (like shard/replica counts) and **mappings** (field definitions) in JSON or YAML files.

```bash
# settings.json: Controls physical aspects like shards and replicas
echo '{"index": {"number_of_shards": 3, "number_of_replicas": 1}}' > settings.json

# mappings.json: Defines fields and their types/analyzers
echo '{"properties": {"message": {"type": "text", "analyzer": "english"}, "timestamp": {"type": "date"}}}' > mappings.json

# Reindex from 'old-index' and create 'newly-structured-index' on the fly
escuse-me indices reindex \
  --source-index old-index \
  --target-index newly-structured-index \
  --create-target \
  --target-settings settings.json \
  --target-mappings mappings.json
```

_Note: Using `--create-target` requires you to provide the structure via `--target-settings` and/or `--target-mappings`._

#### 3. Filtering Documents

Sometimes you only want a subset of the original data. You can provide an Elasticsearch query (in a file) using the `--query` flag. Only documents matching this query will be copied.

```bash
# query.json: Contains the filter logic
echo '{"query": {"term": {"status": "active"}}}' > query.json

# Copy only active users to the new index
escuse-me indices reindex \
  --source-index all-users \
  --target-index active-users \
  --query query.json
```

#### 4. Transforming Documents with a Script

Reindexing allows you to modify documents as they are copied. The `--script` flag takes a script (usually written in Elasticsearch's "Painless" language) that runs for each document. You can add, remove, or modify fields.

```bash
# transform.json: Contains the script logic
cat <<EOF > transform.json
{
  "source": "ctx._source.new_field = ctx._source.remove('old_field'); ctx._source.reindexed_at = params.now;",
  "lang": "painless",
  "params": {
    "now": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  }
}
EOF
# This script:
# 1. Removes 'old_field' and puts its value into 'new_field'.
# 2. Adds a 'reindexed_at' field using a timestamp passed as a parameter.

escuse-me indices reindex \
  --source-index legacy-data \
  --target-index modern-data \
  --script transform.json
```

#### 5. Transforming Documents with an Ingest Pipeline

For more complex or reusable transformations (like enriching data with lookups, parsing logs, etc.), Elasticsearch offers "Ingest Pipelines". If you have defined such a pipeline in your cluster, you can tell reindex to use it via the `--pipeline` flag.

```bash
# Assumes a pipeline named 'my-enrichment-pipeline' is already defined in Elasticsearch
escuse-me indices reindex \
  --source-index raw-logs \
  --target-index enriched-logs \
  --pipeline my-enrichment-pipeline
```

#### 6. Controlling Performance

Reindexing large amounts of data can be resource-intensive. These flags help manage performance:

- `--batch-size`: How many documents Elasticsearch reads from the source in one go. Larger batches can be more efficient but use more memory.
- `--slices`: Splits the read operation into multiple parallel "slices" (like threads) for faster processing on clusters with sufficient resources. `slices > 1` enables parallelism.
- `--requests-per-second`: Throttles how many write requests are sent to the target index per second. Useful to prevent overwhelming the cluster. `-1` means no throttling.

```bash
# Example: Faster processing with larger batches and parallelism, but throttled writes
escuse-me indices reindex \
  --source-index large-source \
  --target-index large-target \
  --batch-size 5000 \
  --slices 4 \
  --requests-per-second 500 # Limit writes to 500 docs/sec
```

#### 7. Running Synchronously

While async monitoring is often preferred for large jobs, you can make the command wait until the entire reindex operation is finished using `--wait-for-completion`. The command will block until Elasticsearch reports completion (or failure) and then output the final summary.

```bash
# Use this for smaller indices or when scripting reindex as part of a larger process
escuse-me indices reindex \
  --source-index small-config \
  --target-index updated-config \
  --wait-for-completion
```

#### 8. Atomic Alias Swap

An **alias** in Elasticsearch is like a symbolic link or pointer to one or more indices. Applications often query an alias (e.g., `live-data`) instead of a specific index name (e.g., `data-v1`). This allows you to switch the underlying index without changing your application code.

The `--swap-alias` flag leverages this for zero-downtime updates. If the reindex completes successfully, the command will atomically perform two actions: remove the specified alias from the `--source-index` and add it to the `--target-index`. Your application querying the alias will seamlessly start hitting the newly reindexed data.

```bash
# 1. Reindex data from data-v1 to data-v2
# 2. If successful, atomically switch the 'live-data' alias to point to data-v2
escuse-me indices reindex \
  --source-index data-v1 \
  --target-index data-v2 \
  --create-target --target-settings settings.json --target-mappings mappings.json \
  --swap-alias live-data
```

### Monitoring Progress

In the default asynchronous mode, you'll see status updates containing fields like:

- `task_id`: The identifier for the background reindex job in Elasticsearch.
- `completed`: `true` if the task finished (successfully or with errors), `false` otherwise.
- `poll_status`: Status reported by `escuse-me` monitor (`in_progress`, `success`, `task_error`, `api_error`).
- `running_time_ns`: How long the task has been running.
- `status_total`: The total number of documents expected to be processed.
- `status_created`, `status_updated`, `status_deleted`: Counts of documents processed.
- `status_batches`: Number of batches processed so far.
- `status_version_conflicts`: Number of documents skipped due to version conflicts (see `op_type: "create"` note below).
- `status_noops`: Documents that didn't require any change.
- `final_*`: Details from the final task result upon successful completion.
- `error_*`: Details if the task failed.
- `@timestamp`: When the status update was generated.

Use glazed flags (`-o table --output-mode stream`, `-o json`, etc.) to control the output format.

### Options Overview

**Source & Target:**

- `--source-index` (required): Index/indices to read from.
- `--target-index` (required): Index to write to.
- `--create-target`: Create the target index if absent.
- `--target-settings`: Structure (shards, replicas) for the new index (JSON/YAML file).
- `--target-mappings`: Field definitions for the new index (JSON/YAML file).

**Filtering & Transformation:**

- `--query`: Filter documents using an Elasticsearch query (JSON/YAML file).
- `--script`: Modify documents using a script (JSON/YAML file containing `source`, `lang`, `params`).
- `--pipeline`: Apply a pre-defined Elasticsearch ingest pipeline.

**Performance & Throttling:**

- `--batch-size` (default: 1000): Number of docs per read batch.
- `--slices` (default: 1): Enable parallel reads (`>1`).
- `--requests-per-second` (default: -1): Throttle writes (-1 = unlimited).

**Control Flow & Monitoring:**

- `--wait-for-completion` (default: false): Run synchronously (blocks until done).
- `--poll-interval` (default: 5s): Check frequency for async monitoring.
- `--timeout` (default: 1m): Timeout for the overall coordination.

**Alias Management:**

- `--swap-alias`: Alias name to atomically switch from source to target upon success.

### Important Notes

- **Resource Intensive:** Reindexing reads and writes data, unlike `clone`. It can heavily load your cluster (CPU, I/O, memory). Monitor your cluster, especially during large operations, and consider using `--requests-per-second` throttling.
- **`op_type: "create"`:** By default, `escuse-me indices reindex` tells Elasticsearch to only _create_ documents in the target index. If a document with the same ID already exists there, it causes a "version conflict" and the document is skipped (counted in `status_version_conflicts`). This is often the safest approach to avoid accidental overwrites. If you intend to overwrite or update existing documents in the target, you would need to use the Elasticsearch API directly with a different `op_type` or potentially use an update script.
- **Failures:** Individual documents might fail during reindexing (e.g., due to script errors or mapping conflicts). The task might complete with failures reported. Check the final status or error messages carefully. The command might return an error status even if _some_ documents were successfully processed.
