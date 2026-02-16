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
        "gemini-3-pro",
        "gemini-3-flash",
        "gemini-2.5-pro",
        "gemini-2.5-flash",
    ],
    codex: [
        "gpt-5.2",
        "gpt-5.3-codex"
    ]
};

export function getModelsForTool(tool: string): string[] {
    return TOOL_MODELS[tool] ?? [];
}
