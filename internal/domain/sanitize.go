package domain

import "strings"

// SanitizeInput strips known prompt injection patterns from user input.
func SanitizeInput(input string) string {
	input = strings.ReplaceAll(input, "\x00", "")

	patterns := []string{
		"<|im_start|>system",
		"<|im_start|>",
		"<|im_end|>",
		"<<SYS>>",
		"<</SYS>>",
		"[INST]",
		"[/INST]",
	}
	for _, p := range patterns {
		input = strings.ReplaceAll(input, p, "")
	}

	return strings.TrimSpace(input)
}
