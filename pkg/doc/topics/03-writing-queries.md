---
Title: Writing Embeddings Queries
Slug: writing-embeddings-queries
Short: Learn how to write effective embeddings queries for escuse-me using YAML
Topics:
  - usage
  - queries
  - yaml
Commands:
  - query
  - run
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

## Embeddings Query Structure

Embedding queries are written in YAML and can use emrichen tags for advanced functionality. A basic query consists of:

1. **Text**: The text to be embedded
2. **Configuration**: Optional settings to customize the embedding process

Here's a simple example:

```yaml
!Embeddings
  text: "This is the text I want to embed"
```

## Configuration Options

The embedding settings can be configured in three ways, in order of precedence:

1. Command-line flags
2. Environment variables
3. Configuration file
4. Query-specific configuration (in the YAML file)

### Command-line Flags

The following flags are available:

```
--embeddings-dimensions    Output dimension of embeddings (default: 1536 for OpenAI, 384 for Ollama all-minilm)
--embeddings-engine       Model to use (default: "text-embedding-3-small")
--embeddings-type        Provider type ("openai" or "ollama") (default: "openai")
--ollama-api-key         API key for Ollama
--ollama-base-url        Base URL for Ollama (default: "http://localhost:11434")
--openai-api-key         API key for OpenAI
--openai-base-url        Base URL for OpenAI (default: "https://api.openai.com/v1")
```

### Environment Variables

All settings can be configured through environment variables with the prefix `ESCUSE_ME_`:

```bash
ESCUSE_ME_EMBEDDINGS_DIMENSIONS=1536
ESCUSE_ME_EMBEDDINGS_ENGINE=text-embedding-3-small
ESCUSE_ME_EMBEDDINGS_TYPE=openai
ESCUSE_ME_OLLAMA_API_KEY=your-key
ESCUSE_ME_OLLAMA_BASE_URL=http://localhost:11434
ESCUSE_ME_OPENAI_API_KEY=your-key
ESCUSE_ME_OPENAI_BASE_URL=https://api.openai.com/v1
```

### Configuration File

The same settings can be specified in a configuration file:

```yaml
embeddings:
  dimensions: 1536
  engine: text-embedding-3-small
  type: openai
  ollama-api-key: your-key
  ollama-base-url: http://localhost:11434
  openai-api-key: your-key
  openai-base-url: https://api.openai.com/v1
```

### Query-specific Configuration

You can override any of the above settings in your query YAML file:

```yaml
!Embeddings
  text: "This is the text I want to embed"
  config:
    type: "openai"
    engine: "text-embedding-3-small"
    dimensions: 1536
    base_url: "https://api.openai.com/v1"
    api_key: "your-api-key"
```

The configuration options are applied in order of precedence, with query-specific settings overriding CLI flags, which override environment variables, which override the config file.

## Provider-Specific Settings

### OpenAI

Default settings for OpenAI:
- dimensions: 1536
- engine: text-embedding-3-small
- base_url: https://api.openai.com/v1

Example configuration:

```yaml
!Embeddings
  text: "My text"
  config:
    type: "openai"
    engine: "text-embedding-3-small"
    api_key: "your-openai-api-key"
```

### Ollama

Default settings for Ollama:
- dimensions: 384 (for all-minilm)
- engine: all-minilm
- base_url: http://localhost:11434

Example configuration:

```yaml
!Embeddings
  text: "My text"
  config:
    type: "ollama"
    engine: "all-minilm"
    base_url: "http://localhost:11434"
```

## Using Variables

You can use emrichen variables in your queries for dynamic content:

```yaml
!Var text_to_embed: "This is my text"

result: !Embeddings
  text: !Var text_to_embed
```

## Best Practices

1. **Configuration Management**:
   - Use environment variables or config files for sensitive data like API keys
   - Set common defaults in the config file
   - Override specific settings via CLI flags when needed
   - Use query-specific config only for one-off changes

2. **Text Preparation**:
   - Keep text concise and focused
   - Remove unnecessary formatting
   - Consider text length limits of your chosen model

3. **Error Handling**:
   - Validate your YAML syntax
   - Check for required fields
   - Monitor API rate limits

## Examples

### Basic Text Embedding

```yaml
simple: !Embeddings
  text: "Hello, world!"
```

### Using Variables and Templates

```yaml
!Var texts:
  - "First text"
  - "Second text"

embeddings: !Loop
  over: !Var texts
  template:
    !Embeddings
      text: !Var item
```

### Complex Configuration

```yaml
!Var api_key: !Env OPENAI_API_KEY

result: !Embeddings
  text: "Text to embed"
  config:
    type: "openai"
    engine: "text-embedding-3-small"
    dimensions: 1536
    api_key: !Var api_key
```

## Troubleshooting

Common issues and their solutions:

1. **Configuration Issues**:
   - Check configuration precedence (query → CLI → env → config file)
   - Verify environment variables are properly set
   - Ensure config file is in the correct location

2. **Missing Required Fields**:
   - Ensure 'text' field is present
   - Verify provider-specific required config

3. **API Errors**:
   - Check API key validity
   - Verify network connectivity
   - Review rate limits

## Next Steps

- Explore advanced emrichen tags for complex queries
- Learn about batch processing for multiple texts
- Understand embedding storage and retrieval 