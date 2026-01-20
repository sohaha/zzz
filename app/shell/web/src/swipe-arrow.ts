/**
 * SwipeArrowController - Long-press and swipe to send arrow keys
 * 
 * Features:
 * - Long press (500ms) to activate arrow key mode
 * - Swipe in any direction to send corresponding arrow key
 * - Distance from origin determines repeat frequency
 * - Haptic feedback on each key sent (Android only, iOS uses visual feedback)
 */

// Debug mode - only log when ?debug=1 is in URL
const DEBUG = typeof window !== 'undefined' && window.location.search.includes('debug=1');

function debugLog(...args: unknown[]) {
  if (DEBUG) {
    console.log('[SwipeArrow]', ...args);
  }
}

// Simple vibration helper - only works on Android
function vibrate(duration: number) {
  if ('vibrate' in navigator) {
    try {
      navigator.vibrate(duration);
    } catch {
      // Ignore errors
    }
  }
}

// Arrow key escape sequences
const ARROW_KEYS = {
  up: '\x1b[A',
  down: '\x1b[B',
  left: '\x1b[D',
  right: '\x1b[C',
} as const;

type Direction = keyof typeof ARROW_KEYS;

interface SwipeArrowOptions {
  /** Callback to send key to terminal */
  onSendKey: (key: string) => void;
  /** Callback to temporarily disable textarea focus (for iOS Safari) */
  onGestureActive?: (active: boolean) => void;
  /** Long press delay in ms (default: 500) */
  longPressDelay?: number;
  /** Minimum distance to trigger direction (default: 20px) */
  minDistance?: number;
  /** Maximum distance for slowest repeat (default: 50px) */
  minRepeatDistance?: number;
  /** Minimum distance for fastest repeat (default: 150px) */
  maxRepeatDistance?: number;
  /** Slowest repeat interval in ms (default: 500) */
  slowRepeatInterval?: number;
  /** Fastest repeat interval in ms (default: 50) */
  fastRepeatInterval?: number;
}

export class SwipeArrowController {
  private element: HTMLElement;
  private options: Required<SwipeArrowOptions>;
  
  // Touch state
  private touchId: number | null = null;
  private startX = 0;
  private startY = 0;
  private currentX = 0;
  private currentY = 0;
  
  // Long press state
  private longPressTimer: ReturnType<typeof setTimeout> | null = null;
  private isActive = false;
  
  // Repeat state
  private repeatTimer: ReturnType<typeof setInterval> | null = null;
  private currentDirection: Direction | null = null;
  private lastSentTime = 0;
  
  // Visual indicator
  private indicator: HTMLElement | null = null;
  private directionIndicator: HTMLElement | null = null;
  
  // Bound event handlers for cleanup
  private boundHandlers: {
    touchstart: (e: TouchEvent) => void;
    touchmove: (e: TouchEvent) => void;
    touchend: (e: TouchEvent) => void;
    touchcancel: (e: TouchEvent) => void;
    click: (e: MouseEvent) => void;
  };
  
  // Flag to block next click event (for iOS Safari)
  private blockNextClick = false;
  
  constructor(element: HTMLElement, options: SwipeArrowOptions) {
    this.element = element;
    this.options = {
      longPressDelay: 500,
      minDistance: 20,
      minRepeatDistance: 50,
      maxRepeatDistance: 150,
      slowRepeatInterval: 500,
      fastRepeatInterval: 50,
      ...options,
    };
    
    // Bind handlers
    this.boundHandlers = {
      touchstart: this.handleTouchStart.bind(this),
      touchmove: this.handleTouchMove.bind(this),
      touchend: this.handleTouchEnd.bind(this),
      touchcancel: this.handleTouchEnd.bind(this),
      click: this.handleClick.bind(this),
    };
    
    this.attach();
  }
  
  private attach() {
    // Use capture phase to ensure we get events before they reach the terminal textarea
    // This is critical for preventing keyboard popup on touchend
    this.element.addEventListener('touchstart', this.boundHandlers.touchstart, { passive: false, capture: true });
    this.element.addEventListener('touchend', this.boundHandlers.touchend, { passive: false, capture: true });
    this.element.addEventListener('touchcancel', this.boundHandlers.touchcancel, { passive: false, capture: true });
    // Block click events that follow our gesture (iOS Safari generates click after touchend)
    this.element.addEventListener('click', this.boundHandlers.click, { passive: false, capture: true });
    // Listen on document for touchmove to ensure we always get them
    // even if pointer-events:none is applied to the element
    document.addEventListener('touchmove', this.boundHandlers.touchmove, { passive: false });
  }
  
