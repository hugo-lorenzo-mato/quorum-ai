import Settings from './Settings';
import { ConfigStoreProvider, globalConfigStore } from '../stores/configStore';

export default function GlobalSettings() {
  return (
    <ConfigStoreProvider store={globalConfigStore}>
      <Settings
        title="Global Settings"
        description="Defaults applied to all projects (unless a project uses a custom config)"
        showProjectBanner={false}
      />
    </ConfigStoreProvider>
  );
}

