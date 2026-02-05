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
  Search,
  MessageCircle,
  MoreVertical,
  ChevronRight,
  Info
} from 'lucide-react';
import Logo from '../components/Logo';
import {
  AgentSelector,
  ModelSelector,
  ReasoningSelector,
  AttachmentPicker,
} from '../components/chat';
import ChatMarkdown from '../components/ChatMarkdown';
import { Button } from '../components/ui/Button';
import { Badge } from '../components/ui/Badge';
import { Input } from '../components/ui/Input';

function TypingIndicator() {
  return (
    <div className="flex items-center gap-3 p-4 animate-fade-in">
      <div className="w-8 h-8 rounded-xl bg-primary/10 flex items-center justify-center border border-primary/20 shadow-sm">
        <Bot className="w-4 h-4 text-primary" />
      </div>
      <div className="flex gap-1.5 p-3 rounded-2xl bg-muted/30 border border-border/50">
        <span className="w-1.5 h-1.5 bg-primary/40 rounded-full animate-bounce" style={{ animationDelay: '0ms' }} />
        <span className="w-1.5 h-1.5 bg-primary/40 rounded-full animate-bounce" style={{ animationDelay: '150ms' }} />
        <span className="w-1.5 h-1.5 bg-primary/40 rounded-full animate-bounce" style={{ animationDelay: '300ms' }} />
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
      // Silent fail
    }
  };

  return (
    <div className={`flex gap-4 ${isUser ? 'flex-row-reverse' : ''} ${isLast ? 'animate-fade-up' : 'animate-fade-in'}`}>
      <div className={`w-9 h-9 rounded-xl flex items-center justify-center flex-shrink-0 shadow-sm border transition-transform hover:scale-105 ${
        isUser ? 'bg-primary border-primary/20' : 'bg-background border-border'
      }`}>
        {isUser ? (
          <User className="w-5 h-5 text-primary-foreground" />
        ) : (
          <Logo className="w-5 h-5 text-primary" />
        )}
      </div>
      <div className={`group max-w-[85%] flex flex-col ${isUser ? 'items-end' : 'items-start'}`}>
        <div className={`rounded-2xl px-5 py-4 shadow-sm border transition-all ${
          isUser
            ? 'bg-primary text-primary-foreground border-primary/20 rounded-tr-none'
            : 'bg-card border-border rounded-tl-none backdrop-blur-sm'
        }`}>
          {!isUser && message.agent && (
            <div className="flex items-center gap-2 mb-2">
               <Badge variant="secondary" className="text-[9px] px-1.5 py-0 bg-primary/10 text-primary border-transparent font-black uppercase tracking-widest">
                  {agentName}
               </Badge>
            </div>
          )}
          <div className={`text-sm leading-relaxed ${isUser ? 'text-primary-foreground' : 'text-foreground'}`}>
            <ChatMarkdown content={message.content} isUser={isUser} />
          </div>
        </div>
        
        <div className={`flex items-center gap-3 mt-1.5 px-1 opacity-0 group-hover:opacity-100 transition-opacity ${isUser ? 'flex-row-reverse' : ''}`}>
          <p className="text-[10px] font-bold text-muted-foreground/50 uppercase tracking-tighter">
            {new Date(message.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
          </p>
          <button
            type="button"
            onClick={handleCopy}
            className={`p-1 rounded-md transition-colors ${copied ? 'text-green-500' : 'text-muted-foreground/40 hover:text-primary hover:bg-primary/5'}`}
            title="Copy message"
          >
            {copied ? <CheckCircle2 className="w-3.5 h-3.5" /> : <Copy className="w-3.5 h-3.5" />}
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

  const displayTitle = session.title || `${session.agent || 'Assistant'} Chat`;

  const handleStartEdit = (e) => {
    e.stopPropagation();
    setEditValue(session.title || '');
    setIsEditing(true);
    setTimeout(() => inputRef.current?.focus(), 0);
  };

  const handleSave = () => {
    const newTitle = editValue.trim();
    if (newTitle && newTitle !== (session.title || '')) {
      onRename(session.id, newTitle);
    }
    setIsEditing(false);
  };

  const handleKeyDown = (e) => {
    if (e.key === 'Enter') handleSave();
    else if (e.key === 'Escape') setIsEditing(false);
  };

  return (
    <button
      onClick={onClick}
      className={`w-full text-left p-3 rounded-xl transition-all group border border-transparent ${
        isActive
          ? 'bg-primary/10 border-primary/20 text-primary shadow-sm'
          : 'hover:bg-accent/50 text-muted-foreground hover:text-foreground'
      }`}
    >
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-3 flex-1 min-w-0">
          <div className={`p-2 rounded-lg transition-colors ${isActive ? 'bg-primary/20' : 'bg-muted/50 group-hover:bg-muted'}`}>
             <MessageCircle className={`w-4 h-4 ${isActive ? 'text-primary' : 'text-muted-foreground'}`} />
          </div>
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
                  className="flex-1 min-w-0 text-sm font-bold bg-background border border-primary/30 rounded-lg px-2 py-1 outline-none"
                />
              </div>
            ) : (
              <p className="text-sm font-bold truncate tracking-tight">
                {displayTitle}
              </p>
            )}
            <p className="text-[10px] font-bold uppercase tracking-widest opacity-50 mt-0.5">
              {new Date(session.created_at).toLocaleDateString()}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
          {!isEditing && (
            <button
              onClick={handleStartEdit}
              className="p-1.5 text-muted-foreground hover:text-primary hover:bg-primary/10 rounded-lg"
            >
              <Pencil className="w-3.5 h-3.5" />
            </button>
          )}
          <button
            onClick={(e) => { e.stopPropagation(); onDelete(session.id); }}
            className="p-1.5 text-muted-foreground hover:text-destructive hover:bg-destructive/10 rounded-lg"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>
    </button>
  );
}

