import { useEffect, useState, useCallback } from 'react';
import { fileApi } from '../lib/api';

function FileIcon({ isDir, name }) {
  if (isDir) {
    return (
      <svg className="w-5 h-5 text-yellow-500" fill="currentColor" viewBox="0 0 20 20">
        <path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
      </svg>
    );
  }

  const ext = name.split('.').pop()?.toLowerCase();
  const iconColors = {
    js: 'text-yellow-400',
    jsx: 'text-blue-400',
    ts: 'text-blue-600',
    tsx: 'text-blue-500',
    go: 'text-cyan-500',
    py: 'text-green-500',
    md: 'text-gray-500',
    json: 'text-orange-400',
    yaml: 'text-red-400',
    yml: 'text-red-400',
  };

  return (
    <svg className={`w-5 h-5 ${iconColors[ext] || 'text-gray-400'}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
    </svg>
  );
}

function FileListItem({ file, onSelect, isSelected }) {
  const formatSize = (bytes) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  };

  return (
    <button
      onClick={() => onSelect(file)}
      className={`w-full text-left px-4 py-2 flex items-center gap-3 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors ${
        isSelected ? 'bg-blue-50 dark:bg-blue-900/30' : ''
      }`}
    >
      <FileIcon isDir={file.is_dir} name={file.name} />
      <span className="flex-1 text-sm text-gray-900 dark:text-white truncate">{file.name}</span>
      {!file.is_dir && (
        <span className="text-xs text-gray-500 dark:text-gray-400">{formatSize(file.size)}</span>
      )}
    </button>
  );
}

function Breadcrumb({ path, onNavigate }) {
  const parts = path.split('/').filter(Boolean);

  return (
    <div className="flex items-center gap-1 text-sm overflow-x-auto">
      <button
        onClick={() => onNavigate('')}
        className="px-2 py-1 text-gray-600 dark:text-gray-400 hover:text-blue-600 dark:hover:text-blue-400"
      >
        ~
      </button>
      {parts.map((part, index) => {
        const fullPath = parts.slice(0, index + 1).join('/');
        return (
          <div key={fullPath} className="flex items-center">
            <span className="text-gray-400">/</span>
            <button
              onClick={() => onNavigate(fullPath)}
              className="px-2 py-1 text-gray-600 dark:text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 truncate max-w-[150px]"
            >
              {part}
            </button>
          </div>
        );
      })}
    </div>
  );
}

function CodeViewer({ content, language, filename }) {
  return (
    <div className="h-full flex flex-col">
      <div className="flex items-center justify-between px-4 py-2 bg-gray-100 dark:bg-gray-700 border-b border-gray-200 dark:border-gray-600">
        <span className="text-sm font-medium text-gray-900 dark:text-white">{filename}</span>
        <span className="text-xs px-2 py-1 bg-gray-200 dark:bg-gray-600 rounded text-gray-600 dark:text-gray-300">
          {language || 'text'}
        </span>
      </div>
      <pre className="flex-1 overflow-auto p-4 text-sm font-mono bg-gray-50 dark:bg-gray-900 text-gray-800 dark:text-gray-200">
        {content}
      </pre>
    </div>
  );
}

export default function Files() {
  const [currentPath, setCurrentPath] = useState('');
  const [files, setFiles] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [selectedFile, setSelectedFile] = useState(null);
  const [fileContent, setFileContent] = useState(null);
  const [loadingContent, setLoadingContent] = useState(false);

  const fetchFiles = useCallback(async (path) => {
    setLoading(true);
    setError(null);
    try {
      const data = await fileApi.list(path);
      setFiles(data.files || []);
    } catch (err) {
      setError(err.message || 'Failed to load files');
      setFiles([]);
    } finally {
      setLoading(false);
    }
  }, []);

  const fetchFileContent = useCallback(async (path) => {
    setLoadingContent(true);
    try {
      const data = await fileApi.getContent(path);
      setFileContent(data);
    } catch (err) {
      setFileContent({ error: err.message || 'Failed to load file content' });
    } finally {
      setLoadingContent(false);
    }
  }, []);

  useEffect(() => {
    fetchFiles(currentPath);
  }, [currentPath, fetchFiles]);

  const handleSelect = (file) => {
    if (file.is_dir) {
      const newPath = currentPath ? `${currentPath}/${file.name}` : file.name;
      setCurrentPath(newPath);
      setSelectedFile(null);
      setFileContent(null);
    } else {
      const filePath = currentPath ? `${currentPath}/${file.name}` : file.name;
      setSelectedFile(file);
      fetchFileContent(filePath);
    }
  };

  const handleNavigate = (path) => {
    setCurrentPath(path);
    setSelectedFile(null);
    setFileContent(null);
  };

  const handleGoUp = () => {
    const parts = currentPath.split('/').filter(Boolean);
    parts.pop();
    setCurrentPath(parts.join('/'));
    setSelectedFile(null);
    setFileContent(null);
  };

  return (
    <div className="h-[calc(100vh-8rem)] flex gap-4">
      {/* File browser */}
      <div className="w-80 flex-shrink-0 bg-white dark:bg-gray-800 rounded-xl shadow-sm flex flex-col">
        {/* Breadcrumb */}
        <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
          <Breadcrumb path={currentPath} onNavigate={handleNavigate} />
        </div>

        {/* File list */}
        <div className="flex-1 overflow-y-auto">
          {currentPath && (
            <button
              onClick={handleGoUp}
              className="w-full text-left px-4 py-2 flex items-center gap-3 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-500 dark:text-gray-400"
            >
              <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 17l-5-5m0 0l5-5m-5 5h12" />
              </svg>
              <span className="text-sm">..</span>
            </button>
          )}

          {loading ? (
            <div className="flex justify-center py-8">
              <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-600" />
            </div>
          ) : error ? (
            <div className="px-4 py-8 text-center">
              <p className="text-red-600 dark:text-red-400 text-sm">{error}</p>
              <button
                onClick={() => fetchFiles(currentPath)}
                className="mt-2 text-blue-600 dark:text-blue-400 text-sm hover:underline"
              >
                Retry
              </button>
            </div>
          ) : files.length > 0 ? (
            files
              .sort((a, b) => {
                if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
                return a.name.localeCompare(b.name);
              })
              .map((file) => (
                <FileListItem
                  key={file.name}
                  file={file}
                  onSelect={handleSelect}
                  isSelected={selectedFile?.name === file.name}
                />
              ))
          ) : (
            <p className="px-4 py-8 text-center text-gray-500 dark:text-gray-400 text-sm">
              Empty directory
            </p>
          )}
        </div>
      </div>

      {/* File content viewer */}
      <div className="flex-1 bg-white dark:bg-gray-800 rounded-xl shadow-sm overflow-hidden">
        {loadingContent ? (
          <div className="h-full flex items-center justify-center">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
          </div>
        ) : fileContent?.error ? (
          <div className="h-full flex items-center justify-center text-red-600 dark:text-red-400">
            {fileContent.error}
          </div>
        ) : fileContent?.content ? (
          <CodeViewer
            content={fileContent.content}
            language={fileContent.language}
            filename={selectedFile?.name || 'file'}
          />
        ) : fileContent?.is_binary ? (
          <div className="h-full flex items-center justify-center text-gray-500 dark:text-gray-400">
            Binary file cannot be displayed
          </div>
        ) : (
          <div className="h-full flex items-center justify-center text-gray-500 dark:text-gray-400">
            <div className="text-center">
              <svg className="w-12 h-12 mx-auto mb-4 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              <p>Select a file to view its contents</p>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
