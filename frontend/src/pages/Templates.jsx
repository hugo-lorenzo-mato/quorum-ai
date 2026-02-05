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
  'analysis': BarChart3,
  'trash': Trash2,
  'flame': Flame,
  'package': Package,
  'architecture': Landmark,
  'database': Database,
  'bundle': Box,
  'memory': Droplets,
  'react': Atom,
  'lock': Lock,
  'shield': Shield,
  'key': Key,
  'globe': Globe,
  'target': Target,
  'dices': Dices,
  'check': CheckCircle2,
  'container': Container,
  'refresh': RefreshCw,
  'logging': FileText,
  'warning': AlertTriangle,
  'monitoring': LineChart,
  'typescript': Code2,
  'accessibility': Accessibility,
  'mobile': Smartphone,
  'map': Map,
  'link': Link,
  'palette': Palette,
  'leaf': Leaf,
  'save': Save,
  'shuffle': Shuffle,
  'zap': Zap,
  'activity': Activity
};

const CATEGORY_COLORS = {
  'Code Analysis': 'border-blue-500',
  'Performance Analysis': 'border-orange-500',
  'Security Analysis': 'border-red-500',
  'Testing Analysis': 'border-emerald-500',
  'Infrastructure Analysis': 'border-sky-500',
  'Observability Analysis': 'border-indigo-500',
  'Migration Analysis': 'border-violet-500',
  'UX/Accessibility Analysis': 'border-pink-500',
  'Consistency Analysis': 'border-amber-500',
  'Java/Spring Boot': 'border-green-600',
  'default': 'border-muted'
};

function TemplateIcon({ name, className = "h-5 w-5" }) {
  const Icon = ICON_MAP[name] || Info;
  return <Icon className={className} />;
}

function TemplatePreviewModal({ template, onClose, onUseTemplate }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-fade-in">
      <div className="bg-card border border-border rounded-xl shadow-2xl max-w-3xl w-full max-h-[85vh] overflow-hidden flex flex-col animate-fade-up">
        {/* Header */}
        <div className="flex items-start justify-between p-6 border-b border-border bg-muted/30">
          <div className="flex items-center gap-4 flex-1">
            <div className={`p-3 rounded-xl bg-background border border-border shadow-sm`}>
              <TemplateIcon name={template.icon} className="h-8 w-8 text-primary" />
            </div>
            <div>
              <h2 className="text-xl font-bold text-foreground tracking-tight">{template.name}</h2>
              <div className="flex items-center gap-2 mt-1">
                <Badge variant="secondary" className="text-[10px] uppercase tracking-wider font-bold bg-primary/10 text-primary border-primary/20">
                  {template.category}
                </Badge>
                <Badge variant="outline" className="text-[10px] uppercase tracking-wider font-bold">
                  {template.executionStrategy === 'multi-agent-consensus' ? 'Multi-Agent Consensus' : 'Single Agent'}
                </Badge>
              </div>
            </div>
          </div>
          <Button variant="ghost" size="icon" onClick={onClose} className="rounded-full">
            <X className="h-5 w-5" />
          </Button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          <section className="space-y-2">
             <h3 className="text-sm font-semibold text-foreground flex items-center gap-2">
                <Info className="h-4 w-4 text-muted-foreground" />
                Description
             </h3>
             <p className="text-muted-foreground text-sm leading-relaxed">
                {template.description}
             </p>
          </section>

          <section className="space-y-2">
             <h3 className="text-sm font-semibold text-foreground flex items-center gap-2">
                <Terminal className="h-4 w-4 text-muted-foreground" />
                Prompt Template
             </h3>
             <div className="relative group">
                <div className="absolute inset-0 bg-primary/5 rounded-lg opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none" />
                <pre className="bg-muted/50 border border-border rounded-lg p-4 text-xs font-mono whitespace-pre-wrap text-muted-foreground max-h-96 overflow-y-auto leading-relaxed scrollbar-thin">
                    {template.prompt}
                </pre>
             </div>
          </section>

          <section className="space-y-2">
            <h3 className="text-sm font-semibold text-foreground">Tags</h3>
            <div className="flex flex-wrap gap-2">
              {template.tags.map((tag) => (
                <Badge key={tag} variant="secondary" className="text-[10px] px-2 py-0.5 bg-secondary/50 text-secondary-foreground">
                  {tag}
                </Badge>
              ))}
            </div>
          </section>
        </div>

        {/* Footer */}
        <div className="flex justify-end gap-3 p-6 border-t border-border bg-muted/10">
          <Button variant="outline" onClick={onClose} className="rounded-lg">
            Cancel
          </Button>
          <Button onClick={() => onUseTemplate(template)} className="rounded-lg px-6 shadow-sm shadow-primary/20">
            Use This Template
          </Button>
        </div>
      </div>
    </div>
  );
}

