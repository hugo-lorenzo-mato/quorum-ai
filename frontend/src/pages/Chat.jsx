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
  Info,
  Paperclip
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
    <div className="flex items-center gap-3 p-4 animate-fade-in opacity-60">
      <div className="w-8 h-8 rounded-xl bg-primary/5 flex items-center justify-center border border-primary/10">
        <Bot className="w-4 h-4 text-primary/60" />
      </div>
      <div className="flex gap-1">
        <span className="w-1 h-1 bg-primary/40 rounded-full animate-bounce" style={{ animationDelay: '0ms' }} />
        <span className="w-1 h-1 bg-primary/40 rounded-full animate-bounce" style={{ animationDelay: '150ms' }} />
        <span className="w-1 h-1 bg-primary/40 rounded-full animate-bounce" style={{ animationDelay: '300ms' }} />
      </div>
    </div>
  );
}

function MessageBubble({ message, isLast }) {
  const [copied, setCopied] = useState(false);
  const isUser = message.role === 'user';
  const agentName = message.agent ? message.agent.charAt(0).toUpperCase() + message.agent.slice(1) : 'Assistant';

  const handleCopy = async () => {
    try { await navigator.clipboard.writeText(message.content); setCopied(true); setTimeout(() => setCopied(false), 2000); } catch { /* ignore */ }
  };

  return (
    <div className={`flex gap-4 ${isUser ? 'flex-row-reverse' : ''} ${isLast ? 'animate-fade-up' : 'animate-fade-in'}`}>
      <div className={`w-8 h-8 rounded-xl flex items-center justify-center flex-shrink-0 border transition-all duration-500 ${
        isUser ? 'bg-primary border-primary/10' : 'bg-background border-border/60'
      }`}>
        {isUser ? <User className="w-4 h-4 text-primary-foreground" /> : <Logo className="w-4 h-4 text-primary/60" />}
      </div>
      <div className={`group max-w-[85%] flex flex-col ${isUser ? 'items-end' : 'items-start'} space-y-1.5`}>
        <div className={`rounded-2xl px-4 py-3 shadow-sm border transition-all duration-300 ${
          isUser ? 'bg-primary text-primary-foreground border-primary/10' : 'bg-card/40 border-border/40 backdrop-blur-md'
        }`}>
          {!isUser && message.agent && (
            <p className="text-[9px] font-bold uppercase tracking-widest text-primary/60 mb-1.5">{agentName}</p>
          )}
          <div className={`text-[13px] leading-relaxed ${isUser ? 'text-primary-foreground' : 'text-foreground/90'}`}>
            <ChatMarkdown content={message.content} isUser={isUser} />
          </div>
        </div>
        
        <div className={`flex items-center gap-3 px-1 opacity-0 group-hover:opacity-100 transition-opacity ${isUser ? 'flex-row-reverse' : ''}`}>
          <p className="text-[9px] font-medium text-muted-foreground/30 uppercase font-mono">
            {new Date(message.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
          </p>
          <button onClick={handleCopy} className={`p-1 rounded-md transition-all ${copied ? 'text-green-500' : 'text-muted-foreground/20 hover:text-primary hover:bg-primary/5'}`}>
            {copied ? <CheckCircle2 className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
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
  const displayTitle = session.title || `${session.agent || 'AI'} Stream`;

  const handleStartEdit = (e) => { e.stopPropagation(); setEditValue(session.title || ''); setIsEditing(true); setTimeout(() => inputRef.current?.focus(), 0); };
  const handleSave = () => { if (editValue.trim() && editValue.trim() !== (session.title || '')) onRename(session.id, editValue.trim()); setIsEditing(false); };
  const handleKeyDown = (e) => { if (e.key === 'Enter') handleSave(); else if (e.key === 'Escape') setIsEditing(false); };

  return (
    <button onClick={onClick} className={`w-full text-left px-3 py-2.5 rounded-xl transition-all group border ${isActive ? 'bg-primary/[0.03] border-primary/10 text-primary' : 'border-transparent hover:bg-accent/40 text-muted-foreground/60 hover:text-foreground'}`}>
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-3 flex-1 min-w-0">
          <div className={`p-1.5 rounded-lg transition-colors ${isActive ? 'bg-primary/10' : 'bg-muted/30 group-hover:bg-muted/50'}`}>
             <MessageCircle className={`w-3.5 h-3.5 ${isActive ? 'text-primary' : 'text-muted-foreground/40'}`} />
          </div>
          <div className="flex-1 min-w-0">
            {isEditing ? (
              <input ref={inputRef} type="text" value={editValue} onChange={(e) => setEditValue(e.target.value)} onKeyDown={handleKeyDown} onBlur={handleSave} className="w-full text-xs font-bold bg-background border border-primary/20 rounded-lg px-2 py-1 outline-none" />
            ) : (
              <p className="text-xs font-bold truncate tracking-tight">{displayTitle}</p>
            )}
          </div>
        </div>
        {!isEditing && <button onClick={(e) => { e.stopPropagation(); onDelete(session.id); }} className="p-1 opacity-0 group-hover:opacity-100 text-muted-foreground/30 hover:text-destructive transition-all"><Trash2 className="w-3 h-3" /></button>}
      </div>
    </button>
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

  const filteredSessions = sessions.filter(s => !searchQuery || (s.title && s.title.toLowerCase().includes(searchQuery.toLowerCase())));

  return (
    <div className="relative flex flex-col md:flex-row m-0 md:h-[calc(100vh-5rem)] md:gap-6 animate-fade-in z-0 pb-8">
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />

      <aside className={`w-full md:w-72 flex-shrink-0 flex flex-col gap-6 p-4 md:p-0 ${activeSession ? 'hidden md:flex' : 'flex h-full'}`}>
        <div className="space-y-4">
           <Button onClick={() => createSession()} disabled={loading} className="w-full h-11 rounded-xl font-bold text-xs uppercase tracking-widest shadow-sm">
            <Plus className="w-4 h-4 mr-2" /> New Neural Stream
          </Button>
          <div className="relative">
             <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground/30" />
             <Input placeholder="Search streams..." value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)} className="pl-9 h-9 bg-card/20 border-border/40 rounded-xl text-xs" />
          </div>
        </div>

        <div className="flex-1 overflow-y-auto space-y-1 min-h-0 scrollbar-none mask-fade-bottom">
          {filteredSessions.map((s) => <SessionItem key={s.id} session={s} isActive={activeSessionId === s.id} onClick={() => selectSession(s.id)} onDelete={deleteSession} onRename={(id, t) => updateSession(id, { title: t })} />)}
        </div>
      </aside>

      <div className={`flex-1 flex flex-col bg-card/10 backdrop-blur-xl border border-border/30 md:rounded-3xl overflow-hidden shadow-soft ${!activeSession ? 'hidden md:flex' : 'flex h-full'} transition-all duration-700`}>
        {activeSession ? (
          <>
            <header className="px-6 py-4 border-b border-border/20 bg-background/20 flex items-center justify-between gap-4 shrink-0">
              <div className="flex items-center gap-4 min-w-0">
                <button onClick={() => selectSession(null)} className="md:hidden p-2 -ml-2 text-muted-foreground/40 hover:text-foreground"><ArrowLeft className="w-5 h-5" /></button>
                <div className="min-w-0 space-y-0.5">
                  <h3 className="text-sm font-bold text-foreground truncate tracking-tight">{activeSession.title || 'Diagnostic Node'}</h3>
                  <div className="flex items-center gap-2 text-[10px] font-bold text-muted-foreground/40 uppercase tracking-widest">
                     <span>{activeSession.agent || 'SYSTEM'}</span>
                     <span className="opacity-30">/</span>
                     <span>{new Date(activeSession.created_at).toLocaleDateString()}</span>
                  </div>
                </div>
              </div>
              <button onClick={() => deleteSession(activeSession.id)} className="p-2 text-muted-foreground/20 hover:text-destructive transition-colors"><Trash2 className="w-4 h-4" /></button>
            </header>

            <div className="flex-1 min-h-0 overflow-y-auto p-6 space-y-8 w-full scrollbar-thin scrollbar-thumb-muted/20">
              {activeMessages.length > 0 ? (
                <>{activeMessages.map((m, i) => <MessageBubble key={m.id || i} message={m} isLast={i === activeMessages.length - 1} />)}{sending && <TypingIndicator />}</>
              ) : (
                <div className="flex flex-col items-center justify-center h-full text-center gap-4 opacity-20">
                  <Sparkles className="w-10 h-10 text-primary/60" />
                  <p className="text-[10px] font-bold tracking-widest uppercase">Ready for transmission</p>
                </div>
              )}
              <div ref={messagesEndRef} className="h-4" />
            </div>

            <div className="shrink-0 w-full border-t border-border/20 bg-background/40 backdrop-blur-md p-4 md:p-6 pb-safe">
              <div className="flex items-center gap-2.5 mb-4 flex-wrap">
                <AgentSelector value={currentAgent} onChange={setCurrentAgent} disabled={sending} />
                <ModelSelector value={currentModel} onChange={setCurrentModel} agent={currentAgent} disabled={sending} />
                <ReasoningSelector value={currentReasoningEffort} onChange={setCurrentReasoningEffort} agent={currentAgent} disabled={sending} />
                <div className="flex-1" />
                <AttachmentPicker attachments={attachments} onAdd={addAttachment} onRemove={removeAttachment} onClear={clearAttachments} onUpload={uploadAttachments} />
              </div>

              <form onSubmit={handleSend} className="flex gap-3 items-end relative group/input">
                <textarea ref={inputRef} value={input} onChange={(e) => { setInput(e.target.value); e.target.style.height = 'auto'; e.target.style.height = Math.min(e.target.scrollHeight, 180) + 'px'; }} onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSend(e); } }} placeholder={`Consult ${currentAgent}...`} disabled={sending} rows={1} className="flex-1 min-h-[44px] max-h-[180px] py-3 px-5 border border-border/40 bg-background/60 text-sm focus:outline-none focus:ring-1 focus:ring-primary/20 rounded-2xl transition-all shadow-inner resize-none overflow-y-auto font-medium" />
                <Button type="submit" disabled={sending || !input.trim()} className="h-11 w-11 md:w-auto md:px-6 rounded-2xl font-bold uppercase text-[10px] tracking-widest shadow-lg shadow-primary/5 shrink-0 transition-all active:scale-95">
                  {sending ? <Loader2 className="w-4 h-4 animate-spin" /> : <Send className="w-4 h-4 md:mr-2" />}
                  <span className="hidden md:inline">Send</span>
                </Button>
              </form>
            </div>
          </>
        ) : (
          <div className="flex-1 flex flex-col items-center justify-center p-8 text-center animate-fade-in opacity-40">
            <MessageSquare className="w-12 h-12 mb-4" />
            <p className="text-xs font-bold uppercase tracking-widest">No Active Node</p>
            <button onClick={() => createSession()} className="mt-6 text-xs font-bold text-primary hover:underline">Initialize Stream</button>
          </div>
        )}
      </div>
    </div>
  );
}
