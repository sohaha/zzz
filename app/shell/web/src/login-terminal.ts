import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebglAddon } from '@xterm/addon-webgl';
import { api } from './api';

// ASCII art logo - one word per line for mobile compatibility
const ASCII_LOGO_LINES = [
  '\x1b[36m ____            _        _   ',
  '|  _ \\ ___   ___| | _____| |_ ',
  '| |_) / _ \\ / __| |/ / _ \\ __|',
  '|  __/ (_) | (__|   <  __/ |_ ',
  '|_|   \\___/ \\___|_|\\_\\___|\\__|',
  '',
  ' ____  _          _ _ ',
  '/ ___|| |__   ___| | |',
  '\\___ \\| \'_ \\ / _ \\ | |',
  ' ___) | | | |  __/ | |',
  '|____/|_| |_|\\___|_|_|\x1b[0m',
];

const TAGLINE = '\x1b[33m  Mobile Web Terminal\x1b[0m';

interface LoginTerminalOptions {
  onLogin: () => void;
}

// Debounce helper
function debounce<T extends (...args: unknown[]) => void>(fn: T, delay: number): T {
  let timeoutId: ReturnType<typeof setTimeout>;
  return ((...args: unknown[]) => {
    clearTimeout(timeoutId);
    timeoutId = setTimeout(() => fn(...args), delay);
  }) as T;
}

export class LoginTerminal {
  private terminal: Terminal;
  private fitAddon: FitAddon;
  private webglAddon: WebglAddon | null = null;
  private container: HTMLElement;
  private onLogin: () => void;
  private disposed = false;
  
  // Input state
  private username = '';
  private password = '';
  private inputMode: 'username' | 'password' = 'username';
  private isLoggingIn = false;
  
  // For cleanup
  private resizeObserver: ResizeObserver | null = null;
  private debouncedFit: (() => void) | null = null;

  constructor(container: HTMLElement, options: LoginTerminalOptions) {
    this.container = container;
    this.onLogin = options.onLogin;
    
    this.terminal = new Terminal({
      cursorBlink: true,
      fontSize: 12,
      fontFamily: 'Menlo, Monaco, "Courier New", monospace',
      scrollback: 100,
      scrollOnUserInput: true,
      overviewRulerWidth: 0,
      allowProposedApi: true,
      cursorStyle: 'underline',
      theme: {
        background: '#1a1a2e',
        foreground: '#eaeaea',
        cursor: '#4ecdc4',
        cursorAccent: '#1a1a2e',
        selectionBackground: '#3a3a5e',
        scrollbarSliderBackground: 'transparent',
        scrollbarSliderHoverBackground: 'transparent',
        scrollbarSliderActiveBackground: 'transparent',
        black: '#1a1a2e',
        red: '#ff6b6b',
        green: '#4ecdc4',
        yellow: '#ffe66d',
        blue: '#4a90d9',
        magenta: '#c56cf0',
        cyan: '#7bed9f',
        white: '#eaeaea',
        brightBlack: '#666666',
        brightRed: '#ff8787',
        brightGreen: '#6ee7de',
        brightYellow: '#ffed8a',
        brightBlue: '#6aa9e9',
        brightMagenta: '#d98bf0',
        brightCyan: '#98f5c6',
        brightWhite: '#ffffff',
      },
    });
    
    this.fitAddon = new FitAddon();
    this.terminal.loadAddon(this.fitAddon);
    this.terminal.open(container);
    
    // Load WebGL addon for GPU acceleration
    this.initWebGL();
    
    this.fit();
    
    // Handle resize with debounce
    this.debouncedFit = debounce(() => this.fit(), 100);
    this.resizeObserver = new ResizeObserver(this.debouncedFit);
    this.resizeObserver.observe(container);
    window.addEventListener('resize', this.debouncedFit);
    
    // Setup IME handling (same as TerminalManager)
    this.setupIMEHandling(container);
    
    // Show welcome screen
    this.showWelcome();
  }
  
  // Setup IME handling for mobile input methods
  private setupIMEHandling(container: HTMLElement) {
    const xtermTextarea = container.querySelector('.xterm-helper-textarea') as HTMLTextAreaElement | null;
    let isComposing = false;
    let onDataCounter = 0;

    const processInputData = (data: string) => {
      if (!data) return;
      const fixedData = data.replace(/\u00A0/g, ' ');
      this.processInput(fixedData);
    };

    if (xtermTextarea) {
      // Track composition state for IME
      xtermTextarea.addEventListener('compositionstart', () => {
        isComposing = true;
      });

      xtermTextarea.addEventListener('compositionend', () => {
        isComposing = false;
      });

      xtermTextarea.addEventListener('beforeinput', (e) => {
        const inputEvent = e as InputEvent;
        const data = inputEvent.data;
        const inputType = inputEvent.inputType;
        if (!data || !inputType || !inputType.startsWith('insert')) {
          return;
        }

        const token = onDataCounter;
        queueMicrotask(() => {
          if (onDataCounter === token && !isComposing) {
            processInputData(data);
          }
        });
      });
    }

    // Handle direct terminal input (committed input)
    this.terminal.onData((data) => {
      onDataCounter += 1;
      processInputData(data);
    });
  }
  
