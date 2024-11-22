---
Title: Writing YAML Commands
Slug: writing-yaml-commands
Short: This guide provides a comprehensive overview of how to write YAML commands using go-emrichen templating.
Topics:
- YAML
- Commands
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

## Basic Structure

A YAML command consists of three main sections:
1. Command metadata
2. Flag definitions
3. Query template

Here's a basic example:

```yaml
name: mycommand
short: Brief description
flags:
  - name: parameter1
    type: string
    help: Parameter description
query:
  # Query template here
```

## Flag Definitions

Flags define the parameters your command accepts. Each flag is defined with specific attributes that control its behavior and validation.

### Flag Attributes

- `name`: (required) Parameter name used in command-line and code
- `type`: (required) Data type of the flag. See below for supported types
- `help`: (required) Description shown in help text
- `default`: (optional) Default value if flag is not provided
- `required`: (optional) Boolean indicating if flag is mandatory
- `shortFlag`: (optional) Single-character alias for the flag
- `hidden`: (optional) Boolean to hide flag from help text
- `choices`: (optional) List of allowed values for the flag (when using types: choice, choiceList)

Glazed supports these parameter types for both flags and arguments:

### Basic Types
- `string`: Text values
- `int`: Integer values
- `float`: Floating-point numbers
- `bool`: Boolean true/false values
- `date`: Date values
- `choice`: Single selection from predefined choices
- `choiceList`: Multiple selections from predefined choices

### List Types
- `stringList`: List of strings
- `intList`: List of integers
- `floatList`: List of floating-point numbers

### File-Related Types
- `file`: Single file input, provides detailed file metadata and content
- `fileList`: List of files with metadata and content
- `stringFromFile`: Load string content from a file (prefix with @)
- `stringFromFiles`: Load string content from multiple files
- `stringListFromFile`: Load list of strings from a file
- `stringListFromFiles`: Load list of strings from multiple files

### Object Types
- `objectFromFile`: Load and parse structured data from a file
- `objectListFromFile`: Load and parse list of objects from a file
- `objectListFromFiles`: Load and parse lists of objects from multiple files

### Key-Value Types
- `keyValue`: Parse key-value pairs from string (comma-separated) or file (with @ prefix)


### Flag Type Examples

```yaml
flags:
  # String flag
  - name: name
    type: string
    help: User's name
    default: "guest"
    required: true

  # String list with choices
  - name: categories
    type: stringList
    help: Product categories to search
    choices: ["electronics", "books", "clothing"]
    default: ["electronics"]

  # Boolean flag
  - name: verbose
    type: bool
    help: Enable verbose output
    default: false
    shorthand: v

  # Integer with validation
  - name: age
    type: int
    help: User's age
    min: 0
    max: 150

  # Float with validation
  - name: price
    type: float
    help: Product price
    min: 0.0

  # Duration flag
  - name: timeout
    type: duration
    help: Operation timeout
    default: "30s"

  # File input
  - name: config
    type: file
    help: Configuration file path
    must_exist: true

  # Hidden flag
  - name: debug
    type: bool
    help: Enable debug mode
    hidden: true
```

### Advanced Flag Features

#### 4. Dynamic Defaults

Use template expressions for dynamic default values:

```yaml
flags:
  - name: home
    type: directory
    help: Home directory
    default: !Env "HOME"  # uses environment variable
  
  - name: backup_dir
    type: directory
    help: Backup directory
    default: !Format "{home}/backups"  # references another flag
```

### Using Flags in Templates

Flags defined in the command can be accessed in the template using various go-emrichen tags:

```yaml
query:
  # Simple value access
  user_name: !Var name

  # Check if flag exists
  has_email: !Exists email

  # Default value if flag not set
  timeout: !Default
    value: !Var timeout
    default: "60s"

  # Loop through list flag
  categories: !Loop
    over: !Var categories
    template: !Var item

  # Conditional based on flag
  output_format: !If
    test: !Var json
    then: "application/json"
    else: "text/plain"
```

## Go-Emrichen Template Tags

### 1. Variable Operations

- `!Var`: Access a variable
  ```yaml
  value: !Var first
  ```

- `!Exists`: Check if a variable exists
  ```yaml
  test: !Exists first
  ```

- `!Not`: Negate a condition
  ```yaml
  test: !Not,Exists first
  ```

### 2. Conditional Logic

- `!If`: Conditional branching
  ```yaml
  !If
    test: !Exists source_fields
    then: value_if_true
    else: value_if_false
  ```

### 3. List Operations

- `!Loop`: Iterate over lists
  ```yaml
  !Loop
    over: !Var source_fields
    template: !Var item
  ```

- `!All`: Check if all conditions are true
  ```yaml
  test: !All
    - !Exists first
    - !Exists second
  ```

## Real-World Example

Here's a complete example that demonstrates these concepts:

```yaml
name: manuel
short: Search a simple filter field
flags:
  - name: first
    type: string
    help: First name
  - name: second
    type: string
    help: Last name
  - name: all_fields
    type: bool
    default: false
    help: Return all fields
query:
  _source: !If
    test: !Var all_fields
    then: true
    else: !If
      test: !Exists source_fields
      then: !Loop
        over: !Var source_fields
        template: !Var item
      else: !Void
  query:
    !If
    test: !All
      - !Not,Exists first
      - !Not,Exists second
    then:
      match_all: {}
    else:
      bool:
        must:
          - !If
            test: !Exists first
            then:
              fuzzy:
                first:
                  value: !Var first
                  fuzziness: 2
            else: !Void
```
