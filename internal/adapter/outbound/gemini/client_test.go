package gemini

import (
	"testing"

	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
)

func TestBuildContentsUsesToolNameForFunctionResponse(t *testing.T) {
	messages := []domain.Message{
		{
			Role:         vo.RoleTool,
			Content:      `{"result":"ok"}`,
			ToolResultID: "call-1",
			Metadata: map[string]string{
				"tool_name": domain.ToolShell,
			},
		},
	}

	contents := buildContents(messages)
	if got, want := contents[0].Parts[0].FunctionResponse.Name, domain.ToolShell; got != want {
		t.Fatalf("function response name = %q, want %q", got, want)
	}
}
