## Enhanced Error Handling with Raw Results

Added support for printing raw error responses when the --raw-results flag is enabled. This helps with debugging by showing the complete error response from Elasticsearch.

- Print complete error response to stderr when raw-results is enabled
- Print error reason and root cause to stderr when raw-results is disabled 

# Refactor Embeddings Settings Factory

Simplified the embeddings settings factory to use a minimal configuration struct instead of depending on the full StepSettings. Added backwards compatibility method.

- Created new EmbeddingsConfig struct for minimal configuration
- Modified SettingsFactory to use EmbeddingsConfig instead of StepSettings
- Added NewSettingsFactoryFromStepSettings for backwards compatibility 

# Fix Embeddings Settings Type Handling

Fixed type handling in embeddings settings to properly handle pointer types in StepSettings and non-pointer types in EmbeddingsConfig.

- Updated CreateEmbeddingsConfig to properly dereference pointer types
- Modified NewProvider to handle non-pointer types in EmbeddingsConfig
- Fixed error checks to use empty string checks instead of nil checks 

# Add Provider Options for Embeddings Factory

Added functional options pattern to the embeddings provider factory for more flexible configuration.

- Added WithType, WithEngine, WithBaseURL, WithAPIKey, and WithDimensions option functions
- Modified NewProvider to accept variadic options
- Improved configuration handling with options overriding defaults 

# Add Custom Tags Documentation

Added comprehensive documentation for implementing custom tags in go-emrichen, including:
- Basic tag implementation patterns
- Argument handling and validation
- Environment interaction
- Node processing utilities
- Testing guidelines and best practices
- Conceptual explanations and rationale for design patterns
- Detailed best practices and common patterns
- In-depth discussion of error handling and type safety

# Fix Custom Tags Documentation Signature

Updated custom tags documentation to reflect correct function signature:
- Changed tag handler signature to include interpreter parameter
- Clarified pure function nature of tag handlers
- Updated all code examples to use correct signature
- Added explanation of interpreter parameter usage

# Enhance Custom Tags Documentation with ParseArgs Guidelines

Added detailed documentation about argument handling and recursive processing:
- Comprehensive guide to using ParseArgs
- Core principles for implementing tag handlers
- Examples of proper argument validation and processing
- Guidelines for recursive processing of nested structures
- Detailed error handling patterns for arguments

# Update Custom Tags Documentation with Proper Namespace

Updated custom tags documentation to use proper import paths and namespaces:
- Added proper import statements for all examples
- Updated all type references to use emrichen namespace
- Fixed function signatures to use emrichen.Interpreter
- Updated utility function calls to use emrichen namespace 