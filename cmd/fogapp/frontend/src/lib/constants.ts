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
    gemini: [
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
