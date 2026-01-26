import { useEffect, useRef, useCallback } from 'react';
import useWorkflowStore from '../stores/workflowStore';
import useTaskStore from '../stores/taskStore';
import useUIStore from '../stores/uiStore';
import useAgentStore from '../stores/agentStore';

const SSE_URL = '/api/v1/sse/events';
const RECONNECT_DELAY = 3000;
const MAX_RECONNECT_ATTEMPTS = 10;

export default function useSSE() {
  const eventSourceRef = useRef(null);
  const reconnectAttemptRef = useRef(0);
  const reconnectTimeoutRef = useRef(null);
  const connectRef = useRef(null);

  const setSSEConnected = useUIStore(state => state.setSSEConnected);
  const notifyError = useUIStore(state => state.notifyError);
  const notifyInfo = useUIStore(state => state.notifyInfo);

  // Workflow event handlers
  const handleWorkflowStarted = useWorkflowStore(state => state.handleWorkflowStarted);
  const handleWorkflowStateUpdated = useWorkflowStore(state => state.handleWorkflowStateUpdated);
  const handleWorkflowCompleted = useWorkflowStore(state => state.handleWorkflowCompleted);
  const handleWorkflowFailed = useWorkflowStore(state => state.handleWorkflowFailed);

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
        // Handle pause if needed
        break;
      case 'workflow_resumed':
        // Handle resume if needed
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

      // Connection events
      case 'connected':
        setSSEConnected(true);
        reconnectAttemptRef.current = 0;
        break;

      default:
        console.log('Unhandled SSE event:', eventType, data);
    }
  }, [
    handleWorkflowStarted,
    handleWorkflowStateUpdated,
    handleWorkflowCompleted,
    handleWorkflowFailed,
    handleTaskCreated,
    handleTaskStarted,
    handleTaskProgress,
    handleTaskCompleted,
    handleTaskFailed,
    handleTaskSkipped,
    handleTaskRetry,
    handleAgentEvent,
    setSSEConnected,
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
      reconnectAttemptRef.current = 0;
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
        notifyError('Lost connection to server. Please refresh the page.');
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
  }, [handleEvent, setSSEConnected, notifyError]);

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
    setSSEConnected(false);
  }, [setSSEConnected]);

  useEffect(() => {
    connect();
    return () => disconnect();
  }, [connect, disconnect]);

  return {
    connect,
    disconnect,
    isConnected: useUIStore(state => state.sseConnected),
  };
}
