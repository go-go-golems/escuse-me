package cmds

import (
	"encoding/json"

	orderedmap "github.com/wk8/go-ordered-map/v2"
)

type SearchShards struct {
	Total      int `json:"total"`
	Successful int `json:"successful"`
	Skipped    int `json:"skipped"`
	Failed     int `json:"failed"`
}

type SearchTotal struct {
	Value    int    `json:"value"`
	Relation string `json:"relation"`
}

type SearchExplanation struct {
	Value       float64                  `json:"value"`
	Description string                   `json:"description"`
	Details     []map[string]interface{} `json:"details,omitempty"`
}

type SearchHit struct {
	Index       string                                      `json:"_index"`
	ID          string                                      `json:"_id"`
	Score       float64                                     `json:"_score"`
	Type        string                                      `json:"_type,omitempty"`
	Version     int64                                       `json:"_version,omitempty"`
	SeqNo       int64                                       `json:"_seq_no,omitempty"`
	PrimaryTerm int64                                       `json:"_primary_term,omitempty"`
	Found       bool                                        `json:"found,omitempty"`
	Routing     string                                      `json:"_routing,omitempty"`
	Source      *orderedmap.OrderedMap[string, interface{}] `json:"_source,omitempty"`
	Fields      map[string]interface{}                      `json:"fields,omitempty"`
	Highlight   map[string][]string                         `json:"highlight,omitempty"`
	Sort        []interface{}                               `json:"sort,omitempty"`
	Explanation *SearchExplanation                          `json:"_explanation,omitempty"`
}

type SearchHits struct {
	Total    SearchTotal `json:"total"`
	MaxScore float64     `json:"max_score"`
	Hits     []SearchHit `json:"hits,omitempty"`
}

type SuggestOption struct {
	Text      string  `json:"text"`
	Score     float64 `json:"score"`
	FreqScore float64 `json:"freq_score,omitempty"`
}

type SuggestResult struct {
	Text    string          `json:"text"`
	Offset  int             `json:"offset"`
	Length  int             `json:"length"`
	Options []SuggestOption `json:"options"`
}

type ProfileSearch struct {
	Query     []map[string]interface{} `json:"query"`
	Rewrite   []map[string]interface{} `json:"rewrite"`
	Collector []map[string]interface{} `json:"collector"`
}

type ProfileShard struct {
	ID           string                   `json:"id"`
	Searches     []ProfileSearch          `json:"searches"`
	Aggregations []map[string]interface{} `json:"aggregations"`
}

type SearchProfile struct {
	Shards []ProfileShard `json:"shards"`
}

type ElasticSearchResult struct {
	Took            int                        `json:"took"`
	TimedOut        bool                       `json:"timed_out"`
	Shards          SearchShards               `json:"_shards"`
	Hits            SearchHits                 `json:"hits"`
	Aggregations    map[string]json.RawMessage `json:"aggregations,omitempty"`
	Suggest         map[string][]SuggestResult `json:"suggest,omitempty"`
	Profile         *SearchProfile             `json:"profile,omitempty"`
	TerminatedEarly bool                       `json:"terminated_early,omitempty"`
	NumReducePhases int                        `json:"num_reduce_phases,omitempty"`
}
