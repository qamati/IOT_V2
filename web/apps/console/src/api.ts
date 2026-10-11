export interface User {
  id: string;
  tenant_id: string;
  email: string;
  authority: string;
  first_name?: string;
  last_name?: string;
}

export interface LoginResponse {
  token: string;
  user: User;
}

export interface Device {
  id: string;
  tenant_id: string;
  profile_id?: string;
  name: string;
  label?: string;
  created_time: number;
  updated_time: number;
}

export interface DeviceCredentials {
  id: string;
  device_id: string;
  credentials_type: string;
  credentials_id: string;
  credentials_value: string;
}

export interface CreateDeviceResult {
  device: Device;
  credentials: DeviceCredentials;
}

export interface TelemetryPoint {
  ts: number;
  value: any;
}

export type TelemetryMap = Record<string, TelemetryPoint[]>;

export interface AttributeKv {
  entity_id: string;
  scope: 'CLIENT_SCOPE' | 'SERVER_SCOPE' | 'SHARED_SCOPE';
  key: string;
  last_update_ts: number;
  value: any;
}

export interface DeviceActivity {
  device_id: string;
  last_activity_time: number;
  active: boolean;
}

export interface Alarm {
  id: string;
  created_time: number;
  tenant_id: string;
  originator_id: string;
  type: string;
  severity: 'CRITICAL' | 'MAJOR' | 'MINOR' | 'WARNING' | 'INDETERMINATE';
  status: 'ACTIVE_UNACK' | 'ACTIVE_ACK' | 'CLEARED_UNACK' | 'CLEARED_ACK';
  start_ts: number;
  end_ts?: number;
  ack_ts?: number;
  clear_ts?: number;
  details?: Record<string, any>;
}

const BASE_URL = '/api/v1';

class ApiClient {
  private token: string | null = null;

  constructor() {
    this.token = localStorage.getItem('iotp_token');
  }

  setToken(token: string | null) {
    this.token = token;
    if (token) {
      localStorage.setItem('iotp_token', token);
    } else {
      localStorage.removeItem('iotp_token');
    }
  }

  getToken(): string | null {
    return this.token;
  }

  private async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const headers = new Headers(options.headers || {});
    if (this.token && !headers.has('Authorization')) {
      headers.set('Authorization', `Bearer ${this.token}`);
    }
    if (!headers.has('Content-Type') && options.body && typeof options.body === 'string') {
      headers.set('Content-Type', 'application/json');
    }

    const res = await fetch(`${BASE_URL}${path}`, {
      ...options,
      headers,
    });

    if (!res.ok) {
      let errText = await res.text();
      try {
        const errJson = JSON.parse(errText);
        throw new Error(errJson.detail || errJson.title || errText);
      } catch (e: any) {
        throw new Error(e.message || `HTTP error ${res.status}`);
      }
    }

    if (res.status === 204) {
      return {} as T;
    }

    const contentType = res.headers.get('Content-Type');
    if (contentType && contentType.includes('application/json')) {
      return res.json();
    }
    return {} as T;
  }

  async login(email: string, password: string): Promise<LoginResponse> {
    const res = await this.request<LoginResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
    this.setToken(res.token);
    return res;
  }

  logout() {
    this.setToken(null);
  }

  async listDevices(page = 1, pageSize = 50): Promise<{ data: Device[]; total: number }> {
    return this.request<{ data: Device[]; total: number }>(`/devices?page=${page}&pageSize=${pageSize}`);
  }

  async getDevice(id: string): Promise<Device> {
    return this.request<Device>(`/devices/${id}`);
  }

  async getCredentials(id: string): Promise<DeviceCredentials> {
    return this.request<DeviceCredentials>(`/devices/${id}/credentials`);
  }

  async getDeviceActivity(id: string): Promise<DeviceActivity> {
    return this.request<DeviceActivity>(`/devices/${id}/activity`);
  }

  async createDevice(name: string, label: string, token: string): Promise<CreateDeviceResult> {
    return this.request<CreateDeviceResult>('/devices', {
      method: 'POST',
      body: JSON.stringify({ name, label, token }),
    });
  }

  async deleteDevice(id: string): Promise<void> {
    await this.request(`/devices/${id}`, {
      method: 'DELETE',
    });
  }

  // Device telemetry upload
  async uploadTelemetry(deviceToken: string, payload: Record<string, any>): Promise<void> {
    await fetch(`${BASE_URL}/${deviceToken}/telemetry`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
  }

  // Device attributes upload
  async uploadClientAttributes(deviceToken: string, payload: Record<string, any>): Promise<void> {
    await fetch(`${BASE_URL}/${deviceToken}/attributes`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
  }

  async getLatestTelemetry(deviceId: string, keys?: string[]): Promise<TelemetryMap> {
    const q = keys && keys.length > 0 ? `?keys=${keys.join(',')}` : '';
    return this.request<TelemetryMap>(`/telemetry/DEVICE/${deviceId}/values/latest${q}`);
  }

  async getTimeseries(deviceId: string, keys?: string[], startTs?: number, endTs?: number, limit = 100): Promise<TelemetryMap> {
    const params = new URLSearchParams();
    if (keys && keys.length > 0) params.set('keys', keys.join(','));
    if (startTs) params.set('startTs', startTs.toString());
    if (endTs) params.set('endTs', endTs.toString());
    if (limit) params.set('limit', limit.toString());
    const q = params.toString() ? `?${params.toString()}` : '';
    return this.request<TelemetryMap>(`/telemetry/DEVICE/${deviceId}/values/timeseries${q}`);
  }

  // Attributes
  async getAllAttributes(deviceId: string): Promise<AttributeKv[]> {
    return this.request<AttributeKv[]>(`/plugins/telemetry/DEVICE/${deviceId}/values/attributes`);
  }

  async saveScopedAttributes(deviceId: string, scope: 'CLIENT_SCOPE' | 'SERVER_SCOPE' | 'SHARED_SCOPE', attributes: Record<string, any>): Promise<void> {
    await this.request(`/plugins/telemetry/DEVICE/${deviceId}/values/attributes/${scope}`, {
      method: 'POST',
      body: JSON.stringify(attributes),
    });
  }

  async deleteScopedAttributes(deviceId: string, scope: 'CLIENT_SCOPE' | 'SERVER_SCOPE' | 'SHARED_SCOPE', keys: string[]): Promise<void> {
    await this.request(`/plugins/telemetry/DEVICE/${deviceId}/values/attributes/${scope}?keys=${keys.join(',')}`, {
      method: 'DELETE',
    });
  }

  // Alarms
  async listAlarms(page = 1, pageSize = 50, originatorId?: string): Promise<{ data: Alarm[]; total: number }> {
    let q = `?page=${page}&pageSize=${pageSize}`;
    if (originatorId) q += `&originatorId=${originatorId}`;
    return this.request<{ data: Alarm[]; total: number }>(`/alarms${q}`);
  }

  async ackAlarm(alarmId: string): Promise<Alarm> {
    return this.request<Alarm>(`/alarms/${alarmId}/ack`, { method: 'POST' });
  }

  async clearAlarm(alarmId: string): Promise<Alarm> {
    return this.request<Alarm>(`/alarms/${alarmId}/clear`, { method: 'POST' });
  }
}

export const api = new ApiClient();
