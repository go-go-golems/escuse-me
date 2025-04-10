## Developer Guide: Adding Commands to `escuse-me`

### 1. Project Overview

`escuse-me` is a command-line interface (CLI) tool designed to interact with Elasticsearch clusters. It leverages the `glazed` framework for command structure, parameter handling, and structured output formatting (like tables, JSON, YAML). The goal is to provide convenient access to common Elasticsearch API operations directly from the terminal.

The base Go package for this project is `github.com/go-go-golems/escuse-me`. It utilizes the official Go Elasticsearch client library `github.com/elastic/go-elasticsearch/v8`.

### 2. Current Status: Alias Commands

We are currently working on implementing commands related to Elasticsearch index aliases under the `indices` subcommand. So far, the following have been added:

- **`indices aliases`**: Fetches and displays alias information.
  - Implementation: `escuse-me/cmd/escuse-me/cmds/indices/alias.go`
  - Uses: `esapi.IndicesGetAlias`
- **`indices delete-alias`**: Deletes one or more aliases from specified indices.
  - Implementation: `escuse-me/cmd/escuse-me/cmds/indices/delete_alias.go`
  - Uses: `esapi.IndicesDeleteAlias`

### 3. Goal: Completing Alias Commands & Beyond

The immediate next steps are to implement the remaining standard alias operations:

- **`indices create-alias`**: To add a new alias to one or more indices. (Likely using `esapi.IndicesPutAlias`)
- **`indices update-aliases`**: To perform multiple alias operations (add, remove) atomically. (Likely using `esapi.IndicesUpdateAliases`)

Beyond aliases, the same pattern can be used to add commands for other Elasticsearch APIs (e.g., document management, search, cluster health).

### 4. Key Packages and Concepts

- **`github.com/go-go-golems/glazed/pkg/cmds`**: Defines the core command interfaces (`Command`, `GlazeCommand`, `BareCommand`) and the `CommandDescription` struct.
- **`github.com/go-go-golems/glazed/pkg/cmds/parameters`**: Used for defining command flags and arguments (`ParameterDefinition`, `ParameterType`, `WithHelp`, `WithDefault`, etc.).
- **`github.com/go-go-golems/glazed/pkg/cmds/layers`**: Provides a way to group related parameters (like `glazed` output settings or ES connection settings). We use `ParsedLayers` to access parsed values.
- **`github.com/go-go-golems/glazed/pkg/settings`**: Contains helpers for creating standard parameter layers, like `NewGlazedParameterLayers`.
- **`github.com/go-go-golems/glazed/pkg/middlewares`**: The `Processor` interface (often `gp` variable) is used by `GlazeCommand` to output structured data.
- **`github.com/go-go-golems/glazed/pkg/types`**: Defines `Row` for structured data output.
- **`github.com/go-go-golems/glazed/pkg/cli`**: Contains functions like `BuildCobraCommandFromGlazeCommand` to convert `glazed` commands into `cobra` commands.
- **`github.com/elastic/go-elasticsearch/v8/esapi`**: The Elasticsearch Go client API functions (e.g., `IndicesGetAlias`, `IndicesDeleteAlias`).
- **`github.com/go-go-golems/escuse-me/pkg/cmds/layers`**: Defines custom layers specific to `escuse-me`, primarily `NewESParameterLayer` for Elasticsearch connection details.
- **`github.com/go-go-golems/escuse-me/pkg/cmds`**: Contains helper functions like `BuildCobraCommandWithEscuseMeMiddlewares` which wraps `glazed/pkg/cli` functions to add `escuse-me` specific setup.
- **`github.com/spf13/cobra`**: The underlying CLI framework used by `glazed`.

### 5. Key Files

