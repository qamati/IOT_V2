import { useState, useEffect } from 'react';
import { 
  Network, 
  RefreshCw, 
  CheckCircle, 
  Server, 
  Radio, 
  ShieldCheck, 
  ArrowUpRight,
  Database
} from 'lucide-react';
import { api } from '../api';
import type { EdgeNode } from '../api';

export function EdgeView() {
  const [nodes, setNodes] = useState<EdgeNode[]>([]);
  const [loading, setLoading] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [syncLog, setSyncLog] = useState<string | null>(null);

  useEffect(() => {
    loadNodes();
  }, []);

  const loadNodes = async () => {
    setLoading(true);
    try {
      const list = await api.listEdgeNodes();
      setNodes(list);
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  const handleSimulateSync = async (node: EdgeNode) => {
    setSyncing(true);
    setSyncLog(null);
    try {
      const nextSeq = Date.now() % 100000;
      const batch = {
        edge_id: node.id,
        sequence: nextSeq,
        timestamp: Date.now(),
        lane_l0: [
          {
            entity_id: node.id,
            timestamp: Date.now(),
            type: 'ALARM',
            payload: {
              type: 'Grid Voltage Fluctuation',
              severity: 'WARNING',
              details: { message: 'Edge battery backup engaged' }
            }
          }
        ],
        lane_l1: [
          {
            entity_id: node.id,
            timestamp: Date.now(),
            type: 'TELEMETRY',
            payload: [
              { key: 'edge_cpu_temp', ts: Date.now(), dbl_v: 48.6 },
              { key: 'edge_buffer_backlog', ts: Date.now(), long_v: 0 }
            ]
          }
        ],
        lane_l2: []
      };

      const ack = await api.triggerEdgeSync(batch);
      setSyncLog(`Sequence #${ack.ack_sequence} ACKed! Ingested ${ack.processed_items} priority lane items.`);
      await loadNodes();
    } catch (e: any) {
      alert('Edge sync failed: ' + e.message);
    } finally {
      setSyncing(false);
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      <div className="page-header">
        <div>
          <h2 className="page-title">Edge Fleet & Store-and-Forward Hub</h2>
          <p className="page-subtitle">Autonomous Go Edge nodes running local brokers, Pebble priority lanes (L0/L1/L2), and lossless cloud synchronization</p>
        </div>
        <button className="btn btn-secondary" onClick={loadNodes} disabled={loading}>
          <RefreshCw className={loading ? 'animate-spin' : ''} size={15} />
          <span>Refresh Fleet</span>
        </button>
      </div>

      {/* Priority Lanes Architecture Highlight */}
      <div className="stats-grid">
        <div className="card">
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '8px' }}>
            <Radio size={20} style={{ color: 'var(--accent-rose)' }} />
            <h3 style={{ fontSize: '1rem', fontWeight: 700 }}>Lane L0 (Critical)</h3>
          </div>
          <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
            Process alarms, safety interlocks, and emergency control logs. Never dropped during disconnects.
          </p>
          <div style={{ marginTop: '12px' }}>
            <span className="badge-tag live" style={{ background: 'rgba(244, 63, 94, 0.2)', color: 'var(--accent-rose)' }}>0 ms Priority</span>
          </div>
        </div>

        <div className="card">
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '8px' }}>
            <Database size={20} style={{ color: 'var(--accent-cyan)' }} />
            <h3 style={{ fontSize: '1rem', fontWeight: 700 }}>Lane L1 (Telemetry)</h3>
          </div>
          <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
            Real-time process state and sensor measurements. Downsampled per storage quotas if disconnected &gt; 72h.
          </p>
          <div style={{ marginTop: '12px' }}>
            <span className="badge-tag live">Ordered Drain</span>
          </div>
        </div>

        <div className="card">
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '8px' }}>
            <ShieldCheck size={20} style={{ color: 'var(--accent-emerald)' }} />
            <h3 style={{ fontSize: '1rem', fontWeight: 700 }}>Lane L2 (Historical)</h3>
          </div>
          <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
            Bulk audit trails and high-frequency wave captures batched and drained during idle network windows.
          </p>
          <div style={{ marginTop: '12px' }}>
            <span className="badge-tag live">Zero Loss</span>
          </div>
        </div>
      </div>

      {/* Edge Nodes Directory */}
      <div className="card">
        <h3 style={{ fontSize: '1.1rem', fontWeight: 700, marginBottom: '16px', display: 'flex', alignItems: 'center', gap: '8px' }}>
          <Network size={18} style={{ color: 'var(--accent-emerald)' }} />
          <span>Active Fleet Edge Nodes ({nodes.length})</span>
        </h3>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(360px, 1fr))', gap: '16px' }}>
          {nodes.map(node => (
            <div 
              key={node.id}
              style={{
                background: 'rgba(255, 255, 255, 0.02)',
                border: '1px solid var(--border-color)',
                borderRadius: 'var(--radius-lg)',
                padding: '20px',
                display: 'flex',
                flexDirection: 'column',
                gap: '14px'
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                  <div className="logo-badge" style={{ width: '38px', height: '38px', borderRadius: '8px' }}>
                    <Server size={18} />
                  </div>
                  <div>
                    <div style={{ fontWeight: 700, fontSize: '0.95rem', color: '#fff' }}>{node.name}</div>
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{node.hardware_model}</div>
                  </div>
                </div>
                <span className="badge-tag live">
                  {node.status}
                </span>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '10px', fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                <div>
                  <span style={{ color: 'var(--text-muted)' }}>Firmware: </span>
                  <strong style={{ color: '#fff' }}>{node.firmware_version}</strong>
                </div>
                <div>
                  <span style={{ color: 'var(--text-muted)' }}>Backlog: </span>
                  <strong style={{ color: 'var(--accent-emerald)' }}>{node.backlog_count} msgs</strong>
                </div>
                <div>
                  <span style={{ color: 'var(--text-muted)' }}>Edge Key: </span>
                  <code className="font-mono" style={{ color: 'var(--accent-cyan)' }}>{node.edge_key}</code>
                </div>
                <div>
                  <span style={{ color: 'var(--text-muted)' }}>Last Sync: </span>
                  <span>{new Date(node.last_sync_ts).toLocaleTimeString()}</span>
                </div>
              </div>

              <button 
                className="btn btn-primary"
                onClick={() => handleSimulateSync(node)}
                disabled={syncing}
              >
                {syncing ? <RefreshCw className="animate-spin" size={15} /> : <ArrowUpRight size={15} />}
                <span>Simulate Store-and-Forward Catch-Up</span>
              </button>
            </div>
          ))}
        </div>

        {syncLog && (
          <div style={{
            marginTop: '16px',
            background: 'rgba(16, 185, 129, 0.15)',
            border: '1px solid rgba(16, 185, 129, 0.3)',
            color: 'var(--accent-emerald)',
            padding: '12px 16px',
            borderRadius: 'var(--radius-md)',
            fontSize: '0.85rem',
            display: 'flex',
            alignItems: 'center',
            gap: '8px'
          }}>
            <CheckCircle size={16} />
            <span>{syncLog}</span>
          </div>
        )}
      </div>
    </div>
  );
}
