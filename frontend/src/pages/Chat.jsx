import { useEffect, useRef, useState } from 'react';
import { useChatStore } from '../stores';
import {
  Send,
  Plus,
  Trash2,
  X,
  MessageSquare,
  Sparkles,
  Bot,
  User,
  Loader2,
} from 'lucide-react';

function TypingIndicator() {
  return (
    <div className="flex items-center gap-3 p-4">
      <div className="w-8 h-8 rounded-full bg-primary/10 flex items-center justify-center">
        <Bot className="w-4 h-4 text-primary" />
      </div>
      <div className="flex gap-1">
        <span className="w-2 h-2 bg-muted-foreground rounded-full animate-bounce" style={{ animationDelay: '0ms' }} />
        <span className="w-2 h-2 bg-muted-foreground rounded-full animate-bounce" style={{ animationDelay: '150ms' }} />
        <span className="w-2 h-2 bg-muted-foreground rounded-full animate-bounce" style={{ animationDelay: '300ms' }} />
      </div>
    </div>
  );
}

function MessageBubble({ message, isLast }) {
  const isUser = message.role === 'user';

  return (
    <div className={`flex gap-3 ${isUser ? 'flex-row-reverse' : ''} ${isLast ? 'animate-fade-up' : ''}`}>
      <div className={`w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0 ${
        isUser ? 'bg-primary' : 'bg-muted'
      }`}>
        {isUser ? (
          <User className="w-4 h-4 text-primary-foreground" />
        ) : (
          <Bot className="w-4 h-4 text-muted-foreground" />
        )}
      </div>
      <div className={`max-w-[70%] rounded-2xl px-4 py-3 ${
        isUser
          ? 'bg-primary text-primary-foreground rounded-br-md'
          : 'bg-card border border-border rounded-bl-md'
      }`}>
        <p className="text-sm whitespace-pre-wrap leading-relaxed">{message.content}</p>
        <p className={`text-xs mt-2 ${isUser ? 'text-primary-foreground/60' : 'text-muted-foreground'}`}>
          {new Date(message.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
        </p>
      </div>
    </div>
  );
}

function SessionItem({ session, isActive, onClick, onDelete }) {
  return (
    <button
      onClick={onClick}
      className={`w-full text-left p-3 rounded-lg transition-all group ${
        isActive
          ? 'bg-accent text-accent-foreground'
          : 'hover:bg-accent/50 text-muted-foreground hover:text-foreground'
      }`}
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3 flex-1 min-w-0">
          <Sparkles className={`w-4 h-4 ${isActive ? 'text-primary' : ''}`} />
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium truncate">
              {session.agent || 'Claude'} Session
            </p>
            <p className="text-xs text-muted-foreground">
              {new Date(session.created_at).toLocaleDateString()}
            </p>
          </div>
        </div>
        <button
          onClick={(e) => { e.stopPropagation(); onDelete(session.id); }}
          className="p-1.5 opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-destructive hover:bg-destructive/10 rounded-md transition-all"
        >
          <Trash2 className="w-4 h-4" />
        </button>
      </div>
    </button>
  );
}

function NewSessionForm({ onSubmit, onCancel, loading }) {
  const [agent, setAgent] = useState('claude');

  const agents = [
    { value: 'claude', label: 'Claude', description: 'Anthropic AI Assistant' },
    { value: 'gemini', label: 'Gemini', description: 'Google AI' },
    { value: 'codex', label: 'Codex', description: 'Code Assistant' },
  ];

  return (
    <div className="p-4 border border-border rounded-xl bg-card animate-fade-up">
      <h3 className="font-medium text-foreground mb-4">New Chat Session</h3>
      <div className="space-y-2">
        {agents.map((a) => (
          <button
            key={a.value}
            type="button"
            onClick={() => setAgent(a.value)}
            className={`w-full p-3 rounded-lg border-2 transition-all text-left ${
              agent === a.value
                ? 'border-primary bg-primary/5'
                : 'border-border hover:border-muted-foreground'
            }`}
          >
            <p className="font-medium text-foreground">{a.label}</p>
            <p className="text-xs text-muted-foreground">{a.description}</p>
          </button>
        ))}
      </div>
      <div className="flex gap-2 mt-4">
        <button
          onClick={() => onSubmit(agent)}
          disabled={loading}
          className="flex-1 px-4 py-2 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors"
        >
          {loading ? <Loader2 className="w-4 h-4 animate-spin mx-auto" /> : 'Create Session'}
        </button>
        <button
          onClick={onCancel}
          className="px-4 py-2 rounded-lg bg-secondary text-secondary-foreground text-sm font-medium hover:bg-secondary/80 transition-colors"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}

function EmptyChat({ onCreateSession }) {
  return (
    <div className="flex-1 flex items-center justify-center">
      <div className="text-center">
        <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-primary/10 flex items-center justify-center">
          <MessageSquare className="w-8 h-8 text-primary" />
        </div>
        <h3 className="text-lg font-semibold text-foreground mb-2">No session selected</h3>
        <p className="text-sm text-muted-foreground mb-4 max-w-sm">
          Create a new chat session or select an existing one to start chatting.
        </p>
        <button
          onClick={onCreateSession}
          className="px-4 py-2 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
        >
          Create Session
        </button>
      </div>
    </div>
  );
}

export default function Chat() {
  const {
    sessions, activeSessionId, loading, sending, error,
    fetchSessions, createSession, selectSession, deleteSession,
    sendMessage, getActiveMessages, clearError,
  } = useChatStore();

  const [input, setInput] = useState('');
  const [showNewSession, setShowNewSession] = useState(false);
  const messagesEndRef = useRef(null);
  const inputRef = useRef(null);

  useEffect(() => { fetchSessions(); }, [fetchSessions]);
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [getActiveMessages(), activeSessionId]);

  const handleSend = async (e) => {
    e.preventDefault();
    if (!input.trim() || sending) return;
    const msg = input.trim();
    setInput('');
    await sendMessage(msg);
    inputRef.current?.focus();
  };

  const handleCreateSession = async (agent) => {
    const session = await createSession(agent);
    if (session) setShowNewSession(false);
  };

  const activeMessages = getActiveMessages();
  const activeSession = sessions.find((s) => s.id === activeSessionId);

  return (
    <div className="h-[calc(100vh-8rem)] flex gap-4 animate-fade-in">
      {/* Sessions sidebar */}
      <div className="w-72 flex-shrink-0 flex flex-col gap-4">
        <button
          onClick={() => setShowNewSession(true)}
          className="flex items-center justify-center gap-2 w-full px-4 py-2.5 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
        >
          <Plus className="w-4 h-4" />
          New Session
        </button>

        {showNewSession && (
          <NewSessionForm
            onSubmit={handleCreateSession}
            onCancel={() => setShowNewSession(false)}
            loading={loading}
          />
        )}

        <div className="flex-1 overflow-y-auto space-y-1">
          {sessions.length > 0 ? (
            sessions.map((session) => (
              <SessionItem
                key={session.id}
                session={session}
                isActive={activeSessionId === session.id}
                onClick={() => selectSession(session.id)}
                onDelete={deleteSession}
              />
            ))
          ) : (
            <p className="text-center py-8 text-sm text-muted-foreground">No sessions yet</p>
          )}
        </div>
      </div>

      {/* Chat area */}
      <div className="flex-1 flex flex-col rounded-xl border border-border bg-card overflow-hidden">
        {activeSession ? (
          <>
            {/* Header */}
            <div className="px-6 py-4 border-b border-border flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-primary/20 to-info/20 flex items-center justify-center">
                  <Sparkles className="w-5 h-5 text-primary" />
                </div>
                <div>
                  <h3 className="font-semibold text-foreground">{activeSession.agent || 'Claude'}</h3>
                  <p className="text-xs text-muted-foreground">Session {activeSession.id.substring(0, 8)}...</p>
                </div>
              </div>
              <button
                onClick={() => deleteSession(activeSession.id)}
                className="p-2 rounded-lg text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors"
              >
                <Trash2 className="w-4 h-4" />
              </button>
            </div>

            {/* Messages */}
            <div className="flex-1 overflow-y-auto p-6 space-y-4 bg-background">
              {activeMessages.length > 0 ? (
                <>
                  {activeMessages.map((message, index) => (
                    <MessageBubble
                      key={message.id || index}
                      message={message}
                      isLast={index === activeMessages.length - 1}
                    />
                  ))}
                  {sending && <TypingIndicator />}
                </>
              ) : (
                <div className="flex items-center justify-center h-full">
                  <div className="text-center">
                    <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-primary/10 flex items-center justify-center">
                      <Sparkles className="w-8 h-8 text-primary" />
                    </div>
                    <p className="text-foreground font-medium">Start a conversation</p>
                    <p className="text-sm text-muted-foreground mt-1">Send a message to begin</p>
                  </div>
                </div>
              )}
              <div ref={messagesEndRef} />
            </div>

            {/* Input */}
            <div className="p-4 border-t border-border bg-card">
              {error && (
                <div className="mb-3 p-3 bg-destructive/10 text-destructive text-sm rounded-lg flex items-center justify-between">
                  <span>{error}</span>
                  <button onClick={clearError} className="p-1 hover:bg-destructive/20 rounded">
                    <X className="w-4 h-4" />
                  </button>
                </div>
              )}
              <form onSubmit={handleSend} className="flex gap-3">
                <input
                  ref={inputRef}
                  type="text"
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && !e.shiftKey && handleSend(e)}
                  placeholder="Type your message..."
                  disabled={sending}
                  className="flex-1 h-10 px-4 border border-input rounded-lg bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background disabled:opacity-50 transition-all"
                />
                <button
                  type="submit"
                  disabled={sending || !input.trim()}
                  className="h-10 px-4 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-50 transition-colors"
                >
                  {sending ? <Loader2 className="w-5 h-5 animate-spin" /> : <Send className="w-5 h-5" />}
                </button>
              </form>
            </div>
          </>
        ) : (
          <EmptyChat onCreateSession={() => setShowNewSession(true)} />
        )}
      </div>
    </div>
  );
}