function TemplateCard({ template, onUse, onPreview }) {
  const borderClass = CATEGORY_COLORS[template.category] || CATEGORY_COLORS.default;
  
  return (
    <Card className={`group flex flex-col h-full hover:shadow-xl transition-all duration-300 border-l-[4px] ${borderClass} bg-card/50 backdrop-blur-sm overflow-hidden hover:-translate-y-1`}>
      <CardHeader className="flex-1 p-5 space-y-4">
        <div className="flex items-start justify-between gap-4">
          <div className="p-2.5 rounded-lg bg-background border border-border shadow-sm group-hover:border-primary/30 transition-colors">
            <TemplateIcon name={template.icon} className="h-6 w-6 text-primary" />
          </div>
          <Badge variant="secondary" className="text-[9px] uppercase tracking-widest font-black opacity-60 group-hover:opacity-100 transition-opacity">
            {template.executionStrategy === 'multi-agent-consensus' ? 'Consensus' : 'Single'}
          </Badge>
        </div>

        <div className="space-y-2">
          <CardTitle className="text-lg font-bold tracking-tight text-foreground group-hover:text-primary transition-colors">
            {template.name}
          </CardTitle>
          <CardDescription className="text-xs leading-relaxed line-clamp-3 min-h-[4.5em] text-muted-foreground">
            {template.description}
          </CardDescription>
        </div>

        <div className="flex flex-wrap gap-1.5 pt-2">
          {template.tags.slice(0, 3).map((tag) => (
            <span key={tag} className="text-[10px] font-medium px-2 py-0.5 rounded-full bg-muted border border-border/50 text-muted-foreground">
              #{tag}
            </span>
          ))}
          {template.tags.length > 3 && (
            <span className="text-[10px] font-medium px-2 py-0.5 text-muted-foreground/50">
              +{template.tags.length - 3} more
            </span>
          )}
        </div>
      </CardHeader>

      <div className="p-4 pt-0 mt-auto flex gap-2">
        <Button 
          variant="outline" 
          size="sm"
          onClick={() => onPreview(template)}
          className="flex-1 rounded-lg text-xs font-semibold"
        >
          Details
        </Button>
        <Button 
          size="sm"
          onClick={() => onUse(template)}
          className="flex-1 rounded-lg text-xs font-bold shadow-sm shadow-primary/10"
        >
          Use Template
          <ChevronRight className="ml-1 h-3 w-3" />
        </Button>
      </div>
    </Card>
  );
}

