import React, { useState, useEffect } from 'react';
import {
  AlertTriangle,
  Bell,
  CheckCircle2,
  Play,
  Plus,
  RefreshCw,
  Search,
  Send,
  Trash2,
  Sliders,
  ExternalLink,
  Info
} from 'lucide-react';
import { api } from '../api';
import type {
  Alarm,
  AlarmRule,
  AlarmRuleTestRequest,
  AlarmRuleTestResponse,
  NotificationTarget,
  NotificationRule,
} from '../api';
import './AlarmsView.css';

interface AlarmsViewProps {
  alarms: Alarm[];
  onRefresh: () => void;
  onAckAlarm: (id: string) => void;
  onClearAlarm: (id: string) => void;
}

export const AlarmsView: React.FC<AlarmsViewProps> = ({
  alarms,
  onRefresh,
  onAckAlarm,
  onClearAlarm,
}) => {
  const [subTab, setSubTab] = useState<'incidents' | 'rules' | 'notifications'>('incidents');
  const [severityFilter, setSeverityFilter] = useState<string>('ALL');
  const [statusFilter, setStatusFilter] = useState<string>('ALL');
  const [searchQuery, setSearchQuery] = useState<string>('');

  // Selected Alarm Modal
  const [selectedAlarm, setSelectedAlarm] = useState<Alarm | null>(null);

  // Stage 5 Alarm Rules State
  const [rules, setRules] = useState<AlarmRule[]>([]);
  const [rulesLoading, setRulesLoading] = useState(false);
  const [showCreateRuleModal, setShowCreateRuleModal] = useState(false);
  const [newRuleName, setNewRuleName] = useState('');
  const [newRuleAlarmType, setNewRuleAlarmType] = useState('High Temperature');
  const [newRuleSeverity, setNewRuleSeverity] = useState('CRITICAL');
  const [newRuleCondType, setNewRuleCondType] = useState<'SIMPLE' | 'DURATION' | 'REPEATING'>('SIMPLE');
  const [newRuleMetricKey, setNewRuleMetricKey] = useState('temperature');
  const [newRuleOperator, setNewRuleOperator] = useState<'GREATER' | 'LESS' | 'EQUAL'>('GREATER');
  const [newRuleThreshold, setNewRuleThreshold] = useState('45');
  const [newRuleDuration, setNewRuleDuration] = useState('60'); // seconds
  const [newRuleDetails, setNewRuleDetails] = useState('Temperature exceeded threshold');

  // Simulation / Test on History Modal
  const [showSimModal, setShowSimModal] = useState(false);
  const [simRule, setSimRule] = useState<AlarmRule | null>(null);
  const [simSeries, setSimSeries] = useState<string>('30, 35, 42, 46, 48, 51, 38, 28');
  const [simLoading, setSimLoading] = useState(false);
  const [simResult, setSimResult] = useState<AlarmRuleTestResponse | null>(null);

  // Stage 5 Notification Targets & Rules
  const [targets, setTargets] = useState<NotificationTarget[]>([]);
  const [notifRules, setNotifRules] = useState<NotificationRule[]>([]);
  const [_notifLoading, setNotifLoading] = useState(false);
  const [testEmailSending, setTestEmailSending] = useState(false);
  const [testEmailSuccess, setTestEmailSuccess] = useState(false);

  useEffect(() => {
    if (subTab === 'rules') {
      loadRules();
    } else if (subTab === 'notifications') {
      loadNotifications();
    }
  }, [subTab]);

  const loadRules = async () => {
    setRulesLoading(true);
    try {
      const data = await api.listAlarmRules();
      setRules(data);
    } catch (err) {
      console.error('Failed to load alarm rules', err);
    } finally {
      setRulesLoading(false);
    }
  };

  const loadNotifications = async () => {
    setNotifLoading(true);
    try {
      const [tList, rList] = await Promise.all([
        api.listNotificationTargets(),
        api.listNotificationRules(),
      ]);
      setTargets(tList);
      setNotifRules(rList);
    } catch (err) {
      console.error('Failed to load notifications data', err);
    } finally {
      setNotifLoading(false);
    }
  };

  const handleCreateRule = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newRuleName.trim()) return;

    try {
      const parsedThreshold = parseFloat(newRuleThreshold) || 0;
      const parsedDurationMs = (parseInt(newRuleDuration, 10) || 60) * 1000;

      const rulePayload: Partial<AlarmRule> = {
        name: newRuleName,
        scope: 'DEVICE_PROFILE',
        alarm_type: newRuleAlarmType,
        enabled: true,
        create_rules: {
          [newRuleSeverity]: {
            condition: {
              type: newRuleCondType,
              duration_ms: newRuleCondType === 'DURATION' ? parsedDurationMs : undefined,
              count: newRuleCondType === 'REPEATING' ? 3 : undefined,
              filters: [
                {
                  key: newRuleMetricKey,
                  value_type: 'NUMERIC',
                  operator: newRuleOperator as any,
                  value: parsedThreshold,
                },
              ],
            },
            details: newRuleDetails,
          },
        },
        clear_rule: {
          condition: {
            type: 'SIMPLE',
            filters: [
              {
                key: newRuleMetricKey,
                value_type: 'NUMERIC',
                operator: newRuleOperator === 'GREATER' ? 'LESS_OR_EQUAL' : 'GREATER_OR_EQUAL',
                value: parsedThreshold,
              },
            ],
          },
          details: 'Metric normalized',
        },
      };

      await api.saveAlarmRule(rulePayload);
      setShowCreateRuleModal(false);
      setNewRuleName('');
      loadRules();
    } catch (err) {
      console.error('Failed to create alarm rule', err);
      alert('Failed to save alarm rule');
    }
  };

  const handleDeleteRule = async (id: string) => {
    if (!confirm('Are you sure you want to delete this alarm rule?')) return;
    try {
      await api.deleteAlarmRule(id);
      loadRules();
    } catch (err) {
      console.error('Failed to delete rule', err);
    }
  };

  const handleRunSimulation = async () => {
    if (!simRule) return;
    setSimLoading(true);

    try {
      const values = simSeries
        .split(',')
        .map(v => parseFloat(v.trim()))
        .filter(v => !isNaN(v));

      const now = Date.now();
      const dataPoints = values.map((val, idx) => ({
        timestamp: now + idx * 10000,
        values: {
          [newRuleMetricKey]: val,
          temperature: val,
          pressure: val,
        },
      }));

      const req: AlarmRuleTestRequest = {
        rule: simRule,
        data_points: dataPoints,
      };

      const resp = await api.testAlarmRule(req);
      setSimResult(resp);
    } catch (err) {
      console.error('Simulation failed', err);
    } finally {
      setSimLoading(false);
    }
  };

  const handleSendTestAlert = async () => {
    setTestEmailSending(true);
    setTestEmailSuccess(false);
    try {
      const emailTarget = targets.find(t => t.type === 'EMAIL');
      const targetIds = emailTarget ? [emailTarget.id] : targets.map(t => t.id);

      await api.sendNotification({
        targets: targetIds,
        subject: '[TEST CRITICAL] High Temperature Simulation on Boiler 101',
        body: 'Alert dispatched to local Mailpit relay on port 1025. Please check the Mailpit Web UI at http://localhost:8025 to verify delivery!',
      });
      setTestEmailSuccess(true);
      setTimeout(() => setTestEmailSuccess(false), 5000);
    } catch (err) {
      console.error('Failed to send test alert', err);
      alert('Failed to send test alert');
    } finally {
      setTestEmailSending(false);
    }
  };

  // Stats calculation
  const totalAlarms = alarms.length;
  const activeUnackCount = alarms.filter(a => a.status === 'ACTIVE_UNACK').length;
  const criticalCount = alarms.filter(a => a.severity === 'CRITICAL').length;
  const ackedCount = alarms.filter(a => a.status.includes('ACK')).length;

  // Filtered alarms
  const filteredAlarms = alarms.filter(a => {
    if (severityFilter !== 'ALL' && a.severity !== severityFilter) return false;
    if (statusFilter !== 'ALL') {
      if (statusFilter === 'ACTIVE_UNACK' && a.status !== 'ACTIVE_UNACK') return false;
      if (statusFilter === 'ACTIVE_ACK' && a.status !== 'ACTIVE_ACK') return false;
      if (statusFilter === 'CLEARED' && !a.status.startsWith('CLEARED')) return false;
    }
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      return (
        a.type.toLowerCase().includes(q) ||
        a.originator_id.toLowerCase().includes(q) ||
        a.severity.toLowerCase().includes(q)
      );
    }
    return true;
  });

  return (
    <div className="alarms-container">
      {/* Top Header */}
      <div className="page-header">
        <div>
          <h2 className="page-title">Alarms & Notification Center</h2>
          <p className="page-subtitle">
            Stage 5 Engine: Declarative Alarm Rules, Condition Evaluator, Escalation Chains, and Mailpit Integration
          </p>
        </div>
        <div style={{ display: 'flex', gap: '0.75rem' }}>
          <button className="btn btn-secondary" onClick={onRefresh}>
            <RefreshCw size={15} />
            <span>Refresh</span>
          </button>
          {subTab === 'rules' && (
            <button className="btn btn-primary" onClick={() => setShowCreateRuleModal(true)}>
              <Plus size={15} />
              <span>New Alarm Rule</span>
            </button>
          )}
          {subTab === 'notifications' && (
            <button
              className="btn btn-primary"
              onClick={handleSendTestAlert}
              disabled={testEmailSending}
            >
              <Send size={15} />
              <span>{testEmailSending ? 'Dispatching...' : 'Dispatch Test Alert'}</span>
            </button>
          )}
        </div>
      </div>

      {testEmailSuccess && (
        <div style={{
          background: 'rgba(16, 185, 129, 0.15)',
          border: '1px solid rgba(16, 185, 129, 0.3)',
          color: 'var(--accent-emerald)',
          padding: '0.75rem 1.25rem',
          borderRadius: 'var(--radius-md)',
          display: 'flex',
          alignItems: 'center',
          gap: '0.75rem',
          fontSize: '0.9rem'
        }}>
          <CheckCircle2 size={18} />
          <span>Test email dispatched to Mailpit (127.0.0.1:1025)! Open <a href="http://localhost:8025" target="_blank" rel="noreferrer" style={{ color: '#fff', textDecoration: 'underline' }}>Mailpit Web UI (http://localhost:8025)</a> to inspect the message.</span>
        </div>
      )}

      {/* Metric Stat Cards */}
      <div className="alarms-stats-grid">
        <div className="alarm-stat-card">
          <div className="alarm-stat-icon" style={{ background: 'rgba(244, 63, 94, 0.15)', color: 'var(--accent-rose)' }}>
            <AlertTriangle size={22} />
          </div>
          <div className="alarm-stat-content">
            <h4>Active Unack</h4>
            <div className="stat-val" style={{ color: activeUnackCount > 0 ? 'var(--accent-rose)' : 'inherit' }}>
              {activeUnackCount}
            </div>
          </div>
        </div>

        <div className="alarm-stat-card">
          <div className="alarm-stat-icon" style={{ background: 'rgba(239, 68, 68, 0.15)', color: '#f87171' }}>
            <Sliders size={22} />
          </div>
          <div className="alarm-stat-content">
            <h4>Critical Severities</h4>
            <div className="stat-val">{criticalCount}</div>
          </div>
        </div>

        <div className="alarm-stat-card">
          <div className="alarm-stat-icon" style={{ background: 'rgba(56, 189, 248, 0.15)', color: 'var(--accent-cyan)' }}>
            <CheckCircle2 size={22} />
          </div>
          <div className="alarm-stat-content">
            <h4>Acknowledged</h4>
            <div className="stat-val">{ackedCount}</div>
          </div>
        </div>

        <div className="alarm-stat-card">
          <div className="alarm-stat-icon" style={{ background: 'rgba(124, 58, 237, 0.15)', color: 'var(--accent-violet)' }}>
            <Bell size={22} />
          </div>
          <div className="alarm-stat-content">
            <h4>Total Recorded</h4>
            <div className="stat-val">{totalAlarms}</div>
          </div>
        </div>
      </div>

      {/* Sub-tab Navigation */}
      <div className="alarms-subtabs">
        <button
          className={`alarms-tab-btn ${subTab === 'incidents' ? 'active' : ''}`}
          onClick={() => setSubTab('incidents')}
        >
          <AlertTriangle size={16} />
          <span>Active Incidents & Journal</span>
        </button>
        <button
          className={`alarms-tab-btn ${subTab === 'rules' ? 'active' : ''}`}
          onClick={() => setSubTab('rules')}
        >
          <Sliders size={16} />
          <span>Alarm Rules Studio</span>
        </button>
        <button
          className={`alarms-tab-btn ${subTab === 'notifications' ? 'active' : ''}`}
          onClick={() => setSubTab('notifications')}
        >
          <Bell size={16} />
          <span>Notification Center & Relays</span>
        </button>
      </div>

      {/* SUBTAB 1: INCIDENTS JOURNAL */}
      {subTab === 'incidents' && (
        <>
          <div className="alarms-filter-bar">
            <div className="filter-group">
              <Search size={16} color="var(--text-muted)" />
              <input
                type="text"
                placeholder="Filter alarms by type, originator..."
                className="filter-select"
                style={{ width: '260px' }}
                value={searchQuery}
                onChange={e => setSearchQuery(e.target.value)}
              />
            </div>

            <div className="filter-group">
              <label style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>Severity:</label>
              <select
                className="filter-select"
                value={severityFilter}
                onChange={e => setSeverityFilter(e.target.value)}
              >
                <option value="ALL">All Severities</option>
                <option value="CRITICAL">Critical</option>
                <option value="MAJOR">Major</option>
                <option value="MINOR">Minor</option>
                <option value="WARNING">Warning</option>
              </select>

              <label style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', marginLeft: '0.5rem' }}>Status:</label>
              <select
                className="filter-select"
                value={statusFilter}
                onChange={e => setStatusFilter(e.target.value)}
              >
                <option value="ALL">All Statuses</option>
                <option value="ACTIVE_UNACK">Active & Unack</option>
                <option value="ACTIVE_ACK">Active & Acknowledged</option>
                <option value="CLEARED">Cleared</option>
              </select>
            </div>
          </div>

          <div className="table-container">
            <table className="table">
              <thead>
                <tr>
                  <th>Severity</th>
                  <th>Alarm Type</th>
                  <th>Originator Device</th>
                  <th>Status</th>
                  <th>Trigger Time</th>
                  <th style={{ textAlign: 'right' }}>Actions</th>
                </tr>
              </thead>
              <tbody>
                {filteredAlarms.length === 0 ? (
                  <tr>
                    <td colSpan={6} style={{ textAlign: 'center', padding: '48px', color: 'var(--text-muted)' }}>
                      No alarms matching current criteria. All telemetry parameters are nominal.
                    </td>
                  </tr>
                ) : (
                  filteredAlarms.map(al => {
                    const isAcked = al.status.includes('ACK');
                    const isCleared = al.status.startsWith('CLEARED');

                    return (
                      <tr key={al.id}>
                        <td>
                          <span className={`severity-pill ${al.severity}`}>
                            <AlertTriangle size={12} />
                            {al.severity}
                          </span>
                        </td>
                        <td>
                          <div style={{ fontWeight: 600, color: '#fff' }}>{al.type}</div>
                          {al.details && (
                            <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                              {(al.details as any).message ||
                                (al.details as any).reason ||
                                (typeof al.details === 'string' ? al.details : JSON.stringify(al.details))}
                            </div>
                          )}
                        </td>
                        <td className="font-mono" style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                          {al.originator_id}
                        </td>
                        <td>
                          <span className={`status-pill ${al.status}`}>{al.status}</span>
                        </td>
                        <td style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                          {new Date(al.created_time || al.start_ts).toLocaleString()}
                        </td>
                        <td style={{ textAlign: 'right' }}>
                          <div style={{ display: 'inline-flex', gap: '0.4rem' }}>
                            <button
                              className="btn btn-secondary"
                              style={{ padding: '0.25rem 0.6rem', fontSize: '0.75rem' }}
                              onClick={() => setSelectedAlarm(al)}
                            >
                              <Info size={12} />
                              <span>Details</span>
                            </button>
                            {!isAcked && !isCleared && (
                              <button
                                className="btn btn-secondary"
                                style={{ padding: '0.25rem 0.6rem', fontSize: '0.75rem', color: 'var(--accent-cyan)' }}
                                onClick={() => onAckAlarm(al.id)}
                              >
                                <span>Ack</span>
                              </button>
                            )}
                            {!isCleared && (
                              <button
                                className="btn btn-secondary"
                                style={{ padding: '0.25rem 0.6rem', fontSize: '0.75rem', color: 'var(--accent-emerald)' }}
                                onClick={() => onClearAlarm(al.id)}
                              >
                                <span>Clear</span>
                              </button>
                            )}
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

      {/* SUBTAB 2: ALARM RULES STUDIO */}
      {subTab === 'rules' && (
        <>
          <div className="rules-grid">
            {rulesLoading ? (
              <div style={{ padding: '3rem', textAlign: 'center', color: 'var(--text-muted)' }}>
                Loading alarm rules...
              </div>
            ) : rules.length === 0 ? (
              <div style={{ padding: '3rem', textAlign: 'center', color: 'var(--text-muted)' }}>
                No alarm rules configured. Click &quot;New Alarm Rule&quot; to define threshold triggers!
              </div>
            ) : (
              rules.map(rule => (
                <div key={rule.id} className="rule-card">
                  <div className="rule-header">
                    <div>
                      <h3 className="rule-title">{rule.name}</h3>
                      <div className="rule-subtitle">
                        Type: <span style={{ color: '#fff', fontWeight: 600 }}>{rule.alarm_type}</span> &middot; Scope: {rule.scope}
                      </div>
                    </div>
                    <span
                      className="badge-tag"
                      style={{
                        background: rule.enabled ? 'rgba(16, 185, 129, 0.15)' : 'rgba(255, 255, 255, 0.05)',
                        color: rule.enabled ? 'var(--accent-emerald)' : 'var(--text-muted)',
                      }}
                    >
                      {rule.enabled ? 'ACTIVE' : 'DISABLED'}
                    </span>
                  </div>

                  <div className="rule-conditions-box">
                    <div style={{ fontWeight: 600, color: 'var(--text-secondary)' }}>Create Triggers:</div>
                    {Object.entries(rule.create_rules || {}).map(([sev, def]) => (
                      <div key={sev} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                        <span className={`severity-pill ${sev}`}>{sev}</span>
                        <span style={{ color: 'var(--text-primary)' }}>
                          {def.condition.filters?.map(f => `${f.key} ${f.operator} ${f.value}`).join(' AND ')}
                        </span>
                        <span style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>
                          ({def.condition.type}{def.condition.duration_ms ? ` ${def.condition.duration_ms / 1000}s` : ''})
                        </span>
                      </div>
                    ))}

                    {rule.clear_rule && (
                      <div style={{ marginTop: '0.25rem', paddingTop: '0.25rem', borderTop: '1px solid rgba(255,255,255,0.08)' }}>
                        <span style={{ color: 'var(--accent-emerald)', fontWeight: 600 }}>Clear Condition: </span>
                        <span style={{ color: 'var(--text-secondary)' }}>
                          {rule.clear_rule.condition?.filters?.map(f => `${f.key} ${f.operator} ${f.value}`).join(' AND ') || 'Default normalization'}
                        </span>
                      </div>
                    )}
                  </div>

                  <div className="rule-actions">
                    <button
                      className="btn btn-secondary"
                      style={{ padding: '0.3rem 0.6rem', fontSize: '0.75rem' }}
                      onClick={() => {
                        setSimRule(rule);
                        setShowSimModal(true);
                      }}
                    >
                      <Play size={12} />
                      <span>Test Simulation</span>
                    </button>
                    <button
                      className="btn btn-secondary"
                      style={{ padding: '0.3rem 0.6rem', fontSize: '0.75rem', color: 'var(--accent-rose)' }}
                      onClick={() => handleDeleteRule(rule.id)}
                    >
                      <Trash2 size={12} />
                      <span>Delete</span>
                    </button>
                  </div>
                </div>
              ))
            )}
          </div>
        </>
      )}

      {/* SUBTAB 3: NOTIFICATIONS CENTER */}
      {subTab === 'notifications' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
          {/* Targets Section */}
          <div className="card">
            <h3 style={{ fontSize: '1.1rem', fontWeight: 600, color: '#fff', marginBottom: '0.5rem' }}>
              Notification Delivery Targets
            </h3>
            <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', marginBottom: '1rem' }}>
              Configured dispatch destinations across Local Mailpit SMTP (:1025), Slack Webhooks, and In-App Web Inbox.
            </p>

            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '1rem' }}>
              {targets.map(target => (
                <div
                  key={target.id}
                  style={{
                    background: 'var(--bg-primary)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: 'var(--radius-sm)',
                    padding: '1rem',
                  }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <div style={{ fontWeight: 600, color: '#fff' }}>{target.name}</div>
                    <span className="badge-tag" style={{ background: 'rgba(56, 189, 248, 0.15)', color: 'var(--accent-cyan)' }}>
                      {target.type}
                    </span>
                  </div>
                  <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginTop: '0.5rem' }}>
                    {target.type === 'EMAIL' && (
                      <div>
                        Relay: <code>127.0.0.1:1025</code> (Mailpit)
                        <br />
                        Recipients: {(target.config as any)?.emails?.join(', ') || 'alerts@iotp.local'}
                      </div>
                    )}
                    {target.type === 'PLATFORM_USERS' && 'Delivered directly to in-app user bell inbox & realtime stream'}
                    {target.type === 'SLACK' && `Webhook URL: ${(target.config as any)?.webhook_url || 'https://hooks.slack.com/...'}`}
                  </div>
                </div>
              ))}
            </div>

            <div style={{ marginTop: '1.25rem', display: 'flex', alignItems: 'center', gap: '1rem' }}>
              <a
                href="http://localhost:8025"
                target="_blank"
                rel="noreferrer"
                className="btn btn-secondary"
                style={{ fontSize: '0.85rem', display: 'inline-flex', alignItems: 'center', gap: '0.4rem' }}
              >
                <ExternalLink size={14} />
                <span>Open Mailpit Web Interface (http://localhost:8025)</span>
              </a>
            </div>
          </div>

          {/* Rules & Escalation Tiers */}
          <div className="card">
            <h3 style={{ fontSize: '1.1rem', fontWeight: 600, color: '#fff', marginBottom: '0.5rem' }}>
              Alarm Escalation Rules
            </h3>
            <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', marginBottom: '1rem' }}>
              Multi-tier escalation triggers that dispatch alerts immediately and schedule escalation tiers (cancelled automatically if acknowledged or cleared).
            </p>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
              {notifRules.map(nr => (
                <div
                  key={nr.id}
                  style={{
                    background: 'var(--bg-primary)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: 'var(--radius-sm)',
                    padding: '1rem',
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                  }}
                >
                  <div>
                    <div style={{ fontWeight: 600, color: '#fff' }}>{nr.name}</div>
                    <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginTop: '0.2rem' }}>
                      Trigger: <span style={{ color: 'var(--accent-rose)' }}>{nr.trigger_type}</span> &middot; Escalation Steps: {nr.recipients_config?.length || 1}
                    </div>
                  </div>
                  <div style={{ display: 'flex', gap: '0.5rem' }}>
                    {nr.recipients_config?.map((step, idx) => (
                      <span
                        key={idx}
                        className="badge-tag"
                        style={{ background: 'rgba(124, 58, 237, 0.15)', color: 'var(--accent-violet)' }}
                      >
                        Tier {step.step}: {step.delay_seconds === 0 ? 'Immediate' : `+${step.delay_seconds}s`}
                      </span>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* MODAL 1: CREATE ALARM RULE */}
      {showCreateRuleModal && (
        <div className="modal-overlay" onClick={() => setShowCreateRuleModal(false)}>
          <div className="modal-content" onClick={e => e.stopPropagation()} style={{ maxWidth: '580px' }}>
            <h3 style={{ margin: '0 0 1rem 0', color: '#fff', fontSize: '1.2rem' }}>Define Declarative Alarm Rule</h3>
            <form onSubmit={handleCreateRule} style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
              <div className="form-group">
                <label>Rule Name</label>
                <input
                  type="text"
                  className="form-input"
                  placeholder="e.g. Critical Boiler Overheating"
                  value={newRuleName}
                  onChange={e => setNewRuleName(e.target.value)}
                  required
                />
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
                <div className="form-group">
                  <label>Alarm Type</label>
                  <input
                    type="text"
                    className="form-input"
                    value={newRuleAlarmType}
                    onChange={e => setNewRuleAlarmType(e.target.value)}
                    required
                  />
                </div>
                <div className="form-group">
                  <label>Severity Level</label>
                  <select
                    className="form-input"
                    value={newRuleSeverity}
                    onChange={e => setNewRuleSeverity(e.target.value)}
                  >
                    <option value="CRITICAL">CRITICAL</option>
                    <option value="MAJOR">MAJOR</option>
                    <option value="MINOR">MINOR</option>
                    <option value="WARNING">WARNING</option>
                  </select>
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '0.75rem' }}>
                <div className="form-group">
                  <label>Metric Key</label>
                  <input
                    type="text"
                    className="form-input"
                    value={newRuleMetricKey}
                    onChange={e => setNewRuleMetricKey(e.target.value)}
                    required
                  />
                </div>
                <div className="form-group">
                  <label>Operator</label>
                  <select
                    className="form-input"
                    value={newRuleOperator}
                    onChange={e => setNewRuleOperator(e.target.value as any)}
                  >
                    <option value="GREATER">&gt; (Greater than)</option>
                    <option value="LESS">&lt; (Less than)</option>
                    <option value="EQUAL">== (Equals)</option>
                  </select>
                </div>
                <div className="form-group">
                  <label>Threshold Value</label>
                  <input
                    type="number"
                    step="any"
                    className="form-input"
                    value={newRuleThreshold}
                    onChange={e => setNewRuleThreshold(e.target.value)}
                    required
                  />
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
                <div className="form-group">
                  <label>Temporal Criteria</label>
                  <select
                    className="form-input"
                    value={newRuleCondType}
                    onChange={e => setNewRuleCondType(e.target.value as any)}
                  >
                    <option value="SIMPLE">SIMPLE (Immediate on Match)</option>
                    <option value="DURATION">DURATION (True for N seconds)</option>
                    <option value="REPEATING">REPEATING (N consecutive matches)</option>
                  </select>
                </div>

                {newRuleCondType === 'DURATION' && (
                  <div className="form-group">
                    <label>Duration Window (Seconds)</label>
                    <input
                      type="number"
                      className="form-input"
                      value={newRuleDuration}
                      onChange={e => setNewRuleDuration(e.target.value)}
                    />
                  </div>
                )}
              </div>

              <div className="form-group">
                <label>Alarm Details Message</label>
                <input
                  type="text"
                  className="form-input"
                  value={newRuleDetails}
                  onChange={e => setNewRuleDetails(e.target.value)}
                />
              </div>

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.75rem', marginTop: '1rem' }}>
                <button type="button" className="btn btn-secondary" onClick={() => setShowCreateRuleModal(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary">
                  Save Alarm Rule
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL 2: SIMULATION / TEST ON HISTORY */}
      {showSimModal && simRule && (
        <div className="modal-overlay" onClick={() => setShowSimModal(false)}>
          <div className="modal-content" onClick={e => e.stopPropagation()} style={{ maxWidth: '680px' }}>
            <h3 style={{ margin: '0 0 0.5rem 0', color: '#fff', fontSize: '1.2rem' }}>
              Test Rule on History / Simulation
            </h3>
            <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', marginBottom: '1rem' }}>
              Evaluate <strong>{simRule.name}</strong> across a simulated sequence of readings to test trigger & clear actions.
            </p>

            <div className="form-group">
              <label>Sequence of Metric Values (Comma Separated)</label>
              <input
                type="text"
                className="form-input"
                value={simSeries}
                onChange={e => setSimSeries(e.target.value)}
              />
            </div>

            <button
              className="btn btn-primary"
              style={{ marginTop: '0.5rem' }}
              onClick={handleRunSimulation}
              disabled={simLoading}
            >
              <Play size={14} />
              <span>{simLoading ? 'Running Simulation...' : 'Run Simulation Trace'}</span>
            </button>

            {simResult && (
              <div className="trace-table-container">
                <table className="table" style={{ fontSize: '0.8rem' }}>
                  <thead>
                    <tr>
                      <th>Step</th>
                      <th>Action</th>
                      <th>Severity</th>
                      <th>Trace Explanation</th>
                    </tr>
                  </thead>
                  <tbody>
                    {simResult.traces.map((trace, idx) => (
                      <tr key={idx} className={`trace-row ${trace.action}`}>
                        <td>Step {idx + 1}</td>
                        <td>
                          <span className={`trace-action-badge ${trace.action}`}>{trace.action}</span>
                        </td>
                        <td>
                          {trace.severity ? (
                            <span className={`severity-pill ${trace.severity}`}>{trace.severity}</span>
                          ) : (
                            '-'
                          )}
                        </td>
                        <td style={{ color: 'var(--text-secondary)' }}>{trace.details}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}

            <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '1.5rem' }}>
              <button className="btn btn-secondary" onClick={() => setShowSimModal(false)}>
                Close
              </button>
            </div>
          </div>
        </div>
      )}

      {/* MODAL 3: ALARM DETAILS INSPECTOR */}
      {selectedAlarm && (
        <div className="modal-overlay" onClick={() => setSelectedAlarm(null)}>
          <div className="modal-content" onClick={e => e.stopPropagation()} style={{ maxWidth: '540px' }}>
            <h3 style={{ margin: '0 0 0.5rem 0', color: '#fff', fontSize: '1.2rem' }}>
              Alarm Incident Inspector
            </h3>
            <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem' }}>
              <span className={`severity-pill ${selectedAlarm.severity}`}>{selectedAlarm.severity}</span>
              <span className={`status-pill ${selectedAlarm.status}`}>{selectedAlarm.status}</span>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem', fontSize: '0.85rem' }}>
              <div>
                <span style={{ color: 'var(--text-muted)' }}>Alarm Type: </span>
                <strong style={{ color: '#fff' }}>{selectedAlarm.type}</strong>
              </div>
              <div>
                <span style={{ color: 'var(--text-muted)' }}>Originator Device: </span>
                <code style={{ color: 'var(--accent-cyan)' }}>{selectedAlarm.originator_id}</code>
              </div>
              <div>
                <span style={{ color: 'var(--text-muted)' }}>Trigger Timestamp: </span>
                <span>{new Date(selectedAlarm.start_ts).toLocaleString()}</span>
              </div>
              {selectedAlarm.ack_ts && (
                <div>
                  <span style={{ color: 'var(--text-muted)' }}>Acknowledged At: </span>
                  <span>{new Date(selectedAlarm.ack_ts).toLocaleString()}</span>
                </div>
              )}
              {selectedAlarm.clear_ts && (
                <div>
                  <span style={{ color: 'var(--text-muted)' }}>Cleared At: </span>
                  <span>{new Date(selectedAlarm.clear_ts).toLocaleString()}</span>
                </div>
              )}

              <div>
                <span style={{ color: 'var(--text-muted)' }}>Diagnostic Details Payload:</span>
                <pre
                  style={{
                    background: 'rgba(0,0,0,0.4)',
                    padding: '0.75rem',
                    borderRadius: 'var(--radius-sm)',
                    overflowX: 'auto',
                    fontSize: '0.8rem',
                    color: '#e2e8f0',
                    marginTop: '0.35rem',
                  }}
                >
                  {JSON.stringify(selectedAlarm.details, null, 2)}
                </pre>
              </div>
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '1.5rem', gap: '0.5rem' }}>
              {!selectedAlarm.status.includes('ACK') && !selectedAlarm.status.startsWith('CLEARED') && (
                <button
                  className="btn btn-secondary"
                  style={{ color: 'var(--accent-cyan)' }}
                  onClick={() => {
                    onAckAlarm(selectedAlarm.id);
                    setSelectedAlarm(null);
                  }}
                >
                  Acknowledge
                </button>
              )}
              {!selectedAlarm.status.startsWith('CLEARED') && (
                <button
                  className="btn btn-secondary"
                  style={{ color: 'var(--accent-emerald)' }}
                  onClick={() => {
                    onClearAlarm(selectedAlarm.id);
                    setSelectedAlarm(null);
                  }}
                >
                  Clear Incident
                </button>
              )}
              <button className="btn btn-secondary" onClick={() => setSelectedAlarm(null)}>
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
