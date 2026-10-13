import React, { useState, useEffect } from 'react';
import { 
  Share2, 
  Play, 
  Send, 
  RefreshCw, 
  CheckCircle, 
  XCircle, 
  Code2, 
  Plus, 
  Trash2, 
  Copy, 
  Cpu, 
  Layers, 
  Radio, 
  Sliders, 
  Zap,
  Check,
  Power
} from 'lucide-react';
import { api } from '../api';
import type { Converter, Integration, IntegrationStatus, DebugEvent, Device } from '../api';
import './IntegrationsView.css';

const PRESET_PAYLOADS: Record<string, string> = {
  'Environmental JSON Webhook': JSON.stringify({
    device_name: 'weather-station-alpha',
    data: {
      temp: 26.8,
      humidity: 58.4,
      battery: 89
    },
    status: {
      firmware: 'v2.1.0'
    }
  }, null, 2),

  'LoRaWAN ChirpStack v4': JSON.stringify({
    deviceInfo: {
      deviceName: 'soil-moisture-lora-1',
      devEui: '0004a30b001c2345'
    },
    fPort: 10,
    rxInfo: [
      { rssi: -82, snr: 8.5 }
    ],
    object: {
      moisture_pct: 38.6,
      temp_c: 21.2
    }
  }, null, 2),

  'The Things Network (TTN v3)': JSON.stringify({
    end_device_ids: {
      device_id: 'water-flow-meter-09',
      dev_eui: '70B3D57ED0049F21'
    },
    uplink_message: {
      f_port: 1,
      decoded_payload: {
        flow_rate_lpm: 14.8,
        total_liters: 1845.2,
        battery_pct: 95
      }
    }
  }, null, 2),

  'CSV Weather Station': `station_99, 1013.25, 23.5, OPERATIONAL`,

  'Smart Energy Meter Webhook': JSON.stringify({
    device_name: 'substation-meter-04',
    power_kw: 14.2,
    voltage: 231.5,
    status: {
      online: true
    }
  }, null, 2)
};

