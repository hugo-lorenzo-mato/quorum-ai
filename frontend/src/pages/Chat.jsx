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
  Pencil,
  Check,
  Copy,
  CheckCircle2,
  ArrowLeft,
  PanelLeftClose,
  PanelLeft,
} from 'lucide-react';
import Logo from '../components/Logo';
import {
  AgentSelector,
  ModelSelector,
  ReasoningSelector,
  AttachmentPicker,
} from '../components/chat';
import ChatMarkdown from '../components/ChatMarkdown';

function TypingIndicator() {
  return (
    <div className="flex items-center gap-2 px-0.5">
      <div className="w-6 h-6 rounded-full bg-primary/10 flex items-center justify-center mt-1">
        <Bot className="w-3 h-3 text-primary" />
      </div>
      <div className="flex gap-1">
        <span className="w-1.5 h-1.5 bg-muted-foreground rounded-full animate-bounce" style={{ animationDelay: '0ms' }} />
        <span className="w-1.5 h-1.5 bg-muted-foreground rounded-full animate-bounce" style={{ animationDelay: '150ms' }} />
        <span className="w-1.5 h-1.5 bg-muted-foreground rounded-full animate-bounce" style={{ animationDelay: '300ms' }} />
      </div>
    </div>
  );
}

function MessageBubble({ message, isLast }) {
  const [copied, setCopied] = useState(false);
  const isUser = message.role === 'user';
  const agentName = message.agent ? message.agent.charAt(0).toUpperCase() + message.agent.slice(1) : 'Assistant';

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(message.content);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Silent fail - copy button is a convenience feature
    }
  };

  return (
    <div className={`flex gap-2 ${isUser ? 'flex-row-reverse' : ''} ${isLast ? 'animate-fade-up' : ''}`}>
      <div className={`w-6 h-6 rounded-full flex items-center justify-center flex-shrink-0 mt-1 ${
        isUser ? 'bg-primary' : 'bg-muted'
      }`}>
        {isUser ? (
          <User className="w-3 h-3 text-primary-foreground" />
        ) : (
          <Logo className="w-3 h-3 text-muted-foreground" />
        )}
      </div>
      <div className={`max-w-[80%] rounded-lg px-3 py-2 min-w-[120px] ${
        isUser
          ? 'bg-primary text-primary-foreground rounded-br-sm ml-auto'
          : 'bg-card border border-border rounded-bl-sm mr-auto'
      }`}>
        {!isUser && message.agent && (
          <p className="text-[10px] font-mono font-medium text-primary mb-1">{agentName}</p>
        )}
        <div className={`text-sm leading-relaxed ${isUser ? 'text-primary-foreground' : 'text-foreground'}`}>
          <ChatMarkdown content={message.content} isUser={isUser} />
        </div>
        <div className="flex items-center justify-between mt-1.5 pt-1 border-t border-white/10 opacity-70">
          <p className={`text-[9px] font-mono ${isUser ? 'text-primary-foreground/80' : 'text-muted-foreground'}`}>
            {new Date(message.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
          </p>
          <button
            type="button"
            onClick={handleCopy}
            className={`p-0.5 rounded hover:bg-white/10 transition-colors ${copied ? 'text-green-300' : ''}`}
            title={copied ? 'Copied!' : 'Copy message'}
          >
            {copied ? (
              <CheckCircle2 className="w-3 h-3" />
            ) : (
              <Copy className="w-3 h-3" />
            )}
          </button>
        </div>
      </div>
    </div>
  );
}

function SessionItem({ session, isActive, onClick, onDelete, onRename }) {
  const [isEditing, setIsEditing] = useState(false);
  const [editValue, setEditValue] = useState('');
  const inputRef = useRef(null);

  const displayTitle = session.title || `${session.agent || 'Claude'} Session`;

  const handleStartEdit = (e) => {
    e.stopPropagation();
    setEditValue(session.title || '');
    setIsEditing(true);
    setTimeout(() => inputRef.current?.focus(), 0);
  };

  const handleSave = () => {
    const newTitle = editValue.trim();
    if (newTitle !== (session.title || '')) {
      onRename(session.id, newTitle);
    }
    setIsEditing(false);
  };

  const handleKeyDown = (e) => {
    if (e.key === 'Enter') {
      handleSave();
    } else if (e.key === 'Escape') {
      setIsEditing(false);
    }
  };

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
          <Sparkles className={`w-4 h-4 flex-shrink-0 ${isActive ? 'text-primary' : ''}`} />
          <div className="flex-1 min-w-0">
            {isEditing ? (
              <div className="flex items-center gap-1" onClick={e => e.stopPropagation()}>
                <input
                  ref={inputRef}
                  type="text"
                  value={editValue}
                  onChange={(e) => setEditValue(e.target.value)}
                  onKeyDown={handleKeyDown}
                  onBlur={handleSave}
                  placeholder="Session title"
                  className="flex-1 min-w-0 text-sm font-medium bg-background border border-input rounded px-1.5 py-0.5 focus:outline-none focus:ring-1 focus:ring-ring"
                />
                <button
                  onClick={handleSave}
                  className="p-1 text-primary hover:bg-primary/10 rounded"
                >
                  <Check className="w-3.5 h-3.5" />
                </button>
              </div>
            ) : (
              <p className="text-sm font-medium truncate">
                {displayTitle}
              </p>
            )}
            <p className="text-xs text-muted-foreground">
              {new Date(session.created_at).toLocaleDateString()}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-1">
          {!isEditing && (
            <button
              onClick={handleStartEdit}
              className="p-1.5 opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-foreground hover:bg-accent rounded-md transition-all"
              title="Rename session"
            >
              <Pencil className="w-3.5 h-3.5" />
            </button>
          )}
          <button
            onClick={(e) => { e.stopPropagation(); onDelete(session.id); }}
            className="p-1.5 opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-destructive hover:bg-destructive/10 rounded-md transition-all"
          >
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
      </div>
    </button>
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
    sessions, activeSessionId, loading, sending, error, sidebarCollapsed,
    fetchSessions, createSession, selectSession, deleteSession, updateSession,
    sendMessage, getActiveMessages, clearError, toggleSidebar,
    // Per-message options
    currentAgent, currentModel, currentReasoningEffort, attachments,
    setCurrentAgent, setCurrentModel, setCurrentReasoningEffort,
    addAttachment, removeAttachment, clearAttachments, uploadAttachments,
  } = useChatStore();

  const [input, setInput] = useState('');
  const messagesEndRef = useRef(null);
  const inputRef = useRef(null);

  const activeMessages = getActiveMessages();
  const activeSession = sessions.find((s) => s.id === activeSessionId);

  useEffect(() => { fetchSessions(); }, [fetchSessions]);
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [activeMessages, activeSessionId]);

  const handleSend = async (e) => {
    e.preventDefault();
    if (!input.trim() || sending) return;
    const msg = input.trim();
    setInput('');
    if (inputRef.current) {
      inputRef.current.style.height = 'auto';
    }
    await sendMessage(msg);
    inputRef.current?.focus();
  };

  const handleCreateSession = async () => {
    await createSession();
  };

  const handleRenameSession = async (sessionId, newTitle) => {
    await updateSession(sessionId, { title: newTitle });
  };

  const handleBackToList = () => {
    selectSession(null);
  };

  return (
    <div className="h-[calc(100dvh-3.5rem)] md:h-[calc(100vh-3.5rem)] flex overflow-hidden animate-fade-in pb-[calc(4rem+env(safe-area-inset-bottom))] md:pb-0">{/* 3.5rem = h-14 del header, pb for mobile nav + safe area */}
      {/* Sessions sidebar */}
      <div className={`transition-all duration-300 ease-in-out flex-shrink-0 flex flex-col gap-4 p-4 bg-card border-r border-border ${
        activeSession ? 'hidden md:flex' : 'flex'
      } ${
        sidebarCollapsed ? 'w-full md:w-16 overflow-hidden' : 'w-full md:w-80'
      }`}>
        <div className={`flex items-center ${sidebarCollapsed ? 'justify-center' : 'justify-between'}`}>
          {!sidebarCollapsed && (
            <h2 className="text-lg font-semibold text-foreground animate-fade-in">Chats</h2>
          )}
          <div className="flex items-center gap-2">
            {!sidebarCollapsed && (
              <button
                onClick={handleCreateSession}
                disabled={loading}
                className="flex items-center justify-center gap-2 px-3 py-1.5 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors animate-fade-in"
              >
                {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Plus className="w-4 h-4" />}
                New
              </button>
            )}
            <button
              onClick={toggleSidebar}
              className="hidden md:flex p-1.5 rounded-lg text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
              title={sidebarCollapsed ? "Expand chat list" : "Collapse chat list"}
            >
              {sidebarCollapsed ? <PanelLeft className="w-4 h-4" /> : <PanelLeftClose className="w-4 h-4" />}
            </button>
          </div>
        </div>

        {!sidebarCollapsed && (
          <div className="flex-1 overflow-y-auto space-y-1 min-h-0 pr-2 animate-fade-in">
            {sessions.length > 0 ? (
              sessions.map((session) => (
                <SessionItem
                  key={session.id}
                  session={session}
                  isActive={activeSessionId === session.id}
                  onClick={() => selectSession(session.id)}
                  onDelete={deleteSession}
                  onRename={handleRenameSession}
                />
              ))
            ) : (
              <div className="text-center py-12">
                <MessageSquare className="w-12 h-12 mx-auto mb-3 text-muted-foreground opacity-50" />
                <p className="text-sm text-muted-foreground">No chats yet</p>
              </div>
            )}
          </div>
        )}

        {sidebarCollapsed && (
          <div className="flex-1 flex flex-col items-center gap-3 overflow-hidden">
            <button
              onClick={handleCreateSession}
              disabled={loading}
              className="p-2 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-50 transition-colors"
              title="New chat"
            >
              {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Plus className="w-4 h-4" />}
            </button>
            <div className="flex flex-col gap-1 overflow-y-auto">
              {sessions.slice(0, 8).map((session) => (
                <button
                  key={session.id}
                  onClick={() => selectSession(session.id)}
                  className={`p-2 rounded-lg transition-all ${
                    activeSessionId === session.id
                      ? 'bg-accent text-accent-foreground'
                      : 'text-muted-foreground hover:text-foreground hover:bg-accent/50'
                  }`}
                  title={session.title || `${session.agent || 'Claude'} Session`}
                >
                  <Sparkles className={`w-4 h-4 ${activeSessionId === session.id ? 'text-primary' : ''}`} />
                </button>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Chat area */}
      <div className={`flex-1 flex flex-col bg-background ${!activeSession ? 'hidden md:flex' : 'flex'}`}>
        {activeSession ? (
          <>
            {/* Header */}
            <div className="px-3 py-2.5 md:px-4 md:py-3 border-b border-border bg-card backdrop-blur-sm z-10 flex items-center justify-between gap-3 shrink-0">
              <div className="flex items-center gap-2.5 overflow-hidden flex-1">
                <button 
                  onClick={handleBackToList}
                  className="md:hidden p-1.5 -ml-1 rounded-lg hover:bg-accent text-muted-foreground"
                >
                  <ArrowLeft className="w-4 h-4" />
                </button>
                <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-primary/20 to-primary/10 flex items-center justify-center flex-shrink-0">
                  <Sparkles className="w-4 h-4 text-primary" />
                </div>
                <div className="min-w-0 flex-1">
                  <h3 className="font-semibold text-foreground truncate text-sm">
                    {activeSession.title || `${activeSession.agent || 'Claude'} Chat`}
                  </h3>
                  <p className="text-[11px] text-muted-foreground truncate">
                    {activeSession.agent || 'Claude'} · {new Date(activeSession.created_at).toLocaleDateString()}
                  </p>
                </div>
              </div>
              <button
                onClick={() => deleteSession(activeSession.id)}
                className="p-1.5 rounded-lg text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors flex-shrink-0"
              >
                <Trash2 className="w-4 h-4" />
              </button>
            </div>

            {/* Messages */}
            <div className="flex-1 min-h-0 overflow-y-auto flex flex-col">
              {activeMessages.length > 0 ? (
                <div className="w-full px-3 md:px-6 py-3 space-y-3">
                  {activeMessages.map((message, index) => (
                    <MessageBubble
                      key={message.id || index}
                      message={message}
                      isLast={index === activeMessages.length - 1}
                    />
                  ))}
                  {sending && <TypingIndicator />}
                  <div ref={messagesEndRef} className="h-1" />
                </div>
              ) : (
                <div className="flex items-center justify-center flex-1">
                  <div className="text-center">
                    <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-primary/10 flex items-center justify-center">
                      <Sparkles className="w-8 h-8 text-primary" />
                    </div>
                    <p className="text-foreground font-medium">Start a conversation</p>
                    <p className="text-sm text-muted-foreground mt-1">Send a message to begin</p>
                  </div>
                </div>
              )}
            </div>

            {/* Input */}
            <div className="shrink-0 border-t border-border bg-card backdrop-blur-sm">
              <div className="w-full px-3 md:px-6 py-3">
              {error && (
                <div className="mb-3 p-2.5 bg-destructive/10 text-destructive text-sm rounded-lg flex items-center justify-between">
                  <span>{error}</span>
                  <button onClick={clearError} className="p-1 hover:bg-destructive/20 rounded">
                    <X className="w-4 h-4" />
                  </button>
                </div>
              )}

              {/* Message options bar */}
              <div className="flex items-center gap-2 mb-3 flex-wrap">
                <AgentSelector
                  value={currentAgent}
                  onChange={setCurrentAgent}
                  disabled={sending}
                />
                <ModelSelector
                  value={currentModel}
                  onChange={setCurrentModel}
                  agent={currentAgent}
                  disabled={sending}
                />
                <ReasoningSelector
                  value={currentReasoningEffort}
                  onChange={setCurrentReasoningEffort}
                  agent={currentAgent}
                  disabled={sending}
                />
                <div className="flex-1" />
                <AttachmentPicker
                  attachments={attachments}
                  onAdd={addAttachment}
                  onRemove={removeAttachment}
                  onClear={clearAttachments}
                  onUpload={uploadAttachments}
                />
              </div>

              <form onSubmit={handleSend} className="flex gap-2.5 items-end">
                <textarea
                  ref={inputRef}
                  value={input}
                  onChange={(e) => {
                    setInput(e.target.value);
                    e.target.style.height = 'auto';
                    e.target.style.height = Math.min(e.target.scrollHeight, 140) + 'px';
                  }}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && !e.shiftKey) {
                      e.preventDefault();
                      handleSend(e);
                    }
                  }}
                  placeholder={`Message ${currentAgent}...`}
                  disabled={sending}
                  rows={1}
                  className="flex-1 min-h-[42px] max-h-[140px] py-2.5 px-3.5 border border-input rounded-lg bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background disabled:opacity-50 transition-all text-sm resize-none overflow-y-auto"
                />
                <button
                  type="submit"
                  disabled={sending || !input.trim()}
                  className="h-[42px] w-[42px] md:h-10 md:w-auto md:px-4 flex items-center justify-center rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-50 transition-colors shrink-0"
                >
                  {sending ? <Loader2 className="w-4 h-4 animate-spin" /> : <Send className="w-4 h-4" />}
                </button>
              </form>
              </div>
            </div>
          </>
        ) : (
          <EmptyChat onCreateSession={handleCreateSession} />
        )}
      </div>
    </div>
  );
}