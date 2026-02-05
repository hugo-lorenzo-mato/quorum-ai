import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { useEffect } from 'react';
import Layout from './components/Layout';
import Dashboard from './pages/Dashboard';
import Workflows from './pages/Workflows';
import Templates from './pages/Templates';
import IssuesEditor from './pages/IssuesEditor';
import Chat from './pages/Chat';
import Settings from './pages/Settings';
import Kanban from './pages/Kanban';
import Projects from './pages/Projects';
import useSSE from './hooks/useSSE';
import { useUIStore } from './stores';
import { loadEnums } from './lib/agents';

function AppContent() {
  // Initialize SSE connection
  useSSE();

  // Initialize theme
  const { theme, setTheme } = useUIStore();

  useEffect(() => {
    setTheme(theme);
  }, [theme, setTheme]);

  // Load enums from API on app start
  useEffect(() => {
    loadEnums();
  }, []);

  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/workflows" element={<Workflows />} />
        <Route path="/workflows/:id" element={<Workflows />} />
        <Route path="/workflows/:id/issues" element={<IssuesEditor />} />
        <Route path="/templates" element={<Templates />} />
        <Route path="/kanban" element={<Kanban />} />
        <Route path="/chat" element={<Chat />} />
        <Route path="/projects" element={<Projects />} />
        <Route path="/settings" element={<Settings />} />
      </Routes>
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
