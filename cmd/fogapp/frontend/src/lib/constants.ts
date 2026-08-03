export interface MCPPreset {
    id: string;
    label: string;
    defaultName: string;
    description: string;
    defaultURL: string;
    needsToken: boolean;
    tokenLabel: string;
    docsHint: string;
}

export const MCP_PRESETS: MCPPreset[] = [
    {
        id: "github",
        label: "GitHub",
        defaultName: "github",
        description: "File, PR, and repo tools via GitHub MCP",
        defaultURL: "https://api.githubcopilot.com/mcp/",
        needsToken: true,
        tokenLabel: "GitHub Copilot token",
        docsHint: "Requires an active GitHub Copilot subscription.",
    },
    {
        id: "sentry",
        label: "Sentry",
        defaultName: "sentry",
        description: "Error lookup and triage from Sentry",
        defaultURL: "https://mcp.sentry.io/mcp",
        needsToken: true,
        tokenLabel: "Sentry auth token",
        docsHint: "Create a token in Sentry → Settings → Auth Tokens.",
    },
    {
        id: "datadog",
        label: "Datadog",
        defaultName: "datadog",
        description: "Metrics, logs, and monitors from Datadog",
        defaultURL: "",
        needsToken: true,
        tokenLabel: "Datadog API key",
        docsHint: "Paste your Datadog MCP server URL from the Datadog docs.",
    },
    {
        id: "notion",
        label: "Notion",
        defaultName: "notion",
        description: "Read and write Notion pages and databases",
        defaultURL: "",
        needsToken: true,
        tokenLabel: "Notion integration token",
        docsHint: "Paste your Notion MCP server URL.",
    },
    {
        id: "linear",
        label: "Linear",
        defaultName: "linear",
        description: "Issues and projects from Linear",
        defaultURL: "",
        needsToken: true,
        tokenLabel: "Linear API key",
        docsHint: "Paste your Linear MCP server URL.",
    },
    {
        id: "slack",
        label: "Slack",
        defaultName: "slack",
        description: "Read and post messages in Slack",
        defaultURL: "",
        needsToken: true,
        tokenLabel: "Slack bot token",
        docsHint: "Paste your Slack MCP server URL.",
    },
    {
        id: "google",
        label: "Google",
        defaultName: "google",
        description: "Google Drive, Calendar, or Workspace tools",
        defaultURL: "",
        needsToken: true,
        tokenLabel: "Service account key",
        docsHint: "Paste your Google MCP server URL.",
    },
];

export const TOOL_MODELS: Record<string, string[]> = {
    claude: [
        "opus-4.6",
        "opus-4.5",
        "sonnet-4.5",
    ],
    cursor: [
        "auto",
        "opus-4.6-thinking",
        "opus-4.5-thinking",
        "sonnet-4.5-thinking",
        "gpt-5.3-codex"
    ],
    antigravity: [
        "auto",
        "gemini-3-pro-preview",
        "gemini-3-flash-preview",
        "gemini-2.5-pro",
        "gemini-2.5-flash",
        "gemini-2.5-flash-lite",
    ],
    codex: [
        "gpt-5.2",
        "gpt-5.3-codex"
    ]
};

export function getModelsForTool(tool: string): string[] {
    return TOOL_MODELS[tool] ?? [];
}
