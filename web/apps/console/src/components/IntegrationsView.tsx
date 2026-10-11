import { useState, useEffect } from 'react';
import { 
  Share2, 
  Play, 
  Send, 
  RefreshCw, 
  CheckCircle, 
  XCircle, 
  Terminal, 
  Code2, 
  Sparkles
} from 'lucide-react';
import { api } from '../api';
import type { Converter, Integration, IntegrationStatus, DebugEvent } from '../api';

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
  const [integrations, setIntegrations] = useState<Integration[]>([]);
  const [converters, setConverters] = useState<Converter[]>([]);
  const [statuses, setStatuses] = useState<Record<string, IntegrationStatus>>({});
  const [debugEvents, setDebugEvents] = useState<Record<string, DebugEvent[]>>({});
  const [loading, setLoading] = useState(false);

  // Live Tester State
  const [selectedPreset, setSelectedPreset] = useState('Environmental JSON Webhook');
  const [testPayload, setTestPayload] = useState(PRESET_PAYLOADS['Environmental JSON Webhook']);
  const [selectedConverterId, setSelectedConverterId] = useState<string>('');
  const [testResult, setTestResult] = useState<any>(null);
  const [testError, setTestError] = useState<string | null>(null);
  const [testing, setTesting] = useState(false);
  const [webhookSentMessage, setWebhookSentMessage] = useState<string | null>(null);

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

      // Fetch statuses and debug events for integrations
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
      setWebhookSentMessage(`Uplink ingested! Status: ${res.status || 'OK'}`);
      await loadData();
      setTimeout(() => setWebhookSentMessage(null), 4000);
    } catch (err: any) {
      alert('Webhook error: ' + err.message);
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      <div className="page-header">
        <div>
          <h2 className="page-title">Integrations & Payload Converters</h2>
          <p className="page-subtitle">Third-party networks (LoRaWAN ChirpStack, TTN, HTTP Webhooks, OPC UA) with declarative transforms</p>
        </div>
        <button className="btn btn-secondary" onClick={loadData} disabled={loading}>
          <RefreshCw className={loading ? 'animate-spin' : ''} size={15} />
          <span>Refresh Integrations</span>
        </button>
      </div>

      {/* Active Integrations Catalog */}
      <div className="card">
        <h3 style={{ fontSize: '1.1rem', fontWeight: 700, marginBottom: '16px', display: 'flex', alignItems: 'center', gap: '8px' }}>
          <Share2 size={18} style={{ color: 'var(--accent-cyan)' }} />
          <span>Configured Integration Connectors</span>
        </h3>

        {integrations.length === 0 ? (
          <div style={{ padding: '24px', textAlign: 'center', color: 'var(--text-muted)' }}>
            No integrations configured.
          </div>
        ) : (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(360px, 1fr))', gap: '16px' }}>
            {integrations.map(integ => {
              const st = statuses[integ.id];
              const dbg = debugEvents[integ.id] || [];
              const webhookUrl = `${window.location.origin}/api/v1/integrations/webhook/${integ.routing_key}`;

              return (
                <div 
                  key={integ.id} 
                  style={{
                    background: 'rgba(255, 255, 255, 0.02)',
                    border: '1px solid var(--border-color)',
                    borderRadius: 'var(--radius-lg)',
                    padding: '16px',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '12px'
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                      <div className="logo-badge" style={{ width: '34px', height: '34px', borderRadius: '8px' }}>
                        <Share2 size={16} />
                      </div>
                      <div>
                        <div style={{ fontWeight: 700, fontSize: '0.95rem' }}>{integ.name}</div>
                        <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>Type: {integ.type}</div>
                      </div>
                    </div>
                    <span className={`badge-tag ${st?.state === 'RUNNING' ? 'live' : ''}`}>
                      {st?.state || 'RUNNING'}
                    </span>
                  </div>

                  <div style={{ background: 'rgba(0, 0, 0, 0.25)', padding: '10px', borderRadius: '6px', fontSize: '0.75rem' }}>
                    <div style={{ color: 'var(--text-muted)', marginBottom: '4px' }}>Public Ingestion Webhook URL:</div>
                    <code className="font-mono" style={{ color: 'var(--accent-cyan)', wordBreak: 'break-all' }}>
                      {webhookUrl}
                    </code>
                  </div>

                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                    <span>Rx Events: <strong style={{ color: '#fff' }}>{st?.counters?.rx_events ?? 0}</strong></span>
                    <span>Auto-created: <strong style={{ color: 'var(--accent-emerald)' }}>{st?.counters?.auto_created ?? 0}</strong></span>
                    <span>Errors: <strong style={{ color: st?.counters?.errors ? 'var(--accent-rose)' : 'inherit' }}>{st?.counters?.errors ?? 0}</strong></span>
                  </div>

                  <div style={{ display: 'flex', gap: '8px', marginTop: '4px' }}>
                    <button 
                      className="btn btn-primary btn-sm"
                      style={{ flex: 1 }}
                      onClick={() => handleSendLiveWebhook(integ.routing_key)}
                    >
                      <Send size={13} />
                      <span>Simulate HTTP Ingest</span>
                    </button>
                  </div>

                  {dbg.length > 0 && (
                    <div style={{ borderTop: '1px solid var(--border-color)', paddingTop: '10px' }}>
                      <div style={{ fontSize: '0.75rem', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '6px' }}>
                        Recent Debug Trace ({dbg.length} events):
                      </div>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: '4px', maxHeight: '100px', overflowY: 'auto' }}>
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

      {/* Interactive Converter Test Harness */}
      <div className="card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px', flexWrap: 'wrap', gap: '12px' }}>
          <div>
            <h3 style={{ fontSize: '1.1rem', fontWeight: 700, display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Code2 size={18} style={{ color: 'var(--accent-violet)' }} />
              <span>Live Converter Test Harness & Payload Debugger</span>
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
                <option key={c.id} value={c.id}>{c.name} ({c.mode})</option>
              ))}
            </select>
          </div>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(400px, 1fr))', gap: '16px' }}>
          {/* Input raw payload */}
          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '6px' }}>
              <label style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-secondary)' }}>Sample Raw Message (JSON / Hex / Envelope)</label>
            </div>
            <textarea
              className="form-input font-mono"
              rows={12}
              style={{ resize: 'vertical', fontSize: '0.8rem', lineHeight: '1.4' }}
              value={testPayload}
              onChange={e => setTestPayload(e.target.value)}
            />
            <div style={{ marginTop: '10px' }}>
              <button 
                className="btn btn-primary"
                onClick={handleRunTest}
                disabled={testing}
              >
                {testing ? <RefreshCw className="animate-spin" size={15} /> : <Play size={15} />}
                <span>Execute Converter Test</span>
              </button>
            </div>
          </div>

          {/* Test Output Panel */}
          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '6px' }}>
              <label style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-secondary)' }}>Canonical Decoded UplinkEvent</label>
              {testResult && (
                <span className="badge-tag live" style={{ fontSize: '0.7rem' }}>
                  <Sparkles size={11} style={{ marginRight: '4px' }} />
                  Decode Succeeded
                </span>
              )}
            </div>

            {testError ? (
              <div style={{
                background: 'rgba(244, 63, 94, 0.1)',
                border: '1px solid rgba(244, 63, 94, 0.3)',
                color: 'var(--accent-rose)',
                padding: '16px',
                borderRadius: 'var(--radius-md)',
                fontSize: '0.85rem'
              }}>
                <div style={{ fontWeight: 700, marginBottom: '4px' }}>Evaluation Error:</div>
                <code>{testError}</code>
              </div>
            ) : testResult ? (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
                <div style={{ background: 'rgba(0, 0, 0, 0.35)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '12px', fontSize: '0.8rem' }}>
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '8px', marginBottom: '8px' }}>
                    <div>
                      <span style={{ color: 'var(--text-muted)' }}>Resolved Device: </span>
                      <strong style={{ color: 'var(--accent-cyan)' }}>{testResult.device?.name}</strong>
                    </div>
                    <div>
                      <span style={{ color: 'var(--text-muted)' }}>Type: </span>
                      <strong style={{ color: '#fff' }}>{testResult.device?.type || 'generic'}</strong>
                    </div>
                  </div>

                  {testResult.telemetry && testResult.telemetry.length > 0 && (
                    <div style={{ marginTop: '8px' }}>
                      <span style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>Decoded Telemetry Points:</span>
                      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px', marginTop: '4px' }}>
                        {Object.entries(testResult.telemetry[0]?.values || {}).map(([k, v]) => (
                          <div key={k} style={{ background: 'rgba(6, 182, 212, 0.15)', border: '1px solid rgba(6, 182, 212, 0.3)', borderRadius: '4px', padding: '3px 8px', fontSize: '0.75rem' }}>
                            <span style={{ color: 'var(--text-secondary)' }}>{k}: </span>
                            <strong style={{ color: '#fff' }}>{String(v)}</strong>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {testResult.attributes && Object.keys(testResult.attributes).length > 0 && (
                    <div style={{ marginTop: '8px' }}>
                      <span style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>Extracted Attributes (Client Scope):</span>
                      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px', marginTop: '4px' }}>
                        {Object.entries(testResult.attributes).map(([k, v]) => (
                          <div key={k} style={{ background: 'rgba(168, 85, 247, 0.15)', border: '1px solid rgba(168, 85, 247, 0.3)', borderRadius: '4px', padding: '3px 8px', fontSize: '0.75rem' }}>
                            <span style={{ color: 'var(--text-secondary)' }}>{k}: </span>
                            <strong style={{ color: '#fff' }}>{String(v)}</strong>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </div>

                <pre className="font-mono" style={{
                  background: 'rgba(0, 0, 0, 0.45)',
                  border: '1px solid var(--border-color)',
                  borderRadius: 'var(--radius-md)',
                  padding: '12px',
                  fontSize: '0.75rem',
                  maxHeight: '160px',
                  overflowY: 'auto',
                  color: 'var(--accent-emerald)',
                  margin: 0
                }}>
                  {JSON.stringify(testResult, null, 2)}
                </pre>
              </div>
            ) : (
              <div style={{
                background: 'rgba(0, 0, 0, 0.2)',
                border: '1px dashed var(--border-color)',
                borderRadius: 'var(--radius-md)',
                padding: '36px 16px',
                textAlign: 'center',
                color: 'var(--text-muted)',
                fontSize: '0.85rem'
              }}>
                <Terminal size={24} style={{ margin: '0 auto 8px', opacity: 0.5 }} />
                Click "Execute Converter Test" to test the transform pipeline.
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
