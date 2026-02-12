import { useEffect, useRef, useCallback, useState } from 'react';
import useWorkflowStore from '../stores/workflowStore';
import useTaskStore from '../stores/taskStore';
import useUIStore from '../stores/uiStore';
import useAgentStore from '../stores/agentStore';
import useExecutionStore from '../stores/executionStore';
import useKanbanStore from '../stores/kanbanStore';
import useProjectStore from '../stores/projectStore';
import useIssuesStore from '../stores/issuesStore';
import { workflowApi } from '../lib/api';

const SSE_BASE_URL = '/api/v1/sse/events';
const RECONNECT_DELAY = 3000;
const MAX_RECONNECT_ATTEMPTS = 10;
const POLLING_INTERVAL = 5000;

// Connection modes
export const CONNECTION_MODE = {
  SSE: 'sse',
  POLLING: 'polling',
  DISCONNECTED: 'disconnected',
};

export default function useSSE() {
  const eventSourceRef = useRef(null);
  const reconnectAttemptRef = useRef(0);
  const reconnectTimeoutRef = useRef(null);
  const pollingIntervalRef = useRef(null);
  const connectRef = useRef(null);
  const handleEventRef = useRef(null);
  const [connectionMode, setConnectionModeLocal] = useState(CONNECTION_MODE.DISCONNECTED);

  // Project context for filtering
  const currentProjectId = useProjectStore(state => state.currentProjectId);
  const prevProjectIdRef = useRef(currentProjectId);

  const setSSEConnected = useUIStore(state => state.setSSEConnected);
  const setConnectionModeInStore = useUIStore(state => state.setConnectionMode);
  const setRetrySSEFn = useUIStore(state => state.setRetrySSEFn);
  const notifyError = useUIStore(state => state.notifyError);
  const notifyInfo = useUIStore(state => state.notifyInfo);

  // Update both local state and store
  const setConnectionMode = useCallback((mode) => {
    setConnectionModeLocal(mode);
    setConnectionModeInStore(mode);
  }, [setConnectionModeInStore]);

  // Workflow event handlers
  const handleWorkflowStarted = useWorkflowStore(state => state.handleWorkflowStarted);
  const handleWorkflowStateUpdated = useWorkflowStore(state => state.handleWorkflowStateUpdated);
  const handleWorkflowCompleted = useWorkflowStore(state => state.handleWorkflowCompleted);
  const handleWorkflowFailed = useWorkflowStore(state => state.handleWorkflowFailed);
  const handleWorkflowPaused = useWorkflowStore(state => state.handleWorkflowPaused);
  const handleWorkflowResumed = useWorkflowStore(state => state.handleWorkflowResumed);
  const handlePhaseStarted = useWorkflowStore(state => state.handlePhaseStarted);
  const handlePhaseCompleted = useWorkflowStore(state => state.handlePhaseCompleted);
  const handlePhaseAwaitingReview = useWorkflowStore(state => state.handlePhaseAwaitingReview);
  const handlePhaseReviewApproved = useWorkflowStore(state => state.handlePhaseReviewApproved);
  const handlePhaseReviewRejected = useWorkflowStore(state => state.handlePhaseReviewRejected);
  const setWorkflows = useWorkflowStore(state => state.setWorkflows);

  // Task event handlers
  const handleTaskCreated = useTaskStore(state => state.handleTaskCreated);
  const handleTaskStarted = useTaskStore(state => state.handleTaskStarted);
  const handleTaskProgress = useTaskStore(state => state.handleTaskProgress);
  const handleTaskCompleted = useTaskStore(state => state.handleTaskCompleted);
  const handleTaskFailed = useTaskStore(state => state.handleTaskFailed);
  const handleTaskSkipped = useTaskStore(state => state.handleTaskSkipped);
  const handleTaskRetry = useTaskStore(state => state.handleTaskRetry);

  // Agent event handler
  const handleAgentEvent = useAgentStore(state => state.handleAgentEvent);
  const ingestSSEEvent = useExecutionStore(state => state.ingestSSEEvent);

  // Issues store (for issue generation/publishing progress)
  const updateGenerationProgress = useIssuesStore(state => state.updateGenerationProgress);
  const updatePublishingProgress = useIssuesStore(state => state.updatePublishingProgress);

  // Kanban event handlers
  const handleKanbanWorkflowMoved = useKanbanStore(state => state.handleWorkflowMoved);
  const handleKanbanExecutionStarted = useKanbanStore(state => state.handleExecutionStarted);
  const handleKanbanExecutionCompleted = useKanbanStore(state => state.handleExecutionCompleted);
  const handleKanbanExecutionFailed = useKanbanStore(state => state.handleExecutionFailed);
  const handleKanbanEngineStateChanged = useKanbanStore(state => state.handleEngineStateChanged);
  const handleKanbanCircuitBreakerOpened = useKanbanStore(state => state.handleCircuitBreakerOpened);

  // Polling function
  const poll = useCallback(async () => {
    try {
      const workflows = await workflowApi.list();
      setWorkflows(workflows);
    } catch (error) {
      console.error('Polling failed:', error);
    }
  }, [setWorkflows]);

  // Start polling fallback
  const startPolling = useCallback(() => {
    if (pollingIntervalRef.current) return; // Already polling

    console.log('Starting polling fallback');
    setConnectionMode(CONNECTION_MODE.POLLING);
    notifyInfo('Using polling mode (SSE unavailable)');

    // Poll immediately, then at interval
    poll();
    pollingIntervalRef.current = setInterval(poll, POLLING_INTERVAL);
  }, [poll, notifyInfo, setConnectionMode]);

  // Stop polling
  const stopPolling = useCallback(() => {
    if (pollingIntervalRef.current) {
      console.log('Stopping polling');
      clearInterval(pollingIntervalRef.current);
      pollingIntervalRef.current = null;
    }
  }, []);

  // --- Issue-specific SSE handlers (extracted to reduce cognitive complexity) ---

  const handleIssuesGenerationProgress = useCallback((data) => {
    const st = useIssuesStore.getState();
    if (!st.generating || !st.workflowId || st.workflowId !== data?.workflow_id) return;

    const total = typeof data?.total === 'number' ? data.total : null;
    const current = typeof data?.current === 'number' ? data.current : st.generationProgress;

    // Clear stale agent activity on first event so the overlay starts fresh.
    if (data?.stage === 'started') {
      useAgentStore.getState().clearActivity(data.workflow_id);
    }

    // Inject into agent activity feed so the overlay's Agent Telemetry section
    // shows issue generation progress (not just agent_event SSE events).
    const stageMessages = {
      started: `Issue generation started (${total ?? '?'} issues)`,
      batch_started: data?.message || 'Processing batch...',
      batch_completed: data?.message || 'Batch completed',
      batch_failed: data?.message || 'Batch failed',
      file_generated: data?.title || data?.file_name || 'Issue generated',
      progress: data?.message || `Progress: ${current}/${total ?? '?'}`,
      completed: `Issue generation completed (${current} issues)`,
    };
    const msg = stageMessages[data?.stage] || data?.message || data?.stage;
    const kind = data?.stage === 'batch_failed' ? 'error'
      : data?.stage === 'completed' ? 'completed'
      : 'progress';
    handleAgentEvent({
      workflow_id: data.workflow_id,
      agent: 'issues',
      event_kind: kind,
      message: msg,
      timestamp: data?.timestamp || new Date().toISOString(),
    });

    if (data?.stage === 'file_generated') {
      const issue = {
        title: data?.title || data?.file_name || 'Generated issue',
        body: '',
        labels: [],
        assignees: [],
        is_main_issue: !!data?.is_main_issue,
        task_id: data?.task_id || null,
        file_path: data?.file_name || null,
      };
      updateGenerationProgress(current, issue, total);
    } else {
      updateGenerationProgress(current, null, total);
    }
  }, [updateGenerationProgress, handleAgentEvent]);

  const handleIssuesPublishingProgress = useCallback((data) => {
    const st = useIssuesStore.getState();
    if (!st.submitting || !st.workflowId || st.workflowId !== data?.workflow_id) return;

    const total = typeof data?.total === 'number' ? data.total : null;
    const current = typeof data?.current === 'number' ? data.current : 0;
    updatePublishingProgress(current, total, data?.message || null);
  }, [updatePublishingProgress]);

  const handleWorkflowFailedEvent = useCallback((data) => {
    handleWorkflowFailed(data);
    if (String(data?.error_code || '').toUpperCase() === 'CANCELLED') {
      notifyInfo(`Workflow ${data.workflow_id} cancelled`);
    } else {
      notifyError(`Workflow ${data.workflow_id} failed: ${data.error}`);
    }
  }, [handleWorkflowFailed, notifyInfo, notifyError]);

  const handleConnectedEvent = useCallback(() => {
    setSSEConnected(true);
    setConnectionMode(CONNECTION_MODE.SSE);
    reconnectAttemptRef.current = 0;
    stopPolling();
  }, [setSSEConnected, setConnectionMode, stopPolling]);

  // --- Event dispatch map ---

  const eventHandlers = useCallback(() => ({
    // Workflow events
    workflow_started:       (data) => handleWorkflowStarted(data),
    workflow_state_updated: (data) => handleWorkflowStateUpdated(data),
    workflow_completed:     (data) => { handleWorkflowCompleted(data); notifyInfo(`Workflow ${data.workflow_id} completed`); },
    workflow_failed:        (data) => handleWorkflowFailedEvent(data),
    workflow_paused:        (data) => { handleWorkflowPaused(data); notifyInfo(`Workflow ${data.workflow_id} paused`); },
    workflow_resumed:       (data) => { handleWorkflowResumed(data); notifyInfo(`Workflow ${data.workflow_id} resumed`); },

    // Phase events
    phase_started:          (data) => handlePhaseStarted(data),
    phase_completed:        (data) => handlePhaseCompleted(data),
    phase_awaiting_review:  (data) => handlePhaseAwaitingReview(data),
    phase_review_approved:  (data) => handlePhaseReviewApproved(data),
    phase_review_rejected:  (data) => handlePhaseReviewRejected(data),

    // Task events
    task_created:   (data) => handleTaskCreated(data),
    task_started:   (data) => handleTaskStarted(data),
    task_progress:  (data) => handleTaskProgress(data),
    task_completed: (data) => handleTaskCompleted(data),
    task_failed:    (data) => handleTaskFailed(data),
    task_skipped:   (data) => handleTaskSkipped(data),
    task_retry:     (data) => handleTaskRetry(data),

    // Agent events
    agent_event: (data) => handleAgentEvent(data),

    // Config / provenance & log events (persisted by ingestSSEEvent; no store updates)
    config_loaded: () => {},
    log:           () => {},

    // Kanban events
    kanban_workflow_moved:       (data) => handleKanbanWorkflowMoved(data),
    kanban_execution_started:    (data) => handleKanbanExecutionStarted(data),
    kanban_execution_completed:  (data) => handleKanbanExecutionCompleted(data),
    kanban_execution_failed:     (data) => handleKanbanExecutionFailed(data),
    kanban_engine_state_changed: (data) => handleKanbanEngineStateChanged(data),
    kanban_circuit_breaker_opened: (data) => { handleKanbanCircuitBreakerOpened(data); notifyError('Kanban engine circuit breaker opened - too many failures'); },

    // Issues progress events
    issues_generation_progress:  (data) => handleIssuesGenerationProgress(data),
    issues_publishing_progress:  (data) => handleIssuesPublishingProgress(data),

    // Connection events
    connected: () => handleConnectedEvent(),
  }), [
    handleWorkflowStarted,
    handleWorkflowStateUpdated,
    handleWorkflowCompleted,
    handleWorkflowFailedEvent,
    handleWorkflowPaused,
    handleWorkflowResumed,
    handlePhaseStarted,
    handlePhaseCompleted,
    handlePhaseAwaitingReview,
    handlePhaseReviewApproved,
    handlePhaseReviewRejected,
    handleTaskCreated,
    handleTaskStarted,
    handleTaskProgress,
    handleTaskCompleted,
    handleTaskFailed,
    handleTaskSkipped,
    handleTaskRetry,
    handleAgentEvent,
    handleKanbanWorkflowMoved,
    handleKanbanExecutionStarted,
    handleKanbanExecutionCompleted,
    handleKanbanExecutionFailed,
    handleKanbanEngineStateChanged,
    handleKanbanCircuitBreakerOpened,
    handleIssuesGenerationProgress,
    handleIssuesPublishingProgress,
    handleConnectedEvent,
    notifyInfo,
    notifyError,
  ]);

  const handleEvent = useCallback((eventType, data) => {
    // Persist a replayable execution timeline (per project + workflow) for the workflow detail view.
    // We intentionally ingest before dispatch so we don't miss anything due to handler errors.
    if (eventType && eventType !== 'connected' && eventType !== 'message') {
      try {
        ingestSSEEvent(eventType, data, currentProjectId);
      } catch (e) {
        // Never break live updates due to telemetry persistence failures.
        console.warn('Failed to ingest SSE event into execution store', e);
      }
    }

    const handlers = eventHandlers();
    const handler = handlers[eventType];
    if (handler) {
      handler(data);
    } else {
      console.log('Unhandled SSE event:', eventType, data);
    }
  }, [ingestSSEEvent, currentProjectId, eventHandlers]);

  // Keep ref in sync so connect() always calls the latest handleEvent
  // without needing it as a dependency (which would destabilize connect).
  useEffect(() => {
    handleEventRef.current = handleEvent;
  }, [handleEvent]);

  const connect = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }

    // Build SSE URL with optional project filter
    let sseUrl = SSE_BASE_URL;
    if (currentProjectId) {
      sseUrl = `${SSE_BASE_URL}?project=${encodeURIComponent(currentProjectId)}`;
    }

    const eventSource = new EventSource(sseUrl);
    eventSourceRef.current = eventSource;

    eventSource.onopen = () => {
      console.log('SSE connection established');
      setSSEConnected(true);
      setConnectionMode(CONNECTION_MODE.SSE);
      reconnectAttemptRef.current = 0;
      stopPolling(); // Stop polling when SSE connects
    };

    eventSource.onerror = (error) => {
      console.error('SSE connection error:', error);
      setSSEConnected(false);
      eventSource.close();

      // Attempt reconnection
      if (reconnectAttemptRef.current < MAX_RECONNECT_ATTEMPTS) {
        reconnectAttemptRef.current++;
        const delay = RECONNECT_DELAY * Math.pow(1.5, reconnectAttemptRef.current - 1);
        console.log(`Reconnecting in ${delay}ms (attempt ${reconnectAttemptRef.current})`);
        reconnectTimeoutRef.current = setTimeout(() => connectRef.current?.(), delay);
      } else {
        // Max attempts reached, fall back to polling
        console.log('Max reconnect attempts reached, falling back to polling');
        startPolling();
      }
    };

    // Generic message handler for standard SSE format
    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        handleEventRef.current('message', data);
      } catch (error) {
        console.error('Failed to parse SSE message:', error);
      }
    };

    // Specific event type handlers
    const eventTypes = [
      'connected',
      'workflow_started',
      'workflow_state_updated',
      'workflow_completed',
      'workflow_failed',
      'workflow_paused',
      'workflow_resumed',
      'phase_started',
      'phase_completed',
      'phase_awaiting_review',
      'phase_review_approved',
      'phase_review_rejected',
      'task_created',
      'task_started',
      'task_progress',
      'task_completed',
      'task_failed',
      'task_skipped',
      'task_retry',
      'agent_event',
      'issues_generation_progress',
      'issues_publishing_progress',
      'config_loaded',
      'log',
      'kanban_workflow_moved',
      'kanban_execution_started',
      'kanban_execution_completed',
      'kanban_execution_failed',
      'kanban_engine_state_changed',
      'kanban_circuit_breaker_opened',
    ];

    eventTypes.forEach(eventType => {
      eventSource.addEventListener(eventType, (event) => {
        try {
          const data = JSON.parse(event.data);
          handleEventRef.current(eventType, data);
        } catch (error) {
          console.error(`Failed to parse ${eventType} event:`, error);
        }
      });
    });
  }, [setConnectionMode, setSSEConnected, startPolling, stopPolling, currentProjectId]);

  // Keep ref updated for reconnection
  useEffect(() => {
    connectRef.current = connect;
  }, [connect]);

  const disconnect = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
    }
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
    stopPolling();
    setSSEConnected(false);
    setConnectionMode(CONNECTION_MODE.DISCONNECTED);
  }, [setConnectionMode, setSSEConnected, stopPolling]);

  // Retry SSE connection (useful for manual reconnect)
  const retrySSE = useCallback(() => {
    stopPolling();
    reconnectAttemptRef.current = 0;
    connect();
  }, [connect, stopPolling]);

  // Register retry function with store so UI can access it
  useEffect(() => {
    setRetrySSEFn(() => retrySSE);
  }, [retrySSE, setRetrySSEFn]);

  // Reconnect when project changes
  useEffect(() => {
    if (prevProjectIdRef.current !== currentProjectId) {
      console.log(`Project changed from ${prevProjectIdRef.current} to ${currentProjectId}, reconnecting SSE`);
      prevProjectIdRef.current = currentProjectId;
      reconnectAttemptRef.current = 0;
      connect();
    }
  }, [currentProjectId, connect]);

  useEffect(() => {
    connect();
    return () => disconnect();
  }, [connect, disconnect]);

  return {
    connect,
    disconnect,
    retrySSE,
    connectionMode,
    isConnected: useUIStore(state => state.sseConnected),
  };
}
