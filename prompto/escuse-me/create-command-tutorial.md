# Tutorial: Creating YAML Commands for Escuse-me

## Introduction

Escuse-me is a powerful tool for creating Elasticsearch query commands using YAML configuration files. At its core, it uses Emrichen, a templating system that extends YAML with powerful features like variables, conditionals, and loops. This combination allows you to create dynamic and flexible Elasticsearch queries that can adapt to different inputs and conditions. This tutorial will show you how to create these commands step by step.

## Basic Command Structure

A basic escuse-me command YAML file has the following structure:

```yaml
name: "command-name"
short: "Short description"
long: "Optional longer description"
flags:
  - name: field1
    type: string
    help: "Help text for field1"
  - name: field2
    type: bool
    default: false
    help: "Help text for field2"
query:
  query:
    match:
      field1: !Var field1
```

## Command Components

### 1. Basic Metadata

```yaml
name: "search-users"
short: "Search users in the database"
long: "A more detailed description of what this command does and how to use it"
```

### 2. Parameter Definitions

Parameters can be defined using the `flags` section. Available parameter types include:

```yaml
flags:
  - name: name
    type: string
    help: "User's name to search for"
    
  - name: age
    type: int
    default: 18
    help: "Minimum age to filter by"
    
  - name: active
    type: bool
    default: true
    help: "Filter by active status"
    
  - name: tags
    type: stringList
    help: "List of tags to filter by"
```

Available parameter types:
- `string`: Simple string value
- `stringFromFile`: String loaded from a file
- `file`: File data with parsing capabilities
- `fileList`: List of files
- `int`: Integer value
- `float`: Floating point value
- `bool`: Boolean value
- `date`: Date value
- `stringList`: List of strings
- `intList`: List of integers
- `floatList`: List of floating point numbers
- `choice`: Single selection from predefined options
- `choiceList`: Multiple selections from predefined options

### 3. Query Templates

The query section defines the complete Elasticsearch request body. The `query` field at the top level of your command represents the entire body that will be sent to Elasticsearch, including both the query itself and other parameters like `_source`, `size`, `from`, etc:

1. Direct YAML query:
```yaml
query:         # represents the complete ES request body
  _source: true
  size: 10
  query:       # the actual query part
    bool:
      must:
        - match:
            name: !Var name
        - range:
            age:
              gte: !Var age
```

2. Query string template:
```yaml
query:         # represents the complete ES request body
  _source: ["field1", "field2"]
  from: 0
  query:       # the actual query part
    bool:
      must: [
        { "match": { "name": "{{ .name }}" } },
        { "range": { "age": { "gte": {{ .age }} } } }
      ]
```

### 4. Emrichen Templating System

Emrichen is the templating engine that powers escuse-me's dynamic query generation. It extends YAML with special tags that enable powerful templating features.

#### Basic Concepts

1. **Variables**: Access input parameters using `!Var`
```yaml
query:
  query:
    match:
        field: !Var my_parameter
```

2. **Existence Checks**: Check if parameters are provided using `!Exists`
```yaml
!If
  test: !Exists parameter_name
  then: "Parameter exists"
  else: "Parameter is missing"
```

3. **Logical Operations**: Combine conditions with `!All`, `!Any`, and `!Not`
```yaml
!If
  test: !All
    - !Exists param1
    - !Not,Exists param2
  then: "Condition met"
  else: "Condition not met"
```

#### Advanced Emrichen Features

1. **Loops and Iterations**:
```yaml
fields: !Loop
  over: !Var field_list
  template:
    - field: !Var item
      boost: 1.0
```

2. **String Formatting**:
```yaml
message: !Format "Hello {name}, your score is {score}"
```

3. **Concatenation**:
```yaml
tags: !Concat
  - !Var base_tags
  - !Var additional_tags
```

4. **Default Values**:
```yaml
name: !Default
  value: !Var user_name
  default: "anonymous"
```

