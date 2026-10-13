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
  last_connect_time?: number;
  last_disconnect_time?: number;
  inactivity_timeout_ms?: number;
}

export interface DeviceState {
  device_id: string;
  tenant_id?: string;
  active: boolean;
  last_connect_time: number;
  last_disconnect_time: number;
  last_activity_time: number;
  inactivity_timeout_ms: number;
  inactivity_alarm_time?: number;
  deadline: number;
  overridden?: boolean;
}

export interface StateSummary {
  tenant_id: string;
  total: number;
  active: number;
  inactive: number;
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

  async getDeviceState(id: string): Promise<DeviceState> {
    return this.request<DeviceState>(`/devices/${id}/state`);
  }

  async overrideDeviceState(id: string, active: boolean): Promise<DeviceState> {
    return this.request<DeviceState>(`/devices/${id}/state/override`, {
      method: 'POST',
      body: JSON.stringify({ active }),
    });
  }

  async getDevStateSummary(): Promise<StateSummary> {
    return this.request<StateSummary>('/devstate/summary');
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

  // Dashboards
  async listDashboards(page = 1, pageSize = 50): Promise<{ data: Dashboard[]; total: number }> {
    return this.request<{ data: Dashboard[]; total: number }>(`/dashboards?page=${page}&pageSize=${pageSize}`);
  }

  async getDashboard(id: string): Promise<Dashboard> {
    return this.request<Dashboard>(`/dashboards/${id}`);
  }

  async createDashboard(title: string, configuration: any): Promise<Dashboard> {
    return this.request<Dashboard>('/dashboards', {
      method: 'POST',
      body: JSON.stringify({ title, configuration }),
    });
  }

  async deleteDashboard(id: string): Promise<void> {
    await this.request(`/dashboards/${id}`, { method: 'DELETE' });
  }

  // Downlink RPC
  async sendOneWayRpc(deviceId: string, method: string, params: any): Promise<RpcCommand> {
    return this.request<RpcCommand>(`/plugins/rpc/oneway/DEVICE/${deviceId}`, {
      method: 'POST',
      body: JSON.stringify({ method, params }),
    });
  }

  async sendTwoWayRpc(deviceId: string, method: string, params: any, timeoutMs = 3000): Promise<RpcCommand> {
    return this.request<RpcCommand>(`/plugins/rpc/twoway/DEVICE/${deviceId}?timeout=${timeoutMs}`, {
      method: 'POST',
      body: JSON.stringify({ method, params }),
    });
  }

  async getRpcHistory(deviceId: string): Promise<{ data: RpcCommand[]; total: number }> {
    return this.request<{ data: RpcCommand[]; total: number }>(`/plugins/rpc/DEVICE/${deviceId}`);
  }

  // Device-side RPC polling & response simulation
  async pollDeviceRpc(deviceToken: string): Promise<RpcCommand | null> {
    const res = await fetch(`${BASE_URL}/${deviceToken}/rpc`);
    if (res.status === 204) return null;
    if (!res.ok) return null;
    return res.json();
  }

  async respondDeviceRpc(deviceToken: string, requestId: string, response: any): Promise<void> {
    await fetch(`${BASE_URL}/${deviceToken}/rpc/${requestId}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(response),
    });
  }

  async getPersistentRpc(commandId: string): Promise<RpcCommand> {
    return this.request<RpcCommand>(`/rpc/persistent/${commandId}`);
  }

  async deletePersistentRpc(commandId: string): Promise<void> {
    await this.request(`/rpc/persistent/${commandId}`, { method: 'DELETE' });
  }

  // --- Stage 7: Bulk Devices Import & Export ---
  async bulkImportDevices(data: any): Promise<BulkImportResult> {
    const isString = typeof data === 'string';
    return this.request<BulkImportResult>('/devices/bulk/import', {
      method: 'POST',
      headers: isString ? { 'Content-Type': 'text/plain' } : { 'Content-Type': 'application/json' },
      body: isString ? data : JSON.stringify(data),
    });
  }

  async bulkExportDevicesCsv(): Promise<string> {
    const res = await fetch(`${BASE_URL}/devices/bulk/export`, {
      headers: {
        ...(this.token ? { Authorization: `Bearer ${this.token}` } : {}),
      },
    });
    if (!res.ok) throw new Error('Failed to export devices CSV');
    return res.text();
  }

  // --- Stage 7: OTA Firmware & Software Management ---
  async listOtaPackages(type?: string, page = 1, pageSize = 50): Promise<{ data: OtaPackage[]; total: number }> {
    let q = `?page=${page}&pageSize=${pageSize}`;
    if (type) q += `&type=${type}`;
    return this.request<{ data: OtaPackage[]; total: number }>(`/otaPackages${q}`);
  }

  async createOtaPackage(pkg: { type: 'FIRMWARE' | 'SOFTWARE'; title: string; version: string; tag?: string; url?: string }): Promise<OtaPackage> {
    return this.request<OtaPackage>('/otaPackages', {
      method: 'POST',
      body: JSON.stringify(pkg),
    });
  }

  async getOtaPackage(id: string): Promise<OtaPackage> {
    return this.request<OtaPackage>(`/otaPackages/${id}`);
  }

  async deleteOtaPackage(id: string): Promise<void> {
    await this.request(`/otaPackages/${id}`, { method: 'DELETE' });
  }

  async uploadOtaBinary(packageId: string, file: File): Promise<OtaPackage> {
    const formData = new FormData();
    formData.append('file', file);
    const res = await fetch(`${BASE_URL}/otaPackages/${packageId}/upload`, {
      method: 'POST',
      headers: {
        ...(this.token ? { Authorization: `Bearer ${this.token}` } : {}),
      },
      body: formData,
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ message: res.statusText }));
      throw new Error(err.message || 'Binary upload failed');
    }
    return res.json();
  }

  async assignOtaPackage(deviceId: string, packageId: string, kind: 'FIRMWARE' | 'SOFTWARE'): Promise<OtaAssignment> {
    return this.request<OtaAssignment>(`/devices/${deviceId}/ota/assign`, {
      method: 'POST',
      body: JSON.stringify({ packageId, kind }),
    });
  }

  async getDeviceOtaState(deviceId: string, kind = 'FIRMWARE'): Promise<DeviceOtaState> {
    return this.request<DeviceOtaState>(`/devices/${deviceId}/ota/state?kind=${kind}`);
  }

  async listFleetOtaStates(packageId: string): Promise<DeviceOtaState[]> {
    const res = await this.request<DeviceOtaState[]>(`/otaPackages/${packageId}/fleet`);
    return Array.isArray(res) ? res : [];
  }

  // --- Stage 8: Integrations & Converters ---
  async listConverters(): Promise<Converter[]> {
    const res = await this.request<Converter[]>('/converters');
    return Array.isArray(res) ? res : [];
  }

  async createConverter(converter: Partial<Converter>): Promise<Converter> {
    return this.request<Converter>('/converters', {
      method: 'POST',
      body: JSON.stringify(converter),
    });
  }

  async deleteConverter(id: string): Promise<void> {
    return this.request<void>(`/converters/${id}`, {
      method: 'DELETE',
    });
  }

  async testConverter(converter: Converter, payload: string, metadata: Record<string, string> = {}): Promise<{ success: boolean; event: any }> {
    return this.request<{ success: boolean; event: any }>('/converters/test', {
      method: 'POST',
      body: JSON.stringify({ converter, payload, metadata }),
    });
  }

  async listIntegrations(): Promise<Integration[]> {
    const res = await this.request<Integration[]>('/integrations');
    return Array.isArray(res) ? res : [];
  }

  async createIntegration(integration: Partial<Integration>): Promise<Integration> {
    return this.request<Integration>('/integrations', {
      method: 'POST',
      body: JSON.stringify(integration),
    });
  }

  async deleteIntegration(id: string): Promise<void> {
    return this.request<void>(`/integrations/${id}`, {
      method: 'DELETE',
    });
  }

  async getIntegrationStatus(id: string): Promise<IntegrationStatus> {
    return this.request<IntegrationStatus>(`/integrations/${id}/status`);
  }

  async getIntegrationDebug(id: string): Promise<DebugEvent[]> {
    const res = await this.request<DebugEvent[]>(`/integrations/${id}/debug`);
    return Array.isArray(res) ? res : [];
  }

  async sendIntegrationWebhook(routingKey: string, payload: any): Promise<any> {
    const res = await fetch(`${BASE_URL}/integrations/webhook/${routingKey}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: typeof payload === 'string' ? payload : JSON.stringify(payload),
    });
    return res.json();
  }

