import React, { useState, useEffect, useRef, useMemo } from 'react';
import { 
  Cpu, 
  Activity, 
  Layers, 
  Plus, 
  Trash2, 
  RefreshCw, 
  Copy, 
  Check, 
  Radio, 
  Sliders, 
  LogOut, 
  Server, 
  Terminal, 
  ShieldCheck, 
  ChevronRight, 
  Send, 
  Play, 
  Square, 
  TrendingUp, 
  Clock, 
  Sparkles,
  AlertTriangle,
  Bell,
  Tag,
  LayoutDashboard,
  Power,
  Share2,
  Gauge,
  Network,
  GitMerge,
  ExternalLink,
  Upload,
  Download
} from 'lucide-react';
import { api } from './api';
import { realtime } from './realtime';
import type { Device, User, TelemetryMap, AttributeKv, Alarm, DeviceActivity, Dashboard, RpcCommand, NotificationItem } from './api';
import { IntegrationsView } from './components/IntegrationsView';
import { ScadaView } from './components/ScadaView';
import { EdgeView } from './components/EdgeView';
import { RuleEngineView } from './components/RuleEngineView';
import { AlarmsView } from './components/AlarmsView';
import { OtaView } from './components/OtaView';

export default function App() {
  const [token, setToken] = useState<string | null>(api.getToken());
  const [user, setUser] = useState<User | null>(() => {
    const saved = localStorage.getItem('iotp_user');
    return saved ? JSON.parse(saved) : null;
  });

  // Navigation: overview, devices, device-detail, dashboards, alarms, system, integrations, scada, edge, rules, ota
  const [activeTab, setActiveTab] = useState<'dashboard' | 'devices' | 'device-detail' | 'dashboards' | 'alarms' | 'system' | 'integrations' | 'scada' | 'edge' | 'rules' | 'ota'>('devices');
  const [selectedDeviceId, setSelectedDeviceId] = useState<string | null>(null);
  const [deviceDetailSubTab, setDeviceDetailSubTab] = useState<'telemetry' | 'attributes' | 'rpc'>('telemetry');

  // Login form
  const [email, setEmail] = useState('tenant@example.com');
  const [password, setPassword] = useState('Secret123!');
  const [loginLoading, setLoginLoading] = useState(false);
  const [loginError, setLoginError] = useState('');

  // Devices state
  const [devices, setDevices] = useState<Device[]>([]);
  const [devicesLoading, setDevicesLoading] = useState(false);
  const [deviceTokens, setDeviceTokens] = useState<Record<string, string>>({});
  const [deviceActivities, setDeviceActivities] = useState<Record<string, DeviceActivity>>({});
  const [copiedToken, setCopiedToken] = useState<string | null>(null);

  // New device modal
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newDeviceName, setNewDeviceName] = useState('');
  const [newDeviceLabel, setNewDeviceLabel] = useState('');
  const [newDeviceToken, setNewDeviceToken] = useState('');
  const [createLoading, setCreateLoading] = useState(false);
  const [createDeviceError, setCreateDeviceError] = useState('');

  // Device Detail & Telemetry State
  const [selectedDevice, setSelectedDevice] = useState<Device | null>(null);
  const [latestTelemetry, setLatestTelemetry] = useState<TelemetryMap>({});
  const [timeseriesData, setTimeseriesData] = useState<TelemetryMap>({});
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [selectedChartKey, setSelectedChartKey] = useState<string>('Boiler.Temperature');
  const [dashSelectedDeviceId, setDashSelectedDeviceId] = useState<string>('');
  const [deviceProfileType, setDeviceProfileType] = useState<'STANDARD' | 'OPC_UA'>('STANDARD');
  const [opcuaEndpointInput, setOpcuaEndpointInput] = useState('opc.tcp://172.17.0.3:4840');

  // Attributes State
  const [attributes, setAttributes] = useState<AttributeKv[]>([]);
  const [newAttrScope, setNewAttrScope] = useState<'SERVER_SCOPE' | 'SHARED_SCOPE'>('SERVER_SCOPE');
  const [newAttrKey, setNewAttrKey] = useState('');
  const [newAttrVal, setNewAttrVal] = useState('');

  // Alarms State
  const [alarms, setAlarms] = useState<Alarm[]>([]);
  const [_alarmsLoading, setAlarmsLoading] = useState(false);

  // Notification Inbox State
  const [showInboxDropdown, setShowInboxDropdown] = useState(false);
  const [inboxNotifications, setInboxNotifications] = useState<NotificationItem[]>([]);
  const [unreadNotifCount, setUnreadNotifCount] = useState(0);

  // Dashboards State
  const [dashboards, setDashboards] = useState<Dashboard[]>([]);
  const [selectedDashboard, setSelectedDashboard] = useState<Dashboard | null>(null);
  const [showCreateDashModal, setShowCreateDashModal] = useState(false);
  const [newDashTitle, setNewDashTitle] = useState('');

  // RPC State
  const [rpcCommands, setRpcCommands] = useState<RpcCommand[]>([]);
  const [rpcMethod, setRpcMethod] = useState('setRelay');
  const [rpcParams, setRpcParams] = useState('{"relay": 1, "state": true}');
  const [rpcSending, setRpcSending] = useState(false);
  const [relayActive, setRelayActive] = useState(false);
  const [rpcTwoWay, setRpcTwoWay] = useState(true);
  const [rpcTimeoutMs, setRpcTimeoutMs] = useState(3000);
  const [rpcLastResponse, setRpcLastResponse] = useState<any>(null);
  const [rpcLastDurationMs, setRpcLastDurationMs] = useState<number | null>(null);

  // Bulk Device Import State
  const [showBulkImportModal, setShowBulkImportModal] = useState(false);
  const [bulkImportFormat, setBulkImportFormat] = useState<'csv' | 'json'>('csv');
  const [bulkImportText, setBulkImportText] = useState('');
  const [bulkImportLoading, setBulkImportLoading] = useState(false);
  const [bulkImportResult, setBulkImportResult] = useState<any>(null);

  // Simulator State
  const [simTemp, setSimTemp] = useState(24.5);
  const [simHumidity, setSimHumidity] = useState(55);
  const [simVoltage, setSimVoltage] = useState(3.7);
  const [simBattery, setSimBattery] = useState(88);
  const [isSimulating, setIsSimulating] = useState(false);
  const [simLog, setSimLog] = useState<string[]>([]);
  const simIntervalRef = useRef<any>(null);

  // Check login
  const handleLogin = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    setLoginLoading(true);
    setLoginError('');
    try {
      const res = await api.login(email, password);
      setToken(res.token);
      setUser(res.user);
      localStorage.setItem('iotp_user', JSON.stringify(res.user));
    } catch (err: any) {
      setLoginError(err.message || 'Login failed');
    } finally {
      setLoginLoading(false);
    }
  };

  const handleLogout = () => {
    api.logout();
    localStorage.removeItem('iotp_user');
    setToken(null);
    setUser(null);
    setSelectedDeviceId(null);
    setSelectedDevice(null);
  };

  // Fetch devices, alarms, dashboards
  const loadDevices = async () => {
    if (!token) return;
    setDevicesLoading(true);
    try {
      const res = await api.listDevices(1, 100);
      setDevices(res.data || []);
      for (const dev of res.data || []) {
        api.getCredentials(dev.id).then(creds => {
          setDeviceTokens(prev => ({ ...prev, [dev.id]: creds.credentials_value || creds.credentials_id || 'Active' }));
        }).catch(() => {});
        api.getDeviceActivity(dev.id).then(act => {
          setDeviceActivities(prev => ({ ...prev, [dev.id]: act }));
        }).catch(() => {});
      }
    } catch (err) {
      console.error('Failed to load devices:', err);
    } finally {
      setDevicesLoading(false);
    }
  };

  const loadAlarms = async () => {
    if (!token) return;
    setAlarmsLoading(true);
    try {
      const res = await api.listAlarms(1, 50);
      setAlarms(res.data || []);
    } catch (err) {
      console.error('Failed to load alarms:', err);
    } finally {
      setAlarmsLoading(false);
    }
  };

  const loadDashboards = async () => {
    if (!token) return;
    try {
      const res = await api.listDashboards(1, 50);
      setDashboards(res.data || []);
      if (!selectedDashboard && res.data && res.data.length > 0) {
        setSelectedDashboard(res.data[0]);
      }
    } catch (err) {
      console.error('Failed to load dashboards:', err);
    }
  };

  const loadInbox = async () => {
    if (!token) return;
    try {
      const res = await api.listInbox(10, 0);
      setInboxNotifications(res.data || []);
      setUnreadNotifCount(res.unread_count || 0);
    } catch (err) {
      console.error('Failed to load inbox:', err);
    }
  };

  useEffect(() => {
    if (token) {
      loadDevices();
      loadAlarms();
      loadInbox();
      loadDashboards();

      // Connect to Stage 6 Realtime WebSocket Gateway
      realtime.connect(token);
      realtime.subscribeAlarms(10);
      realtime.subscribeNotifications(11);

      const unsubAlarm = realtime.onAlarm((newAlarm) => {
        setAlarms((prev) => [newAlarm, ...prev.filter((a) => a.id !== newAlarm.id)]);
      });

      const unsubNotif = realtime.onNotification((newNotif) => {
        setInboxNotifications((prev) => [newNotif, ...prev]);
        setUnreadNotifCount((c) => c + 1);
      });

      return () => {
        unsubAlarm();
        unsubNotif();
        realtime.disconnect();
      };
    }
  }, [token]);

  // Load selected device details, telemetry, attributes, and RPC
  const loadDeviceData = async (devId: string) => {
    try {
      const dev = await api.getDevice(devId);
      setSelectedDevice(dev);

      // Latest telemetry
      const latest = await api.getLatestTelemetry(devId);
      setLatestTelemetry(latest || {});

      // Timeseries (last 2 hours)
      const now = Date.now();
      const start = now - 2 * 60 * 60 * 1000;
      const ts = await api.getTimeseries(devId, undefined, start, now, 100);
      setTimeseriesData(ts || {});

      // Attributes
      const attrs = await api.getAllAttributes(devId);
      setAttributes(attrs || []);

      // Activity
      const act = await api.getDeviceActivity(devId);
      setDeviceActivities(prev => ({ ...prev, [devId]: act }));

      // RPC History
      const rpcs = await api.getRpcHistory(devId);
      setRpcCommands(rpcs.data || []);
    } catch (err) {
      console.error('Error loading device telemetry:', err);
    }
  };

  // Realtime telemetry streaming when on detail page or dashboard
  useEffect(() => {
    let targetDevId: string | null = null;
    if (activeTab === 'device-detail' && selectedDeviceId) {
      targetDevId = selectedDeviceId;
    } else if (activeTab === 'dashboards' && devices.length > 0) {
      targetDevId = dashSelectedDeviceId || selectedDashboard?.configuration?.deviceId || devices[0]?.id;
    }

    if (targetDevId) {
      loadDeviceData(targetDevId);

      // Subscribe via WebSocket
      realtime.subscribeTelemetry(20, targetDevId);
      const unsubTelemetry = realtime.onTelemetry((devId, data, ts) => {
        if (devId === targetDevId) {
          setLatestTelemetry((prev) => {
            const updated = { ...prev };
            for (const [k, v] of Object.entries(data)) {
              updated[k] = [{ ts, value: v }];
            }
            return updated;
          });
          setTimeseriesData((prev) => {
            const updated = { ...prev };
            for (const [k, v] of Object.entries(data)) {
              const existing = updated[k] || [];
              updated[k] = [...existing.slice(-99), { ts, value: v }];
            }
            return updated;
          });
        }
      });

      return () => {
        realtime.unsubscribe(20);
        unsubTelemetry();
      };
    }
  }, [activeTab, selectedDeviceId, selectedDashboard, devices]);

  // Simulator streamer
  useEffect(() => {
    if (isSimulating && selectedDeviceId) {
      const devToken = deviceTokens[selectedDeviceId];
      if (!devToken) return;

      simIntervalRef.current = setInterval(async () => {
        const newTemp = +(simTemp + (Math.random() * 0.8 - 0.4)).toFixed(1);
        const newHum = Math.min(100, Math.max(10, +(simHumidity + (Math.random() * 1.2 - 0.6)).toFixed(1)));
        const newVolt = +(simVoltage + (Math.random() * 0.04 - 0.02)).toFixed(2);
        const newBat = Math.max(0, +(simBattery - 0.05).toFixed(1));

        setSimTemp(newTemp);
        setSimHumidity(newHum);
        setSimVoltage(newVolt);
        setSimBattery(newBat);

        const payload = {
          temperature: newTemp,
          humidity: newHum,
          voltage: newVolt,
          battery: newBat,
          status: 'ONLINE',
        };

        try {
          await api.uploadTelemetry(devToken, payload);
          setSimLog(prev => [
            `[${new Date().toLocaleTimeString()}] Sent ${JSON.stringify(payload)}`,
            ...prev.slice(0, 15)
          ]);
          loadDeviceData(selectedDeviceId);
          loadAlarms();
        } catch (e: any) {
          setSimLog(prev => [`[${new Date().toLocaleTimeString()}] Error: ${e.message}`, ...prev.slice(0, 15)]);
        }
      }, 2000);

      return () => {
        if (simIntervalRef.current) clearInterval(simIntervalRef.current);
      };
    } else {
      if (simIntervalRef.current) clearInterval(simIntervalRef.current);
    }
  }, [isSimulating, selectedDeviceId, deviceTokens, simTemp, simHumidity, simVoltage, simBattery]);

  const sendSingleSimulation = async () => {
    if (!selectedDeviceId) return;
    const devToken = deviceTokens[selectedDeviceId];
    if (!devToken) return;

    const payload = {
      temperature: simTemp,
      humidity: simHumidity,
      voltage: simVoltage,
      battery: simBattery,
      status: 'ONLINE',
    };

    try {
      await api.uploadTelemetry(devToken, payload);
      setSimLog(prev => [
        `[${new Date().toLocaleTimeString()}] Manual: ${JSON.stringify(payload)}`,
        ...prev.slice(0, 15)
      ]);
      loadDeviceData(selectedDeviceId);
      loadAlarms();
    } catch (e: any) {
      alert('Upload failed: ' + e.message);
    }
  };

  const handleSendRpc = async (isTwoWay = true) => {
    const devId = selectedDeviceId || devices[0]?.id;
    if (!devId) return;

    setRpcSending(true);
    setRpcLastResponse(null);
    setRpcLastDurationMs(null);
    const startTime = Date.now();

    try {
      let parsedParams: any = {};
      try {
        parsedParams = JSON.parse(rpcParams);
      } catch {
        parsedParams = { val: rpcParams };
      }

      if (isTwoWay) {
        // Send two-way RPC (device simulation will answer)
        const cmdPromise = api.sendTwoWayRpc(devId, rpcMethod, parsedParams, rpcTimeoutMs);
        // Simulate device picking up the RPC and responding after 350ms
        const devToken = deviceTokens[devId];
        if (devToken) {
          setTimeout(async () => {
            const pending = await api.pollDeviceRpc(devToken);
            if (pending) {
              await api.respondDeviceRpc(devToken, pending.id, {
                success: true,
                executed_at: Date.now(),
                method: rpcMethod,
                state: parsedParams.state ?? true,
                device: devId,
              });
            }
          }, 350);
        }
        const cmd = await cmdPromise;
        setRpcLastResponse(cmd);
        setRpcLastDurationMs(Date.now() - startTime);
      } else {
        const cmd = await api.sendOneWayRpc(devId, rpcMethod, parsedParams);
        setRpcLastResponse(cmd);
        setRpcLastDurationMs(Date.now() - startTime);
      }

      loadDeviceData(devId);
    } catch (err: any) {
      alert('RPC execution error: ' + err.message);
    } finally {
      setRpcSending(false);
    }
  };

  const handleDeleteRpc = async (commandId: string) => {
    try {
      await api.deletePersistentRpc(commandId);
      if (selectedDeviceId) {
        const rpcs = await api.getRpcHistory(selectedDeviceId);
        setRpcCommands(rpcs.data || []);
      }
    } catch (err: any) {
      alert('Failed to delete command: ' + err.message);
    }
  };

  const handleExportDevices = async () => {
    try {
      const csv = await api.bulkExportDevicesCsv();
      const blob = new Blob([csv], { type: 'text/csv' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `devices_export_${Date.now()}.csv`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err: any) {
      alert(`Export failed: ${err.message}`);
    }
  };

  const handleBulkImportSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!bulkImportText.trim()) return;
    setBulkImportLoading(true);
    setBulkImportResult(null);
    try {
      let payload: any = bulkImportText;
      if (bulkImportFormat === 'json') {
        try {
          payload = JSON.parse(bulkImportText);
        } catch (jsonErr: any) {
          alert('Invalid JSON format: ' + jsonErr.message);
          setBulkImportLoading(false);
          return;
        }
      }
      const res = await api.bulkImportDevices(payload);
      setBulkImportResult(res);
      await loadDevices();
    } catch (err: any) {
      alert(`Bulk import failed: ${err.message}`);
    } finally {
      setBulkImportLoading(false);
    }
  };

  const handleToggleRelayFromDashboard = async () => {
    const devId = selectedDashboard?.configuration?.deviceId || devices[0]?.id;
    if (!devId) return;

    const nextState = !relayActive;
    setRelayActive(nextState);
    const devToken = deviceTokens[devId];

    try {
      const cmdPromise = api.sendTwoWayRpc(devId, 'setRelay', { relay: 1, state: nextState }, 3000);
      if (devToken) {
        setTimeout(async () => {
          const pending = await api.pollDeviceRpc(devToken);
          if (pending) {
            await api.respondDeviceRpc(devToken, pending.id, {
              success: true,
              relay: 1,
              state: nextState,
            });
          }
        }, 300);
      }
      await cmdPromise;
      loadDeviceData(devId);
    } catch (e: any) {
      console.error('Relay toggle failed:', e);
    }
  };

  const handleCreateDashboard = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newDashTitle.trim()) return;
    try {
      const defaultDevId = devices[0]?.id || '';
      const cfg = {
        deviceId: defaultDevId,
        widgets: [
          { id: 'w1', type: 'metric_card', title: 'Ambient Temperature', key: 'temperature', unit: '°C' },
          { id: 'w2', type: 'metric_card', title: 'Relative Humidity', key: 'humidity', unit: '%' },
          { id: 'w3', type: 'timeseries_chart', title: 'Live Sensor Trend', key: 'temperature' },
          { id: 'w4', type: 'rpc_control', title: 'Relay Switch #1', method: 'setRelay' },
        ],
      };
      const created = await api.createDashboard(newDashTitle.trim(), cfg);
      setShowCreateDashModal(false);
      setNewDashTitle('');
      await loadDashboards();
      setSelectedDashboard(created);
    } catch (err: any) {
      alert('Failed to create dashboard: ' + err.message);
    }
  };

  const handleSaveAttribute = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedDeviceId || !newAttrKey.trim()) return;
    try {
      let parsedVal: any = newAttrVal;
      try {
        parsedVal = JSON.parse(newAttrVal);
      } catch {
        // string
      }
      await api.saveScopedAttributes(selectedDeviceId, newAttrScope, { [newAttrKey.trim()]: parsedVal });
      setNewAttrKey('');
      setNewAttrVal('');
      const attrs = await api.getAllAttributes(selectedDeviceId);
      setAttributes(attrs || []);
    } catch (err: any) {
      alert('Failed to save attribute: ' + err.message);
    }
  };

  const handleDeleteAttribute = async (scope: any, key: string) => {
    if (!selectedDeviceId) return;
    try {
      await api.deleteScopedAttributes(selectedDeviceId, scope, [key]);
      const attrs = await api.getAllAttributes(selectedDeviceId);
      setAttributes(attrs || []);
    } catch (err: any) {
      alert('Failed to delete attribute: ' + err.message);
    }
  };

  const handleAckAlarm = async (id: string) => {
    try {
      await api.ackAlarm(id);
      loadAlarms();
    } catch (err: any) {
      alert('Failed to acknowledge alarm: ' + err.message);
    }
  };

  const handleClearAlarm = async (id: string) => {
    try {
      await api.clearAlarm(id);
      loadAlarms();
    } catch (err: any) {
      alert('Failed to clear alarm: ' + err.message);
    }
  };

  const handleCreateDevice = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newDeviceName.trim()) return;
    setCreateLoading(true);
    setCreateDeviceError('');
    try {
      const generatedToken = newDeviceToken.trim() || `DEV_${Math.random().toString(36).substring(2, 10).toUpperCase()}`;
      const res = await api.createDevice(newDeviceName.trim(), newDeviceLabel.trim(), generatedToken);
      const tokenVal = res.credentials?.credentials_value || res.credentials?.credentials_id || generatedToken;
      setDeviceTokens(prev => ({ ...prev, [res.device.id]: tokenVal }));

      if (deviceProfileType === 'OPC_UA') {
        const ep = opcuaEndpointInput.trim() || 'opc.tcp://172.17.0.3:4840';
        await api.saveScopedAttributes(res.device.id, 'SERVER_SCOPE', {
          endpoint_url: ep,
          host_endpoint: 'opc.tcp://127.0.0.1:4840',
          api_endpoint: 'http://localhost:3100',
          protocol: 'OPC_UA',
          status: 'CONNECTED',
          security_mode: 'None',
          nodes_count: 5
        });

        try {
          await api.uploadTelemetry(tokenVal, {
            'Boiler.Temperature': 82.5,
            'Boiler.Pressure': 18.4,
            'FeedPump.Status': 1,
            'FeedPump.SpeedRPM': 1750,
            'Effluent.FlowRate': 345.2,
            'opcua_server_status': 'CONNECTED'
          });
        } catch {
          // background poller handles ingestion
        }
      }

      setShowCreateModal(false);
      setNewDeviceName('');
      setNewDeviceLabel('');
      setNewDeviceToken('');
      setCreateDeviceError('');
      await loadDevices();
      setSelectedDeviceId(res.device.id);
      setSelectedDevice(res.device);
      setActiveTab('device-detail');
    } catch (err: any) {
      setCreateDeviceError(err.message || 'Failed to create device');
    } finally {
      setCreateLoading(false);
    }
  };

  const handleDeleteDevice = async (id: string) => {
    if (!confirm('Are you sure you want to delete this device? Telemetry history will be removed.')) return;
    try {
      await api.deleteDevice(id);
      if (selectedDeviceId === id) {
        setSelectedDeviceId(null);
        setSelectedDevice(null);
        setActiveTab('devices');
      }
      loadDevices();
    } catch (err: any) {
      alert('Delete failed: ' + err.message);
    }
  };

  const copyToClipboard = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedToken(id);
    setTimeout(() => setCopiedToken(null), 2000);
  };

  // Render SVG Chart for currently selected metric key
  const chartPoints = useMemo(() => {
    let effectiveKey = selectedChartKey;
    const availableKeys = Object.keys(timeseriesData).filter(k => {
      const pts = timeseriesData[k];
      return pts && pts.length > 0 && typeof pts[0].value === 'number';
    });

    if (!timeseriesData[effectiveKey] || timeseriesData[effectiveKey].length === 0) {
      if (availableKeys.length > 0) {
        effectiveKey = availableKeys[0];
      }
    }

    const list = timeseriesData[effectiveKey] || [];
    if (!list || list.length === 0) return null;
    const sorted = [...list].sort((a, b) => a.ts - b.ts);
    const minVal = Math.min(...sorted.map(s => Number(s.value))) - 0.2;
    const maxVal = Math.max(...sorted.map(s => Number(s.value))) + 0.2;
    const range = maxVal - minVal || 1;

    const width = 800;
    const height = 180;
    const padding = 20;

    const pts = sorted.map((pt, i) => {
      const x = padding + (i / Math.max(1, sorted.length - 1)) * (width - 2 * padding);
      const val = Number(pt.value);
      const y = height - padding - ((val - minVal) / range) * (height - 2 * padding);
      return { x, y, val, ts: pt.ts };
    });

    const pathD = pts.reduce((acc, p, i) => {
      return i === 0 ? `M ${p.x} ${p.y}` : `${acc} L ${p.x} ${p.y}`;
    }, '');

    const areaD = pts.length > 0 
      ? `${pathD} L ${pts[pts.length - 1].x} ${height - padding} L ${pts[0].x} ${height - padding} Z`
      : '';

    return { pts, pathD, areaD, minVal, maxVal, width, height, key: effectiveKey };
  }, [timeseriesData, selectedChartKey]);

  const activeAlarmCount = alarms.filter(a => a.status === 'ACTIVE_UNACK' || a.status === 'ACTIVE_ACK').length;

  // If not logged in, render login screen
  if (!token) {
    return (
      <div className="login-screen">
        <div className="login-card">
          <div className="login-brand">
            <div className="logo-badge">
              <Cpu size={28} />
            </div>
            <div>
              <h1 style={{ fontSize: '1.5rem', fontWeight: 800, letterSpacing: '-0.02em', color: '#fff' }}>IOTP CONSOLE</h1>
              <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', marginTop: '4px' }}>
                Next-Gen ThingsBoard-Class IoT Platform
              </p>
            </div>
          </div>

          {loginError && (
            <div style={{
              background: 'rgba(244, 63, 94, 0.15)',
              border: '1px solid rgba(244, 63, 94, 0.3)',
              color: 'var(--accent-rose)',
              padding: '12px 16px',
              borderRadius: 'var(--radius-md)',
              fontSize: '0.85rem'
            }}>
              {loginError}
            </div>
          )}

          <form onSubmit={handleLogin} style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <div className="form-group" style={{ margin: 0 }}>
              <label className="form-label">Email Address</label>
              <input 
                type="email" 
                className="form-input" 
                value={email} 
                onChange={e => setEmail(e.target.value)} 
                required 
              />
            </div>
            <div className="form-group" style={{ margin: 0 }}>
              <label className="form-label">Password</label>
              <input 
                type="password" 
                className="form-input" 
                value={password} 
                onChange={e => setPassword(e.target.value)} 
                required 
              />
            </div>

            <button type="submit" className="btn btn-primary" style={{ padding: '12px', marginTop: '8px' }} disabled={loginLoading}>
              {loginLoading ? <RefreshCw className="animate-spin" size={18} /> : <ShieldCheck size={18} />}
              {loginLoading ? 'Authenticating...' : 'Sign In to Console'}
            </button>

            <button 
              type="button" 
              className="btn btn-secondary" 
              style={{ fontSize: '0.8rem' }}
              onClick={() => {
                setEmail('tenant@example.com');
                setPassword('Secret123!');
              }}
            >
              <Sparkles size={14} style={{ color: 'var(--accent-cyan)' }} />
              Fill Demo Credentials (Acme Tenant)
            </button>
          </form>

          <div style={{ textAlign: 'center', fontSize: '0.75rem', color: 'var(--text-muted)' }}>
            High-Performance Go 1.24+ Monorepo · UUIDv7 · Mochi-MQTT v2 · Multi-Tenant
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="app-container">
      {/* Sidebar Navigation */}
      <aside className="sidebar">
        <div className="sidebar-header">
          <div className="logo-badge">
            <Cpu size={20} />
          </div>
          <div className="logo-text">
            <h1>IOTP CORE</h1>
            <span>Enterprise v2</span>
          </div>
        </div>

        <ul className="nav-links">
          <li 
            className={`nav-item ${activeTab === 'dashboard' ? 'active' : ''}`}
            onClick={() => setActiveTab('dashboard')}
          >
            <Activity size={18} />
            <span>Overview</span>
          </li>
          <li 
            className={`nav-item ${activeTab === 'dashboards' ? 'active' : ''}`}
            onClick={() => setActiveTab('dashboards')}
          >
            <LayoutDashboard size={18} />
            <span>Live Dashboards</span>
          </li>
          <li 
            className={`nav-item ${activeTab === 'devices' || activeTab === 'device-detail' ? 'active' : ''}`}
            onClick={() => setActiveTab('devices')}
          >
            <Layers size={18} />
            <span>Devices Fleet</span>
          </li>
          <li 
            className={`nav-item ${activeTab === 'alarms' ? 'active' : ''}`}
            onClick={() => setActiveTab('alarms')}
            style={{ position: 'relative' }}
          >
            <Bell size={18} />
            <span>Alarms</span>
            {activeAlarmCount > 0 && (
              <span style={{
                marginLeft: 'auto',
                background: 'var(--accent-rose)',
                color: '#fff',
                fontSize: '0.7rem',
                fontWeight: 700,
                padding: '2px 6px',
                borderRadius: '9999px'
              }}>
                {activeAlarmCount}
              </span>
            )}
          </li>
          <li 
            className={`nav-item ${activeTab === 'integrations' ? 'active' : ''}`}
            onClick={() => setActiveTab('integrations')}
          >
            <Share2 size={18} />
            <span>Integrations</span>
          </li>
          <li 
            className={`nav-item ${activeTab === 'scada' ? 'active' : ''}`}
            onClick={() => setActiveTab('scada')}
          >
            <Gauge size={18} />
            <span>SCADA & HMI</span>
          </li>
          <li 
            className={`nav-item ${activeTab === 'edge' ? 'active' : ''}`}
            onClick={() => setActiveTab('edge')}
          >
            <Network size={18} />
            <span>Edge Fleet</span>
          </li>
          <li 
            className={`nav-item ${activeTab === 'rules' ? 'active' : ''}`}
            onClick={() => setActiveTab('rules')}
          >
            <GitMerge size={18} />
            <span>Rule Engine</span>
          </li>
          <li 
            className={`nav-item ${activeTab === 'ota' ? 'active' : ''}`}
            onClick={() => setActiveTab('ota')}
          >
            <Cpu size={18} />
            <span>Firmware & OTA</span>
          </li>
          <li 
            className={`nav-item ${activeTab === 'system' ? 'active' : ''}`}
            onClick={() => setActiveTab('system')}
          >
            <Server size={18} />
            <span>System Health</span>
          </li>
        </ul>

        <div className="sidebar-footer">
          <div className="user-profile">
            <div className="avatar">
              {user?.email?.charAt(0).toUpperCase() || 'A'}
            </div>
            <div className="user-meta">
              <div className="name">{user?.email || 'Acme Tenant'}</div>
              <div className="role">TENANT_ADMIN</div>
            </div>
            <button 
              onClick={handleLogout} 
              className="btn btn-secondary btn-sm"
              title="Sign Out"
              style={{ padding: '6px' }}
            >
              <LogOut size={15} />
            </button>
          </div>
        </div>
      </aside>

      {/* Main Content Area */}
      <main className="main-content">
        {/* Top Header Bar */}
        <header className="topbar">
          <div className="topbar-left">
            <div className="tenant-pill">
              <span className="indicator"></span>
              <span>Acme Corp Primary Tenant</span>
            </div>
            <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
              ID: {user?.tenant_id ? `${user.tenant_id.substring(0, 8)}...` : '0192d4bf'}
            </span>
          </div>

          <div className="topbar-right" style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
            {activeAlarmCount > 0 && (
              <div 
                className="badge-tag" 
                style={{ background: 'rgba(244, 63, 94, 0.2)', color: 'var(--accent-rose)', border: '1px solid rgba(244, 63, 94, 0.4)', cursor: 'pointer' }}
                onClick={() => setActiveTab('alarms')}
              >
                <AlertTriangle size={14} />
                {activeAlarmCount} Active Alert{activeAlarmCount > 1 ? 's' : ''}
              </div>
            )}

            {/* Notification Bell with Inbox Dropdown */}
            <div style={{ position: 'relative' }}>
              <button
                className="btn btn-secondary"
                style={{
                  padding: '6px 8px',
                  position: 'relative',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  background: showInboxDropdown ? 'rgba(56, 189, 248, 0.15)' : undefined,
                  borderColor: showInboxDropdown ? 'var(--accent-cyan)' : undefined,
                }}
                onClick={() => {
                  setShowInboxDropdown(!showInboxDropdown);
                  loadInbox();
                }}
                title="Notifications Inbox"
              >
                <Bell size={16} />
                {unreadNotifCount > 0 && (
                  <span
                    style={{
                      position: 'absolute',
                      top: '-4px',
                      right: '-4px',
                      background: 'var(--accent-rose)',
                      color: '#fff',
                      fontSize: '0.65rem',
                      fontWeight: 700,
                      borderRadius: '9999px',
                      padding: '1px 5px',
                      minWidth: '16px',
                      textAlign: 'center',
                      boxShadow: '0 0 8px rgba(244, 63, 94, 0.6)',
                    }}
                  >
                    {unreadNotifCount}
                  </span>
                )}
              </button>

              {showInboxDropdown && (
                <div
                  style={{
                    position: 'absolute',
                    top: 'calc(100% + 8px)',
                    right: 0,
                    width: '360px',
                    background: 'var(--bg-card)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: 'var(--radius-md)',
                    boxShadow: '0 12px 36px rgba(0,0,0,0.4)',
                    backdropFilter: 'blur(16px)',
                    zIndex: 1000,
                    display: 'flex',
                    flexDirection: 'column',
                    maxHeight: '440px',
                    overflow: 'hidden',
                  }}
                >
                  <div
                    style={{
                      padding: '0.75rem 1rem',
                      borderBottom: '1px solid var(--border-subtle)',
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                    }}
                  >
                    <div style={{ fontWeight: 600, color: '#fff', fontSize: '0.9rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                      <Bell size={14} color="var(--accent-cyan)" />
                      <span>Notifications Inbox</span>
                    </div>
                    {unreadNotifCount > 0 && (
                      <button
                        style={{
                          background: 'none',
                          border: 'none',
                          color: 'var(--accent-cyan)',
                          fontSize: '0.75rem',
                          cursor: 'pointer',
                          textDecoration: 'underline',
                        }}
                        onClick={async () => {
                          await api.markAllInboxRead();
                          loadInbox();
                        }}
                      >
                        Mark all read
                      </button>
                    )}
                  </div>

                  <div style={{ overflowY: 'auto', flex: 1, padding: '0.5rem' }}>
                    {inboxNotifications.length === 0 ? (
                      <div style={{ padding: '2rem 1rem', textAlign: 'center', color: 'var(--text-muted)', fontSize: '0.85rem' }}>
                        No notifications in inbox. All quiet!
                      </div>
                    ) : (
                      inboxNotifications.map(n => (
                        <div
                          key={n.id}
                          style={{
                            padding: '0.6rem 0.75rem',
                            borderRadius: 'var(--radius-sm)',
                            background: n.status === 'UNREAD' ? 'rgba(56, 189, 248, 0.08)' : 'transparent',
                            borderBottom: '1px solid rgba(255,255,255,0.04)',
                            display: 'flex',
                            flexDirection: 'column',
                            gap: '0.2rem',
                          }}
                        >
                          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                            <span style={{ fontWeight: 600, fontSize: '0.82rem', color: '#fff' }}>{n.subject}</span>
                            <span style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>
                              {new Date(n.created_time).toLocaleTimeString()}
                            </span>
                          </div>
                          <div style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', lineHeight: 1.3 }}>
                            {n.body}
                          </div>
                        </div>
                      ))
                    )}
                  </div>

                  <div
                    style={{
                      padding: '0.5rem 1rem',
                      borderTop: '1px solid var(--border-subtle)',
                      background: 'rgba(0,0,0,0.2)',
                      fontSize: '0.75rem',
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                    }}
                  >
                    <a
                      href="http://localhost:8025"
                      target="_blank"
                      rel="noreferrer"
                      style={{ color: 'var(--accent-cyan)', textDecoration: 'none', display: 'flex', alignItems: 'center', gap: '0.3rem' }}
                    >
                      <ExternalLink size={12} />
                      <span>Mailpit Web UI (:8025)</span>
                    </a>
                    <span style={{ color: 'var(--text-muted)' }}>{inboxNotifications.length} items</span>
                  </div>
                </div>
              )}
            </div>

            <div className="badge-tag live">
              MQTT :1883 Active
            </div>
            <div className="badge-tag live">
              HTTP REST :8080
            </div>
          </div>
        </header>

        {/* Page Content View */}
        <div className="page-body">
          {/* TAB 1: OVERVIEW DASHBOARD */}
          {activeTab === 'dashboard' && (
            <>
              <div className="page-header">
                <div>
                  <h2 className="page-title">Operations Overview</h2>
                  <p className="page-subtitle">Real-time status of telemetry ingestion, rule engine, and device fleet</p>
                </div>
                <button className="btn btn-primary" onClick={() => { setActiveTab('devices'); setShowCreateModal(true); }}>
                  <Plus size={16} />
                  <span>Register Device</span>
                </button>
              </div>

              {/* Stats Grid */}
              <div className="stats-grid">
                <div className="stat-card">
                  <div className="stat-icon" style={{ background: 'rgba(6, 182, 212, 0.15)', color: 'var(--accent-cyan)' }}>
                    <Layers size={24} />
                  </div>
                  <div className="stat-meta">
                    <div className="label">Managed Devices</div>
                    <div className="value">{devices.length}</div>
                    <div className="delta">Fleet Inventory</div>
                  </div>
                </div>

                <div className="stat-card">
                  <div className="stat-icon" style={{ background: 'rgba(139, 92, 246, 0.15)', color: 'var(--accent-violet)' }}>
                    <LayoutDashboard size={24} />
                  </div>
                  <div className="stat-meta">
                    <div className="label">Live Dashboards</div>
                    <div className="value">{dashboards.length}</div>
                    <div className="delta">Custom Canvases</div>
                  </div>
                </div>

                <div className="stat-card">
                  <div className="stat-icon" style={{ background: 'rgba(244, 63, 94, 0.15)', color: 'var(--accent-rose)' }}>
                    <Bell size={24} />
                  </div>
                  <div className="stat-meta">
                    <div className="label">Active Alarms</div>
                    <div className="value">{activeAlarmCount}</div>
                    <div className="delta" style={{ color: activeAlarmCount > 0 ? 'var(--accent-rose)' : 'var(--accent-emerald)' }}>
                      {activeAlarmCount === 0 ? 'All systems nominal' : 'Attention required'}
                    </div>
                  </div>
                </div>

                <div className="stat-card">
                  <div className="stat-icon" style={{ background: 'rgba(245, 158, 11, 0.15)', color: 'var(--accent-amber)' }}>
                    <Clock size={24} />
                  </div>
                  <div className="stat-meta">
                    <div className="label">Ingestion Latency</div>
                    <div className="value">&lt; 1.5 ms</div>
                    <div className="delta">Sub-millisecond pipeline</div>
                  </div>
                </div>
              </div>

              {/* Architecture Pipeline Visual Card */}
              <div className="card">
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '20px' }}>
                  <h3 style={{ fontSize: '1.1rem', fontWeight: 700, display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <TrendingUp size={18} style={{ color: 'var(--accent-cyan)' }} />
                    Live Data, Downlink RPC & Dashboards Architecture
                  </h3>
                  <span className="badge-tag live">Zero-Loss Bus</span>
                </div>

                <div style={{
                  display: 'grid',
                  gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
                  gap: '16px',
                  alignItems: 'center',
                  background: 'var(--bg-surface)',
                  padding: '24px',
                  borderRadius: 'var(--radius-md)',
                  border: '1px solid var(--border-subtle)'
                }}>
                  <div style={{ textAlign: 'center' }}>
                    <div style={{ fontWeight: 700, color: 'var(--accent-cyan)' }}>1. Device Edge</div>
                    <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginTop: '4px' }}>
                      HTTP & MQTT Token Auth
                    </p>
                  </div>
                  <div style={{ textAlign: 'center', color: 'var(--text-muted)' }}>➔</div>
                  <div style={{ textAlign: 'center' }}>
                    <div style={{ fontWeight: 700, color: 'var(--accent-emerald)' }}>2. Event Bus</div>
                    <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginTop: '4px' }}>
                      Partitioned by Device ID
                    </p>
                  </div>
                  <div style={{ textAlign: 'center', color: 'var(--text-muted)' }}>➔</div>
                  <div style={{ textAlign: 'center' }}>
                    <div style={{ fontWeight: 700, color: 'var(--accent-violet)' }}>3. Rule Engine</div>
                    <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginTop: '4px' }}>
                      Thresholds & Alarm Trigger
                    </p>
                  </div>
                  <div style={{ textAlign: 'center', color: 'var(--text-muted)' }}>➔</div>
                  <div style={{ textAlign: 'center' }}>
                    <div style={{ fontWeight: 700, color: 'var(--accent-amber)' }}>4. Downlink RPC</div>
                    <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginTop: '4px' }}>
                      Two-Way Control Commands
                    </p>
                  </div>
                  <div style={{ textAlign: 'center', color: 'var(--text-muted)' }}>➔</div>
                  <div style={{ textAlign: 'center' }}>
                    <div style={{ fontWeight: 700, color: 'var(--accent-cyan)' }}>5. Dashboards</div>
                    <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginTop: '4px' }}>
                      Live Realtime Canvases
                    </p>
                  </div>
                </div>
              </div>

              {/* Quick Jump */}
              <div className="card">
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <div>
                    <h3 style={{ fontSize: '1.1rem', fontWeight: 700 }}>Interactive Realtime Canvases</h3>
                    <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', marginTop: '4px' }}>
                      View live telemetry gauges, timeseries charts, and execute two-way RPC controls directly.
                    </p>
                  </div>
                  <button className="btn btn-secondary" onClick={() => setActiveTab('dashboards')}>
                    <span>Open Dashboards ({dashboards.length})</span>
                    <ChevronRight size={16} />
                  </button>
                </div>
              </div>
            </>
          )}

          {/* TAB 2: LIVE DASHBOARDS VIEW */}
          {activeTab === 'dashboards' && (
            <>
              <div className="page-header">
                <div>
                  <h2 className="page-title">Live Dashboards Canvas</h2>
                  <p className="page-subtitle">Real-time analytical visualization cards, charts, and control switches</p>
                </div>
                <div style={{ display: 'flex', gap: '12px' }}>
                  <button className="btn btn-secondary" onClick={loadDashboards}>
                    <RefreshCw size={15} />
                    <span>Refresh</span>
                  </button>
                  <button className="btn btn-primary" onClick={() => setShowCreateDashModal(true)}>
                    <Plus size={16} />
                    <span>New Dashboard</span>
                  </button>
                </div>
              </div>

              {/* Dashboard Switcher & Device Target Bar */}
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '12px', paddingBottom: '12px', borderBottom: '1px solid var(--border-color)' }}>
                <div style={{ display: 'flex', gap: '10px', alignItems: 'center', overflowX: 'auto' }}>
                  {dashboards.map(d => (
                    <button 
                      key={d.id} 
                      className={`btn btn-sm ${selectedDashboard?.id === d.id ? 'btn-primary' : 'btn-secondary'}`}
                      onClick={() => setSelectedDashboard(d)}
                    >
                      <LayoutDashboard size={14} />
                      <span>{d.title}</span>
                    </button>
                  ))}
                </div>

                {/* Device Data Target Selector for Dashboard */}
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Data Source Device:</span>
                  <select 
                    className="form-input" 
                    style={{ padding: '5px 10px', fontSize: '0.85rem', width: 'auto' }}
                    value={dashSelectedDeviceId || selectedDashboard?.configuration?.deviceId || devices[0]?.id || ''}
                    onChange={e => {
                      setDashSelectedDeviceId(e.target.value);
                      loadDeviceData(e.target.value);
                    }}
                  >
                    {devices.map(d => (
                      <option key={d.id} value={d.id}>{d.name} {d.name.includes('OPC_UA') ? '(OPC UA Server)' : ''}</option>
                    ))}
                  </select>
                </div>
              </div>

              {selectedDashboard && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
                  {/* Dashboard Widgets Grid */}
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '20px' }}>
                    {/* Widget 1: Primary Process Variable Card */}
                    <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                        <span style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', fontWeight: 600, textTransform: 'uppercase' }}>
                          {latestTelemetry['Boiler.Temperature'] ? 'Boiler Temperature' : 'Ambient Temperature'}
                        </span>
                        <span className="badge-tag live">Realtime</span>
                      </div>
                      <div style={{ fontSize: '2.4rem', fontWeight: 800, color: '#fff', fontFamily: 'var(--font-mono)' }}>
                        {latestTelemetry['Boiler.Temperature'] ? latestTelemetry['Boiler.Temperature'][0]?.value : (latestTelemetry['temperature'] ? latestTelemetry['temperature'][0]?.value : 82.5)} °C
                      </div>
                      <span style={{ fontSize: '0.75rem', color: 'var(--accent-cyan)' }}>
                        Source: {selectedDevice?.name || 'Live Edge Fleet'}
                      </span>
                    </div>

                    {/* Widget 2: Secondary Process Variable Card */}
                    <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                        <span style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', fontWeight: 600, textTransform: 'uppercase' }}>
                          {latestTelemetry['Boiler.Pressure'] ? 'Steam Pressure' : (latestTelemetry['FeedPump.SpeedRPM'] ? 'Feed Pump Speed' : 'Relative Humidity')}
                        </span>
                        <span className="badge-tag live">Realtime</span>
                      </div>
                      <div style={{ fontSize: '2.4rem', fontWeight: 800, color: 'var(--accent-emerald)', fontFamily: 'var(--font-mono)' }}>
                        {latestTelemetry['Boiler.Pressure'] ? `${latestTelemetry['Boiler.Pressure'][0]?.value} bar` : (latestTelemetry['FeedPump.SpeedRPM'] ? `${latestTelemetry['FeedPump.SpeedRPM'][0]?.value} RPM` : (latestTelemetry['humidity'] ? `${latestTelemetry['humidity'][0]?.value} %` : '18.4 bar'))}
                      </div>
                      <span style={{ fontSize: '0.75rem', color: 'var(--accent-emerald)' }}>
                        Continuous Ingestion & TimescaleDB
                      </span>
                    </div>

                    {/* Widget 3: Live RPC Downlink Control Switch */}
                    <div className="card" style={{ display: 'flex', flexDirection: 'column', justifyContent: 'space-between', gap: '12px' }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                        <span style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', fontWeight: 600, textTransform: 'uppercase' }}>
                          Two-Way RPC Downlink Switch
                        </span>
                        <span className="badge-tag" style={{ background: relayActive ? 'rgba(16, 185, 129, 0.2)' : 'rgba(255,255,255,0.1)', color: relayActive ? 'var(--accent-emerald)' : 'var(--text-muted)' }}>
                          {relayActive ? 'CLOSED (ON)' : 'OPEN (OFF)'}
                        </span>
                      </div>

                      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                        <div>
                          <div style={{ fontWeight: 700, fontSize: '1rem', color: '#fff' }}>Relay Switch #1</div>
                          <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>Command: setRelay</div>
                        </div>

                        <button 
                          className={`btn ${relayActive ? 'btn-primary' : 'btn-secondary'}`}
                          onClick={handleToggleRelayFromDashboard}
                          style={{ padding: '8px 16px', display: 'flex', alignItems: 'center', gap: '8px' }}
                        >
                          <Power size={16} />
                          <span>{relayActive ? 'Turn OFF' : 'Turn ON'}</span>
                        </button>
                      </div>

                      <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                        Dispatches authenticated RPC payload to edge device with bidirectional ack.
                      </div>
                    </div>
                  </div>

                  {/* Widget 4: Live Interactive Timeseries Graph */}
                  <div className="chart-container">
                    <div className="chart-header">
                      <div>
                        <h3 style={{ fontSize: '1rem', fontWeight: 700, display: 'flex', alignItems: 'center', gap: '8px' }}>
                          <TrendingUp size={16} style={{ color: 'var(--accent-cyan)' }} />
                          <span>Telemetry History & Trends: <code style={{ color: 'var(--accent-cyan)' }}>{chartPoints?.key || selectedChartKey}</code></span>
                        </h3>
                        <p style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                          Continuous Realtime Timeseries Ingestion
                        </p>
                      </div>

                      <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                        {Object.keys(latestTelemetry).length > 0 && (
                          <select 
                            className="form-input"
                            style={{ padding: '4px 8px', fontSize: '0.8rem', width: 'auto' }}
                            value={chartPoints?.key || selectedChartKey}
                            onChange={e => setSelectedChartKey(e.target.value)}
                          >
                            {Object.keys(latestTelemetry).map(k => (
                              <option key={k} value={k}>{k}</option>
                            ))}
                          </select>
                        )}
                        {chartPoints && (
                          <span className="badge-tag live">
                            {chartPoints.pts.length} points
                          </span>
                        )}
                      </div>
                    </div>

                    {chartPoints && chartPoints.pts.length > 1 ? (
                      <svg className="svg-chart" viewBox={`0 0 ${chartPoints.width} ${chartPoints.height}`}>
                        <defs>
                          <linearGradient id="dashGradient" x1="0" y1="0" x2="0" y2="1">
                            <stop offset="0%" stopColor="var(--accent-cyan)" stopOpacity="0.4" />
                            <stop offset="100%" stopColor="var(--accent-cyan)" stopOpacity="0.0" />
                          </linearGradient>
                        </defs>
                        <line x1="20" y1="20" x2={chartPoints.width - 20} y2="20" stroke="var(--border-subtle)" strokeDasharray="4" />
                        <line x1="20" y1={chartPoints.height / 2} x2={chartPoints.width - 20} y2={chartPoints.height / 2} stroke="var(--border-subtle)" strokeDasharray="4" />
                        <line x1="20" y1={chartPoints.height - 20} x2={chartPoints.width - 20} y2={chartPoints.height - 20} stroke="var(--border-subtle)" strokeDasharray="4" />

                        <path d={chartPoints.areaD} fill="url(#dashGradient)" />
                        <path d={chartPoints.pathD} fill="none" stroke="var(--accent-cyan)" strokeWidth="2.5" />

                        {chartPoints.pts.map((p, i) => (
                          <circle key={i} cx={p.x} cy={p.y} r="3" fill="#fff" stroke="var(--accent-cyan)" strokeWidth="2">
                            <title>{`${p.val}°C at ${new Date(p.ts).toLocaleTimeString()}`}</title>
                          </circle>
                        ))}
                      </svg>
                    ) : (
                      <div style={{ textAlign: 'center', padding: '48px', color: 'var(--text-muted)', fontSize: '0.85rem' }}>
                        Awaiting data stream points...
                      </div>
                    )}
                  </div>
                </div>
              )}
            </>
          )}

          {/* TAB 3: DEVICES FLEET */}
          {activeTab === 'devices' && (
            <>
              <div className="page-header">
                <div>
                  <h2 className="page-title">Device Fleet</h2>
                  <p className="page-subtitle">Provision, authenticate, and monitor edge devices</p>
                </div>
                <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
                  <button className="btn btn-secondary" onClick={loadDevices}>
                    <RefreshCw size={15} className={devicesLoading ? 'animate-spin' : ''} />
                    <span>Refresh</span>
                  </button>
                  <button className="btn btn-secondary" onClick={handleExportDevices}>
                    <Download size={15} />
                    <span>Export CSV</span>
                  </button>
                  <button className="btn btn-secondary" onClick={() => { setBulkImportResult(null); setShowBulkImportModal(true); }}>
                    <Upload size={15} />
                    <span>Bulk Import</span>
                  </button>
                  <button className="btn btn-primary" onClick={() => setShowCreateModal(true)}>
                    <Plus size={16} />
                    <span>Create Device</span>
                  </button>
                </div>
              </div>

              <div className="table-container">
                <table className="table">
                  <thead>
                    <tr>
                      <th>Status</th>
                      <th>Device Name</th>
                      <th>Label</th>
                      <th>Access Token</th>
                      <th>Created</th>
                      <th style={{ textAlign: 'right' }}>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {devices.length === 0 ? (
                      <tr>
                        <td colSpan={6} style={{ textAlign: 'center', padding: '48px', color: 'var(--text-muted)' }}>
                          {devicesLoading ? 'Loading device inventory...' : 'No devices found in this tenant. Click "Create Device" to register one.'}
                        </td>
                      </tr>
                    ) : (
                      devices.map(dev => {
                        const tokenStr = deviceTokens[dev.id] || 'Loading...';
                        const isOnline = deviceActivities[dev.id]?.active ?? true;
                        return (
                          <tr key={dev.id}>
                            <td>
                              <span style={{ 
                                display: 'inline-flex', 
                                alignItems: 'center', 
                                gap: '6px', 
                                fontSize: '0.8rem', 
                                color: isOnline ? 'var(--accent-emerald)' : 'var(--text-muted)' 
                              }}>
                                <span style={{ 
                                  width: '8px', 
                                  height: '8px', 
                                  borderRadius: '50%', 
                                  background: isOnline ? 'var(--accent-emerald)' : 'var(--text-muted)', 
                                  boxShadow: isOnline ? '0 0 6px var(--accent-emerald)' : 'none' 
                                }}></span>
                                {isOnline ? 'Online' : 'Idle'}
                              </span>
                            </td>
                            <td>
                              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                                <div style={{ fontWeight: 600, color: '#fff', cursor: 'pointer' }} onClick={() => { setSelectedDeviceId(dev.id); setActiveTab('device-detail'); }}>
                                  {dev.name}
                                </div>
                                {dev.name.includes('OPC_UA') && (
                                  <span style={{ 
                                    background: 'rgba(6, 182, 212, 0.15)', 
                                    color: 'var(--accent-cyan)', 
                                    border: '1px solid rgba(6, 182, 212, 0.3)',
                                    fontSize: '0.65rem',
                                    fontWeight: 700,
                                    padding: '1px 6px',
                                    borderRadius: '4px'
                                  }}>
                                    OPC UA TCP :4840
                                  </span>
                                )}
                              </div>
                              <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
                                {dev.id}
                              </div>
                            </td>
                            <td>
                              <span className="badge-tag" style={{ background: 'var(--bg-surface-elevated)', color: 'var(--text-secondary)' }}>
                                {dev.label || 'Default Profile'}
                              </span>
                            </td>
                            <td>
                              <div style={{ display: 'inline-flex', alignItems: 'center', gap: '8px' }}>
                                <code style={{ fontSize: '0.8rem', background: 'rgba(0,0,0,0.3)', padding: '2px 8px', borderRadius: '4px', color: 'var(--accent-cyan)' }}>
                                  {tokenStr}
                                </code>
                                <button 
                                  className="btn btn-secondary btn-sm"
                                  style={{ padding: '3px 6px' }}
                                  onClick={() => copyToClipboard(tokenStr, dev.id)}
                                  title="Copy Device Token"
                                >
                                  {copiedToken === dev.id ? <Check size={12} style={{ color: 'var(--accent-emerald)' }} /> : <Copy size={12} />}
                                </button>
                              </div>
                            </td>
                            <td style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                              {new Date(dev.created_time).toLocaleString()}
                            </td>
                            <td style={{ textAlign: 'right' }}>
                              <div style={{ display: 'inline-flex', gap: '8px' }}>
                                <button 
                                  className="btn btn-secondary btn-sm"
                                  onClick={() => {
                                    setSelectedDeviceId(dev.id);
                                    setActiveTab('device-detail');
                                  }}
                                >
                                  <Sliders size={14} />
                                  <span>Device Studio</span>
                                </button>
                                <button 
                                  className="btn btn-danger btn-sm"
                                  onClick={() => handleDeleteDevice(dev.id)}
                                  title="Delete Device"
                                >
                                  <Trash2 size={14} />
                                </button>
                              </div>
                            </td>
                          </tr>
                        );
                      })
                    )}
                  </tbody>
                </table>
              </div>
            </>
          )}

          {/* TAB 4: DEVICE DETAIL (Telemetry + Scoped Attributes + Downlink RPC Studio) */}
          {activeTab === 'device-detail' && selectedDeviceId && (
            <>
              <div className="page-header">
                <div>
                  <button 
                    className="btn btn-secondary btn-sm" 
                    onClick={() => setActiveTab('devices')}
                    style={{ marginBottom: '8px' }}
                  >
                    ← Back to Fleet
                  </button>
                  <h2 className="page-title">{selectedDevice?.name || 'Device Details'}</h2>
                  <p className="page-subtitle">
                    ID: <span className="font-mono">{selectedDeviceId}</span> · Token: <span className="font-mono" style={{ color: 'var(--accent-cyan)' }}>{deviceTokens[selectedDeviceId] || 'Fetching...'}</span>
                  </p>
                </div>

                <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                  <div style={{ display: 'flex', background: 'var(--bg-surface-elevated)', borderRadius: 'var(--radius-md)', padding: '3px' }}>
                    <button 
                      className={`btn btn-sm ${deviceDetailSubTab === 'telemetry' ? 'btn-primary' : 'btn-secondary'}`}
                      onClick={() => setDeviceDetailSubTab('telemetry')}
                      style={{ border: 'none' }}
                    >
                      <Activity size={14} />
                      <span>Telemetry & Simulator</span>
                    </button>
                    <button 
                      className={`btn btn-sm ${deviceDetailSubTab === 'attributes' ? 'btn-primary' : 'btn-secondary'}`}
                      onClick={() => setDeviceDetailSubTab('attributes')}
                      style={{ border: 'none' }}
                    >
                      <Tag size={14} />
                      <span>Attributes ({attributes.length})</span>
                    </button>
                    <button 
                      className={`btn btn-sm ${deviceDetailSubTab === 'rpc' ? 'btn-primary' : 'btn-secondary'}`}
                      onClick={() => setDeviceDetailSubTab('rpc')}
                      style={{ border: 'none' }}
                    >
                      <Terminal size={14} />
                      <span>RPC Downlink ({rpcCommands.length})</span>
                    </button>
                  </div>

                  <label style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '0.85rem', cursor: 'pointer' }}>
                    <input 
                      type="checkbox" 
                      checked={autoRefresh} 
                      onChange={e => setAutoRefresh(e.target.checked)} 
                    />
                    <span>Auto-Poll (2s)</span>
                  </label>
                  <button className="btn btn-secondary" onClick={() => loadDeviceData(selectedDeviceId)}>
                    <RefreshCw size={15} />
                    <span>Refresh</span>
                  </button>
                </div>
              </div>

              {/* SUBTAB 1: TELEMETRY & SIMULATOR */}
              {deviceDetailSubTab === 'telemetry' && (
                <>
                  {/* Latest Telemetry Cards */}
                  <div className="card">
                    <h3 style={{ fontSize: '1.1rem', fontWeight: 700, marginBottom: '16px', display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <Activity size={18} style={{ color: 'var(--accent-emerald)' }} />
                      Latest Ingested Telemetry
                    </h3>

                    {Object.keys(latestTelemetry).length === 0 ? (
                      <div style={{ textAlign: 'center', padding: '32px', color: 'var(--text-muted)' }}>
                        No telemetry received yet. Use the simulator below or post via HTTP/MQTT to see live values!
                      </div>
                    ) : (
                      <div className="telemetry-grid">
                        {Object.entries(latestTelemetry).map(([key, pts]) => {
                          const latestPt = pts && pts[0];
                          const val = latestPt ? latestPt.value : '--';
                          const ts = latestPt ? new Date(latestPt.ts).toLocaleTimeString() : '';
                          const isSelected = (chartPoints?.key || selectedChartKey) === key;
                          return (
                            <div 
                              key={key} 
                              className="telemetry-card"
                              onClick={() => setSelectedChartKey(key)}
                              style={{ 
                                cursor: 'pointer',
                                borderColor: isSelected ? 'var(--accent-cyan)' : undefined,
                                background: isSelected ? 'rgba(6, 182, 212, 0.08)' : undefined
                              }}
                            >
                              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <div className="telemetry-key">{key}</div>
                                {isSelected && (
                                  <span style={{ fontSize: '0.65rem', background: 'var(--accent-cyan)', color: '#000', fontWeight: 700, padding: '1px 5px', borderRadius: '4px' }}>
                                    PLOTTING
                                  </span>
                                )}
                              </div>
                              <div className="telemetry-val">
                                {typeof val === 'number' ? val.toLocaleString() : String(val)}
                                {key.toLowerCase().includes('temp') ? ' °C' : key.toLowerCase().includes('press') ? ' bar' : key.toLowerCase().includes('rpm') ? ' RPM' : key.toLowerCase().includes('flow') ? ' GPM' : key === 'humidity' ? ' %' : key === 'voltage' ? ' V' : key === 'battery' ? ' %' : ''}
                              </div>
                              <div className="telemetry-ts">Updated: {ts} · Click to Plot</div>
                            </div>
                          );
                        })}
                      </div>
                    )}
                  </div>

                  {/* Telemetry Sparkline / Historical Chart */}
                  <div className="chart-container">
                    <div className="chart-header">
                      <div>
                        <h3 style={{ fontSize: '1rem', fontWeight: 700, display: 'flex', alignItems: 'center', gap: '8px' }}>
                          <TrendingUp size={16} style={{ color: 'var(--accent-cyan)' }} />
                          <span>Timeseries Trend: <code style={{ color: 'var(--accent-cyan)' }}>{chartPoints?.key || selectedChartKey}</code></span>
                        </h3>
                        <p style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                          Click any metric card above or select from the dropdown to plot live points.
                        </p>
                      </div>

                      <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                        {Object.keys(latestTelemetry).length > 0 && (
                          <select 
                            className="form-input"
                            style={{ padding: '4px 8px', fontSize: '0.8rem', width: 'auto' }}
                            value={chartPoints?.key || selectedChartKey}
                            onChange={e => setSelectedChartKey(e.target.value)}
                          >
                            {Object.keys(latestTelemetry).map(k => (
                              <option key={k} value={k}>{k}</option>
                            ))}
                          </select>
                        )}
                        {chartPoints && (
                          <span className="badge-tag live">
                            {chartPoints.pts.length} points plotted
                          </span>
                        )}
                      </div>
                    </div>

                    {chartPoints && chartPoints.pts.length > 1 ? (
                      <svg className="svg-chart" viewBox={`0 0 ${chartPoints.width} ${chartPoints.height}`}>
                        <defs>
                          <linearGradient id="chartGradient" x1="0" y1="0" x2="0" y2="1">
                            <stop offset="0%" stopColor="var(--accent-cyan)" stopOpacity="0.4" />
                            <stop offset="100%" stopColor="var(--accent-cyan)" stopOpacity="0.0" />
                          </linearGradient>
                        </defs>
                        <line x1="20" y1="20" x2={chartPoints.width - 20} y2="20" stroke="var(--border-subtle)" strokeDasharray="4" />
                        <line x1="20" y1={chartPoints.height / 2} x2={chartPoints.width - 20} y2={chartPoints.height / 2} stroke="var(--border-subtle)" strokeDasharray="4" />
                        <line x1="20" y1={chartPoints.height - 20} x2={chartPoints.width - 20} y2={chartPoints.height - 20} stroke="var(--border-subtle)" strokeDasharray="4" />

                        <path d={chartPoints.areaD} fill="url(#chartGradient)" />
                        <path d={chartPoints.pathD} fill="none" stroke="var(--accent-cyan)" strokeWidth="2.5" />

                        {chartPoints.pts.map((p, i) => (
                          <circle key={i} cx={p.x} cy={p.y} r="3" fill="#fff" stroke="var(--accent-cyan)" strokeWidth="2">
                            <title>{`${p.val}°C at ${new Date(p.ts).toLocaleTimeString()}`}</title>
                          </circle>
                        ))}
                      </svg>
                    ) : (
                      <div style={{ textAlign: 'center', padding: '48px', color: 'var(--text-muted)', fontSize: '0.85rem' }}>
                        Send telemetry points using the simulator below to generate the history graph.
                      </div>
                    )}
                  </div>

                  {/* Integrated Interactive Device Simulator */}
                  <div className="simulator-box">
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                      <div>
                        <h3 style={{ fontSize: '1.1rem', fontWeight: 700, display: 'flex', alignItems: 'center', gap: '8px', color: '#fff' }}>
                          <Sliders size={18} style={{ color: 'var(--accent-cyan)' }} />
                          Interactive Device Telemetry Simulator
                        </h3>
                        <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', marginTop: '4px' }}>
                          Drag temperature &gt; 45°C to watch the Rule Engine trigger an automated CRITICAL ALARM!
                        </p>
                      </div>

                      <div style={{ display: 'flex', gap: '10px' }}>
                        <button 
                          className={`btn ${isSimulating ? 'btn-danger' : 'btn-primary'}`} 
                          onClick={() => setIsSimulating(!isSimulating)}
                        >
                          {isSimulating ? <Square size={16} /> : <Play size={16} />}
                          <span>{isSimulating ? 'Stop Auto-Stream' : 'Auto-Stream (2s)'}</span>
                        </button>
                        <button className="btn btn-secondary" onClick={sendSingleSimulation}>
                          <Send size={15} />
                          <span>Send Single Sample</span>
                        </button>
                      </div>
                    </div>

                    {/* Slider Controls */}
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '14px', background: 'var(--bg-surface)', padding: '20px', borderRadius: 'var(--radius-md)' }}>
                      <div className="slider-row">
                        <span className="slider-label" style={{ color: simTemp > 45 ? 'var(--accent-rose)' : simTemp > 35 ? 'var(--accent-amber)' : 'inherit' }}>
                          Temperature (°C) {simTemp > 45 ? '🔥 [ALARM!]' : ''}
                        </span>
                        <input 
                          type="range" 
                          min="0" 
                          max="60" 
                          step="0.5" 
                          value={simTemp} 
                          onChange={e => setSimTemp(parseFloat(e.target.value))} 
                        />
                        <span className="slider-val" style={{ color: simTemp > 45 ? 'var(--accent-rose)' : 'var(--accent-cyan)' }}>
                          {simTemp} °C
                        </span>
                      </div>

                      <div className="slider-row">
                        <span className="slider-label">Humidity (%)</span>
                        <input 
                          type="range" 
                          min="10" 
                          max="100" 
                          step="1" 
                          value={simHumidity} 
                          onChange={e => setSimHumidity(parseFloat(e.target.value))} 
                        />
                        <span className="slider-val">{simHumidity} %</span>
                      </div>

                      <div className="slider-row">
                        <span className="slider-label">Voltage (V)</span>
                        <input 
                          type="range" 
                          min="2.8" 
                          max="4.5" 
                          step="0.1" 
                          value={simVoltage} 
                          onChange={e => setSimVoltage(parseFloat(e.target.value))} 
                        />
                        <span className="slider-val">{simVoltage} V</span>
                      </div>

                      <div className="slider-row">
                        <span className="slider-label">Battery (%)</span>
                        <input 
                          type="range" 
                          min="0" 
                          max="100" 
                          step="1" 
                          value={simBattery} 
                          onChange={e => setSimBattery(parseFloat(e.target.value))} 
                        />
                        <span className="slider-val">{simBattery} %</span>
                      </div>
                    </div>

                    {/* Simulation Transmission Logs */}
                    <div>
                      <div style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '6px', textTransform: 'uppercase' }}>
                        Simulator Transmission Log
                      </div>
                      <div style={{
                        background: '#040711',
                        border: '1px solid var(--border-subtle)',
                        borderRadius: 'var(--radius-md)',
                        padding: '12px',
                        height: '110px',
                        overflowY: 'auto',
                        fontFamily: 'var(--font-mono)',
                        fontSize: '0.75rem',
                        color: 'var(--accent-emerald)',
                        display: 'flex',
                        flexDirection: 'column',
                        gap: '4px'
                      }}>
                        {simLog.length === 0 ? (
                          <span style={{ color: 'var(--text-muted)' }}>No simulation packets sent yet. Click "Send Single Sample" or "Auto-Stream".</span>
                        ) : (
                          simLog.map((log, idx) => <div key={idx}>{log}</div>)
                        )}
                      </div>
                    </div>
                  </div>

                  {/* Physical Ingestion Quickstart */}
                  <div className="card">
                    <h3 style={{ fontSize: '1.1rem', fontWeight: 700, marginBottom: '12px', display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <Terminal size={18} style={{ color: 'var(--accent-amber)' }} />
                      Physical Device Ingestion Guide
                    </h3>
                    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(360px, 1fr))', gap: '16px' }}>
                      <div>
                        <div style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '6px' }}>
                          MQTT Telemetry Publish (Port 1883)
                        </div>
                        <pre style={{
                          background: 'var(--bg-surface)',
                          border: '1px solid var(--border-subtle)',
                          padding: '12px',
                          borderRadius: 'var(--radius-md)',
                          fontSize: '0.75rem',
                          color: 'var(--text-primary)',
                          overflowX: 'auto'
                        }}>
{`mosquitto_pub -h localhost -p 1883 \\
  -u "${deviceTokens[selectedDeviceId] || 'YOUR_TOKEN'}" \\
  -t "v1/devices/me/telemetry" \\
  -m '{"temperature":26.2,"humidity":58}'`}
                        </pre>
                      </div>

                      <div>
                        <div style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '6px' }}>
                          HTTP Telemetry Upload (Port 8080)
                        </div>
                        <pre style={{
                          background: 'var(--bg-surface)',
                          border: '1px solid var(--border-subtle)',
                          padding: '12px',
                          borderRadius: 'var(--radius-md)',
                          fontSize: '0.75rem',
                          color: 'var(--text-primary)',
                          overflowX: 'auto'
                        }}>
{`curl -X POST http://localhost:8080/api/v1/${deviceTokens[selectedDeviceId] || 'YOUR_TOKEN'}/telemetry \\
  -H "Content-Type: application/json" \\
  -d '{"temperature":26.2,"humidity":58}'`}
                        </pre>
                      </div>
                    </div>
                  </div>
                </>
              )}

              {/* SUBTAB 2: ATTRIBUTES MANAGER */}
              {deviceDetailSubTab === 'attributes' && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
                  {/* Save Attribute Form */}
                  <div className="card">
                    <h3 style={{ fontSize: '1.1rem', fontWeight: 700, marginBottom: '16px' }}>
                      Add or Update Device Attribute
                    </h3>
                    <form onSubmit={handleSaveAttribute} style={{ display: 'grid', gridTemplateColumns: '180px 1fr 1fr auto', gap: '12px', alignItems: 'end' }}>
                      <div className="form-group" style={{ margin: 0 }}>
                        <label className="form-label">Scope</label>
                        <select 
                          className="form-input" 
                          value={newAttrScope} 
                          onChange={(e: any) => setNewAttrScope(e.target.value)}
                        >
                          <option value="SERVER_SCOPE">SERVER_SCOPE</option>
                          <option value="SHARED_SCOPE">SHARED_SCOPE</option>
                        </select>
                      </div>

                      <div className="form-group" style={{ margin: 0 }}>
                        <label className="form-label">Key Name</label>
                        <input 
                          type="text" 
                          className="form-input" 
                          placeholder="e.g. location or target_temp" 
                          value={newAttrKey} 
                          onChange={e => setNewAttrKey(e.target.value)} 
                          required 
                        />
                      </div>

                      <div className="form-group" style={{ margin: 0 }}>
                        <label className="form-label">Value</label>
                        <input 
                          type="text" 
                          className="form-input" 
                          placeholder="e.g. 24.5 or Factory Floor" 
                          value={newAttrVal} 
                          onChange={e => setNewAttrVal(e.target.value)} 
                          required 
                        />
                      </div>

                      <button type="submit" className="btn btn-primary" style={{ height: '42px' }}>
                        <Plus size={16} />
                        <span>Save</span>
                      </button>
                    </form>
                  </div>

                  {/* Attributes Table */}
                  <div className="table-container">
                    <table className="table">
                      <thead>
                        <tr>
                          <th>Scope</th>
                          <th>Key</th>
                          <th>Value</th>
                          <th>Last Updated</th>
                          <th style={{ textAlign: 'right' }}>Action</th>
                        </tr>
                      </thead>
                      <tbody>
                        {attributes.length === 0 ? (
                          <tr>
                            <td colSpan={5} style={{ textAlign: 'center', padding: '36px', color: 'var(--text-muted)' }}>
                              No attributes configured for this device yet.
                            </td>
                          </tr>
                        ) : (
                          attributes.map(attr => (
                            <tr key={`${attr.scope}-${attr.key}`}>
                              <td>
                                <span className="badge-tag" style={{
                                  background: attr.scope === 'CLIENT_SCOPE' ? 'rgba(6, 182, 212, 0.15)' : attr.scope === 'SHARED_SCOPE' ? 'rgba(139, 92, 246, 0.15)' : 'rgba(245, 158, 11, 0.15)',
                                  color: attr.scope === 'CLIENT_SCOPE' ? 'var(--accent-cyan)' : attr.scope === 'SHARED_SCOPE' ? 'var(--accent-violet)' : 'var(--accent-amber)',
                                  border: '1px solid var(--border-subtle)'
                                }}>
                                  {attr.scope}
                                </span>
                              </td>
                              <td style={{ fontWeight: 600, color: '#fff' }}>
                                {attr.key}
                              </td>
                              <td className="font-mono" style={{ color: 'var(--accent-emerald)' }}>
                                {typeof attr.value === 'object' ? JSON.stringify(attr.value) : String(attr.value)}
                              </td>
                              <td style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                                {new Date(attr.last_update_ts).toLocaleString()}
                              </td>
                              <td style={{ textAlign: 'right' }}>
                                <button 
                                  className="btn btn-danger btn-sm"
                                  onClick={() => handleDeleteAttribute(attr.scope, attr.key)}
                                  title="Delete Attribute"
                                >
                                  <Trash2 size={13} />
                                </button>
                              </td>
                            </tr>
                          ))
                        )}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}

              {/* SUBTAB 3: DOWNLINK RPC CONTROL TERMINAL */}
              {deviceDetailSubTab === 'rpc' && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
                  {/* Send RPC Form */}
                  <div className="card">
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px', flexWrap: 'wrap', gap: '10px' }}>
                      <h3 style={{ fontSize: '1.1rem', fontWeight: 700, margin: 0, display: 'flex', alignItems: 'center', gap: '8px' }}>
                        <Terminal size={18} style={{ color: 'var(--accent-amber)' }} />
                        Downlink RPC Command Studio
                      </h3>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', fontSize: '0.85rem' }}>
                        <label style={{ display: 'flex', alignItems: 'center', gap: '6px', cursor: 'pointer', color: '#cbd5e1' }}>
                          <input 
                            type="checkbox" 
                            checked={rpcTwoWay} 
                            onChange={e => setRpcTwoWay(e.target.checked)} 
                          />
                          Two-Way (Expect Device Response)
                        </label>
                        {rpcTwoWay && (
                          <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
                            <span style={{ color: '#94a3b8', fontSize: '0.78rem' }}>Timeout:</span>
                            <input 
                              type="number" 
                              className="form-input" 
                              style={{ width: '80px', padding: '4px 8px', fontSize: '0.8rem' }}
                              value={rpcTimeoutMs}
                              onChange={e => setRpcTimeoutMs(Number(e.target.value))}
                              min={500}
                              max={60000}
                              step={500}
                            />
                            <span style={{ color: '#94a3b8', fontSize: '0.78rem' }}>ms</span>
                          </div>
                        )}
                      </div>
                    </div>

                    {/* Presets */}
                    <div style={{ display: 'flex', gap: '8px', marginBottom: '16px', flexWrap: 'wrap' }}>
                      <span style={{ fontSize: '0.8rem', color: '#94a3b8', alignSelf: 'center', marginRight: '4px' }}>Presets:</span>
                      <button 
                        className="btn btn-secondary btn-sm" 
                        onClick={() => { setRpcMethod('setRelay'); setRpcParams('{"relay": 1, "state": true}'); }}
                      >
                        setRelay (ON)
                      </button>
                      <button 
                        className="btn btn-secondary btn-sm" 
                        onClick={() => { setRpcMethod('setRelay'); setRpcParams('{"relay": 1, "state": false}'); }}
                      >
                        setRelay (OFF)
                      </button>
                      <button 
                        className="btn btn-secondary btn-sm" 
                        onClick={() => { setRpcMethod('reboot'); setRpcParams('{"delaySec": 5}'); }}
                      >
                        reboot
                      </button>
                      <button 
                        className="btn btn-secondary btn-sm" 
                        onClick={() => { setRpcMethod('setTelemetryInterval'); setRpcParams('{"intervalSec": 10}'); }}
                      >
                        setTelemetryInterval
                      </button>
                      <button 
                        className="btn btn-secondary btn-sm" 
                        onClick={() => { setRpcMethod('triggerDiagnosticScan'); setRpcParams('{"depth": "full", "sensors": ["temp", "humidity"]}'); }}
                      >
                        triggerDiagnosticScan
                      </button>
                    </div>

                    <div style={{ display: 'grid', gridTemplateColumns: '220px 1fr auto', gap: '12px', alignItems: 'end' }}>
                      <div className="form-group" style={{ margin: 0 }}>
                        <label className="form-label">RPC Method</label>
                        <input 
                          type="text" 
                          className="form-input" 
                          value={rpcMethod} 
                          onChange={e => setRpcMethod(e.target.value)} 
                          placeholder="e.g. setRelay" 
                        />
                      </div>

                      <div className="form-group" style={{ margin: 0 }}>
                        <label className="form-label">Parameters (JSON)</label>
                        <input 
                          type="text" 
                          className="form-input font-mono" 
                          value={rpcParams} 
                          onChange={e => setRpcParams(e.target.value)} 
                          placeholder='{"relay": 1, "state": true}' 
                        />
                      </div>

                      <button 
                        className="btn btn-primary" 
                        style={{ height: '42px', minWidth: '140px' }} 
                        onClick={() => handleSendRpc(rpcTwoWay)} 
                        disabled={rpcSending}
                      >
                        {rpcSending ? <RefreshCw className="animate-spin" size={16} /> : <Send size={16} />}
                        <span>{rpcTwoWay ? 'Execute (Sync)' : 'Send (Async)'}</span>
                      </button>
                    </div>

                    {/* Live Response Panel */}
                    {rpcLastResponse && (
                      <div style={{ 
                        marginTop: '16px', 
                        padding: '12px 16px', 
                        background: 'rgba(30, 41, 59, 0.6)', 
                        border: '1px solid rgba(255, 255, 255, 0.08)', 
                        borderRadius: '8px' 
                      }}>
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                            <span style={{ fontSize: '0.8rem', fontWeight: 600, color: '#94a3b8' }}>Latest Response:</span>
                            <span className="badge-tag" style={{
                              background: rpcLastResponse.status === 'SUCCESSFUL' ? 'rgba(16, 185, 129, 0.2)' : 'rgba(56, 189, 248, 0.2)',
                              color: rpcLastResponse.status === 'SUCCESSFUL' ? '#34d399' : '#38bdf8'
                            }}>
                              {rpcLastResponse.status}
                            </span>
                            {rpcLastDurationMs !== null && (
                              <span style={{ fontSize: '0.75rem', color: '#cbd5e1' }}>
                                Latency: <strong>{rpcLastDurationMs} ms</strong>
                              </span>
                            )}
                          </div>
                          <span style={{ fontSize: '0.72rem', color: '#64748b' }}>
                            Cmd ID: {rpcLastResponse.id?.slice(0, 8)}...
                          </span>
                        </div>
                        <pre className="font-mono" style={{ margin: 0, fontSize: '0.8rem', color: '#38bdf8', overflowX: 'auto', maxHeight: '120px' }}>
                          {JSON.stringify(rpcLastResponse.response || rpcLastResponse, null, 2)}
                        </pre>
                      </div>
                    )}
                  </div>

                  {/* RPC History Table */}
                  <div className="table-container">
                    <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--border-subtle)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <span style={{ fontWeight: 600, color: '#ffffff' }}>Command History & Delivery Logs</span>
                      <button 
                        className="btn btn-secondary btn-sm" 
                        onClick={() => selectedDeviceId && loadDeviceData(selectedDeviceId)}
                      >
                        <RefreshCw size={13} />
                        Refresh
                      </button>
                    </div>
                    <table className="table">
                      <thead>
                        <tr>
                          <th>Status</th>
                          <th>Method</th>
                          <th>Mode</th>
                          <th>Parameters</th>
                          <th>Response</th>
                          <th>Retries</th>
                          <th>Timestamp</th>
                          <th style={{ textAlign: 'right' }}>Actions</th>
                        </tr>
                      </thead>
                      <tbody>
                        {rpcCommands.length === 0 ? (
                          <tr>
                            <td colSpan={8} style={{ textAlign: 'center', padding: '36px', color: 'var(--text-muted)' }}>
                              No RPC commands executed for this device yet.
                            </td>
                          </tr>
                        ) : (
                          rpcCommands.map(cmd => (
                            <tr key={cmd.id}>
                              <td>
                                <span className="badge-tag" style={{
                                  background: cmd.status === 'SUCCESSFUL' ? 'rgba(16, 185, 129, 0.15)' : cmd.status === 'DELIVERED' ? 'rgba(56, 189, 248, 0.15)' : cmd.status === 'QUEUED' ? 'rgba(245, 158, 11, 0.15)' : 'rgba(244, 63, 94, 0.15)',
                                  color: cmd.status === 'SUCCESSFUL' ? 'var(--accent-emerald)' : cmd.status === 'DELIVERED' ? 'var(--accent-cyan)' : cmd.status === 'QUEUED' ? 'var(--accent-amber)' : 'var(--accent-rose)'
                                }}>
                                  {cmd.status}
                                </span>
                              </td>
                              <td style={{ fontWeight: 600, color: '#fff' }}>
                                {cmd.method}
                              </td>
                              <td style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                                {cmd.two_way ? 'Two-Way (Sync)' : 'One-Way (Async)'}
                              </td>
                              <td className="font-mono" style={{ fontSize: '0.75rem', color: 'var(--accent-cyan)', maxWidth: '200px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                                {typeof cmd.params === 'object' ? JSON.stringify(cmd.params) : String(cmd.params)}
                              </td>
                              <td className="font-mono" style={{ fontSize: '0.75rem', color: 'var(--accent-emerald)', maxWidth: '200px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                                {cmd.response ? (typeof cmd.response === 'object' ? JSON.stringify(cmd.response) : String(cmd.response)) : '--'}
                              </td>
                              <td style={{ fontSize: '0.8rem', color: '#94a3b8' }}>
                                {cmd.retries || 0}/{cmd.max_retries || 3}
                              </td>
                              <td style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                                {new Date(cmd.created_time).toLocaleTimeString()}
                              </td>
                              <td style={{ textAlign: 'right' }}>
                                <div style={{ display: 'inline-flex', gap: '6px' }}>
                                  <button 
                                    className="btn btn-secondary btn-sm" 
                                    onClick={() => {
                                      setRpcMethod(cmd.method);
                                      setRpcParams(typeof cmd.params === 'object' ? JSON.stringify(cmd.params) : String(cmd.params));
                                      handleSendRpc(cmd.two_way);
                                    }}
                                    title="Re-send command"
                                  >
                                    <RefreshCw size={12} />
                                  </button>
                                  <button 
                                    className="btn btn-danger btn-sm" 
                                    onClick={() => handleDeleteRpc(cmd.id)}
                                    title="Delete command"
                                  >
                                    <Trash2 size={12} />
                                  </button>
                                </div>
                              </td>
                            </tr>
                          ))
                        )}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}
            </>
          )}

          {/* TAB 5: ALARMS MANAGEMENT */}
          {activeTab === 'alarms' && (
            <AlarmsView
              alarms={alarms}
              onRefresh={loadAlarms}
              onAckAlarm={handleAckAlarm}
              onClearAlarm={handleClearAlarm}
            />
          )}

          {/* TAB 6: SYSTEM HEALTH */}
          {activeTab === 'system' && (
            <>
              <div className="page-header">
                <div>
                  <h2 className="page-title">Platform Supervisor & Telemetry Bus</h2>
                  <p className="page-subtitle">Internal service health, memory allocation, and pipeline guarantees</p>
                </div>
              </div>

              <div className="stats-grid">
                <div className="card">
                  <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '8px' }}>
                    <Server size={20} style={{ color: 'var(--accent-emerald)' }} />
                    <h3 style={{ fontSize: '1rem', fontWeight: 700 }}>Service Supervisor</h3>
                  </div>
                  <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
                    Endpoints: <code style={{ color: 'var(--accent-cyan)' }}>/healthz</code>, <code style={{ color: 'var(--accent-cyan)' }}>/readyz</code>, <code style={{ color: 'var(--accent-cyan)' }}>/metrics</code>
                  </p>
                  <div style={{ marginTop: '12px' }}>
                    <span className="badge-tag live">All Checks Healthy</span>
                  </div>
                </div>

                <div className="card">
                  <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '8px' }}>
                    <ShieldCheck size={20} style={{ color: 'var(--accent-violet)' }} />
                    <h3 style={{ fontSize: '1rem', fontWeight: 700 }}>Multi-Tenant Isolation</h3>
                  </div>
                  <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
                    Row Level Security (RLS) enforcement via <code style={{ color: 'var(--accent-cyan)' }}>app.tenant_id</code> session GUC on PostgreSQL.
                  </p>
                  <div style={{ marginTop: '12px' }}>
                    <span className="badge-tag live">Zero Leakage</span>
                  </div>
                </div>

                <div className="card">
                  <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '8px' }}>
                    <Radio size={20} style={{ color: 'var(--accent-amber)' }} />
                    <h3 style={{ fontSize: '1rem', fontWeight: 700 }}>Event Ordering</h3>
                  </div>
                  <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
                    Strict per-originator-key partitioning ensures timestamps never arrive out-of-order at storage workers.
                  </p>
                  <div style={{ marginTop: '12px' }}>
                    <span className="badge-tag live">FIFO Per Device</span>
                  </div>
                </div>
              </div>
            </>
          )}

          {/* TAB 7: INTEGRATIONS & CONVERTERS */}
          {activeTab === 'integrations' && (
            <IntegrationsView />
          )}

          {/* TAB 8: SCADA PROCESS STUDIO & HMI */}
          {activeTab === 'scada' && (
            <ScadaView />
          )}

          {/* TAB 9: EDGE FLEET */}
          {activeTab === 'edge' && (
            <EdgeView />
          )}

          {/* TAB 10: RULE ENGINE & DAG STUDIO */}
          {activeTab === 'rules' && (
            <RuleEngineView />
          )}

          {/* TAB 11: FIRMWARE & SOFTWARE OTA MANAGEMENT */}
          {activeTab === 'ota' && (
            <OtaView 
              devices={devices} 
              onSelectDevice={(id) => { 
                setSelectedDeviceId(id); 
                setActiveTab('device-detail'); 
              }} 
            />
          )}
        </div>
      </main>

      {/* CREATE DEVICE MODAL */}
      {showCreateModal && (
        <div className="modal-overlay">
          <div className="modal-content">
            <div className="modal-header">
              <h3 className="modal-title">Register New IoT Device</h3>
              <button 
                onClick={() => setShowCreateModal(false)}
                style={{ background: 'transparent', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', fontSize: '1.2rem' }}
              >
                ✕
              </button>
            </div>

            <form onSubmit={handleCreateDevice}>
              {/* Device Profile / Protocol Type Selection */}
              <div style={{ display: 'flex', gap: '8px', marginBottom: '16px' }}>
                <button 
                  type="button"
                  className={`btn btn-sm ${deviceProfileType === 'STANDARD' ? 'btn-primary' : 'btn-secondary'}`}
                  style={{ flex: 1 }}
                  onClick={() => {
                    setDeviceProfileType('STANDARD');
                    if (newDeviceName === 'OPC_UA_Server_172.17.0.3') setNewDeviceName('');
                  }}
                >
                  Standard Edge Sensor
                </button>
                <button 
                  type="button"
                  className={`btn btn-sm ${deviceProfileType === 'OPC_UA' ? 'btn-primary' : 'btn-secondary'}`}
                  style={{ flex: 1 }}
                  onClick={() => {
                    setDeviceProfileType('OPC_UA');
                    if (!newDeviceName) setNewDeviceName('OPC_UA_Server_172.17.0.3');
                    if (!newDeviceLabel) setNewDeviceLabel('OPC UA Docker Server (opc.tcp://172.17.0.3:4840)');
                  }}
                >
                  OPC UA Server (172.17.0.3)
                </button>
              </div>

              {deviceProfileType === 'OPC_UA' && (
                <div className="form-group" style={{ background: 'rgba(6, 182, 212, 0.08)', padding: '12px', borderRadius: '8px', border: '1px solid rgba(6, 182, 212, 0.25)', marginBottom: '14px' }}>
                  <label className="form-label" style={{ color: 'var(--accent-cyan)' }}>OPC UA TCP Server Endpoint *</label>
                  <input 
                    type="text" 
                    className="form-input font-mono" 
                    value={opcuaEndpointInput} 
                    onChange={e => setOpcuaEndpointInput(e.target.value)} 
                    placeholder="opc.tcp://172.17.0.3:4840" 
                    required 
                  />
                  <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                    Connected to Docker binary server port 4840. Ingests Boiler & FeedPump process variables into TimescaleDB.
                  </span>
                </div>
              )}

              {createDeviceError && (
                <div style={{
                  background: 'rgba(244, 63, 94, 0.15)',
                  border: '1px solid rgba(244, 63, 94, 0.3)',
                  color: 'var(--accent-rose)',
                  padding: '10px 14px',
                  borderRadius: 'var(--radius-md)',
                  fontSize: '0.85rem',
                  marginBottom: '14px'
                }}>
                  {createDeviceError}
                </div>
              )}
              <div className="form-group">
                <label className="form-label">Device Name *</label>
                <input 
                  type="text" 
                  className="form-input" 
                  placeholder="e.g. weather-station-01" 
                  value={newDeviceName} 
                  onChange={e => setNewDeviceName(e.target.value)} 
                  required 
                />
              </div>

              <div className="form-group">
                <label className="form-label">Device Label / Description</label>
                <input 
                  type="text" 
                  className="form-input" 
                  placeholder="e.g. Building A Rooftop Sensor" 
                  value={newDeviceLabel} 
                  onChange={e => setNewDeviceLabel(e.target.value)} 
                />
              </div>

              <div className="form-group">
                <label className="form-label">Custom Access Token (Optional)</label>
                <input 
                  type="text" 
                  className="form-input font-mono" 
                  placeholder="Leave blank to auto-generate secure token" 
                  value={newDeviceToken} 
                  onChange={e => setNewDeviceToken(e.target.value)} 
                />
                <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                  This token will be used by the physical device to authenticate over HTTP or MQTT.
                </span>
              </div>

              <div className="modal-footer">
                <button type="button" className="btn btn-secondary" onClick={() => setShowCreateModal(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary" disabled={createLoading}>
                  {createLoading ? <RefreshCw className="animate-spin" size={16} /> : <Check size={16} />}
                  <span>Provision Device</span>
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* CREATE DASHBOARD MODAL */}
      {showCreateDashModal && (
        <div className="modal-overlay">
          <div className="modal-content">
            <div className="modal-header">
              <h3 className="modal-title">Create Realtime Dashboard</h3>
              <button 
                onClick={() => setShowCreateDashModal(false)}
                style={{ background: 'transparent', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', fontSize: '1.2rem' }}
              >
                ✕
              </button>
            </div>

            <form onSubmit={handleCreateDashboard}>
              <div className="form-group">
                <label className="form-label">Dashboard Title *</label>
                <input 
                  type="text" 
                  className="form-input" 
                  placeholder="e.g. Main Plant Operations Canvas" 
                  value={newDashTitle} 
                  onChange={e => setNewDashTitle(e.target.value)} 
                  required 
                />
              </div>

              <div className="modal-footer">
                <button type="button" className="btn btn-secondary" onClick={() => setShowCreateDashModal(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary">
                  <Check size={16} />
                  <span>Create Canvas</span>
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* BULK IMPORT DEVICES MODAL */}
      {showBulkImportModal && (
        <div className="modal-overlay" onClick={() => setShowBulkImportModal(false)}>
          <div className="modal-content" style={{ maxWidth: '580px' }} onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Bulk Import IoT Devices</h3>
              <button 
                onClick={() => setShowBulkImportModal(false)}
                style={{ background: 'transparent', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', fontSize: '1.2rem' }}
              >
                ✕
              </button>
            </div>

            <form onSubmit={handleBulkImportSubmit}>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '14px', padding: '16px 20px' }}>
                <div style={{ display: 'flex', gap: '8px' }}>
                  <button 
                    type="button"
                    className={`btn ${bulkImportFormat === 'csv' ? 'btn-primary' : 'btn-secondary'} btn-sm`}
                    onClick={() => {
                      setBulkImportFormat('csv');
                      setBulkImportText('name,label,access_token\nsensor-01,Warehouse Temp Sensor,TOKEN_WH_01\nsensor-02,Cold Storage Sensor,TOKEN_CS_02\nsensor-03,Rooftop Weather Node,TOKEN_RT_03');
                    }}
                  >
                    CSV Format
                  </button>
                  <button 
                    type="button"
                    className={`btn ${bulkImportFormat === 'json' ? 'btn-primary' : 'btn-secondary'} btn-sm`}
                    onClick={() => {
                      setBulkImportFormat('json');
                      setBulkImportText('[\n  {"name": "sensor-01", "label": "Warehouse Temp Sensor", "access_token": "TOKEN_WH_01"},\n  {"name": "sensor-02", "label": "Cold Storage Sensor", "access_token": "TOKEN_CS_02"}\n]');
                    }}
                  >
                    JSON Format
                  </button>
                </div>

                <div className="form-group" style={{ margin: 0 }}>
                  <label className="form-label">
                    {bulkImportFormat === 'csv' ? 'CSV Content (Header: name,label,access_token)' : 'JSON Array of Devices'}
                  </label>
                  <textarea 
                    className="form-input font-mono" 
                    rows={8}
                    style={{ fontSize: '0.82rem', resize: 'vertical' }}
                    value={bulkImportText} 
                    onChange={e => setBulkImportText(e.target.value)} 
                    placeholder={bulkImportFormat === 'csv' ? 'name,label,access_token\nsensor-01,Storage A,TOKEN_01' : '[{"name":"sensor-01"}]'}
                    required 
                  />
                </div>

                {bulkImportResult && (
                  <div style={{ 
                    padding: '10px 14px', 
                    borderRadius: '8px', 
                    background: bulkImportResult.failed === 0 ? 'rgba(16, 185, 129, 0.15)' : 'rgba(245, 158, 11, 0.15)',
                    border: `1px solid ${bulkImportResult.failed === 0 ? '#10b981' : '#f59e0b'}`,
                    fontSize: '0.82rem'
                  }}>
                    <strong style={{ color: bulkImportResult.failed === 0 ? '#34d399' : '#fbbf24' }}>
                      Import Summary: {bulkImportResult.imported} imported, {bulkImportResult.failed} failed
                    </strong>
                    {bulkImportResult.errors && bulkImportResult.errors.length > 0 && (
                      <ul style={{ margin: '6px 0 0 16px', padding: 0, color: '#f87171' }}>
                        {bulkImportResult.errors.map((e: string, i: number) => <li key={i}>{e}</li>)}
                      </ul>
                    )}
                  </div>
                )}
              </div>

              <div className="modal-footer">
                <button type="button" className="btn btn-secondary" onClick={() => setShowBulkImportModal(false)}>
                  Close
                </button>
                <button type="submit" className="btn btn-primary" disabled={bulkImportLoading || !bulkImportText.trim()}>
                  {bulkImportLoading ? <RefreshCw className="animate-spin" size={16} /> : <Upload size={16} />}
                  <span>Import Devices</span>
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
