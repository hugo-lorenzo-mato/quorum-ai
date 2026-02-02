import { useState, useMemo, createElement } from 'react';
import { ChevronRight, ChevronDown, Folder, FileText, File } from 'lucide-react';

const getIconForFile = (filename) => {
  if (filename.endsWith('.md')) return FileText;
  if (filename.endsWith('.js') || filename.endsWith('.jsx') || filename.endsWith('.ts') || filename.endsWith('.tsx')) return File; // Generic for now, can be specific
  return File;
};

const FileTreeNode = ({ node, level = 0, onSelect, selectedKey }) => {
  const [expanded, setExpanded] = useState(level < 1); // Expand root levels by default
  const isFolder = node.type === 'folder';
  const IconComponent = isFolder ? Folder : getIconForFile(node.name);
  const isSelected = !isFolder && selectedKey === node.key;

  const handleClick = (e) => {
    e.stopPropagation();
    if (isFolder) {
      setExpanded(!expanded);
    } else {
      onSelect(node);
    }
  };

  return (
    <div className="select-none">
      <div
        onClick={handleClick}
        className={`flex items-center gap-1.5 py-1.5 px-2 rounded-lg cursor-pointer transition-colors ${
          isSelected
            ? 'bg-primary/10 text-primary font-medium'
            : 'text-muted-foreground hover:bg-accent/50 hover:text-foreground'
        }`}
        style={{ paddingLeft: `${level * 12 + 8}px` }}
      >
        <span className={`shrink-0 ${isFolder ? 'text-muted-foreground' : isSelected ? 'text-primary' : 'text-muted-foreground'}`}>
          {isFolder ? (
            expanded ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />
          ) : (
            createElement(IconComponent, { className: "w-4 h-4" })
          )}
        </span>
        <span className="truncate text-sm">{node.name}</span>
      </div>
      {isFolder && expanded && node.children && (
        <div>
          {node.children.map((child) => (
            <FileTreeNode
              key={child.id}
              node={child}
              level={level + 1}
              onSelect={onSelect}
              selectedKey={selectedKey}
            />
          ))}
        </div>
      )}
    </div>
  );
};

export default function FileTree({ items, onSelect, selectedKey }) {
  // Convert flat list of items (with paths) to tree structure
  // Uses treePath for visual structure if available, otherwise falls back to path or title
  const tree = useMemo(() => {
    const root = { id: 'root', type: 'folder', name: 'root', children: [] };

    items.forEach((item) => {
      const pathParts = (item.treePath || item.path || item.title || '').split('/');
      let currentLevel = root.children;
      
      pathParts.forEach((part, index) => {
        const isFile = index === pathParts.length - 1;
        const existingNode = currentLevel.find(n => n.name === part);
        
        if (existingNode) {
          if (isFile) {
            // Merge file info
            Object.assign(existingNode, { ...item, type: 'file', name: part, key: item.key || item.path });
          } else {
            currentLevel = existingNode.children;
          }
        } else {
          const newNode = {
            id: `${item.key || item.path}-${index}`,
            name: part,
            type: isFile ? 'file' : 'folder',
            children: [],
            ...(isFile ? item : {})
          };
          currentLevel.push(newNode);
          if (!isFile) {
            currentLevel = newNode.children;
          }
        }
      });
    });
    
    // Sort: Folders first, then files
    const sortNodes = (nodes) => {
      nodes.sort((a, b) => {
        if (a.type === b.type) return a.name.localeCompare(b.name);
        return a.type === 'folder' ? -1 : 1;
      });
      nodes.forEach(n => {
        if (n.children) sortNodes(n.children);
      });
    };
    
    sortNodes(root.children);
    return root.children;
  }, [items]);

  return (
    <div className="overflow-y-auto">
      {tree.map((node) => (
        <FileTreeNode
          key={node.id}
          node={node}
          onSelect={onSelect}
          selectedKey={selectedKey}
        />
      ))}
      {tree.length === 0 && (
        <div className="p-4 text-center text-xs text-muted-foreground">
          No files
        </div>
      )}
    </div>
  );
}
