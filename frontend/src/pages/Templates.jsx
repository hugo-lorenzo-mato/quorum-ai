import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { workflowTemplates, templateCategories } from '../data/workflowTemplates';
import { Card, CardHeader, CardTitle, CardDescription } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Badge } from '../components/ui/Badge';
import { Input } from '../components/ui/Input';
import { Search, Sparkles, X } from 'lucide-react';

function TemplatePreviewModal({ template, onClose, onUseTemplate }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm">
      <div className="bg-card border border-border rounded-xl shadow-xl max-w-3xl w-full max-h-[80vh] overflow-hidden flex flex-col">
        {/* Header */}
        <div className="flex items-start justify-between p-6 border-b border-border">
          <div className="flex items-center gap-3 flex-1">
            <span className="text-3xl">{template.icon}</span>
            <div>
              <h2 className="text-xl font-semibold text-foreground">{template.name}</h2>
              <p className="text-sm text-muted-foreground mt-1">{template.description}</p>
            </div>
          </div>
          <Button variant="ghost" size="sm" onClick={onClose}>
            <X className="h-4 w-4" />
          </Button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-6">
          <div className="space-y-4">
            <div>
              <h3 className="text-sm font-medium text-foreground mb-2">Category</h3>
              <Badge variant="secondary">{template.category}</Badge>
            </div>

            <div>
              <h3 className="text-sm font-medium text-foreground mb-2">Execution Strategy</h3>
              <Badge variant={template.executionStrategy === 'multi-agent-consensus' ? 'default' : 'outline'}>
                {template.executionStrategy === 'multi-agent-consensus' ? 'Multi-Agent Consensus' : 'Single Agent'}
              </Badge>
            </div>

            <div>
              <h3 className="text-sm font-medium text-foreground mb-2">Tags</h3>
              <div className="flex flex-wrap gap-2">
                {template.tags.map((tag) => (
                  <Badge key={tag} variant="outline" className="text-xs">
                    {tag}
                  </Badge>
                ))}
              </div>
            </div>

            <div>
              <h3 className="text-sm font-medium text-foreground mb-2">Prompt Template</h3>
              <div className="bg-muted rounded-lg p-4 text-sm font-mono whitespace-pre-wrap text-muted-foreground max-h-96 overflow-y-auto">
                {template.prompt}
              </div>
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="flex justify-end gap-2 p-6 border-t border-border">
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
          <Button onClick={() => onUseTemplate(template)}>
            Use This Template
          </Button>
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
    // Navigate to workflows page with template data in state
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
    <div className="space-y-6 p-6">
      {/* Header */}
      <div className="space-y-2">
        <div className="flex items-center gap-2">
          <Sparkles className="h-8 w-8 text-primary" />
          <h1 className="text-3xl font-bold">Workflow Templates</h1>
        </div>
        <p className="text-muted-foreground">
          Pre-configured workflows for common software development tasks. Select a template to get started quickly.
        </p>
      </div>

      {/* Search and Filters */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search templates..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-9"
          />
        </div>
      </div>

      {/* Category Tabs */}
      <div className="flex gap-2 overflow-x-auto pb-2">
        {templateCategories.map((category) => (
          <Button
            key={category}
            variant={selectedCategory === category ? 'default' : 'outline'}
            size="sm"
            onClick={() => setSelectedCategory(category)}
            className="whitespace-nowrap"
          >
            {category}
          </Button>
        ))}
      </div>

      {/* Template Grid */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {filteredTemplates.map((template) => (
          <Card key={template.id} className="flex flex-col hover:shadow-lg transition-shadow">
            <CardHeader className="flex-1">
              <div className="flex items-start justify-between">
                <div className="space-y-2 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-2xl">{template.icon}</span>
                    <CardTitle className="text-lg">{template.name}</CardTitle>
                  </div>
                  <CardDescription className="text-sm line-clamp-3">{template.description}</CardDescription>
                </div>
              </div>

              {/* Tags */}
              <div className="flex flex-wrap gap-1 pt-2">
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

              {/* Execution Strategy Badge */}
              <div className="pt-2">
                <Badge variant={template.executionStrategy === 'multi-agent-consensus' ? 'default' : 'outline'}>
                  {template.executionStrategy === 'multi-agent-consensus' ? 'Multi-Agent' : 'Single-Agent'}
                </Badge>
              </div>
            </CardHeader>

            {/* Actions */}
            <div className="p-4 pt-0 flex gap-2">
              <Button onClick={() => useTemplate(template)} className="flex-1">
                Use Template
              </Button>
              <Button
                variant="outline"
                onClick={() => setPreviewTemplate(template)}
              >
                Preview
              </Button>
            </div>
          </Card>
        ))}
      </div>

      {/* No Results */}
      {filteredTemplates.length === 0 && (
        <div className="text-center py-12">
          <p className="text-muted-foreground">No templates found matching your criteria.</p>
        </div>
      )}

      {/* Stats Footer */}
      <div className="border-t pt-4 text-center text-sm text-muted-foreground">
        Showing {filteredTemplates.length} of {workflowTemplates.length} templates
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
