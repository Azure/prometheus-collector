/**
 * ICM Browser Scraper — connects to a running Edge instance via CDP
 * to scrape ICM portal pages for incident details not available via API.
 *
 * Works on both **Windows (native)** and **WSL2**:
 *
 * **Windows:** Edge connects directly on localhost:9222.
 *   Launch Edge:
 *     Start-Process 'C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe' `
 *       -ArgumentList '--remote-debugging-port=9222','--user-data-dir=C:\Users\<user>\.edge-cdp-debug','--no-first-run'
 *
 * **WSL2:** Edge runs on the Windows host; WSL reaches it via netsh port proxy on 9223.
 *   1. Launch Edge on Windows with --remote-debugging-port=9222
 *   2. Set up port proxy: netsh interface portproxy add v4tov4 listenport=9223 listenaddress=0.0.0.0 connectport=9222 connectaddress=127.0.0.1
 *   3. User authenticated to ICM in that Edge instance
 */

import WebSocket from "ws";
import { platform } from "os";
import { execSync } from "child_process";

/** Auto-detect the WSL2 default gateway IP (the Windows host) */
function detectWSLGateway(): string {
  try {
    const route = execSync("ip route show default 2>/dev/null", {
      encoding: "utf-8",
      timeout: 3000,
    });
    const match = route.match(/via\s+([\d.]+)/);
    if (match) return match[1];
  } catch {
    // not on WSL2 or ip command unavailable
  }
  return "172.29.112.1"; // fallback
}

/** Auto-detect the CDP endpoint based on platform (overridable via CDP_ENDPOINT env var) */
function getDefaultCdpEndpoint(): string {
  if (platform() === "win32") {
    return "http://localhost:9222";
  }
  // WSL2 / Linux: reach the Windows host Edge via port proxy, auto-detect gateway
  return `http://${detectWSLGateway()}:9223`;
}

const CDP_HTTP = process.env.CDP_ENDPOINT || getDefaultCdpEndpoint();

interface CDPTab {
  id: string;
  type: string;
  title: string;
  url: string;
  webSocketDebuggerUrl: string;
}

async function fetchJSON(url: string): Promise<any> {
  const resp = await fetch(url);
  if (!resp.ok) throw new Error(`HTTP ${resp.status} from ${url}`);
  return resp.json();
}

/** Open a persistent WebSocket to a tab and send multiple commands */
class CDPSession {
  private ws: WebSocket;
  private msgId = 1;
  private pending = new Map<number, (msg: any) => void>();
  private eventHandlers = new Map<string, (params: any) => void>();

  private constructor(ws: WebSocket) {
    this.ws = ws;
    ws.on("message", (data: WebSocket.RawData) => {
      const msg = JSON.parse(data.toString());
      if (msg.id && this.pending.has(msg.id)) {
        this.pending.get(msg.id)!(msg);
        this.pending.delete(msg.id);
      }
      if (msg.method && this.eventHandlers.has(msg.method)) {
        this.eventHandlers.get(msg.method)!(msg.params);
      }
    });
  }

  static async connect(wsUrl: string): Promise<CDPSession> {
    const ws = new WebSocket(wsUrl);
    await new Promise<void>((resolve, reject) => {
      ws.on("open", resolve);
      ws.on("error", reject);
    });
    return new CDPSession(ws);
  }

  on(event: string, handler: (params: any) => void): void {
    this.eventHandlers.set(event, handler);
  }

  send(method: string, params: Record<string, any> = {}, timeoutMs = 15000): Promise<any> {
    return new Promise((resolve, reject) => {
      const id = this.msgId++;
      const timer = setTimeout(() => {
        this.pending.delete(id);
        resolve(null); // don't reject — some responses just time out
      }, timeoutMs);
      this.pending.set(id, (msg) => {
        clearTimeout(timer);
        if (msg.error) reject(new Error(msg.error.message));
        else resolve(msg.result);
      });
      this.ws.send(JSON.stringify({ id, method, params }));
    });
  }

  async eval(expression: string): Promise<string> {
    const result = await this.send("Runtime.evaluate", {
      expression,
      returnByValue: true,
    });
    return result?.result?.value ?? "";
  }

  close(): void {
    this.ws.close();
  }
}

