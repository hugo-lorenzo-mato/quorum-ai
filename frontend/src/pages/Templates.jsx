import { useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { workflowTemplates, templateCategories } from '../data/workflowTemplates';
import { Card, CardHeader, CardTitle, CardDescription } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Badge } from '../components/ui/Badge';
import { Input } from '../components/ui/Input';
import {
  Search,
  Sparkles,
  X,
  BarChart3,
  Trash2,
  Flame,
  Package,
  Landmark,
  Database,
  Box,
  Droplets,
  Atom,
  Lock,
  Shield,
  Key,
  Globe,
  Target,
  Dices,
  CheckCircle2,
  Container,
  RefreshCw,
  FileText,
  AlertTriangle,
  LineChart,
  Code2,
  Accessibility,
  Smartphone,
  Map,
  Link,
  Palette,
  Leaf,
  Save,
  Shuffle,
  Zap,
  Activity,
  ChevronRight,
  Info,
  Terminal
} from 'lucide-react';

const ICON_MAP = {
  'analysis': BarChart3, 'trash': Trash2, 'flame': Flame, 'package': Package, 'architecture': Landmark, 'database': Database, 'bundle': Box, 'memory': Droplets, 'react': Atom, 'lock': Lock, 'shield': Shield, 'key': Key, 'globe': Globe, 'target': Target, 'dices': Dices, 'check': CheckCircle2, 'container': Container, 'refresh': RefreshCw, 'logging': FileText, 'warning': AlertTriangle, 'monitoring': LineChart, 'typescript': Code2, 'accessibility': Accessibility, 'mobile': Smartphone, 'map': Map, 'link': Link, 'palette': Palette, 'leaf': Leaf, 'save': Save, 'shuffle': Shuffle, 'zap': Zap, 'activity': Activity
};

function TemplateIcon({ name, className = "h-5 w-5" }) {
  const Icon = ICON_MAP[name] || Info; return <Icon className={className} />;
}

function TemplatePreviewModal({ template, onClose, onUseTemplate }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-background/80 backdrop-blur-md animate-fade-in">
      <div className="bg-card border border-border/30 rounded-3xl shadow-2xl max-w-2xl w-full max-h-[85vh] overflow-hidden flex flex-col animate-fade-up">
        <div className="flex items-start justify-between p-8 border-b border-border/20">
          <div className="flex items-center gap-5">
            <div className={`p-3 rounded-2xl bg-primary/[0.03] text-primary/60`}><TemplateIcon name={template.icon} className="h-6 w-6" /></div>
            <div className="min-w-0">
              <h2 className="text-xl font-bold text-foreground tracking-tight">{template.name}</h2>
              <div className="flex items-center gap-2 mt-1">
                <Badge variant="outline" className="text-[9px] px-1.5 py-0 font-bold border-primary/20 text-primary/60 uppercase tracking-widest">{template.category}</Badge>
              </div>
            </div>
          </div>
          <button onClick={onClose} className="p-2 text-muted-foreground/30 hover:text-foreground"><X className="w-5 h-5" /></button>
        </div>

        <div className="flex-1 overflow-y-auto p-8 space-y-8 scrollbar-none">
          <section className="space-y-3">
             <h3 className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/40">Blueprint Description</h3>
             <p className="text-foreground/70 text-sm leading-relaxed font-medium">{template.description}</p>
          </section>
          <section className="space-y-3">
             <h3 className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/40">Neural Logic</h3>
             <pre className="bg-muted/20 border border-border/20 rounded-2xl p-6 text-xs font-mono whitespace-pre-wrap text-foreground/60 leading-relaxed max-h-96 overflow-y-auto">{template.prompt}</pre>
          </section>
        </div>

        <div className="flex justify-end gap-3 p-8 border-t border-border/20">
          <Button variant="ghost" onClick={onClose} className="text-xs font-bold text-muted-foreground/40">Abort</Button>
          <Button onClick={() => onUseTemplate(template)} className="rounded-xl px-6 h-10 text-xs font-bold uppercase tracking-widest shadow-lg shadow-primary/10">Initialize Blueprint</Button>
        </div>
      </div>
    </div>
  );
}

