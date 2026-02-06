import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import SyntaxHighlighter from '../lib/syntax';
import { oneDark, oneLight, dracula, nord } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { useUIStore } from '../stores';
import { useState, useMemo } from 'react';
import { Copy, CheckCircle2 } from 'lucide-react';

function CodeCopyButton({ text }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Silent fail
    }
  };

  return (
    <button
      type="button"
      onClick={handleCopy}
      className="absolute top-2 right-2 p-1.5 rounded-md bg-background/50 hover:bg-background shadow-sm border border-border text-muted-foreground hover:text-foreground opacity-0 group-hover:opacity-100 transition-all z-10"
      title={copied ? 'Copied!' : 'Copy code'}
    >
      {copied ? (
        <CheckCircle2 className="w-3.5 h-3.5 text-green-500" />
      ) : (
        <Copy className="w-3.5 h-3.5" />
      )}
    </button>
  );
}

export default function ChatMarkdown({ content, isUser }) {
  const theme = useUIStore((s) => s.theme);
  
  const isDark = useMemo(() => {
    if (theme === 'dark' || theme === 'midnight') return true;
    if (theme === 'light') return false;
    if (typeof window === 'undefined') return false;
    return window.matchMedia?.('(prefers-color-scheme: dark)')?.matches ?? false;
  }, [theme]);

  // For user messages, we might want simpler rendering or different styling
  // But using the same renderer ensures code blocks still look good
  
  const syntaxTheme = useMemo(() => {
    if (theme === 'dracula') return dracula;
    if (theme === 'nord') return nord;
    return isDark ? oneDark : oneLight;
  }, [theme, isDark]);

  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      components={{
        p: ({ children }) => <p className="mb-2 last:mb-0 leading-relaxed text-sm">{children}</p>,
        a: ({ children, href }) => (
          <a
            href={href}
            target="_blank"
            rel="noreferrer"
            className="text-primary underline underline-offset-4 hover:opacity-80"
          >
            {children}
          </a>
        ),
        code: ({ children, className, inline, ...props }) => {
          const match = /language-([a-zA-Z0-9_-]+)/.exec(className || '');
          const codeContent = String(children || '').replace(/\n$/, '');

          if (!inline && match) {
            return (
              <div className="relative group my-2 rounded-md overflow-hidden border border-border/50">
                <CodeCopyButton text={codeContent} />
                <SyntaxHighlighter
                  language={match[1]}
                  style={syntaxTheme}
                  PreTag="div"
                  customStyle={{ margin: 0, padding: '1rem', background: isUser ? 'rgba(0,0,0,0.1)' : undefined }}
                  codeTagProps={{ style: { fontFamily: 'var(--font-mono)' } }}
                  {...props}
                >
                  {codeContent}
                </SyntaxHighlighter>
              </div>
            );
          }

          if (!inline) {
             return (
              <div className="relative group my-2 rounded-md overflow-hidden border border-border/50">
                <CodeCopyButton text={codeContent} />
                <div className={`p-3 overflow-x-auto ${isUser ? 'bg-black/10' : 'bg-muted'}`}>
                  <code className="block whitespace-pre font-mono text-xs" {...props}>
                    {codeContent}
                  </code>
                </div>
              </div>
            );
          }

          return (
            <code className={`px-1 py-0.5 rounded text-xs font-mono ${isUser ? 'bg-black/10' : 'bg-muted'}`} {...props}>
              {children}
            </code>
          );
        },
        ul: ({ children }) => <ul className="list-disc pl-4 mb-2 space-y-1">{children}</ul>,
        ol: ({ children }) => <ol className="list-decimal pl-4 mb-2 space-y-1">{children}</ol>,
        li: ({ children }) => <li>{children}</li>,
      }}
    >
      {content}
    </ReactMarkdown>
  );
}
