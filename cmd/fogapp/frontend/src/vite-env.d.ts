/// <reference types="svelte" />
/// <reference types="vite/client" />

declare global {
    interface Window {
        __FOG_API_BASE_URL__?: string;
        go?: {
            main?: {
                desktopApp?: {
                    APIBaseURL: () => Promise<string>;
                    Version: () => Promise<string>;
                    APIToken: () => Promise<string>;
                    OpenExternal: (url: string) => Promise<void>;
                };
            };
        };
        // Injected by the Wails shell; absent when the frontend is served
        // standalone, so every use must be guarded.
        runtime?: {
            BrowserOpenURL: (url: string) => void;
        };
    }
}

export { };
