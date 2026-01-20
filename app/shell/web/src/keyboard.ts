import type { TerminalManager } from './terminal';

// Debug mode - only log when ?debug=1 is in URL
const DEBUG = typeof window !== 'undefined' && window.location.search.includes('debug=1');

function debugLog(...args: unknown[]) {
  if (DEBUG) {
    console.log('[Keyboard]', ...args);
  }
}

interface KeyConfig {
  label: string;
  key: string;
  wide?: boolean;
}

const specialKeys: KeyConfig[] = [
  { label: 'Esc', key: '\x1b' },
  { label: 'Tab', key: '\t' },
  { label: 'Ctrl', key: '' },
  { label: 'Alt', key: '' },
  { label: '|', key: '|' },
  { label: '/', key: '/' },
  { label: '-', key: '-' },
  { label: '~', key: '~' },
];

const arrowKeys: KeyConfig[] = [
  { label: '\u2191', key: '\x1b[A' },
  { label: '\u2193', key: '\x1b[B' },
  { label: '\u2190', key: '\x1b[D' },
  { label: '\u2192', key: '\x1b[C' },
];

const ctrlShortcuts: KeyConfig[] = [
  { label: '^C', key: '\x03' },  // Ctrl+C
  { label: '^L', key: '\x0c' },  // Ctrl+L (clear screen)
];

export class VirtualKeyboard {
  private container: HTMLElement;
  private terminal: TerminalManager;
  private onLogout: () => void;
  private ctrlActive = false;
  private altActive = false;
  private ctrlLocked = false;  // Long-press to lock Ctrl
  private altLocked = false;   // Long-press to lock Alt
  private isDragging = false;
  private isDraggingMinimized = false;  // Separate state for minimized button drag
  private hasDragged = false;  // Track if actual movement occurred
  private dragStartY = 0;
  private dragStartX = 0;
  private dragStartTop = 0;
  private dragStartRight = 0;
  private isMinimized = false;
  private minimizedButton: HTMLElement | null = null;
  private isMultilineMode = false;  // Multi-line input mode
  private inputArea: HTMLTextAreaElement | null = null;
  private isComposing = false;  // Track IME composition state
  private pendingEnter = false;
  private ctrlButton: HTMLElement | null = null;  // Cached Ctrl button
  private altButton: HTMLElement | null = null;   // Cached Alt button
  private selectBtn: HTMLElement | null = null;   // Cached Select button
  private copyBtn: HTMLElement | null = null;     // Cached Copy button
  private disposed = false;
  private eventCleanups: (() => void)[] = [];  // Track event listeners for cleanup

  constructor(container: HTMLElement, terminal: TerminalManager, onLogout: () => void) {
    this.container = container;
    this.terminal = terminal;
    this.onLogout = onLogout;
    this.render();
    this.setupDrag();
    this.setupInputInterceptor();
    this.setupGlobalDragListeners();
    this.setupResizeListener();
    // Start minimized by default
    this.minimize();
  }
  
  private addEventListenerWithCleanup<K extends keyof WindowEventMap>(
    target: Window,
    type: K,
    listener: (ev: WindowEventMap[K]) => void,
    options?: boolean | AddEventListenerOptions
  ): void;
  private addEventListenerWithCleanup<K extends keyof DocumentEventMap>(
    target: Document,
    type: K,
    listener: (ev: DocumentEventMap[K]) => void,
    options?: boolean | AddEventListenerOptions
  ): void;
  private addEventListenerWithCleanup(
    target: Window | Document,
    type: string,
    listener: EventListener,
    options?: boolean | AddEventListenerOptions
  ): void {
    target.addEventListener(type, listener, options);
    this.eventCleanups.push(() => target.removeEventListener(type, listener, options));
  }

