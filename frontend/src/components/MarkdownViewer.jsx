import { useMemo, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { PrismLight as SyntaxHighlighter } from 'react-syntax-highlighter';
import bash from 'react-syntax-highlighter/dist/esm/languages/prism/bash';
import diff from 'react-syntax-highlighter/dist/esm/languages/prism/diff';
import go from 'react-syntax-highlighter/dist/esm/languages/prism/go';
import javascript from 'react-syntax-highlighter/dist/esm/languages/prism/javascript';
import json from 'react-syntax-highlighter/dist/esm/languages/prism/json';
import jsx from 'react-syntax-highlighter/dist/esm/languages/prism/jsx';
import markdown from 'react-syntax-highlighter/dist/esm/languages/prism/markdown';
import python from 'react-syntax-highlighter/dist/esm/languages/prism/python';
import sql from 'react-syntax-highlighter/dist/esm/languages/prism/sql';
import tsx from 'react-syntax-highlighter/dist/esm/languages/prism/tsx';
import typescript from 'react-syntax-highlighter/dist/esm/languages/prism/typescript';
import yaml from 'react-syntax-highlighter/dist/esm/languages/prism/yaml';
import { oneDark, oneLight } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { ChevronDown, ChevronRight } from 'lucide-react';
import { useUIStore } from '../stores';

SyntaxHighlighter.registerLanguage('bash', bash);
SyntaxHighlighter.registerLanguage('sh', bash);
SyntaxHighlighter.registerLanguage('diff', diff);
SyntaxHighlighter.registerLanguage('go', go);
SyntaxHighlighter.registerLanguage('javascript', javascript);
SyntaxHighlighter.registerLanguage('js', javascript);
SyntaxHighlighter.registerLanguage('json', json);
SyntaxHighlighter.registerLanguage('jsx', jsx);
SyntaxHighlighter.registerLanguage('markdown', markdown);
SyntaxHighlighter.registerLanguage('md', markdown);
SyntaxHighlighter.registerLanguage('python', python);
SyntaxHighlighter.registerLanguage('py', python);
SyntaxHighlighter.registerLanguage('sql', sql);
SyntaxHighlighter.registerLanguage('tsx', tsx);
SyntaxHighlighter.registerLanguage('typescript', typescript);
SyntaxHighlighter.registerLanguage('ts', typescript);
SyntaxHighlighter.registerLanguage('yaml', yaml);
SyntaxHighlighter.registerLanguage('yml', yaml);

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

export default function MarkdownViewer({ markdown }) {
  const { frontmatter, body } = useMemo(() => splitFrontmatter(markdown), [markdown]);
  const [showFrontmatter, setShowFrontmatter] = useState(false);
  const theme = useUIStore((s) => s.theme);
  const isDark = useMemo(() => {
    if (theme === 'dark' || theme === 'midnight') return true;
    if (theme === 'light') return false;
    if (typeof window === 'undefined') return false;
    return window.matchMedia?.('(prefers-color-scheme: dark)')?.matches ?? false;
  }, [theme]);

  return (
    <div className="space-y-4">
      {frontmatter && (
        <div className="rounded-lg border border-border bg-card">
          <button
            type="button"
            onClick={() => setShowFrontmatter((v) => !v)}
            className="w-full flex items-center justify-between gap-2 px-3 py-2 text-left text-sm font-medium text-foreground hover:bg-accent/40 rounded-lg"
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

      <div className="min-w-0">
        <ReactMarkdown
          remarkPlugins={[remarkGfm]}
          components={{
            h1: ({ children, ...props }) => (
              <h1 className="text-2xl font-semibold tracking-tight mt-2 mb-4" {...props}>
                {children}
              </h1>
            ),
            h2: ({ children, ...props }) => (
              <h2 className="text-xl font-semibold tracking-tight mt-6 mb-3" {...props}>
                {children}
              </h2>
            ),
            h3: ({ children, ...props }) => (
              <h3 className="text-lg font-semibold mt-5 mb-2" {...props}>
                {children}
              </h3>
            ),
            p: ({ children, ...props }) => (
              <p className="text-sm leading-6 text-foreground/90 my-3" {...props}>
                {children}
              </p>
            ),
            a: ({ children, href, ...props }) => (
              <a
                href={href}
                target="_blank"
                rel="noreferrer"
                className="text-primary underline underline-offset-4 hover:opacity-80"
                {...props}
              >
                {children}
              </a>
            ),
            ul: ({ children, ...props }) => (
              <ul className="my-3 pl-5 list-disc space-y-1 text-sm" {...props}>
                {children}
              </ul>
            ),
            ol: ({ children, ...props }) => (
              <ol className="my-3 pl-5 list-decimal space-y-1 text-sm" {...props}>
                {children}
              </ol>
            ),
            li: ({ children, ...props }) => (
              <li className="text-foreground/90" {...props}>
                {children}
              </li>
            ),
            blockquote: ({ children, ...props }) => (
              <blockquote className="my-4 border-l-2 border-border pl-4 text-sm text-muted-foreground" {...props}>
                {children}
              </blockquote>
            ),
            hr: (props) => <hr className="my-6 border-border" {...props} />,
            code: ({ children, className, inline, ...props }) => {
              const match = /language-([a-zA-Z0-9_-]+)/.exec(className || '');
              const content = String(children || '').replace(/\n$/, '');

              if (!inline && match) {
                return (
                  <SyntaxHighlighter
                    language={match[1]}
                    style={isDark ? oneDark : oneLight}
                    PreTag="div"
                    customStyle={{ margin: 0, background: 'transparent' }}
                    codeTagProps={{ style: { background: 'transparent' } }}
                    {...props}
                  >
                    {content}
                  </SyntaxHighlighter>
                );
              }

              if (!inline) {
                return (
                  <code className="block whitespace-pre font-mono text-xs" {...props}>
                    {content}
                  </code>
                );
              }

              return (
                <code className="px-1 py-0.5 rounded bg-muted text-xs font-mono" {...props}>
                  {children}
                </code>
              );
            },
            pre: ({ children, ...props }) => (
              <div className="my-4 p-4 rounded-lg bg-muted overflow-x-auto border border-border text-xs" {...props}>
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
