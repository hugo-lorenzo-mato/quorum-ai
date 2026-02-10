import { useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { promptPresets, promptCategories } from '../data/promptPresets';
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

function PresetIcon({ name, className = "" }) {
  const Icon = ICON_MAP[name] || Sparkles;
  return <Icon className={className} />;
}

function PresetPreviewModal({ preset, onClose, onUsePreset }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 animate-fade-in">
      <button
        type="button"
        className="absolute inset-0 bg-background/80 backdrop-blur-sm"
        onClick={onClose}
        aria-label="Close preset preview"
      />
      <div
        className="relative bg-card border border-border shadow-2xl max-w-2xl w-full max-h-[90vh] overflow-hidden flex flex-col rounded-2xl animate-fade-up"
      >
        {/* Header */}
        <div className="relative flex items-start justify-between p-6 border-b border-border/50 bg-muted/5">
          <div className="absolute top-0 left-6 right-6 h-0.5 rounded-full bg-gradient-to-r from-transparent via-primary to-transparent" />
          <div className="flex items-center gap-5">
            <div className="p-3 rounded-2xl bg-primary/10 border border-primary/20 text-primary">
              <PresetIcon name={preset.icon} className="w-8 h-8" />
            </div>
            <div className="min-w-0">
              <h2 className="text-xl font-bold text-foreground tracking-tight">{preset.name}</h2>
              <div className="flex items-center gap-2 mt-1.5">
                <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70 bg-muted/50 px-2 py-0.5 rounded border border-border/50">
                  {preset.category}
                </span>
                <Badge variant={preset.executionStrategy === 'multi-agent-consensus' ? 'default' : 'outline'} className="text-[10px] h-5">
                  {preset.executionStrategy === 'multi-agent-consensus' ? 'Multi-Agent' : 'Single-Agent'}
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
              {preset.description}
            </p>
          </div>

          <div>
            <h3 className="text-xs font-bold uppercase tracking-widest text-muted-foreground mb-3">Tags</h3>
            <div className="flex flex-wrap gap-2">
              {preset.tags.map((tag) => (
                <Badge key={tag} variant="secondary" className="text-xs font-medium px-2.5">
                  {tag}
                </Badge>
              ))}
            </div>
          </div>

          <div>
            <h3 className="text-xs font-bold uppercase tracking-widest text-muted-foreground mb-3">Prompt</h3>
            <div className="relative group">
              <div className="absolute -inset-0.5 bg-gradient-to-r from-primary/10 to-transparent rounded-xl blur opacity-20 group-hover:opacity-40 transition-opacity" />
              <div className="relative bg-muted/30 backdrop-blur-sm rounded-xl p-5 text-sm font-mono whitespace-pre-wrap text-foreground/80 max-h-80 overflow-y-auto border border-border/50 shadow-inner leading-relaxed">
                {preset.prompt}
              </div>
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="flex justify-end items-center p-6 border-t border-border/50 bg-muted/5">
          <button
            onClick={() => onUsePreset(preset)}
            className="h-8 px-5 rounded-md text-xs font-bold bg-primary text-primary-foreground hover:bg-primary/90 hover:shadow-md hover:-translate-y-0.5 hover:scale-[1.02] transition-all shadow-sm active:scale-[0.97]"
          >
            Use This Prompt
          </button>
        </div>
      </div>
    </div>
  );
}

