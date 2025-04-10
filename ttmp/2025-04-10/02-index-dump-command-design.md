# Design: `escuse-me indices dump` Command

**Date:** 2025-04-10

**Goal:** Create a command to efficiently dump all documents from one or more Elasticsearch indices. The output should be suitable for inspection and potentially re-importing using `escuse-me documents bulk`.

## 1. Core Functionality

The command will retrieve documents from a specified Elasticsearch index (or indices matching a pattern). It needs to handle potentially large indices efficiently without overwhelming memory or hitting Elasticsearch search limits.

The primary mechanism for retrieving large datasets will be the **Point in Time (PIT)** API combined with **`search_after`**.

### What is Point in Time (PIT)?

The PIT API creates a lightweight "frozen" view of the index state at the moment it's created. Think of it as a snapshot marker. When you subsequently search using this PIT ID, you are guaranteed to see the data _exactly_ as it was when the PIT was opened, regardless of any documents being indexed or deleted concurrently.

### Why use PIT for dumping?

When dumping large indices, the process can take time. If documents are being added, updated, or deleted while the dump is in progress, standard pagination methods (like basic `from`/`size`) can become unreliable, potentially skipping documents or returning duplicates. PIT solves this by providing a stable, unchanging view of the data for the duration of the dump.

### How PIT works with `search_after`

`search_after` is an efficient way to retrieve the next "page" of results by using the sort values of the last document from the previous page as a starting point for the next search. Combining PIT and `search_after`:

1.  We open a PIT for the target index, getting back a PIT ID and an initial `keep_alive` duration.
2.  We perform an initial search using the PIT ID, sorting by `_shard_doc` (a requirement for `search_after`), and retrieve the first batch of documents.
3.  For subsequent batches, we perform searches using the same PIT ID, the same sort order, and the `search_after` parameter populated with the sort values from the _last_ document of the _previous_ batch.
4.  Each search using the PIT ID implicitly refreshes its `keep_alive` timer.
5.  Once all documents are retrieved (or the limit is reached), we explicitly close the PIT to release server resources.

This combination ensures consistent and efficient pagination over large, potentially changing datasets. It is generally preferred over the older `Scroll` API as it's more resource-friendly on the Elasticsearch cluster and stateless on the client side (after the initial PIT creation).

## 2. Command Definition

- **Name:** `escuse-me indices dump`
- **Parent:** `escuse-me indices`
- **Short Description:** Dumps documents from an Elasticsearch index.
- **Long Description:** Retrieves documents from the specified Elasticsearch index(es) using the Point in Time (PIT) API and `search_after` for efficient pagination. Supports filtering via Query DSL and multiple output formats, including one compatible with the `bulk` command.

## 3. Parameters/Flags

Flags will control the source index, filtering, output format, and performance.

**Source & Filtering:**

- `--index` (string, required): The name of the index or index pattern to dump documents from (e.g., `my-index`, `logs-*`).
- `--query` (string or file path): Path to a JSON/YAML file containing an Elasticsearch Query DSL query object, or the query JSON directly as a string. Defaults to `{"query": {"match_all": {}}}` to retrieve all documents. Allows filtering the documents to be dumped.
- `--limit` (int): Maximum number of documents to dump. Defaults to `-1` (no limit). Useful for sampling or testing.

**Output Format & Control:**

- `--bulk-format` (bool, default: false):
  - If `false` (default): Output documents using the standard Glazed processor. Each row will typically represent one document's `_source`. Users can use standard Glazed flags (`-o table`, `-o json`, `-o yaml`, `--output jsonl`, etc.) for formatting. Metadata like `_id` and `_index` can be included using `--fields _id,_index,_source.*`. This is best for inspection.
  - If `true`: Output documents in the Elasticsearch Bulk API format (alternating action/metadata line and source line). This format is directly consumable by `escuse-me documents bulk`.
