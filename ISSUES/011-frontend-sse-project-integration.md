# Frontend SSE Project Integration

## Summary

Update frontend Server-Sent Events (SSE) infrastructure to connect to project-scoped event streams, automatically reconnecting and refetching data when the project changes.

## Context

The frontend currently has a single global SSE connection that receives events for the single project. With multi-project support:

1. **Project-Scoped Endpoints**: SSE should connect to `/api/v1/projects/{projectId}/sse/events`
2. **Auto-Reconnect**: When project changes, close old connection and open new one
3. **Event Routing**: Route events to correct store based on project
4. **State Refresh**: Trigger data refresh when new project selected

## Implementation Details

### Files to Create

- `frontend/src/services/sseManager.js` - Singleton SSE manager
- `frontend/src/hooks/useSSE.js` - Hook for SSE access
- `frontend/src/hooks/useSSEEvent.js` - Hook for specific event types
- `frontend/src/hooks/useNetworkStatus.js` - Hook for connection status
- `frontend/src/components/SSEStatus.jsx` - Connection status indicator

### SSE Manager

Singleton service managing project-scoped connections:

```javascript
class SSEManager {
  constructor() {
    this.connections = new Map();      // projectId -> EventSource
    this.handlers = new Map();         // projectId -> Set<handler>
    this.reconnectTimers = new Map();
  }

  // Connect to specific project's SSE stream
  connect(projectId) {
    if (this.connections.has(projectId)) {
      return;  // Already connected
    }

    const url = `/api/v1/projects/${projectId}/sse/events`;
    const eventSource = new EventSource(url);

    eventSource.addEventListener('message', (event) => {
      this._routeEvent(projectId, event);
    });

    eventSource.addEventListener('error', () => {
      this._handleError(projectId);
    });

    this.connections.set(projectId, eventSource);
  }

  // Disconnect from project
  disconnect(projectId) {
    const source = this.connections.get(projectId);
    if (source) {
      source.close();
      this.connections.delete(projectId);
    }
    if (this.reconnectTimers.has(projectId)) {
      clearTimeout(this.reconnectTimers.get(projectId));
      this.reconnectTimers.delete(projectId);
    }
  }

  // Subscribe to events from a project
  onEvent(projectId, handler) {
    if (!this.handlers.has(projectId)) {
      this.handlers.set(projectId, new Set());
    }
    this.handlers.get(projectId).add(handler);

    return () => {
      this.handlers.get(projectId).delete(handler);
      if (this.handlers.get(projectId).size === 0) {
        this.handlers.delete(projectId);
      }
    };
  }

  // Get connection status
  getStatus(projectId) {
    const source = this.connections.get(projectId);
    if (!source) return 'disconnected';
    return source.readyState === 0 ? 'connecting' :
           source.readyState === 1 ? 'connected' :
           'error';
  }

  _routeEvent(projectId, eventData) {
    const handlers = this.handlers.get(projectId);
    if (handlers) {
      handlers.forEach(handler => handler(eventData));
    }
  }

  _handleError(projectId) {
    this.disconnect(projectId);
    // Exponential backoff reconnect
    const timer = setTimeout(() => {
      this.connect(projectId);
    }, Math.min(1000 * Math.pow(2, this.retries), 30000));
    this.reconnectTimers.set(projectId, timer);
  }
}

export const sseManager = new SSEManager();
```

### useSSE Hook

```javascript
export function useSSE(projectId) {
  const [status, setStatus] = useState('disconnected');

  useEffect(() => {
    if (!projectId) return;

    // Connect to SSE
    sseManager.connect(projectId);

    // Update status
    setStatus(sseManager.getStatus(projectId));

    // Listen for events
    const unsubscribe = sseManager.onEvent(projectId, () => {
      setStatus(sseManager.getStatus(projectId));
    });

    return () => {
      unsubscribe();
      // Don't disconnect yet - other components might need it
    };
  }, [projectId]);

  return { status };
}
```

### useSSEEvent Hook

```javascript
export function useSSEEvent(projectId, eventType, callback) {
  useEffect(() => {
    if (!projectId) return;

    sseManager.connect(projectId);

    const unsubscribe = sseManager.onEvent(projectId, (rawEvent) => {
      try {
        const data = JSON.parse(rawEvent.data);
        if (!eventType || data.type === eventType) {
          callback(data);
        }
      } catch (error) {
        console.error('Failed to parse SSE event', error);
      }
    });

    return unsubscribe;
  }, [projectId, eventType, callback]);
}
```

### useRefetchOnSSEEvent Hook

```javascript
export function useRefetchOnSSEEvent(projectId, eventType, refetchFn) {
  useSSEEvent(projectId, eventType, () => {
    refetchFn();
  });
}
```

### useProjectChange Integration

Automatically switch SSE when project changes:

```javascript
export function useProjectSSE(projectId) {
  useProjectChange((newProjectId) => {
    // Disconnect from old project
    if (projectId) {
      sseManager.disconnect(projectId);
    }
    // Connect to new project
    if (newProjectId) {
      sseManager.connect(newProjectId);
    }
  });
}
```

## Acceptance Criteria

- [ ] SSE manager handles multiple project connections
- [ ] useSSE hook connects to project-scoped SSE
- [ ] useSSEEvent hook filters events by type
- [ ] Auto-reconnect with exponential backoff
- [ ] Project change triggers disconnect/reconnect
- [ ] Event handlers properly cleaned up
- [ ] Connection status available to components
- [ ] SSEStatus component shows connection state
- [ ] No memory leaks from old connections
- [ ] All unit tests pass
- [ ] Integration tests verify project switching

## Testing Strategy

1. **Unit Tests**:
   - SSE manager creates/closes connections
   - Event routing to correct handlers
   - Auto-reconnect after disconnect
   - Status reporting

2. **Integration Tests**:
   - useSSE hook works with projects
   - Project change reconnects
   - useSSEEvent filters correctly
   - Memory cleanup

3. **Manual Testing**:
   - Open SSE and switch projects
   - Verify connection to new project
   - Check status indicator
   - Trigger events, verify receipt

## Definition of Done

- All acceptance criteria met
- Unit test coverage >80%
- No memory leaks
- Proper error handling
- Clear status visibility
