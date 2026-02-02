import { Editor } from '@monaco-editor/react';
import { useUIStore } from '../stores';
import { Loader2 } from 'lucide-react';

export default function CodeEditor({ value, language = 'markdown', readOnly = true, onChange }) {
  const theme = useUIStore((state) => state.theme);
  
  // Map our themes to Monaco themes
  const editorTheme = theme === 'dark' || theme === 'midnight' ? 'vs-dark' : 'light';

  return (
    <div className="h-full w-full min-h-[400px] border border-border rounded-lg overflow-hidden">
      <Editor
        height="100%"
        defaultLanguage={language}
        value={value}
        theme={editorTheme}
        onChange={onChange}
        options={{
          readOnly,
          minimap: { enabled: false },
          scrollBeyondLastLine: false,
          fontSize: 13,
          fontFamily: "'JetBrains Mono', monospace",
          padding: { top: 16, bottom: 16 },
          lineNumbers: 'on',
          renderLineHighlight: 'all',
          automaticLayout: true,
        }}
        loading={<div className="flex items-center justify-center h-full text-muted-foreground"><Loader2 className="w-5 h-5 animate-spin" /></div>}
      />
    </div>
  );
}