  // --- Stage 9: Edge Fleet ---
  async listEdgeNodes(): Promise<EdgeNode[]> {
    const res = await this.request<EdgeNode[]>('/edge');
    return Array.isArray(res) ? res : [];
  }

  async createEdgeNode(node: Partial<EdgeNode>): Promise<EdgeNode> {
    return this.request<EdgeNode>('/edge', {
      method: 'POST',
      body: JSON.stringify(node),
    });
  }

  async deleteEdgeNode(id: string): Promise<void> {
    await this.request<void>(`/edge/${id}`, { method: 'DELETE' });
  }

  async assignEdgeDevices(edgeId: string, deviceIds: string[]): Promise<any> {
    return this.request(`/edge/${edgeId}/assignments`, {
      method: 'POST',
      body: JSON.stringify({ device_ids: deviceIds }),
    });
  }

  async unassignEdgeDevice(edgeId: string, deviceId: string): Promise<void> {
    await this.request<void>(`/edge/${edgeId}/assignments/${deviceId}`, { method: 'DELETE' });
  }

  async getEdgeAssignments(edgeId: string): Promise<string[]> {
    const res = await this.request<string[]>(`/edge/${edgeId}/assignments`);
    return Array.isArray(res) ? res : [];
  }

  async executeEdgeCommand(edgeId: string, command: string, params?: Record<string, any>): Promise<any> {
    return this.request(`/edge/${edgeId}/commands`, {
      method: 'POST',
      body: JSON.stringify({ command, params }),
    });
  }

