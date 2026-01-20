import type { LoginResponse, SessionInfo } from './types';

class API {
  private token: string | null = null;

  setToken(token: string | null) {
    this.token = token;
    if (token) {
      localStorage.setItem('token', token);
    } else {
      localStorage.removeItem('token');
    }
  }

  getToken(): string | null {
    if (!this.token) {
      this.token = localStorage.getItem('token');
    }
    return this.token;
  }

  private async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string>),
    };

    const token = this.getToken();
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    const response = await fetch(path, {
      ...options,
      headers,
    });

    if (!response.ok) {
      const errorText = await response.text();
      if (errorText) {
        try {
          const errorJson = JSON.parse(errorText) as { error?: string; message?: string };
          throw new Error(errorJson.error || errorJson.message || response.statusText || 'Request failed');
        } catch {
          throw new Error(errorText);
        }
      }
      throw new Error(response.statusText || 'Request failed');
    }

    if (response.status === 204 || response.status === 205) {
      return undefined as T;
    }

    const text = await response.text();
    if (!text) {
      return undefined as T;
    }

    try {
      return JSON.parse(text) as T;
    } catch {
      throw new Error('Invalid JSON response');
    }
  }

  async login(username: string, password: string): Promise<LoginResponse> {
    const response = await this.request<LoginResponse>('/api/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    });
    this.setToken(response.token);
    return response;
  }

  async logout(): Promise<void> {
    await this.request<void>('/api/logout', { method: 'POST' });
    this.setToken(null);
  }

  async listSessions(): Promise<SessionInfo[]> {
    return this.request<SessionInfo[]>('/api/sessions');
  }

  async createSession(): Promise<SessionInfo> {
    return this.request<SessionInfo>('/api/sessions', { method: 'POST' });
  }

  async deleteSession(id: string): Promise<void> {
    await this.request<void>(`/api/sessions/${id}`, { method: 'DELETE' });
  }

  createWebSocket(sessionId: string): WebSocket {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const token = this.getToken();
    const url = `${protocol}//${window.location.host}/ws/${sessionId}?token=${token}`;
    return new WebSocket(url);
  }
}

export const api = new API();
