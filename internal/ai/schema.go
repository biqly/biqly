package ai

import (
	"github.com/biqly/biqly/internal/query"
)

// AIRequest is the input for the AI query endpoint.
type AIRequest struct {
	DatasourceID string `json:"datasource_id"`
	ModelID      string `json:"model_id"`
	Question     string `json:"question"`
}

// AIResponse is the output from the AI query endpoint.
type AIResponse struct {
	LogicalQuery *query.LogicalQuery `json:"logical_query,omitempty"`
	SQL          string              `json:"sql,omitempty"`
	Args         []any               `json:"args,omitempty"`
	Warnings     []string            `json:"warnings,omitempty"`
	Result       *query.QueryResult  `json:"result,omitempty"`
	Confidence   float64             `json:"confidence"`
}

// LogicalQuerySchema defines the JSON schema the AI must output.
const LogicalQuerySchema = `{
  "type": "object",
  "required": ["select"],
  "properties": {
    "datasource_id": {"type": "string"},
    "model_id": {"type": "string"},
    "select": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["type", "name"],
        "properties": {
          "type": {"type": "string", "enum": ["dimension", "metric"]},
          "name": {"type": "string"},
          "alias": {"type": "string"}
        }
      }
    },
    "filters": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["field", "operator"],
        "properties": {
          "field": {"type": "string"},
          "operator": {"type": "string", "enum": ["eq","neq","gt","gte","lt","lte","in","not_in","contains","starts_with","ends_with","between","is_null","is_not_null"]},
          "value": {}
        }
      }
    },
    "group_by": {
      "type": "array",
      "items": {"type": "object", "required": ["field"], "properties": {"field": {"type": "string"}}}
    },
    "order_by": {
      "type": "array",
      "items": {"type": "object", "required": ["field"], "properties": {"field": {"type": "string"}, "direction": {"type": "string", "enum": ["asc", "desc"]}}}
    },
    "limit": {"type": "integer", "minimum": 0},
    "offset": {"type": "integer", "minimum": 0}
  },
  "additionalProperties": false
}`
