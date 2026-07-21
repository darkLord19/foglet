// ── Utility functions ──

export function formatDate(value?: string): string {
    if (!value) return "–";
    const dt = new Date(value);
    if (isNaN(dt.getTime())) return "–";
    return dt.toLocaleString();
}

export function firstPromptLine(prompt?: string): string {
    const text = (prompt ?? "").trim();
    if (!text) return "Untitled session";
    const first =
        text.split(/\r?\n/).find((l) => l.trim() !== "") ?? text;
    const trimmed = first.trim();
    return trimmed.length > 110 ? trimmed.slice(0, 110) + "…" : trimmed;
}

export function relativeTime(value?: string): string {
    if (!value) return "";
    const dt = new Date(value);
    if (isNaN(dt.getTime())) return "";
    const diff = Date.now() - dt.getTime();
    const seconds = Math.floor(diff / 1000);
    if (seconds < 60) return "just now";
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours}h ago`;
    const days = Math.floor(hours / 24);
    return `${days}d ago`;
}

/**
 * Open a URL in the user's real browser.
 *
 * Prefers the bound Go method (`OpenExternal`), which is what `main_desktop.go`
 * exposes for this. Falls back to the Wails runtime global, and finally to a
 * plain `window.open` so the app still works when served outside the desktop
 * shell (the e2e harness does exactly that).
 */
export function openExternal(url?: string): void {
    if (!url) return;

    const bound = window.go?.main?.desktopApp?.OpenExternal;
    if (bound) {
        void bound(url);
        return;
    }

    if (window.runtime?.BrowserOpenURL) {
        window.runtime.BrowserOpenURL(url);
        return;
    }

    window.open(url, "_blank", "noopener,noreferrer");
}

// Aliases used by components
export const formatRelativeTime = relativeTime;
export const truncatePrompt = firstPromptLine;
