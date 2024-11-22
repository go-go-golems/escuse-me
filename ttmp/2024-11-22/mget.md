Elastic Docs ›Elasticsearch Guide [8.16] ›REST APIs ›Document APIs
Multi get (mget) API
edit

Retrieves multiple JSON documents by ID.

GET /_mget
{
  "docs": [
    {
      "_index": "my-index-000001",
      "_id": "1"
    },
    {
      "_index": "my-index-000001",
      "_id": "2"
    }
  ]
}

Copy as curl
Try in Elastic
 
Request
edit

GET /_mget

GET /<index>/_mget
Prerequisites
edit

    If the Elasticsearch security features are enabled, you must have the read index privilege for the target index or index alias.

Description
edit

You use mget to retrieve multiple documents from one or more indices. If you specify an index in the request URI, you only need to specify the document IDs in the request body.
Security
edit

See URL-based access control.
Partial responses
edit

To ensure fast responses, the multi get API responds with partial results if one or more shards fail. See Shard failures for more information.
Path parameters
edit

<index>
    (Optional, string) Name of the index to retrieve documents from when ids are specified, or when a document in the docs array does not specify an index. 

Query parameters
edit

preference
    (Optional, string) Specifies the node or shard the operation should be performed on. Random by default. 
realtime
    (Optional, Boolean) If true, the request is real-time as opposed to near-real-time. Defaults to true. See Realtime. 
refresh
    (Optional, Boolean) If true, the request refreshes relevant shards before retrieving documents. Defaults to false. 
routing
    (Optional, string) Custom value used to route operations to a specific shard. 
stored_fields
    (Optional, string) A comma-separated list of stored fields to include in the response. 
_source
    (Optional, string) True or false to return the _source field or not, or a list of fields to return. 
_source_excludes

    (Optional, string) A comma-separated list of source fields to exclude from the response.

    You can also use this parameter to exclude fields from the subset specified in _source_includes query parameter.

    If the _source parameter is false, this parameter is ignored.
_source_includes

    (Optional, string) A comma-separated list of source fields to include in the response.

    If this parameter is specified, only these source fields are returned. You can exclude fields from this subset using the _source_excludes query parameter.

    If the _source parameter is false, this parameter is ignored.

Request body
edit

docs

    (Optional, array) The documents you want to retrieve. Required if no index is specified in the request URI. You can specify the following attributes for each document:

    _id
        (Required, string) The unique document ID. 
    _index
        (Optional, string) The index that contains the document. Required if no index is specified in the request URI. 
    routing
        (Optional, string) The key for the primary shard the document resides on. Required if routing is used during indexing. 
    _source

        (Optional, Boolean) If false, excludes all _source fields. Defaults to true.

        source_include
            (Optional, array) The fields to extract and return from the _source field. 
        source_exclude
            (Optional, array) The fields to exclude from the returned _source field. 

    _stored_fields
        (Optional, array) The stored fields you want to retrieve. 

ids
    (Optional, array) The IDs of the documents you want to retrieve. Allowed when the index is specified in the request URI. 

Response body
edit

The response includes a docs array that contains the documents in the order specified in the request. The structure of the returned documents is similar to that returned by the get API. If there is a failure getting a particular document, the error is included in place of the document.
Examples
edit
Get documents by ID
edit

If you specify an index in the request URI, only the document IDs are required in the request body:

GET /my-index-000001/_mget
{
  "docs": [
    {
      "_id": "1"
    },
    {
      "_id": "2"
    }
  ]
}

Copy as curl
Try in Elastic
 

You can use the ids element to simplify the request:

GET /my-index-000001/_mget
{
  "ids" : ["1", "2"]
}

Copy as curl
Try in Elastic
 
Filter source fields
edit

By default, the _source field is returned for every document (if stored). Use the _source and _source_include or source_exclude attributes to filter what fields are returned for a particular document. You can include the _source, _source_includes, and _source_excludes query parameters in the request URI to specify the defaults to use when there are no per-document instructions.

For example, the following request sets _source to false for document 1 to exclude the source entirely, retrieves field3 and field4 from document 2, and retrieves the user field from document 3 but filters out the user.location field.