- **`escuse-me/cmd/escuse-me/main.go`**: Entry point of the application, sets up the root cobra command.
- **`escuse-me/cmd/escuse-me/cmds/indices/indices.go`**: Registers all commands under the `indices` subcommand. **New commands need to be added here.**
- **`escuse-me/cmd/escuse-me/cmds/indices/*.go`**: Implementation files for individual `indices` subcommands (e.g., `alias.go`, `mappings.go`, `delete_alias.go`). **New command logic goes into new files here.**
- **`escuse-me/pkg/cmds/layers/es.go`**: Defines the `ESParameterLayer` and the helper `NewESClientFromParsedLayers` to get an initialized Elasticsearch client based on command-line flags or environment variables.
- **`escuse-me/pkg/cmds/cobra.go`**: Contains `BuildCobraCommandWithEscuseMeMiddlewares`.
- **`glazed/prompto/glazed/create-command-tutorial.md`**: The general tutorial for creating `glazed` commands. Refer to this for foundational concepts.

### 6. Workflow for Adding a New Command (e.g., `create-alias`)

1.  **Create the Go File**: Create `escuse-me/cmd/escuse-me/cmds/indices/create_alias.go`.
2.  **Define Command Struct**:

    ```go
    package indices

    import (
        // ... necessary imports: context, fmt, io, time, glazed/cmds, glazed/layers, glazed/parameters, glazed/settings, glazed/middlewares, es/layers, esapi, pkg/errors, log, etc.
    )

    type IndicesCreateAliasCommand struct {
        *cmds.CommandDescription
    }

    // Decide on the command type. For create/update/delete, often a simple confirmation is needed.
    // A BareCommand or WriterCommand might be suitable if you just print the ES response.
    // Let's assume BareCommand for now, as Glaze tables aren't the primary output.
    var _ cmds.BareCommand = &IndicesCreateAliasCommand{} // Or cmds.GlazeCommand / cmds.WriterCommand
    ```

3.  **Create Constructor (`New...`)**:

    ```go
    func NewIndicesCreateAliasCommand() (*IndicesCreateAliasCommand, error) {
        // Create standard glazed layer (might customize if not outputting tables)
        glazedLayer, err := settings.NewGlazedParameterLayers( /* options? */)
        if err != nil { /* handle error */ }

        // Create ES connection layer
        esLayer, err := layers.NewESParameterLayer()
        if err != nil { /* handle error */ }

        return &IndicesCreateAliasCommand{
            CommandDescription: cmds.NewCommandDescription(
                "create-alias",
                cmds.WithShort("Creates an index alias"),
                cmds.WithFlags(
                    // Define flags using parameters.NewParameterDefinition
                    // Example: Index name(s), Alias name, Filter (JSON string?), Routing, IsWriteIndex etc.
                    // Refer to esapi.IndicesPutAliasRequest documentation for needed fields.
                    parameters.NewParameterDefinition(
                        "index",
                        parameters.ParameterTypeStringList,
                        parameters.WithHelp("Index name(s) to add the alias to"),
                        parameters.WithRequired(true),
                    ),
                    parameters.NewParameterDefinition(
                        "name",
                        parameters.ParameterTypeString, // Typically one alias name per creation via PutAlias
                        parameters.WithHelp("The name of the alias to create"),
                        parameters.WithRequired(true),
                    ),
                    parameters.NewParameterDefinition(
                        "body", // Often easier to pass complex things like filters as JSON
                        parameters.ParameterTypeObjectFromFile, // Reads JSON/YAML from file or stdin
                        parameters.WithHelp("JSON object defining alias properties (filter, routing, is_write_index)"),
                        parameters.WithOptional(true), // Optional if no filter/routing needed
                    ),
                    // Add other relevant flags: timeout, master_timeout etc.
                    parameters.NewParameterDefinition(
                        "timeout",
                        parameters.ParameterTypeInteger,
                        parameters.WithHelp("Explicit operation timeout in seconds"),
                        parameters.WithDefault(30),
                    ),
                ),
                cmds.WithLayersList(glazedLayer, esLayer),
            ),
        }, nil
    }
    ```

