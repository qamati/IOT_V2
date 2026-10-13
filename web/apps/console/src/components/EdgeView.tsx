import { useState, useEffect } from 'react';
import { 
  Network, 
  RefreshCw, 
  CheckCircle, 
  Server, 
  Radio, 
  ShieldCheck, 
  ArrowUpRight,
  Database,
  Plus,
  Trash2,
  Terminal,
  Activity,
  X,
  Cpu,
  Layers
} from 'lucide-react';
import { api } from '../api';
import type { EdgeNode, SyncJournalEntry, Device } from '../api';
import './EdgeView.css';

export function EdgeView() {
  const [nodes, setNodes] = useState<EdgeNode[]>([]);
  const [loading, setLoading] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [syncLog, setSyncLog] = useState<string | null>(null);

  // Modals state
  const [showRegisterModal, setShowRegisterModal] = useState(false);
  const [showAssignModal, setShowAssignModal] = useState(false);
  const [showJournalModal, setShowJournalModal] = useState(false);
  const [selectedNode, setSelectedNode] = useState<EdgeNode | null>(null);

  // Form states
  const [newName, setNewName] = useState('');
  const [newModel, setNewModel] = useState('Advantech UNO-2271G');
  const [newFw, setNewFw] = useState('v1.0.0-edge');

  // Device assignment state
  const [allDevices, setAllDevices] = useState<Device[]>([]);
  const [selectedDevIds, setSelectedDevIds] = useState<string[]>([]);
  const [savingAssign, setSavingAssign] = useState(false);

  // Journal state
  const [journalEntries, setJournalEntries] = useState<SyncJournalEntry[]>([]);
  const [loadingJournal, setLoadingJournal] = useState(false);

  useEffect(() => {
    loadNodes();
    loadDevices();
  }, []);

  const loadNodes = async () => {
    setLoading(true);
    try {
      const list = await api.listEdgeNodes();
      setNodes(Array.isArray(list) ? list : []);
    } catch (e) {
      console.error('Failed to load edge nodes:', e);
      setNodes([]);
    } finally {
      setLoading(false);
    }
  };

  const loadDevices = async () => {
    try {
      const res = await api.listDevices();
      setAllDevices(res.data || []);
    } catch (e) {
      console.error('Failed to load devices:', e);
    }
  };

  const handleRegisterNode = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newName.trim()) return;

    try {
      await api.createEdgeNode({
        name: newName,
        hardware_model: newModel,
        firmware_version: newFw,
        status: 'OFFLINE',
      });
      setShowRegisterModal(false);
      setNewName('');
      await loadNodes();
    } catch (err: any) {
      alert('Failed to register edge node: ' + err.message);
    }
  };

  const handleDeleteNode = async (node: EdgeNode) => {
    if (!confirm(`Are you sure you want to remove edge node "${node.name}"?`)) return;
    try {
      await api.deleteEdgeNode(node.id);
      await loadNodes();
    } catch (err: any) {
      alert('Failed to delete edge node: ' + err.message);
    }
  };

  const openAssignModal = async (node: EdgeNode) => {
    setSelectedNode(node);
    try {
      const currentAssigned = await api.getEdgeAssignments(node.id);
      setSelectedDevIds(currentAssigned || []);
    } catch (e) {
      setSelectedDevIds(node.assigned_devices || []);
    }
    setShowAssignModal(true);
  };

  const handleSaveAssignments = async () => {
    if (!selectedNode) return;
    setSavingAssign(true);
    try {
      await api.assignEdgeDevices(selectedNode.id, selectedDevIds);
      setShowAssignModal(false);
      await loadNodes();
    } catch (err: any) {
      alert('Failed to save device assignments: ' + err.message);
    } finally {
      setSavingAssign(false);
    }
  };

  const openJournalModal = async (node: EdgeNode) => {
    setSelectedNode(node);
    setLoadingJournal(true);
    setShowJournalModal(true);
    try {
      const entries = await api.getEdgeJournal(node.id, 25);
      setJournalEntries(entries);
    } catch (err) {
      console.error('Failed to fetch journal:', err);
      setJournalEntries([]);
    } finally {
      setLoadingJournal(false);
    }
  };

  const handleRemoteCommand = async (node: EdgeNode, command: string) => {
    try {
      const resp = await api.executeEdgeCommand(node.id, command);
      alert(`[Command: ${resp.command}] ${resp.message}`);
      await loadNodes();
    } catch (err: any) {
      alert(`Remote command failed: ${err.message}`);
    }
  };

  const handleSimulateSync = async (node: EdgeNode) => {
    setSyncing(true);
    setSyncLog(null);
    try {
      const nextSeq = (node.uplink_seq || 0) + 1;
      const now = Date.now();
      const pressure = (48 + Math.random() * 8).toFixed(2);
      const flow = (115 + Math.random() * 20).toFixed(1);

      const batch = {
        edge_id: node.id,
        sequence: nextSeq,
        timestamp: now,
        lane_l0: [
          {
            entity_id: node.id,
            timestamp: now,
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
            timestamp: now,
            type: 'TELEMETRY',
            payload: [
              { key: 'line_pressure_bar', ts: now, dbl_v: parseFloat(pressure) },
              { key: 'feed_flow_lpm', ts: now, dbl_v: parseFloat(flow) },
              { key: 'edge_buffer_backlog', ts: now, long_v: 0 }
            ]
          }
        ],
        lane_l2: [
          {
            entity_id: node.id,
            timestamp: now,
            type: 'LOG',
            payload: { event: 'store_forward_flush', level: 'INFO', items: 3 }
          }
        ]
      };

      const ack = await api.triggerEdgeSync(batch);
      setSyncLog(`Sequence #${ack.ack_sequence} ACKed! Ingested ${ack.processed_items} items across L0 (Alarms), L1 (Telemetry) & L2.`);
      await loadNodes();
    } catch (e: any) {
      alert('Edge sync failed: ' + e.message);
    } finally {
      setSyncing(false);
    }
  };

  const safeNodes = Array.isArray(nodes) ? nodes : [];
  const onlineCount = safeNodes.filter(n => n.status === 'ONLINE').length;
  const offlineCount = safeNodes.filter(n => n.status !== 'ONLINE').length;
  const totalBacklog = safeNodes.reduce((acc, n) => acc + (n.backlog_count || 0), 0);

  return (
    <div className="edge-container">
      <div className="page-header">
        <div>
          <h2 className="page-title">Edge Fleet & Store-and-Forward Hub</h2>
          <p className="page-subtitle">Autonomous Go Edge nodes running local brokers, Pebble priority lanes (L0/L1/L2), and lossless cloud synchronization</p>
        </div>
        <div className="edge-header-actions">
          <button className="btn btn-secondary" onClick={loadNodes} disabled={loading}>
            <RefreshCw className={loading ? 'animate-spin' : ''} size={15} />
            <span>Refresh Fleet</span>
          </button>
          <button className="btn btn-primary" onClick={() => setShowRegisterModal(true)}>
            <Plus size={15} />
            <span>Register Edge</span>
          </button>
        </div>
      </div>

      {/* Fleet Overview Metrics */}
      <div className="stats-grid">
        <div className="card">
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '8px' }}>
            <Server size={20} style={{ color: 'var(--accent-cyan)' }} />
            <h3 style={{ fontSize: '1rem', fontWeight: 700 }}>Total Edge Nodes</h3>
          </div>
          <p style={{ fontSize: '1.6rem', fontWeight: 800, color: '#fff' }}>{safeNodes.length}</p>
          <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Registered in PostgreSQL</span>
        </div>

        <div className="card">
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '8px' }}>
            <Activity size={20} style={{ color: 'var(--accent-emerald)' }} />
            <h3 style={{ fontSize: '1rem', fontWeight: 700 }}>Connected (Online)</h3>
          </div>
          <p style={{ fontSize: '1.6rem', fontWeight: 800, color: 'var(--accent-emerald)' }}>{onlineCount}</p>
          <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>{offlineCount} nodes offline / buffer mode</span>
        </div>

        <div className="card">
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '8px' }}>
            <Database size={20} style={{ color: 'var(--accent-amber)' }} />
            <h3 style={{ fontSize: '1rem', fontWeight: 700 }}>Total Backlog</h3>
          </div>
          <p style={{ fontSize: '1.6rem', fontWeight: 800, color: 'var(--accent-amber)' }}>{totalBacklog} msgs</p>
          <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Store-and-forward buffered events</span>
        </div>

        <div className="card">
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '8px' }}>
            <Radio size={20} style={{ color: 'var(--accent-rose)' }} />
            <h3 style={{ fontSize: '1rem', fontWeight: 700 }}>Lane L0 Integrity</h3>
          </div>
          <p style={{ fontSize: '1.6rem', fontWeight: 800, color: 'var(--accent-rose)' }}>100%</p>
          <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Critical Alarms never dropped</span>
        </div>
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
            <span className="edge-lane-badge l0">0 ms Priority</span>
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
            <span className="edge-lane-badge l1">Ordered Drain</span>
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
            <span className="edge-lane-badge l2">Zero Loss</span>
          </div>
        </div>
      </div>

      {/* Edge Nodes Directory */}
      <div className="card">
        <h3 style={{ fontSize: '1.1rem', fontWeight: 700, marginBottom: '16px', display: 'flex', alignItems: 'center', gap: '8px' }}>
          <Network size={18} style={{ color: 'var(--accent-emerald)' }} />
          <span>Active Fleet Edge Nodes ({safeNodes.length})</span>
        </h3>

        <div className="edge-grid">
          {safeNodes.map(node => (
            <div key={node.id} className="edge-card">
              <div className="edge-card-header">
                <div className="edge-card-title-group">
                  <div className="edge-icon-box">
                    <Server size={20} />
                  </div>
                  <div>
                    <div className="edge-name">{node.name}</div>
                    <div className="edge-model">{node.hardware_model}</div>
                  </div>
                </div>
                <span className={`badge-tag ${node.status === 'ONLINE' ? 'live' : 'offline'}`}>
                  {node.status}
                </span>
              </div>

              <div className="edge-details-grid">
                <div className="edge-detail-item">
                  <span>Firmware</span>
                  <strong style={{ color: '#fff' }}>{node.firmware_version}</strong>
                </div>
                <div className="edge-detail-item">
                  <span>Backlog</span>
                  <strong style={{ color: node.backlog_count > 0 ? 'var(--accent-amber)' : 'var(--accent-emerald)' }}>
                    {node.backlog_count || 0} msgs
                  </strong>
                </div>
                <div className="edge-detail-item">
                  <span>Uplink Seq</span>
                  <strong style={{ color: 'var(--accent-cyan)' }}>#{node.uplink_seq || 0}</strong>
                </div>
                <div className="edge-detail-item">
                  <span>Last Sync</span>
                  <span>{node.last_sync_ts ? new Date(node.last_sync_ts).toLocaleTimeString() : 'Never'}</span>
                </div>
                <div className="edge-detail-item" style={{ gridColumn: 'span 2' }}>
                  <span>Edge Key</span>
                  <code className="font-mono" style={{ color: 'var(--accent-cyan)', fontSize: '0.78rem' }}>{node.edge_key}</code>
                </div>
              </div>

              {/* Action Buttons */}
              <div className="edge-card-actions">
                <button 
                  className="btn btn-primary"
                  style={{ flex: '1 1 140px', fontSize: '0.8rem' }}
                  onClick={() => handleSimulateSync(node)}
                  disabled={syncing}
                >
                  {syncing ? <RefreshCw className="animate-spin" size={13} /> : <ArrowUpRight size={13} />}
                  <span>Simulate Sync</span>
                </button>
                <button 
                  className="btn btn-secondary"
                  style={{ fontSize: '0.8rem' }}
                  title="Assign Devices"
                  onClick={() => openAssignModal(node)}
                >
                  <Layers size={13} />
                  <span>Devices</span>
                </button>
                <button 
                  className="btn btn-secondary"
                  style={{ fontSize: '0.8rem' }}
                  title="Sync Journal"
                  onClick={() => openJournalModal(node)}
                >
                  <Terminal size={13} />
                  <span>Journal</span>
                </button>
                <button 
                  className="btn btn-secondary"
                  style={{ fontSize: '0.8rem' }}
                  title="Remote Reboot"
                  onClick={() => handleRemoteCommand(node, 'REBOOT')}
                >
                  <Cpu size={13} />
                  <span>Reboot</span>
                </button>
                <button 
                  className="btn btn-danger"
                  style={{ padding: '6px 10px' }}
                  title="Delete Edge Node"
                  onClick={() => handleDeleteNode(node)}
                >
                  <Trash2 size={13} />
                </button>
              </div>
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

      {/* Modal: Register New Edge */}
      {showRegisterModal && (
        <div className="edge-modal-overlay" onClick={() => setShowRegisterModal(false)}>
          <div className="edge-modal" onClick={e => e.stopPropagation()}>
            <div className="edge-modal-header">
              <h3 className="edge-modal-title">
                <Plus size={18} style={{ color: 'var(--accent-cyan)' }} />
                <span>Register New Edge Node</span>
              </h3>
              <button className="btn btn-secondary" style={{ padding: '4px' }} onClick={() => setShowRegisterModal(false)}>
                <X size={16} />
              </button>
            </div>

            <form onSubmit={handleRegisterNode} style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
              <div className="edge-form-group">
                <label>Node Name *</label>
                <input 
                  type="text" 
                  className="edge-form-input" 
                  placeholder="e.g. edge-station-hvac-02" 
                  value={newName} 
                  onChange={e => setNewName(e.target.value)} 
                  required 
                />
              </div>

              <div className="edge-form-group">
                <label>Hardware Model</label>
                <input 
                  type="text" 
                  className="edge-form-input" 
                  placeholder="e.g. Advantech UNO-2271G or Raspberry Pi 4" 
                  value={newModel} 
                  onChange={e => setNewModel(e.target.value)} 
                />
              </div>

              <div className="edge-form-group">
                <label>Firmware Version</label>
                <input 
                  type="text" 
                  className="edge-form-input" 
                  placeholder="e.g. v1.0.0-edge" 
                  value={newFw} 
                  onChange={e => setNewFw(e.target.value)} 
                />
              </div>

              <div className="edge-modal-footer">
                <button type="button" className="btn btn-secondary" onClick={() => setShowRegisterModal(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary">
                  Register Node
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: Device Assignments */}
      {showAssignModal && selectedNode && (
        <div className="edge-modal-overlay" onClick={() => setShowAssignModal(false)}>
          <div className="edge-modal" onClick={e => e.stopPropagation()}>
            <div className="edge-modal-header">
              <h3 className="edge-modal-title">
                <Layers size={18} style={{ color: 'var(--accent-cyan)' }} />
                <span>Device Assignments: {selectedNode.name}</span>
              </h3>
              <button className="btn btn-secondary" style={{ padding: '4px' }} onClick={() => setShowAssignModal(false)}>
                <X size={16} />
              </button>
            </div>

            <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
              Select cloud devices whose telemetry and downlink RPCs will be routed through this autonomous edge gateway.
            </p>

            <div style={{ maxHeight: '280px', overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: '8px' }}>
              {allDevices.length === 0 ? (
                <div style={{ color: 'var(--text-muted)', fontSize: '0.85rem', textAlign: 'center', padding: '16px' }}>
                  No devices available in tenant.
                </div>
              ) : (
                allDevices.map(dev => {
                  const isChecked = selectedDevIds.includes(dev.id);
                  return (
                    <label 
                      key={dev.id} 
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: '10px',
                        padding: '8px 12px',
                        background: isChecked ? 'rgba(6, 182, 212, 0.1)' : 'rgba(255, 255, 255, 0.02)',
                        border: `1px solid ${isChecked ? 'rgba(6, 182, 212, 0.3)' : 'var(--border-color)'}`,
                        borderRadius: 'var(--radius-md)',
                        cursor: 'pointer'
                      }}
                    >
                      <input 
                        type="checkbox" 
                        checked={isChecked} 
                        onChange={e => {
                          if (e.target.checked) {
                            setSelectedDevIds([...selectedDevIds, dev.id]);
                          } else {
                            setSelectedDevIds(selectedDevIds.filter(id => id !== dev.id));
                          }
                        }}
                      />
                      <div style={{ flex: 1 }}>
                        <div style={{ fontWeight: 600, fontSize: '0.88rem', color: '#fff' }}>{dev.name}</div>
                        <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)' }}>Profile: {dev.label || 'Standard Device'}</div>
                      </div>
                    </label>
                  );
                })
              )}
            </div>

            <div className="edge-modal-footer">
              <button type="button" className="btn btn-secondary" onClick={() => setShowAssignModal(false)}>
                Cancel
              </button>
              <button 
                type="button" 
                className="btn btn-primary" 
                onClick={handleSaveAssignments}
                disabled={savingAssign}
              >
                {savingAssign ? 'Saving...' : `Save Assignments (${selectedDevIds.length})`}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Modal: Sync Journal */}
      {showJournalModal && selectedNode && (
        <div className="edge-modal-overlay" onClick={() => setShowJournalModal(false)}>
          <div className="edge-modal" style={{ maxWidth: '680px' }} onClick={e => e.stopPropagation()}>
            <div className="edge-modal-header">
              <h3 className="edge-modal-title">
                <Terminal size={18} style={{ color: 'var(--accent-emerald)' }} />
                <span>Sync Journal: {selectedNode.name}</span>
              </h3>
              <button className="btn btn-secondary" style={{ padding: '4px' }} onClick={() => setShowJournalModal(false)}>
                <X size={16} />
              </button>
            </div>

            <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
              Historical store-and-forward batch entries recorded in PostgreSQL hypertable.
            </p>

            {loadingJournal ? (
              <div style={{ textAlign: 'center', padding: '24px', color: 'var(--text-muted)' }}>
                Loading journal history...
              </div>
            ) : journalEntries.length === 0 ? (
              <div style={{ textAlign: 'center', padding: '24px', color: 'var(--text-muted)' }}>
                No sync batches recorded yet for this node.
              </div>
            ) : (
              <div style={{ maxHeight: '360px', overflowY: 'auto' }}>
                <table className="edge-journal-table">
                  <thead>
                    <tr>
                      <th>Seq #</th>
                      <th>L0 (Alarms)</th>
                      <th>L1 (Telemetry)</th>
                      <th>L2 (Logs)</th>
                      <th>Processed</th>
                      <th>Status</th>
                      <th>Time</th>
                    </tr>
                  </thead>
                  <tbody>
                    {journalEntries.map(entry => (
                      <tr key={entry.id}>
                        <td><strong>#{entry.sequence}</strong></td>
                        <td><span className="edge-lane-badge l0">{entry.lane_l0_count}</span></td>
                        <td><span className="edge-lane-badge l1">{entry.lane_l1_count}</span></td>
                        <td><span className="edge-lane-badge l2">{entry.lane_l2_count}</span></td>
                        <td>{entry.processed_items} items</td>
                        <td><span className="badge-tag live" style={{ fontSize: '0.7rem' }}>{entry.status}</span></td>
                        <td>{new Date(entry.synced_at).toLocaleTimeString()}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}

            <div className="edge-modal-footer">
              <button type="button" className="btn btn-secondary" onClick={() => setShowJournalModal(false)}>
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