  private setupResizeListener() {
    // Handle window resize to keep minimized button in bounds
    const resizeHandler = () => {
      if (this.isMinimized && this.minimizedButton) {
        // Clamp minimized button position (using top positioning)
        const currentTop = parseInt(this.minimizedButton.style.top) || 10;
        const currentRight = parseInt(this.minimizedButton.style.right) || 10;
        const maxTop = window.innerHeight - 60;
        const maxRight = window.innerWidth - 60;
        
        this.minimizedButton.style.top = `${Math.max(10, Math.min(maxTop, currentTop))}px`;
        this.minimizedButton.style.right = `${Math.max(10, Math.min(maxRight, currentRight))}px`;
      }
      // Keyboard position is fixed via CSS, no need to clamp
    };
    this.addEventListenerWithCleanup(window, 'resize', resizeHandler);
  }

  private setupGlobalDragListeners() {
    // Global mouse/touch listeners for minimized button drag
    this.addEventListenerWithCleanup(document, 'mousemove', (e) => {
      if (this.isDraggingMinimized && this.isMinimized && this.minimizedButton) {
        e.preventDefault();
        this.handleMinimizedDragMove(e.clientX, e.clientY);
      }
    });

    this.addEventListenerWithCleanup(document, 'mouseup', () => {
      if (this.isMinimized && this.isDraggingMinimized) {
        this.handleMinimizedDragEnd();
      }
    });

    this.addEventListenerWithCleanup(document, 'touchmove', (e) => {
      if (this.isDraggingMinimized && this.isMinimized && this.minimizedButton) {
        e.preventDefault();
        this.handleMinimizedDragMove(e.touches[0].clientX, e.touches[0].clientY);
      }
    }, { passive: false });

    this.addEventListenerWithCleanup(document, 'touchend', () => {
      if (this.isMinimized && this.isDraggingMinimized) {
        this.handleMinimizedDragEnd();
      }
    });
  }

  private handleMinimizedDragMove(clientX: number, clientY: number) {
    if (!this.minimizedButton) return;
    
    const deltaY = clientY - this.dragStartY;
    const deltaX = this.dragStartX - clientX;
    
    // Check if actually moved (more than 5px)
    if (Math.abs(deltaX) > 5 || Math.abs(deltaY) > 5) {
      this.hasDragged = true;
    }
    
    const newTop = Math.max(10, Math.min(
      window.innerHeight - 60,
      this.dragStartTop + deltaY
    ));
    const newRight = Math.max(10, Math.min(
      window.innerWidth - 60,
      this.dragStartRight + deltaX
    ));
    
    this.minimizedButton.style.top = `${newTop}px`;
    this.minimizedButton.style.right = `${newRight}px`;
  }

  private handleMinimizedDragEnd() {
    if (!this.minimizedButton) return;
    
    this.minimizedButton.classList.remove('dragging');
    this.isDraggingMinimized = false;
    
    // If it was just a click (no movement), restore the keyboard
    if (!this.hasDragged) {
      this.restore();
    }
    // If dragged, just leave it where it is
  }

  private minimize() {
    this.isMinimized = true;
    
    // Save current keyboard position before hiding
    const computedStyle = getComputedStyle(this.container);
    const currentTop = parseInt(computedStyle.top) || 10;
    
    this.container.style.display = 'none';
    
    // Create minimized button
    this.minimizedButton = document.createElement('div');
    this.minimizedButton.className = 'keyboard-minimized';
    this.minimizedButton.title = '展开工具栏';
    
    // Position at current keyboard position (top right corner of keyboard)
    this.minimizedButton.style.top = `${currentTop}px`;
    this.minimizedButton.style.right = '10px';
    
    // Add to parent
    this.container.parentElement?.appendChild(this.minimizedButton);
    
    // Setup drag for minimized button (must be before click handler)
    this.setupMinimizedDrag();
  }

  private restore() {
    this.isMinimized = false;
    
    // Get the position of minimized button before removing it
    let savedTop: number | null = null;
    if (this.minimizedButton) {
      savedTop = parseInt(this.minimizedButton.style.top) || null;
      this.minimizedButton.remove();
      this.minimizedButton = null;
    }
    
    // Show the container and set its position to match the minimized button
    this.container.style.display = '';
    if (savedTop !== null) {
      // Clamp the position to ensure keyboard fits on screen
      const maxTop = window.innerHeight - this.container.offsetHeight - 10;
      const clampedTop = Math.max(10, Math.min(maxTop, savedTop));
      this.container.style.top = `${clampedTop}px`;
    }
  }

