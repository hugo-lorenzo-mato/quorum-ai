import { useMemo, useEffect, useState } from 'react';
import { Tag, Users, X, Loader2, Activity, Brain, Wrench, CheckCircle2, Zap } from 'lucide-react';
import { Button } from '../ui/Button';
import Logo from '../Logo';
import { useAgentStore } from '../../stores';

/**
 * Skeleton component for loading states
 */
function Skeleton({ className = '' }) {
  return (
    <div className={`animate-pulse bg-muted/30 rounded ${className}`} />
  );
}

/**
 * Animated Logo Component for the center of the screen
 */
function NeuralNexus({ status = 'idle', activeCount = 0 }) {
  const isActive = status !== 'idle' && status !== 'completed' && status !== 'error';
  
  const particles = useMemo(() => Array.from({ length: Math.max(activeCount, 1) }), [activeCount]);
  
  return (
    <div className="relative flex items-center justify-center">
      {/* Dynamic Glow - Breathing effect (Simplified) */}
      <div className={`absolute inset-0 rounded-full bg-primary/20 blur-[80px] transition-all duration-1000 ${isActive ? 'opacity-80 animate-pulse' : 'opacity-0'}`} />
      
      {/* Logo Container */}
      <div className={`relative p-12 rounded-full bg-background/10 backdrop-blur-md border border-primary/10 transition-all duration-700 ${isActive ? 'scale-110 shadow-[0_0_50px_rgba(99,102,241,0.2)]' : 'scale-100'}`}>
        <Logo 
          className={`w-32 h-32 transition-all duration-700 ${isActive ? 'drop-shadow-[0_0_20px_rgba(99,102,241,0.8)]' : ''}`} 
        />
        
        {/* Subtle breathing border */}
        {isActive && (
          <div className="absolute inset-0 rounded-full border border-primary/30 animate-pulse" />
        )}
      </div>
      
      {/* Sutil feedback particles */}
      {isActive && activeCount > 0 && particles.map((_, i) => (
        <div 
          key={i}
          className="absolute inset-0 pointer-events-none"
          style={{ transform: `rotate(${(360 / particles.length) * i}deg)` }}
        >
          <div 
            className="absolute top-0 left-1/2 -translate-x-1/2 -translate-y-12 w-1.5 h-1.5 bg-primary rounded-full shadow-[0_0_15px_rgba(99,102,241,1)] animate-ping"
            style={{ animationDelay: `${i * 0.4}s`, animationDuration: '3s' }}
          />
        </div>
      ))}
    </div>
  );
}

/**
 * Mini activity entry for the loading screen
 */
function MiniActivityEntry({ entry }) {
  const Icon = entry.eventKind === 'thinking' ? Brain : 
               entry.eventKind === 'tool_use' ? Wrench : 
               entry.eventKind === 'completed' ? CheckCircle2 : Activity;
               
  return (
    <div className="flex items-center gap-3 py-2 px-3 rounded-lg bg-card/40 border border-border/50 animate-slide-in hover:border-primary/30 transition-colors">
      <div className="p-1.5 rounded bg-primary/10">
        <Icon className="w-3.5 h-3.5 text-primary" />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-[10px] font-bold text-primary uppercase tracking-wider">{entry.agent}</span>
          <span className="text-[10px] text-muted-foreground">{entry.eventKind}</span>
        </div>
        <p className="text-xs text-foreground/80 truncate font-mono">{entry.message}</p>
      </div>
    </div>
  );
}

/**
 * Skeleton issue item in sidebar
 */
function SkeletonIssueItem({ filled = false, issue = null }) {
  if (filled && issue) {
    return (
      <div className="p-3 rounded-lg border border-primary/20 bg-primary/5 animate-fade-in shadow-sm group hover:border-primary/40 transition-colors">
        <div className="flex items-center gap-2 mb-1">
          <span className={`px-1.5 py-0.5 text-[10px] font-bold rounded ${
            issue.is_main_issue
              ? 'bg-primary text-primary-foreground'
              : 'bg-muted text-muted-foreground'
          }`}>
            {issue.is_main_issue ? 'MAIN' : 'SUB'}
          </span>
          {issue.file_path && (
            <span className="text-[10px] text-muted-foreground truncate font-mono max-w-[120px]">
              {issue.file_path.split('/').pop()}
            </span>
          )}
        </div>
        <p className="text-sm font-medium text-foreground line-clamp-1">
          {issue.title}
        </p>
      </div>
    );
  }

  return (
    <div className="p-3 rounded-lg border border-border/30 bg-card/20 opacity-50">
      <div className="flex items-center gap-2 mb-2">
        <Skeleton className="w-8 h-3" />
        <Skeleton className="w-16 h-3" />
      </div>
      <Skeleton className="h-3 w-full" />
    </div>
  );
}