  private handleTouchStart(e: TouchEvent) {
    // Only track single touch
    if (e.touches.length !== 1) {
      this.reset();
      return;
    }
    
    const touch = e.touches[0];
    this.touchId = touch.identifier;
    this.startX = touch.clientX;
    this.startY = touch.clientY;
    this.currentX = touch.clientX;
    this.currentY = touch.clientY;
    
    debugLog('touchstart', { x: this.startX, y: this.startY });
    
    // Start long press timer
    this.longPressTimer = setTimeout(() => {
      this.activate();
    }, this.options.longPressDelay);
  }
  
  private handleTouchMove(e: TouchEvent) {
    // Only process if we're tracking a touch
    if (this.touchId === null) return;
    
    const touch = this.getTrackedTouch(e.touches);
    if (!touch) return;
    
    this.currentX = touch.clientX;
    this.currentY = touch.clientY;
    
    const dx = this.currentX - this.startX;
    const dy = this.currentY - this.startY;
    const distance = Math.sqrt(dx * dx + dy * dy);
    
    // If moved too much before long press, cancel
    if (!this.isActive && distance > this.options.minDistance) {
      this.cancelLongPress();
      return;
    }
    
    // If active, handle direction
    if (this.isActive) {
      e.preventDefault(); // Prevent scrolling while in arrow mode
      this.updateDirection(dx, dy, distance);
    }
  }
  
  private handleTouchEnd(e: TouchEvent) {
    // Only process if we're tracking a touch
    if (this.touchId === null) return;
    
    debugLog('touchend', { isActive: this.isActive, touchId: this.touchId });
    
    // Check if our tracked touch ended
    const touch = this.getTrackedTouch(e.changedTouches);
    if (touch) {
      // Our touch ended
      if (this.isActive) {
        // IMPORTANT: Prevent the touchend from reaching the terminal textarea
        // This stops the keyboard from popping up when the gesture ends
        e.preventDefault();
        e.stopPropagation();
        e.stopImmediatePropagation();
        // iOS Safari: block the synthetic click event that follows touchend
        this.blockNextClick = true;
        setTimeout(() => { this.blockNextClick = false; }, 500);
      }
      this.reset();
      return;
    }
    
    // Check if our touch is still in the active touches
    const stillActive = this.getTrackedTouch(e.touches);
    if (!stillActive) {
      // Our touch is gone
      this.reset();
    }
  }
  
  private getTrackedTouch(touches: TouchList): Touch | null {
    if (this.touchId === null) return null;
    for (let i = 0; i < touches.length; i++) {
      if (touches[i].identifier === this.touchId) {
        return touches[i];
      }
    }
    return null;
  }
  
  private activate() {
    this.isActive = true;
    this.longPressTimer = null;
    
    debugLog('activated');
    
    // Notify that gesture is active - this allows the terminal to temporarily
    // disable the textarea to prevent keyboard popup on iOS Safari
    this.options.onGestureActive?.(true);
    
    // Haptic feedback for activation
    this.vibrate(50);
    
    // Show visual indicator
    this.showIndicator();
  }
  
  private cancelLongPress() {
    if (this.longPressTimer) {
      clearTimeout(this.longPressTimer);
      this.longPressTimer = null;
    }
  }
  
  private reset() {
    const wasActive = this.isActive;
    
    this.cancelLongPress();
    this.stopRepeat();
    this.hideIndicator();
    
    this.touchId = null;
    this.isActive = false;
    this.currentDirection = null;
    
    // Notify that gesture ended - restore textarea state
    if (wasActive) {
      this.options.onGestureActive?.(false);
    }
  }
  
  private updateDirection(dx: number, dy: number, distance: number) {
    // Determine direction based on which axis has larger absolute value
    let direction: Direction | null = null;
    
    if (distance >= this.options.minDistance) {
      if (Math.abs(dx) > Math.abs(dy)) {
        direction = dx > 0 ? 'right' : 'left';
      } else {
        direction = dy > 0 ? 'down' : 'up';
      }
    }
    
    // Update visual indicator
    this.updateIndicator(dx, dy, direction);
    
    // Handle direction change
    if (direction !== this.currentDirection) {
      this.currentDirection = direction;
      this.stopRepeat();
      
      if (direction) {
        // Send first key immediately
        this.sendArrowKey(direction);
        
        // Start repeating
        this.startRepeat(distance);
      }
    } else if (direction) {
      // Update repeat rate based on distance
      this.updateRepeatRate(distance);
    }
  }
  
  private sendArrowKey(direction: Direction) {
    const key = ARROW_KEYS[direction];
    this.options.onSendKey(key);
    this.lastSentTime = Date.now();
    
    // Haptic feedback
    this.vibrate(10);
    
    debugLog('sent', direction);
  }
  
  private startRepeat(distance: number) {
    const interval = this.calculateInterval(distance);
    
    this.repeatTimer = setInterval(() => {
      if (this.currentDirection) {
        this.sendArrowKey(this.currentDirection);
      }
    }, interval);
    
    debugLog('repeat started', { interval });
  }
  
