import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { useEffect, lazy, Suspense } from 'react';
import Layout from './components/Layout';
import useSSE from './hooks/useSSE';
import { useUIStore } from './stores';
import { useConfigStore } from './stores/configStore';
import { loadEnums } from './lib/agents';

// Lazy load page components
const Landing = lazy(() => import('./pages/Landing'));
const Dashboard = lazy(() => import('./pages/Dashboard'));
const Workflows = lazy(() => import('./pages/Workflows'));
const Prompts = lazy(() => import('./pages/Prompts'));
const SystemPrompts = lazy(() => import('./pages/SystemPrompts'));
const IssuesEditor = lazy(() => import('./pages/IssuesEditor'));
const Chat = lazy(() => import('./pages/Chat'));
const Settings = lazy(() => import('./pages/Settings'));
const GlobalSettings = lazy(() => import('./pages/GlobalSettings'));
const Kanban = lazy(() => import('./pages/Kanban'));
const Projects = lazy(() => import('./pages/Projects'));

// Loading fallback component
const PageLoader = () => (
  <div className="flex items-center justify-center h-[calc(100vh-10rem)]">
    <div className="relative">
      <div className="w-12 h-12 border-4 border-primary/20 border-t-primary rounded-full animate-spin"></div>
      <div className="mt-4 text-sm font-medium text-muted-foreground animate-pulse text-center">Loading...</div>
    </div>
  </div>
);

function AppContent() {
  // Initialize SSE connection
  useSSE();

  // Initialize theme
  const { theme, setTheme } = useUIStore();

  useEffect(() => {
    setTheme(theme);
  }, [theme, setTheme]);

  // Load enums and config from API on app start
  useEffect(() => {
    loadEnums();
    // Load config and metadata globally so agent enabled/disabled state is available everywhere
    const configStore = useConfigStore.getState();
    configStore.loadConfig();
    configStore.loadMetadata();
  }, []);

  return (
    <Layout>
      <Suspense fallback={<PageLoader />}>
        <Routes>
          <Route path="/" element={<Landing />} />
          <Route path="/dashboard" element={<Dashboard />} />
          <Route path="/workflows" element={<Workflows />} />
          <Route path="/workflows/:id" element={<Workflows />} />
          <Route path="/workflows/:id/issues" element={<IssuesEditor />} />
          <Route path="/prompts" element={<Prompts />} />
          <Route path="/system-prompts" element={<SystemPrompts />} />
          <Route path="/kanban" element={<Kanban />} />
          <Route path="/chat" element={<Chat />} />
          <Route path="/projects" element={<Projects />} />
          <Route path="/settings" element={<Settings />} />
          <Route path="/settings/global" element={<GlobalSettings />} />
        </Routes>
      </Suspense>
    </Layout>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <AppContent />
    </BrowserRouter>
  );
}
