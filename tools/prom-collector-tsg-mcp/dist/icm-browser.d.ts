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
/** Scrape the ICM incident summary page */
export declare function scrapeICMIncident(incidentId: number): Promise<string>;
//# sourceMappingURL=icm-browser.d.ts.map