5. **Type Conversions**:
```yaml
count: !Int !Var string_number
timestamp: !Timestamp !Var date_string
```

#### Common Emrichen Patterns

1. **Optional Query Parts**:
```yaml
bool:
  must: !Concat
    - !If
        test: !Exists text
        then:
          - match:
              description: !Var text
        else: !Void
    - !If
        test: !Exists status
        then:
          - term:
              status: !Var status
        else: !Void
```

2. **Dynamic Field Selection**:
```yaml
_source: !If
  test: !Exists fields
  then: !Var fields
  else: true
```

3. **Nested Conditionals**:
```yaml
sort: !If
  test: !Exists sort_field
  then:
    - !If
        test: !Exists sort_order
        then:
          !Format "{sort_field}:{sort_order}"
        else:
          !Var sort_field
  else: 
    - _score
```

### 5. Advanced Features

#### Using Emrichen Tags

Escuse-me supports Emrichen tags for dynamic query generation:

```yaml
query:           # represents the complete ES request body
  _source: !If
    test: !Exists fields
    then: !Var fields
    else: true
  size: !Var size
  query:         # the actual query part
    bool:
      must:
        - !If
          test: !Exists name
          then:
            match:
              name: !Var name
          else: !Void
        - !If
          test: !Exists age
          then:
            range:
              age:
                gte: !Var age
          else: !Void
```

Common Emrichen tags:
- `!Var`: Reference a variable
- `!Exists`: Check if a variable exists
- `!If`: Conditional logic
- `!Void`: Return nothing
- `!Loop`: Iterate over a list
- `!Format`: String formatting

#### Layout Configuration

You can define how the command output is displayed:

```yaml
layout:
  - name: "Basic Info"
    fields:
      - name
      - age
      - status
  - name: "Details"
    fields:
      - description
      - tags
```

## Example Commands

### 1. Simple User Search

```yaml
name: user-search
short: Search users by name and age
flags:
  - name: name
    type: string
    help: "Name to search for"
  - name: min_age
    type: int
    default: 18
    help: "Minimum age"
query:           # represents the complete ES request body
  _source: true
  query:         # the actual query part
    bool:
      must:
        - match:
            name: !Var name
        - range:
            age:
              gte: !Var min_age
```

### 2. Advanced Document Search

```yaml
name: document-search
short: Search documents with multiple criteria
flags:
  - name: query
    type: string
    help: "Search text"
  - name: fields
    type: stringList
    help: "Fields to search in"
  - name: date_from
    type: date
    help: "Start date"
  - name: date_to
    type: date
    help: "End date"
query:           # represents the complete ES request body
  _source: !Var fields
  size: 20
  query:         # the actual query part
    bool:
      must:
        - multi_match:
            query: !Var query
            fields: !Var fields
        - !If
          test: !All
            - !Exists date_from
            - !Exists date_to
          then:
            range:
              created_at:
                gte: !Var date_from
                lte: !Var date_to
          else: !Void
```

## Best Practices

1. **Clear Naming**: Use descriptive names for commands and parameters
2. **Helpful Documentation**: Provide clear help text for all parameters
3. **Default Values**: Set sensible defaults when appropriate
4. **Error Handling**: Use Emrichen conditionals to handle missing parameters
5. **Query Organization**: Structure complex queries using Emrichen tags for better maintainability
6. **Type Safety**: Use appropriate parameter types to ensure data validity

## Common Patterns

1. **Optional Filters**:
```yaml
- !If
  test: !Exists filter_field
  then:
    match:
      field: !Var filter_field
  else: !Void
```

2. **Multi-field Search**:
```yaml
multi_match:
  query: !Var search_text
  fields: ["field1^3", "field2^2", "field3"]
```

3. **Date Range Queries**:
```yaml
range:
  timestamp:
    gte: !Var start_date
    lte: !Var end_date
```

