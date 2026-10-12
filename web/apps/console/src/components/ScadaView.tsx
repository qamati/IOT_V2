import { useState, useEffect } from 'react';
import { 
  Gauge, 
  Power, 
  ShieldAlert, 
  CheckCircle, 
  Clock, 
  RotateCw, 
  Sliders, 
  PlayCircle,
  StopCircle,
  RefreshCw
} from 'lucide-react';
import { api } from '../api';
import type { ScadaTag, ISA182Alarm, ControlCommand } from '../api';

export function ScadaView() {
  const [tags, setTags] = useState<ScadaTag[]>([]);
  const [alarms, setAlarms] = useState<ISA182Alarm[]>([]);
  const [loading, setLoading] = useState(false);

  // SBO State
  const [sboTargetTag] = useState('WaterTreatment.Pump101.Status');
  const [sboDesiredValue, setSboDesiredValue] = useState<number>(0);
  const [activeCommand, setActiveCommand] = useState<ControlCommand | null>(null);
  const [sboTimer, setSboTimer] = useState<number>(0);
  const [operating, setOperating] = useState(false);

  // Simulation input
  const [simLevel, setSimLevel] = useState<number>(76.4);

  useEffect(() => {
    loadData();
    const interval = setInterval(loadData, 4000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    if (activeCommand?.status === 'SELECTED' && sboTimer > 0) {
      const t = setTimeout(() => setSboTimer(sboTimer - 1), 1000);
      return () => clearTimeout(t);
    }
  }, [activeCommand, sboTimer]);

  const loadData = async () => {
    setLoading(true);
    try {
      const [tagList, alarmList] = await Promise.all([
        api.listScadaTags(),
        api.listScadaAlarms()
      ]);
      const safeTagList = Array.isArray(tagList) ? tagList : [];
      const safeAlarmList = Array.isArray(alarmList) ? alarmList : [];
      setTags(safeTagList);
      setAlarms(safeAlarmList);

      const tankTag = safeTagList.find(t => t.path === 'WaterTreatment.Tank101.Level');
      if (tankTag && typeof tankTag.value === 'number') {
        setSimLevel(tankTag.value);
      }
    } catch (e) {
      console.error(e);
      setTags([]);
      setAlarms([]);
    } finally {
      setLoading(false);
    }
  };

  const handleUpdateTag = async (path: string, val: any) => {
    try {
      await api.updateScadaTag(path, val);
      await loadData();
    } catch (e: any) {
      alert('Tag update failed: ' + e.message);
    }
  };

  const handleSelectSBO = async () => {
    try {
      const cmd = await api.selectScadaControl(sboTargetTag, sboDesiredValue);
      setActiveCommand(cmd);
      setSboTimer(60);
    } catch (e: any) {
      alert('SBO Select failed: ' + e.message);
    }
  };

  const handleOperateSBO = async () => {
    if (!activeCommand) return;
    setOperating(true);
    try {
      const cmd = await api.operateScadaControl(activeCommand.select_id);
      setActiveCommand(cmd);
      await loadData();
    } catch (e: any) {
      alert('SBO Operate failed: ' + e.message);
    } finally {
      setOperating(false);
    }
  };

  const handleAckAlarm = async (id: string) => {
    try {
      await api.ackScadaAlarm(id);
      await loadData();
    } catch (e: any) {
      alert('Ack failed: ' + e.message);
    }
  };

  const handleShelveAlarm = async (id: string) => {
    try {
      await api.shelveScadaAlarm(id, 300000);
      await loadData();
    } catch (e: any) {
      alert('Shelve failed: ' + e.message);
    }
  };

  // Find tags for HMI synoptic
  const safeTags = Array.isArray(tags) ? tags : [];
  const safeAlarms = Array.isArray(alarms) ? alarms : [];

  const tankLevelTag = safeTags.find(t => t.path === 'WaterTreatment.Tank101.Level');
  const pumpStatusTag = safeTags.find(t => t.path === 'WaterTreatment.Pump101.Status');
  const flowTag = safeTags.find(t => t.path === 'WaterTreatment.Pump101.Flow');
  const pressureTag = safeTags.find(t => t.path === 'WaterTreatment.Discharge.Pressure');

  const tankLevel = typeof tankLevelTag?.value === 'number' ? tankLevelTag.value : simLevel;
  const isPumpRunning = pumpStatusTag?.value === 1 || pumpStatusTag?.value === true;
  const flowVal = typeof flowTag?.value === 'number' ? flowTag.value : 342.8;
  const pressureVal = typeof pressureTag?.value === 'number' ? pressureTag.value : 62.5;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      <div className="page-header">
        <div>
          <h2 className="page-title">Industrial SCADA Process Studio & HMI</h2>
          <p className="page-subtitle">Tags with OPC quality, deadband suppression, ISA-18.2 alarm lifecycle, and Select-Before-Operate controls</p>
        </div>
        <button className="btn btn-secondary" onClick={loadData} disabled={loading}>
          <RefreshCw className={loading ? 'animate-spin' : ''} size={15} />
          <span>Refresh Tags</span>
        </button>
      </div>

      {/* ISA-18.2 Process Alarms Banner */}
      {safeAlarms.length > 0 && (
        <div style={{
          background: 'rgba(244, 63, 94, 0.08)',
          border: '1px solid rgba(244, 63, 94, 0.3)',
          borderRadius: 'var(--radius-lg)',
          padding: '16px'
        }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '12px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: 'var(--accent-rose)' }}>
              <ShieldAlert size={18} />
              <strong style={{ fontSize: '0.95rem' }}>Active ISA-18.2 Process Alarms ({safeAlarms.length})</strong>
            </div>
            <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>Standard ISA-18.2 / IEC 62682 state machine</span>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            {safeAlarms.map(a => (
              <div 
                key={a.id} 
                style={{
                  background: 'rgba(0, 0, 0, 0.35)',
                  padding: '10px 14px',
                  borderRadius: 'var(--radius-md)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: '12px',
                  flexWrap: 'wrap'
                }}
              >
                <div>
                  <div style={{ fontWeight: 600, fontSize: '0.85rem', color: '#fff' }}>
                    {a.tag_path}: <span style={{ color: 'var(--accent-rose)' }}>{a.alarm_type}</span> ({a.message})
                  </div>
                  <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '2px' }}>
                    State: <strong style={{ color: a.state === 'UNACK_ALARM' ? 'var(--accent-rose)' : 'var(--accent-amber)' }}>{a.state}</strong> · Setpoint: {a.setpoint} · Active since: {new Date(a.active_ts).toLocaleTimeString()}
                  </div>
                </div>

                <div style={{ display: 'flex', gap: '8px' }}>
                  {a.state === 'UNACK_ALARM' && (
                    <button className="btn btn-secondary btn-sm" onClick={() => handleAckAlarm(a.id)}>
                      <CheckCircle size={13} style={{ color: 'var(--accent-emerald)' }} />
                      <span>Acknowledge</span>
                    </button>
                  )}
                  {a.state !== 'SHELVED' && (
                    <button className="btn btn-secondary btn-sm" onClick={() => handleShelveAlarm(a.id)}>
                      <Clock size={13} style={{ color: 'var(--accent-amber)' }} />
                      <span>Shelve (5m)</span>
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Realtime Synoptic HMI Process Canvas */}
      <div className="card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
          <h3 style={{ fontSize: '1.1rem', fontWeight: 700, display: 'flex', alignItems: 'center', gap: '8px' }}>
            <Gauge size={18} style={{ color: 'var(--accent-cyan)' }} />
            <span>Water Treatment Synoptic Process HMI Canvas</span>
          </h3>
          <span className="badge-tag live" style={{ fontSize: '0.75rem' }}>
            30 FPS Animated Runtime
          </span>
        </div>

        <div style={{
          background: 'linear-gradient(180deg, rgba(15, 23, 42, 0.8) 0%, rgba(2, 6, 23, 0.95) 100%)',
          border: '1px solid var(--border-color)',
          borderRadius: 'var(--radius-lg)',
          padding: '24px',
          display: 'flex',
          justifyContent: 'center'
        }}>
          <svg width="100%" height="280" viewBox="0 0 840 280" style={{ maxWidth: '840px' }}>
            <defs>
              <linearGradient id="waterGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                <stop offset="0%" stopColor="#06b6d4" stopOpacity="0.85" />
                <stop offset="100%" stopColor="#0284c7" stopOpacity="0.95" />
              </linearGradient>
              <filter id="glow" x="-20%" y="-20%" width="140%" height="140%">
                <feGaussianBlur stdDeviation="4" result="blur" />
                <feComposite in="SourceGraphic" in2="blur" operator="over" />
              </filter>
            </defs>

            {/* Vessel Tank Background */}
            <rect x="60" y="40" width="160" height="200" rx="10" fill="rgba(30, 41, 59, 0.6)" stroke="#475569" strokeWidth="2" />
            <text x="140" y="30" textAnchor="middle" fill="#94a3b8" fontSize="12" fontWeight="700">TANK 101</text>

            {/* Liquid Fill */}
            {(() => {
              const maxH = 190;
              const curH = (Math.min(100, Math.max(0, tankLevel)) / 100) * maxH;
              const y = 235 - curH;
              return (
                <g>
                  <rect x="65" y={y} width="150" height={curH} rx="6" fill="url(#waterGrad)" />
                  {/* High level limit line */}
                  <line x1="60" y1="59" x2="220" y2="59" stroke="#f43f5e" strokeWidth="2" strokeDasharray="4 3" />
                  <text x="225" y="63" fill="#f43f5e" fontSize="10" fontWeight="700">90% HI-HI</text>
                </g>
              );
            })()}

            <text x="140" y="145" textAnchor="middle" fill="#ffffff" fontSize="20" fontWeight="800" filter="url(#glow)">
              {tankLevel.toFixed(1)} %
            </text>
            <text x="140" y="165" textAnchor="middle" fill="#cbd5e1" fontSize="11">
              Level Height
            </text>

            {/* Connecting Pipe 1 (Tank to Pump) */}
            <path d="M 220 200 L 360 200" stroke="#64748b" strokeWidth="12" strokeLinecap="round" />
            {isPumpRunning && (
              <path d="M 220 200 L 360 200" stroke="#06b6d4" strokeWidth="4" strokeDasharray="10 6">
                <animate attributeName="stroke-dashoffset" from="32" to="0" dur="0.8s" repeatCount="indefinite" />
              </path>
            )}

            {/* Pump 101 Body */}
            <g transform="translate(380, 200)">
              <circle cx="0" cy="0" r="38" fill="rgba(30, 41, 59, 0.9)" stroke={isPumpRunning ? '#10b981' : '#f43f5e'} strokeWidth="3" filter={isPumpRunning ? 'url(#glow)' : undefined} />
              
              {/* Impeller */}
              <g>
                {isPumpRunning && (
                  <animateTransform 
                    attributeName="transform" 
                    type="rotate" 
                    from="0" 
                    to="360" 
                    dur="1.2s" 
                    repeatCount="indefinite" 
                  />
                )}
                <line x1="-22" y1="0" x2="22" y2="0" stroke="#e2e8f0" strokeWidth="4" strokeLinecap="round" />
                <line x1="0" y1="-22" x2="0" y2="22" stroke="#e2e8f0" strokeWidth="4" strokeLinecap="round" />
              </g>

              <text x="0" y="55" textAnchor="middle" fill="#94a3b8" fontSize="12" fontWeight="700">PUMP 101</text>
              <text x="0" y="70" textAnchor="middle" fill={isPumpRunning ? '#10b981' : '#f43f5e'} fontSize="11" fontWeight="800">
                {isPumpRunning ? 'RUNNING' : 'STOPPED'}
              </text>
            </g>

            {/* Connecting Pipe 2 (Pump to Discharge) */}
            <path d="M 418 200 L 580 200 L 580 120 L 700 120" stroke="#64748b" strokeWidth="12" strokeLinecap="round" strokeLinejoin="round" />
            {isPumpRunning && (
              <path d="M 418 200 L 580 200 L 580 120 L 700 120" stroke="#06b6d4" strokeWidth="4" strokeDasharray="10 6" strokeLinejoin="round">
                <animate attributeName="stroke-dashoffset" from="32" to="0" dur="0.8s" repeatCount="indefinite" />
              </path>
            )}

            {/* Flow Transmitters */}
            <g transform="translate(500, 150)">
              <rect x="-45" y="-25" width="90" height="50" rx="8" fill="rgba(15, 23, 42, 0.9)" stroke="#06b6d4" strokeWidth="1.5" />
              <text x="0" y="-8" textAnchor="middle" fill="#94a3b8" fontSize="10" fontWeight="600">FLOW (FT-101)</text>
              <text x="0" y="14" textAnchor="middle" fill="#fff" fontSize="14" fontWeight="800">{isPumpRunning ? flowVal.toFixed(1) : '0.0'} GPM</text>
            </g>

            {/* Pressure Transmitter */}
            <g transform="translate(680, 75)">
              <circle cx="0" cy="0" r="32" fill="rgba(15, 23, 42, 0.9)" stroke="#a855f7" strokeWidth="2" />
              <text x="0" y="-8" textAnchor="middle" fill="#94a3b8" fontSize="9" fontWeight="600">DISCHARGE</text>
              <text x="0" y="10" textAnchor="middle" fill="#fff" fontSize="13" fontWeight="800">{isPumpRunning ? pressureVal.toFixed(1) : '14.7'} PSI</text>
            </g>
          </svg>
        </div>
      </div>

      {/* Grid: Tags Table + SBO Control Panel */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(420px, 1fr))', gap: '20px' }}>
        {/* Process Tags Directory */}
        <div className="card">
          <h3 style={{ fontSize: '1rem', fontWeight: 700, marginBottom: '14px', display: 'flex', alignItems: 'center', gap: '8px' }}>
            <Sliders size={18} style={{ color: 'var(--accent-cyan)' }} />
            <span>Process Tags (Deadbands & Quality)</span>
          </h3>

          <div className="table-container" style={{ maxHeight: '340px', overflowY: 'auto' }}>
            <table className="data-table">
              <thead>
                <tr>
                  <th>Tag Path</th>
                  <th>Value</th>
                  <th>Quality</th>
                  <th>Deadband</th>
                </tr>
              </thead>
              <tbody>
                {safeTags.map(t => (
                  <tr key={t.id}>
                    <td>
                      <div className="font-mono" style={{ fontWeight: 600, fontSize: '0.8rem', color: '#fff' }}>
                        {t.path}
                      </div>
                      <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>{t.data_type}</div>
                    </td>
                    <td>
                      <strong style={{ color: 'var(--accent-cyan)', fontSize: '0.9rem' }}>
                        {typeof t.value === 'number' ? t.value.toFixed(1) : String(t.value)}
                      </strong>{' '}
                      <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>{t.eng_unit}</span>
                    </td>
                    <td>
                      <span style={{
                        background: t.quality === 'GOOD' ? 'rgba(16, 185, 129, 0.15)' : 'rgba(244, 63, 94, 0.15)',
                        color: t.quality === 'GOOD' ? 'var(--accent-emerald)' : 'var(--accent-rose)',
                        padding: '2px 8px',
                        borderRadius: '4px',
                        fontSize: '0.75rem',
                        fontWeight: 700
                      }}>
                        {t.quality}
                      </span>
                    </td>
                    <td style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                      {t.deadband ? `±${t.deadband} ${t.eng_unit || ''}` : 'None'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Quick Simulation Slider */}
          <div style={{ marginTop: '16px', background: 'rgba(255, 255, 255, 0.02)', padding: '12px', borderRadius: 'var(--radius-md)' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '8px', fontSize: '0.8rem' }}>
              <span>Simulate Tank Level Sensor:</span>
              <strong style={{ color: 'var(--accent-cyan)' }}>{simLevel.toFixed(1)} %</strong>
            </div>
            <input 
              type="range" 
              min="0" 
              max="100" 
              step="0.5"
              value={simLevel}
              onChange={e => {
                const n = parseFloat(e.target.value);
                setSimLevel(n);
                handleUpdateTag('WaterTreatment.Tank101.Level', n);
              }}
              style={{ width: '100%', cursor: 'pointer' }}
            />
            <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
              Values exceeding deadband (0.5%) will update live; set {'>'} 90% to trigger ISA-18.2 High-High Alarm!
            </span>
          </div>
        </div>

        {/* Select-Before-Operate (SBO) Safe Control */}
        <div className="card">
          <h3 style={{ fontSize: '1rem', fontWeight: 700, marginBottom: '6px', display: 'flex', alignItems: 'center', gap: '8px' }}>
            <Power size={18} style={{ color: 'var(--accent-amber)' }} />
            <span>Select-Before-Operate (SBO) Industrial Control</span>
          </h3>
          <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '16px' }}>
            Two-phase verified control mechanism to prevent accidental industrial actuation.
          </p>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
            <div className="form-group" style={{ margin: 0 }}>
              <label className="form-label">Control Target Tag</label>
              <input 
                type="text" 
                className="form-input font-mono" 
                value={sboTargetTag} 
                readOnly 
              />
            </div>

            <div className="form-group" style={{ margin: 0 }}>
              <label className="form-label">Desired State Action</label>
              <div style={{ display: 'flex', gap: '10px' }}>
                <button 
                  type="button"
                  className={`btn ${sboDesiredValue === 1 ? 'btn-primary' : 'btn-secondary'}`}
                  style={{ flex: 1 }}
                  onClick={() => setSboDesiredValue(1)}
                >
                  <PlayCircle size={15} />
                  <span>START PUMP (1)</span>
                </button>
                <button 
                  type="button"
                  className={`btn ${sboDesiredValue === 0 ? 'btn-primary' : 'btn-secondary'}`}
                  style={{ flex: 1 }}
                  onClick={() => setSboDesiredValue(0)}
                >
                  <StopCircle size={15} />
                  <span>STOP PUMP (0)</span>
                </button>
              </div>
            </div>

            {/* Step 1: Select Button */}
            {(!activeCommand || activeCommand.status !== 'SELECTED' || sboTimer === 0) ? (
              <button 
                className="btn btn-secondary" 
                style={{ padding: '12px', borderColor: 'var(--accent-amber)' }}
                onClick={handleSelectSBO}
              >
                <ShieldAlert size={16} style={{ color: 'var(--accent-amber)' }} />
                <span>Phase 1: Arm & Select Control Reservation</span>
              </button>
            ) : (
              /* Step 2: Armed Operator State */
              <div style={{
                background: 'rgba(245, 158, 11, 0.1)',
                border: '1px solid rgba(245, 158, 11, 0.3)',
                padding: '16px',
                borderRadius: 'var(--radius-md)',
                display: 'flex',
                flexDirection: 'column',
                gap: '12px'
              }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: 'var(--accent-amber)' }}>
                    <Clock size={16} />
                    <strong>Armed Reservation Active</strong>
                  </div>
                  <span className="badge-tag live" style={{ background: 'var(--accent-amber)', color: '#000' }}>
                    {sboTimer}s Remaining
                  </span>
                </div>

                <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                  Reservation Token: <code className="font-mono">{activeCommand.select_id.slice(0, 18)}...</code>
                </div>

                <button 
                  className="btn btn-primary"
                  style={{ background: 'var(--accent-emerald)', borderColor: 'var(--accent-emerald)', color: '#000', fontWeight: 700 }}
                  onClick={handleOperateSBO}
                  disabled={operating}
                >
                  {operating ? <RotateCw className="animate-spin" size={16} /> : <CheckCircle size={16} />}
                  <span>Phase 2: Confirm & OPERATE Command</span>
                </button>
              </div>
            )}

            {activeCommand?.status === 'OPERATED' && (
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
                <span>Command verified and executed successfully!</span>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
