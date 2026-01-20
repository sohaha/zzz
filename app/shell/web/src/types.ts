export interface LoginResponse {
  token: string;
  expires_in: number;
}

export interface SessionInfo {
  id: string;
  created_at: string;
}

export interface WSMessage {
  type: 'input' | 'output' | 'resize' | 'ping' | 'pong';
  data: unknown;
}

export interface ResizeData {
  rows: number;
  cols: number;
}

// Wake Lock API types (not yet in all TypeScript versions)
declare global {
  interface Navigator {
    wakeLock?: WakeLock;
  }

  interface WakeLock {
    request(type: 'screen'): Promise<WakeLockSentinel>;
  }

  interface WakeLockSentinel extends EventTarget {
    readonly released: boolean;
    readonly type: 'screen';
    release(): Promise<void>;
  }
}