/** List open tabs in the Edge instance */
async function listTabs(): Promise<CDPTab[]> {
  return fetchJSON(`${CDP_HTTP}/json/list`);
}

/** Find or open an ICM incident page, return the tab's WS URL */
async function getOrOpenICMTab(
  incidentId: number
): Promise<{ wsUrl: string; alreadyOpen: boolean }> {
  const icmUrl = `https://portal.microsofticm.com/imp/v5/incidents/details/${incidentId}/summary`;

  // Check if already open
  const tabs = await listTabs();
  for (const tab of tabs) {
    if (tab.type === "page" && tab.url.includes(`${incidentId}`)) {
      return { wsUrl: tab.webSocketDebuggerUrl, alreadyOpen: true };
    }
  }

  // Open a new tab via CDP browser-level WS
  const version = await fetchJSON(`${CDP_HTTP}/json/version`);
  const browserWs = version.webSocketDebuggerUrl as string;
  const session = await CDPSession.connect(browserWs);
  const result = await session.send("Target.createTarget", { url: icmUrl });
  session.close();
  const targetId = result.targetId;

  // Wait for tab to load
  await new Promise((r) => setTimeout(r, 8000));

  const updatedTabs = await listTabs();
  for (const tab of updatedTabs) {
    if (tab.id === targetId || tab.url.includes(`${incidentId}`)) {
      return { wsUrl: tab.webSocketDebuggerUrl, alreadyOpen: false };
    }
  }

  throw new Error(
    `Could not find ICM tab for incident ${incidentId} after opening`
  );
}

interface DiscussionEntry {
  by: string;
  date: string;
  text: string;
}

interface ICMPageData {
  summary: string;
  description: string;
  discussions: DiscussionEntry[];
}

/**
 * Reload the ICM page and capture API responses via CDP Network domain:
 * - GetIncidentDetails → Summary (authored summary) + Description fields
 * - getdescriptionentries → Discussion entries (Items array)
 */
async function captureIncidentData(
  session: CDPSession,
  incidentId: number
): Promise<ICMPageData | null> {
  const capturedRequests: { requestId: string; url: string }[] = [];

  session.on("Network.responseReceived", (params: any) => {
    const url = params?.response?.url || "";
    if (url.includes("GetIncidentDetails") || url.includes("getdescriptionentries")) {
      capturedRequests.push({ requestId: params.requestId, url });
    }
  });

  await session.send("Network.enable", {
    maxTotalBufferSize: 50_000_000,
    maxResourceBufferSize: 10_000_000,
  });
  await session.send("Page.reload", { ignoreCache: true });

  // Wait for API responses
  await new Promise((r) => setTimeout(r, 12000));

  let summary = "";
  let description = "";
  const discussions: DiscussionEntry[] = [];

  for (const req of capturedRequests) {
    try {
      const bodyResult = await session.send("Network.getResponseBody", {
        requestId: req.requestId,
      });
      if (!bodyResult?.body) continue;

      const data = JSON.parse(bodyResult.body);

      if (req.url.includes("GetIncidentDetails")) {
        summary = summary || data.Summary || "";
        description = description || data.Description || "";
      }

      if (req.url.includes("getdescriptionentries") && discussions.length === 0) {
        const items = data.Items || [];
        for (const item of items) {
          const text = item.Text || "";
          if (!text) continue;
          discussions.push({
            by: item.SubmittedByDisplayName || item.SubmittedBy || "",
            date: item.SubmitDate || item.Date || "",
            text: stripHtml(text),
          });
        }
      }
    } catch {
      // body may have been evicted from cache
    }
  }

  if (!summary && !description && discussions.length === 0) return null;
  return { summary, description, discussions };
}

/** Strip HTML tags and decode common entities */
function stripHtml(html: string): string {
  return html
    .replace(/<[^>]*>/g, "")
    .replace(/&nbsp;/g, " ")
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&#\d+;/g, "")
    .replace(/\s+/g, " ")
    .trim();
}

