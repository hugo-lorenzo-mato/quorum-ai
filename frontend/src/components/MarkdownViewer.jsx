import { useMemo, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import SyntaxHighlighter from '../lib/syntax';
import { oneDark, oneLight, dracula, nord } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { ChevronDown, ChevronRight, Copy, CheckCircle2 } from 'lucide-react';
import { useUIStore } from '../stores';

function splitFrontmatter(markdown) {
  const normalized = (markdown || '').replace(/\r\n/g, '\n');
  const lines = normalized.split('\n');

  if (lines[0] !== '---') {
    return { frontmatter: null, body: normalized };
  }

  const endIndex = lines.slice(1).findIndex((line) => line.trim() === '---');
  if (endIndex === -1) {
    return { frontmatter: null, body: normalized };
  }

  const frontmatterLines = lines.slice(1, endIndex + 1);
  const bodyLines = lines.slice(endIndex + 2);
  return {
    frontmatter: frontmatterLines.join('\n').trim(),
    body: bodyLines.join('\n').trimStart(),
  };
}

function CopyButton({ text }) {
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
      className="absolute top-2 right-2 p-1.5 rounded-md bg-background/50 hover:bg-background shadow-sm border border-border text-muted-foreground hover:text-foreground opacity-0 group-hover:opacity-100 focus:opacity-100 transition-all z-10"
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

export default function MarkdownViewer({ markdown }) {
  const { frontmatter, body } = useMemo(() => splitFrontmatter(markdown), [markdown]);
  const [showFrontmatter, setShowFrontmatter] = useState(false);
  const [copied, setCopied] = useState(false);
  const theme = useUIStore((s) => s.theme);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(markdown || '');
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Silent fail - copy button is a convenience feature
    }
  };
  const isDark = useMemo(() => {
    if (theme === 'dark' || theme === 'midnight') return true;
    if (theme === 'light') return false;
    if (typeof window === 'undefined') return false;
    return window.matchMedia?.('(prefers-color-scheme: dark)')?.matches ?? false;
  }, [theme]);

  return (
    <div className="space-y-4 relative">
      <button
        type="button"
        onClick={handleCopy}
        className={`absolute top-0 right-0 p-2 rounded-lg hover:bg-accent/50 transition-colors ${copied ? 'text-green-500' : 'text-muted-foreground'}`}
        title={copied ? 'Copied!' : 'Copy content'}
      >
        {copied ? (
          <CheckCircle2 className="w-4 h-4" />
        ) : (
          <Copy className="w-4 h-4" />
        )}
      </button>
      {frontmatter && (
        <div className="rounded-lg border border-border bg-card">
          <button
            type="button"
            onClick={() => setShowFrontmatter((v) => !v)}
            className="w-full flex items-center justify-between gap-2 px-3 py-2 pr-12 text-left text-sm font-medium text-foreground hover:bg-accent/40 rounded-lg"
          >
            <span>Metadata</span>
            {showFrontmatter ? (
              <ChevronDown className="w-4 h-4 text-muted-foreground" />
            ) : (
              <ChevronRight className="w-4 h-4 text-muted-foreground" />
            )}
          </button>
          {showFrontmatter && (
            <pre className="px-3 pb-3 overflow-x-auto text-xs text-muted-foreground">
              {frontmatter}
            </pre>
          )}
        </div>
      )}

      <div className="min-w-0 pr-12">
        <ReactMarkdown
          remarkPlugins={[remarkGfm]}
          components={{
            h1: ({ children, ...props }) => (
              <h1 className="text-2xl font-bold tracking-tight mt-2 mb-4 border-b border-border pb-2" {...props}>
                {children}
              </h1>
            ),
            h2: ({ children, ...props }) => (
              <h2 className="text-xl font-semibold tracking-tight mt-8 mb-4 border-b border-border/50 pb-1" {...props}>
                {children}
              </h2>
            ),
            h3: ({ children, ...props }) => (
              <h3 className="text-lg font-semibold mt-6 mb-2" {...props}>
                {children}
              </h3>
            ),
            p: ({ children, ...props }) => (
              <p className="text-sm leading-7 text-foreground/90 my-4" {...props}>
                {children}
              </p>
            ),
            img: ({ src, alt, ...props }) => (
              <img
                src={src}
                alt={alt}
                className="max-w-full h-auto rounded-lg border border-border my-6 mx-auto"
                loading="lazy"
                {...props}
              />
            ),
            a: ({ children, href, ...props }) => (
              <a
                href={href}
                target="_blank"
                rel="noreferrer"
                className="text-primary underline underline-offset-4 hover:opacity-80 break-all"
                {...props}
              >
                {children}
              </a>
            ),
            ul: ({ children, ...props }) => (
              <ul className="my-3 pl-5 list-disc space-y-1 text-sm [&_ul]:my-1 [&_ol]:my-1 [&_ul]:space-y-0 [&_ol]:space-y-0" {...props}>
                {children}
              </ul>
            ),
            ol: ({ children, ...props }) => (
              <ol className="my-3 pl-5 list-decimal space-y-1 text-sm [&_ul]:my-1 [&_ol]:my-1 [&_ul]:space-y-0 [&_ol]:space-y-0" {...props}>
                {children}
              </ol>
            ),
            li: ({ children, ...props }) => (
              <li className="text-sm text-foreground/90 pl-1" {...props}>
                {children}
              </li>
            ),
            blockquote: ({ children, ...props }) => (
              <blockquote className="my-4 border-l-4 border-primary/50 pl-4 py-1 bg-muted/30 rounded-r-lg text-sm text-muted-foreground italic" {...props}>
                {children}
              </blockquote>
            ),
            hr: (props) => <hr className="my-8 border-border" {...props} />,
            code: ({ children, className, ...props }) => {
              const match = /language-([a-zA-Z0-9_-]+)/.exec(className || '');
              const content = String(children || '').replace(/\n$/, '');
              // In react-markdown v9, inline code has no className and no newlines
              const isInline = !className && !content.includes('\n');

              const syntaxTheme = useMemo(() => {
                if (theme === 'dracula') return dracula;
                if (theme === 'nord') return nord;
                return isDark ? oneDark : oneLight;
              }, [theme, isDark]);

              if (match) {
                // Block code with syntax highlighting
                return (
                  <>
                    <CopyButton text={content} />
                    <div className="p-4 overflow-x-auto">
                      <SyntaxHighlighter
                        language={match[1]}
                        style={syntaxTheme}
                        PreTag="div"
                        customStyle={{ margin: 0, background: 'transparent' }}
                        codeTagProps={{ style: { background: 'transparent' } }}
                        {...props}
                      >
                        {content}
                      </SyntaxHighlighter>
                    </div>
                  </>
                );
              }

              if (!isInline) {
                // Block code without language
                return (
                  <>
                    <CopyButton text={content} />
                    <div className="p-4 overflow-x-auto">
                      <code className="block whitespace-pre font-mono text-xs" {...props}>
                        {content}
                      </code>
                    </div>
                  </>
                );
              }

              // Inline code
              return (
                <code className="px-1.5 py-0.5 rounded-md bg-muted font-mono text-[0.9em] border border-border/50" {...props}>
                  {children}
                </code>
              );
            },
            pre: ({ children, ...props }) => (
              <div className="my-4 rounded-lg bg-muted border border-border text-xs relative group" {...props}>
                {children}
              </div>
            ),
            table: ({ children, ...props }) => (
              <div className="my-4 overflow-x-auto border border-border rounded-lg">
                <table className="w-full text-sm" {...props}>
                  {children}
                </table>
              </div>
            ),
            thead: ({ children, ...props }) => (
              <thead className="bg-muted/60" {...props}>
                {children}
              </thead>
            ),
            th: ({ children, ...props }) => (
              <th className="px-3 py-2 text-left font-medium text-foreground whitespace-nowrap" {...props}>
                {children}
              </th>
            ),
            td: ({ children, ...props }) => (
              <td className="px-3 py-2 text-foreground/90 align-top" {...props}>
                {children}
              </td>
            ),
          }}
        >
          {body}
        </ReactMarkdown>
      </div>
    </div>
  );
}
