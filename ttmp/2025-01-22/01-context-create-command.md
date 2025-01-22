# Go-Emrichen: Documentation and Tutorial

## Introduction

Go-Emrichen is a powerful templating engine designed for generating YAML configurations with ease and precision. It's a Go implementation of the original Python Emrichen, bringing the same flexibility and robustness to Go developers. Go-Emrichen allows you to dynamically generate configuration files for a wide range of applications, including Kubernetes deployments, configuration management, and more.

## Installation

To use go-emrichen in your Go project, you first need to install it. Run the following command:

```
go get github.com/go-go-golems/go-emrichen
```

## Basic Usage

### Importing the Library

To use go-emrichen in your Go program, import it as follows:

```go
import (
    "github.com/go-go-golems/go-emrichen/pkg/emrichen"
    "gopkg.in/yaml.v3"
)
```

(make sure you have the yaml v3 import if you are using go modules.)

### Creating an Interpreter

The core of go-emrichen is the `Interpreter`. To create a new interpreter:

```go
interpreter, err := emrichen.NewInterpreter()
if err != nil {
    // Handle error
}
```

### Processing YAML

To process a YAML file with go-emrichen:

```go
func processFile(interpreter *emrichen.Interpreter, filePath string, w io.Writer) error {
    f, err := os.Open(filePath)
    if err != nil {
        return err
    }
    defer f.Close()

    decoder := yaml.NewDecoder(f)

    for {
        var document interface{}
        err = decoder.Decode(interpreter.CreateDecoder(&document))
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }

        // Skip empty documents
        if document == nil {
            continue
        }

        processedYAML, err := yaml.Marshal(&document)
        if err != nil {
            return err
        }

        _, err = w.Write(processedYAML)
        if err != nil {
            return err
        }
    }

    return nil
}
```

This function processes a YAML file, applying the Emrichen transformations, and writes the result to the provided writer.

## Advanced Usage

### Adding Variables

You can add variables to the Interpreter to use in your YAML templates:

```go
vars := map[string]interface{}{
    "environment": "production",
    "replicas": 3,
}

interpreter, err := emrichen.NewInterpreter(emrichen.WithVars(vars))
if err != nil {
    // Handle error
}
```

### Custom Functions

Go-Emrichen allows you to add custom functions to extend its capabilities:

```go
import "text/template"

customFuncs := template.FuncMap{
    "uppercase": strings.ToUpper,
    "lowercase": strings.ToLower,
}

interpreter, err := emrichen.NewInterpreter(emrichen.WithFuncMap(customFuncs))
if err != nil {
    // Handle error
}
```

### Additional Tags

You can add custom tags to enhance the functionality of go-emrichen:

```go
customTags := map[string]func(node *yaml.Node) (*yaml.Node, error){
    "!CustomTag": func(node *yaml.Node) (*yaml.Node, error) {
        // Implement custom tag logic
        return node, nil
    },
}

interpreter, err := emrichen.NewInterpreter(emrichen.WithAdditionalTags(customTags))
if err != nil {
    // Handle error
}
```

## Tutorial: Using Go-Emrichen in a Kubernetes Deployment

Let's create a simple program that uses go-emrichen to generate a Kubernetes deployment configuration.

1. Create a new Go file named `main.go`:

