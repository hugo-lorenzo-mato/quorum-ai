package chat

import (
	"strings"
	"testing"
)

func TestFormatUserMessage(t *testing.T) {
	styles := NewMessageStyles(100)

	result := styles.FormatUserMessage("hola", "14:30", false)

	// Should contain "You" header
	if !strings.Contains(result, "You") {
		t.Error("User message should contain 'You' header")
	}
	// Should contain content
	if !strings.Contains(result, "hola") {
		t.Error("User message should contain content")
	}
	// Should contain timestamp
	if !strings.Contains(result, "14:30") {
		t.Error("User message should contain timestamp")
	}
	// Should have rounded border (bubble style)
	if !strings.Contains(result, "╭") || !strings.Contains(result, "╯") {
		t.Error("User message should have rounded border (bubble)")
	}
}

func TestFormatUserMessageCommand(t *testing.T) {
	styles := NewMessageStyles(100)

	result := styles.FormatUserMessage("/status", "14:31", true)

	// Should contain command with icon
	if !strings.Contains(result, "⚡") {
		t.Error("Command message should contain lightning icon")
	}
	if !strings.Contains(result, "/status") {
		t.Error("Command message should contain command text")
	}
}

func TestFormatBotMessage(t *testing.T) {
	styles := NewMessageStyles(100)

	result := styles.FormatBotMessage("claude", "Hello world", "14:32", 0, "")

	// Should contain agent name
	if !strings.Contains(result, "Claude") {
		t.Error("Message should contain agent name")
	}
	// Should contain content
	if !strings.Contains(result, "Hello world") {
		t.Error("Message should contain content")
	}
	// Should contain timestamp
	if !strings.Contains(result, "14:32") {
		t.Error("Message should contain timestamp")
	}
	// Should have rounded border (bubble style)
	if !strings.Contains(result, "╭") || !strings.Contains(result, "╯") {
		t.Error("Bot message should have rounded border (bubble)")
	}
}

func TestFormatSystemMessage(t *testing.T) {
	styles := NewMessageStyles(100)

	result := styles.FormatSystemMessage("Session restored")

	// Should contain gear icon
	if !strings.Contains(result, "⚙") {
		t.Error("System message should contain gear icon")
	}
	// Should contain content
	if !strings.Contains(result, "Session restored") {
		t.Error("System message should contain content")
	}
}

func TestUserAndBotHaveSameFormat(t *testing.T) {
	styles := NewMessageStyles(100)

	userMsg := styles.FormatUserMessage("test", "14:30", false)
	botMsg := styles.FormatBotMessage("claude", "test", "14:30", 0, "")

	// Both should have rounded border (bubble)
	if !strings.Contains(userMsg, "╭") {
		t.Error("User message should have rounded border")
	}
	if !strings.Contains(botMsg, "╭") {
		t.Error("Bot message should have rounded border")
	}

	// Both should have header + newline + content structure
	userLines := strings.Split(userMsg, "\n")
	botLines := strings.Split(botMsg, "\n")

	if len(userLines) < 2 {
		t.Error("User message should have header and content on separate lines")
	}
	if len(botLines) < 2 {
		t.Error("Bot message should have header and content on separate lines")
	}
}
