import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebLinksAddon } from '@xterm/addon-web-links';
import { WebglAddon } from '@xterm/addon-webgl';
import { api } from './api';
import type { WSMessage } from './types';
import { SwipeArrowController } from './swipe-arrow';

// Debug mode - only log when ?debug=1 is in URL
const DEBUG = typeof window !== 'undefined' && window.location.search.includes('debug=1');

function debugLog(...args: unknown[]) {
  if (DEBUG) {
    console.log('[Terminal]', ...args);
  }
}

// Debounce helper
function debounce<T extends (...args: unknown[]) => void>(fn: T, delay: number): T {
  let timeoutId: ReturnType<typeof setTimeout>;
  return ((...args: unknown[]) => {
    clearTimeout(timeoutId);
    timeoutId = setTimeout(() => fn(...args), delay);
  }) as T;
}

// Module-level cache for font size to avoid repeated localStorage access
const FONT_SIZE_KEY = 'pocket-shell-font-size';
let cachedFontSize: number | null = null;

function getCachedFontSize(minSize: number, maxSize: number, defaultSize: number): number {
  if (cachedFontSize !== null) {
    return cachedFontSize;
  }
  try {
    const saved = localStorage.getItem(FONT_SIZE_KEY);
    if (saved) {
      const parsed = parseInt(saved, 10);
      if (!isNaN(parsed) && parsed >= minSize && parsed <= maxSize) {
        cachedFontSize = parsed;
        return cachedFontSize;
      }
    }
  } catch {
    // localStorage might be unavailable
  }
  cachedFontSize = defaultSize;
  return cachedFontSize;
}

function setCachedFontSize(size: number): void {
  cachedFontSize = size;
  try {
    localStorage.setItem(FONT_SIZE_KEY, size.toString());
  } catch {
    // localStorage might be unavailable
  }
}

export class TerminalManager {
  private terminal: Terminal;
  private fitAddon: FitAddon;
  private webglAddon: WebglAddon | null = null;
  private ws: WebSocket | null = null;
  private sessionId: string | null = null;
  private container: HTMLElement;
  private pingInterval: ReturnType<typeof setInterval> | null = null;
  private lastRows = 0;
  private lastCols = 0;
  private inputInterceptor: ((data: string) => string | null) | null = null;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectDelay = 1000;
  private isReconnecting = false;
  private fontSize = 12;
  private readonly minFontSize = 8;
  private readonly maxFontSize = 32;
  
  // For cleanup
  private resizeObserver: ResizeObserver | null = null;
  private debouncedFit: (() => void) | null = null;
  private disposed = false;
  
  // Viewport resize handler for mobile keyboard
  private viewportResizeHandler: (() => void) | null = null;
  
  // Keyboard lock - prevents soft keyboard from appearing
  private keyboardLocked = false;
  
  // Selection mode - allows finger selection on mobile
  private selectionMode = false;
  
  // Touch selection state
  private touchSelectionStart: { col: number; row: number } | null = null;
  private touchSelectionActive = false;
  
  // Swipe arrow controller for long-press arrow key gestures
  private swipeArrowController: SwipeArrowController | null = null;
  
  // Message batching for performance
  private outputBuffer = '';
  private outputFlushScheduled = false;
  private inputBuffer = '';
  private inputFlushScheduled = false;