4.  **Define Settings Struct**:
    ```go
    type IndicesCreateAliasSettings struct {
        Index         []string               `glazed.parameter:"index"`
        Name          string                 `glazed.parameter:"name"`
        Body          map[string]interface{} `glazed.parameter:"body"` // Parsed from --body flag
        Timeout       time.Duration          `glazed.parameter:"timeout"`
        // Add other fields matching flag names
    }
    ```
5.  **Implement `Run` Method (for `BareCommand`)**:

    ```go
    func (c *IndicesCreateAliasCommand) Run(
        ctx context.Context,
        parsedLayers *layers.ParsedLayers,
    ) error {
        s := &IndicesCreateAliasSettings{}
        err := parsedLayers.InitializeStruct(layers.DefaultSlug, s)
        if err != nil { return errors.Wrap(err, "...") }

        es, err := layers.NewESClientFromParsedLayers(parsedLayers)
        if err != nil { return errors.Wrap(err, "...") }

        log.Debug().Str("name", s.Name).Strs("indices", s.Index).Msg("Creating alias")

        // Prepare options for the ES API call
        options := []func(*esapi.IndicesPutAliasRequest){
            es.Indices.PutAlias.WithContext(ctx),
            es.Indices.PutAlias.WithTimeout(s.Timeout),
            // Add master_timeout, etc.
        }

        // Handle the body if provided
        var requestBody io.Reader
        if s.Body != nil && len(s.Body) > 0 {
            bodyBytes, err := json.Marshal(s.Body)
            if err != nil {
                return errors.Wrap(err, "failed to marshal request body")
            }
            requestBody = bytes.NewReader(bodyBytes)
            options = append(options, es.Indices.PutAlias.WithBody(requestBody))
        }


        // Make the call (Indices.PutAlias)
        res, err := es.Indices.PutAlias(
            s.Index, // Target indices
            s.Name,  // Alias name
            options...,
        )
        if err != nil { return errors.Wrap(err, "...") }
        defer res.Body.Close()

        // Process response (check for errors, print confirmation)
        bodyBytes, err := io.ReadAll(res.Body)
        if err != nil { return errors.Wrap(err, "failed to read response body") }
        bodyString := string(bodyBytes)

        if res.IsError() {
            log.Error().Int("status", res.StatusCode).Str("body", bodyString).Msg("Failed to create alias")
            return fmt.Errorf("elasticsearch error [%d]: %s", res.StatusCode, bodyString)
        }

        log.Info().Int("status", res.StatusCode).Str("body", bodyString).Msg("Alias created successfully")
        fmt.Println(bodyString) // Print the success response (usually {"acknowledged": true})

        return nil
    }

    // If using GlazeCommand, implement RunIntoGlazeProcessor instead and use gp.AddRow
    // If using WriterCommand, implement RunIntoWriter and use w.Write
    ```

6.  **Register Command**: Open `escuse-me/cmd/escuse-me/cmds/indices/indices.go` and add:

    ```go
    // ... inside AddToRootCommand function ...

    indicesCreateAliasCommand, err := NewIndicesCreateAliasCommand()
    if err != nil {
        return err
    }
    // Use the helper to add middlewares (like logging)
    indicesCreateAliasCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(indicesCreateAliasCommand)
    if err != nil {
        return err
    }
    indicesCommand.AddCommand(indicesCreateAliasCmd)

    // ... rest of the function ...
    ```

7.  **Build and Test**: Run `go build ./cmd/escuse-me` and test the new command against an Elasticsearch instance.

### 7. Next Steps

1.  Implement `IndicesCreateAliasCommand` following the steps above, using `esapi.IndicesPutAlias`.
2.  Implement `IndicesUpdateAliasesCommand` using `esapi.IndicesUpdateAliases`. This API call usually takes a structured body defining multiple `add` and `remove` actions, so the `--body` flag (using `ParameterTypeObjectFromFile`) will be crucial.
3.  Continue adding other useful Elasticsearch commands following the established pattern.

Good luck! Remember to check the `esapi` documentation for the specific requirements of each API call.
