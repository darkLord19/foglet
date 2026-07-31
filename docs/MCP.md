# MCP proxy

Fog exposes one authenticated local MCP endpoint at `/mcp`. The proxy keeps
upstream server configuration in `FOG_HOME/mcp.json`, resolves credentials from
Fog's encrypted secret store, and forwards `initialize`, `tools/list`, and
`tools/call` requests.

The current proxy is intentionally narrow:

- upstreams must expose a JSON MCP HTTP endpoint;
- upstream tools are namespaced as `<server>__<tool>`;
- credentials are referenced by secret key, never stored in the registry file;
- the local API bearer token protects the endpoint;
- unavailable upstreams fail the request instead of silently returning partial
  tool results.
