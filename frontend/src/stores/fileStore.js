import { create } from 'zustand';

const API_BASE = '/api/v1';

const useFileStore = create((set, get) => ({
  files: [],
  loading: false,
  uploading: false,
  error: null,

  fetchFiles: async () => {
    set({ loading: true, error: null });
    try {
      const response = await fetch(`${API_BASE}/files`);
      if (!response.ok) throw new Error('Failed to fetch files');
      const data = await response.json();
      set({ files: data.files || [], loading: false });
    } catch (error) {
      set({ error: error.message, loading: false });
      // Return empty array on error for graceful degradation
      set({ files: [] });
    }
  },

  uploadFiles: async (fileList) => {
    set({ uploading: true, error: null });
    try {
      const formData = new FormData();
      for (const file of fileList) {
        formData.append('files', file);
      }

      const response = await fetch(`${API_BASE}/files/upload`, {
        method: 'POST',
        body: formData,
      });

      if (!response.ok) throw new Error('Failed to upload files');

      // Refresh file list
      await get().fetchFiles();
      set({ uploading: false });
    } catch (error) {
      set({ error: error.message, uploading: false });
    }
  },

  downloadFile: async (file) => {
    try {
      const response = await fetch(`${API_BASE}/files/${file.id}/download`);
      if (!response.ok) throw new Error('Failed to download file');

      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = file.name;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
    } catch (error) {
      set({ error: error.message });
    }
  },

  deleteFile: async (fileId) => {
    try {
      const response = await fetch(`${API_BASE}/files/${fileId}`, {
        method: 'DELETE',
      });

      if (!response.ok) throw new Error('Failed to delete file');

      // Remove from local state
      set(state => ({
        files: state.files.filter(f => f.id !== fileId),
      }));
    } catch (error) {
      set({ error: error.message });
    }
  },

  clearError: () => set({ error: null }),
}));

export default useFileStore;
