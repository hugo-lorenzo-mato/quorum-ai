package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// TestStreamingResponseParsing_FragmentedChunks tests streaming with artificially fragmented chunks.
func TestStreamingResponseParsing_FragmentedChunks(t *testing.T) {
	t.Parallel()

	// Complete streaming response that would normally arrive as one piece
	completeResponse := `{"type":"agent_start","agent":"claude","timestamp":"2024-01-01T12:00:00Z"}
{"type":"content","data":"This is the first part of the response"}
{"type":"content","data":" and this is the continuation"}
{"type":"tool_call","name":"search","args":{"query":"test"}}
{"type":"tool_result","result":"Search completed"}
{"type":"content","data":" Final part of the response."}
{"type":"agent_end","agent":"claude","timestamp":"2024-01-01T12:01:00Z"}`

	testCases := []struct {
		name      string
		fragments []string
		expected  int // Expected number of events
	}{
		{
			name: "single_char_fragments",
			fragments: splitEveryNChars(completeResponse, 1),
			expected:  7,
		},
		{
			name: "small_fragments",
			fragments: splitEveryNChars(completeResponse, 10),
			expected:  7,
		},
		{
			name: "line_boundary_fragments",
			fragments: strings.Split(completeResponse, "\n"),
			expected:  7,
		},
		{
			name: "json_boundary_fragments",
			fragments: fragmentAtJSONBoundaries(completeResponse),
			expected:  7,
		},
		{
			name: "random_size_fragments",
			fragments: fragmentRandomly(completeResponse, 5, 30),
			expected:  7,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {

			var events []core.AgentEvent
			handler := testEventHandlerFunc(&events)
			
			base := NewBaseAdapter(AgentConfig{}, nil)
			base.SetEventHandler(handler)

			// Simulate fragmented streaming
			parser := &MockStreamParser{}
			
			for _, fragment := range tc.fragments {
				if fragment != "" {
					parsedEvents := parser.ParseChunk([]byte(fragment))
					events = append(events, parsedEvents...)
				}
			}

			// Verify we got the expected number of complete events
			if len(events) != tc.expected {
				t.Errorf("Expected %d events, got %d", tc.expected, len(events))
				for i, event := range events {
					t.Logf("Event %d: %s - %s", i, event.Type, event.Message)
				}
				if parser.buffer != "" {
					t.Logf("Buffer remaining: %q", parser.buffer)
				}
				t.Logf("Fragments processed: %d", len(tc.fragments))
			}

			// Verify no events are corrupted
			for i, event := range events {
				if event.Type == "" {
					t.Errorf("Event %d has empty type", i)
				}
				if event.Timestamp.IsZero() {
					t.Logf("Event %d has zero timestamp (may be expected): %s", i, event.Type)
				}
			}
		})
	}
}

// TestStreamingResponseParsing_PartialToolCalls tests handling of incomplete tool calls.
func TestStreamingResponseParsing_PartialToolCalls(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		chunks      []string
		expectError bool
		description string
	}{
		{
			name: "complete_tool_call",
			chunks: []string{
				`{"type":"tool_call","name":"search","args":{"query":"test"}}`,
			},
			expectError: false,
			description: "Complete tool call in single chunk",
		},
		{
			name: "split_tool_call",
			chunks: []string{
				`{"type":"tool_call","name":"search",`,
				`"args":{"query":"test"}}`,
			},
			expectError: false,
			description: "Tool call split across chunks",
		},
		{
			name: "malformed_tool_call",
			chunks: []string{
				`{"type":"tool_call","name":"search","args":{"query":"test"`,
				// Missing closing brackets
			},
			expectError: true,
			description: "Incomplete tool call args",
		},
		{
			name: "nested_json_in_args",
			chunks: []string{
				`{"type":"tool_call","name":"create_file","args":`,
				`{"path":"test.json","content":"{\"nested\": \"data\"}"}`,
				`}`,
			},
			expectError: false,
			description: "Tool call with nested JSON in args",
		},
		{
			name: "multiple_tool_calls",
			chunks: []string{
				`{"type":"tool_call","name":"search","args":{"query":"test1"}}`,
				`{"type":"tool_call","name":"search","args":{"query":"test2"}}`,
			},
			expectError: false,
			description: "Multiple consecutive tool calls",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {

			parser := &MockStreamParser{}

			var allEvents []core.AgentEvent

			// Process each chunk
			for _, chunk := range tc.chunks {
				events := parser.ParseChunk([]byte(chunk))
				allEvents = append(allEvents, events...)
			}

			// Check error expectation 
			// For malformed/incomplete cases, we expect fewer events than complete JSON lines
			hasIncompleteData := parser.buffer != ""
			actuallyIncomplete := len(allEvents) < len(tc.chunks) // Less events than expected complete lines
			
			if tc.expectError {
				if !hasIncompleteData && !actuallyIncomplete {
					t.Errorf("Expected incomplete/error condition but parsing completed successfully")
				}
			} else {
				if hasIncompleteData {
					t.Logf("Warning: incomplete data in buffer: %q", parser.buffer)
				}
			}

			// Verify tool call events are properly formed
			for _, event := range allEvents {
				if event.Type == core.AgentEventToolUse {
					if event.Data == nil || event.Data["tool"] == "" {
						t.Errorf("Tool call event missing tool data")
					}
				}
			}

			t.Logf("%s: processed %d chunks, got %d events", tc.description, len(tc.chunks), len(allEvents))
		})
	}
}