  private setupMinimizedDrag() {
    if (!this.minimizedButton) return;
    
    const btn = this.minimizedButton;
    
    const onDragStart = (clientX: number, clientY: number) => {
      this.isDraggingMinimized = true;
      this.hasDragged = false;
      this.dragStartY = clientY;
      this.dragStartX = clientX;
      this.dragStartTop = parseInt(btn.style.top) || 10;
      this.dragStartRight = parseInt(btn.style.right) || 10;
      btn.classList.add('dragging');
    };

    // Touch events - only on button
    btn.addEventListener('touchstart', (e) => {
      e.preventDefault();
      onDragStart(e.touches[0].clientX, e.touches[0].clientY);
    }, { passive: false });

    // Mouse events - only on button
    btn.addEventListener('mousedown', (e) => {
      e.preventDefault();
      onDragStart(e.clientX, e.clientY);
    });
  }

  private setupInputInterceptor() {
    this.terminal.setInputInterceptor((data: string) => {
      debugLog('[Interceptor] input:', JSON.stringify(data), 'ctrl:', this.ctrlActive, 'alt:', this.altActive);
      
      // If no modifiers active, pass through unchanged
      if (!this.ctrlActive && !this.altActive) {
        debugLog('[Interceptor] no modifiers, passing through');
        return data;
      }

      let result = data;
      let ctrlApplied = false;
      let altApplied = false;

      // Apply Ctrl modifier - only for single printable characters
      if (this.ctrlActive && data.length === 1) {
        const code = data.toUpperCase().charCodeAt(0);
        // A-Z (65-90) -> Ctrl codes 1-26
        if (code >= 65 && code <= 90) {
          result = String.fromCharCode(code - 64);
          ctrlApplied = true;
        } else if (code >= 97 && code <= 122) {
          // lowercase a-z
          result = String.fromCharCode(code - 96);
          ctrlApplied = true;
        }
      }

      // Apply Alt modifier (ESC prefix) - only for single characters
      if (this.altActive && data.length === 1) {
        result = '\x1b' + result;
        altApplied = true;
      }

      // Only reset modifier state if it was actually applied AND not locked
      if (ctrlApplied && !this.ctrlLocked) {
        this.ctrlActive = false;
        this.updateModifierButtons();
      }
      if (altApplied && !this.altLocked) {
        this.altActive = false;
        this.updateModifierButtons();
      }

      debugLog('[Interceptor] result:', JSON.stringify(result));
      return result;
    });
  }

  private setupDrag() {
    const handle = this.container.querySelector('.keyboard-drag-handle') as HTMLElement;
    if (!handle) return;

    const onDragStart = (clientY: number) => {
      this.isDragging = true;
      this.dragStartY = clientY;
      const computedStyle = getComputedStyle(this.container);
      this.dragStartTop = parseInt(computedStyle.top) || 10;
      this.container.classList.add('dragging');
    };

    const onDragMove = (clientY: number) => {
      if (!this.isDragging) return;
      const deltaY = clientY - this.dragStartY;
      const newTop = Math.max(0, Math.min(
        window.innerHeight - this.container.offsetHeight - 10,
        this.dragStartTop + deltaY
      ));
      this.container.style.top = `${newTop}px`;
    };

    const onDragEnd = () => {
      if (!this.isDragging) return;
      this.isDragging = false;
      this.container.classList.remove('dragging');
    };

    // Touch events
    handle.addEventListener('touchstart', (e) => {
      e.preventDefault();
      onDragStart(e.touches[0].clientY);
    }, { passive: false });

    this.addEventListenerWithCleanup(document, 'touchmove', (e) => {
      if (this.isDragging) {
        e.preventDefault();
        onDragMove(e.touches[0].clientY);
      }
    }, { passive: false });

    this.addEventListenerWithCleanup(document, 'touchend', onDragEnd);

    // Mouse events
    handle.addEventListener('mousedown', (e) => {
      e.preventDefault();
      onDragStart(e.clientY);
    });

    this.addEventListenerWithCleanup(document, 'mousemove', (e) => {
      if (this.isDragging) {
        e.preventDefault();
        onDragMove(e.clientY);
      }
    });

    this.addEventListenerWithCleanup(document, 'mouseup', onDragEnd);
  }

