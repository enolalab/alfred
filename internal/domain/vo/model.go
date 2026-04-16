package vo

type ModelID string

const (
	ModelClaude4Sonnet ModelID = "claude-sonnet-4-20250514"
	ModelClaude4Opus   ModelID = "claude-opus-4-20250918"
	ModelGPT4o         ModelID = "gpt-4o"
	ModelGemini25Flash ModelID = "gemini-2.5-flash"
	ModelGemini25Pro   ModelID = "gemini-2.5-pro"
)

type ModelAPI string

const (
	ModelAPIAnthropic ModelAPI = "anthropic"
	ModelAPIOpenAI    ModelAPI = "openai"
	ModelAPIOllama    ModelAPI = "ollama"
	ModelAPIGemini    ModelAPI = "gemini"
)
