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
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-fade-in">
      <div className="bg-card border border-border rounded-xl shadow-2xl max-w-2xl w-full max-h-[85vh] overflow-hidden flex flex-col animate-fade-up">
        {/* Header */}
        <div className="flex items-start justify-between p-6 border-b border-border bg-muted/30">
          <div className="flex items-center gap-4 flex-1">
            <div className={`p-3 rounded-xl bg-background border border-border shadow-sm`}>
              <TemplateIcon name={template.icon} className="h-8 w-8 text-primary" />
            </div>
            <div>
              <h2 className="text-xl font-bold text-foreground tracking-tight">{template.name}</h2>
              <div className="flex items-center gap-2 mt-1">
                <Badge variant="secondary" className="text-[9px] uppercase tracking-widest font-black bg-primary/10 text-primary border-primary/20">
                  {template.category}
                </Badge>
              </div>
            </div>
          </div>
          <button onClick={onClose} className="p-2 text-muted-foreground hover:text-foreground">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          <section className="space-y-2">
             <h3 className="text-xs font-bold uppercase tracking-widest text-muted-foreground flex items-center gap-2">
                <Info className="h-3.5 w-3.5" />
                Description
             </h3>
             <p className="text-muted-foreground text-sm leading-relaxed">
                {template.description}
             </p>
          </section>
          <section className="space-y-2">
             <h3 className="text-xs font-bold uppercase tracking-widest text-muted-foreground flex items-center gap-2">
                <Terminal className="h-3.5 w-3.5" />
                Prompt Template
             </h3>
             <pre className="bg-muted/50 border border-border rounded-lg p-4 text-xs font-mono whitespace-pre-wrap text-muted-foreground max-h-96 overflow-y-auto leading-relaxed">
                {template.prompt}
             </pre>
          </section>
        </div>

        {/* Footer */}
        <div className="flex justify-end gap-3 p-6 border-t border-border bg-muted/10">
          <Button variant="outline" onClick={onClose} className="rounded-lg text-xs font-bold">
            Cancel
          </Button>
          <Button onClick={() => onUseTemplate(template)} className="rounded-lg px-6 text-xs font-bold shadow-sm shadow-primary/20">
            Use This Template
          </Button>
        </div>
      </div>
    </div>
  );
}

function TemplateCard({ template, onUse, onPreview }) {
  return (
    <div
      className={`group flex flex-col h-full rounded-xl border border-border bg-card p-3 transition-all hover:border-primary/30 hover:shadow-lg animate-fade-up`}
    >
      <div className="flex-1 space-y-3 cursor-pointer" onClick={() => onPreview(template)}>
        <div className="flex items-start justify-between">
          <div className="p-2 rounded-lg bg-primary/10 text-primary">
            <TemplateIcon name={template.icon} className="h-4 w-4" />
          </div>
          <Badge variant="outline" className="text-[9px] uppercase tracking-widest font-black opacity-60">
            {template.executionStrategy === 'multi-agent-consensus' ? 'Consensus' : 'Direct'}
          </Badge>
        </div>
        <div className="space-y-1">
          <h3 className="font-bold text-sm text-foreground group-hover:text-primary transition-colors">{template.name}</h3>
          <p className="text-xs leading-relaxed line-clamp-3 text-muted-foreground">{template.description}</p>
        </div>
        <div className="flex flex-wrap gap-1.5">
          {template.tags.slice(0, 3).map((t) => (
            <span key={t} className="text-[10px] font-medium px-2 py-0.5 rounded-full bg-muted text-muted-foreground">
              #{t}
            </span>
          ))}
        </div>
      </div>
      <div className="pt-3 mt-auto flex gap-2 border-t border-border/50">
        <Button variant="outline" size="sm" onClick={() => onPreview(template)} className="flex-1 text-xs font-bold">
          <Info className="w-3 h-3 mr-1" />
          Details
        </Button>
        <Button variant="outline" size="sm" onClick={() => onUse(template)} className="flex-1 text-xs font-bold bg-primary/5 border-primary/20 hover:bg-primary/10">
          Apply
        </Button>
      </div>
    </div>
  );
}

export default function Templates() {
  const navigate = useNavigate(); const [selectedCategory, setSelectedCategory] = useState('All'); const [searchQuery, setSearchQuery] = useState(''); const [previewTemplate, setPreviewTemplate] = useState(null);
  const filteredTemplates = useMemo(() => workflowTemplates.filter((t) => (selectedCategory === 'All' || t.category === selectedCategory) && (!searchQuery || t.name.toLowerCase().includes(searchQuery.toLowerCase()) || t.description.toLowerCase().includes(searchQuery.toLowerCase()) || t.tags.some((tag) => tag.toLowerCase().includes(searchQuery.toLowerCase())))), [selectedCategory, searchQuery]);
  const useTemplate = (t) => navigate('/workflows', { state: { template: { prompt: t.prompt, executionStrategy: t.executionStrategy, name: t.name } } });

  return (
    <div className="space-y-6 animate-fade-in pb-10">
      <div className="px-4 sm:px-6 space-y-6">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-border/50 pb-6">
        <div>
          <h1 className="text-2xl font-bold text-foreground tracking-tight">Templates</h1>
          <p className="text-sm text-muted-foreground mt-1">Jumpstart your workflow with pre-configured blueprints</p>
        </div>
        <div className="relative w-full md:w-72">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input placeholder="Search library..." value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)} className="pl-10 h-10 bg-card rounded-xl" />
        </div>
      </div>

      <div className="flex gap-2 overflow-x-auto no-scrollbar pb-2">
        {templateCategories.map((c) => (
          <button
            key={c}
            onClick={() => setSelectedCategory(c)}
            className={`whitespace-nowrap px-4 py-2 rounded-xl text-sm font-semibold transition-all ${
              selectedCategory === c
                ? 'bg-primary text-primary-foreground shadow-lg'
                : 'bg-card border border-border text-muted-foreground hover:border-primary/30'
            }`}
          >
            {c}
          </button>
        ))}
      </div>

      {filteredTemplates.length > 0 ? (
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {filteredTemplates.map((t) => <TemplateCard key={t.id} template={t} onUse={useTemplate} onPreview={setPreviewTemplate} />)}
        </div>
      ) : (
        <div className="text-center py-24 opacity-40">
          <Search className="w-12 h-12 mx-auto mb-4" />
          <p className="text-lg font-medium">No blueprints found</p>
        </div>
      )}
      {previewTemplate && <TemplatePreviewModal template={previewTemplate} onClose={() => setPreviewTemplate(null)} onUseTemplate={(t) => { setPreviewTemplate(null); useTemplate(t); }} />}
      </div>
    </div>
  );
}
