package domain

type PrometheusQueryResult struct {
	Query      string             `json:"query"`
	ExecutedAt string             `json:"executed_at"`
	ResultType string             `json:"result_type"`
	Series     []PrometheusSeries `json:"series"`
	Truncated  bool               `json:"truncated"`
	Warnings   []string           `json:"warnings,omitempty"`
}

type PrometheusSeries struct {
	Metric map[string]string  `json:"metric"`
	Values []PrometheusSample `json:"values"`
}

type PrometheusSample struct {
	Timestamp string `json:"timestamp"`
	Value     string `json:"value"`
}
