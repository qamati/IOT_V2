// realtime.ts — Service 16 Realtime WebSocket Client SDK

export type RealtimeFrameType = 'AUTH' | 'SUBSCRIBE' | 'UNSUBSCRIBE' | 'PING' | 'PONG' | 'CONNECTED' | 'UPDATE' | 'ERROR';
export type RealtimeKind = 'TELEMETRY' | 'ALARMS' | 'NOTIFICATIONS' | 'SCADA';

export interface RealtimeServerFrame {
  type: RealtimeFrameType;
  sessionId?: string;
  cmdId?: number;
  kind?: RealtimeKind;
  entityId?: string;
  data?: any;
  timestamp?: number;
  error?: string;
}

type TelemetryListener = (entityId: string, data: Record<string, any>, timestamp: number) => void;
type AlarmListener = (alarm: any) => void;
type NotificationListener = (notification: any) => void;

class RealtimeClient {
  private ws: WebSocket | null = null;
  private token: string | null = null;
  private reconnectTimer: any = null;
  private activeSubs: Map<number, { kind: RealtimeKind; entityId?: string; keys?: string[] }> = new Map();
  private telemetryListeners: Set<TelemetryListener> = new Set();
  private alarmListeners: Set<AlarmListener> = new Set();
  private notificationListeners: Set<NotificationListener> = new Set();

  connect(token: string | null) {
    this.token = token;
    if (!token) {
      this.disconnect();
      return;
    }

    if (this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)) {
      return;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/api/v1/ws?token=${encodeURIComponent(token)}`;

    try {
      this.ws = new WebSocket(wsUrl);

      this.ws.onopen = () => {
        console.log('[Realtime] WebSocket connected');
        // Resubscribe active subscriptions on reconnect
        this.activeSubs.forEach((sub, cmdId) => {
          this.send({
            type: 'SUBSCRIBE',
            cmdId,
            kind: sub.kind,
            entityId: sub.entityId,
            keys: sub.keys,
          });
        });
      };

      this.ws.onmessage = (event) => {
        try {
          const frame: RealtimeServerFrame = JSON.parse(event.data);
          this.handleFrame(frame);
        } catch (err) {
          console.error('[Realtime] Failed to parse incoming frame:', err);
        }
      };

      this.ws.onclose = () => {
        this.ws = null;
        this.scheduleReconnect();
      };

      this.ws.onerror = (err) => {
        console.warn('[Realtime] WebSocket error:', err);
        this.ws?.close();
      };
    } catch (err) {
      this.scheduleReconnect();
    }
  }

  disconnect() {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.activeSubs.clear();
  }

  private scheduleReconnect() {
    if (!this.token || this.reconnectTimer) return;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      if (this.token) {
        this.connect(this.token);
      }
    }, 3000);
  }

  private send(data: any) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data));
    }
  }

  private handleFrame(frame: RealtimeServerFrame) {
    if (frame.type === 'UPDATE') {
      if (frame.kind === 'TELEMETRY' && frame.entityId && frame.data) {
        this.telemetryListeners.forEach((fn) =>
          fn(frame.entityId!, frame.data, frame.timestamp || Date.now())
        );
      } else if (frame.kind === 'ALARMS' && frame.data) {
        this.alarmListeners.forEach((fn) => fn(frame.data));
      } else if (frame.kind === 'NOTIFICATIONS' && frame.data) {
        this.notificationListeners.forEach((fn) => fn(frame.data));
      }
    }
  }

  // Subscription management
  subscribeTelemetry(cmdId: number, entityId: string, keys?: string[]) {
    this.activeSubs.set(cmdId, { kind: 'TELEMETRY', entityId, keys });
    this.send({
      type: 'SUBSCRIBE',
      cmdId,
      kind: 'TELEMETRY',
      entityId,
      keys,
    });
  }

  subscribeAlarms(cmdId: number) {
    this.activeSubs.set(cmdId, { kind: 'ALARMS' });
    this.send({
      type: 'SUBSCRIBE',
      cmdId,
      kind: 'ALARMS',
    });
  }

  subscribeNotifications(cmdId: number) {
    this.activeSubs.set(cmdId, { kind: 'NOTIFICATIONS' });
    this.send({
      type: 'SUBSCRIBE',
      cmdId,
      kind: 'NOTIFICATIONS',
    });
  }

  unsubscribe(cmdId: number) {
    this.activeSubs.delete(cmdId);
    this.send({
      type: 'UNSUBSCRIBE',
      cmdId,
    });
  }

  onTelemetry(fn: TelemetryListener) {
    this.telemetryListeners.add(fn);
    return () => this.telemetryListeners.delete(fn);
  }

  onAlarm(fn: AlarmListener) {
    this.alarmListeners.add(fn);
    return () => this.alarmListeners.delete(fn);
  }

  onNotification(fn: NotificationListener) {
    this.notificationListeners.add(fn);
    return () => this.notificationListeners.delete(fn);
  }
}

export const realtime = new RealtimeClient();