  async getEdgeJournal(edgeId: string, limit = 20): Promise<SyncJournalEntry[]> {
    const res = await this.request<SyncJournalEntry[]>(`/edge/${edgeId}/journal?limit=${limit}`);
    return Array.isArray(res) ? res : [];
  }

  async triggerEdgeSync(batch: any): Promise<any> {
    const res = await fetch(`${BASE_URL}/edge/sync`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(batch),
    });
    return res.json();
  }

  // --- Stage 10: SCADA ---
  async listScadaTags(): Promise<ScadaTag[]> {
    const res = await this.request<ScadaTag[]>('/scada/tags');
    return Array.isArray(res) ? res : [];
  }

  async updateScadaTag(path: string, value: any, quality = 'GOOD'): Promise<{ tag: ScadaTag; emitted: boolean }> {
    return this.request<{ tag: ScadaTag; emitted: boolean }>('/scada/tags/update', {
      method: 'POST',
      body: JSON.stringify({ path, value, quality }),
    });
  }

  async listScadaAlarms(): Promise<ISA182Alarm[]> {
    const res = await this.request<ISA182Alarm[]>('/scada/alarms');
    return Array.isArray(res) ? res : [];
  }

  async ackScadaAlarm(id: string): Promise<ISA182Alarm> {
    return this.request<ISA182Alarm>(`/scada/alarms/${id}/ack`, { method: 'POST' });
  }

  async shelveScadaAlarm(id: string, durationMs = 300000): Promise<ISA182Alarm> {
    return this.request<ISA182Alarm>(`/scada/alarms/${id}/shelve`, {
      method: 'POST',
      body: JSON.stringify({ duration_ms: durationMs }),
    });
  }

  async unshelveScadaAlarm(id: string): Promise<ISA182Alarm> {
    return this.request<ISA182Alarm>(`/scada/alarms/${id}/unshelve`, { method: 'POST' });
  }

  async getScadaAlarmKPIs(): Promise<AlarmKPIs> {
    return this.request<AlarmKPIs>('/scada/alarms/kpis');
  }

  async selectScadaControl(tagPath: string, desiredValue: any): Promise<ControlCommand> {
    return this.request<ControlCommand>('/scada/control/select', {
      method: 'POST',
      body: JSON.stringify({ tag_path: tagPath, desired_value: desiredValue }),
    });
  }

  async operateScadaControl(selectId: string): Promise<ControlCommand> {
    return this.request<ControlCommand>('/scada/control/operate', {
      method: 'POST',
      body: JSON.stringify({ select_id: selectId }),
    });
  }

  async listScadaControlLogs(limit = 50): Promise<ControlLogEntry[]> {
    const res = await this.request<ControlLogEntry[]>(`/scada/control/log?limit=${limit}`);
    return Array.isArray(res) ? res : [];
  }

  async listScadaSymbols(category = ''): Promise<ScadaSymbol[]> {
    const query = category ? `?category=${encodeURIComponent(category)}` : '';
    const res = await this.request<ScadaSymbol[]>(`/scada/symbols${query}`);
    return Array.isArray(res) ? res : [];
  }

  async getScadaSymbol(id: string): Promise<ScadaSymbol> {
    return this.request<ScadaSymbol>(`/scada/symbols/${id}`);
  }

  async saveScadaSymbol(symbol: Partial<ScadaSymbol>): Promise<ScadaSymbol> {
    return this.request<ScadaSymbol>('/scada/symbols', {
      method: 'POST',
      body: JSON.stringify(symbol),
    });
  }

  async deleteScadaSymbol(id: string): Promise<{ deleted: boolean }> {
    return this.request<{ deleted: boolean }>(`/scada/symbols/${id}`, { method: 'DELETE' });
  }

  // --- Stage 4: Rule Engine & DAG Studio ---
  async listRuleChains(page = 1, pageSize = 20): Promise<{ data: RuleChain[]; total: number; page: number; pageSize: number }> {
    return this.request<{ data: RuleChain[]; total: number; page: number; pageSize: number }>(`/ruleChains?page=${page}&pageSize=${pageSize}`);
  }

  async getRuleChain(id: string): Promise<RuleChain> {
    return this.request<RuleChain>(`/ruleChains/${id}`);
  }

  async createRuleChain(chain: Partial<RuleChain>): Promise<RuleChain> {
    return this.request<RuleChain>('/ruleChains', {
      method: 'POST',
      body: JSON.stringify(chain),
    });
  }

  async deleteRuleChain(id: string): Promise<void> {
    return this.request<void>(`/ruleChains/${id}`, { method: 'DELETE' });
  }

  async setRootRuleChain(id: string): Promise<{ success: boolean; rootChainId: string }> {
    return this.request<{ success: boolean; rootChainId: string }>(`/ruleChains/${id}/root`, { method: 'POST' });
  }

  async getRuleChainMetadata(id: string): Promise<RuleChainRevision> {
    return this.request<RuleChainRevision>(`/ruleChains/${id}/metadata`);
  }

  async saveRuleChainMetadata(id: string, definition: ChainDefinition, comment?: string): Promise<RuleChainRevision> {
    return this.request<RuleChainRevision>(`/ruleChains/${id}/metadata`, {
      method: 'POST',
      body: JSON.stringify({ definition, comment }),
    });
  }

  async listRuleDescriptors(): Promise<RuleDescriptor[]> {
    return this.request<RuleDescriptor[]>('/ruleChains/descriptors');
  }

  async testRuleChain(payload: { definition?: ChainDefinition; msgType?: string; data?: any; metadata?: Record<string, string>; originatorId?: string }): Promise<ExecutionTrace> {
    return this.request<ExecutionTrace>('/ruleChains/test', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  }

  async testRuleChainById(id: string, payload: { msgType?: string; data?: any; metadata?: Record<string, string>; originatorId?: string }): Promise<ExecutionTrace> {
    return this.request<ExecutionTrace>(`/ruleChains/${id}/test`, {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  }

  // --- Stage 5: Alarms & Notification Center ---
  async listAlarmRules(): Promise<AlarmRule[]> {
    return this.request<AlarmRule[]>('/alarmRules');
  }

  async saveAlarmRule(rule: Partial<AlarmRule>): Promise<AlarmRule> {
    return this.request<AlarmRule>('/alarmRules', {
      method: 'POST',
      body: JSON.stringify(rule),
    });
  }

  async getAlarmRule(id: string): Promise<AlarmRule> {
    return this.request<AlarmRule>(`/alarmRules/${id}`);
  }

  async deleteAlarmRule(id: string): Promise<void> {
    return this.request<void>(`/alarmRules/${id}`, { method: 'DELETE' });
  }

  async testAlarmRule(req: AlarmRuleTestRequest): Promise<AlarmRuleTestResponse> {
    return this.request<AlarmRuleTestResponse>('/alarmRules/test', {
      method: 'POST',
      body: JSON.stringify(req),
    });
  }

  async listNotificationTargets(): Promise<NotificationTarget[]> {
    return this.request<NotificationTarget[]>('/notification/targets');
  }

  async saveNotificationTarget(target: Partial<NotificationTarget>): Promise<NotificationTarget> {
    return this.request<NotificationTarget>('/notification/targets', {
      method: 'POST',
      body: JSON.stringify(target),
    });
  }

  async deleteNotificationTarget(id: string): Promise<void> {
    return this.request<void>(`/notification/targets/${id}`, { method: 'DELETE' });
  }

  async listNotificationTemplates(): Promise<NotificationTemplate[]> {
    return this.request<NotificationTemplate[]>('/notification/templates');
  }

  async saveNotificationTemplate(template: Partial<NotificationTemplate>): Promise<NotificationTemplate> {
    return this.request<NotificationTemplate>('/notification/templates', {
      method: 'POST',
      body: JSON.stringify(template),
    });
  }

  async deleteNotificationTemplate(id: string): Promise<void> {
    return this.request<void>(`/notification/templates/${id}`, { method: 'DELETE' });
  }

  async listNotificationRules(): Promise<NotificationRule[]> {
    return this.request<NotificationRule[]>('/notification/rules');
  }

  async saveNotificationRule(rule: Partial<NotificationRule>): Promise<NotificationRule> {
    return this.request<NotificationRule>('/notification/rules', {
      method: 'POST',
      body: JSON.stringify(rule),
    });
  }

  async deleteNotificationRule(id: string): Promise<void> {
    return this.request<void>(`/notification/rules/${id}`, { method: 'DELETE' });
  }

  async listInbox(limit = 20, offset = 0): Promise<{ data: NotificationItem[]; total: number; unread_count: number }> {
    return this.request<{ data: NotificationItem[]; total: number; unread_count: number }>(`/notification/inbox?limit=${limit}&offset=${offset}`);
  }

  async markInboxRead(id: string): Promise<void> {
    return this.request<void>(`/notification/inbox/${id}/read`, { method: 'POST' });
  }

  async markAllInboxRead(): Promise<void> {
    return this.request<void>('/notification/inbox/read-all', { method: 'POST' });
  }

  async getInboxUnreadCount(): Promise<{ unread_count: number }> {
    return this.request<{ unread_count: number }>('/notification/inbox/unread-count');
  }

  async sendNotification(req: NotificationSendRequest): Promise<any> {
    return this.request<any>('/notification/send', {
      method: 'POST',
      body: JSON.stringify(req),
    });
  }
}

export interface Dashboard {
  id: string;
  created_time: number;
  tenant_id: string;
  title: string;
  configuration: {
    deviceId?: string;
    widgets?: Array<{
      id: string;
      type: string;
      title: string;
      key?: string;
      unit?: string;
      method?: string;
    }>;
  };
}

export interface RpcCommand {
  id: string;
  tenant_id: string;
  device_id: string;
  method: string;
  params: any;
  two_way: boolean;
  status: 'QUEUED' | 'SENT' | 'DELIVERED' | 'SUCCESSFUL' | 'TIMEOUT' | 'EXPIRED' | 'FAILED';
  retries?: number;
  max_retries?: number;
  created_time: number;
  delivered_time?: number;
  expiration_time?: number;
  response_time?: number;
  response?: any;
  error?: string;
}

export interface BulkImportResult {
  imported: number;
  failed: number;
  errors: string[];
  devices: Device[];
}

export interface OtaPackage {
  id: string;
  tenant_id: string;
  type: 'FIRMWARE' | 'SOFTWARE';
  title: string;
  version: string;
  tag?: string;
  url?: string;
  file_name?: string;
  content_type?: string;
  checksum_algorithm?: string;
  checksum?: string;
  data_size?: number;
  created_time: number;
}

export interface OtaAssignment {
  id: string;
  tenant_id: string;
  kind: 'FIRMWARE' | 'SOFTWARE';
  package_id: string;
  scope: 'DEVICE' | 'DEVICE_PROFILE';
  target_id: string;
  created_time: number;
}

export interface DeviceOtaState {
  device_id: string;
  kind: 'FIRMWARE' | 'SOFTWARE';
  package_id?: string;
  state: 'QUEUED' | 'DOWNLOADING' | 'DOWNLOADED' | 'VERIFIED' | 'UPDATING' | 'UPDATED' | 'FAILED';
  progress: number;
  error?: string;
  updated_ts: number;
}

export interface Converter {
  id: string;
  tenant_id: string;
  name: string;
  direction: 'UPLINK' | 'DOWNLINK';
  mode: 'DECLARATIVE' | 'EXPRESSION';
  definition: any;
  version: number;
}

export interface Integration {
  id: string;
  tenant_id: string;
  name: string;
  type: string;
  enabled: boolean;
  uplink_converter_id: string;
  routing_key: string;
  allow_create_device: boolean;
  default_device_type: string;
}

export interface IntegrationStatus {
  integration_id: string;
  state: 'RUNNING' | 'STOPPED' | 'ERROR';
  last_error?: string;
  last_ok_ts: number;
  counters: {
    rx_events: number;
    tx_events: number;
    errors: number;
    auto_created: number;
  };
  instance: string;
}

export interface DebugEvent {
  id: string;
  ts: number;
  direction: string;
  payload: string;
  metadata: Record<string, string>;
  success: boolean;
  error?: string;
  result?: any;
}

export interface EdgeNode {
  id: string;
  tenant_id: string;
  name: string;
  edge_key: string;
  edge_secret?: string;
  status: 'ONLINE' | 'OFFLINE' | 'SYNCING';
  firmware_version: string;
  hardware_model: string;
  last_sync_ts: number;
  backlog_count: number;
  assigned_devices?: string[];
  downlink_seq?: number;
  uplink_seq?: number;
}

export interface SyncJournalEntry {
  id: string;
  edge_id: string;
  sequence: number;
  lane_l0_count: number;
  lane_l1_count: number;
  lane_l2_count: number;
  processed_items: number;
  status: string;
  synced_at: number;
}

export interface ScadaTag {
  id: string;
  tenant_id: string;
  path: string;
  data_type: 'ANALOG' | 'DISCRETE' | 'STRING';
  eng_unit?: string;
  deadband?: number;
  value: any;
  last_value?: any;
  quality: string;
  timestamp: number;
  scan_rate_ms?: number;
}

export interface ISA182Alarm {
  id: string;
  tenant_id: string;
  tag_path: string;
  alarm_type: string;
  setpoint: number;
  severity: string;
  state: string;
  active_ts: number;
  ack_ts?: number;
  ack_user?: string;
  shelve_until?: number;
  message: string;
}

export interface ControlCommand {
  select_id: string;
  tenant_id: string;
  tag_path: string;
  desired_value: any;
  operator: string;
  selected_at: number;
  operated_at?: number;
  status: 'SELECTED' | 'OPERATED' | 'CANCELLED' | 'INTERLOCK_DENIED';
  reason?: string;
}

export interface ControlLogEntry {
  id: string;
  tenant_id: string;
  tag_path: string;
  desired_value: any;
  operator: string;
  status: 'SELECTED' | 'OPERATED' | 'CANCELLED' | 'TIMED_OUT' | 'INTERLOCK_DENIED';
  reason?: string;
  selected_at: number;
  operated_at?: number;
}

export interface AlarmKPIs {
  total_alarms: number;
  unack_alarms: number;
  acked_alarms: number;
  shelved_alarms: number;
  rtn_unack_alarms: number;
  normal_alarms: number;
  mtta_seconds: number;
  alarm_rate_10min: number;
  in_flood: boolean;
}

export interface SymbolTag {
  name: string;
  role: string;
  elementId: string;
}

export interface SymbolProperty {
  id: string;
  type: string;
  default: any;
  label: string;
}

export interface SymbolBehavior {
  id: string;
  kind: 'value' | 'action' | 'widgetAction';
  valueType?: string;
  label: string;
  control?: any;
}

export interface SymbolAnimation {
  target: string;
  when: string;
  set?: Record<string, any>;
  rotate?: Record<string, any>;
}

export interface ScadaSymbol {
  id: string;
  tenant_id?: string;
  name: string;
  category: string;
  style: 'standard' | 'isa-101';
  svg_content: string;
  tags: SymbolTag[];
  properties: SymbolProperty[];
  behaviors: SymbolBehavior[];
  animations: SymbolAnimation[];
  version: number;
  created_at?: number;
}

export interface RuleChain {
  id: string;
  created_time: number;
  tenant_id: string;
  name: string;
  type: string;
  is_root: boolean;
  published_revision: number;
  additional_info?: any;
  version: number;
}

export interface RuleChainRevision {
  rule_chain_id: string;
  revision: number;
  status: string;
  definition: ChainDefinition;
  author_id?: string;
  comment?: string;
  created_time: number;
}

export interface ChainDefinition {
  firstNodeId: string;
  nodes: RuleNode[];
  connections: RuleConnection[];
}

export interface RuleNode {
  id: string;
  type: string;
  name: string;
  configuration: any;
  additionalInfo?: any;
}

export interface RuleConnection {
  from: string;
  to: string;
  label: string;
}

export interface RuleDescriptor {
  type: string;
  name: string;
  category: 'FILTER' | 'ENRICHMENT' | 'TRANSFORMATION' | 'ACTION' | 'EXTERNAL' | 'FLOW';
  relations: string[];
  configSchema: any;
  uiSchema?: any;
  version: number;
  docs?: string;
}

export interface NodeTraceRecord {
  nodeId: string;
  nodeType: string;
  inMessage: any;
  outMessage?: any;
  relations: string[];
  error?: string;
  durationMs: number;
  timestamp: number;
}

export interface ExecutionTrace {
  messageId: string;
  chainId: string;
  success: boolean;
  records: NodeTraceRecord[];
  totalTimeMs: number;
  error?: string;
}

// Stage 5 Alarms & Notification Center Types
export interface AlarmFilter {
  key: string;
  value_type: 'NUMERIC' | 'STRING' | 'BOOLEAN';
  operator: 'EQUAL' | 'NOT_EQUAL' | 'GREATER' | 'GREATER_OR_EQUAL' | 'LESS' | 'LESS_OR_EQUAL' | 'CONTAINS';
  value: any;
  dynamic_value?: { source: string; key: string };
}

export interface AlarmCondition {
  type: 'SIMPLE' | 'DURATION' | 'REPEATING';
  duration_ms?: number;
  count?: number;
  filters: AlarmFilter[];
}

export interface AlarmRuleCreateDef {
  condition: AlarmCondition;
  details?: string;
}

export interface AlarmRule {
  id: string;
  created_time: number;
  tenant_id: string;
  name: string;
  scope: 'DEVICE_PROFILE' | 'DEVICE' | 'ASSET' | 'CUSTOMER';
  target_id?: string;
  alarm_type: string;
  enabled: boolean;
  create_rules: Record<string, AlarmRuleCreateDef>;
  clear_rule?: AlarmRuleCreateDef;
  propagate?: any;
  version: number;
}

export interface AlarmRuleTestDataPoint {
  timestamp: number;
  values: Record<string, any>;
}

export interface AlarmRuleTestRequest {
  rule: AlarmRule;
  data_points: AlarmRuleTestDataPoint[];
}

export interface AlarmRuleTestTraceStep {
  timestamp: number;
  severity?: string;
  matched: boolean;
  action: 'NONE' | 'TRIGGER' | 'UPDATE' | 'CLEAR';
  details?: string;
  duration_true_ms: number;
  repeat_count: number;
}

export interface AlarmRuleTestResponse {
  rule_name: string;
  traces: AlarmRuleTestTraceStep[];
}

export interface NotificationTarget {
  id: string;
  created_time: number;
  tenant_id: string;
  name: string;
  type: 'PLATFORM_USERS' | 'EMAIL' | 'SLACK' | 'WEBHOOK' | 'SMS';
  config: any;
}

export interface NotificationTemplate {
  id: string;
  created_time: number;
  tenant_id: string;
  name: string;
  type: string;
  config: {
    subject?: string;
    body?: string;
    [key: string]: any;
  };
}

export interface EscalationStep {
  step: number;
  delay_seconds: number;
  target_ids: string[];
}

export interface NotificationRule {
  id: string;
  created_time: number;
  tenant_id: string;
  name: string;
  enabled: boolean;
  template_id: string;
  trigger_type: string;
  trigger_config: any;
  recipients_config: EscalationStep[];
  throttle?: any;
}

export interface NotificationItem {
  id: string;
  created_time: number;
  tenant_id: string;
  recipient_id: string;
  type: string;
  subject: string;
  body: string;
  info?: any;
  status: 'UNREAD' | 'READ';
}

export interface NotificationSendRequest {
  targets: string[];
  subject: string;
  body: string;
  info?: any;
}

export const api = new ApiClient();