  private stopRepeat() {
    if (this.repeatTimer) {
      clearInterval(this.repeatTimer);
      this.repeatTimer = null;
    }
  }
  
  private updateRepeatRate(distance: number) {
    // Recalculate interval and restart if significantly different
    const newInterval = this.calculateInterval(distance);
    
    // Only restart if interval changed significantly (>20%)
    if (this.repeatTimer) {
      this.stopRepeat();
      this.repeatTimer = setInterval(() => {
        if (this.currentDirection) {
          this.sendArrowKey(this.currentDirection);
        }
      }, newInterval);
    }
  }
  
  private calculateInterval(distance: number): number {
    const { minRepeatDistance, maxRepeatDistance, slowRepeatInterval, fastRepeatInterval } = this.options;
    
    // Clamp distance to range
    const clampedDistance = Math.max(minRepeatDistance, Math.min(maxRepeatDistance, distance));
    
    // Linear interpolation: closer = slower, farther = faster
    const t = (clampedDistance - minRepeatDistance) / (maxRepeatDistance - minRepeatDistance);
    const interval = slowRepeatInterval - t * (slowRepeatInterval - fastRepeatInterval);
    
    return Math.round(interval);
  }
  
  private vibrate(duration: number) {
    vibrate(duration);
    // Also trigger visual feedback pulse on the direction indicator
    this.pulseIndicator();
  }
  
  private pulseIndicator() {
    if (this.directionIndicator) {
      this.directionIndicator.classList.remove('pulse');
      // Force reflow to restart animation
      void this.directionIndicator.offsetWidth;
      this.directionIndicator.classList.add('pulse');
    }
  }
  
  private showIndicator() {
    // Create indicator overlay at touch position
    this.indicator = document.createElement('div');
    this.indicator.className = 'swipe-arrow-indicator';
    this.indicator.style.left = `${this.startX}px`;
    this.indicator.style.top = `${this.startY}px`;
    
    // Direction indicator (shows current direction)
    this.directionIndicator = document.createElement('div');
    this.directionIndicator.className = 'swipe-arrow-direction';
    this.indicator.appendChild(this.directionIndicator);
    
    // Add crosshair lines
    const crosshair = document.createElement('div');
    crosshair.className = 'swipe-arrow-crosshair';
    this.indicator.appendChild(crosshair);
    
    document.body.appendChild(this.indicator);
  }
  
  private updateIndicator(dx: number, dy: number, direction: Direction | null) {
    if (!this.directionIndicator) return;
    
    // Update direction indicator position to show current finger position
    const maxOffset = 40; // Maximum visual offset
    const scale = Math.min(1, Math.sqrt(dx * dx + dy * dy) / this.options.maxRepeatDistance);
    const visualDx = (dx / Math.max(Math.abs(dx), Math.abs(dy)) || 0) * maxOffset * scale;
    const visualDy = (dy / Math.max(Math.abs(dx), Math.abs(dy)) || 0) * maxOffset * scale;
    
    // Use CSS variables for position so pulse animation can preserve it
    this.directionIndicator.style.setProperty('--tx', `${visualDx}px`);
    this.directionIndicator.style.setProperty('--ty', `${visualDy}px`);
    this.directionIndicator.style.transform = `translate(${visualDx}px, ${visualDy}px)`;
    
    // Update direction class (preserve pulse class if present)
    const hadPulse = this.directionIndicator.classList.contains('pulse');
    this.directionIndicator.className = 'swipe-arrow-direction';
    if (direction) {
      this.directionIndicator.classList.add(`direction-${direction}`);
    }
    if (hadPulse) {
      this.directionIndicator.classList.add('pulse');
    }
  }
  
  private hideIndicator() {
    if (this.indicator) {
      this.indicator.remove();
      this.indicator = null;
      this.directionIndicator = null;
    }
  }
  
  // Handle click events - block synthetic clicks after gesture on iOS Safari
  private handleClick(e: MouseEvent) {
    if (this.blockNextClick) {
      debugLog('blocking click after gesture');
      e.preventDefault();
      e.stopPropagation();
      e.stopImmediatePropagation();
      this.blockNextClick = false;
    }
  }
  
  dispose() {
    this.reset();
    
    this.element.removeEventListener('touchstart', this.boundHandlers.touchstart, { capture: true });
    this.element.removeEventListener('touchend', this.boundHandlers.touchend, { capture: true });
    this.element.removeEventListener('touchcancel', this.boundHandlers.touchcancel, { capture: true });
    this.element.removeEventListener('click', this.boundHandlers.click, { capture: true });
    document.removeEventListener('touchmove', this.boundHandlers.touchmove);
  }
}