  // Process input characters (from both direct input and IME)
  private processInput(data: string) {
    if (this.isLoggingIn) return;
    
    for (const char of data) {
      if (char === '\r' || char === '\n') {
        this.handleEnter();
      } else if (char === '\x7f' || char === '\b') {
        this.handleBackspace();
      } else if (char === '\x03') {
        // Ctrl+C - reset
        this.terminal.write('^C\r\n');
        this.showUsernamePrompt();
      } else if (char === '\x15') {
        // Ctrl+U - clear line
        this.clearCurrentInput();
      } else if (char.charCodeAt(0) >= 32) {
        // Any printable character (including unicode)
        this.handleChar(char);
      }
    }
  }
  
  private initWebGL() {
    if (this.disposed || this.webglAddon) return;
    
    try {
      this.webglAddon = new WebglAddon();
      this.webglAddon.onContextLoss(() => {
        this.webglAddon?.dispose();
        this.webglAddon = null;
        if (!this.disposed) {
          setTimeout(() => this.initWebGL(), 1000);
        }
      });
      this.terminal.loadAddon(this.webglAddon);
    } catch {
      this.webglAddon = null;
    }
  }
  
  private fit() {
    if (this.disposed) return;
    try {
      this.fitAddon.fit();
    } catch {
      // Ignore fit errors during disposal
    }
  }
  
  private showWelcome() {
    this.terminal.clear();
    // Write logo line by line
    for (const line of ASCII_LOGO_LINES) {
      this.terminal.writeln(line);
    }
    this.terminal.writeln('');
    this.terminal.writeln(TAGLINE);
    this.terminal.writeln('');
    this.showUsernamePrompt();
  }
  
  private showUsernamePrompt() {
    this.inputMode = 'username';
    this.username = '';
    this.terminal.write('\x1b[32mUsername:\x1b[0m ');
  }
  
  private showPasswordPrompt() {
    this.inputMode = 'password';
    this.password = '';
    this.terminal.write('\x1b[32mPassword:\x1b[0m ');
  }
  
  private handleChar(char: string) {
    if (this.inputMode === 'username') {
      this.username += char;
      this.terminal.write(char);
    } else {
      this.password += char;
      this.terminal.write('*');
    }
  }
  
  private handleBackspace() {
    if (this.inputMode === 'username' && this.username.length > 0) {
      this.username = this.username.slice(0, -1);
      this.terminal.write('\b \b');
    } else if (this.inputMode === 'password' && this.password.length > 0) {
      this.password = this.password.slice(0, -1);
      this.terminal.write('\b \b');
    }
  }
  
  private clearCurrentInput() {
    const len = this.inputMode === 'username' ? this.username.length : this.password.length;
    for (let i = 0; i < len; i++) {
      this.terminal.write('\b \b');
    }
    if (this.inputMode === 'username') {
      this.username = '';
    } else {
      this.password = '';
    }
  }
  
  private handleEnter() {
    this.terminal.write('\r\n');
    
    if (this.inputMode === 'username') {
      if (this.username.trim()) {
        this.showPasswordPrompt();
      } else {
        this.showUsernamePrompt();
      }
    } else {
      this.attemptLogin();
    }
  }
  
  private async attemptLogin() {
    if (this.isLoggingIn) return;
    this.isLoggingIn = true;
    
    this.terminal.write('\x1b[90mAuthenticating...\x1b[0m\r\n');
    
    try {
      await api.login(this.username, this.password);
      this.terminal.write('\x1b[32mLogin successful!\x1b[0m\r\n');
      
      setTimeout(() => {
        this.onLogin();
      }, 500);
    } catch (err) {
      this.isLoggingIn = false;
      const message = err instanceof Error ? err.message : 'Authentication failed';
      this.terminal.write(`\x1b[31mError: ${message}\x1b[0m\r\n\r\n`);
      this.showUsernamePrompt();
    }
  }
  
  focus() {
    this.terminal.focus();
  }
  
  dispose() {
    if (this.disposed) return;
    this.disposed = true;
    
    if (this.resizeObserver) {
      this.resizeObserver.disconnect();
      this.resizeObserver = null;
    }
    
    if (this.debouncedFit) {
      window.removeEventListener('resize', this.debouncedFit);
      this.debouncedFit = null;
    }
    
    if (this.webglAddon) {
      this.webglAddon.dispose();
      this.webglAddon = null;
    }
    
    this.terminal.dispose();
  }
}