  private render() {
    this.container.innerHTML = '';
    this.container.className = 'virtual-keyboard';

    // === Row 1: Drag handle + Input + Function buttons ===
    const inputRow = document.createElement('div');
    inputRow.className = 'keyboard-row input-row';
    
    // Drag handle (integrated into row 1)
    const dragHandle = document.createElement('div');
    dragHandle.className = 'keyboard-drag-handle';
    inputRow.appendChild(dragHandle);
    
    // Create textarea for input
    this.inputArea = document.createElement('textarea');
    this.inputArea.className = 'command-input';
    this.inputArea.placeholder = 'Command...';
    this.inputArea.rows = 1;
    this.inputArea.setAttribute('autocomplete', 'off');
    this.inputArea.setAttribute('autocorrect', 'off');
    this.inputArea.setAttribute('autocapitalize', 'off');
    this.inputArea.setAttribute('spellcheck', 'false');
    
    // Handle IME composition events
    this.inputArea.addEventListener('compositionstart', (e) => {
      debugLog('[IME] compositionstart:', e.data);
      this.isComposing = true;
    });
    
    this.inputArea.addEventListener('compositionend', (e) => {
      debugLog('[IME] compositionend:', e.data);
      setTimeout(() => {
        this.isComposing = false;
        debugLog('[IME] isComposing set to false (after timeout)');
        const shouldSend = this.pendingEnter && !this.isMultilineMode;
        this.pendingEnter = false;
        if (shouldSend) {
          this.sendInputCommand();
        }
      }, 0);
    });
    
    // Handle Enter key
    this.inputArea.addEventListener('keydown', (e) => {
      const isIMEProcessing = e.keyCode === 229;
      debugLog('[Keyboard] keydown:', e.key, 'keyCode:', e.keyCode, 'e.isComposing:', e.isComposing, 'this.isComposing:', this.isComposing, 'isIMEProcessing:', isIMEProcessing);
      const isImeActive = e.isComposing || this.isComposing || isIMEProcessing;
      
      if (isImeActive) {
        if (e.key === 'Enter' && !this.isMultilineMode) {
          this.pendingEnter = true;
          debugLog('[Keyboard] IME active - deferring Enter');
        }
        debugLog('[Keyboard] skipping - IME active');
        return;
      }
      
      if (e.key === 'Enter') {
        if (!this.isMultilineMode) {
          e.preventDefault();
          this.sendInputCommand();
        }
      }
    });
    
    // Auto-resize textarea
    this.inputArea.addEventListener('input', (e) => {
      const target = e.target as HTMLTextAreaElement;
      debugLog('[Keyboard] input event, value:', target.value, 'isComposing:', this.isComposing);
      if (this.isMultilineMode && this.inputArea) {
        this.inputArea.style.height = 'auto';
        this.inputArea.style.height = Math.min(this.inputArea.scrollHeight, 100) + 'px';
      }
    });
    
    inputRow.appendChild(this.inputArea);
    
    // Lock button (prevents soft keyboard from appearing)
    const lockBtn = document.createElement('button');
    lockBtn.className = 'keyboard-btn func-btn lock-btn';
    lockBtn.innerHTML = '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>';
    lockBtn.title = '锁定键盘';
    
    const updateLockState = (newLockState: boolean, focusInput: boolean) => {
      this.terminal.setKeyboardLocked(newLockState);
      lockBtn.classList.toggle('active', newLockState);
      // Locked: closed lock, Unlocked: open lock
      lockBtn.innerHTML = newLockState 
        ? '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>'
        : '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 5-5 5 5 0 0 1 5 5"/></svg>';
      if (focusInput && !newLockState && this.inputArea) {
        this.inputArea.focus();
      }
    };
    
    let lockTouchHandled = false;
    lockBtn.addEventListener('touchend', (e) => {
      e.preventDefault();
      lockTouchHandled = true;
      updateLockState(!this.terminal.isKeyboardLocked(), true);
    }, { passive: false });
    lockBtn.addEventListener('click', (e) => {
      if (lockTouchHandled) { lockTouchHandled = false; return; }
      e.preventDefault();
      updateLockState(!this.terminal.isKeyboardLocked(), true);
    });
    inputRow.appendChild(lockBtn);
    
    // Mode toggle button (multiline)
    const modeBtn = this.createButton('\u2261', () => {
      this.isMultilineMode = !this.isMultilineMode;
      modeBtn.classList.toggle('active', this.isMultilineMode);
      if (this.inputArea) {
        if (this.isMultilineMode) {
          this.inputArea.rows = 3;
          this.inputArea.style.height = 'auto';
        } else {
          this.inputArea.rows = 1;
          this.inputArea.style.height = '';
        }
      }
    });
    modeBtn.className = 'keyboard-btn func-btn mode-btn';
    modeBtn.title = '多行模式';
    inputRow.appendChild(modeBtn);
    
    // Send button
    const sendBtn = this.createButton('\u27A4', () => {
      this.sendInputCommand();
    });
    sendBtn.className = 'keyboard-btn func-btn send-btn';
    sendBtn.title = '发送';
    inputRow.appendChild(sendBtn);
    
    // Minimize button
    const minimizeBtn = document.createElement('button');
    minimizeBtn.className = 'keyboard-btn func-btn minimize-btn';
    minimizeBtn.innerHTML = '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><path d="M15 3h6v6M9 21H3v-6M21 3l-7 7M3 21l7-7"/></svg>';
    minimizeBtn.title = '最小化';
    minimizeBtn.addEventListener('click', (e) => {
      e.preventDefault();
      this.minimize();
    });
    inputRow.appendChild(minimizeBtn);
    
    this.container.appendChild(inputRow);

    // === Row 2: Shortcuts (horizontally scrollable) ===
    const shortcutsRow = document.createElement('div');
    shortcutsRow.className = 'keyboard-row shortcuts-row';
    
    const shortcutsScroll = document.createElement('div');
    shortcutsScroll.className = 'shortcuts-scroll';
    
    // Logout button (icon) - leftmost
    const logoutBtn = document.createElement('button');
    logoutBtn.className = 'keyboard-btn shortcut-btn logout-btn';
    logoutBtn.innerHTML = '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>';
    logoutBtn.title = '退出登录';
    
    let logoutTouchMove = false;
    let logoutTouchStartX = 0;
    let logoutTouchStartY = 0;
    logoutBtn.addEventListener('touchstart', (e) => {
      logoutTouchStartX = e.touches[0].clientX;
      logoutTouchStartY = e.touches[0].clientY;
      logoutTouchMove = false;
    }, { passive: true });
    logoutBtn.addEventListener('touchmove', (e) => {
      const dx = Math.abs(e.touches[0].clientX - logoutTouchStartX);
      const dy = Math.abs(e.touches[0].clientY - logoutTouchStartY);
      if (dx > 10 || dy > 10) logoutTouchMove = true;
    }, { passive: true });
    logoutBtn.addEventListener('touchend', (e) => {
      if (!logoutTouchMove) {
        e.preventDefault();
        this.onLogout();
      }
    });
    logoutBtn.addEventListener('click', (e) => {
      if (e.detail !== 0) {
        e.preventDefault();
        this.onLogout();
      }
    });
    shortcutsScroll.appendChild(logoutBtn);
    
    // Selection mode button (icon)
    this.selectBtn = document.createElement('button');
    this.selectBtn.className = 'keyboard-btn shortcut-btn select-btn';
    this.selectBtn.innerHTML = '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 7V4h3"/><path d="M20 7V4h-3"/><path d="M4 17v3h3"/><path d="M20 17v3h-3"/><line x1="9" y1="12" x2="15" y2="12"/></svg>';
    this.selectBtn.title = '选择模式';
    
    let selectTouchMove = false;
    let selectTouchStartX = 0;
    let selectTouchStartY = 0;
    this.selectBtn.addEventListener('touchstart', (e) => {
      selectTouchStartX = e.touches[0].clientX;
      selectTouchStartY = e.touches[0].clientY;
      selectTouchMove = false;
    }, { passive: true });
    this.selectBtn.addEventListener('touchmove', (e) => {
      const dx = Math.abs(e.touches[0].clientX - selectTouchStartX);
      const dy = Math.abs(e.touches[0].clientY - selectTouchStartY);
      if (dx > 10 || dy > 10) selectTouchMove = true;
    }, { passive: true });
    this.selectBtn.addEventListener('touchend', (e) => {
      if (!selectTouchMove) {
        e.preventDefault();
        this.toggleSelectionMode();
      }
    });
    this.selectBtn.addEventListener('click', (e) => {
      if (e.detail !== 0) {
        e.preventDefault();
        this.toggleSelectionMode();
      }
    });
    shortcutsScroll.appendChild(this.selectBtn);
    
    // Copy button (only visible in selection mode)
    this.copyBtn = document.createElement('button');
    this.copyBtn.className = 'keyboard-btn shortcut-btn copy-btn';
    this.copyBtn.innerHTML = '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>';
    this.copyBtn.title = '复制选中';
    this.copyBtn.style.display = 'none';  // Hidden by default
    
    let copyTouchMove = false;
    let copyTouchStartX = 0;
    let copyTouchStartY = 0;
    this.copyBtn.addEventListener('touchstart', (e) => {
      copyTouchStartX = e.touches[0].clientX;
      copyTouchStartY = e.touches[0].clientY;
      copyTouchMove = false;
    }, { passive: true });
    this.copyBtn.addEventListener('touchmove', (e) => {
      const dx = Math.abs(e.touches[0].clientX - copyTouchStartX);
      const dy = Math.abs(e.touches[0].clientY - copyTouchStartY);
      if (dx > 10 || dy > 10) copyTouchMove = true;
    }, { passive: true });
    this.copyBtn.addEventListener('touchend', (e) => {
      if (!copyTouchMove) {
        e.preventDefault();
        this.copySelection();
      }
    });
    this.copyBtn.addEventListener('click', (e) => {
      if (e.detail !== 0) {
        e.preventDefault();
        this.copySelection();
      }
    });
    shortcutsScroll.appendChild(this.copyBtn);
    
    // Ctrl shortcuts (^C, ^L)
    ctrlShortcuts.forEach((shortcut) => {
      const btn = this.createButton(shortcut.label, () => {
        this.terminal.sendKey(shortcut.key);
      });
      btn.classList.add('shortcut-btn');
      shortcutsScroll.appendChild(btn);
    });
    
    // Special keys
    specialKeys.forEach((key) => {
      if (key.label === 'Ctrl' || key.label === 'Alt') {
        // Ctrl/Alt with long-press lock support
        const btn = this.createModifierButton(key.label, key.label === 'Ctrl');
        btn.classList.add('shortcut-btn');
        if (key.label === 'Ctrl') this.ctrlButton = btn;
        else this.altButton = btn;
        shortcutsScroll.appendChild(btn);
      } else {
        const btn = this.createButton(key.label, () => {
          if (key.key) {
            this.sendWithModifiers(key.key);
          }
        });
        btn.classList.add('shortcut-btn');
        shortcutsScroll.appendChild(btn);
      }
    });
    
    // Arrow keys
    arrowKeys.forEach((key) => {
      const btn = this.createButton(key.label, () => {
        this.terminal.sendKey(key.key);
      });
      btn.classList.add('shortcut-btn');
      shortcutsScroll.appendChild(btn);
    });
    
    
    shortcutsRow.appendChild(shortcutsScroll);
    this.container.appendChild(shortcutsRow);
  }

