---
Title: Reindexing with Mapping Changes and Alias Swapping
Slug: reindexing-tutorial
Short: Tutorial on reindexing Elasticsearch data with mapping changes and zero downtime using aliases.
Topics:
  - elasticsearch
  - reindexing
  - aliases
  - mapping
  - zero-downtime
Commands:
  - indices create
  - documents bulk-index
  - indices dump
  - indices create-alias
  - indices aliases
  - indices reindex
  - indices update-aliases
  - indices delete
Flags:
  - --index
  - --mappings
  - --files
  - -o
  - --fields
  - --name
  - --source-index
  - --target-index
  - --script
  - --wait-for-completion
  - --actions
IsTopLevel: false
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
---

# Tutorial: Reindexing with Mapping Changes and Alias Swapping using escuse-me

This tutorial demonstrates a common Elasticsearch workflow: changing the mapping of an existing field, which requires creating a new index and reindexing the data. We will use an alias to ensure applications querying the data experience zero downtime during the switch.

**Scenario:**

We have an index (`my-data-v1`) containing documents with a `metadata` field mapped as an `object`. We realize we need this field to be analyzed as `text` instead. Since existing field mappings cannot be changed, we'll:

1.  Create the initial index (`my-data-v1`) with the `object` mapping.
2.  (Assume data is indexed into `my-data-v1`).
3.  Create an alias (`my-data-alias`) pointing to `my-data-v1`.
4.  Define and create a new index (`my-data-v2`) with the `metadata` field mapped as `text`.
5.  Reindex the data from `my-data-v1` to `my-data-v2`.
6.  Atomically update the alias (`my-data-alias`) to point to the new index (`my-data-v2`).

**Prerequisites:**

- `escuse-me` CLI installed and configured to connect to your Elasticsearch cluster.
- Familiarity with basic JSON and shell commands.

---

## Step 1: Create the Initial Index (v1)

First, define the mappings for our initial index, `my-data-v1`. Note the `metadata` field is of type `object`.

**`mappings-v1.json`:**

```json
{
  "properties": {
    "id": { "type": "integer" },
    "user": { "type": "keyword" },
    "timestamp": { "type": "date" },
    "metadata": {
      "type": "object"
    }
  }
}
```

Now, create the index using the `create` command:

```bash
escuse-me indices create \
  --index my-data-v1 \
  --mappings mappings-v1.json
```

You should see confirmation that the index was created.

---

## Step 2: Index Sample Data

This step involves populating `my-data-v1` with documents using the `bulk-index` command. This command reads JSON objects from one or more files and sends them to the specified index using Elasticsearch's bulk API.

First, create a file named `sample-data.jsonl` (using `.jsonl` for JSON Lines format, where each line is a valid JSON object) with the documents you want to index:

**`sample-data.jsonl`:**

```json
{ "id": 1, "user": "alice", "timestamp": "2023-10-26T10:00:00Z", "metadata": {"source": "web", "session": "xyz"} }
{ "id": 2, "user": "bob", "timestamp": "2023-10-26T10:05:00Z", "metadata": {"source": "api", "version": 2} }
```

Now, use the `bulk-index` command to index the documents from this file into `my-data-v1`:

```bash
escuse-me documents bulk-index \\
  --index my-data-v1 \\
  --files sample-data.jsonl
```

The command will output the results of the bulk operation, indicating successful indexing or any errors.

**Optional: Inspect Indexed Data with `dump`**

You can use the `indices dump` command to view the documents you just indexed. This command uses the efficient Point-in-Time (PIT) API. By default, it outputs using Glazed, so you can format it as a table, JSON, YAML, or JSON Lines (often useful for programmatic processing).

```bash
# Dump documents from my-data-v1 as JSON Lines
escuse-me indices dump --index my-data-v1 -o jsonl

# Or view as a table (adjust fields as needed)
# escuse-me indices dump --index my-data-v1 -o table --fields _id,_source.*
```

---

## Step 3: Create an Alias for the Index

Aliases provide a stable endpoint for applications. Create an alias `my-data-alias` pointing to our current index `my-data-v1`.

