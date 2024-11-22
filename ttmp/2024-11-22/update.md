Elastic Docs ›Elasticsearch Guide [8.16] ›REST APIs ›Document APIs
Update API
edit

Updates a document using the specified script.
Request
edit

POST /<index>/_update/<_id>
Prerequisites
edit

    If the Elasticsearch security features are enabled, you must have the index or write index privilege for the target index or index alias.

Description
edit

Enables you to script document updates. The script can update, delete, or skip modifying the document. The update API also supports passing a partial document, which is merged into the existing document. To fully replace an existing document, use the index API.

This operation:

    Gets the document (collocated with the shard) from the index.
    Runs the specified script.
    Indexes the result.

The document must still be reindexed, but using update removes some network roundtrips and reduces chances of version conflicts between the GET and the index operation.

The _source field must be enabled to use update. In addition to _source, you can access the following variables through the ctx map: _index, _type, _id, _version, _routing, and _now (the current timestamp).
Path parameters
edit

<index>
    (Required, string) Name of the target index. By default, the index is created automatically if it doesn’t exist. For more information, see Automatically create data streams and indices. 
<_id>
    (Required, string) Unique identifier for the document to be updated. 

Query parameters
edit

if_seq_no
    (Optional, integer) Only perform the operation if the document has this sequence number. See Optimistic concurrency control. 
if_primary_term
    (Optional, integer) Only perform the operation if the document has this primary term. See Optimistic concurrency control. 
lang
    (Optional, string) The script language. Default: painless. 
require_alias
    (Optional, Boolean) If true, the destination must be an index alias. Defaults to false. 
refresh
    (Optional, enum) If true, Elasticsearch refreshes the affected shards to make this operation visible to search, if wait_for then wait for a refresh to make this operation visible to search, if false do nothing with refreshes. Valid values: true, false, wait_for. Default: false. 
retry_on_conflict
    (Optional, integer) Specify how many times should the operation be retried when a conflict occurs. Default: 0. 
routing
    (Optional, string) Custom value used to route operations to a specific shard. 
_source
    (Optional, list) Set to false to disable source retrieval (default: true). You can also specify a comma-separated list of the fields you want to retrieve. 
_source_excludes
    (Optional, list) Specify the source fields you want to exclude. 
_source_includes
    (Optional, list) Specify the source fields you want to retrieve. 
timeout

    (Optional, time units) Period to wait for the following operations:

        Dynamic mapping updates
        Waiting for active shards

    Defaults to 1m (one minute). This guarantees Elasticsearch waits for at least the timeout before failing. The actual wait time could be longer, particularly when multiple waits occur.
wait_for_active_shards

    (Optional, string) The number of copies of each shard that must be active before proceeding with the operation. Set to all or any non-negative integer up to the total number of copies of each shard in the index (number_of_replicas+1). Defaults to 1, meaning to wait just for each primary shard to be active.

    See Active shards.

Examples
edit

First, let’s index a simple doc:

PUT test/_doc/1
{
  "counter" : 1,
  "tags" : ["red"]
}

Copy as curl
Try in Elastic
 

To increment the counter, you can submit an update request with the following script:

POST test/_update/1
{
  "script" : {
    "source": "ctx._source.counter += params.count",
    "lang": "painless",
    "params" : {
      "count" : 4
    }
  }
}

Copy as curl
Try in Elastic
 

Similarly, you could use and update script to add a tag to the list of tags (this is just a list, so the tag is added even it exists):

POST test/_update/1
{
  "script": {
    "source": "ctx._source.tags.add(params.tag)",
    "lang": "painless",
    "params": {
      "tag": "blue"
    }
  }
}

Copy as curl
Try in Elastic
 

You could also remove a tag from the list of tags. The Painless function to remove a tag takes the array index of the element you want to remove. To avoid a possible runtime error, you first need to make sure the tag exists. If the list contains duplicates of the tag, this script just removes one occurrence.

POST test/_update/1
{
  "script": {
    "source": "if (ctx._source.tags.contains(params.tag)) { ctx._source.tags.remove(ctx._source.tags.indexOf(params.tag)) }",
    "lang": "painless",
    "params": {
      "tag": "blue"
    }
  }
}

Copy as curl
Try in Elastic
 

You can also add and remove fields from a document. For example, this script adds the field new_field:

POST test/_update/1
{
  "script" : "ctx._source.new_field = 'value_of_new_field'"
}

Copy as curl
Try in Elastic
 

Conversely, this script removes the field new_field:

POST test/_update/1
{
  "script" : "ctx._source.remove('new_field')"
}

Copy as curl
Try in Elastic
 

The following script removes a subfield from an object field:

POST test/_update/1
{
  "script": "ctx._source['my-object'].remove('my-subfield')"
}

Copy as curl
Try in Elastic
 

Instead of updating the document, you can also change the operation that is executed from within the script. For example, this request deletes the doc if the tags field contains green, otherwise it does nothing (noop):

POST test/_update/1
{
  "script": {
    "source": "if (ctx._source.tags.contains(params.tag)) { ctx.op = 'delete' } else { ctx.op = 'noop' }",
    "lang": "painless",
    "params": {
      "tag": "green"
    }
  }
}

Copy as curl
Try in Elastic
 
Update part of a document
edit

The following partial update adds a new field to the existing document:

POST test/_update/1
{
  "doc": {
    "name": "new_name"
  }
}

Copy as curl
Try in Elastic
 

If both doc and script are specified, then doc is ignored. If you specify a scripted update, include the fields you want to update in the script.
Detect noop updates
edit

By default updates that don’t change anything detect that they don’t change anything and return "result": "noop":

POST test/_update/1
{
  "doc": {
    "name": "new_name"
  }
}

Copy as curl
Try in Elastic
 

If the value of name is already new_name, the update request is ignored and the result element in the response returns noop:

{
   "_shards": {
        "total": 0,
        "successful": 0,
        "failed": 0
   },
   "_index": "test",
   "_id": "1",
   "_version": 2,
   "_primary_term": 1,
   "_seq_no": 1,
   "result": "noop"
}

You can disable this behavior by setting "detect_noop": false:

POST test/_update/1
{
  "doc": {
    "name": "new_name"
  },
  "detect_noop": false
}

Copy as curl
Try in Elastic
 
Upsert
edit

If the document does not already exist, the contents of the upsert element are inserted as a new document. If the document exists, the script is executed:

POST test/_update/1
{
  "script": {
    "source": "ctx._source.counter += params.count",
    "lang": "painless",
    "params": {
      "count": 4
    }
  },
  "upsert": {
    "counter": 1
  }
}

Copy as curl
Try in Elastic
 
Scripted upsert
edit

To run the script whether or not the document exists, set scripted_upsert to true:

POST test/_update/1
{
  "scripted_upsert": true,
  "script": {
    "source": """
      if ( ctx.op == 'create' ) {
        ctx._source.counter = params.count
      } else {
        ctx._source.counter += params.count
      }
    """,
    "params": {
      "count": 4
    }
  },
  "upsert": {}
}

Copy as curl
Try in Elastic
 
Doc as upsert
edit

Instead of sending a partial doc plus an upsert doc, you can set doc_as_upsert to true to use the contents of doc as the upsert value:

POST test/_update/1
{
  "doc": {
    "name": "new_name"
  },
  "doc_as_upsert": true
}