  // Removed custom scroll handling - using native CSS scroll with hidden scrollbar

  private createRow(className: string): HTMLElement {
    const row = document.createElement('div');
    row.className = `keyboard-row ${className}`;
    return row;
  }

  private createButton(label: string, onClick: () => void): HTMLElement {
    const btn = document.createElement('button');
    btn.className = 'keyboard-btn';
    btn.textContent = label;
    
    // Track touch to distinguish tap from scroll
    let touchStartX = 0;
    let touchStartY = 0;
    let isTouchMove = false;
    
    btn.addEventListener('touchstart', (e) => {
      touchStartX = e.touches[0].clientX;
      touchStartY = e.touches[0].clientY;
      isTouchMove = false;
    }, { passive: true });
    
    btn.addEventListener('touchmove', (e) => {
      const dx = Math.abs(e.touches[0].clientX - touchStartX);
      const dy = Math.abs(e.touches[0].clientY - touchStartY);
      // If moved more than 10px, consider it a scroll
      if (dx > 10 || dy > 10) {
        isTouchMove = true;
      }
    }, { passive: true });
    
    btn.addEventListener('touchend', (e) => {
      if (!isTouchMove) {
        e.preventDefault();
        onClick();
      }
    });
    
    // Fallback for non-touch devices
    btn.addEventListener('click', (e) => {
      // Only handle if not from touch
      if (e.detail !== 0) {  // detail is 0 for keyboard/programmatic clicks
        e.preventDefault();
        onClick();
      }
    });
    
    return btn;
  }

