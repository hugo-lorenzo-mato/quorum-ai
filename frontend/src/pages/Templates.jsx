import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { workflowTemplates, templateCategories } from '../data/workflowTemplates';
import { Badge } from '../components/ui/Badge';
import { 
  Search, 
  Sparkles, 
  X, 
  Network, 
  Zap,
  BarChart3,
  Trash2,
  Flame,
  Package,
  Layers,
  Database,
  Box,
  Cpu,
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
  Activity,
  FileCode2,
  Accessibility,
  Smartphone,
  Map,
  Link,
  Palette,
  Leaf,
  Save,
  Shuffle
} from 'lucide-react';

const ICON_MAP = {
  analysis: BarChart3,
  trash: Trash2,
  flame: Flame,
  package: Package,
  architecture: Layers,
  database: Database,
  bundle: Box,
  memory: Cpu,
  react: Atom,
  lock: Lock,
  shield: Shield,
  key: Key,
  globe: Globe,
  target: Target,
  dices: Dices,
  check: CheckCircle2,
  container: Container,
  refresh: RefreshCw,
  logging: FileText,
  warning: AlertTriangle,
  monitoring: Activity,
  typescript: FileCode2,
  accessibility: Accessibility,
  mobile: Smartphone,
  map: Map,
  link: Link,
  palette: Palette,
  leaf: Leaf,
  save: Save,
  shuffle: Shuffle,
  zap: Zap
};

function TemplateIcon({ name, className = "" }) {
  const Icon = ICON_MAP[name] || Sparkles;
  return <Icon className={className} />;
}

function TemplatePreviewModal({ template, onClose, onUseTemplate }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-background/80 backdrop-blur-sm animate-fade-in">
      <div className="bg-card border border-border shadow-2xl max-w-2xl w-full max-h-[90vh] overflow-hidden flex flex-col rounded-2xl animate-fade-up">
        {/* Header */}
        <div className="relative flex items-start justify-between p-6 border-b border-border/50 bg-muted/5">
          <div className="absolute top-0 left-6 right-6 h-0.5 rounded-full bg-gradient-to-r from-transparent via-primary to-transparent" />
          <div className="flex items-center gap-5">
            <div className="p-3 rounded-2xl bg-primary/10 border border-primary/20 text-primary">
              <TemplateIcon name={template.icon} className="w-8 h-8" />
            </div>
            <div className="min-w-0">
              <h2 className="text-xl font-bold text-foreground tracking-tight">{template.name}</h2>
              <div className="flex items-center gap-2 mt-1.5">
                <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70 bg-muted/50 px-2 py-0.5 rounded border border-border/50">
                  {template.category}
                </span>
                <Badge variant={template.executionStrategy === 'multi-agent-consensus' ? 'default' : 'outline'} className="text-[10px] h-5">
                  {template.executionStrategy === 'multi-agent-consensus' ? 'Multi-Agent' : 'Single-Agent'}
                </Badge>
              </div>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-2 rounded-lg hover:bg-accent text-muted-foreground hover:text-foreground transition-all"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          <div>
            <h3 className="text-xs font-bold uppercase tracking-widest text-muted-foreground mb-3 flex items-center gap-2">
              <Sparkles className="w-3 h-3" />
              Description
            </h3>
            <p className="text-sm text-foreground/90 leading-relaxed bg-muted/20 p-4 rounded-xl border border-border/30">
              {template.description}
            </p>
          </div>

          <div>
            <h3 className="text-xs font-bold uppercase tracking-widest text-muted-foreground mb-3">Tags</h3>
            <div className="flex flex-wrap gap-2">
              {template.tags.map((tag) => (
                <Badge key={tag} variant="secondary" className="text-xs font-medium px-2.5">
                  {tag}
                </Badge>
              ))}
            </div>
          </div>

          <div>
            <h3 className="text-xs font-bold uppercase tracking-widest text-muted-foreground mb-3">Prompt Template</h3>
            <div className="relative group">
              <div className="absolute -inset-0.5 bg-gradient-to-r from-primary/10 to-transparent rounded-xl blur opacity-20 group-hover:opacity-40 transition-opacity" />
              <div className="relative bg-muted/30 backdrop-blur-sm rounded-xl p-5 text-sm font-mono whitespace-pre-wrap text-foreground/80 max-h-80 overflow-y-auto border border-border/50 shadow-inner leading-relaxed">
                {template.prompt}
              </div>
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="flex justify-end items-center gap-3 p-6 border-t border-border/50 bg-muted/5">
          <button
            onClick={onClose}
            className="h-9 px-4 rounded-md text-sm font-medium text-muted-foreground hover:text-foreground hover:bg-accent transition-all"
          >
            Cancel
          </button>
          <button
            onClick={() => onUseTemplate(template)}
            className="h-9 px-5 rounded-md text-sm font-medium bg-foreground text-background hover:bg-foreground/90 transition-all shadow-sm active:scale-[0.98]"
          >
            Use This Template
          </button>
        </div>
      </div>
    </div>
  );
}

