import { Paperclip, X, Folder, File, ChevronRight, ChevronDown, Search, Loader2, Upload } from 'lucide-react';
import { useState, useRef, useEffect, useCallback } from 'react';
import { fileApi } from '../../lib/api';

function FileTreeNode({ item, selectedPaths, onToggle, level = 0 }) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [children, setChildren] = useState(null);
  const [loading, setLoading] = useState(false);

  const isSelected = selectedPaths.includes(item.path);
  const isDirectory = item.is_dir;

  const handleExpand = async (e) => {
    e.stopPropagation();
    if (isDirectory) {
      if (!isExpanded && !children) {
        setLoading(true);
        try {
          const result = await fileApi.list(item.path);
          setChildren(Array.isArray(result) ? result : []);
        } catch {
          setChildren([]);
        }
        setLoading(false);
      }
      setIsExpanded(!isExpanded);
    }
  };

  const handleSelect = (e) => {
    e.stopPropagation();
    if (!isDirectory) {
      onToggle(item.path);
    }
  };

  return (
    <div>
      <div
        className={`flex items-center gap-2 py-1.5 px-2 rounded-md cursor-pointer transition-colors ${
          isSelected ? 'bg-primary/10 text-primary' : 'hover:bg-accent'
        }`}
        style={{ paddingLeft: `${level * 16 + 8}px` }}
      >
        <button
          type="button"
          onClick={isDirectory ? handleExpand : handleSelect}
          className="flex items-center gap-2 flex-1 min-w-0 text-left bg-transparent border-0 p-0 appearance-none"
        >
          {isDirectory ? (
            <span className="p-0.5">
              {loading ? (
                <Loader2 className="w-3 h-3 animate-spin text-muted-foreground" />
              ) : isExpanded ? (
                <ChevronDown className="w-3 h-3 text-muted-foreground" />
              ) : (
                <ChevronRight className="w-3 h-3 text-muted-foreground" />
              )}
            </span>
          ) : (
            <span className="w-4" />
          )}
          {isDirectory ? (
            <Folder className="w-4 h-4 text-muted-foreground" />
          ) : (
            <File className={`w-4 h-4 ${isSelected ? 'text-primary' : 'text-muted-foreground'}`} />
          )}
          <span className="text-sm truncate flex-1">{item.name}</span>
        </button>
        {!isDirectory && (
          <input
            type="checkbox"
            checked={isSelected}
            onChange={() => onToggle(item.path)}
            className="w-4 h-4 rounded border-border"
          />
        )}
      </div>
      {isExpanded && children && (
        <div>
          {children.map((child) => (
            <FileTreeNode
              key={child.path}
              item={child}
              selectedPaths={selectedPaths}
              onToggle={onToggle}
              level={level + 1}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export default function AttachmentPicker({ attachments, onAdd, onRemove, onClear, onUpload }) {
  const [isOpen, setIsOpen] = useState(false);
  const [files, setFiles] = useState([]);
  const [loading, setLoading] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [search, setSearch] = useState('');
  const ref = useRef(null);
  const fileInputRef = useRef(null);

  useEffect(() => {
    const handleClickOutside = (event) => {
      if (ref.current && !ref.current.contains(event.target)) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const loadFiles = useCallback(async () => {
    setLoading(true);
    try {
      const result = await fileApi.list('');
      setFiles(Array.isArray(result) ? result : []);
    } catch {
      setFiles([]);
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    if (isOpen && files.length === 0) {
      loadFiles();
    }
  }, [isOpen, files.length, loadFiles]);

  const handleToggle = (path) => {
    if (attachments.includes(path)) {
      onRemove(path);
    } else {
      onAdd(path);
    }
  };

  const filteredFiles = search
    ? files.filter(f => f.name.toLowerCase().includes(search.toLowerCase()))
    : files;

  const handleUploadClick = () => fileInputRef.current?.click();

  const handleUploadSelected = async (e) => {
    const selected = Array.from(e.target.files || []);
    e.target.value = '';
    if (selected.length === 0) return;
    if (!onUpload) return;

    setUploading(true);
    try {
      await onUpload(selected);
    } finally {
      setUploading(false);
    }
  };

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className={`flex items-center gap-2 px-3 py-1.5 rounded-lg border transition-colors ${
          attachments.length > 0
            ? 'border-primary bg-primary/10 text-primary'
            : 'border-border bg-background hover:bg-accent text-sm'
        }`}
      >
        <Paperclip className="w-4 h-4" />
        {attachments.length > 0 ? (
          <span className="font-medium">{attachments.length} files</span>
        ) : (
          <span className="font-medium text-muted-foreground">Attach</span>
        )}
      </button>

      {isOpen && (
        <div className="absolute bottom-full right-0 mb-1 z-50 w-80 max-h-96 rounded-lg border border-border bg-popover shadow-lg animate-fade-in flex flex-col">
          <div className="p-2 border-b border-border">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
              <input
                type="text"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Search files..."
                className="w-full h-8 pl-9 pr-3 text-sm rounded-md border border-input bg-background focus:outline-none focus:ring-2 focus:ring-ring"
              />
            </div>

            {onUpload && (
              <div className="mt-2 flex items-center justify-between gap-2">
                <input
                  ref={fileInputRef}
                  type="file"
                  multiple
                  className="hidden"
                  onChange={handleUploadSelected}
                />
                <button
                  type="button"
                  onClick={handleUploadClick}
                  disabled={uploading}
                  className="inline-flex items-center gap-2 px-2.5 py-1.5 rounded-md border border-border bg-background hover:bg-accent text-xs font-medium disabled:opacity-50"
                >
                  {uploading ? (
                    <Loader2 className="w-3.5 h-3.5 animate-spin text-muted-foreground" />
                  ) : (
                    <Upload className="w-3.5 h-3.5 text-muted-foreground" />
                  )}
                  Upload
                </button>
                <span className="text-[11px] text-muted-foreground">
                  Stored in <span className="font-mono">.quorum/attachments</span>
                </span>
              </div>
            )}
          </div>

          <div className="flex-1 overflow-y-auto p-1">
            {loading ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
              </div>
            ) : filteredFiles.length > 0 ? (
              filteredFiles.map((item) => (
                <FileTreeNode
                  key={item.path}
                  item={item}
                  selectedPaths={attachments}
                  onToggle={handleToggle}
                />
              ))
            ) : (
              <p className="text-center py-8 text-sm text-muted-foreground">No files found</p>
            )}
          </div>

          {attachments.length > 0 && (
            <div className="p-2 border-t border-border">
              <div className="flex items-center justify-between">
                <span className="text-xs text-muted-foreground">{attachments.length} selected</span>
                <button
                  type="button"
                  onClick={onClear}
                  className="text-xs text-destructive hover:underline"
                >
                  Clear all
                </button>
              </div>
              <div className="mt-2 flex flex-wrap gap-1">
                {attachments.slice(0, 3).map((path) => (
                  <span
                    key={path}
                    className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-primary/10 text-primary text-xs"
                  >
                    {path.split('/').pop()}
                    <X
                      className="w-3 h-3 cursor-pointer hover:text-destructive"
                      onClick={() => onRemove(path)}
                    />
                  </span>
                ))}
                {attachments.length > 3 && (
                  <span className="text-xs text-muted-foreground">+{attachments.length - 3} more</span>
                )}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
