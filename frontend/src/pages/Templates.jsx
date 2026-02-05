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

const CATEGORY_COLORS = {
  'Code Analysis': 'border-blue-500', 'Performance Analysis': 'border-orange-500', 'Security Analysis': 'border-red-500', 'Testing Analysis': 'border-emerald-500', 'Infrastructure Analysis': 'border-sky-500', 'Observability Analysis': 'border-indigo-500', 'Migration Analysis': 'border-violet-500', 'UX/Accessibility Analysis': 'border-pink-500', 'Consistency Analysis': 'border-amber-500', 'Java/Spring Boot': 'border-green-600', 'default': 'border-muted'
};

function TemplateIcon({ name, className = "h-5 w-5" }) {
  const Icon = ICON_MAP[name] || Info; return <Icon className={className} />;
}

function TemplatePreviewModal({ template, onClose, onUseTemplate }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-background/80 backdrop-blur-md animate-fade-in">
      <div className="bg-card border border-border/40 rounded-[2rem] shadow-2xl max-w-3xl w-full max-h-[85vh] overflow-hidden flex flex-col animate-fade-up">
        <div className="flex items-start justify-between p-8 border-b border-border/30 bg-muted/10">
          <div className="flex items-center gap-6 flex-1">
            <div className={`p-4 rounded-2xl bg-background border border-border/60 shadow-sm text-primary/60`}><TemplateIcon name={template.icon} className="h-8 w-8" /></div>
            <div className="min-w-0 space-y-1.5">
              <h2 className="text-2xl font-bold text-foreground tracking-tight leading-none">{template.name}</h2>
              <div className="flex items-center gap-3">
                <Badge variant="secondary" className="text-[10px] px-2 py-0 bg-primary/5 text-primary/60 border-primary/10 font-bold uppercase tracking-widest">{template.category}</Badge>
                <Badge variant="outline" className="text-[10px] px-2 py-0 font-bold border-border/40 text-muted-foreground/60 uppercase tracking-widest">{template.executionStrategy === 'multi-agent-consensus' ? 'Multi-Agent' : 'Direct Agent'}</Badge>
              </div>
            </div>
          </div>
          <Button variant="ghost" size="icon" onClick={onClose} className="rounded-xl h-10 w-10 text-muted-foreground/30 hover:bg-accent"><X className="h-5 w-5" /></Button>
        </div>

        <div className="flex-1 overflow-y-auto p-8 space-y-10 scrollbar-thin">
          <section className="space-y-3">
             <h3 className="text-[11px] font-bold uppercase tracking-[0.2em] text-muted-foreground/40 flex items-center gap-2"><Info className="h-3.5 w-3.5" /> Description</h3>
             <p className="text-foreground/80 text-base leading-relaxed font-medium">{template.description}</p>
          </section>
          <section className="space-y-3">
             <h3 className="text-[11px] font-bold uppercase tracking-[0.2em] text-muted-foreground/40 flex items-center gap-2"><Terminal className="h-3.5 w-3.5" /> Logic Blueprint</h3>
             <div className="relative group/pre">
                <div className="absolute inset-0 bg-primary/[0.01] rounded-2xl pointer-events-none" />
                <pre className="bg-muted/30 border border-border/30 rounded-2xl p-6 text-[13px] font-mono whitespace-pre-wrap text-foreground/70 leading-relaxed max-h-96 overflow-y-auto scrollbar-none">{template.prompt}</pre>
             </div>
          </section>
          <section className="space-y-3">
            <h3 className="text-[11px] font-bold uppercase tracking-[0.2em] text-muted-foreground/40">Resource Tags</h3>
            <div className="flex flex-wrap gap-2.5">
              {template.tags.map((tag) => (
                <span key={tag} className="text-[10px] font-bold px-3 py-1 rounded-full bg-secondary/50 text-secondary-foreground border border-border/40 tracking-tight">#{tag}</span>
              ))}
            </div>
          </section>
        </div>

        <div className="flex justify-end gap-4 p-8 border-t border-border/30 bg-primary/[0.01]">
          <Button variant="ghost" onClick={onClose} className="rounded-xl px-6 text-xs font-bold text-muted-foreground/60 hover:text-foreground">Cancel</Button>
          <Button onClick={() => onUseTemplate(template)} className="rounded-xl px-8 h-11 text-xs font-bold uppercase tracking-widest shadow-xl shadow-primary/10">Initialize Template</Button>
        </div>
      </div>
    </div>
  );
}

