import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { workflowTemplates, templateCategories } from '../data/workflowTemplates';
import { Badge } from '../components/ui/Badge';
import { Input } from '../components/ui/Input';
import { Search, Sparkles, X, Network, Zap } from 'lucide-react';

function TemplatePreviewModal({ template, onClose, onUseTemplate }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm animate-fade-in">
      <div className="bg-gradient-to-b from-card to-card/95 border border-border/50 rounded-2xl shadow-2xl max-w-3xl w-full max-h-[85vh] overflow-hidden flex flex-col">
        {/* Header */}
        <div className="flex items-start justify-between p-6 border-b border-border/30 bg-gradient-to-r from-transparent via-muted/5 to-transparent">
          <div className="flex items-center gap-4 flex-1">
            <span className="text-4xl">{template.icon}</span>
            <div>
              <h2 className="text-xl font-bold text-foreground">{template.name}</h2>
              <p className="text-sm text-muted-foreground mt-1.5 leading-relaxed">{template.description}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-2 rounded-lg hover:bg-accent text-muted-foreground hover:text-foreground transition-colors"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-6 space-y-4">
          <div>
            <h3 className="text-sm font-semibold text-foreground mb-2">Category</h3>
            <Badge variant="secondary">{template.category}</Badge>
          </div>

          <div>
            <h3 className="text-sm font-semibold text-foreground mb-2">Execution Strategy</h3>
            <Badge variant={template.executionStrategy === 'multi-agent-consensus' ? 'default' : 'outline'}>
              {template.executionStrategy === 'multi-agent-consensus' ? (
                <><Network className="w-3 h-3 mr-1" /> Multi-Agent Consensus</>
              ) : (
                <><Zap className="w-3 h-3 mr-1" /> Single Agent</>
              )}
            </Badge>
          </div>

          <div>
            <h3 className="text-sm font-semibold text-foreground mb-2">Tags</h3>
            <div className="flex flex-wrap gap-2">
              {template.tags.map((tag) => (
                <Badge key={tag} variant="outline" className="text-xs">
                  {tag}
                </Badge>
              ))}
            </div>
          </div>

          <div>
            <h3 className="text-sm font-semibold text-foreground mb-3">Prompt Template</h3>
            <div className="bg-gradient-to-br from-muted/50 to-muted/30 backdrop-blur-sm rounded-xl p-4 text-sm font-mono whitespace-pre-wrap text-muted-foreground max-h-96 overflow-y-auto border border-border/30">
              {template.prompt}
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="flex justify-end gap-3 p-6 border-t border-border/30 bg-gradient-to-r from-transparent via-muted/5 to-transparent">
          <button
            onClick={onClose}
            className="h-9 px-4 rounded-lg text-sm font-medium border border-border bg-background/50 hover:bg-accent hover:text-foreground transition-colors"
          >
            Close
          </button>
          <button
            onClick={() => onUseTemplate(template)}
            className="h-9 px-4 rounded-lg text-sm font-medium bg-primary/90 text-primary-foreground hover:bg-primary transition-colors shadow-sm"
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
    <div className="space-y-6 p-6 animate-fade-in">
      {/* Header - Enhanced */}
      <div className="space-y-3">
        <div className="flex items-center gap-3">
          <div className="p-3 rounded-xl bg-gradient-to-br from-primary/10 to-primary/5 border border-primary/20">
            <Sparkles className="h-7 w-7 text-primary" />
          </div>
          <div>
            <h1 className="text-3xl font-bold text-foreground tracking-tight">Workflow Templates</h1>
            <p className="text-sm text-muted-foreground mt-1 leading-relaxed">
              Pre-configured workflows for common software development tasks. Select a template to get started quickly.
            </p>
          </div>
        </div>
      </div>

      {/* Search Bar */}
      <div className="relative max-w-md">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder="Search templates..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="pl-10 h-9 rounded-lg border-border/50 bg-background/50 backdrop-blur-sm focus:ring-2 focus:ring-primary/20"
        />
      </div>

      {/* Category Filters */}
      <div className="flex gap-2 overflow-x-auto pb-2 scrollbar-thin">
        {templateCategories.map((category) => (
          <button
            key={category}
            onClick={() => setSelectedCategory(category)}
            className={`h-9 px-4 rounded-lg text-sm font-medium whitespace-nowrap transition-all ${
              selectedCategory === category 
                ? 'bg-primary/10 text-primary border border-primary/20' 
                : 'bg-background/50 text-muted-foreground border border-border hover:bg-accent hover:text-foreground'
            }`}
          >
            {category}
          </button>
        ))}
      </div>

      {/* Template Grid - Unified Style */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {filteredTemplates.map((template) => (
          <div 
            key={template.id} 
            className="group relative flex flex-col rounded-xl border border-border/50 bg-gradient-to-br from-card via-card to-card backdrop-blur-sm shadow-sm transition-all duration-300 hover:shadow-lg hover:border-primary/30 hover:-translate-y-0.5 overflow-hidden"
          >
            {/* Top accent */}
            <div className="absolute top-0 left-4 right-4 h-0.5 bg-gradient-to-r from-transparent via-primary to-transparent" />

            {/* Header */}
            <div className="flex-1 p-5">
              <div className="flex items-start gap-3 mb-3">
                <span className="text-3xl">{template.icon}</span>
                <div className="flex-1 min-w-0">
                  <h3 className="text-base font-semibold text-foreground leading-snug line-clamp-2 group-hover:text-primary transition-colors">
                    {template.name}
                  </h3>
                  <p className="text-sm text-muted-foreground mt-1.5 line-clamp-3 leading-relaxed">
                    {template.description}
                  </p>
                </div>
              </div>

              {/* Tags */}
              <div className="flex flex-wrap gap-1.5 mb-3">
                {template.tags.slice(0, 3).map((tag) => (
                  <Badge key={tag} variant="secondary" className="text-xs">
                    {tag}
                  </Badge>
                ))}
                {template.tags.length > 3 && (
                  <Badge variant="secondary" className="text-xs">
                    +{template.tags.length - 3}
                  </Badge>
                )}
              </div>

              {/* Execution Strategy */}
              <Badge 
                variant={template.executionStrategy === 'multi-agent-consensus' ? 'default' : 'outline'}
                className="inline-flex items-center gap-1.5"
              >
                {template.executionStrategy === 'multi-agent-consensus' ? (
                  <><Network className="w-3 h-3" /> Multi-Agent</>
                ) : (
                  <><Zap className="w-3 h-3" /> Single-Agent</>
                )}
              </Badge>
            </div>

            {/* Actions */}
            <div className="flex gap-2 p-4 border-t border-border/30 bg-gradient-to-r from-transparent via-muted/5 to-transparent">
              <button
                onClick={() => useTemplate(template)} 
                className="flex-1 h-9 px-4 rounded-lg text-sm font-medium bg-primary/90 text-primary-foreground hover:bg-primary transition-colors"
              >
                Use Template
              </button>
              <button
                onClick={() => setPreviewTemplate(template)}
                className="h-9 px-4 rounded-lg text-sm font-medium border border-border bg-background/50 hover:bg-accent hover:text-foreground transition-colors"
              >
                Preview
              </button>
            </div>
          </div>
        ))}
      </div>

      {/* No Results */}
      {filteredTemplates.length === 0 && (
        <div className="flex flex-col items-center justify-center py-16 rounded-xl border-2 border-dashed border-border/30 bg-muted/5">
          <Sparkles className="w-12 h-12 text-muted-foreground/30 mb-4" />
          <p className="text-muted-foreground text-sm font-medium">No templates found matching your criteria.</p>
        </div>
      )}

      {/* Stats Footer */}
      <div className="border-t border-border/30 pt-4 text-center text-sm text-muted-foreground">
        Showing <span className="font-semibold text-foreground">{filteredTemplates.length}</span> of{' '}
        <span className="font-semibold text-foreground">{workflowTemplates.length}</span> templates
      </div>

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