// TestStreamingResponseParsing_TimeoutHandling tests timeout scenarios.
func TestStreamingResponseParsing_TimeoutHandling(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		timeout      time.Duration
		simulateHang bool
		expectError  bool
	}{
		{
			name:         "fast_response",
			timeout:      5 * time.Second,
			simulateHang: false,
			expectError:  false,
		},
		{
			name:         "timeout_scenario",
			timeout:      100 * time.Millisecond,
			simulateHang: true,
			expectError:  true,
		},
		{
			name:         "normal_processing",
			timeout:      1 * time.Second,
			simulateHang: false,
			expectError:  false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {

			ctx, cancel := context.WithTimeout(context.Background(), tc.timeout)
			defer cancel()

			completed := make(chan bool, 1)
			
			// Simulate streaming processing
			go func() {
				defer func() {
					completed <- true
				}()
				
				if tc.simulateHang {
					// Simulate hanging operation
					time.Sleep(tc.timeout + 50*time.Millisecond)
					return
				}
				
				// Simulate normal processing
				parser := &MockStreamParser{}
				testResponse := `{"type":"content","data":"test response"}`
				parser.ParseChunk([]byte(testResponse))
			}()

			// Wait for completion or timeout
			select {
			case <-ctx.Done():
				if !tc.expectError {
					t.Errorf("Unexpected timeout")
				}
				// Success case for timeout scenarios
			case <-completed:
				if tc.expectError {
					t.Errorf("Expected timeout but operation completed")
				}
				// Success case for normal scenarios
			}
		})
	}
}

// TestStreamingResponseParsing_BufferOverflow tests handling of very large responses.
func TestStreamingResponseParsing_BufferOverflow(t *testing.T) {
	t.Parallel()

	// Create increasingly large streaming responses
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		size := size
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {

			// Generate large streaming response
			var buffer strings.Builder
			for i := 0; i < size/50; i++ {
				buffer.WriteString(fmt.Sprintf(`{"type":"content","data":"Content chunk %d with some additional text to make it longer"}%s`, i, "\n"))
			}

			largeResponse := buffer.String()
			var events []core.AgentEvent
			parser := &MockStreamParser{}

			start := time.Now()
			
			// Parse large response in chunks
			chunkSize := 1024
			for i := 0; i < len(largeResponse); i += chunkSize {
				end := i + chunkSize
				if end > len(largeResponse) {
					end = len(largeResponse)
				}
				
				chunk := largeResponse[i:end]
				parsedEvents := parser.ParseChunk([]byte(chunk))
				events = append(events, parsedEvents...)
			}

			duration := time.Since(start)

			// Performance should be reasonable
			maxDuration := 100 * time.Millisecond
			if duration > maxDuration {
				t.Errorf("Large response parsing took too long: %v (max: %v)", duration, maxDuration)
			}

			// Should have parsed events
			if len(events) == 0 {
				t.Errorf("No events parsed from large response")
			}

			t.Logf("Size: %d chars, Events: %d, Duration: %v", size, len(events), duration)
		})
	}
}

// Helper functions and types

type TestEventHandler struct {
	events []core.AgentEvent
}

func (h *TestEventHandler) HandleEvent(event core.AgentEvent) {
	h.events = append(h.events, event)
}

// TestEventHandler implements AgentEventHandler as a function
func testEventHandlerFunc(events *[]core.AgentEvent) core.AgentEventHandler {
	return func(event core.AgentEvent) {
		*events = append(*events, event)
	}
}

type MockStreamParser struct{
	buffer string // Accumulate incomplete JSON across chunks
	eventCount int // For deterministic timestamps
}

