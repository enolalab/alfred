package domain

import "encoding/json"

type Tool struct {
	ID          string
	Name        string
	Description string
	Parameters  json.RawMessage
}