GET /_mget
{
  "docs": [
    {
      "_index": "test",
      "_id": "1",
      "_source": false
    },
    {
      "_index": "test",
      "_id": "2",
      "_source": [ "field3", "field4" ]
    },
    {
      "_index": "test",
      "_id": "3",
      "_source": {
        "include": [ "user" ],
        "exclude": [ "user.location" ]
      }
    }
  ]
}

Copy as curl
Try in Elastic
 
Get stored fields
edit

Use the stored_fields attribute to specify the set of stored fields you want to retrieve. Any requested fields that are not stored are ignored. You can include the stored_fields query parameter in the request URI to specify the defaults to use when there are no per-document instructions.

For example, the following request retrieves field1 and field2 from document 1, and field3 and field4 from document 2:

GET /_mget
{
  "docs": [
    {
      "_index": "test",
      "_id": "1",
      "stored_fields": [ "field1", "field2" ]
    },
    {
      "_index": "test",
      "_id": "2",
      "stored_fields": [ "field3", "field4" ]
    }
  ]
}

Copy as curl
Try in Elastic
 

The following request retrieves field1 and field2 from all documents by default. These default fields are returned for document 1, but overridden to return field3 and field4 for document 2.

GET /test/_mget?stored_fields=field1,field2
{
  "docs": [
    {
      "_id": "1"
    },
    {
      "_id": "2",
      "stored_fields": [ "field3", "field4" ]
    }
  ]
}

Copy as curl
Try in Elastic
 
Specify document routing
edit

If routing is used during indexing, you need to specify the routing value to retrieve documents. For example, the following request fetches test/_doc/2 from the shard corresponding to routing key key1, and fetches test/_doc/1 from the shard corresponding to routing key key2.

GET /_mget?routing=key1
{
  "docs": [
    {
      "_index": "test",
      "_id": "1",
      "routing": "key2"
    },
    {
      "_index": "test",
      "_id": "2"
    }
  ]
}

# Multi Get Document Command (mget)

The `mget` command allows you to retrieve multiple documents from Elasticsearch in a single request. This is more efficient than making multiple individual get requests.

## Syntax

```bash
escuse-me documents mget [flags]
```

## Required Parameters

- `--index`: The name of the index to retrieve documents from
- `--ids`: Comma-separated list of document IDs to retrieve

## Optional Parameters

- `--source-includes`: Fields to include in the returned _source field
- `--source-excludes`: Fields to exclude from the returned _source field
- `--stored-fields`: List of stored fields to retrieve
- `--routing`: Custom routing value
- `--preference`: Preference for which shard/node to execute the request on
- `--realtime`: Whether to perform the operation in realtime (default: true)
- `--refresh`: Whether to refresh the relevant shards before operation

## Examples

1. Basic usage - retrieve multiple documents by ID:
```bash
escuse-me documents mget --index products --ids "1,2,3"
```

2. Retrieve specific fields only:
```bash
escuse-me documents mget --index products --ids "1,2,3" --source-includes "title,price"
```

3. Exclude certain fields:
```bash
escuse-me documents mget --index products --ids "1,2,3" --source-excludes "description,metadata"
```

4. Retrieve with custom routing and refresh:
```bash
escuse-me documents mget --index products --ids "1,2,3" --routing "user1" --refresh true
```

## Request Body Format

Internally, the command constructs an Elasticsearch request body in the following format:

```json
{
  "docs": [
    {
      "_id": "1",
      "_index": "products",
      "_source": ["field1", "field2"]
    },
    {
      "_id": "2",
      "_index": "products",
      "_source": ["field1", "field2"]
    }
  ]
}
```

## Response Format

The response includes the requested documents and their metadata:

```json
{
  "docs": [
    {
      "_index": "products",
      "_id": "1",
      "_version": 1,
      "_seq_no": 0,
      "_primary_term": 1,
      "found": true,
      "_source": {
        "field1": "value1",
        "field2": "value2"
      }
    },
    {
      "_index": "products",
      "_id": "2",
      "found": true,
      "_source": {
        "field1": "value3",
        "field2": "value4"
      }
    }
  ]
}
```

## Notes

- The command is optimized for retrieving multiple documents in a single request
- All documents must be from the same index
- Missing documents will be indicated in the response with `"found": false`
- Source filtering (includes/excludes) applies to all requested documents