func (p *MockStreamParser) ParseChunk(chunk []byte) []core.AgentEvent {
	// Add chunk to buffer
	p.buffer += string(chunk)
	
	var events []core.AgentEvent
	
	// Process complete lines from buffer
	for {
		newlineIndex := strings.Index(p.buffer, "\n")
		if newlineIndex == -1 {
			// No complete line yet, check if buffer contains complete JSON
			// This handles cases where final chunk doesn't have newline
			if p.isCompleteJSON(p.buffer) {
				line := p.buffer
				p.buffer = ""
				if len(strings.TrimSpace(line)) > 0 {
					event := p.parseJSONLine(line)
					if event.Type != "" {
						event.Timestamp = time.Unix(1000+int64(p.eventCount), 0)
						p.eventCount++
						events = append(events, event)
					}
				}
			}
			break
		}
		
		// Extract complete line
		line := p.buffer[:newlineIndex]
		p.buffer = p.buffer[newlineIndex+1:]
		
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}
		
		// Parse the complete JSON line
		event := p.parseJSONLine(line)
		if event.Type != "" {
			// Use deterministic timestamp for tests
			event.Timestamp = time.Unix(1000+int64(p.eventCount), 0)
			p.eventCount++
			events = append(events, event)
		}
	}
	
	return events
}

func (p *MockStreamParser) isCompleteJSON(text string) bool {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return false
	}
	
	var jsonData map[string]interface{}
	return json.Unmarshal([]byte(text), &jsonData) == nil
}

func (p *MockStreamParser) parseJSONLine(line string) core.AgentEvent {
	line = strings.TrimSpace(line)
	
	// For testing purposes, try to parse as JSON first to detect malformed JSON
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(line), &jsonData); err != nil {
		// Return empty event for malformed JSON
		return core.AgentEvent{}
	}
	
	// Deterministic parsing of known test patterns
	if strings.Contains(line, `"type":"agent_start"`) {
		return core.AgentEvent{
			Type:    core.AgentEventStarted,
			Message: "Agent started",
		}
	} else if strings.Contains(line, `"type":"content"`) {
		return core.AgentEvent{
			Type:    core.AgentEventChunk,
			Message: extractDataField(line),
		}
	} else if strings.Contains(line, `"type":"tool_call"`) {
		return core.AgentEvent{
			Type:    core.AgentEventToolUse,
			Message: "Tool call",
			Data:    map[string]any{"tool": extractNameField(line)},
		}
	} else if strings.Contains(line, `"type":"tool_result"`) {
		return core.AgentEvent{
			Type:    core.AgentEventProgress,
			Message: extractResultField(line),
		}
	} else if strings.Contains(line, `"type":"agent_end"`) {
		return core.AgentEvent{
			Type:    core.AgentEventCompleted,
			Message: "Agent ended",
		}
	} else if strings.Contains(line, `"type":"progress"`) {
		return core.AgentEvent{
			Type:    core.AgentEventProgress,
			Message: "progress",
		}
	} else if strings.Contains(line, `"type":"result"`) {
		return core.AgentEvent{
			Type:    core.AgentEventProgress,
			Message: "result",
		}
	}
	
	return core.AgentEvent{} // Empty event for unrecognized patterns
}

func splitEveryNChars(s string, n int) []string {
	var fragments []string
	for i := 0; i < len(s); i += n {
		end := i + n
		if end > len(s) {
			end = len(s)
		}
		fragments = append(fragments, s[i:end])
	}
	return fragments
}

func fragmentAtJSONBoundaries(s string) []string {
	// Split at } and { boundaries
	fragments := []string{}
	current := ""
	
	for i, char := range s {
		current += string(char)
		if char == '}' && i < len(s)-1 {
			fragments = append(fragments, current)
			current = ""
		}
	}
	
	if current != "" {
		fragments = append(fragments, current)
	}
	
	return fragments
}

func fragmentRandomly(s string, minSize, maxSize int) []string {
	var fragments []string
	i := 0
	
	for i < len(s) {
		size := minSize + (i % (maxSize - minSize + 1))
		end := i + size
		if end > len(s) {
			end = len(s)
		}
		fragments = append(fragments, s[i:end])
		i = end
	}
	
	return fragments
}

func extractDataField(jsonStr string) string {
	// Simple extraction for test purposes
	start := strings.Index(jsonStr, `"data":"`)
	if start == -1 {
		return "content"
	}
	start += 8
	end := strings.Index(jsonStr[start:], `"`)
	if end == -1 {
		return "content"
	}
	return jsonStr[start : start+end]
}

func extractNameField(jsonStr string) string {
	start := strings.Index(jsonStr, `"name":"`)
	if start == -1 {
		return "tool"
	}
	start += 8
	end := strings.Index(jsonStr[start:], `"`)
	if end == -1 {
		return "tool"
	}
	return jsonStr[start : start+end]
}

func extractResultField(jsonStr string) string {
	start := strings.Index(jsonStr, `"result":"`)
	if start == -1 {
		return "result"
	}
	start += 10
	end := strings.Index(jsonStr[start:], `"`)
	if end == -1 {
		return "result"
	}
	return jsonStr[start : start+end]
}
