#!/usr/bin/env node

import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";
import { exec } from "child_process";
import { promisify } from "util";

const execAsync = promisify(exec);

// MCP Server for wtx
const server = new Server(
  {
    name: "wtx-mcp-server",
    version: "0.1.0",
  },
  {
    capabilities: {
      tools: {},
    },
  }
);

// List available tools
server.setRequestHandler(ListToolsRequestSchema, async () => {
  return {
    tools: [
      {
        name: "wtx_list_worktrees",
        description: "List all Git worktrees in the current repository with their status",
        inputSchema: {
          type: "object",
          properties: {},
        },
      },
      {
        name: "wtx_switch_worktree",
        description: "Switch to a different worktree by opening it in the editor",
        inputSchema: {
          type: "object",
          properties: {
            name: {
              type: "string",
              description: "Name of the worktree to switch to",
            },
          },
          required: ["name"],
        },
      },
      {
        name: "wtx_create_worktree",
        description: "Create a new Git worktree",
        inputSchema: {
          type: "object",
          properties: {
            name: {
              type: "string",
              description: "Name for the new worktree",
            },
            branch: {
              type: "string",
              description: "Branch name (defaults to worktree name if not provided)",
            },
          },
          required: ["name"],
        },
      },
      {
        name: "wtx_delete_worktree",
        description: "Delete a Git worktree",
        inputSchema: {
          type: "object",
          properties: {
            name: {
              type: "string",
              description: "Name of the worktree to delete",
            },
          },
          required: ["name"],
        },
      },
    ],
  };
});

// Handle tool calls
server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args } = request.params;

  try {
    switch (name) {
      case "wtx_list_worktrees": {
        const { stdout } = await execAsync("wtx list --json");
        return {
          content: [
            {
              type: "text",
              text: stdout,
            },
          ],
        };
      }

      case "wtx_switch_worktree": {
        const worktreeName = args.name as string;
        await execAsync(`wtx open ${worktreeName}`);
        return {
          content: [
            {
              type: "text",
              text: `Switched to worktree: ${worktreeName}`,
            },
          ],
        };
      }

      case "wtx_create_worktree": {
        const worktreeName = args.name as string;
        const branch = (args.branch as string) || worktreeName;
        await execAsync(`wtx add ${worktreeName} ${branch}`);
        return {
          content: [
            {
              type: "text",
              text: `Created worktree: ${worktreeName} (branch: ${branch})`,
            },
          ],
        };
      }

      case "wtx_delete_worktree": {
        const worktreeName = args.name as string;
        await execAsync(`wtx rm ${worktreeName}`);
        return {
          content: [
            {
              type: "text",
              text: `Deleted worktree: ${worktreeName}`,
            },
          ],
        };
      }

      default:
        throw new Error(`Unknown tool: ${name}`);
    }
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : String(error);
    return {
      content: [
        {
          type: "text",
          text: `Error: ${errorMessage}`,
        },
      ],
      isError: true,
    };
  }
});

// Start the server
async function main() {
  const transport = new StdioServerTransport();
  await server.connect(transport);
  
  // Log to stderr (stdout is used for MCP protocol)
  console.error("wtx MCP server running");
}

main().catch((error) => {
  console.error("Server error:", error);
  process.exit(1);
});
