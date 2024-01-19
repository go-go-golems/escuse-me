package helpers

import "encoding/json"

type ElasticsearchError struct {
	Error struct {
		RootCause []struct {
			Type      string `json:"type"`
			Reason    string `json:"reason"`
			IndexUUID string `json:"index_uuid,omitempty"`
			Index     string `json:"index,omitempty"`
		} `json:"root_cause"`
		Type      string `json:"type"`
		Reason    string `json:"reason"`
		IndexUUID string `json:"index_uuid,omitempty"`
		Index     string `json:"index,omitempty"`
	} `json:"error"`
	Status int `json:"status"`
}

// ParseErrorResponse parses the JSON response and checks for the error schema.
func ParseErrorResponse(jsonData []byte) (*ElasticsearchError, bool) {
	var esError ElasticsearchError
	if err := json.Unmarshal(jsonData, &esError); err != nil {
		return nil, false
	}
	if esError.Status == 0 {
		return nil, false
	}
	return &esError, true
}