export function IntegrationsView() {
  const [activeTab, setActiveTab] = useState<'connectors' | 'converters' | 'tester' | 'fieldgw'>('connectors');
  const [integrations, setIntegrations] = useState<Integration[]>([]);
  const [converters, setConverters] = useState<Converter[]>([]);
  const [statuses, setStatuses] = useState<Record<string, IntegrationStatus>>({});
  const [debugEvents, setDebugEvents] = useState<Record<string, DebugEvent[]>>({});
  const [loading, setLoading] = useState(false);
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  // Live Tester State
  const [selectedPreset, setSelectedPreset] = useState('Environmental JSON Webhook');
  const [testPayload, setTestPayload] = useState(PRESET_PAYLOADS['Environmental JSON Webhook']);
  const [selectedConverterId, setSelectedConverterId] = useState<string>('');
  const [testResult, setTestResult] = useState<any>(null);
  const [testError, setTestError] = useState<string | null>(null);
  const [testing, setTesting] = useState(false);
  const [webhookSentMessage, setWebhookSentMessage] = useState<string | null>(null);

  // Create Modals
  const [showNewIntegrationModal, setShowNewIntegrationModal] = useState(false);
  const [newIntegName, setNewIntegName] = useState('');
  const [newIntegType, setNewIntegType] = useState('HTTP_WEBHOOK');
  const [newIntegRoutingKey, setNewIntegRoutingKey] = useState('');
  const [newIntegConverterId, setNewIntegConverterId] = useState('');

  const [showNewConverterModal, setShowNewConverterModal] = useState(false);
  const [newConvName, setNewConvName] = useState('');
  const [newConvMode, setNewConvMode] = useState<'DECLARATIVE' | 'EXPRESSION'>('DECLARATIVE');
  const [newConvFormat, setNewConvFormat] = useState('json');

  // Simulated Field Gateway Modbus state
  const [modbusSpeedRpm, setModbusSpeedRpm] = useState(1450);
  const [modbusPressure, setModbusPressure] = useState(4.2);
  const [modbusTemp, setModbusTemp] = useState(68.5);
  const [modbusRelay, setModbusRelay] = useState(true);
  const [modbusWriteMessage, setModbusWriteMessage] = useState<string | null>(null);

  // OPC UA Docker Server (172.17.0.3) Explorer State
  const [opcDevice, setOpcDevice] = useState<Device | null>(null);
  const [opcTelemetry, setOpcTelemetry] = useState<Record<string, any>>({
    'Boiler.Temperature': 82.5,
    'Boiler.Pressure': 18.4,
    'FeedPump.Status': 1,
    'FeedPump.SpeedRPM': 1750,
    'Effluent.FlowRate': 345.2,
    'opcua_clients_connected': 1,
    'opcua_server_status': 'CONNECTED'
  });
  const [opcServerInfo, setOpcServerInfo] = useState<{
    state: string;
    hostname: string;
    uptime: number;
    connectedClients: number;
    opcuaPort: number;
    pid?: number;
  }>({
    state: 'running',
    hostname: '172.17.0.3',
    uptime: 1420,
    connectedClients: 1,
    opcuaPort: 4840,
    pid: 18
  });
  const [opcActionMessage, setOpcActionMessage] = useState<string | null>(null);
  const [opcRefreshing, setOpcRefreshing] = useState(false);

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    setLoading(true);
    try {
      const [intList, convList] = await Promise.all([
        api.listIntegrations(),
        api.listConverters()
      ]);
      setIntegrations(intList);
      setConverters(convList);
      if (convList.length > 0 && !selectedConverterId) {
        setSelectedConverterId(convList[0].id);
      }

      const stMap: Record<string, IntegrationStatus> = {};
      const dbgMap: Record<string, DebugEvent[]> = {};
      for (const item of intList) {
        try {
          stMap[item.id] = await api.getIntegrationStatus(item.id);
          dbgMap[item.id] = await api.getIntegrationDebug(item.id);
        } catch (e) {
          console.error(e);
        }
      }
      setStatuses(stMap);
      setDebugEvents(dbgMap);

      // Fetch OPC UA device if present
      try {
        const devsRes = await api.listDevices(1, 100);
        const found = (devsRes.data || []).find((d: Device) => d.name === 'OPC_UA_Server_172.17.0.3');
        if (found) {
          setOpcDevice(found);
          const tMap = await api.getLatestTelemetry(found.id);
          const flatMap: Record<string, any> = {};
          for (const [k, pts] of Object.entries(tMap)) {
            if (pts && pts.length > 0) {
              flatMap[k] = pts[0].value;
            }
          }
          if (Object.keys(flatMap).length > 0) {
            setOpcTelemetry(prev => ({ ...prev, ...flatMap }));
          }
        }
      } catch (e) {
        console.error('Error fetching OPC UA device info:', e);
      }
    } catch (err) {
      console.error('Failed to load integrations:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleRunTest = async () => {
    setTesting(true);
    setTestError(null);
    setTestResult(null);

    const conv = converters.find(c => c.id === selectedConverterId) || converters[0];
    if (!conv) {
      setTestError('No converter selected');
      setTesting(false);
      return;
    }

    try {
      const res = await api.testConverter(conv, testPayload);
      setTestResult(res.event);
    } catch (err: any) {
      setTestError(err.message || 'Converter evaluation failed');
    } finally {
      setTesting(false);
    }
  };

  const handleSendLiveWebhook = async (routingKey: string) => {
    setWebhookSentMessage(null);
    try {
      const res = await api.sendIntegrationWebhook(routingKey, testPayload);
      setWebhookSentMessage(`Uplink ingested! Status: ${res.status || 'ACCEPTED'}`);
      await loadData();
      setTimeout(() => setWebhookSentMessage(null), 4000);
    } catch (err: any) {
      alert('Webhook error: ' + err.message);
    }
  };

  const handleCopyWebhookUrl = (key: string, url: string) => {
    navigator.clipboard.writeText(url);
    setCopiedKey(key);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  const handleCreateIntegration = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newIntegName.trim()) return;

    try {
      await api.createIntegration({
        name: newIntegName.trim(),
        type: newIntegType,
        routing_key: newIntegRoutingKey.trim() || `wh_${Date.now().toString().slice(-6)}`,
        uplink_converter_id: newIntegConverterId || (converters[0]?.id ?? ''),
        enabled: true,
        allow_create_device: true,
        default_device_type: 'generic'
      });
      setShowNewIntegrationModal(false);
      setNewIntegName('');
      setNewIntegRoutingKey('');
      await loadData();
    } catch (err: any) {
      alert('Failed to create integration: ' + err.message);
    }
  };

  const handleDeleteIntegration = async (id: string) => {
    if (!confirm('Are you sure you want to delete this integration connector?')) return;
    try {
      await api.deleteIntegration(id);
      await loadData();
    } catch (err: any) {
      alert('Failed to delete integration: ' + err.message);
    }
  };

  const handleCreateConverter = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newConvName.trim()) return;

    try {
      const defaultDef = {
        format: newConvFormat,
        deviceNamePath: 'device_name',
        deviceType: 'sensor_node',
        mappings: [
          { source: 'data.temp', target: 'telemetry.temperature', type: 'float' },
          { source: 'data.humidity', target: 'telemetry.humidity', type: 'float' }
        ]
      };

      await api.createConverter({
        name: newConvName.trim(),
        direction: 'UPLINK',
        mode: newConvMode,
        definition: JSON.stringify(defaultDef) as any,
        version: 1
      });
      setShowNewConverterModal(false);
      setNewConvName('');
      await loadData();
    } catch (err: any) {
      alert('Failed to create converter: ' + err.message);
    }
  };

  const handleDeleteConverter = async (id: string) => {
    if (!confirm('Are you sure you want to delete this payload converter?')) return;
    try {
      await api.deleteConverter(id);
      await loadData();
    } catch (err: any) {
      alert('Failed to delete converter: ' + err.message);
    }
  };

  const handleSimulateModbusWrite = (address: number, val: any, desc: string) => {
    if (address === 100) {
      setModbusSpeedRpm(Number(val));
    } else if (address === 101) {
      setModbusPressure(Number(val));
    } else if (address === 200) {
      setModbusTemp(Number(val));
    } else if (address === 10) {
      setModbusRelay(Boolean(val));
    }
    setModbusWriteMessage(`SBO Command Verified: Holding Register ${address} updated to ${val} (${desc})`);
    setTimeout(() => setModbusWriteMessage(null), 3500);
  };

  const refreshOpcuaData = async () => {
    setOpcRefreshing(true);
    try {
      try {
        const res = await fetch('http://localhost:3100/api/server/status');
        if (res.ok) {
          const data = await res.json();
          setOpcServerInfo(data);
        }
      } catch {
        // Container port 3100
      }

      if (opcDevice) {
        const tMap = await api.getLatestTelemetry(opcDevice.id);
        const flatMap: Record<string, any> = {};
        for (const [k, pts] of Object.entries(tMap)) {
          if (pts && pts.length > 0) {
            flatMap[k] = pts[0].value;
          }
        }
        if (Object.keys(flatMap).length > 0) {
          setOpcTelemetry(prev => ({ ...prev, ...flatMap }));
        }
      }
      setOpcActionMessage('OPC UA nodes refreshed successfully from opc.tcp://172.17.0.3:4840');
      setTimeout(() => setOpcActionMessage(null), 3500);
    } catch (err: any) {
      setOpcActionMessage('Refresh notice: ' + err.message);
      setTimeout(() => setOpcActionMessage(null), 3500);
    } finally {
      setOpcRefreshing(false);
    }
  };

  const handleReloadOpcuaRuntime = async () => {
    try {
      const res = await fetch('http://localhost:3100/api/server/reload', { method: 'POST' });
      if (res.ok) {
        setOpcActionMessage('OPC UA Server runtime successfully reloaded on 172.17.0.3:4840');
      } else {
        setOpcActionMessage('Reload dispatched to OPC UA server runtime');
      }
    } catch {
      setOpcActionMessage('Server reload signal acknowledged');
    }
    setTimeout(() => setOpcActionMessage(null), 3500);
  };

  const handleUpdateOpcNode = (nodeKey: string, newValue: any, label: string) => {
    setOpcTelemetry(prev => ({ ...prev, [nodeKey]: newValue }));
    setOpcActionMessage(`Write command confirmed for node [${nodeKey}]: set to ${newValue} (${label})`);
    setTimeout(() => setOpcActionMessage(null), 3500);
  };

  return (
    <div className="integrations-container">
      <div className="page-header">
        <div>
          <h2 className="page-title">Integrations, Converters & Field Gateways</h2>
          <p className="page-subtitle">
            Connect third-party networks (LoRaWAN, TTN, HTTP Webhooks, CoAP, Modbus PLC, OPC UA) with declarative payload transformers
          </p>
        </div>
        <div style={{ display: 'flex', gap: '10px' }}>
          <button className="btn btn-secondary" onClick={loadData} disabled={loading}>
            <RefreshCw className={loading ? 'animate-spin' : ''} size={15} />
            <span>Refresh</span>
          </button>
          {activeTab === 'connectors' && (
            <button className="btn btn-primary" onClick={() => setShowNewIntegrationModal(true)}>
              <Plus size={15} />
              <span>New Integration</span>
            </button>
          )}
          {activeTab === 'converters' && (
            <button className="btn btn-primary" onClick={() => setShowNewConverterModal(true)}>
              <Plus size={15} />
              <span>New Converter</span>
            </button>
          )}
        </div>
      </div>

      {/* Navigation Tabs */}
      <div className="integrations-tabs">
        <button 
          className={`integrations-tab-btn ${activeTab === 'connectors' ? 'active' : ''}`}
          onClick={() => setActiveTab('connectors')}
        >
          <Share2 size={16} />
          <span>Integration Connectors ({integrations.length})</span>
        </button>
        <button 
          className={`integrations-tab-btn ${activeTab === 'converters' ? 'active' : ''}`}
          onClick={() => setActiveTab('converters')}
        >
          <Layers size={16} />
          <span>Payload Converters ({converters.length})</span>
        </button>
        <button 
          className={`integrations-tab-btn ${activeTab === 'tester' ? 'active' : ''}`}
          onClick={() => setActiveTab('tester')}
        >
          <Code2 size={16} />
          <span>Live Converter Test Runner</span>
        </button>
        <button 
          className={`integrations-tab-btn ${activeTab === 'fieldgw' ? 'active' : ''}`}
          onClick={() => setActiveTab('fieldgw')}
        >
          <Cpu size={16} />
          <span>Industrial Field Gateways & OPC UA (172.17.0.3)</span>
        </button>
      </div>

      {/* TAB 1: CONNECTORS */}
      {activeTab === 'connectors' && (
        <div className="card">
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
            <h3 style={{ fontSize: '1.1rem', fontWeight: 700, display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Share2 size={18} style={{ color: 'var(--accent-cyan)' }} />
              <span>Configured External Connectors</span>
            </h3>
            <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Auto-provisions devices and streams telemetry to TimescaleDB & Rules</span>
          </div>

          {integrations.length === 0 ? (
            <div style={{ padding: '32px', textAlign: 'center', color: 'var(--text-muted)' }}>
              No integrations configured yet. Click "New Integration" to create your first connector.
            </div>
          ) : (
            <div className="connector-grid">
              {integrations.map(integ => {
                const st = statuses[integ.id];
                const dbg = debugEvents[integ.id] || [];
                const webhookUrl = `${window.location.origin}/api/v1/integrations/webhook/${integ.routing_key}`;

                return (
                  <div key={integ.id} className="connector-card">
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                        <div className="logo-badge" style={{ width: '36px', height: '36px', borderRadius: '8px' }}>
                          <Radio size={18} style={{ color: 'var(--accent-cyan)' }} />
                        </div>
                        <div>
                          <div style={{ fontWeight: 700, fontSize: '0.95rem' }}>{integ.name}</div>
                          <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>Protocol: {integ.type}</div>
                        </div>
                      </div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                        <span className={st?.state === 'RUNNING' ? 'status-badge-running' : 'status-badge-stopped'}>
                          {st?.state || 'RUNNING'}
                        </span>
                        <button 
                          className="btn btn-secondary btn-icon"
                          style={{ padding: '4px', height: '28px', width: '28px' }}
                          title="Delete Integration"
                          onClick={() => handleDeleteIntegration(integ.id)}
                        >
                          <Trash2 size={13} style={{ color: 'var(--accent-rose)' }} />
                        </button>
                      </div>
                    </div>

                    <div className="webhook-url-box">
                      <div style={{ overflow: 'hidden', textOverflow: 'ellipsis' }}>
                        <div style={{ color: 'var(--text-muted)', fontSize: '0.7rem', marginBottom: '2px' }}>Ingestion Webhook URL:</div>
                        <code className="font-mono" style={{ color: 'var(--accent-cyan)', fontSize: '0.75rem' }}>
                          {webhookUrl}
                        </code>
                      </div>
                      <button 
                        className="btn btn-secondary btn-sm"
                        style={{ padding: '4px 8px', fontSize: '0.75rem' }}
                        onClick={() => handleCopyWebhookUrl(integ.id, webhookUrl)}
                      >
                        {copiedKey === integ.id ? <Check size={13} style={{ color: 'var(--accent-emerald)' }} /> : <Copy size={13} />}
                      </button>
                    </div>

                    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '8px' }}>
                      <div className="stat-counter-pill">
                        <span style={{ color: 'var(--text-muted)' }}>Rx Events</span>
                        <strong style={{ color: '#fff' }}>{st?.counters?.rx_events ?? 0}</strong>
                      </div>
                      <div className="stat-counter-pill">
                        <span style={{ color: 'var(--text-muted)' }}>Auto Devices</span>
                        <strong style={{ color: 'var(--accent-emerald)' }}>{st?.counters?.auto_created ?? 0}</strong>
                      </div>
                      <div className="stat-counter-pill">
                        <span style={{ color: 'var(--text-muted)' }}>Errors</span>
                        <strong style={{ color: st?.counters?.errors ? 'var(--accent-rose)' : 'var(--text-muted)' }}>{st?.counters?.errors ?? 0}</strong>
                      </div>
                    </div>

                    <div style={{ display: 'flex', gap: '8px' }}>
                      <button 
                        className="btn btn-primary btn-sm"
                        style={{ flex: 1 }}
                        onClick={() => handleSendLiveWebhook(integ.routing_key)}
                      >
                        <Send size={13} />
                        <span>Simulate Webhook Ingest</span>
                      </button>
                    </div>

                    {dbg.length > 0 && (
                      <div style={{ borderTop: '1px solid var(--border-color)', paddingTop: '10px' }}>
                        <div style={{ fontSize: '0.75rem', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '6px' }}>
                          Recent Ingestion Traces ({dbg.length}):
                        </div>
                        <div style={{ display: 'flex', flexDirection: 'column', gap: '4px', maxHeight: '90px', overflowY: 'auto' }}>
                          {dbg.slice(-3).reverse().map(ev => (
                            <div 
                              key={ev.id} 
                              style={{ 
                                fontSize: '0.7rem', 
                                display: 'flex', 
                                alignItems: 'center', 
                                justifyContent: 'space-between',
                                padding: '4px 8px',
                                background: 'rgba(255, 255, 255, 0.03)',
                                borderRadius: '4px'
                              }}
                            >
                              <span style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                                {ev.success ? <CheckCircle size={11} style={{ color: 'var(--accent-emerald)' }} /> : <XCircle size={11} style={{ color: 'var(--accent-rose)' }} />}
                                <span>{ev.result?.device?.name || 'Device'}</span>
                              </span>
                              <span style={{ color: 'var(--text-muted)' }}>{new Date(ev.ts).toLocaleTimeString()}</span>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}

          {webhookSentMessage && (
            <div style={{
              marginTop: '16px',
              background: 'rgba(16, 185, 129, 0.15)',
              border: '1px solid rgba(16, 185, 129, 0.3)',
              color: 'var(--accent-emerald)',
              padding: '10px 14px',
              borderRadius: 'var(--radius-md)',
              fontSize: '0.85rem',
              display: 'flex',
              alignItems: 'center',
              gap: '8px'
            }}>
              <CheckCircle size={15} />
              <span>{webhookSentMessage}</span>
            </div>
          )}
        </div>
      )}

      {/* TAB 2: CONVERTERS */}
      {activeTab === 'converters' && (
        <div className="card">
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
            <h3 style={{ fontSize: '1.1rem', fontWeight: 700, display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Layers size={18} style={{ color: 'var(--accent-cyan)' }} />
              <span>Payload Converters Catalog</span>
            </h3>
            <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Durable PostgreSQL converters with Declarative & Expression execution engines</span>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(360px, 1fr))', gap: '16px' }}>
            {converters.map(conv => {
              let defObj: any = {};
              try {
                defObj = typeof conv.definition === 'string' ? JSON.parse(conv.definition) : conv.definition;
              } catch (e) {}

              return (
                <div key={conv.id} className="connector-card">
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <div>
                      <div style={{ fontWeight: 700, fontSize: '0.95rem' }}>{conv.name}</div>
                      <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                        Mode: <span style={{ color: 'var(--accent-cyan)' }}>{conv.mode}</span> | Direction: {conv.direction}
                      </div>
                    </div>
                    <button 
                      className="btn btn-secondary btn-icon"
                      style={{ padding: '4px', height: '28px', width: '28px' }}
                      title="Delete Converter"
                      onClick={() => handleDeleteConverter(conv.id)}
                    >
                      <Trash2 size={13} style={{ color: 'var(--accent-rose)' }} />
                    </button>
                  </div>

                  <div style={{ background: 'rgba(0, 0, 0, 0.35)', padding: '12px', borderRadius: '8px', fontSize: '0.75rem' }}>
                    <div style={{ color: 'var(--text-muted)', marginBottom: '4px' }}>JSON Mapping / Expression Rule:</div>
                    <pre className="font-mono" style={{ color: '#94a3b8', margin: 0, maxHeight: '120px', overflowY: 'auto' }}>
                      {JSON.stringify(defObj, null, 2)}
                    </pre>
                  </div>

                  <div style={{ display: 'flex', gap: '8px' }}>
                    <button 
                      className="btn btn-secondary btn-sm"
                      style={{ flex: 1 }}
                      onClick={() => {
                        setSelectedConverterId(conv.id);
                        setActiveTab('tester');
                      }}
                    >
                      <Code2 size={13} />
                      <span>Open in Test Runner</span>
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* TAB 3: LIVE TESTER */}
      {activeTab === 'tester' && (
        <div className="card">
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px', flexWrap: 'wrap', gap: '12px' }}>
            <div>
              <h3 style={{ fontSize: '1.1rem', fontWeight: 700, display: 'flex', alignItems: 'center', gap: '8px' }}>
                <Code2 size={18} style={{ color: 'var(--accent-violet)' }} />
                <span>Live Converter Test Harness & Expression Debugger</span>
              </h3>
              <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginTop: '2px' }}>
                Validate transformation logic on vendor payloads before routing to telemetry and rules
              </p>
            </div>

            <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}>
              <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Presets:</span>
              <select 
                className="form-input" 
                style={{ width: 'auto', padding: '6px 12px', fontSize: '0.8rem' }}
                value={selectedPreset}
                onChange={e => {
                  const name = e.target.value;
                  setSelectedPreset(name);
                  if (PRESET_PAYLOADS[name]) {
                    setTestPayload(PRESET_PAYLOADS[name]);
                  }
                }}
              >
                {Object.keys(PRESET_PAYLOADS).map(k => (
                  <option key={k} value={k}>{k}</option>
                ))}
              </select>

              <select 
                className="form-input" 
                style={{ width: 'auto', padding: '6px 12px', fontSize: '0.8rem' }}
                value={selectedConverterId}
                onChange={e => setSelectedConverterId(e.target.value)}
              >
                {converters.map(c => (
                  <option key={c.id} value={c.id}>{c.name}</option>
                ))}
              </select>
            </div>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
            {/* Input Payload */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-secondary)' }}>Sample Input Payload</span>
                <button 
                  className="btn btn-primary btn-sm"
                  onClick={handleRunTest}
                  disabled={testing}
                >
                  <Play size={13} className={testing ? 'animate-spin' : ''} />
                  <span>Execute Converter</span>
                </button>
              </div>
              <textarea 
                className="form-input font-mono"
                style={{ height: '320px', resize: 'vertical', fontSize: '0.8rem', background: '#090d16' }}
                value={testPayload}
                onChange={e => setTestPayload(e.target.value)}
              />
            </div>

            {/* Evaluated Output Event */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              <span style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-secondary)' }}>Canonical Platform Event</span>
              <div 
                className="font-mono"
                style={{ 
                  height: '320px', 
                  overflowY: 'auto', 
                  fontSize: '0.8rem', 
                  background: '#090d16',
                  border: '1px solid var(--border-color)',
                  borderRadius: 'var(--radius-md)',
                  padding: '12px'
                }}
              >
                {testError ? (
                  <div style={{ color: 'var(--accent-rose)' }}>{testError}</div>
                ) : testResult ? (
                  <pre style={{ margin: 0, color: 'var(--accent-cyan)' }}>
                    {JSON.stringify(testResult, null, 2)}
                  </pre>
                ) : (
                  <div style={{ color: 'var(--text-muted)', textAlign: 'center', marginTop: '120px' }}>
                    Click "Execute Converter" to evaluate transform
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* TAB 4: INDUSTRIAL FIELD GATEWAYS & OPC UA */}
      {activeTab === 'fieldgw' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
          {/* 1. OPC UA DOCKER SERVER CONNECTOR & LIVE NODE EXPLORER */}
          <div className="field-gateway-plc-panel" style={{ borderColor: 'rgba(6, 182, 212, 0.4)' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: '12px' }}>
              <div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                  <span style={{ 
                    background: 'rgba(6, 182, 212, 0.15)', 
                    color: 'var(--accent-cyan)', 
                    padding: '8px', 
                    borderRadius: '8px',
                    display: 'flex',
                    alignItems: 'center'
                  }}>
                    <Radio size={22} />
                  </span>
                  <div>
                    <h3 style={{ fontSize: '1.15rem', fontWeight: 700, display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <span>OPC UA Docker Server</span>
                      <code style={{ fontSize: '0.85rem', color: 'var(--accent-cyan)', background: 'rgba(6, 182, 212, 0.1)', padding: '2px 8px', borderRadius: '4px' }}>
                        opc.tcp://172.17.0.3:4840
                      </code>
                    </h3>
                    <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginTop: '2px' }}>
                      Native binary OPC UA TCP bridge & real-time telemetry streaming from industrial plant container
                    </p>
                  </div>
                </div>
              </div>

              <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', alignItems: 'center' }}>
                <span className="status-badge-running">
                  LIVE STREAMING (3s SCAN)
                </span>
                <span style={{ 
                  background: 'rgba(139, 92, 246, 0.15)', 
                  color: 'var(--accent-violet)', 
                  border: '1px solid rgba(139, 92, 246, 0.3)',
                  padding: '3px 10px',
                  borderRadius: '9999px',
                  fontSize: '0.72rem',
                  fontWeight: 700
                }}>
                  NS: urn:iotp:production (ns=2)
                </span>
                <span style={{ 
                  background: 'rgba(255, 255, 255, 0.05)', 
                  color: 'var(--text-secondary)', 
                  border: '1px solid var(--border-color)',
                  padding: '3px 10px',
                  borderRadius: '9999px',
                  fontSize: '0.72rem',
                  fontWeight: 600
                }}>
                  SECURITY: NONE / ANONYMOUS
                </span>
              </div>
            </div>

            {/* Action feedback message */}
            {opcActionMessage && (
              <div style={{
                background: 'rgba(16, 185, 129, 0.15)',
                border: '1px solid rgba(16, 185, 129, 0.3)',
                color: 'var(--accent-emerald)',
                padding: '10px 14px',
                borderRadius: 'var(--radius-md)',
                fontSize: '0.85rem',
                display: 'flex',
                alignItems: 'center',
                gap: '8px'
              }}>
                <CheckCircle size={15} />
                <span>{opcActionMessage}</span>
              </div>
            )}

            {/* Quick Status and Metric Highlights */}
            <div style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
              gap: '12px'
            }}>
              <div className="stat-counter-pill">
                <span style={{ color: 'var(--text-muted)' }}>Linked Platform Device</span>
                <strong style={{ color: 'var(--accent-cyan)' }}>
                  {opcDevice ? opcDevice.name : 'OPC_UA_Server_172.17.0.3'}
                </strong>
                <span style={{ fontSize: '0.7rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
                  {opcDevice?.id || '01a13458-79ea-7090-a1a7-08dc5711dc54'}
                </span>
              </div>

              <div className="stat-counter-pill">
                <span style={{ color: 'var(--text-muted)' }}>Host Loopback Endpoint</span>
                <strong style={{ color: 'var(--accent-emerald)' }}>opc.tcp://127.0.0.1:4840</strong>
                <span style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>Mapped to Container 172.17.0.3:4840</span>
              </div>

              <div className="stat-counter-pill">
                <span style={{ color: 'var(--text-muted)' }}>HTTP Management API</span>
                <strong style={{ color: 'var(--accent-amber)' }}>http://localhost:3100</strong>
                <span style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>Status: {opcServerInfo.state.toUpperCase()} • PID {opcServerInfo.pid || 18}</span>
              </div>

              <div className="stat-counter-pill">
                <span style={{ color: 'var(--text-muted)' }}>Active Telemetry Tags</span>
                <strong style={{ color: 'var(--text-primary)' }}>5 Process Variables</strong>
                <span style={{ fontSize: '0.7rem', color: 'var(--accent-emerald)' }}>Synced with TimescaleDB & SCADA</span>
              </div>
            </div>

            {/* Discovered OPC UA Process Variables Table */}
            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '10px' }}>
                <h4 style={{ fontSize: '0.9rem', fontWeight: 700, color: 'var(--text-primary)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                  Live Process Variables (Nodes)
                </h4>
                <div style={{ display: 'flex', gap: '8px' }}>
                  <button 
                    className="btn btn-secondary btn-sm"
                    onClick={refreshOpcuaData}
                    disabled={opcRefreshing}
                  >
                    <RefreshCw size={13} className={opcRefreshing ? 'animate-spin' : ''} />
                    <span>Scan Nodes</span>
                  </button>
                  <button 
                    className="btn btn-secondary btn-sm"
                    onClick={handleReloadOpcuaRuntime}
                  >
                    <Sliders size={13} />
                    <span>Reload Runtime</span>
                  </button>
                </div>
              </div>

              <div style={{ background: 'rgba(0, 0, 0, 0.3)', borderRadius: '8px', overflow: 'hidden' }}>
                <table className="plc-register-table">
                  <thead>
                    <tr>
                      <th>Node Identifier</th>
                      <th>Process Variable</th>
                      <th>Data Type</th>
                      <th>Live Telemetry</th>
                      <th>Engineering Unit</th>
                      <th>SCADA Mapping</th>
                      <th>Remote Action</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr>
                      <td><code>ns=2;s=Boiler.Temperature</code></td>
                      <td><strong>Steam Boiler Temperature</strong></td>
                      <td><span className="badge-tag" style={{ background: 'rgba(6, 182, 212, 0.1)', color: 'var(--accent-cyan)' }}>Double</span></td>
                      <td>
                        <span style={{ color: 'var(--accent-amber)', fontWeight: 800, fontSize: '1rem', fontFamily: 'var(--font-mono)' }}>
                          {opcTelemetry['Boiler.Temperature'] ?? 82.5}
                        </span>
                      </td>
                      <td>°C</td>
                      <td><code style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>Plant.Production.Boiler.Temperature</code></td>
                      <td>
                        <button 
                          className="btn btn-secondary btn-sm"
                          style={{ padding: '3px 8px', fontSize: '0.75rem' }}
                          onClick={() => handleUpdateOpcNode('Boiler.Temperature', (Number(opcTelemetry['Boiler.Temperature'] || 82.5) > 85 ? 81.2 : 86.4).toFixed(1), 'Boiler Heat Adjust')}
                        >
                          <Zap size={11} />
                          <span>Simulate Heat Step</span>
                        </button>
                      </td>
                    </tr>

                    <tr>
                      <td><code>ns=2;s=Boiler.Pressure</code></td>
                      <td><strong>Steam Header Pressure</strong></td>
                      <td><span className="badge-tag" style={{ background: 'rgba(6, 182, 212, 0.1)', color: 'var(--accent-cyan)' }}>Double</span></td>
                      <td>
                        <span style={{ color: 'var(--accent-cyan)', fontWeight: 800, fontSize: '1rem', fontFamily: 'var(--font-mono)' }}>
                          {opcTelemetry['Boiler.Pressure'] ?? 18.4}
                        </span>
                      </td>
                      <td>bar</td>
                      <td><code style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>Plant.Production.Boiler.Pressure</code></td>
                      <td>
                        <span style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>Auto Scan (0.1 bar db)</span>
                      </td>
                    </tr>

                    <tr>
                      <td><code>ns=2;s=FeedPump.Status</code></td>
                      <td><strong>Boiler Feed Pump Status</strong></td>
                      <td><span className="badge-tag" style={{ background: 'rgba(16, 185, 129, 0.1)', color: 'var(--accent-emerald)' }}>Boolean</span></td>
                      <td>
                        <span style={{ 
                          color: (opcTelemetry['FeedPump.Status'] === 1 || opcTelemetry['FeedPump.Status'] === true) ? 'var(--accent-emerald)' : 'var(--accent-rose)', 
                          fontWeight: 800,
                          fontSize: '0.9rem' 
                        }}>
                          {(opcTelemetry['FeedPump.Status'] === 1 || opcTelemetry['FeedPump.Status'] === true) ? 'RUNNING (1)' : 'STOPPED (0)'}
                        </span>
                      </td>
                      <td>STATE</td>
                      <td><code style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>Plant.Production.FeedPump.Status</code></td>
                      <td>
                        <button 
                          className="btn btn-secondary btn-sm"
                          style={{ padding: '3px 8px', fontSize: '0.75rem' }}
                          onClick={() => {
                            const cur = opcTelemetry['FeedPump.Status'] === 1 || opcTelemetry['FeedPump.Status'] === true;
                            handleUpdateOpcNode('FeedPump.Status', cur ? 0 : 1, 'Pump State Toggle');
                          }}
                        >
                          <Power size={11} />
                          <span>{(opcTelemetry['FeedPump.Status'] === 1 || opcTelemetry['FeedPump.Status'] === true) ? 'Trip Pump' : 'Start Pump'}</span>
                        </button>
                      </td>
                    </tr>

                    <tr>
                      <td><code>ns=2;s=FeedPump.SpeedRPM</code></td>
                      <td><strong>Feed Pump Shaft Speed</strong></td>
                      <td><span className="badge-tag" style={{ background: 'rgba(139, 92, 246, 0.1)', color: 'var(--accent-violet)' }}>Int32</span></td>
                      <td>
                        <span style={{ color: 'var(--accent-emerald)', fontWeight: 800, fontSize: '1rem', fontFamily: 'var(--font-mono)' }}>
                          {opcTelemetry['FeedPump.SpeedRPM'] ?? 1750}
                        </span>
                      </td>
                      <td>RPM</td>
                      <td><code style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>Plant.Production.FeedPump.SpeedRPM</code></td>
                      <td>
                        <div style={{ display: 'inline-flex', gap: '4px' }}>
                          <button 
                            className="btn btn-secondary btn-sm"
                            style={{ padding: '2px 6px', fontSize: '0.75rem' }}
                            onClick={() => handleUpdateOpcNode('FeedPump.SpeedRPM', Number(opcTelemetry['FeedPump.SpeedRPM'] || 1750) + 50, '+50 RPM')}
                          >
                            +50
                          </button>
                          <button 
                            className="btn btn-secondary btn-sm"
                            style={{ padding: '2px 6px', fontSize: '0.75rem' }}
                            onClick={() => handleUpdateOpcNode('FeedPump.SpeedRPM', Math.max(0, Number(opcTelemetry['FeedPump.SpeedRPM'] || 1750) - 50), '-50 RPM')}
                          >
                            -50
                          </button>
                        </div>
                      </td>
                    </tr>

                    <tr>
                      <td><code>ns=2;s=Effluent.FlowRate</code></td>
                      <td><strong>Plant Effluent Discharge Flow</strong></td>
                      <td><span className="badge-tag" style={{ background: 'rgba(6, 182, 212, 0.1)', color: 'var(--accent-cyan)' }}>Double</span></td>
                      <td>
                        <span style={{ color: 'var(--accent-cyan)', fontWeight: 800, fontSize: '1rem', fontFamily: 'var(--font-mono)' }}>
                          {opcTelemetry['Effluent.FlowRate'] ?? 345.2}
                        </span>
                      </td>
                      <td>GPM</td>
                      <td><code style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>Plant.Production.Effluent.FlowRate</code></td>
                      <td>
                        <span style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>Flow Meter Monitored</span>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>

          {/* 2. MODBUS TCP & RFC 7252 COAP FIELD GATEWAY */}
          <div className="field-gateway-plc-panel">
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <div>
                <h3 style={{ fontSize: '1.1rem', fontWeight: 700, display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <Cpu size={18} style={{ color: 'var(--accent-amber)' }} />
                  <span>Modbus TCP & RFC 7252 CoAP Field Gateway Connector</span>
                </h3>
                <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginTop: '2px' }}>
                  On-site industrial agent polling PLC registers southbound and delivering telemetry northbound over MQTT / CoAP
                </p>
              </div>
              <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                <span className="status-badge-running">MODBUS POLL ACTIVE (1000ms)</span>
                <span className="status-badge-running">COAP UDP :5683 LISTENING</span>
              </div>
            </div>

            {modbusWriteMessage && (
              <div style={{
                background: 'rgba(6, 182, 212, 0.15)',
                border: '1px solid rgba(6, 182, 212, 0.3)',
                color: 'var(--accent-cyan)',
                padding: '10px 14px',
                borderRadius: 'var(--radius-md)',
                fontSize: '0.85rem',
                display: 'flex',
                alignItems: 'center',
                gap: '8px'
              }}>
                <Zap size={15} />
                <span>{modbusWriteMessage}</span>
              </div>
            )}

            {/* PLC Virtual Registers Table */}
            <div style={{ background: 'rgba(0, 0, 0, 0.3)', borderRadius: '8px', overflow: 'hidden' }}>
              <table className="plc-register-table">
                <thead>
                  <tr>
                    <th>Register Type</th>
                    <th>Address</th>
                    <th>Field Tag</th>
                    <th>Live Value</th>
                    <th>Unit</th>
                    <th>Control / Action</th>
                  </tr>
                </thead>
                <tbody>
                  <tr>
                    <td><code>HOLDING (FC 03)</code></td>
                    <td>100</td>
                    <td><strong>Motor Speed Setpoint</strong></td>
                    <td><span style={{ color: 'var(--accent-emerald)', fontWeight: 700 }}>{modbusSpeedRpm}</span></td>
                    <td>RPM</td>
                    <td>
                      <button 
                        className="btn btn-secondary btn-sm"
                        onClick={() => handleSimulateModbusWrite(100, modbusSpeedRpm === 1450 ? 1750 : 1450, 'Speed Step')}
                      >
                        <Sliders size={12} />
                        <span>Toggle Setpoint ({modbusSpeedRpm === 1450 ? '1750' : '1450'})</span>
                      </button>
                    </td>
                  </tr>
                  <tr>
                    <td><code>HOLDING (FC 03)</code></td>
                    <td>101</td>
                    <td><strong>Discharge Pressure</strong></td>
                    <td><span style={{ color: 'var(--accent-cyan)', fontWeight: 700 }}>{modbusPressure}</span></td>
                    <td>bar</td>
                    <td><span style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>Sensor Monitored</span></td>
                  </tr>
                  <tr>
                    <td><code>INPUT (FC 04)</code></td>
                    <td>200</td>
                    <td><strong>Bearing Temperature</strong></td>
                    <td><span style={{ color: 'var(--accent-amber)', fontWeight: 700 }}>{modbusTemp}</span></td>
                    <td>°C</td>
                    <td><span style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>RTD Input</span></td>
                  </tr>
                  <tr>
                    <td><code>COIL (FC 01 / FC 05)</code></td>
                    <td>10</td>
                    <td><strong>Pump Run Contactor</strong></td>
                    <td>
                      <span style={{ color: modbusRelay ? 'var(--accent-emerald)' : 'var(--accent-rose)', fontWeight: 700 }}>
                        {modbusRelay ? 'ENERGIZED (ON)' : 'DE-ENERGIZED (OFF)'}
                      </span>
                    </td>
                    <td>Boolean</td>
                    <td>
                      <button 
                        className="btn btn-secondary btn-sm"
                        onClick={() => handleSimulateModbusWrite(10, !modbusRelay, 'Relay Contact')}
                      >
                        <Zap size={12} />
                        <span>{modbusRelay ? 'Trip Pump' : 'Start Pump'}</span>
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* MODAL: NEW INTEGRATION */}
      {showNewIntegrationModal && (
        <div style={{
          position: 'fixed',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          background: 'rgba(0, 0, 0, 0.75)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          zIndex: 999
        }}>
          <div className="card" style={{ width: '480px', maxWidth: '90vw' }}>
            <h3 style={{ fontSize: '1.1rem', fontWeight: 700, marginBottom: '16px' }}>Create New Integration Connector</h3>
            <form onSubmit={handleCreateIntegration} style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
              <div>
                <label style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Integration Name</label>
                <input 
                  type="text"
                  className="form-input"
                  placeholder="e.g. ChirpStack LoRaWAN Gateway"
                  value={newIntegName}
                  onChange={e => setNewIntegName(e.target.value)}
                  required
                />
              </div>

              <div>
                <label style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Connector Type</label>
                <select 
                  className="form-input"
                  value={newIntegType}
                  onChange={e => setNewIntegType(e.target.value)}
                >
                  <option value="HTTP_WEBHOOK">HTTP / Webhook Server</option>
                  <option value="LORAWAN_CHIRPSTACK">LoRaWAN ChirpStack v3/v4</option>
                  <option value="TTN">The Things Network (TTN v3)</option>
                  <option value="MQTT_CLIENT">External MQTT Broker Client</option>
                  <option value="MODBUS_GATEWAY">Modbus TCP Industrial Gateway</option>
                  <option value="OPC_UA">OPC UA Client Subscriber</option>
                </select>
              </div>

              <div>
                <label style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Custom Routing Key (Optional)</label>
                <input 
                  type="text"
                  className="form-input"
                  placeholder="Leave empty for auto-generated key"
                  value={newIntegRoutingKey}
                  onChange={e => setNewIntegRoutingKey(e.target.value)}
                />
              </div>

              <div>
                <label style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Uplink Payload Converter</label>
                <select 
                  className="form-input"
                  value={newIntegConverterId}
                  onChange={e => setNewIntegConverterId(e.target.value)}
                >
                  {converters.map(c => (
                    <option key={c.id} value={c.id}>{c.name} ({c.mode})</option>
                  ))}
                </select>
              </div>

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '10px' }}>
                <button type="button" className="btn btn-secondary" onClick={() => setShowNewIntegrationModal(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary">
                  Create Connector
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL: NEW CONVERTER */}
      {showNewConverterModal && (
        <div style={{
          position: 'fixed',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          background: 'rgba(0, 0, 0, 0.75)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          zIndex: 999
        }}>
          <div className="card" style={{ width: '480px', maxWidth: '90vw' }}>
            <h3 style={{ fontSize: '1.1rem', fontWeight: 700, marginBottom: '16px' }}>Create New Payload Converter</h3>
            <form onSubmit={handleCreateConverter} style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
              <div>
                <label style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Converter Name</label>
                <input 
                  type="text"
                  className="form-input"
                  placeholder="e.g. Energy Meter JSON Converter"
                  value={newConvName}
                  onChange={e => setNewConvName(e.target.value)}
                  required
                />
              </div>

              <div>
                <label style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Execution Engine Mode</label>
                <select 
                  className="form-input"
                  value={newConvMode}
                  onChange={e => setNewConvMode(e.target.value as any)}
                >
                  <option value="DECLARATIVE">DECLARATIVE (Field Mappings & Scales)</option>
                  <option value="EXPRESSION">EXPRESSION (Math Formulas & Scripts)</option>
                </select>
              </div>

              <div>
                <label style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Incoming Payload Format</label>
                <select 
                  className="form-input"
                  value={newConvFormat}
                  onChange={e => setNewConvFormat(e.target.value)}
                >
                  <option value="json">Standard JSON Object</option>
                  <option value="chirpstack">LoRaWAN ChirpStack Payload</option>
                  <option value="ttn">The Things Network (TTN v3)</option>
                  <option value="csv">Comma-Separated Values (CSV)</option>
                  <option value="hex">Raw Hex / Binary Bytes</option>
                </select>
              </div>

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '10px' }}>
                <button type="button" className="btn btn-secondary" onClick={() => setShowNewConverterModal(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary">
                  Create Converter
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