function TemplateCard({ template, onUse, onPreview }) {
  return (
    <div
      className={`group flex flex-col h-full rounded-2xl border border-border/30 bg-card/10 backdrop-blur-sm transition-all duration-500 hover:shadow-soft hover:border-primary/20 hover:-translate-y-0.5 overflow-hidden`}
    >
      <div className="flex-1 p-6 space-y-5 cursor-pointer">
        <div className="flex items-start justify-between">
          <div className="p-2 rounded-xl bg-primary/[0.03] border border-primary/5 transition-all group-hover:border-primary/20">
            <TemplateIcon name={template.icon} className="h-4 w-4 text-primary/60" />
          </div>
          <span className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground/30">{template.executionStrategy === 'multi-agent-consensus' ? 'LOOP' : 'DIRECT'}</span>
        </div>
        <div className="space-y-1">
          <h3 className="font-bold text-base text-foreground/90 transition-colors duration-300 group-hover:text-primary">{template.name}</h3>
          <p className="text-xs leading-relaxed line-clamp-2 text-muted-foreground/50 font-medium">{template.description}</p>
        </div>
        <div className="flex flex-wrap gap-2 pt-1">
          {template.tags.slice(0, 3).map((t) => <span key={t} className="text-[10px] font-bold text-muted-foreground/30 uppercase tracking-tight">#{t}</span>)}
        </div>
      </div>
      <div className="px-6 py-3 mt-auto flex gap-3 border-t border-border/20 bg-background/20">
        <Button variant="ghost" size="sm" onClick={() => onPreview(template)} className="flex-1 rounded-lg text-[10px] font-bold uppercase tracking-widest text-muted-foreground/40 hover:text-primary">Details</Button>
        <Button size="sm" onClick={() => onUse(template)} className="flex-1 rounded-xl text-[10px] font-bold uppercase tracking-widest h-8 shadow-md shadow-primary/5">Apply</Button>
      </div>
    </div>
  );
}

export default function Templates() {
  const navigate = useNavigate(); const [selectedCategory, setSelectedCategory] = useState('All'); const [searchQuery, setSearchQuery] = useState(''); const [previewTemplate, setPreviewTemplate] = useState(null);
  const filteredTemplates = useMemo(() => workflowTemplates.filter((t) => (selectedCategory === 'All' || t.category === selectedCategory) && (!searchQuery || t.name.toLowerCase().includes(searchQuery.toLowerCase()) || t.description.toLowerCase().includes(searchQuery.toLowerCase()) || t.tags.some((tag) => tag.toLowerCase().includes(searchQuery.toLowerCase())))), [selectedCategory, searchQuery]);
  const useTemplate = (t) => navigate('/workflows', { state: { template: { prompt: t.prompt, executionStrategy: t.executionStrategy, name: t.name } } });

  return (
    <div className="relative min-h-full space-y-8 pb-12 animate-fade-in">
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />
      
      <header className="flex flex-col md:flex-row md:items-end justify-between gap-6 pt-4 pb-2 border-b border-border/20">
        <div className="space-y-1">
          <div className="flex items-center gap-2 text-primary"><div className="w-1 h-1 rounded-full bg-current" /><span className="text-[10px] font-bold uppercase tracking-widest opacity-70">Knowledge Base</span></div>
          <h1 className="text-3xl font-bold tracking-tight text-foreground">Pattern <span className="text-muted-foreground/40 font-medium">Library</span></h1>
        </div>
        <div className="relative group"><Search className="absolute left-3.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground/30 group-focus-within:text-primary/60 transition-colors" /><Input placeholder="Query library..." value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)} className="h-10 pl-10 pr-4 bg-card/20 border-border/30 rounded-2xl text-xs shadow-sm transition-all" /></div>
      </header>

      <div className="flex gap-3 overflow-x-auto no-scrollbar mask-fade-right pb-2">
        {templateCategories.map((c) => <button key={c} onClick={() => setSelectedCategory(c)} className={`whitespace-nowrap px-4 py-1.5 rounded-xl text-[10px] font-bold uppercase tracking-widest transition-all duration-500 ${selectedCategory === c ? 'bg-primary text-primary-foreground shadow-lg shadow-primary/10' : 'text-muted-foreground/40 hover:text-foreground'}`}>{c}</button>)}
      </div>

      {filteredTemplates.length > 0 ? <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">{filteredTemplates.map((t) => <TemplateCard key={t.id} template={t} onUse={useTemplate} onPreview={setPreviewTemplate} />)}</div>
      : <div className="text-center py-32 opacity-20"><Search className="w-12 h-12 mx-auto mb-4" /><p className="text-xs font-bold uppercase tracking-widest">No blueprints match criteria</p></div>}
      {previewTemplate && <TemplatePreviewModal template={previewTemplate} onClose={() => setPreviewTemplate(null)} onUseTemplate={(t) => { setPreviewTemplate(null); useTemplate(t); }} />}
    </div>
  );
}