export default function Prompts() {
  const navigate = useNavigate();
  const [selectedCategory, setSelectedCategory] = useState('All');
  const [searchQuery, setSearchQuery] = useState('');
  const [previewPreset, setPreviewPreset] = useState(null);

  // Calculate counts per category
  const categoryCounts = useMemo(() => {
    const counts = { All: promptPresets.length };
    promptPresets.forEach(t => {
      counts[t.category] = (counts[t.category] || 0) + 1;
    });
    return counts;
  }, []);

  const filteredPresets = promptPresets.filter((preset) => {
    const matchesCategory = selectedCategory === 'All' || preset.category === selectedCategory;
    const matchesSearch =
      searchQuery === '' ||
      preset.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      preset.description.toLowerCase().includes(searchQuery.toLowerCase()) ||
      preset.tags.some((tag) => tag.toLowerCase().includes(searchQuery.toLowerCase()));
    return matchesCategory && matchesSearch;
  });

  const handleUsePreset = (preset) => {
    navigate('/workflows/new', {
      state: {
        promptPreset: {
          prompt: preset.prompt,
          executionStrategy: preset.executionStrategy,
          name: preset.name
        }
      }
    });
  };

  return (
    <div className="space-y-6 animate-fade-in pb-10">
      <div className="px-4 sm:px-6 space-y-8">
        <div className="flex flex-col gap-6">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <h1 className="text-2xl font-semibold text-foreground tracking-tight">Prompt Presets</h1>
              <p className="text-sm text-muted-foreground mt-1">Select a pre-configured prompt to accelerate your workflow</p>
            </div>
            
            <div className="flex flex-col sm:flex-row gap-3 w-full lg:w-auto">
              <button
                type="button"
                onClick={() => navigate('/system-prompts')}
                className="h-9 px-3 rounded-md border border-border bg-background text-sm font-medium text-foreground hover:bg-accent transition-all flex items-center justify-center gap-2"
              >
                <FileCode2 className="w-4 h-4" />
                System Prompts
              </button>

              <div className="relative w-full sm:w-64">
                <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                <input
                  type="text"
                  placeholder="Search prompts..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="h-9 w-full pl-9 pr-4 rounded-md border border-border bg-background text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring/10 hover:border-border/80 transition-all"
                />
              </div>
            </div>
          </div>

          {/* Segmented/Tabs Filters with Counts */}
          <div className="flex items-center gap-1 p-1 rounded-lg bg-muted/30 overflow-x-auto no-scrollbar max-w-full border border-border/50">
            {promptCategories.map((category) => (
              <button
                key={category}
                onClick={() => setSelectedCategory(category)}
                className={`flex items-center gap-2 px-3 py-1.5 rounded-md text-xs font-medium whitespace-nowrap transition-all duration-200 ${
                  selectedCategory === category
                    ? 'bg-background text-foreground shadow-sm'
                    : 'text-muted-foreground hover:text-foreground hover:bg-background/50'
                }`}
              >
                {category}
                <span className={`text-[10px] px-1.5 py-0.5 rounded-full ${
                  selectedCategory === category ? 'bg-muted text-foreground' : 'bg-muted/50 text-muted-foreground'
                }`}>
                  {categoryCounts[category] || 0}
                </span>
              </button>
            ))}
          </div>
        </div>

        {/* Preset Grid */}
        <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
        {filteredPresets.map((preset) => (
          <button
            key={preset.id}
            type="button"
            onClick={() => setPreviewPreset(preset)}
            className="group flex flex-col text-left rounded-xl border border-border bg-card transition-all duration-200 overflow-hidden shadow-sm hover:shadow-md hover:border-foreground/30 cursor-pointer"
          >
            <div className="flex-1 p-4">
              {/* Header: Icon, Name, Tags */}
              <div className="flex items-start justify-between gap-3 mb-1.5">
                <div className="flex items-center gap-2.5 min-w-0 flex-1">
                  <div className="flex-shrink-0 p-1 rounded-lg bg-muted/50 text-muted-foreground border border-transparent group-hover:text-primary group-hover:bg-primary/10 group-hover:border-primary/20 transition-all duration-300">
                    <PresetIcon name={preset.icon} className="w-4 h-4" />
                  </div>
                  <h3 className="text-sm font-bold text-foreground leading-tight truncate group-hover:text-primary transition-colors">
                    {preset.name}
                  </h3>
                </div>
                
                <div className="flex gap-1 flex-shrink-0 flex-wrap justify-end max-w-[40%]">
                  {preset.tags.slice(0, 2).map((tag) => (
                    <span 
                      key={tag} 
                      className="text-[9px] px-1.5 py-0.5 font-medium lowercase bg-muted/50 text-muted-foreground/80 rounded border border-transparent group-hover:border-primary/20 group-hover:text-primary group-hover:bg-primary/5 transition-all duration-300"
                    >
                      {tag}
                    </span>
                  ))}
                  {preset.tags.length > 2 && (
                    <span className="text-[9px] text-muted-foreground/40 self-center">+{preset.tags.length - 2}</span>
                  )}
                </div>
              </div>

              {/* Description */}
              <div className="pl-[34px]">
                <p className="text-xs text-muted-foreground/80 line-clamp-2 leading-relaxed h-10">
                  {preset.description}
                </p>
              </div>
            </div>

            {/* Compact Actions Footer */}
            <div className="flex items-center justify-between px-3 py-2 border-t border-border/40 bg-muted/5 mt-auto h-10">
              <div 
                className="flex items-center justify-center gap-1.5 text-[10px] text-muted-foreground/60 group-hover:text-primary transition-colors duration-300"
                title={preset.executionStrategy === 'multi-agent-consensus' ? 'Multi-Agent' : 'Single-Agent'}
              >
                {preset.executionStrategy === 'multi-agent-consensus' ? (
                  <Network className="w-3.5 h-3.5" />
                ) : (
                  <Zap className="w-3.5 h-3.5" />
                )}
                <span className="hidden sm:inline font-medium">
                  {preset.executionStrategy === 'multi-agent-consensus' ? 'Multi-Agent' : 'Single-Agent'}
                </span>
              </div>

              <button
                onClick={(e) => {
                  e.stopPropagation();
                  handleUsePreset(preset);
                }}
                className="h-6 px-2.5 rounded text-[10px] font-bold bg-background/50 border border-border/60 text-foreground/70 hover:border-primary hover:text-primary hover:bg-primary/5 hover:-translate-y-0.5 transition-all active:scale-[0.97] shadow-sm flex items-center justify-center"
              >
                Use Prompt
              </button>
            </div>
          </button>
        ))}
      </div>

      {/* No Results */}
      {filteredPresets.length === 0 && (
        <div className="flex flex-col items-center justify-center py-24 rounded-2xl border border-dashed border-border bg-muted/5 animate-fade-in">
          <div className="w-16 h-16 rounded-2xl bg-muted flex items-center justify-center mb-4">
            <Search className="w-8 h-8 text-muted-foreground/50" />
          </div>
          <h3 className="text-lg font-semibold text-foreground mb-2">No prompts found</h3>
          <p className="text-sm text-muted-foreground text-center max-w-xs">
            We couldn&apos;t find any prompts matching &quot;{searchQuery}&quot;. Try adjusting your filters or search terms.
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
      {filteredPresets.length > 0 && (
        <div className="border-t border-border/50 pt-8 mt-4 text-center">
          <p className="text-xs text-muted-foreground font-medium uppercase tracking-widest">
            Showing <span className="text-foreground">{filteredPresets.length}</span> of{' '}
            <span className="text-foreground">{promptPresets.length}</span> prompt presets
          </p>
        </div>
      )}

      {/* Preview Modal */}
      {previewPreset && (
        <PresetPreviewModal
          preset={previewPreset}
          onClose={() => setPreviewPreset(null)}
          onUsePreset={(preset) => {
            setPreviewPreset(null);
            handleUsePreset(preset);
          }}
        />
      )}
    </div>
  </div>
);
}