  // Create modifier button (Ctrl/Alt) with double-tap lock support
  private createModifierButton(label: string, isCtrl: boolean): HTMLElement {
    const btn = document.createElement('button');
    btn.className = 'keyboard-btn';
    btn.textContent = label;
    
    const DOUBLE_TAP_DELAY = 300; // ms
    let lastTapTime = 0;
    let touchStartX = 0;
    let touchStartY = 0;
    let isTouchMove = false;
    
    const getActive = () => isCtrl ? this.ctrlActive : this.altActive;
    const setActive = (val: boolean) => {
      if (isCtrl) this.ctrlActive = val;
      else this.altActive = val;
    };
    const getLocked = () => isCtrl ? this.ctrlLocked : this.altLocked;
    const setLocked = (val: boolean) => {
      if (isCtrl) this.ctrlLocked = val;
      else this.altLocked = val;
    };
    
    const updateButtonState = () => {
      const active = getActive();
      const locked = getLocked();
      btn.classList.toggle('active', active && !locked);
      btn.classList.toggle('locked', locked);
    };
    
    const handleTap = () => {
      const now = Date.now();
      const isDoubleTap = (now - lastTapTime) < DOUBLE_TAP_DELAY;
      lastTapTime = now;
      
      if (isDoubleTap) {
        // Double tap: toggle lock
        if (getLocked()) {
          setLocked(false);
          setActive(false);
        } else {
          setLocked(true);
          setActive(true);
        }
      } else {
        // Single tap
        if (getLocked()) {
          // If locked, tap unlocks
          setLocked(false);
          setActive(false);
        } else {
          // Toggle active state
          setActive(!getActive());
        }
      }
      updateButtonState();
    };
    
    // Touch events
    btn.addEventListener('touchstart', (e) => {
      touchStartX = e.touches[0].clientX;
      touchStartY = e.touches[0].clientY;
      isTouchMove = false;
    }, { passive: true });
    
    btn.addEventListener('touchmove', (e) => {
      const dx = Math.abs(e.touches[0].clientX - touchStartX);
      const dy = Math.abs(e.touches[0].clientY - touchStartY);
      if (dx > 10 || dy > 10) {
        isTouchMove = true;
      }
    }, { passive: true });
    
    btn.addEventListener('touchend', (e) => {
      if (!isTouchMove) {
        e.preventDefault();
        handleTap();
      }
    });
    
    // Mouse events for non-touch devices
    btn.addEventListener('click', (e) => {
      if (e.detail !== 0) {
        e.preventDefault();
        handleTap();
      }
    });
    
    return btn;
  }

