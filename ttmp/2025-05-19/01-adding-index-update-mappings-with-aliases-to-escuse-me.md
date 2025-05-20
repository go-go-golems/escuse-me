# Adding Index Update Mappings with Aliases to escuse-me

## Context

The `escuse-me` tool currently has basic index management capabilities, but lacks a sophisticated way to update index mappings with zero-downtime support. The `go-go-mento` project has implemented a robust solution that we want to port over.

## Current State

- `escuse-me` has basic index operations (create, delete, reindex)
- `go-go-mento` has a sophisticated update-mappings command that handles:
  - Schema validation
  - Zero-downtime updates
  - Alias management
  - Automatic cleanup

## Goals

1. Add a new `update-mappings` command to `escuse-me`
2. Support zero-downtime updates via reindexing
3. Handle alias management automatically
4. Support both interactive and non-interactive modes

## Reference Files

For reference, here are the key files in the existing implementation:

1. Current update mapping implementation:
   - [update-mapping.go](https://github.com/go-go-golems/escuse-me/blob/main/cmd/escuse-me/cmds/indices/update-mapping.go) - Basic mapping update functionality
   - [reindex.go](https://github.com/go-go-golems/escuse-me/blob/main/cmd/escuse-me/cmds/indices/reindex.go) - Reindexing functionality we'll leverage for zero-downtime updates
   - [main.go](https://github.com/go-go-golems/escuse-me/blob/main/cmd/escuse-me/main.go) - Root command setup

2. Source implementation from go-go-mento:
   - [update-mappings.go](https://github.com/team-mento/mento-playground/blob/main/go/cmd/mento-service/cmds/rag/update-mappings.go) - The implementation we're porting over

## Detailed Implementation Steps

### 1. Update Command Structure

```go
// cmd/escuse-me/cmds/indices/update_mappings.go

type UpdateMappingsSettings struct {
    IndexName      string                 `glazed.parameter:"index"`
    Mappings       map[string]interface{} `glazed.parameter:"mappings"`
    ZeroDowntime   bool                   `glazed.parameter:"zero-downtime"`
    DeleteOldIndex bool                   `glazed.parameter:"delete-old-index"`
    BatchSize      int                    `glazed.parameter:"batch-size"`
    TimeoutSeconds int                    `glazed.parameter:"update-timeout"`
    NonInteractive bool                   `glazed.parameter:"non-interactive"`
    WriteIndexOnly bool                   `glazed.parameter:"write_index_only"`
}

type UpdateMappingsCommand struct {
    *cmds.CommandDescription
}

func NewUpdateMappingsCommand() (*UpdateMappingsCommand, error) {
    glazedParameterLayer, err := settings.NewGlazedParameterLayers()
    if err != nil {
        return nil, errors.Wrap(err, "could not create Glazed parameter layer")
    }
    esParameterLayer, err := es_layers.NewESParameterLayer()
    if err != nil {
        return nil, errors.Wrap(err, "could not create ES parameter layer")
    }

    return &UpdateMappingsCommand{
        CommandDescription: cmds.NewCommandDescription(
            "update-mappings",
            cmds.WithShort("Updates the mapping of an existing index with zero-downtime support"),
            cmds.WithFlags(
                parameters.NewParameterDefinition(
                    "index",
                    parameters.ParameterTypeString,
                    parameters.WithHelp("Name of the index to update mapping for"),
                    parameters.WithRequired(true),
                ),
                parameters.NewParameterDefinition(
                    "mappings",
                    parameters.ParameterTypeObjectFromFile,
                    parameters.WithHelp("JSON/YAML file containing updated index mappings"),
                    parameters.WithRequired(true),
                ),
                parameters.NewParameterDefinition(
                    "zero-downtime",
                    parameters.ParameterTypeBool,
                    parameters.WithHelp("Use zero-downtime update approach (reindex if needed)"),
                    parameters.WithDefault(true),
                ),
                parameters.NewParameterDefinition(
                    "delete-old-index",
                    parameters.ParameterTypeBool,
                    parameters.WithHelp("Delete old index after successful zero-downtime update"),
                    parameters.WithDefault(false),
                ),
                parameters.NewParameterDefinition(
                    "batch-size",
                    parameters.ParameterTypeInteger,
                    parameters.WithHelp("Batch size for reindexing"),
                    parameters.WithDefault(1000),
                ),
                parameters.NewParameterDefinition(
                    "update-timeout",
                    parameters.ParameterTypeInteger,
                    parameters.WithHelp("Timeout in seconds (0 means no timeout)"),
                    parameters.WithDefault(0),
                ),
                parameters.NewParameterDefinition(
                    "non-interactive",
                    parameters.ParameterTypeBool,
                    parameters.WithHelp("Run in non-interactive mode without confirmation prompts"),
                    parameters.WithDefault(false),
                ),
                parameters.NewParameterDefinition(
                    "write_index_only",
                    parameters.ParameterTypeBool,
                    parameters.WithHelp("If true, the mappings are applied only to the current write index for the target."),
                ),
            ),
            cmds.WithLayersList(glazedParameterLayer, esParameterLayer),
        ),
    }, nil
}
```

### 2. Update Options and Core Logic

```go
// pkg/es/update_options.go

type UpdateOptions struct {
    ZeroDowntime      bool
    DeleteOldIndex    bool
    WaitForCompletion bool
    Timeout          time.Duration
    ReindexOptions   *ReindexOptions
    WriteIndexOnly   bool
}

type ReindexOptions struct {
    BatchSize         int
    RequestsPerSecond float32
    Slices           int
}

// pkg/es/index.go

func (idx *ElasticsearchIndex) UpdateMappings(
    ctx context.Context,
    mappings map[string]interface{},
    opts *UpdateOptions,
) error {
    // 1. Get current index info
    currentMappings, err := idx.GetMappings(ctx)
    if err != nil {
        return errors.Wrap(err, "failed to get current mappings")
    }

    // 2. Get current aliases
    aliases, err := idx.GetAliases(ctx)
    if err != nil {
        return errors.Wrap(err, "failed to get current aliases")
    }

    // 3. If zero-downtime update:
    if opts.ZeroDowntime {
        // a. Create new index with new mappings
        newIndexName := fmt.Sprintf("%s-%s", idx.indexName, time.Now().Format("20060102150405"))
        err = idx.CreateIndex(ctx, newIndexName, nil, mappings)
        if err != nil {
            return errors.Wrap(err, "failed to create new index")
        }

        // b. Reindex data
        reindexBody := map[string]interface{}{
            "source": map[string]interface{}{
                "index": idx.indexName,
                "size":  opts.ReindexOptions.BatchSize,
            },
            "dest": map[string]interface{}{
                "index": newIndexName,
            },
        }

        if opts.ReindexOptions.RequestsPerSecond > 0 {
            reindexBody["requests_per_second"] = opts.ReindexOptions.RequestsPerSecond
        }
        if opts.ReindexOptions.Slices > 0 {
            reindexBody["slices"] = opts.ReindexOptions.Slices
        }

        // Execute reindex
        err = idx.Reindex(ctx, reindexBody, opts.WaitForCompletion)
        if err != nil {
            return errors.Wrap(err, "failed to reindex data")
        }

        // c. Update aliases atomically
        if len(aliases) > 0 {
            actions := make([]map[string]interface{}, 0, len(aliases)*2)
            for _, alias := range aliases {
                actions = append(actions,
                    map[string]interface{}{
                        "remove": map[string]interface{}{
                            "index": idx.indexName,
                            "alias": alias,
                        },
                    },
                    map[string]interface{}{
                        "add": map[string]interface{}{
                            "index": newIndexName,
                            "alias": alias,
                        },
                    },
                )
            }
            err = idx.UpdateAliases(ctx, actions)
            if err != nil {
                return errors.Wrap(err, "failed to update aliases")
            }
        }

        // d. Delete old index if requested
        if opts.DeleteOldIndex {
            err = idx.DeleteIndex(ctx, idx.indexName)
            if err != nil {
                return errors.Wrap(err, "failed to delete old index")
            }
        }
    } else {
        // Direct mapping update
        options := []func(*esapi.IndicesPutMappingRequest){
            es.Indices.PutMapping.WithWriteIndexOnly(opts.WriteIndexOnly),
        }
        err = idx.PutMapping(ctx, mappings, options...)
        if err != nil {
            return errors.Wrap(err, "failed to update mappings")
        }
    }

    return nil
}
```

### 3. Command Implementation

```go
// cmd/escuse-me/cmds/indices/update_mappings.go

func (c *UpdateMappingsCommand) RunIntoWriter(
    ctx context.Context,
    parsedLayers *layers.ParsedLayers,
    w io.Writer,
) error {
    // 1. Load and validate settings
    s := &UpdateMappingsSettings{}
    if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
        return errors.Wrap(err, "failed to initialize settings")
    }

    // 2. Get ES client
    es, err := es_layers.NewESClientFromParsedLayers(parsedLayers)
    if err != nil {
        return errors.Wrap(err, "failed to create ES client")
    }

    // 3. Interactive confirmation if needed
    if !s.NonInteractive {
        fmt.Fprintf(w, "You are about to update mappings for index %s\n", s.IndexName)
        if s.ZeroDowntime {
            fmt.Fprintf(w, "This will be a zero-downtime update with reindexing\n")
        }
        fmt.Fprintf(w, "Type 'yes' to continue: ")
        
        var response string
        fmt.Scanln(&response)
        if response != "yes" {
            return errors.New("operation cancelled by user")
        }
    }

    // 4. Configure update options
    updateOpts := &es.UpdateOptions{
        ZeroDowntime:      s.ZeroDowntime,
        DeleteOldIndex:    s.DeleteOldIndex,
        WaitForCompletion: true,
        WriteIndexOnly:    s.WriteIndexOnly,
    }

    if s.TimeoutSeconds > 0 {
        updateOpts.Timeout = time.Duration(s.TimeoutSeconds) * time.Second
    }

    if s.ZeroDowntime {
        updateOpts.ReindexOptions = &es.ReindexOptions{
            BatchSize: s.BatchSize,
        }
    }

    // 5. Execute update
    err = es.UpdateMappings(ctx, s.Mappings, updateOpts)
    if err != nil {
        return errors.Wrap(err, "failed to update mappings")
    }

    // 6. Report results
    fmt.Fprintf(w, "Successfully updated mappings for index %s\n", s.IndexName)
    if s.ZeroDowntime {
        fmt.Fprintf(w, "Zero-downtime update completed\n")
    }

    return nil
}
```

### 4. Elasticsearch API Calls

The implementation uses these Elasticsearch APIs:

1. **Get Current Mappings**
```http
GET /{index}/_mapping
```

2. **Get Current Aliases**
```http
GET /_alias/{index}
```

3. **Create New Index**
```http
PUT /{new_index}
{
  "mappings": { ... }
}
```

4. **Reindex**
```http
POST /_reindex
{
  "source": {
    "index": "old_index",
    "size": 1000
  },
  "dest": {
    "index": "new_index"
  },
  "requests_per_second": 1000,
  "slices": 1
}
```

5. **Update Aliases**
```http
POST /_aliases
{
  "actions": [
    {
      "remove": {
        "index": "old_index",
        "alias": "my_alias"
      }
    },
    {
      "add": {
        "index": "new_index",
        "alias": "my_alias"
      }
    }
  ]
}
```

6. **Delete Index**
```http
DELETE /{old_index}
```

7. **Put Mapping** (for non-zero-downtime updates)
```http
PUT /{index}/_mapping
{
  "properties": { ... }
}
```

### 5. Integration with Existing Code

The new implementation should:

1. Use the existing `es_layers.NewESClientFromParsedLayers` for client creation
2. Follow the same error handling patterns as other commands
3. Use the same output formatting through the `io.Writer` interface
4. Integrate with the existing parameter layer system

### 6. Testing Strategy

1. **Unit Tests**
   - Mapping validation
   - Options handling
   - Mock ES client responses

2. **Integration Tests**
   - Full update workflow
   - Alias management
   - Error handling
   - Timeout handling

3. **Manual Testing Scenarios**
   - Small index update
   - Large index update
   - Index with aliases
   - Index with complex mappings
   - Failed updates
   - Timeout scenarios

## Next Steps

1. [ ] Implement update options
2. [ ] Add core update logic
3. [ ] Create command structure
4. [ ] Add to root command
5. [ ] Write tests
6. [ ] Add documentation
7. [ ] Manual testing

## Resources

- [Elasticsearch Reindex API](https://www.elastic.co/guide/en/elasticsearch/reference/current/docs-reindex.html)
- [Elasticsearch Alias API](https://www.elastic.co/guide/en/elasticsearch/reference/current/indices-aliases.html)
- [go-go-mento Implementation](https://github.com/team-mento/mento-playground/blob/main/go/cmd/mento-service/cmds/rag/update-mappings.go) 