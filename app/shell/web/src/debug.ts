// Debug panel for mobile debugging
// Enable by adding ?debug=1 to URL
// Filter by category: ?debug=1&filter=terminal,keyboard (comma-separated)
// Exclude categories: ?debug=1&exclude=mouse,swipe

// Known debug categories (for reference):
// - terminal: Terminal operations
// - keyboard: Virtual keyboard events
// - swipe: Swipe gesture detection
// - mouse: Mouse/touch coordinate tracking
// - ws: WebSocket messages
// - api: API calls

type LogLevel = 'log' | 'info' | 'warn' | 'error';

// Category configuration
interface CategoryConfig {
  enabled: boolean;
  color?: string;
}

class DebugPanel {
  private container: HTMLElement | null = null;
  private logContainer: HTMLElement | null = null;
  private filterContainer: HTMLElement | null = null;
  private maxLogs = 100;
  private isEnabled = false;
  private isMinimized = false;
  
  // Category filtering
  private categories: Map<string, CategoryConfig> = new Map();
  private includeFilter: Set<string> | null = null; // null = include all
  private excludeFilter: Set<string> = new Set();
  private showFilterPanel = false;

  // Category colors for visual distinction
  private categoryColors: Record<string, string> = {
    terminal: '#4fc3f7',
    keyboard: '#81c784',
    swipe: '#ffb74d',
    mouse: '#f06292',
    ws: '#ba68c8',
    api: '#4db6ac',
  };

  constructor() {
    // Check if debug mode is enabled via URL parameter
    const params = new URLSearchParams(window.location.search);
    this.isEnabled = params.get('debug') === '1';

    if (this.isEnabled) {
      // Parse filter parameters
      const filterParam = params.get('filter');
      if (filterParam) {
        this.includeFilter = new Set(filterParam.split(',').map(s => s.trim().toLowerCase()));
      }

      const excludeParam = params.get('exclude');
      if (excludeParam) {
        this.excludeFilter = new Set(excludeParam.split(',').map(s => s.trim().toLowerCase()));
      }

      this.init();
      this.interceptConsole();
    }
  }

  private init() {
    // Create debug panel container
    this.container = document.createElement('div');
    this.container.id = 'debug-panel';
    this.container.innerHTML = `
      <div class="debug-header">
        <span class="debug-title">Debug</span>
        <div class="debug-actions">
          <button class="debug-btn" id="debug-filter">Filter</button>
          <button class="debug-btn" id="debug-clear">Clear</button>
          <button class="debug-btn" id="debug-toggle">_</button>
        </div>
      </div>
      <div class="debug-filter-panel" id="debug-filter-panel"></div>
      <div class="debug-logs" id="debug-logs"></div>
    `;

    // Add styles
    const style = document.createElement('style');
    style.textContent = `
      #debug-panel {
        position: fixed;
        bottom: 0;
        left: 0;
        right: 0;
        max-height: 40vh;
        background: rgba(0, 0, 0, 0.9);
        border-top: 1px solid #444;
        z-index: 10000;
        font-family: monospace;
        font-size: 11px;
        display: flex;
        flex-direction: column;
      }
      #debug-panel.minimized {
        max-height: none;
      }
      #debug-panel.minimized .debug-logs,
      #debug-panel.minimized .debug-filter-panel {
        display: none;
      }
      .debug-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 4px 8px;
        background: #333;
        border-bottom: 1px solid #444;
        flex-shrink: 0;
      }
      .debug-title {
        color: #0f0;
        font-weight: bold;
      }
      .debug-actions {
        display: flex;
        gap: 4px;
      }
      .debug-btn {
        background: #555;
        border: none;
        color: #fff;
        padding: 2px 8px;
        border-radius: 3px;
        cursor: pointer;
        font-size: 11px;
      }
      .debug-btn:active {
        background: #777;
      }
      .debug-btn.active {
        background: #0a0;
      }
      .debug-filter-panel {
        display: none;
        padding: 8px;
        background: #2a2a2a;
        border-bottom: 1px solid #444;
        flex-wrap: wrap;
        gap: 6px;
      }
      .debug-filter-panel.show {
        display: flex;
      }
      .debug-filter-chip {
        display: inline-flex;
        align-items: center;
        padding: 3px 8px;
        border-radius: 12px;
        font-size: 10px;
        cursor: pointer;
        border: 1px solid #555;
        background: #333;
        color: #888;
        transition: all 0.2s;
      }
      .debug-filter-chip.enabled {
        background: #1a3a1a;
        border-color: #4a4;
        color: #8f8;
      }
      .debug-filter-chip .chip-dot {
        width: 6px;
        height: 6px;
        border-radius: 50%;
        margin-right: 4px;
      }
      .debug-logs {
        overflow-y: auto;
        padding: 4px 8px;
        flex: 1;
        min-height: 0;
      }
      .debug-log {
        padding: 2px 0;
        border-bottom: 1px solid #333;
        word-break: break-all;
        white-space: pre-wrap;
      }
      .debug-log.log { color: #fff; }
      .debug-log.info { color: #0af; }
      .debug-log.warn { color: #fa0; }
      .debug-log.error { color: #f55; }
      .debug-log.filtered { display: none; }
      .debug-log .time {
        color: #888;
        margin-right: 8px;
      }
      .debug-log .category {
        font-weight: bold;
        margin-right: 4px;
      }
    `;
    document.head.appendChild(style);
    document.body.appendChild(this.container);

    this.logContainer = document.getElementById('debug-logs');
    this.filterContainer = document.getElementById('debug-filter-panel');

    // Event listeners
    document.getElementById('debug-filter')?.addEventListener('click', () => this.toggleFilterPanel());
    document.getElementById('debug-clear')?.addEventListener('click', () => this.clear());
    document.getElementById('debug-toggle')?.addEventListener('click', () => this.toggle());
  }