/** Scrape the ICM incident summary page */
export async function scrapeICMIncident(
  incidentId: number
): Promise<string> {
  try {
    await fetchJSON(`${CDP_HTTP}/json/version`);
  } catch {
    const isWindows = platform() === "win32";
    const instructions = isWindows
      ? [
          "❌ Cannot connect to Edge browser via CDP.",
          "",
          "To use this tool on Windows, launch Edge with remote debugging enabled:",
          "",
          '  Start-Process "C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe" -ArgumentList "--remote-debugging-port=9222","--user-data-dir=C:\\Users\\<user>\\.edge-cdp-debug","--no-first-run"',
          "",
          `Then navigate to https://portal.microsofticm.com/imp/v5/incidents/details/${incidentId}/summary and sign in if needed.`,
        ]
      : [
          "❌ Cannot connect to Edge browser via CDP.",
          "",
          "To use this tool from WSL2, launch Edge on the Windows host with remote debugging:",
          "",
          '  powershell.exe -Command "Start-Process \'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe\' -ArgumentList \'--remote-debugging-port=9222\',\'--user-data-dir=C:\\Users\\<user>\\.edge-cdp-debug\',\'--no-first-run\'"',
          "",
          "Ensure the Windows port proxy is set up:",
          "  netsh interface portproxy add v4tov4 listenport=9223 listenaddress=0.0.0.0 connectport=9222 connectaddress=127.0.0.1",
          "",
          `Then navigate to https://portal.microsofticm.com/imp/v5/incidents/details/${incidentId}/summary and sign in if needed.`,
        ];
    return instructions.join("\n");
  }

  try {
    const { wsUrl, alreadyOpen } = await getOrOpenICMTab(incidentId);
    const session = await CDPSession.connect(wsUrl);

    if (!alreadyOpen) {
      await new Promise((r) => setTimeout(r, 5000));
    }

    // Check if we landed on a sign-in page
    const currentUrl = await session.eval("window.location.href");
    if (
      currentUrl.includes("login.microsoftonline.com") ||
      currentUrl.includes("login.live.com")
    ) {
      session.close();
      return [
        "⚠️ Edge is on a sign-in page. Please sign in to ICM in the browser, then retry this tool.",
        `Current URL: ${currentUrl}`,
      ].join("\n");
    }

    // Capture authored summary + discussion entries via Network interception.
    // The ICM "Authored summary" is the `Summary` field in GetIncidentDetails.
    // Discussion entries come from the getdescriptionentries API.
    // Neither is in the DOM innerText — ICM's React UI lazy-renders them.
    const details = await captureIncidentData(session, incidentId);

    // Also get the visible page text for context (title, status, etc.)
    const pageText = await session.eval("document.body.innerText");
    session.close();

    const lines: string[] = [
      `### ICM ${incidentId} — Browser Scrape`,
      "",
      `**URL**: https://portal.microsofticm.com/imp/v5/incidents/details/${incidentId}/summary`,
      "",
    ];

    if (details?.summary) {
      const plainSummary = stripHtml(details.summary);
      lines.push("#### Authored Summary");
      lines.push(plainSummary);
      lines.push("");

      // Extract ARM resource IDs from authored summary
      const armIds =
        details.summary.match(
          /\/subscriptions\/[a-f0-9-]+\/resource[Gg]roups\/[^"'<>\s&;]+/gi
        ) || [];
      if (armIds.length > 0) {
        lines.push("#### Extracted ARM Resource IDs");
        const unique = [...new Set(armIds.map((m) => m.replace(/&amp;/g, "&")))];
        unique.forEach((id) => lines.push(`- \`${id}\``));
        lines.push("");
      }
    }

    if (details?.discussions && details.discussions.length > 0) {
      lines.push(`#### Discussion (${details.discussions.length} entries)`);
      for (const d of details.discussions) {
        const dateStr = d.date ? new Date(d.date).toISOString().split("T")[0] : "";
        lines.push(`**${d.by}** (${dateStr}):`);
        // Truncate very long entries but keep enough context
        lines.push(d.text.length > 500 ? d.text.substring(0, 500) + "..." : d.text);
        lines.push("");
      }
    }

    if (pageText && pageText.length > 100) {
      lines.push("#### Page Header");
      lines.push(pageText.substring(0, 1500));
    } else if (!details?.summary) {
      lines.push(
        "⚠️ Could not extract authored summary or page content. The page may still be loading — retry in a few seconds."
      );
    }

    return lines.join("\n");
  } catch (err: any) {
    return `❌ Error scraping ICM ${incidentId}: ${err.message}`;
  }
}