function TemplateCard({ template, onUse, onPreview }) {
  const borderClass = CATEGORY_COLORS[template.category] || CATEGORY_COLORS.default;
  return (
    <Card className={`group flex flex-col h-full hover:shadow-[0_20px_50px_rgba(0,0,0,0.04)] transition-all duration-500 border-l-[2px] ${borderClass} bg-card/40 backdrop-blur-md overflow-hidden hover:-translate-y-1 hover:border-primary/20`}>
      <CardHeader className="flex-1 p-6 space-y-5">
        <div className="flex items-start justify-between gap-4">
          <div className="p-2.5 rounded-xl bg-background border border-border/60 shadow-sm group-hover:border-primary/30 transition-all duration-500">
            <TemplateIcon name={template.icon} className="h-5 w-5 text-primary/70" />
          </div>
          <Badge variant="secondary" className="text-[9px] px-2 py-0 bg-primary/5 text-primary/60 border-transparent font-bold uppercase tracking-[0.15em] opacity-60 group-hover:opacity-100 transition-opacity">
            {template.executionStrategy === 'multi-agent-consensus' ? 'Loop' : 'Direct'}
          </Badge>
        </div>
        <div className="space-y-2">
          <CardTitle className="text-lg font-bold tracking-tight text-foreground group-hover:text-primary transition-colors duration-300">{template.name}</CardTitle>
          <CardDescription className="text-sm leading-relaxed line-clamp-3 min-h-[4.5em] text-muted-foreground/60 font-medium">{template.description}</CardDescription>
        </div>
        <div className="flex flex-wrap gap-2 pt-2">
          {template.tags.slice(0, 3).map((tag) => <span key={tag} className="text-[10px] font-bold px-2 py-0.5 rounded-lg bg-muted/30 border border-border/20 text-muted-foreground/40 transition-colors group-hover:text-muted-foreground/60">#{tag}</span>)}
          {template.tags.length > 3 && <span className="text-[9px] font-bold text-muted-foreground/30 uppercase tracking-tighter mt-1">+{template.tags.length - 3} more</span>}
        </div>
      </CardHeader>
      <div className="px-6 py-4 mt-auto flex gap-3 border-t border-border/30 bg-primary/[0.01]">
        <Button variant="outline" size="sm" onClick={() => onPreview(template)} className="flex-1 rounded-xl text-[10px] font-bold uppercase tracking-widest border-border/60 hover:bg-background h-9">Details</Button>
        <Button size="sm" onClick={() => onUse(template)} className="flex-1 rounded-xl text-[10px] font-bold uppercase tracking-widest h-9 shadow-lg shadow-primary/10">Use Blueprint</Button>
      </div>
    </Card>
  );
}

export default function Templates() {
  const navigate = useNavigate(); const [selectedCategory, setSelectedCategory] = useState('All'); const [searchQuery, setSearchQuery] = useState(''); const [previewTemplate, setPreviewTemplate] = useState(null);
  const filteredTemplates = useMemo(() => workflowTemplates.filter((t) => (selectedCategory === 'All' || t.category === selectedCategory) && (!searchQuery || t.name.toLowerCase().includes(searchQuery.toLowerCase()) || t.description.toLowerCase().includes(searchQuery.toLowerCase()) || t.tags.some((tag) => tag.toLowerCase().includes(searchQuery.toLowerCase())))), [selectedCategory, searchQuery]);
  const useTemplate = (t) => navigate('/workflows', { state: { template: { prompt: t.prompt, executionStrategy: t.executionStrategy, name: t.name } } });

  return (
    <div className="relative min-h-full space-y-10 pb-12 animate-fade-in">
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />
      <div className="relative overflow-hidden rounded-[2.5rem] border border-border/40 bg-card/20 backdrop-blur-xl p-8 md:p-12 shadow-[0_8px_30px_rgb(0,0,0,0.02)]">
        <div className="absolute top-0 right-0 w-[500px] h-[500px] bg-primary/[0.03] rounded-full blur-[120px] -translate-y-1/2 translate-x-1/4 pointer-events-none" />
        <div className="relative z-10 max-w-2xl space-y-6">
          <div className="inline-flex items-center gap-2.5 px-4 py-1.5 rounded-full bg-primary/5 border border-primary/10 text-primary text-[10px] font-bold uppercase tracking-[0.2em]"><Sparkles className="h-3 w-3 opacity-70" /> Logic Library</div>
          <h1 className="text-4xl md:text-5xl font-bold text-foreground tracking-tight leading-[1.1]">Execution <span className="text-primary/80">Blueprints</span></h1>
          <p className="text-base md:text-lg text-muted-foreground font-medium leading-relaxed max-w-lg">Jumpstart autonomous development with pre-architected logic patterns for analysis, security, and optimization.</p>
        </div>
      </div>

      <div className="sticky top-14 z-30 flex flex-col gap-6 bg-background/80 backdrop-blur-xl py-6 border-b border-border/30">
        <div className="flex flex-col md:flex-row gap-6 md:items-center justify-between">
          <div className="relative flex-1 max-w-md group">
            <Search className="absolute left-4 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground/30 group-focus-within:text-primary/60 transition-colors" />
            <Input placeholder="Query blueprint ecosystem..." value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)} className="h-12 pl-12 pr-6 bg-card/30 border-border/40 rounded-2xl shadow-sm focus-visible:ring-primary/10 transition-all" />
          </div>
          <div className="hidden sm:flex items-center gap-3 text-[10px] font-bold uppercase tracking-widest text-muted-foreground/40 bg-muted/10 px-4 py-2 rounded-xl border border-border/30"><Info className="h-3.5 w-3.5" /><span>{filteredTemplates.length} Blueprints Available</span></div>
        </div>
        <div className="flex gap-2.5 overflow-x-auto pb-2 no-scrollbar mask-fade-right">
          {templateCategories.map((c) => <button key={c} onClick={() => setSelectedCategory(c)} className={`whitespace-nowrap px-5 py-2.5 rounded-xl text-[10px] font-bold uppercase tracking-widest transition-all duration-500 ${selectedCategory === c ? 'bg-primary text-primary-foreground shadow-lg shadow-primary/10 scale-105 z-10' : 'bg-card/20 border border-border/30 text-muted-foreground/60 hover:border-primary/20 hover:text-foreground'}`}>{c}</button>)}
        </div>
      </div>

      {filteredTemplates.length > 0 ? <div className="grid gap-8 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">{filteredTemplates.map((t) => <TemplateCard key={t.id} template={t} onUse={useTemplate} onPreview={setPreviewTemplate} />)}</div>
      : <div className="flex flex-col items-center justify-center py-32 text-center space-y-8 rounded-[3rem] border border-dashed border-border/30 bg-muted/[0.02] animate-fade-in"><div className="p-8 rounded-[2rem] bg-muted/10 border border-border/20 text-muted-foreground/20"><Search className="h-16 w-16" /></div><div className="space-y-3"><h3 className="text-2xl font-bold text-foreground/80 tracking-tight">No match found</h3><p className="text-muted-foreground/40 max-w-xs mx-auto font-medium leading-relaxed">Adjust your telemetry query to locate compatible blueprints.</p></div><Button variant="outline" onClick={() => { setSearchQuery(''); setSelectedCategory('All'); }} className="rounded-2xl font-bold px-8 h-12 border-border/60">Reset Ecosystem</Button></div>}
      {previewTemplate && <TemplatePreviewModal template={previewTemplate} onClose={() => setPreviewTemplate(null)} onUseTemplate={(t) => { setPreviewTemplate(null); useTemplate(t); }} />}
    </div>
  );
}