function EmptyChat({ onCreateSession }) {
  return (
    <div className="flex-1 flex flex-col items-center justify-center p-8 text-center animate-fade-in">
      <div className="relative mb-8">
        <div className="absolute inset-0 bg-primary/20 blur-2xl rounded-full" />
        <div className="relative p-8 rounded-3xl bg-card border border-border shadow-xl">
          <MessageSquare className="w-12 h-12 text-primary" />
        </div>
      </div>
      <h3 className="text-2xl font-black text-foreground tracking-tight mb-2">Direct Intelligence</h3>
      <p className="text-sm text-muted-foreground mb-8 max-w-xs mx-auto leading-relaxed">
        Start an isolated chat session with a specialized agent. Perfect for rapid prototyping and technical debugging.
      </p>
      <Button
        onClick={onCreateSession}
        size="lg"
        className="px-8 rounded-2xl font-black uppercase tracking-[0.2em] shadow-xl shadow-primary/20"
      >
        <Plus className="w-5 h-5 mr-2" />
        New Session
      </Button>
    </div>
  );
}

export default function Chat() {
  const {
    sessions, activeSessionId, loading, sending, error,
    fetchSessions, createSession, selectSession, deleteSession, updateSession,
    sendMessage, getActiveMessages, clearError,
    currentAgent, currentModel, currentReasoningEffort, attachments,
    setCurrentAgent, setCurrentModel, setCurrentReasoningEffort,
    addAttachment, removeAttachment, clearAttachments, uploadAttachments,
  } = useChatStore();

  const [input, setInput] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
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

  const filteredSessions = sessions.filter(s => 
    !searchQuery || (s.title && s.title.toLowerCase().includes(searchQuery.toLowerCase()))
  );

  return (
    <div className="relative flex flex-col md:flex-row m-0 md:h-[calc(100vh-5rem)] md:gap-6 animate-fade-in z-0 pb-10">
      {/* Background Pattern */}
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />

      {/* Sessions Sidebar */}
      <aside className={`w-full md:w-80 flex-shrink-0 flex flex-col gap-6 p-4 md:p-0 ${activeSession ? 'hidden md:flex' : 'flex h-full'}`}>
        <div className="space-y-4">
           <Button
            onClick={handleCreateSession}
            disabled={loading}
            className="w-full h-12 rounded-2xl font-black uppercase tracking-[0.2em] shadow-lg shadow-primary/20"
          >
            {loading ? <Loader2 className="w-5 h-5 animate-spin" /> : <Plus className="w-5 h-5 mr-2" />}
            New Chat
          </Button>

          <div className="relative">
             <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
             <Input 
               placeholder="Filter sessions..." 
               value={searchQuery}
               onChange={(e) => setSearchQuery(e.target.value)}
               className="pl-9 h-10 bg-card/50 backdrop-blur-sm border-border rounded-xl"
             />
          </div>
        </div>

        <div className="flex-1 overflow-y-auto space-y-1.5 min-h-0 scrollbar-none mask-fade-bottom">
          {filteredSessions.length > 0 ? (
            filteredSessions.map((session) => (
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
            <div className="text-center py-12 px-4 rounded-3xl border border-dashed border-border bg-muted/5">
               <p className="text-xs font-bold uppercase tracking-widest text-muted-foreground/40">No sessions found</p>
            </div>
          )}
        </div>
      </aside>

      {/* Chat Interface Area */}
      <div className={`flex-1 flex flex-col bg-background/40 backdrop-blur-md border border-border md:rounded-3xl overflow-hidden shadow-2xl ${!activeSession ? 'hidden md:flex' : 'flex h-full'} w-full transition-all duration-500`}>
        {activeSession ? (
          <>
            {/* Unified Header */}
            <div className="px-4 py-4 md:px-8 border-b border-border bg-card/20 backdrop-blur-md z-10 flex items-center justify-between gap-4 shrink-0 shadow-sm">
              <div className="flex items-center gap-4 overflow-hidden">
                <button 
                  onClick={() => selectSession(null)}
                  className="md:hidden p-2 -ml-2 rounded-xl hover:bg-accent text-muted-foreground"
                >
                  <ArrowLeft className="w-5 h-5" />
                </button>
                <div className="w-12 h-12 rounded-2xl bg-gradient-to-br from-primary/10 to-info/10 border border-primary/20 flex items-center justify-center flex-shrink-0 shadow-inner">
                  <Sparkles className="w-6 h-6 text-primary" />
                </div>
                <div className="min-w-0">
                  <h3 className="text-lg font-black text-foreground truncate tracking-tight leading-none mb-1.5">
                    {activeSession.title || `${activeSession.agent || 'AI'} Session`}
                  </h3>
                  <div className="flex items-center gap-2">
                     <Badge variant="outline" className="text-[9px] px-1.5 py-0 font-bold border-border/50 text-muted-foreground uppercase tracking-widest">
                        {activeSession.agent || 'CLAUDE'}
                     </Badge>
                     <span className="text-[10px] font-bold text-muted-foreground/40 uppercase tracking-tighter">
                        {new Date(activeSession.created_at).toLocaleDateString()}
                     </span>
                  </div>
                </div>
              </div>
              
              <div className="flex items-center gap-2">
                 <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => deleteSession(activeSession.id)}
                  className="h-10 w-10 rounded-xl text-muted-foreground hover:text-destructive hover:bg-destructive/10"
                >
                  <Trash2 className="w-4 h-4" />
                </Button>
              </div>
            </div>

            {/* Message Thread */}
            <div className="flex-1 min-h-0 overflow-y-auto p-4 md:p-8 space-y-8 w-full scrollbar-thin scrollbar-thumb-muted">
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
                <div className="flex flex-col items-center justify-center h-full text-center gap-6 opacity-40">
                  <div className="p-6 rounded-full bg-muted border border-border">
                    <Sparkles className="w-12 h-12 text-primary" />
                  </div>
                  <div>
                    <p className="text-xl font-black text-foreground uppercase tracking-widest">Begin Dialogue</p>
                    <p className="text-sm font-medium mt-1">Send a message to initialize the conversation stream.</p>
                  </div>
                </div>
              )}
              <div ref={messagesEndRef} className="h-8" />
            </div>

            {/* Input & Controls */}
            <div className="shrink-0 w-full border-t border-border bg-card/30 backdrop-blur-md p-4 md:p-6 pb-safe">
              {error && (
                <div className="mb-4 p-4 bg-destructive/5 text-destructive border border-destructive/20 text-xs font-bold rounded-2xl flex items-center justify-between animate-shake">
                  <span className="flex items-center gap-2"><Info className="w-4 h-4" />{error}</span>
                  <button onClick={clearError} className="p-1 hover:bg-destructive/10 rounded-full transition-colors">
                    <X className="w-4 h-4" />
                  </button>
                </div>
              )}

              {/* Advanced Message Params */}
              <div className="flex items-center gap-2.5 mb-4 flex-wrap">
                <AgentSelector value={currentAgent} onChange={setCurrentAgent} disabled={sending} />
                <ModelSelector value={currentModel} onChange={setCurrentModel} agent={currentAgent} disabled={sending} />
                <ReasoningSelector value={currentReasoningEffort} onChange={setCurrentReasoningEffort} agent={currentAgent} disabled={sending} />
                <div className="flex-1" />
                <AttachmentPicker
                  attachments={attachments}
                  onAdd={addAttachment}
                  onRemove={removeAttachment}
                  onClear={clearAttachments}
                  onUpload={uploadAttachments}
                />
              </div>

              <form onSubmit={handleSend} className="flex gap-4 items-end relative group">
                <div className="absolute inset-0 bg-primary/5 blur-xl rounded-full opacity-0 group-focus-within:opacity-100 transition-opacity pointer-events-none" />
                <textarea
                  ref={inputRef}
                  value={input}
                  onChange={(e) => {
                    setInput(e.target.value);
                    e.target.style.height = 'auto';
                    e.target.style.height = Math.min(e.target.scrollHeight, 200) + 'px';
                  }}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && !e.shiftKey) {
                      e.preventDefault();
                      handleSend(e);
                    }
                  }}
                  placeholder={`Consult ${currentAgent}...`}
                  disabled={sending}
                  rows={1}
                  className="flex-1 min-h-[56px] max-h-[200px] py-4 px-6 border border-border bg-background/80 backdrop-blur-sm text-foreground placeholder:text-muted-foreground/50 focus:outline-none focus:ring-2 focus:ring-primary/20 rounded-2xl transition-all text-base shadow-inner resize-none overflow-y-auto"
                />
                <Button
                  type="submit"
                  disabled={sending || !input.trim()}
                  className="h-[56px] w-[56px] md:w-auto md:px-8 rounded-2xl font-black uppercase tracking-widest shadow-xl shadow-primary/20 shrink-0 mb-0"
                >
                  {sending ? <Loader2 className="w-5 h-5 animate-spin" /> : <Send className="w-5 h-5 md:mr-2" />}
                  <span className="hidden md:inline">Send</span>
                </Button>
              </form>
            </div>
          </>
        ) : (
          <EmptyChat onCreateSession={handleCreateSession} />
        )}
      </div>
    </div>
  );
}