/**
 * Full-screen loading overlay with Neural Nexus and Live Activity.
 */
export default function GenerationLoadingOverlay({
  workflowId,
  progress = 0,
  total = 0,
  generatedIssues = [],
  onCancel,
}) {
  const activity = useAgentStore(state => state.agentActivity[workflowId] || []);
  const agentsMap = useAgentStore(state => state.currentAgents[workflowId] || {});
  
  const activeAgents = useMemo(() => {
    return Object.entries(agentsMap)
      .filter(([, info]) => ['started', 'thinking', 'tool_use', 'progress'].includes(info.status))
      .map(([name, info]) => ({ name, ...info }));
  }, [agentsMap]);
  
  const issueSlots = useMemo(() => {
    const slots = [];
    const generated = generatedIssues || [];

    for (let i = 0; i < generated.length; i++) {
      slots.push({ filled: true, issue: generated[i] });
    }

    const remaining = Math.max(0, (total || 8) - generated.length);
    for (let i = 0; i < remaining; i++) {
      slots.push({ filled: false, issue: null });
    }

    return slots;
  }, [generatedIssues, total]);

  const currentStatus = activeAgents.length > 0 ? activeAgents[0].status : 'idle';
  const stageName = progress === 0 ? "Context Analysis & Planning" : "Issue Synthesis & Drafting";

  return (
    <div className="absolute inset-0 z-30 bg-background flex overflow-hidden animate-fade-in border-0">
      {/* Background Pattern */}
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none opacity-40" />
      
      {/* 1. Synthesis Pipeline - Hidden on mobile, visible on MD+ */}
      <aside className="hidden md:flex w-72 lg:w-80 h-full border-r border-border bg-card/30 backdrop-blur-sm p-4 overflow-y-auto z-10 flex-col gap-4 shrink-0">
         <div className="flex items-center justify-between border-b border-border pb-2">
           <h2 className="text-xs font-bold text-muted-foreground uppercase tracking-wider">Synthesis Pipeline</h2>
           <span className="text-[10px] font-mono bg-primary/10 text-primary px-1.5 py-0.5 rounded border border-primary/20">
             {total > 0 ? Math.round((progress / total) * 100) : 0}%
           </span>
         </div>
         
         <div className="space-y-2.5">
           {issueSlots.map((slot, i) => (
             <SkeletonIssueItem
               key={i}
               filled={slot.filled}
               issue={slot.issue}
             />
           ))}
         </div>
      </aside>

      {/* Right Column: Header + Main + Footer */}
      <div className="flex-1 flex flex-col overflow-hidden relative">
        {/* Top Header - Adjusted for mobile */}
        <header className="flex items-center justify-between px-4 sm:px-6 py-3 sm:py-4 border-b border-border bg-card/50 backdrop-blur-md shrink-0 z-10">
          <div className="flex items-center gap-3 sm:gap-4">
            <div className="w-8 h-8 sm:w-10 sm:h-10 flex items-center justify-center rounded-lg sm:rounded-xl bg-primary/10 border border-primary/20 shadow-[0_0_15px_rgba(99,102,241,0.1)]">
              <Zap className="w-4 h-4 sm:w-5 sm:h-5 text-primary animate-pulse" />
            </div>
            <div>
              <h1 className="text-sm sm:text-lg font-bold text-foreground tracking-tight">Orchestrator</h1>
              <div className="flex flex-col sm:flex-row sm:items-center gap-0.5 sm:gap-3">
                <span className="text-[8px] sm:text-[10px] font-bold text-primary uppercase tracking-wider sm:tracking-[0.2em]">{stageName}</span>
                <span className="hidden sm:inline text-xs text-muted-foreground">•</span>
                <p className="text-[9px] sm:text-xs text-muted-foreground font-mono">
                  {progress} / {total || '??'} Issues
                </p>
              </div>
            </div>
          </div>

          <Button
            variant="outline"
            size="sm"
            onClick={onCancel}
            className="h-8 px-2 sm:px-3 gap-1 sm:gap-2 border-primary/20 hover:bg-primary/5 text-[10px] sm:text-xs font-bold uppercase tracking-wider"
          >
            <X className="w-3 h-3 sm:w-4 sm:h-4" />
            <span className="hidden xs:inline">Abort</span>
          </Button>
        </header>

        {/* Center Stage - Scaled for mobile */}
        <main className="flex-1 flex flex-col items-center justify-center p-4 sm:p-8 relative overflow-y-auto">
          <div className="max-w-2xl w-full flex flex-col items-center gap-8 sm:gap-12 z-10">
            <div className="scale-75 sm:scale-100">
              <NeuralNexus status={currentStatus} activeCount={activeAgents.length} />
            </div>

            <div className="w-full space-y-4 max-w-lg">
              <div className="flex items-center justify-between px-1">
                <div className="flex items-center gap-2">
                  <Activity className="w-3.5 h-3.5 sm:w-4 sm:h-4 text-primary animate-pulse" />
                  <span className="text-[9px] sm:text-[10px] font-bold text-muted-foreground uppercase tracking-[0.2em] sm:tracking-[0.3em]">Agent Telemetry</span>
                </div>
                {activeAgents.length > 0 && (
                  <span className="text-[9px] sm:text-[10px] font-mono text-primary font-bold uppercase">
                    {activeAgents.length} Active
                  </span>
                )}
              </div>
              
              <div className="space-y-1.5 sm:space-y-2 max-h-48 sm:max-h-64 overflow-hidden mask-fade-out">
                {activity.length > 0 ? (
                  activity.slice(0, window.innerWidth < 640 ? 3 : 5).map((entry) => (
                    <MiniActivityEntry key={entry.id} entry={entry} />
                  ))
                ) : (
                  <div className="text-center py-8 sm:py-12 border border-dashed border-border/50 rounded-xl bg-card/20">
                    <Loader2 className="w-4 h-4 sm:w-5 sm:h-5 text-muted-foreground animate-spin mx-auto mb-2" />
                    <p className="text-[10px] sm:text-xs text-muted-foreground font-mono italic">
                      Synchronizing...
                    </p>
                  </div>
                )}
              </div>
            </div>
          </div>
        </main>

        {/* 2. Global Progress Footer - Adjusted for mobile */}
        <footer className="p-4 sm:p-6 shrink-0 z-20 flex justify-center">
          <div className="max-w-4xl w-full bg-card/50 backdrop-blur-xl rounded-xl sm:rounded-2xl border border-border p-4 sm:p-5 shadow-2xl relative overflow-hidden group">
             <div className="absolute -top-24 -left-24 w-48 h-48 bg-primary/5 blur-3xl rounded-full" />
             
             <div className="flex items-center justify-between mb-2 sm:mb-3 px-1 relative z-10">
               <div className="flex items-center gap-2">
                 <span className="text-[10px] sm:text-xs font-bold text-foreground uppercase tracking-wider">Progress</span>
                 {progress > 0 && (
                   <span className="text-[8px] sm:text-[10px] bg-success/10 text-success px-1 sm:px-1.5 py-0.5 rounded font-bold uppercase">Live</span>
                 )}
               </div>
               <span className="text-[10px] sm:text-xs font-mono font-bold text-primary">{progress}/{total}</span>
             </div>
             
             <div className="h-3 sm:h-4 bg-muted/30 rounded-full overflow-hidden border border-border/50 p-0.5 sm:p-1 relative z-10">
               <div 
                 className="h-full bg-gradient-to-r from-primary via-info to-primary rounded-full transition-all duration-1000 ease-in-out relative overflow-hidden"
                 style={{ width: total > 0 ? `${(progress / total) * 100}%` : '8%' }}
               >
                 <div className="absolute inset-0 bg-[linear-gradient(90deg,transparent,rgba(255,255,255,0.3),transparent)] animate-[shimmer_2s_infinite] -translate-x-full" />
               </div>
             </div>
             
             <div className="mt-2 sm:mt-3 flex items-center justify-between px-1 relative z-10">
                <p className="text-[8px] sm:text-[10px] text-muted-foreground uppercase font-bold">
                  Cluster Status: OK
                </p>
                <p className="text-[8px] sm:text-[10px] text-muted-foreground font-mono truncate max-w-[120px]">
                  {activeAgents.length > 0 ? activeAgents[0].name : 'Waiting...'}
                </p>
             </div>
          </div>
        </footer>
      </div>
      
      <style dangerouslySetInnerHTML={{ __html: `
        @keyframes shimmer {
          100% { transform: translateX(100%); }
        }
        .mask-fade-out {
          mask-image: linear-gradient(to bottom, black 80%, transparent 100%);
        }
      `}} />
    </div>
  );
}
