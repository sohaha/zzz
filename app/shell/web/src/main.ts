import '@xterm/xterm/css/xterm.css';
import './types'; // Import for Wake Lock API type declarations
import { api } from './api';
import { TerminalManager } from './terminal';
import { VirtualKeyboard } from './keyboard';
import { LoginTerminal } from './login-terminal';
import './debug';  // Initialize debug panel if ?debug=1

class App {
  private terminal: TerminalManager | null = null;
  private keyboard: VirtualKeyboard | null = null;
  private loginTerminal: LoginTerminal | null = null;
  private wakeLock: WakeLockSentinel | null = null;

  async init() {
    // Check if already logged in
    const token = api.getToken();
    if (token) {
      try {
        // Try to list sessions to verify token is valid
        await api.listSessions();
        this.showTerminalView();
        return;
      } catch {
        // Token invalid, show login
        api.setToken(null);
      }
    }
    this.showLoginView();
  }

  private showLoginView() {
    // Dispose any existing login terminal
    if (this.loginTerminal) {
      this.loginTerminal.dispose();
      this.loginTerminal = null;
    }
    
    const app = document.getElementById('app')!;
    app.innerHTML = `
      <div class="login-terminal-container">
        <div id="login-terminal-area"></div>
      </div>
    `;

    const loginArea = document.getElementById('login-terminal-area')!;
    this.loginTerminal = new LoginTerminal(loginArea, {
      onLogin: () => {
        // Dispose login terminal before showing main terminal
        if (this.loginTerminal) {
          this.loginTerminal.dispose();
          this.loginTerminal = null;
        }
        this.showTerminalView();
      }
    });
    
    // Focus the login terminal
    this.loginTerminal.focus();
  }

  private async showTerminalView() {
    const app = document.getElementById('app')!;
    // Note: keyboard-area is outside terminal-container so it won't move
    // when the terminal is translated up for mobile keyboard
    app.innerHTML = `
      <div class="terminal-container">
        <div id="terminal-area"></div>
      </div>
      <div id="keyboard-area"></div>
    `;

    const terminalArea = document.getElementById('terminal-area')!;
    const keyboardArea = document.getElementById('keyboard-area')!;

    this.terminal = new TerminalManager(terminalArea);
    this.keyboard = new VirtualKeyboard(keyboardArea, this.terminal, () => this.logout());

    // Create or get session
    try {
      const sessions = await api.listSessions();
      let sessionId: string;

      if (sessions.length > 0) {
        sessionId = sessions[0].id;
      } else {
        const session = await api.createSession();
        sessionId = session.id;
      }

      await this.terminal.connect(sessionId);
      this.terminal.focus();

      // Request wake lock to keep screen on
      await this.requestWakeLock();
    } catch (err) {
      console.error('Failed to connect:', err);
      api.setToken(null);
      this.showLoginView();
      return;
    }

    // Logout handler moved to keyboard
  }

  private async logout() {
    await this.releaseWakeLock();
    this.terminal?.dispose();
    this.terminal = null;
    this.keyboard?.dispose();
    this.keyboard = null;
    await api.logout();
    this.showLoginView();
  }

  private async requestWakeLock() {
    if (!('wakeLock' in navigator)) {
      console.log('Wake Lock API not supported');
      return;
    }

    try {
      this.wakeLock = await navigator.wakeLock.request('screen');
      console.log('Wake Lock acquired');

      // Re-acquire wake lock when page becomes visible again
      document.addEventListener('visibilitychange', this.handleVisibilityChange);
    } catch (err) {
      console.error('Failed to acquire Wake Lock:', err);
    }
  }

  private handleVisibilityChange = async () => {
    if (document.visibilityState === 'visible' && this.terminal) {
      await this.requestWakeLock();
    }
  };

  private async releaseWakeLock() {
    document.removeEventListener('visibilitychange', this.handleVisibilityChange);
    if (this.wakeLock) {
      await this.wakeLock.release();
      this.wakeLock = null;
      console.log('Wake Lock released');
    }
  }
}

// Start app
const app = new App();
app.init();
