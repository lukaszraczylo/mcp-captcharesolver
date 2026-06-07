// Minimal newline-delimited JSON-RPC client for the captcha-solver-mcp stdio server.
//
// This is intentionally tiny and dependency-free so the examples are runnable with just
// Node and the built `bin/captcha-solver-mcp`. For production use you can swap this for
// the official `@modelcontextprotocol/sdk` (StdioClientTransport) instead.
//
// The MCP stdio transport frames each JSON-RPC message as a single line of JSON terminated
// by '\n'. We spawn the server, perform the `initialize` handshake, then expose `callTool`.

import { spawn } from 'node:child_process';

export class McpStdioClient {
  /**
   * @param {string} command  Absolute path to the bin/captcha-solver-mcp binary.
   * @param {Record<string,string>} env  CAPTCHA_LLM_* configuration for the server.
   */
  constructor(command, env = {}) {
    this.proc = spawn(command, [], {
      stdio: ['pipe', 'pipe', 'inherit'], // server logs go to our stderr
      env: { ...process.env, ...env },
    });
    this.nextId = 1;
    this.pending = new Map(); // id -> {resolve, reject}
    this.buffer = '';

    this.proc.stdout.on('data', (chunk) => this.#onData(chunk));
    this.proc.on('exit', (code) => {
      for (const { reject } of this.pending.values()) {
        reject(new Error(`server exited with code ${code}`));
      }
      this.pending.clear();
    });
  }

  #onData(chunk) {
    this.buffer += chunk.toString('utf8');
    let nl;
    while ((nl = this.buffer.indexOf('\n')) !== -1) {
      const line = this.buffer.slice(0, nl).trim();
      this.buffer = this.buffer.slice(nl + 1);
      if (!line) continue;
      let msg;
      try {
        msg = JSON.parse(line);
      } catch {
        continue; // ignore non-JSON noise
      }
      if (msg.id == null) continue; // notifications: ignore
      const waiter = this.pending.get(msg.id);
      if (!waiter) continue;
      this.pending.delete(msg.id);
      if (msg.error) waiter.reject(new Error(`${msg.error.code}: ${msg.error.message}`));
      else waiter.resolve(msg.result);
    }
  }

  #request(method, params) {
    const id = this.nextId++;
    const payload = JSON.stringify({ jsonrpc: '2.0', id, method, params }) + '\n';
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.proc.stdin.write(payload);
    });
  }

  #notify(method, params) {
    this.proc.stdin.write(JSON.stringify({ jsonrpc: '2.0', method, params }) + '\n');
  }

  /** Perform the MCP initialize handshake. Call once before callTool. */
  async initialize() {
    const result = await this.#request('initialize', {
      protocolVersion: '2025-06-18',
      capabilities: {},
      clientInfo: { name: 'captcha-example', version: '0.1.0' },
    });
    this.#notify('notifications/initialized', {});
    return result;
  }

  /**
   * Call a tool by name. Returns the tool's structured output (structuredContent),
   * falling back to the parsed text content if no structured output is present.
   * @param {string} name
   * @param {object} args
   */
  async callTool(name, args) {
    const result = await this.#request('tools/call', { name, arguments: args });
    if (result.isError) {
      const text = (result.content || []).map((c) => c.text).join('\n');
      throw new Error(`tool ${name} failed: ${text}`);
    }
    if (result.structuredContent) return result.structuredContent;
    const text = (result.content || []).map((c) => c.text).join('');
    try {
      return JSON.parse(text);
    } catch {
      return text;
    }
  }

  close() {
    this.proc.stdin.end();
    this.proc.kill();
  }
}
