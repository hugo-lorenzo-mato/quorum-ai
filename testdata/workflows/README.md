# Workflow Test Fixtures

This directory contains JSON fixtures for testing workflow creation with different execution mode configurations.

## Files

### single_agent_config.json
Test fixture for single-agent execution mode:
- `execution_mode: "single_agent"`
- Specifies `single_agent_name` and optional `single_agent_model`
- Used for testing single-agent workflow creation via API

### multi_agent_config.json
Test fixture for multi-agent execution mode:
- `execution_mode: "multi_agent"`
- Includes `consensus_threshold` configuration
- Used for testing multi-agent consensus workflow creation

### default_config.json
Test fixture for default workflow configuration:
- No explicit `execution_mode` set
- Tests backward compatibility (defaults to multi-agent mode)
- Used for verifying default behavior

## Usage

### In Go Integration Tests
```go
func loadTestFixture(t *testing.T, name string) map[string]interface{} {
    data, err := os.ReadFile(filepath.Join("testdata/workflows", name))
    require.NoError(t, err)

    var fixture map[string]interface{}
    require.NoError(t, json.Unmarshal(data, &fixture))
    return fixture
}
```

### In E2E Tests
```typescript
import singleAgentConfig from '../../testdata/workflows/single_agent_config.json';

test('create single-agent workflow', async ({ request }) => {
  const response = await request.post('/api/v1/workflows', {
    data: singleAgentConfig,
  });
  expect(response.ok()).toBeTruthy();
});
```

### In CLI Tests
```bash
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d @testdata/workflows/single_agent_config.json
```