  private sendWithModifiers(key: string) {
    let modifiedKey = key;

    if (this.ctrlActive) {
      // Convert to control character
      if (key.length === 1) {
        const code = key.toUpperCase().charCodeAt(0);
        if (code >= 65 && code <= 90) {
          modifiedKey = String.fromCharCode(code - 64);
        }
      }
      // Only reset if not locked
      if (!this.ctrlLocked) {
        this.ctrlActive = false;
        this.updateModifierButtons();
      }
    }

    if (this.altActive) {
      modifiedKey = '\x1b' + modifiedKey;
      // Only reset if not locked
      if (!this.altLocked) {
        this.altActive = false;
        this.updateModifierButtons();
      }
    }

    this.terminal.sendKey(modifiedKey);
  }

  private updateModifierButtons() {
    // Use cached button references instead of querying all buttons
    if (this.ctrlButton) {
      this.ctrlButton.classList.toggle('active', this.ctrlActive && !this.ctrlLocked);
      this.ctrlButton.classList.toggle('locked', this.ctrlLocked);
    }
    if (this.altButton) {
      this.altButton.classList.toggle('active', this.altActive && !this.altLocked);
      this.altButton.classList.toggle('locked', this.altLocked);
    }
  }

  private sendInputCommand() {
    if (!this.inputArea) return;
    // Fix: Replace non-breaking space (U+00A0) with regular space
    const command = this.inputArea.value.replace(/\u00A0/g, ' ');
    debugLog('[Keyboard] sendInputCommand called, value:', JSON.stringify(command), 'isComposing:', this.isComposing);
    if (command.trim()) {
      // Send command with newline
      debugLog('[Keyboard] sending via sendKey:', JSON.stringify(command + '\n'));
      this.terminal.sendKey(command + '\n');
      this.inputArea.value = '';
      // Reset height in multiline mode
      if (this.isMultilineMode) {
        this.inputArea.style.height = '';
      }
    } else {
      // Send just a newline (Enter) if input is empty
      debugLog('[Keyboard] input empty, sending Enter');
      this.terminal.sendKey('\n');
    }
  }