  private interceptConsole() {
    const originalLog = console.log;
    const originalInfo = console.info;
    const originalWarn = console.warn;
    const originalError = console.error;

    console.log = (...args) => {
      this.addLog('log', args);
      originalLog.apply(console, args);
    };

    console.info = (...args) => {
      this.addLog('info', args);
      originalInfo.apply(console, args);
    };

    console.warn = (...args) => {
      this.addLog('warn', args);
      originalWarn.apply(console, args);
    };

    console.error = (...args) => {
      this.addLog('error', args);
      originalError.apply(console, args);
    };
  }

  private formatArg(arg: unknown): string {
    if (typeof arg === 'string') {
      return arg;
    }
    try {
      return JSON.stringify(arg);
    } catch {
      return String(arg);
    }
  }

  // Extract category from log message (e.g., "[Terminal]" -> "terminal")
  private extractCategory(args: unknown[]): string | null {
    if (args.length > 0 && typeof args[0] === 'string') {
      const match = args[0].match(/^\[([^\]]+)\]/);
      if (match) {
        return match[1].toLowerCase();
      }
    }
    return null;
  }

  private isCategoryEnabled(category: string | null): boolean {
    if (category === null) {
      return true; // Always show uncategorized logs
    }

    // Check exclude filter first
    if (this.excludeFilter.has(category)) {
      return false;
    }

    // Check include filter
    if (this.includeFilter !== null) {
      return this.includeFilter.has(category);
    }

    // Check dynamic category toggle
    const config = this.categories.get(category);
    if (config !== undefined) {
      return config.enabled;
    }

    // Default: enabled
    return true;
  }

  private registerCategory(category: string) {
    if (!this.categories.has(category)) {
      // Determine initial state based on filters
      let enabled = true;
      if (this.excludeFilter.has(category)) {
        enabled = false;
      } else if (this.includeFilter !== null) {
        enabled = this.includeFilter.has(category);
      }

      this.categories.set(category, {
        enabled,
        color: this.categoryColors[category] || '#888',
      });
      this.updateFilterPanel();
    }
  }

  private updateFilterPanel() {
    if (!this.filterContainer) return;

    this.filterContainer.innerHTML = '';
    
    // Sort categories alphabetically
    const sortedCategories = Array.from(this.categories.entries()).sort((a, b) => 
      a[0].localeCompare(b[0])
    );

    for (const [category, config] of sortedCategories) {
      const chip = document.createElement('span');
      chip.className = `debug-filter-chip ${config.enabled ? 'enabled' : ''}`;
      chip.innerHTML = `<span class="chip-dot" style="background: ${config.color}"></span>${category}`;
      chip.addEventListener('click', () => this.toggleCategory(category));
      this.filterContainer.appendChild(chip);
    }
  }

  private toggleCategory(category: string) {
    const config = this.categories.get(category);
    if (config) {
      config.enabled = !config.enabled;
      this.categories.set(category, config);
      this.updateFilterPanel();
      this.refilterLogs();
    }
  }

  private refilterLogs() {
    if (!this.logContainer) return;

    const logs = this.logContainer.querySelectorAll('.debug-log');
    logs.forEach((log) => {
      const category = log.getAttribute('data-category');
      const isEnabled = this.isCategoryEnabled(category);
      log.classList.toggle('filtered', !isEnabled);
    });
  }

  private toggleFilterPanel() {
    this.showFilterPanel = !this.showFilterPanel;
    this.filterContainer?.classList.toggle('show', this.showFilterPanel);
    document.getElementById('debug-filter')?.classList.toggle('active', this.showFilterPanel);
  }

  private addLog(level: LogLevel, args: unknown[]) {
    if (!this.logContainer) return;

    const category = this.extractCategory(args);
    
    // Register new categories as they appear
    if (category) {
      this.registerCategory(category);
    }

    const isEnabled = this.isCategoryEnabled(category);

    const time = new Date().toLocaleTimeString('en-US', { 
      hour12: false, 
      hour: '2-digit', 
      minute: '2-digit', 
      second: '2-digit',
      fractionalSecondDigits: 3
    });
    
    const message = args.map(arg => this.formatArg(arg)).join(' ');
    
    const logEntry = document.createElement('div');
    logEntry.className = `debug-log ${level} ${isEnabled ? '' : 'filtered'}`;
    if (category) {
      logEntry.setAttribute('data-category', category);
    }
    
    // Format with category color if present
    let formattedMessage = this.escapeHtml(message);
    if (category) {
      const color = this.categoryColors[category] || '#888';
      formattedMessage = formattedMessage.replace(
        `[${category.charAt(0).toUpperCase() + category.slice(1)}]`,
        `<span class="category" style="color: ${color}">[${category.charAt(0).toUpperCase() + category.slice(1)}]</span>`
      );
    }
    
    logEntry.innerHTML = `<span class="time">${time}</span>${formattedMessage}`;
    
    this.logContainer.appendChild(logEntry);
    
    // Limit number of logs
    while (this.logContainer.children.length > this.maxLogs) {
      this.logContainer.removeChild(this.logContainer.firstChild!);
    }
    
    // Auto scroll to bottom
    this.logContainer.scrollTop = this.logContainer.scrollHeight;
  }

  private escapeHtml(text: string): string {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  private clear() {
    if (this.logContainer) {
      this.logContainer.innerHTML = '';
    }
  }

  private toggle() {
    if (this.container) {
      this.isMinimized = !this.isMinimized;
      this.container.classList.toggle('minimized', this.isMinimized);
      const btn = document.getElementById('debug-toggle');
      if (btn) {
        btn.textContent = this.isMinimized ? '+' : '_';
      }
    }
  }

  // Public API for programmatic control
  public enableCategory(category: string) {
    const config = this.categories.get(category);
    if (config) {
      config.enabled = true;
      this.updateFilterPanel();
      this.refilterLogs();
    }
  }

  public disableCategory(category: string) {
    const config = this.categories.get(category);
    if (config) {
      config.enabled = false;
      this.updateFilterPanel();
      this.refilterLogs();
    }
  }

  public getCategories(): string[] {
    return Array.from(this.categories.keys());
  }
}

// Initialize debug panel
export const debugPanel = new DebugPanel();