```go
package main

import (
    "fmt"
    "os"

    "github.com/go-go-golems/go-emrichen/pkg/emrichen"
    "gopkg.in/yaml.v3"
)

func main() {
    // Create an Interpreter with variables
    vars := map[string]interface{}{
        "APP_NAME": "myapp",
        "REPLICAS": 3,
        "IMAGE": "myapp:latest",
    }

    interpreter, err := emrichen.NewInterpreter(emrichen.WithVars(vars))
    if err != nil {
        fmt.Println("Error creating interpreter:", err)
        os.Exit(1)
    }

    // YAML template
    yamlTemplate := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .APP_NAME }}
spec:
  replicas: {{ .REPLICAS }}
  selector:
    matchLabels:
      app: {{ .APP_NAME }}
  template:
    metadata:
      labels:
        app: {{ .APP_NAME }}
    spec:
      containers:
      - name: {{ .APP_NAME }}
        image: {{ .IMAGE }}
`

    // Process the template
    var result interface{}
    err = yaml.Unmarshal([]byte(yamlTemplate), interpreter.CreateDecoder(&result))
    if err != nil {
        fmt.Println("Error processing template:", err)
        os.Exit(1)
    }

    // Marshal the result back to YAML
    output, err := yaml.Marshal(result)
    if err != nil {
        fmt.Println("Error marshaling result:", err)
        os.Exit(1)
    }

    // Print the result
    fmt.Println(string(output))
}
```

2. Run the program:

```
go run main.go
```

This will output a Kubernetes deployment YAML with the variables interpolated.

## Conclusion

Go-Emrichen provides a powerful way to template and generate YAML configurations in Go programs. By leveraging its features like variable interpolation, custom functions, and additional tags, you can create flexible and dynamic configuration generation systems.

For more advanced usage and a complete list of available tags, refer to the official documentation and examples in the go-emrichen repository:

You can find detailed documentation for each tag in the [doc section](pkg/doc/examples)
as well as an exhaustive list of examples in [the examples yamls](test-data)
and [in the go unit tests](pkg/emrichen/).


=== BEGIN: /home/manuel/code/wesen/corporate-headquarters/escuse-me/cmd/escuse-me/queries/examples/manuel.yaml ===
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
  - name: source_fields
    type: stringList
    help: Fields to return from ES index
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
  debug: !If
    test: !Exists debug
    then:
      input_parameters:
        first: !Exists first
        not_first: !Not,Exists first
        second: !Exists second
        not_second: !Not,Exists second
      debug:
        !If
        test: !All
          - !Not,Exists first
          - !Not,Exists second
        then: !Format "No first or second name"
        else: !Format "First name: {first}, second name: {second}"
    else: !Void
  query:
    !If
    test: !All
      - !Not,Exists first
      - !Not,Exists second
    then:
      match_all: { }
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
                  prefix_length: 0
                  max_expansions: 50
            else: !Void
          - !If
            test: !Exists second
            then:
              fuzzy:
                name.second:
                  value: !Var second
                  fuzziness: 2
                  prefix_length: 0
                  max_expansions: 50
            else: !Void

=== END: /home/manuel/code/wesen/corporate-headquarters/escuse-me/cmd/escuse-me/queries/examples/manuel.yaml ===


// Here are the types that can be used to define parameters in glazed:

package github.com/go-go-golems/glazed/pkg/cmds/parameters

File: pkg/cmds/parameters/parameter-type.go

const ParameterTypeString ParameterType = "string"
  // TODO(2023-02-13, manuel) Should the "default" of a stringFromFile be the filename, or the string?
  //
  // See https://github.com/go-go-golems/glazed/issues/137
const ParameterTypeStringFromFile ParameterType = "stringFromFile"
const ParameterTypeStringFromFiles ParameterType = "stringFromFiles"
  // ParameterTypeFile and ParameterTypeFileList are a more elaborate version that loads and parses
  // the file content and returns a list of FileData objects (or a single object in the case
  // of ParameterTypeFile).
const ParameterTypeFile ParameterType = "file"
const ParameterTypeFileList ParameterType = "fileList"
  // TODO(manuel, 2023-09-19) Add some more types and maybe revisit the entire concept of loading things from files
  // - string (potentially from file if starting with @)
  // - string/int/float list from file is another useful type
const ParameterTypeObjectListFromFile ParameterType = "objectListFromFile"
const ParameterTypeObjectListFromFiles ParameterType = "objectListFromFiles"
const ParameterTypeObjectFromFile ParameterType = "objectFromFile"
const ParameterTypeStringListFromFile ParameterType = "stringListFromFile"
const ParameterTypeStringListFromFiles ParameterType = "stringListFromFiles"
  // ParameterTypeKeyValue signals either a string with comma separate key-value options,
  // or when beginning with @, a file with key-value options
const ParameterTypeKeyValue ParameterType = "keyValue"
const ParameterTypeInteger ParameterType = "int"
const ParameterTypeFloat ParameterType = "float"
const ParameterTypeBool ParameterType = "bool"
const ParameterTypeDate ParameterType = "date"
const ParameterTypeStringList ParameterType = "stringList"
const ParameterTypeIntegerList ParameterType = "intList"
const ParameterTypeFloatList ParameterType = "floatList"
const ParameterTypeChoice ParameterType = "choice"
const ParameterTypeChoiceList ParameterType = "choiceList"

type FileData struct {
	Content          string
	ParsedContent    interface{}
	ParseError       error
	RawContent       []byte
	StringContent    string
	IsList           bool
	IsObject         bool
	BaseName         string
	Extension        string
	FileType         FileType
	Path             string
	RelativePath     string
	AbsolutePath     string
	Size             int64
	LastModifiedTime time.Time
	Permissions      os.FileMode
	IsDirectory      bool
}

