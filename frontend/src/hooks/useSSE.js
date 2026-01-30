import { useEffect, useRef, useCallback, useState } from 'react';
import useWorkflowStore from '../stores/workflowStore';
import useTaskStore from '../stores/taskStore';
import useUIStore from '../stores/uiStore';
import useAgentStore from '../stores/agentStore';
import useKanbanStore from '../stores/kanbanStore';
import { workflowApi } from '../lib/api';

const SSE_URL = '/api/v1/sse/events';
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
  const [connectionMode, setConnectionModeLocal] = useState(CONNECTION_MODE.DISCONNECTED);

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

  const handleEvent = useCallback((eventType, data) => {
    switch (eventType) {
      // Workflow events
      case 'workflow_started':
        handleWorkflowStarted(data);
        break;
      case 'workflow_state_updated':
        handleWorkflowStateUpdated(data);
        break;
      case 'workflow_completed':
        handleWorkflowCompleted(data);
        notifyInfo(`Workflow ${data.workflow_id} completed`);
        break;
      case 'workflow_failed':
        handleWorkflowFailed(data);
        notifyError(`Workflow ${data.workflow_id} failed: ${data.error}`);
        break;
      case 'workflow_paused':
        handleWorkflowPaused(data);
        notifyInfo(`Workflow ${data.workflow_id} paused`);
        break;
      case 'workflow_resumed':
        handleWorkflowResumed(data);
        notifyInfo(`Workflow ${data.workflow_id} resumed`);
        break;

      // Task events
      case 'task_created':
        handleTaskCreated(data);
        break;
      case 'task_started':
        handleTaskStarted(data);
        break;
      case 'task_progress':
        handleTaskProgress(data);
        break;
      case 'task_completed':
        handleTaskCompleted(data);
        break;
      case 'task_failed':
        handleTaskFailed(data);
        break;
      case 'task_skipped':
        handleTaskSkipped(data);
        break;
      case 'task_retry':
        handleTaskRetry(data);
        break;

      // Agent events
      case 'agent_event':
        handleAgentEvent(data);
        break;

      // Kanban events
      case 'kanban_workflow_moved':
        handleKanbanWorkflowMoved(data);
        break;
      case 'kanban_execution_started':
        handleKanbanExecutionStarted(data);
        break;
      case 'kanban_execution_completed':
        handleKanbanExecutionCompleted(data);
        break;
      case 'kanban_execution_failed':
        handleKanbanExecutionFailed(data);
        break;
      case 'kanban_engine_state_changed':
        handleKanbanEngineStateChanged(data);
        break;
      case 'kanban_circuit_breaker_opened':
        handleKanbanCircuitBreakerOpened(data);
        break;

      // Connection events
      case 'connected':
        setSSEConnected(true);
        setConnectionMode(CONNECTION_MODE.SSE);
        reconnectAttemptRef.current = 0;
        stopPolling(); // Stop polling when SSE reconnects
        break;

      default:
        console.log('Unhandled SSE event:', eventType, data);
    }
  }, [
    handleWorkflowStarted,
    handleWorkflowStateUpdated,
    handleWorkflowCompleted,
    handleWorkflowFailed,
    handleWorkflowPaused,
    handleWorkflowResumed,
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
    setSSEConnected,
    setConnectionMode,
    stopPolling,
    notifyInfo,
    notifyError,
  ]);

  const connect = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }

    const eventSource = new EventSource(SSE_URL);
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
        handleEvent('message', data);
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
      'task_created',
      'task_started',
      'task_progress',
      'task_completed',
      'task_failed',
      'task_skipped',
      'task_retry',
      'agent_event',
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
          handleEvent(eventType, data);
        } catch (error) {
          console.error(`Failed to parse ${eventType} event:`, error);
        }
      });
    });
  }, [handleEvent, setConnectionMode, setSSEConnected, startPolling, stopPolling]);

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