```bash
escuse-me indices create-alias \
  --index my-data-v1 \
  --name my-data-alias
```

You can verify the alias was created:

```bash
# Use glazed flags for better table output if available, or just list all aliases
escuse-me indices aliases
# Look for my-data-alias pointing to my-data-v1 in the output
```

This should show `my-data-v1` associated with `my-data-alias`.

---

## Step 4: Define and Create the New Index (v2)

Now, define the mappings for our target index, `my-data-v2`. The key change is `metadata` is now type `text`.

**`mappings-v2.json`:**

```json
{
  "properties": {
    "id": { "type": "integer" },
    "user": { "type": "keyword" },
    "timestamp": { "type": "date" },
    "metadata": {
      "type": "text",
      "analyzer": "standard"
    }
  }
}
```

_Note: We switched type to `text` and specified an analyzer._

Create the `my-data-v2` index:

```bash
escuse-me indices create \
  --index my-data-v2 \
  --mappings mappings-v2.json
```

---

## Step 5: Reindex Data from v1 to v2

Copy the documents from the old index to the new one using the `reindex` command. Because the `metadata` field is changing from `object` to `text`, Elasticsearch cannot automatically convert it. We need to provide a simple script to tell Elasticsearch _how_ to handle this conversion during reindexing.

First, create a file named `transform-metadata.json` with the following script:

**`transform-metadata.json`:**

```json
{
  "source": "ctx._source.metadata = ctx._source.metadata.toString()",
  "lang": "painless"
}
```

This script uses Elasticsearch's Painless scripting language. For each document (`ctx._source`), it takes the existing `metadata` field and converts its value to a string representation before indexing it into the new `text` field in `my-data-v2`.

Now, run the reindex command, referencing this script:

We use `--wait-for-completion` here to simplify the tutorial flow, making the command block until the reindex is done. For large indices, you would typically omit this and monitor the progress using the default asynchronous behavior (see `escuse-me indices reindex --help` or the Reindexing documentation topic).

```bash
escuse-me indices reindex \
  --source-index my-data-v1 \
  --target-index my-data-v2 \
  --script transform-metadata.json \
  --wait-for-completion
```

The output will show the result, including counts of created documents.

**Optional: Inspect Reindexed Data**

You can again use `indices dump` to check the contents of the new index:

```bash
escuse-me indices dump --index my-data-v2 -o jsonl
```

You should see the `metadata` field now contains the stringified version of the original object.

---

## Step 6: Atomically Update the Alias

This is the crucial step for zero downtime. We want to switch `my-data-alias` from `my-data-v1` to `my-data-v2` in a single, atomic operation. The `update-aliases` command requires a JSON structure describing the actions.

**`alias-actions.json`:**

```json
{
  "actions": [
    { "remove": { "index": "my-data-v1", "alias": "my-data-alias" } },
    { "add": { "index": "my-data-v2", "alias": "my-data-alias" } }
  ]
}
```

Execute the update:

```bash
escuse-me indices update-aliases --actions alias-actions.json
```

Verify the alias switch:

```bash
escuse-me indices aliases
# Look for my-data-alias now pointing only to my-data-v2 in the output
```

This should now show `my-data-alias` pointing only to `my-data-v2`. Applications querying `my-data-alias` are now seamlessly using the data in the new index with the updated mapping.

---

## Conclusion

You have successfully:

- Created an index with specific mappings.
- Created an alias for it.
- Created a new index with modified mappings.
- Reindexed the data from the old index to the new one.
- Atomically switched the alias to point to the new index.

This pattern is fundamental for managing evolving Elasticsearch schemas without disrupting applications. You can now safely consider deleting the old `my-data-v1` index if it's no longer needed, using `escuse-me indices delete --index my-data-v1`.

For more advanced reindexing options like scripting, filtering, throttling, and asynchronous monitoring, refer to the documentation for the `escuse-me indices reindex` command (potentially available via `escuse-me indices reindex --help` or in documentation files like `escuse-me/pkg/doc/topics/07-reindexing.md`).
