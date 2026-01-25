import { useEffect, useState, useCallback } from 'react';
import { useFileStore } from '../stores';
import {
  FolderOpen,
  File,
  Upload,
  Download,
  Trash2,
  Search,
  Grid,
  List,
  FileText,
  FileCode,
  FileImage,
  Loader2,
  X,
} from 'lucide-react';

function getFileIcon(filename) {
  const ext = filename.split('.').pop()?.toLowerCase();
  const imageExts = ['jpg', 'jpeg', 'png', 'gif', 'svg', 'webp'];
  const codeExts = ['js', 'jsx', 'ts', 'tsx', 'py', 'go', 'rs', 'java', 'cpp', 'c', 'h'];

  if (imageExts.includes(ext)) return FileImage;
  if (codeExts.includes(ext)) return FileCode;
  return FileText;
}

function formatFileSize(bytes) {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function FileCard({ file, viewMode, onDownload, onDelete }) {
  const Icon = getFileIcon(file.name);

  if (viewMode === 'grid') {
    return (
      <div className="group p-4 rounded-xl border border-border bg-card hover:border-muted-foreground/30 hover:shadow-md transition-all">
        <div className="flex items-center justify-center w-12 h-12 mx-auto mb-3 rounded-xl bg-muted">
          <Icon className="w-6 h-6 text-muted-foreground" />
        </div>
        <p className="text-sm font-medium text-foreground text-center truncate">{file.name}</p>
        <p className="text-xs text-muted-foreground text-center mt-1">{formatFileSize(file.size)}</p>
        <div className="flex items-center justify-center gap-2 mt-3 opacity-0 group-hover:opacity-100 transition-opacity">
          <button
            onClick={() => onDownload(file)}
            className="p-1.5 rounded-lg hover:bg-accent transition-colors"
            title="Download"
          >
            <Download className="w-4 h-4 text-muted-foreground" />
          </button>
          <button
            onClick={() => onDelete(file)}
            className="p-1.5 rounded-lg hover:bg-destructive/10 transition-colors"
            title="Delete"
          >
            <Trash2 className="w-4 h-4 text-destructive" />
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="group flex items-center gap-4 p-3 rounded-lg border border-border bg-card hover:border-muted-foreground/30 transition-all">
      <div className="p-2 rounded-lg bg-muted">
        <Icon className="w-5 h-5 text-muted-foreground" />
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-foreground truncate">{file.name}</p>
        <p className="text-xs text-muted-foreground">
          {formatFileSize(file.size)} · {new Date(file.updated_at).toLocaleDateString()}
        </p>
      </div>
      <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
        <button
          onClick={() => onDownload(file)}
          className="p-2 rounded-lg hover:bg-accent transition-colors"
          title="Download"
        >
          <Download className="w-4 h-4 text-muted-foreground" />
        </button>
        <button
          onClick={() => onDelete(file)}
          className="p-2 rounded-lg hover:bg-destructive/10 transition-colors"
          title="Delete"
        >
          <Trash2 className="w-4 h-4 text-destructive" />
        </button>
      </div>
    </div>
  );
}

function UploadZone({ onUpload, uploading }) {
  const [dragOver, setDragOver] = useState(false);

  const handleDrop = useCallback((e) => {
    e.preventDefault();
    setDragOver(false);
    const files = Array.from(e.dataTransfer.files);
    if (files.length > 0) onUpload(files);
  }, [onUpload]);

  const handleDragOver = useCallback((e) => {
    e.preventDefault();
    setDragOver(true);
  }, []);

  const handleDragLeave = useCallback(() => {
    setDragOver(false);
  }, []);

  const handleFileSelect = (e) => {
    const files = Array.from(e.target.files);
    if (files.length > 0) onUpload(files);
  };

  return (
    <div
      onDrop={handleDrop}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      className={`relative p-8 rounded-xl border-2 border-dashed transition-all ${
        dragOver
          ? 'border-primary bg-primary/5'
          : 'border-border hover:border-muted-foreground/50'
      }`}
    >
      <input
        type="file"
        multiple
        onChange={handleFileSelect}
        className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
        disabled={uploading}
      />
      <div className="text-center">
        {uploading ? (
          <Loader2 className="w-8 h-8 mx-auto mb-3 text-primary animate-spin" />
        ) : (
          <Upload className="w-8 h-8 mx-auto mb-3 text-muted-foreground" />
        )}
        <p className="text-sm font-medium text-foreground">
          {uploading ? 'Uploading...' : 'Drop files here or click to upload'}
        </p>
        <p className="text-xs text-muted-foreground mt-1">
          Supports any file type up to 50MB
        </p>
      </div>
    </div>
  );
}

export default function Files() {
  const { files, loading, uploading, fetchFiles, uploadFiles, downloadFile, deleteFile } = useFileStore();
  const [viewMode, setViewMode] = useState('list');
  const [search, setSearch] = useState('');

  useEffect(() => {
    fetchFiles();
  }, [fetchFiles]);

  const filteredFiles = files.filter(f =>
    f.name.toLowerCase().includes(search.toLowerCase())
  );

  const handleUpload = async (fileList) => {
    await uploadFiles(fileList);
  };

  const handleDelete = async (file) => {
    if (window.confirm(`Delete "${file.name}"?`)) {
      await deleteFile(file.id);
    }
  };

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-foreground tracking-tight">Files</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Manage your uploaded files and documents
          </p>
        </div>
      </div>

      {/* Upload Zone */}
      <UploadZone onUpload={handleUpload} uploading={uploading} />

      {/* Toolbar */}
      <div className="flex items-center justify-between gap-4">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search files..."
            className="w-full h-10 pl-10 pr-4 border border-input rounded-lg bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background"
          />
          {search && (
            <button
              onClick={() => setSearch('')}
              className="absolute right-3 top-1/2 -translate-y-1/2 p-1 hover:bg-accent rounded"
            >
              <X className="w-3 h-3 text-muted-foreground" />
            </button>
          )}
        </div>
        <div className="flex items-center gap-1 p-1 rounded-lg bg-secondary">
          <button
            onClick={() => setViewMode('list')}
            className={`p-2 rounded-md transition-all ${
              viewMode === 'list' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
            }`}
            title="List view"
          >
            <List className="w-4 h-4" />
          </button>
          <button
            onClick={() => setViewMode('grid')}
            className={`p-2 rounded-md transition-all ${
              viewMode === 'grid' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
            }`}
            title="Grid view"
          >
            <Grid className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Files */}
      {loading ? (
        <div className={viewMode === 'grid' ? 'grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4' : 'space-y-2'}>
          {[...Array(6)].map((_, i) => (
            <div key={i} className={`${viewMode === 'grid' ? 'h-32' : 'h-16'} rounded-xl bg-muted animate-pulse`} />
          ))}
        </div>
      ) : filteredFiles.length > 0 ? (
        <div className={viewMode === 'grid' ? 'grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4' : 'space-y-2'}>
          {filteredFiles.map((file) => (
            <FileCard
              key={file.id}
              file={file}
              viewMode={viewMode}
              onDownload={downloadFile}
              onDelete={handleDelete}
            />
          ))}
        </div>
      ) : (
        <div className="text-center py-16">
          <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-muted flex items-center justify-center">
            <FolderOpen className="w-8 h-8 text-muted-foreground" />
          </div>
          <h3 className="text-lg font-semibold text-foreground mb-2">
            {search ? 'No files found' : 'No files yet'}
          </h3>
          <p className="text-sm text-muted-foreground">
            {search ? 'Try a different search term' : 'Upload files to get started'}
          </p>
        </div>
      )}
    </div>
  );
}