export default function Templates() {
  const navigate = useNavigate();
  const [selectedCategory, setSelectedCategory] = useState('All');
  const [searchQuery, setSearchQuery] = useState('');
  const [previewTemplate, setPreviewTemplate] = useState(null);

  const filteredTemplates = workflowTemplates.filter((template) => {
    const matchesCategory = selectedCategory === 'All' || template.category === selectedCategory;
    const matchesSearch =
      searchQuery === '' ||
      template.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      template.description.toLowerCase().includes(searchQuery.toLowerCase()) ||
      template.tags.some((tag) => tag.toLowerCase().includes(searchQuery.toLowerCase()));
    return matchesCategory && matchesSearch;
  });

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
    <div className="space-y-6 animate-fade-in pb-10">
      <div className="px-4 sm:px-6 space-y-6">
        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <h1 className="text-2xl font-semibold text-foreground tracking-tight">Workflow Templates</h1>
              <p className="text-sm text-muted-foreground mt-1">Pre-configured workflows for common software development tasks</p>
            </div>
            
            <div className="relative w-full sm:w-64">
              <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
              <input
                type="text"
                placeholder="Search templates..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="h-10 w-full pl-9 pr-4 rounded-lg border border-border bg-background text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring/20 hover:border-border/80 transition-all"
              />
            </div>
          </div>

          {/* Category Filter Tabs */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-border/50 pb-4">
            <div className="flex items-center gap-1 p-1 rounded-lg bg-muted/50 overflow-x-auto no-scrollbar max-w-full">
              {templateCategories.map((category) => (
                <button
                  key={category}
                  onClick={() => setSelectedCategory(category)}
                  className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium whitespace-nowrap transition-all duration-200 ${
                    selectedCategory === category
                      ? 'bg-background text-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground hover:bg-background/50'
                  }`}
                >
                  {category}
                </button>
              ))}
            </div>
            {selectedCategory !== 'All' && (
              <div className="hidden sm:block text-xs text-muted-foreground whitespace-nowrap px-1">
                Showing {filteredTemplates.length} {selectedCategory} template{filteredTemplates.length !== 1 ? 's' : ''}
              </div>
            )}
          </div>
        </div>

        {/* Template Grid */}
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {filteredTemplates.map((template) => (
          <div 
            key={template.id}
            onClick={() => useTemplate(template)}
            className="group relative flex flex-col rounded-xl border border-border bg-card cursor-pointer transition-all duration-200 hover:border-primary/50 hover:shadow-[0_8px_30px_rgb(0,0,0,0.04)] dark:hover:shadow-[0_8px_30px_rgb(0,0,0,0.2)] overflow-hidden"
          >
            {/* Discrete Preview Action */}
            <button
              onClick={(e) => {
                e.stopPropagation();
                setPreviewTemplate(template);
              }}
              className="absolute top-3 right-3 z-10 p-2 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent opacity-0 group-hover:opacity-100 transition-all duration-200"
              title="Preview template"
            >
              <Globe className="w-4 h-4" />
            </button>

            <div className="flex-1 p-5">
              <div className="flex items-start gap-4 mb-4">
                <div className="p-2.5 rounded-lg bg-muted/50 text-muted-foreground group-hover:text-primary group-hover:bg-primary/5 transition-colors duration-200">
                  <TemplateIcon name={template.icon} className="w-5 h-5" />
                </div>
                <div className="flex-1 min-w-0 pr-6">
                  <h3 className="text-sm font-semibold text-foreground leading-snug line-clamp-2 group-hover:text-primary transition-colors">
                    {template.name}
                  </h3>
                  <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/50 mt-1">
                    {template.category}
                  </p>
                </div>
              </div>

              <p className="text-xs text-muted-foreground line-clamp-3 leading-relaxed mb-6">
                {template.description}
              </p>

              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Badge 
                    variant={template.executionStrategy === 'multi-agent-consensus' ? 'default' : 'outline'}
                    className="text-[9px] px-1.5 py-0 h-4 uppercase font-bold tracking-tighter"
                  >
                    {template.executionStrategy === 'multi-agent-consensus' ? 'Multi' : 'Single'}
                  </Badge>
                  <div className="flex gap-1">
                    {template.tags.slice(0, 1).map((tag) => (
                      <span key={tag} className="text-[10px] text-muted-foreground/60 bg-muted/30 px-1.5 py-0.5 rounded">
                        #{tag}
                      </span>
                    ))}
                  </div>
                </div>
                
                <div className="flex items-center gap-1 text-[11px] font-medium text-primary opacity-0 -translate-x-2 group-hover:opacity-100 group-hover:translate-x-0 transition-all duration-300">
                  <span>Use</span>
                  <Zap className="w-3 h-3 fill-current" />
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* No Results */}
      {filteredTemplates.length === 0 && (
        <div className="flex flex-col items-center justify-center py-24 rounded-2xl border border-dashed border-border bg-muted/5 animate-fade-in">
          <div className="w-16 h-16 rounded-2xl bg-muted flex items-center justify-center mb-4">
            <Search className="w-8 h-8 text-muted-foreground/50" />
          </div>
          <h3 className="text-lg font-semibold text-foreground mb-2">No templates found</h3>
          <p className="text-sm text-muted-foreground text-center max-w-xs">
            We couldn't find any templates matching "{searchQuery}". Try adjusting your filters or search terms.
          </p>
          <button 
            onClick={() => { setSearchQuery(''); setSelectedCategory('All'); }}
            className="mt-6 text-sm font-medium text-primary hover:underline"
          >
            Clear all filters
          </button>
        </div>
      )}

      {/* Stats Footer */}
      {filteredTemplates.length > 0 && (
        <div className="border-t border-border/50 pt-8 mt-4 text-center">
          <p className="text-xs text-muted-foreground font-medium uppercase tracking-widest">
            Showing <span className="text-foreground">{filteredTemplates.length}</span> of{' '}
            <span className="text-foreground">{workflowTemplates.length}</span> workflow templates
          </p>
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
  </div>
);
}
