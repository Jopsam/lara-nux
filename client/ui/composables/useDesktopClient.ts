import type { DesktopClientApi, DesktopWindow } from '~/types/desktop';

const unavailableMessage = 'Lara Nux desktop bridge is unavailable. Start the Wails shell and the privileged daemon before using the UI.';

function createUnavailableClient(): DesktopClientApi {
  const reject = async <T>(): Promise<T> => {
    throw new Error(unavailableMessage);
  };

  return {
    GetShellState: () => reject(),
    LoadDashboard: () => reject(),
    ListSites: () => reject(),
    GetSite: () => reject(),
    RegisterSite: () => reject(),
    UpdateSite: () => reject(),
    GetHealth: () => reject(),
    GetRuntimeCatalog: () => reject(),
    SetDefaultRuntime: () => reject(),
    SwitchSiteRuntime: () => reject(),
    ServiceAction: () => reject(),
  };
}

export function useDesktopClient(): DesktopClientApi {
  const desktopWindow = globalThis as unknown as DesktopWindow;
  return desktopWindow.go?.main?.App ?? createUnavailableClient();
}
