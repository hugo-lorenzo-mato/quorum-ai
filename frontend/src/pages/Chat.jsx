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
    <div className="flex items-center gap-4 p-6 animate-fade-in">
      <div className="w-9 h-9 rounded-2xl bg-primary/5 flex items-center justify-center border border-primary/10 shadow-sm">
        <Bot className="w-4 h-4 text-primary/60" />
      </div>
      <div className="flex gap-1.5 p-4 rounded-3xl bg-muted/20 border border-border/30">
        <span className="w-1 h-1 bg-primary/30 rounded-full animate-bounce" style={{ animationDelay: '0ms' }} />
        <span className="w-1 h-1 bg-primary/30 rounded-full animate-bounce" style={{ animationDelay: '150ms' }} />
        <span className="w-1 h-1 bg-primary/30 rounded-full animate-bounce" style={{ animationDelay: '300ms' }} />
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
    } catch { /* ignore */ }
  };

  return (
    <div className={`flex gap-5 ${isUser ? 'flex-row-reverse' : ''} ${isLast ? 'animate-fade-up' : 'animate-fade-in'}`}>
      <div className={`w-10 h-10 rounded-2xl flex items-center justify-center flex-shrink-0 shadow-sm border transition-all duration-500 hover:scale-105 ${
        isUser ? 'bg-primary border-primary/10 shadow-primary/5' : 'bg-background border-border/60'
      }`}>
        {isUser ? <User className="w-5 h-5 text-primary-foreground" /> : <Logo className="w-5 h-5 text-primary/80" />}
      </div>
      <div className={`group max-w-[85%] flex flex-col ${isUser ? 'items-end' : 'items-start'} space-y-2`}>
        <div className={`rounded-[1.5rem] px-6 py-4 shadow-sm border transition-all duration-300 ${
          isUser
            ? 'bg-primary text-primary-foreground border-primary/10 rounded-tr-none'
            : 'bg-card/60 border-border/40 rounded-tl-none backdrop-blur-md'
        }`}>
          {!isUser && message.agent && (
            <div className="flex items-center gap-2 mb-2">
               <Badge variant="secondary" className="text-[9px] px-2 py-0 bg-primary/5 text-primary/60 border-primary/10 font-bold uppercase tracking-widest">
                  {agentName}
               </Badge>
            </div>
          )}
          <div className={`text-sm leading-relaxed font-medium ${isUser ? 'text-primary-foreground' : 'text-foreground/90'}`}>
            <ChatMarkdown content={message.content} isUser={isUser} />
          </div>
        </div>
        
        <div className={`flex items-center gap-4 px-2 opacity-0 group-hover:opacity-100 transition-opacity duration-300 ${isUser ? 'flex-row-reverse' : ''}`}>
          <p className="text-[9px] font-bold text-muted-foreground/30 uppercase tracking-widest font-mono">
            {new Date(message.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
          </p>
          <button
            type="button"
            onClick={handleCopy}
            className={`p-1.5 rounded-lg transition-all ${copied ? 'text-green-500 bg-green-500/5' : 'text-muted-foreground/30 hover:text-primary hover:bg-primary/5'}`}
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
  const displayTitle = session.title || `${session.agent || 'AI'} Context`;

  const handleStartEdit = (e) => {
    e.stopPropagation(); setEditValue(session.title || ''); setIsEditing(true);
    setTimeout(() => inputRef.current?.focus(), 0);
  };

  const handleSave = () => {
    const newTitle = editValue.trim();
    if (newTitle && newTitle !== (session.title || '')) onRename(session.id, newTitle);
    setIsEditing(false);
  };

  const handleKeyDown = (e) => { if (e.key === 'Enter') handleSave(); else if (e.key === 'Escape') setIsEditing(false); };

  return (
    <button
      onClick={onClick}
      className={`w-full text-left p-4 rounded-2xl transition-all duration-300 group border ${
        isActive
          ? 'bg-primary/5 border-primary/20 text-primary shadow-sm'
          : 'border-transparent hover:bg-accent/40 text-muted-foreground/60 hover:text-foreground'
      }`}
    >
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-4 flex-1 min-w-0">
          <div className={`p-2.5 rounded-xl transition-colors duration-500 ${isActive ? 'bg-primary/10' : 'bg-muted/30 group-hover:bg-muted/50'}`}>
             <MessageCircle className={`w-4 h-4 ${isActive ? 'text-primary' : 'text-muted-foreground/40'}`} />
          </div>
          <div className="flex-1 min-w-0">
            {isEditing ? (
              <div className="flex items-center gap-1" onClick={e => e.stopPropagation()}>
                <input ref={inputRef} type="text" value={editValue} onChange={(e) => setEditValue(e.target.value)} onKeyDown={handleKeyDown} onBlur={handleSave} className="flex-1 min-w-0 text-sm font-semibold bg-background border border-primary/20 rounded-xl px-3 py-1.5 outline-none shadow-inner" />
              </div>
            ) : (
              <p className="text-sm font-bold truncate tracking-tight transition-colors">{displayTitle}</p>
            )}
            <p className="text-[10px] font-bold uppercase tracking-[0.15em] opacity-30 mt-1 font-mono">
              {new Date(session.created_at).toLocaleDateString()}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-all duration-300">
          {!isEditing && <button onClick={handleStartEdit} className="p-2 text-muted-foreground/40 hover:text-primary hover:bg-primary/5 rounded-xl"><Pencil className="w-3.5 h-3.5" /></button>}
          <button onClick={(e) => { e.stopPropagation(); onDelete(session.id); }} className="p-2 text-muted-foreground/40 hover:text-destructive hover:bg-destructive/5 rounded-xl"><Trash2 className="w-3.5 h-3.5" /></button>
        </div>
      </div>
    </button>
  );
}

function EmptyChat({ onCreateSession }) {
  return (
    <div className="flex-1 flex flex-col items-center justify-center p-12 text-center animate-fade-in">
      <div className="relative mb-10">
        <div className="absolute inset-0 bg-primary/10 blur-[60px] rounded-full" />
        <div className="relative p-10 rounded-[2.5rem] bg-card/40 border border-border/40 shadow-sm backdrop-blur-xl">
          <MessageSquare className="w-14 h-14 text-primary/60" />
        </div>
      </div>
      <h3 className="text-2xl font-bold text-foreground tracking-tighter mb-3">Direct Intelligence Node</h3>
      <p className="text-base text-muted-foreground/60 mb-10 max-w-xs mx-auto leading-relaxed font-medium">
        Initialize a secure diagnostic stream with a specialized provider.
      </p>
      <Button
        onClick={onCreateSession}
        size="lg"
        className="px-10 h-14 rounded-2xl font-bold uppercase tracking-[0.2em] shadow-xl shadow-primary/10 hover:shadow-primary/20 transition-all"
      >
        <Plus className="w-5 h-5 mr-3" /> Initialize Node
      </Button>
    </div>
  );
}

export default function Chat() {
  const { sessions, activeSessionId, loading, sending, error, fetchSessions, createSession, selectSession, deleteSession, updateSession, sendMessage, getActiveMessages, clearError, currentAgent, currentModel, currentReasoningEffort, attachments, setCurrentAgent, setCurrentModel, setCurrentReasoningEffort, addAttachment, removeAttachment, clearAttachments, uploadAttachments } = useChatStore();
  const [input, setInput] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const messagesEndRef = useRef(null);
  const inputRef = useRef(null);
  const activeMessages = getActiveMessages();
  const activeSession = sessions.find((s) => s.id === activeSessionId);

  useEffect(() => { fetchSessions(); }, [fetchSessions]);
  useEffect(() => { messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' }); }, [activeMessages, activeSessionId]);

  const handleSend = async (e) => {
    e.preventDefault(); if (!input.trim() || sending) return;
    const msg = input.trim(); setInput(''); if (inputRef.current) inputRef.current.style.height = 'auto';
    await sendMessage(msg); inputRef.current?.focus();
  };

  const handleCreateSession = async () => { await createSession(); };
  const handleRenameSession = async (id, t) => { await updateSession(id, { title: t }); };
  const filteredSessions = sessions.filter(s => !searchQuery || (s.title && s.title.toLowerCase().includes(searchQuery.toLowerCase())));

  return (
    <div className="relative flex flex-col md:flex-row m-0 md:h-[calc(100vh-5rem)] md:gap-8 animate-fade-in z-0 pb-12">
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />

      <aside className={`w-full md:w-80 flex-shrink-0 flex flex-col gap-8 p-4 md:p-0 ${activeSession ? 'hidden md:flex' : 'flex h-full'}`}>
        <div className="space-y-6">
           <Button onClick={handleCreateSession} disabled={loading} className="w-full h-14 rounded-[1.25rem] font-bold uppercase tracking-[0.2em] shadow-lg shadow-primary/10">
            {loading ? <Loader2 className="w-5 h-5 animate-spin" /> : <Plus className="w-5 h-5 mr-3" />} New Context
          </Button>
          <div className="relative group">
             <Search className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground/30 group-focus-within:text-primary/60 transition-colors" />
             <Input placeholder="Filter diagnostic streams..." value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)} className="pl-12 h-12 bg-card/20 backdrop-blur-md border-border/40 rounded-2xl shadow-sm transition-all" />
          </div>
        </div>

        <div className="flex-1 overflow-y-auto space-y-2 min-h-0 scrollbar-none mask-fade-bottom">
          {filteredSessions.length > 0 ? filteredSessions.map((s) => (
            <SessionItem key={s.id} session={s} isActive={activeSessionId === s.id} onClick={() => selectSession(s.id)} onDelete={deleteSession} onRename={handleRenameSession} />
          )) : <div className="text-center py-16 px-6 rounded-[2rem] border border-dashed border-border/30 bg-muted/[0.02]"><p className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/20 italic">No historical contexts</p></div>}
        </div>
      </aside>

      <div className={`flex-1 flex flex-col bg-card/30 backdrop-blur-2xl border border-border/40 md:rounded-[2.5rem] overflow-hidden shadow-[0_20px_50px_rgba(0,0,0,0.05)] ${!activeSession ? 'hidden md:flex' : 'flex h-full'} w-full transition-all duration-700 ease-in-out`}>
        {activeSession ? (
          <>
            <div className="px-6 py-6 md:px-12 border-b border-border/30 bg-primary/[0.01] flex items-center justify-between gap-6 shrink-0 backdrop-blur-xl">
              <div className="flex items-center gap-5 overflow-hidden">
                <button onClick={() => selectSession(null)} className="md:hidden p-2.5 -ml-3 rounded-xl hover:bg-accent text-muted-foreground/60"><ArrowLeft className="w-5 h-5" /></button>
                <div className="w-14 h-14 rounded-3xl bg-background border border-border/60 flex items-center justify-center flex-shrink-0 shadow-sm"><Sparkles className="w-7 h-7 text-primary/60" /></div>
                <div className="min-w-0 space-y-1.5">
                  <h3 className="text-xl font-bold text-foreground truncate tracking-tight leading-none">{activeSession.title || 'Diagnostic Node'}</h3>
                  <div className="flex items-center gap-3">
                     <Badge variant="outline" className="text-[9px] px-2 py-0 font-bold border-border/40 text-muted-foreground/60 uppercase tracking-widest bg-muted/5">{activeSession.agent || 'NODE'}</Badge>
                     <span className="text-[10px] font-bold text-muted-foreground/30 uppercase tracking-tighter font-mono">{new Date(activeSession.created_at).toLocaleDateString()}</span>
                  </div>
                </div>
              </div>
              <Button variant="ghost" size="icon" onClick={() => deleteSession(activeSession.id)} className="h-11 w-11 rounded-[1rem] text-muted-foreground/30 hover:text-destructive hover:bg-destructive/5"><Trash2 className="w-5 h-5" /></Button>
            </div>

            <div className="flex-1 min-h-0 overflow-y-auto p-6 md:p-12 space-y-10 w-full scrollbar-thin scrollbar-thumb-muted/20 selection:bg-primary/10">
              {activeMessages.length > 0 ? (
                <>{activeMessages.map((m, i) => <MessageBubble key={m.id || i} message={m} isLast={i === activeMessages.length - 1} />)}{sending && <TypingIndicator />}</>
              ) : (
                <div className="flex flex-col items-center justify-center h-full text-center gap-10 opacity-20 py-20">
                  <div className="p-10 rounded-[2.5rem] border-2 border-dashed border-border/40"><Sparkles className="w-16 h-16 text-primary/60" /></div>
                  <div className="space-y-2"><p className="text-xl font-bold tracking-widest uppercase">Begin Protocol</p><p className="text-sm font-medium">Initialize message stream to begin technical consultation.</p></div>
                </div>
              )}
              <div ref={messagesEndRef} className="h-12" />
            </div>

            <div className="shrink-0 w-full border-t border-border/30 bg-background/40 backdrop-blur-xl p-6 md:p-10 pb-safe">
              {error && <div className="mb-6 p-5 bg-destructive/[0.02] text-destructive border border-destructive/20 text-xs font-bold rounded-2xl flex items-center justify-between animate-shake shadow-sm"><span className="flex items-center gap-3"><Info className="w-4 h-4" />{error}</span><button onClick={clearError} className="p-1.5 hover:bg-destructive/10 rounded-full transition-all"><X className="w-4 h-4" /></button></div>}

              <div className="flex items-center gap-3 mb-6 flex-wrap">
                <AgentSelector value={currentAgent} onChange={setCurrentAgent} disabled={sending} />
                <ModelSelector value={currentModel} onChange={setCurrentModel} agent={currentAgent} disabled={sending} />
                <ReasoningSelector value={currentReasoningEffort} onChange={setCurrentReasoningEffort} agent={currentAgent} disabled={sending} />
                <div className="flex-1" />
                <AttachmentPicker attachments={attachments} onAdd={addAttachment} onRemove={removeAttachment} onClear={clearAttachments} onUpload={uploadAttachments} />
              </div>

              <form onSubmit={handleSend} className="flex gap-5 items-end relative group/input">
                <div className="absolute inset-0 bg-primary/[0.02] blur-3xl rounded-full opacity-0 group-focus-within/input:opacity-100 transition-opacity duration-1000 pointer-events-none" />
                <textarea ref={inputRef} value={input} onChange={(e) => { setInput(e.target.value); e.target.style.height = 'auto'; e.target.style.height = Math.min(e.target.scrollHeight, 250) + 'px'; }} onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSend(e); } }} placeholder={`Consult ${currentAgent}...`} disabled={sending} rows={1} className="flex-1 min-h-[64px] max-h-[250px] py-5 px-8 border border-border/40 bg-background/60 backdrop-blur-md text-foreground placeholder:text-muted-foreground/30 focus:outline-none focus:ring-2 focus:ring-primary/10 rounded-3xl transition-all text-base shadow-inner resize-none overflow-y-auto font-medium leading-relaxed" />
                <Button type="submit" disabled={sending || !input.trim()} className="h-[64px] w-[64px] md:w-auto md:px-10 rounded-3xl font-bold uppercase tracking-[0.15em] shadow-xl shadow-primary/10 shrink-0 transition-all duration-500 hover:scale-[1.02] active:scale-95">
                  {sending ? <Loader2 className="w-5 h-5 animate-spin" /> : <Send className="w-5 h-5 md:mr-3" />}
                  <span className="hidden md:inline">Transmit</span>
                </Button>
              </form>
            </div>
          </>
        ) : <EmptyChat onCreateSession={handleCreateSession} />}
      </div>
    </div>
  );
}