  private toggleSelectionMode() {
    const newState = !this.terminal.isSelectionMode();
    this.terminal.setSelectionMode(newState);
    
    // Update button states
    if (this.selectBtn) {
      this.selectBtn.classList.toggle('active', newState);
    }
    if (this.copyBtn) {
      this.copyBtn.style.display = newState ? '' : 'none';
    }
    
    debugLog('[Keyboard] toggleSelectionMode:', newState);
  }

  private async copySelection() {
    const success = await this.terminal.copySelection();
    
    if (success) {
      // Visual feedback - briefly highlight the copy button
      if (this.copyBtn) {
        this.copyBtn.classList.add('copied');
        setTimeout(() => {
          this.copyBtn?.classList.remove('copied');
        }, 500);
      }
      debugLog('[Keyboard] copySelection: success');
    } else {
      debugLog('[Keyboard] copySelection: no selection or failed');
    }
  }

  dispose() {
    if (this.disposed) return;
    this.disposed = true;
    
    // Clean up all registered event listeners
    this.eventCleanups.forEach(cleanup => cleanup());
    this.eventCleanups = [];
    
    // Remove minimized button if present
    if (this.minimizedButton) {
      this.minimizedButton.remove();
      this.minimizedButton = null;
    }
    
    // Clear container
    this.container.innerHTML = '';
    
    // Clear references
    this.inputArea = null;
    this.ctrlButton = null;
    this.altButton = null;
    this.selectBtn = null;
    this.copyBtn = null;
  }
}