export default function Templates() {
  const navigate = useNavigate();
  const [selectedCategory, setSelectedCategory] = useState('All');
  const [searchQuery, setSearchQuery] = useState('');
  const [previewTemplate, setPreviewTemplate] = useState(null);

  const filteredTemplates = useMemo(() => {
    return workflowTemplates.filter((template) => {
      const matchesCategory = selectedCategory === 'All' || template.category === selectedCategory;
      const matchesSearch =
        searchQuery === '' ||
        template.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        template.description.toLowerCase().includes(searchQuery.toLowerCase()) ||
        template.tags.some((tag) => tag.toLowerCase().includes(searchQuery.toLowerCase()));
      return matchesCategory && matchesSearch;
    });
  }, [selectedCategory, searchQuery]);

  const useTemplate = (template) => {
    navigate('/workflows', {
      state: {
        template: {
          prompt: template.prompt,
          executionStrategy: template.executionStrategy,
          name: template.name
        }
      }
    });
  };

  return (
    <div className="min-h-full space-y-8 pb-12 bg-dot-pattern animate-fade-in">
      {/* Hero Header */}
      <div className="relative overflow-hidden rounded-3xl border border-border bg-card/40 backdrop-blur-md p-8 sm:p-12 shadow-inner">
        <div className="absolute top-0 right-0 -translate-y-1/2 translate-x-1/3 w-96 h-96 bg-primary/5 rounded-full blur-3xl pointer-events-none" />
        <div className="absolute bottom-0 left-0 translate-y-1/2 -translate-x-1/3 w-64 h-64 bg-primary/10 rounded-full blur-3xl pointer-events-none" />
        
        <div className="relative z-10 max-w-2xl space-y-4">
          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-primary/10 border border-primary/20 text-primary text-[10px] font-bold uppercase tracking-widest">
            <Sparkles className="h-3 w-3" />
            Productivity Suite
          </div>
          <h1 className="text-4xl sm:text-5xl font-black text-foreground tracking-tight leading-tight">
            Workflow <span className="text-primary">Templates</span>
          </h1>
          <p className="text-lg text-muted-foreground leading-relaxed max-w-xl">
            Jumpstart your AI-driven development with pre-configured blueprints for code analysis, security auditing, performance optimization, and more.
          </p>
        </div>
      </div>

      {/* Control Bar */}
      <div className="sticky top-14 z-30 flex flex-col gap-4 bg-background/80 backdrop-blur-md py-4 border-b border-border/50">
        <div className="flex flex-col md:flex-row gap-4 md:items-center justify-between">
          {/* Search */}
          <div className="relative flex-1 max-w-md">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder="Filter by name, description or tags..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-9 h-11 bg-background border-border rounded-xl shadow-sm focus-visible:ring-primary/20"
            />
            {searchQuery && (
              <button 
                onClick={() => setSearchQuery('')}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              >
                <X className="h-4 w-4" />
              </button>
            )}
          </div>

          {/* Stats */}
          <div className="hidden sm:flex items-center gap-2 text-xs font-medium text-muted-foreground bg-muted/50 px-3 py-2 rounded-lg border border-border/50">
            <Info className="h-3.5 w-3.5" />
            <span>Showing {filteredTemplates.length} templates</span>
          </div>
        </div>

        {/* Categories Tab-like Nav */}
        <div className="flex gap-2 overflow-x-auto pb-2 scrollbar-none mask-fade-right">
          {templateCategories.map((category) => (
            <button
              key={category}
              onClick={() => setSelectedCategory(category)}
              className={`whitespace-nowrap px-4 py-2 rounded-xl text-sm font-semibold transition-all ${
                selectedCategory === category
                  ? 'bg-primary text-primary-foreground shadow-lg shadow-primary/20 scale-105 z-10'
                  : 'bg-card border border-border text-muted-foreground hover:border-primary/30 hover:text-foreground'
              }`}
            >
              {category}
            </button>
          ))}
        </div>
      </div>

      {/* Template Grid */}
      {filteredTemplates.length > 0 ? (
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {filteredTemplates.map((template) => (
            <TemplateCard 
              key={template.id} 
              template={template} 
              onUse={useTemplate}
              onPreview={setPreviewTemplate}
            />
          ))}
        </div>
      ) : (
        <div className="flex flex-col items-center justify-center py-24 text-center space-y-4 rounded-3xl border border-dashed border-border bg-muted/5">
          <div className="p-4 rounded-full bg-muted border border-border">
            <Search className="h-8 w-8 text-muted-foreground/50" />
          </div>
          <div className="space-y-1">
            <h3 className="text-xl font-bold text-foreground">No templates found</h3>
            <p className="text-muted-foreground max-w-xs mx-auto">
              We couldn't find any templates matching "{searchQuery}". Try adjusting your search or filters.
            </p>
          </div>
          <Button variant="outline" onClick={() => { setSearchQuery(''); setSelectedCategory('All'); }} className="rounded-xl">
            Clear all filters
          </Button>
        </div>
      )}

      {/* Preview Modal */}
      {previewTemplate && (
        <TemplatePreviewModal
          template={previewTemplate}
          onClose={() => setPreviewTemplate(null)}
          onUseTemplate={(template) => {
            setPreviewTemplate(null);
            useTemplate(template);
          }}
        />
      )}
    </div>
  );
}