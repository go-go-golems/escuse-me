<think>
Based on the request, I need to create a plan to add support for OpenSearch to the existing Elasticsearch client. Let me analyze the current structure and understand what needs to be modified:

1. The project appears to be using Elasticsearch client libraries.
2. We need to add support for OpenSearch while maintaining compatibility with Elasticsearch.
3. The changes will affect multiple parts of the codebase, including client initialization and command execution.

From the provided file contents, I see:
- `pkg/cmds/layers/settings.go` contains ES client settings and initialization code
- `cmd/escuse-me/cmds/indices/ls.go` contains a command to list indices
- The detailed migration guide in `ttmp/2025-03-07/01-add-opensearch-go.md`

Let me outline a plan for implementing OpenSearch support:
</think>

# Implementation Plan: Adding OpenSearch Support

## Progress Update

### Completed Steps:
- [x] Added OpenSearch Go client dependency (`github.com/opensearch-project/opensearch-go/v4`)
- [x] Added client type selection to es.yaml configuration
- [x] Created SearchClient interface for abstracting both clients
- [x] Implemented client wrappers for both Elasticsearch and OpenSearch
- [x] Updated ls command to use the new interface
- [x] Implemented ListIndices for both client types

### OpenSearch API Implementation Notes
After investigating the OpenSearch API, we found that:

1. The OpenSearch Go client provides a lower-level API compared to Elasticsearch:
   - Uses raw HTTP requests with `Client.Perform()`
   - Requires manual URL and query parameter construction
   - No high-level abstractions like `Cat.Indices()` as in Elasticsearch

2. API Compatibility:
   - OpenSearch maintains API endpoint compatibility with Elasticsearch
   - Same endpoints (e.g., `/_cat/indices`) work for both systems
   - Query parameters and response formats are consistent

3. Implementation Strategy:
   ```go
   // OpenSearch implementation
   func (c *OpenSearchClient) ListIndices(ctx context.Context) ([]byte, error) {
       req := &http.Request{
           Method: "GET",
           URL: &url.URL{
               Path: "/_cat/indices",
               RawQuery: "format=json&h=health,status,index,...",
           },
       }
       resp, err := c.Client.Perform(req)
       // ... handle response
   }

   // vs Elasticsearch implementation
   func (c *ElasticsearchClient) ListIndices(ctx context.Context) ([]byte, error) {
       res, err := c.Cat.Indices(c.Cat.Indices.WithFormat("json"))
       // ... handle response
   }
   ```

### Next Steps:
1. Testing & Validation for ls command
   - [ ] Test with Elasticsearch instance
   - [ ] Test with OpenSearch instance
   - [ ] Verify output format consistency

2. Future Command Adaptations
   - [ ] Create helper functions for common OpenSearch API operations
   - [ ] Document OpenSearch API usage patterns
   - [ ] Update other indices commands with similar patterns

## Testing Strategy
1. Manual Testing
   - Test with Elasticsearch 7.x/8.x
   - Test with OpenSearch 1.x/2.x
   - Verify output format consistency

2. Integration Tests (TODO)
   - Add test cases for both client types
   - Test error handling
   - Test configuration parsing

## Notes
- OpenSearch client requires manual request construction
- Most API endpoints are compatible between both systems
- Consider creating helper functions for common OpenSearch operations

## Next Steps
1. Add tests for the ls command implementation
2. Document the client type selection in README
3. Create helper functions for OpenSearch API operations
4. Plan migration of other commands