  constructor(container: HTMLElement) {
    // Load saved font size from cache
    this.fontSize = getCachedFontSize(this.minFontSize, this.maxFontSize, 12);
    this.container = container;
    this.terminal = new Terminal({
      cursorBlink: true,
      fontSize: this.fontSize,
      fontFamily: 'Menlo, Monaco, "Courier New", monospace',
      scrollback: 1000,
      // Disable scrollbar to prevent black border and scrollbar visibility
      scrollOnUserInput: true,
      overviewRulerWidth: 0,
      // Improve IME support for third-party input methods
      allowProposedApi: true,
      theme: {
        background: '#1a1a2e',
        foreground: '#eaeaea',
        cursor: '#eaeaea',
        cursorAccent: '#1a1a2e',
        selectionBackground: '#3a3a5e',
        // Hide scrollbar by making it transparent
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
    this.terminal.loadAddon(new WebLinksAddon());

    this.terminal.open(container);
    
    // Load WebGL addon for GPU acceleration (must be after terminal.open)
    this.initWebGL();
    
    this.fit();

    // IME handling aligned with xterm composition flow
    const xtermTextarea = container.querySelector('.xterm-helper-textarea') as HTMLTextAreaElement | null;
    let isComposing = false;
    let lastOnDataValue: string | null = null;  // Track the last data processed by onData

    const processInputData = (data: string, source: string) => {
      if (!data) return;
      debugLog('[Terminal] processInputData called from:', source, 'data:', JSON.stringify(data));
      // Fix: Replace non-breaking space (U+00A0) with regular space
      let fixedData = data.replace(/\u00A0/g, ' ');
      const processed = this.inputInterceptor ? this.inputInterceptor(fixedData) : fixedData;
      if (processed) {
        this.queueInput(processed);
      }
    };

    if (xtermTextarea) {
      debugLog('[Terminal] Found xterm textarea, wiring IME handlers');

      xtermTextarea.addEventListener('compositionstart', (e) => {
        debugLog('[XtermTextarea] compositionstart:', (e as CompositionEvent).data);
        isComposing = true;
      });

      xtermTextarea.addEventListener('compositionend', (e) => {
        const data = (e as CompositionEvent).data;
        debugLog('[XtermTextarea] compositionend:', data);
        isComposing = false;
      });

      xtermTextarea.addEventListener('beforeinput', (e) => {
        const inputEvent = e as InputEvent;
        debugLog('[XtermTextarea] beforeinput:', JSON.stringify(inputEvent.data), 'inputType:', inputEvent.inputType, 'isComposing:', isComposing, 'lastOnDataValue:', JSON.stringify(lastOnDataValue));

        const data = inputEvent.data;
        const inputType = inputEvent.inputType;
        if (!data || !inputType || !inputType.startsWith('insert')) {
          return;
        }

        // If onData already handled this exact data, skip the fallback
        if (lastOnDataValue === data) {
          debugLog('[XtermTextarea] beforeinput SKIPPED, already handled by onData');
          lastOnDataValue = null;  // Clear for next input
          return;
        }
        
        queueMicrotask(() => {
          debugLog('[XtermTextarea] beforeinput microtask, data:', JSON.stringify(data), 'lastOnDataValue:', JSON.stringify(lastOnDataValue), 'isComposing:', isComposing);
          // Check again in case onData fired between beforeinput and microtask
          if (lastOnDataValue === data) {
            debugLog('[XtermTextarea] beforeinput fallback SKIPPED in microtask, already handled by onData');
            lastOnDataValue = null;
            return;
          }
          if (!isComposing) {
            debugLog('[XtermTextarea] beforeinput fallback send:', JSON.stringify(data));
            processInputData(data, 'beforeinput-fallback');
          }
        });
      });
    } else {
      debugLog('[Terminal] xterm textarea not found');
    }

    // Handle resize with debounce using ResizeObserver
    this.debouncedFit = debounce(() => this.fit(), 100);
    
    // Use ResizeObserver to detect container size changes (including virtual keyboard show/hide)
    this.resizeObserver = new ResizeObserver(this.debouncedFit);
    this.resizeObserver.observe(container);
    
    // Also listen to window resize as fallback
    window.addEventListener('resize', this.debouncedFit);

    // Setup visualViewport listener for mobile keyboard handling
    this.setupViewportResize();

    // Setup pinch-to-zoom for font size adjustment on mobile
    this.setupPinchZoom();

    // Setup swipe arrow controller for long-press arrow key gestures
    this.setupSwipeArrow();

    // Handle input - rely on xterm onData for committed text
    this.terminal.onData((data) => {
      debugLog('[Terminal] onData fired, data:', JSON.stringify(data));
      // Record what onData processed so beforeinput can check for duplicates
      lastOnDataValue = data;
      processInputData(data, 'onData');
      // Don't clear here - let beforeinput check and clear it
    });
  }

  // Initialize WebGL with context loss recovery
  private initWebGL() {
    if (this.disposed || this.webglAddon) return;
    
    try {
      this.webglAddon = new WebglAddon();
      this.webglAddon.onContextLoss(() => {
        debugLog('[Terminal] WebGL context lost, attempting recovery...');
        this.webglAddon?.dispose();
        this.webglAddon = null;
        
        // Attempt to recover WebGL after a delay
        if (!this.disposed) {
          setTimeout(() => this.initWebGL(), 1000);
        }
      });
      this.terminal.loadAddon(this.webglAddon);
      debugLog('[Terminal] WebGL renderer enabled');
    } catch (e) {
      console.warn('[Terminal] WebGL not available, using canvas renderer:', e);
      this.webglAddon = null;
    }
  }

  fit() {
    // Calculate dimensions manually to ignore scrollbar width
    const core = (this.terminal as unknown as { _core: { _renderService: { dimensions: { css: { cell: { width: number; height: number } } } } } })._core;
    const dims = core._renderService.dimensions;
    if (!dims?.css?.cell) {
      this.fitAddon.fit();
      return;
    }
    
    const cellWidth = dims.css.cell.width;
    const cellHeight = dims.css.cell.height;
    
    // Get container dimensions minus padding
    const style = getComputedStyle(this.container.querySelector('.xterm')!);
    const paddingX = parseFloat(style.paddingLeft) + parseFloat(style.paddingRight);
    const paddingY = parseFloat(style.paddingTop) + parseFloat(style.paddingBottom);
    
    const availableWidth = this.container.clientWidth - paddingX;
    const availableHeight = this.container.clientHeight - paddingY;
    
    const newCols = Math.max(2, Math.floor(availableWidth / cellWidth));
    const newRows = Math.max(1, Math.floor(availableHeight / cellHeight));
    
    this.terminal.resize(newCols, newRows);
    // Only send resize if dimensions actually changed
    if (newRows !== this.lastRows || newCols !== this.lastCols) {
      this.lastRows = newRows;
      this.lastCols = newCols;
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.send({
          type: 'resize',
          data: { rows: newRows, cols: newCols },
        });
      }
    }
  }

  // Force a terminal refresh by sending a resize to trigger SIGWINCH on fullscreen apps
  private forceRefresh() {
    const core = (this.terminal as unknown as { _core: { _renderService: { dimensions: { css: { cell: { width: number; height: number } } } } } })._core;
    const dims = core._renderService.dimensions;
    if (!dims?.css?.cell) {
      this.fitAddon.fit();
      return;
    }
    
    const cellWidth = dims.css.cell.width;
    const cellHeight = dims.css.cell.height;
    
    const style = getComputedStyle(this.container.querySelector('.xterm')!);
    const paddingX = parseFloat(style.paddingLeft) + parseFloat(style.paddingRight);
    const paddingY = parseFloat(style.paddingTop) + parseFloat(style.paddingBottom);
    
    const availableWidth = this.container.clientWidth - paddingX;
    const availableHeight = this.container.clientHeight - paddingY;
    
    const newCols = Math.max(2, Math.floor(availableWidth / cellWidth));
    const newRows = Math.max(1, Math.floor(availableHeight / cellHeight));
    
    this.terminal.resize(newCols, newRows);
    this.lastRows = newRows;
    this.lastCols = newCols;
    
    // Send a slightly different size first, then the correct size
    // This triggers a full redraw in fullscreen apps like htop
    this.send({
      type: 'resize',
      data: { rows: Math.max(1, newRows - 1), cols: Math.max(2, newCols - 1) },
    });
    
    // Then send the correct size after a short delay
    setTimeout(() => {
      this.send({
        type: 'resize',
        data: { rows: newRows, cols: newCols },
      });
    }, 50);
  }

  async connect(sessionId: string) {
    if (this.ws) {
      this.disconnect();
    }

    this.sessionId = sessionId;
    this.reconnectAttempts = 0;
    this.isReconnecting = false;
    this.doConnect();
  }

  private doConnect() {
    if (!this.sessionId) return;
    
    this.ws = api.createWebSocket(this.sessionId);

    this.ws.onopen = () => {
      this.reconnectAttempts = 0;
      this.isReconnecting = false;
      // Don't clear terminal - we want to receive the refreshed screen from server
      
      // Delay the resize slightly to ensure mode restore sequences are processed first
      // This is important for alternate screen apps (zellij, vim) - the resize triggers
      // SIGWINCH which makes them redraw their content
      setTimeout(() => {
        this.forceRefresh();
      }, 50);
      
      // Start ping interval (60s to reduce network overhead)
      this.pingInterval = setInterval(() => {
        this.send({ type: 'ping', data: null });
      }, 60000);
    };

    this.ws.onmessage = (event) => {
      try {
        const msg: WSMessage = JSON.parse(event.data);
        if (msg.type === 'output') {
          this.bufferOutput(msg.data as string);
        }
      } catch (e) {
        console.error('Failed to parse WebSocket message:', e);
      }
    };

    this.ws.onclose = (event) => {
      if (this.pingInterval) {
        clearInterval(this.pingInterval);
        this.pingInterval = null;
      }
      
      // Attempt to reconnect if not a clean close and we haven't exceeded max attempts
      if (!event.wasClean && this.sessionId && this.reconnectAttempts < this.maxReconnectAttempts) {
        this.attemptReconnect();
      } else if (this.reconnectAttempts >= this.maxReconnectAttempts) {
        this.terminal.writeln('\r\n\x1b[31m[Connection lost. Max reconnection attempts reached. Please refresh the page.]\x1b[0m');
      } else {
        this.terminal.writeln('\r\n\x1b[31m[Connection closed]\x1b[0m');
      }
    };

    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
  }

  private attemptReconnect() {
    if (this.isReconnecting || !this.sessionId) return;
    
    this.isReconnecting = true;
    this.reconnectAttempts++;
    
    const delay = Math.min(this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1), 10000);
    
    this.terminal.writeln(`\r\n\x1b[33m[Connection lost. Reconnecting in ${delay / 1000}s... (${this.reconnectAttempts}/${this.maxReconnectAttempts})]\x1b[0m`);
    
    setTimeout(() => {
      if (this.sessionId) {
        this.doConnect();
      }
    }, delay);
  }

  disconnect() {
    // Clear session ID first to prevent reconnection attempts
    const wasConnected = this.sessionId !== null;
    this.sessionId = null;
    this.isReconnecting = false;
    this.reconnectAttempts = this.maxReconnectAttempts; // Prevent auto-reconnect
    
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    if (this.pingInterval) {
      clearInterval(this.pingInterval);
      this.pingInterval = null;
    }
  }

  private send(msg: WSMessage) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      debugLog('[Terminal] send:', msg.type, JSON.stringify(msg.data));
      this.ws.send(JSON.stringify(msg));
    } else {
      debugLog('[Terminal] send FAILED - ws not open, readyState:', this.ws?.readyState);
    }
  }

  // Buffer input and flush on microtask to reduce churn
  private queueInput(data: string) {
    if (!data) return;
    this.inputBuffer += data;
    if (!this.inputFlushScheduled) {
      this.inputFlushScheduled = true;
      queueMicrotask(() => this.flushInput());
    }
  }

  private flushInput() {
    this.inputFlushScheduled = false;
    if (!this.inputBuffer) return;
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      debugLog('[Terminal] flushInput skipped - ws not open');
      this.inputBuffer = '';
      return;
    }
    if (this.ws.bufferedAmount > 512 * 1024) {
      debugLog('[Terminal] flushInput backpressure:', this.ws.bufferedAmount);
      this.inputFlushScheduled = true;
      setTimeout(() => this.flushInput(), 16);
      return;
    }
    const payload = this.inputBuffer;
    this.inputBuffer = '';
    this.send({ type: 'input', data: payload });
  }

  // Buffer output and flush on next animation frame for better performance
  private bufferOutput(data: string) {
    this.outputBuffer += data;
    if (!this.outputFlushScheduled) {
      this.outputFlushScheduled = true;
      requestAnimationFrame(() => {
        if (this.outputBuffer) {
          this.terminal.write(this.outputBuffer);
          this.outputBuffer = '';
        }
        this.outputFlushScheduled = false;
      });
    }
  }

  // Send special key
  sendKey(key: string) {
    debugLog('[Terminal] sendKey called:', JSON.stringify(key));
    this.queueInput(key);
    this.terminal.focus();
  }

  focus() {
    this.terminal.focus();
  }

  // Set font size and refit terminal
  setFontSize(size: number) {
    const newSize = Math.max(this.minFontSize, Math.min(this.maxFontSize, Math.round(size)));
    if (newSize === this.fontSize) return;
    
    this.fontSize = newSize;
    this.terminal.options.fontSize = newSize;
    setCachedFontSize(newSize);
    this.fit();
  }

  getFontSize(): number {
    return this.fontSize;
  }

  // Setup visualViewport listener to handle mobile keyboard appearance
  // When keyboard appears, we translate the terminal container upward
  // incrementally as content grows - only moving when cursor would be hidden
  private setupViewportResize() {
    if (!window.visualViewport) {
      debugLog('[Terminal] visualViewport API not available');
      return;
    }

    // Find the terminal container to transform
    const terminalContainer = this.container.closest('.terminal-container') as HTMLElement;
    if (!terminalContainer) {
      debugLog('[Terminal] terminal-container not found');
      return;
    }

    let currentTranslateY = 0;
    let keyboardHeight = 0;
    let initialHeight = window.visualViewport.height;
    let debounceTimer: ReturnType<typeof setTimeout> | null = null;
    let updateTimer: ReturnType<typeof setTimeout> | null = null;
    
    // Function to calculate and apply translation based on cursor position
    const updateTranslation = () => {
      if (this.disposed || keyboardHeight <= 50) {
        return;
      }
      
      const viewportHeight = window.visualViewport!.height;
      const viewportOffsetTop = window.visualViewport!.offsetTop;
      const lineHeight = this.fontSize * 1.2;
      
      // Get safe area inset bottom for iOS Safari floating address bar
      const safeAreaBottom = parseInt(
        getComputedStyle(document.documentElement).getPropertyValue('--sab') || '0'
      ) || 0;
      
      // Extra padding for iOS Safari's floating bottom bar (approximately 30-50px)
      const iosSafariBottomBar = 50;
      
      // Get cursor row from terminal buffer
      const cursorY = this.terminal.buffer.active.cursorY;
      
      // Use fixed original position (0) since terminal starts at top
      // Don't use getBoundingClientRect as it's affected by ongoing CSS transitions
      const originalTop = 0;
      
      // Calculate cursor bottom position in screen coordinates
      const cursorBottom = originalTop + (cursorY + 1) * lineHeight;
      
      // How much cursor overlaps with keyboard (keyboard starts at viewportHeight)
      // Subtract safe area and iOS bottom bar to account for Safari UI
      const visibleBottom = viewportHeight + viewportOffsetTop - safeAreaBottom - iosSafariBottomBar;
      const overlap = cursorBottom - visibleBottom;
      
      debugLog('[Terminal] updateTranslation:', { 
        cursorY, cursorBottom, visibleBottom, safeAreaBottom, iosSafariBottomBar, overlap, currentTranslateY 
      });
      
      if (overlap > 0) {
        // Need to translate up - cursor is behind keyboard
        // Round to nearest pixel to avoid sub-pixel jitter
        const newTranslateY = Math.round(Math.min(overlap + 10, keyboardHeight + iosSafariBottomBar));
        if (newTranslateY > currentTranslateY) {
          currentTranslateY = newTranslateY;
          terminalContainer.style.transform = `translateY(-${currentTranslateY}px)`;
          debugLog('[Terminal] applied translateY:', currentTranslateY);
        }
      }
    };
    
    // Debounced version for terminal output
    const debouncedUpdate = () => {
      if (updateTimer) {
        clearTimeout(updateTimer);
      }
      updateTimer = setTimeout(updateTranslation, 50);
    };
    
    // Listen to terminal data to detect new content
    this.terminal.onWriteParsed(() => {
      if (keyboardHeight > 50) {
        debouncedUpdate();
      }
    });
    
    this.viewportResizeHandler = () => {
      if (!window.visualViewport || this.disposed) return;
      
      // Debounce to wait for iOS keyboard animation to settle
      if (debounceTimer) {
        clearTimeout(debounceTimer);
      }
      
      debounceTimer = setTimeout(() => {
        if (!window.visualViewport || this.disposed) return;
        
        const viewportHeight = window.visualViewport.height;
        
        // Update initial height when keyboard is closed (full viewport)
        if (viewportHeight > initialHeight) {
          initialHeight = viewportHeight;
        }
        
        const newKeyboardHeight = initialHeight - viewportHeight;
        
        debugLog('[Terminal] visualViewport stable:', { initialHeight, viewportHeight, newKeyboardHeight });
        
        if (newKeyboardHeight > 50) {
          // Keyboard opened or resized
          keyboardHeight = newKeyboardHeight;
          updateTranslation();
        } else {
          // Keyboard closed - reset translation with animation
          keyboardHeight = 0;
          currentTranslateY = 0;
          terminalContainer.style.transform = '';
        }
      }, 200);
    };

    // Listen to viewport changes
    window.visualViewport.addEventListener('resize', this.viewportResizeHandler);
    window.visualViewport.addEventListener('scroll', this.viewportResizeHandler);
  }

  // Setup pinch-to-zoom gesture for font size adjustment
  // And single-finger swipe for scrolling terminal history
  private setupPinchZoom() {
    let initialDistance = 0;
    let initialFontSize = this.fontSize;
    let isPinching = false;
    
    // Single finger scroll state
    let isScrolling = false;
    let scrollStartY = 0;
    let lastY = 0;
    let lastTime = 0;
    let velocity = 0;
    let momentumId: number | null = null;
    let accumulatedDelta = 0;
    let lastTouch: Touch | null = null;  // Store last touch for momentum in alternate screen

    const getDistance = (touches: TouchList): number => {
      if (touches.length < 2) return 0;
      const dx = touches[1].clientX - touches[0].clientX;
      const dy = touches[1].clientY - touches[0].clientY;
      return Math.sqrt(dx * dx + dy * dy);
    };
    
    const stopMomentum = () => {
      if (momentumId !== null) {
        cancelAnimationFrame(momentumId);
        momentumId = null;
      }
    };
    
    // Check if terminal is in alternate screen buffer (used by fullscreen apps like vim, htop, zellij)
    const isAlternateBuffer = (): boolean => {
      // xterm.js buffer.active.type is 'alternate' when in alternate screen
      return this.terminal.buffer.active.type === 'alternate';
    };
    
    // Send mouse wheel escape sequence to the terminal application
    // SGR encoding: \x1b[<button;col;rowM for press
    // Button 64 = wheel up, 65 = wheel down
    const sendMouseWheel = (up: boolean, col: number, row: number) => {
      const button = up ? 64 : 65;
      // SGR mouse encoding: CSI < button ; col ; row M
      const seq = `\x1b[<${button};${col};${row}M`;
      this.send({ type: 'input', data: seq });
    };
    
    // Use xterm's scrollLines API for scrolling, or send mouse wheel events for alternate screen
    const scrollByLines = (lines: number, touch?: Touch) => {
      if (isAlternateBuffer()) {
        // In alternate screen (zellij, vim, etc.), send mouse wheel events
        let col = 1, row = 1;
        if (touch) {
          const pos = this.touchToTerminalPosition(touch);
          if (pos) {
            col = pos.col + 1;  // 1-based
            row = pos.row + 1;  // 1-based
          }
        }
        // Send one wheel event per line for better granularity
        const up = lines < 0;
        const count = Math.abs(lines);
        for (let i = 0; i < count; i++) {
          sendMouseWheel(up, col, row);
        }
      } else {
        // Normal buffer - use xterm's built-in scroll
        this.terminal.scrollLines(lines);
      }
    };
    
    const applyMomentum = () => {
      if (Math.abs(velocity) < 0.5) {
        momentumId = null;
        lastTouch = null;
        return;
      }
      
      // Convert velocity to lines (roughly 16px per line)
      const lines = Math.round(velocity / 2);
      if (lines !== 0) {
        // For alternate screen, use last touch position; for normal, touch doesn't matter
        scrollByLines(lines, lastTouch || undefined);
      }
      
      velocity *= 0.92; // Friction
      momentumId = requestAnimationFrame(applyMomentum);
    };

    // For alternate screen (zellij, vim, etc.), we need to dispatch synthetic mouse events
    // because xterm.js handles mouse mode internally but touch events don't automatically
    // trigger mouse event handlers in the same way across all browsers
    const dispatchMouseEvent = (type: string, touch: Touch, button: number = 0) => {
      const target = this.container.querySelector('.xterm-screen') || this.container;
      const mouseEvent = new MouseEvent(type, {
        bubbles: true,
        cancelable: true,
        view: window,
        clientX: touch.clientX,
        clientY: touch.clientY,
        button: button,
        buttons: type === 'mouseup' ? 0 : 1,
      });
      target.dispatchEvent(mouseEvent);
    };

    // Touch state for tap detection
    let touchStartInfo: { x: number; y: number; time: number; touch: Touch } | null = null;
    const TAP_THRESHOLD_DISTANCE = 10;
    const TAP_THRESHOLD_TIME = 300;

    this.container.addEventListener('touchstart', (e: TouchEvent) => {
      stopMomentum();
      
      // In selection mode, handle touch selection
      if (this.selectionMode && e.touches.length === 1) {
        const pos = this.touchToTerminalPosition(e.touches[0]);
        if (pos) {
          this.touchSelectionStart = pos;
          this.touchSelectionActive = true;
          this.terminal.clearSelection();
          e.preventDefault();
        }
        return;
      }
      
      if (e.touches.length === 2) {
        // Two finger pinch
        isPinching = true;
        isScrolling = false;
        touchStartInfo = null;
        initialDistance = getDistance(e.touches);
        initialFontSize = this.fontSize;
        e.preventDefault();
      } else if (e.touches.length === 1) {
        // Single finger - prepare for scroll or tap
        isScrolling = true;
        scrollStartY = e.touches[0].clientY;
        lastY = scrollStartY;
        lastTime = Date.now();
        velocity = 0;
        accumulatedDelta = 0;
        
        // Record touch start for tap detection
        touchStartInfo = {
          x: e.touches[0].clientX,
          y: e.touches[0].clientY,
          time: Date.now(),
          touch: e.touches[0]
        };
        
        // Note: We don't dispatch mousedown here immediately because we don't know
        // if this is a tap or a scroll yet. Mousedown will be dispatched later
        // if we determine this was a tap (short duration, minimal movement).
      }
    }, { passive: false });

    this.container.addEventListener('touchmove', (e: TouchEvent) => {
      // In selection mode, handle touch selection
      if (this.selectionMode && e.touches.length === 1 && this.touchSelectionActive && this.touchSelectionStart) {
        const pos = this.touchToTerminalPosition(e.touches[0]);
        if (pos) {
          this.updateTouchSelection(this.touchSelectionStart, pos);
          e.preventDefault();
        }
        return;
      }
      
      if (isPinching && e.touches.length === 2) {
        const currentDistance = getDistance(e.touches);
        const scale = currentDistance / initialDistance;
        const newFontSize = initialFontSize * scale;
        
        this.setFontSize(newFontSize);
        e.preventDefault();
      } else if (isScrolling && e.touches.length === 1 && !isPinching) {
        const currentY = e.touches[0].clientY;
        const currentTime = Date.now();
        const deltaY = lastY - currentY;
        
        // In alternate buffer, check if movement is significant enough to be a scroll
        // This prevents accidental scrolling during taps
        if (touchStartInfo && isAlternateBuffer()) {
          const dx = e.touches[0].clientX - touchStartInfo.x;
          const dy = e.touches[0].clientY - touchStartInfo.y;
          const distance = Math.sqrt(dx * dx + dy * dy);
          // Don't start scrolling until we've moved more than tap threshold
          if (distance < TAP_THRESHOLD_DISTANCE) {
            e.preventDefault();
            return;
          }
        }
        
        // Calculate velocity for momentum
        const timeDelta = currentTime - lastTime;
        if (timeDelta > 0) {
          velocity = deltaY / timeDelta * 16;
        }
        
        // Accumulate delta and scroll by lines when threshold reached
        accumulatedDelta += deltaY;
        const lineHeight = this.fontSize * 1.2; // Approximate line height
        const linesToScroll = Math.trunc(accumulatedDelta / lineHeight);
        
        if (linesToScroll !== 0) {
          scrollByLines(linesToScroll, e.touches[0]);
          accumulatedDelta -= linesToScroll * lineHeight;
        }
        
        lastY = currentY;
        lastTime = currentTime;
        lastTouch = e.touches[0];  // Save touch for momentum
        
        e.preventDefault(); // Prevent page scroll
      }
    }, { passive: false });

    this.container.addEventListener('touchend', (e: TouchEvent) => {
      // In selection mode, finalize selection
      if (this.selectionMode) {
        this.touchSelectionActive = false;
        // Keep touchSelectionStart for potential future use
        isPinching = false;
        isScrolling = false;
        touchStartInfo = null;
        return;
      }
      
      if (isPinching) {
        isPinching = false;
        touchStartInfo = null;
      }
      if (isScrolling && e.touches.length === 0) {
        isScrolling = false;
        
        // Check if this was a tap in alternate buffer
        if (touchStartInfo && isAlternateBuffer()) {
          const endTouch = e.changedTouches[0];
          if (endTouch) {
            const dx = endTouch.clientX - touchStartInfo.x;
            const dy = endTouch.clientY - touchStartInfo.y;
            const distance = Math.sqrt(dx * dx + dy * dy);
            const duration = Date.now() - touchStartInfo.time;
            
            // If it was a quick tap with minimal movement, dispatch click events
            if (duration < TAP_THRESHOLD_TIME && distance < TAP_THRESHOLD_DISTANCE) {
              // Dispatch mousedown then mouseup to simulate a click
              dispatchMouseEvent('mousedown', endTouch);
              dispatchMouseEvent('mouseup', endTouch);
              touchStartInfo = null;
              return;
            }
          }
        }
        touchStartInfo = null;
        // Apply momentum scrolling
        if (Math.abs(velocity) > 2) {
          momentumId = requestAnimationFrame(applyMomentum);
        }
      }
    });

    this.container.addEventListener('touchcancel', () => {
      isPinching = false;
      isScrolling = false;
      this.touchSelectionActive = false;
      touchStartInfo = null;
      stopMomentum();
    });
  }

  // Setup swipe arrow controller for long-press arrow key gestures
  private setupSwipeArrow() {
    this.swipeArrowController = new SwipeArrowController(this.container, {
      onSendKey: (key: string) => {
        this.sendKey(key);
      },
      onGestureActive: (active: boolean) => {
        // When gesture is active, temporarily disable the textarea to prevent
        // iOS Safari from showing the keyboard when the gesture ends.
        // We use pointer-events:none instead of readonly/disabled because
        // those can cause the existing keyboard to close.
        const xtermTextarea = this.container.querySelector('.xterm-helper-textarea') as HTMLTextAreaElement;
        if (xtermTextarea) {
          if (active) {
            xtermTextarea.style.pointerEvents = 'none';
          } else {
            xtermTextarea.style.pointerEvents = '';
          }
        }
      },
      longPressDelay: 300,
      minDistance: 15,
      minRepeatDistance: 30,
      maxRepeatDistance: 120,
      slowRepeatInterval: 300,
      fastRepeatInterval: 40,
    });
  }

  getTerminal(): Terminal {
    return this.terminal;
  }

  getSessionId(): string | null {
    return this.sessionId;
  }

  // Set input interceptor for modifier keys (Ctrl, Alt)
  setInputInterceptor(interceptor: ((data: string) => string | null) | null) {
    this.inputInterceptor = interceptor;
  }

  // Lock/unlock keyboard to prevent soft keyboard from appearing
  // Uses CSS class to disable pointer events - simple and reliable
  setKeyboardLocked(locked: boolean) {
    this.keyboardLocked = locked;
    debugLog('[Terminal] setKeyboardLocked:', locked);
    
    const xtermTextarea = this.container.querySelector('.xterm-helper-textarea') as HTMLTextAreaElement;
    
    if (locked) {
      // Add CSS class to disable pointer events on the terminal
      this.container.classList.add('keyboard-locked');
      // Make textarea readonly instead of blur - avoids iOS Safari keyboard state issues
      if (xtermTextarea) {
        xtermTextarea.readOnly = true;
        // Move focus away to a non-input element to hide keyboard
        // Using document.body instead of blur() to avoid Safari state issues
        (document.activeElement as HTMLElement)?.blur?.();
      }
    } else {
      // Remove CSS class
      this.container.classList.remove('keyboard-locked');
      // Restore textarea
      if (xtermTextarea) {
        xtermTextarea.readOnly = false;
      }
    }
  }

  isKeyboardLocked(): boolean {
    return this.keyboardLocked;
  }

  // Selection mode - allows finger selection on mobile
  setSelectionMode(enabled: boolean) {
    this.selectionMode = enabled;
    debugLog('[Terminal] setSelectionMode:', enabled);
    
    if (enabled) {
      // Enable selection mode
      this.container.classList.add('selection-mode');
      // Clear any existing selection
      this.terminal.clearSelection();
      this.touchSelectionStart = null;
      this.touchSelectionActive = false;
    } else {
      // Disable selection mode
      this.container.classList.remove('selection-mode');
      this.touchSelectionStart = null;
      this.touchSelectionActive = false;
    }
  }

  isSelectionMode(): boolean {
    return this.selectionMode;
  }

  // Convert touch position to terminal column/row
  private touchToTerminalPosition(touch: Touch): { col: number; row: number } | null {
    const core = (this.terminal as unknown as { _core: { _renderService: { dimensions: { css: { cell: { width: number; height: number } } } } } })._core;
    const dims = core._renderService?.dimensions;
    if (!dims?.css?.cell) {
      return null;
    }
    
    const cellWidth = dims.css.cell.width;
    const cellHeight = dims.css.cell.height;
    
    // Get terminal element bounds
    const xtermElement = this.container.querySelector('.xterm-screen');
    if (!xtermElement) {
      return null;
    }
    
    const rect = xtermElement.getBoundingClientRect();
    const x = touch.clientX - rect.left;
    const y = touch.clientY - rect.top;
    
    // Convert to column/row (viewport-relative)
    const col = Math.floor(x / cellWidth);
    const row = Math.floor(y / cellHeight);
    
    return {
      col: Math.max(0, Math.min(col, this.terminal.cols - 1)),
      row: Math.max(0, Math.min(row, this.terminal.rows - 1))
    };
  }

  // Update selection from start to end position
  private updateTouchSelection(start: { col: number; row: number }, end: { col: number; row: number }) {
    // Determine the direction of selection
    let startCol = start.col;
    let startRow = start.row;
    let endCol = end.col;
    let endRow = end.row;
    
    // Normalize: ensure start is before end
    if (startRow > endRow || (startRow === endRow && startCol > endCol)) {
      [startCol, startRow, endCol, endRow] = [endCol, endRow, startCol, startRow];
    }
    
    // Calculate the length of the selection
    let length: number;
    if (startRow === endRow) {
      // Same row
      length = endCol - startCol + 1;
    } else {
      // Multiple rows
      // First row: from startCol to end of line
      // Middle rows: full lines
      // Last row: from start to endCol
      const cols = this.terminal.cols;
      length = (cols - startCol) + // First row
               (endRow - startRow - 1) * cols + // Middle rows
               (endCol + 1); // Last row
    }
    
    // Use xterm's select method with viewport-relative row
    this.terminal.select(startCol, startRow, length);
    
    debugLog('[Terminal] updateTouchSelection:', { startCol, startRow, length, selection: this.terminal.getSelection() });
  }

  // Get selected text from terminal
  getSelection(): string {
    return this.terminal.getSelection();
  }

  // Check if there's any selection
  hasSelection(): boolean {
    return this.terminal.hasSelection();
  }

  // Clear selection
  clearSelection() {
    this.terminal.clearSelection();
  }

  // Copy selection to clipboard
  async copySelection(): Promise<boolean> {
    const selection = this.terminal.getSelection();
    if (!selection) {
      return false;
    }
    
    try {
      await navigator.clipboard.writeText(selection);
      debugLog('[Terminal] Copied to clipboard:', selection.length, 'chars');
      return true;
    } catch (err) {
      console.error('[Terminal] Failed to copy:', err);
      // Fallback for older browsers
      try {
        const textArea = document.createElement('textarea');
        textArea.value = selection;
        textArea.style.position = 'fixed';
        textArea.style.left = '-9999px';
        document.body.appendChild(textArea);
        textArea.select();
        document.execCommand('copy');
        document.body.removeChild(textArea);
        return true;
      } catch (fallbackErr) {
        console.error('[Terminal] Fallback copy failed:', fallbackErr);
        return false;
      }
    }
  }

  // Dispose and cleanup all resources
  dispose() {
    if (this.disposed) return;
    this.disposed = true;
    
    // Disconnect WebSocket
    this.disconnect();
    
    // Disconnect ResizeObserver
    if (this.resizeObserver) {
      this.resizeObserver.disconnect();
      this.resizeObserver = null;
    }
    
    // Remove window resize listener
    if (this.debouncedFit) {
      window.removeEventListener('resize', this.debouncedFit);
      this.debouncedFit = null;
    }
    
    // Remove visualViewport listeners
    if (this.viewportResizeHandler && window.visualViewport) {
      window.visualViewport.removeEventListener('resize', this.viewportResizeHandler);
      window.visualViewport.removeEventListener('scroll', this.viewportResizeHandler);
      this.viewportResizeHandler = null;
    }
    
    // Dispose WebGL addon
    if (this.webglAddon) {
      this.webglAddon.dispose();
      this.webglAddon = null;
    }
    
    // Dispose swipe arrow controller
    if (this.swipeArrowController) {
      this.swipeArrowController.dispose();
      this.swipeArrowController = null;
    }
    
    // Dispose terminal
    this.terminal.dispose();
  }
}