- `--target-index` (string): **Used only when `--bulk-format` is true.** Specifies the index name to use in the bulk action/metadata line. If not provided, defaults to the original index name from which the document was retrieved (`_index` field of the hit). Allows dumping from `index-v1` but formatting it for import into `index-v2`.
- `--action` (choice: `index`, `create`, default: `index`): **Used only when `--bulk-format` is true.** Specifies the bulk action (`index` or `create`) to use in the action/metadata line.
- `--include-metadata` (string list): A list of top-level metadata fields (e.g., `_id`, `_index`, `_score`, `_routing`) to include as columns when _not_ using `--bulk-format`. Defaults potentially to `_id`, `_index`. Ignored if `--bulk-format` is true. _(Alternatively, rely solely on Glazed's `--fields` flag)_ Let's rely on `--fields` for simplicity.

**Performance & ES Control:**

- `--batch-size` (int, default: 1000): Number of documents to retrieve per `search` request within the PIT/`search_after` loop. Controls the `size` parameter in the ES query.
- `--pit-keep-alive` (duration string, default: "5m"): How long the Point in Time context should be kept alive on the Elasticsearch server. Passed to the initial PIT creation request and refreshed implicitly by subsequent `search` calls using it.
- **Standard ES Connection Flags:** Inherited via the `es-layers` (address, credentials, etc.).
- **Standard Glazed Flags:** Inherited via `glazed-layers` (output format, fields selection, etc.).

## 4. Output Formats

**Default (Glazed Output, `--bulk-format=false`)**

- Each Elasticsearch document hit results in one row passed to the Glazed processor.
- By default, the columns might be derived from the `_source` fields.
- Users can use `--fields _id,_index,_source.field1,_source.field2` etc., to control exactly which parts of the hit are output.
- Format controlled by `-o`/`--output` (table, json, yaml, jsonl). `jsonl` is often ideal for machine processing.

_Example (`-o jsonl --fields _id,_source.user`)_:

```json
{"_id": "doc1", "user": "alice"}
{"_id": "doc2", "user": "bob"}
```

**Bulk Format (`--bulk-format=true`)**

- Output is plain text, directly usable by the ES Bulk API / `escuse-me documents bulk`.
- Alternating lines:
  1.  Action/Metadata JSON: `{"index": {"_index": "target-index-name", "_id": "hit_id"}}` (action and index name controlled by flags)
  2.  Document Source JSON: `{ "field1": "value1", ... }` (the `_source` of the hit)

_Example (`--bulk-format --target-index my-new-index`)_:

```json
{"index": {"_index": "my-new-index", "_id": "doc1"}}
{"user": "alice", "timestamp": "...", ...}
{"index": {"_index": "my-new-index", "_id": "doc2"}}
{"user": "bob", "timestamp": "...", ...}
```

## 5. Implementation Notes

- Use `github.com/go-go-golems/glazed/pkg/cmds` framework.
- Leverage `es-layers` for ES client setup and connection flags.
- Leverage `glazed-layers` for output formatting flags.
- **Core Logic:**
  1.  Parse flags.
  2.  Get ES client.
  3.  Create PIT ID using `client.OpenPointInTime()`. Handle errors.
  4.  Initialize `search_after` variable (starts as `nil`).
  5.  Start loop:
      a. Build `search` request: - Set `pit.id` and `pit.keep_alive`. - Set `size` (`--batch-size`). - Set `query` (from `--query` flag or `match_all`). - Set `sort: [{"_shard_doc": "asc"}]` (required for `search_after`). - Set `search_after` (using value from previous iteration's last hit). - Potentially set `_source_includes`/`_source_excludes` based on Glazed's `--fields` if needed for efficiency (though filtering _after_ fetching might be simpler with Glazed).
      b. Execute `search` request. Handle errors.
      c. Check response for hits. If no hits, break loop.
      d. Process hits: - If `--bulk-format`: Iterate hits, print action line, print `_source` line. - Else (Glazed): Iterate hits, create `types.Row` from `_source` (and potentially other fields based on `--fields`), add row to Glazed processor.
      e. Update `search_after` variable with the `sort` value from the _last_ hit in the current batch.
      f. Check `--limit` and break if reached.
  6.  Close PIT ID using `client.ClosePointInTime()`. Ensure this runs even if the loop breaks early or errors occur (use `defer`).
- Error handling is crucial, especially around PIT creation/closure and network errors during pagination.
- Output should go to standard output, allowing redirection (`> dump.jsonl` or `> dump.bulk`).

## 6. Usage Examples

```bash
# Dump all docs from 'my-index' as JSON Lines (good for jq processing)
escuse-me indices dump --index my-index -o jsonl > my-index-dump.jsonl

# Dump only docs where user=alice as a table, showing only id and user field
escuse-me indices dump --index my-index --query '{"query": {"term": {"user": "alice"}}}' \
  -o table --fields _id,_source.user

# Dump first 100 docs matching a query from 'logs-*' into bulk format for re-import into 'logs-archived'
escuse-me indices dump --index 'logs-*' --query @filter-query.json --limit 100 \
  --bulk-format --target-index logs-archived --action create > logs.bulk

# Pipe dump directly to bulk import (re-indexing with filtering)
escuse-me indices dump --index old-index --query @filter.json --bulk-format --target-index new-index | \
  escuse-me documents bulk --files -
```

## 7. Future Considerations

- Support for the older `Scroll` API as a fallback? (Adds complexity, maybe not needed initially).
- More sophisticated progress indication (e.g., number of documents dumped). Glazed might offer some hooks.
- Directly specifying fields to include/exclude from `_source` via dedicated flags (though `--fields` might suffice).
