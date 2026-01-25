import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { useEffect } from 'react';
import Layout from './components/Layout';
import Dashboard from './pages/Dashboard';
import Workflows from './pages/Workflows';
import Chat from './pages/Chat';
import Files from './pages/Files';
import Settings from './pages/Settings';
import useSSE from './hooks/useSSE';
import { useUIStore } from './stores';

function AppContent() {
  // Initialize SSE connection
  useSSE();

  // Initialize theme
  const { theme, setTheme } = useUIStore();

  useEffect(() => {
    setTheme(theme);
  }, [theme, setTheme]);

  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/workflows" element={<Workflows />} />
        <Route path="/workflows/:id" element={<Workflows />} />
        <Route path="/chat" element={<Chat />} />
        <Route path="/files" element={<Files />} />
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
