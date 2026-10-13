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
  RefreshCw,
  Box,
  Activity,
  History,
  Plus,
  X,
  Radio,
  FileCode,
  Flame,
  Eye,
  Wrench,
  Sparkles,
  Save,
  Trash2,
  Layers
} from 'lucide-react';
import { api } from '../api';
import type { 
  ScadaTag, 
  ISA182Alarm, 
  ControlCommand, 
  AlarmKPIs, 
  ControlLogEntry, 
  ScadaSymbol 
} from '../api';
import './ScadaView.css';

export interface CanvasEquipment {
  id: string;
  name: string;
  type: 'tank' | 'pump' | 'valve' | 'meter' | 'chemical';
  tagPath: string;
  unit: string;
  minValue: number;
  maxValue: number;
  currentValue: number;
}

const TEMPLATES: Record<string, { label: string; icon: string; desc: string; items: CanvasEquipment[] }> = {
  water_treatment: {
    label: '💧 Water Treatment & Dosing',
    icon: '💧',
    desc: 'Dual-tank water buffer with centrifugal pump, modulating control valve, and inline dosing skid',
    items: [
      { id: 'Tank101', name: 'Raw Water Storage TK-101', type: 'tank', tagPath: 'WaterTreatment.Tank101.Level', unit: '%', minValue: 0, maxValue: 100, currentValue: 76.4 },
      { id: 'Pump101', name: 'Duty Centrifugal Pump 101', type: 'pump', tagPath: 'WaterTreatment.Pump101.Status', unit: 'ON/OFF', minValue: 0, maxValue: 1, currentValue: 1 },
      { id: 'FT101', name: 'Electromagnetic Flowmeter FT-101', type: 'meter', tagPath: 'WaterTreatment.Pump101.Flow', unit: 'GPM', minValue: 0, maxValue: 500, currentValue: 342.8 },
      { id: 'Valve101', name: 'Modulating Control Valve FCV-101', type: 'valve', tagPath: 'WaterTreatment.Valve101.Position', unit: '%', minValue: 0, maxValue: 100, currentValue: 100 },
      { id: 'ChemSkid', name: 'Acid Dosing Vessel TK-202', type: 'chemical', tagPath: 'WaterTreatment.Chemical.AcidLevel', unit: '%', minValue: 0, maxValue: 100, currentValue: 48.6 },
      { id: 'PT101', name: 'Discharge Pressure Transmitter PT-101', type: 'meter', tagPath: 'WaterTreatment.Discharge.Pressure', unit: 'PSI', minValue: 0, maxValue: 150, currentValue: 62.5 },
      { id: 'AT101', name: 'Effluent pH Analytical Sensor AT-101', type: 'meter', tagPath: 'WaterTreatment.Discharge.pH', unit: 'pH', minValue: 0, maxValue: 14, currentValue: 7.2 }
    ]
  },
  steam_boiler: {
    label: '🔥 Steam Boiler Station',
    icon: '🔥',
    desc: 'High-pressure steam boiler with feed pump, superheater steam valve, and pressure/temp transmitters',
    items: [
      { id: 'BoilerDrum', name: 'Boiler Steam Drum B-201', type: 'tank', tagPath: 'SteamPlant.Boiler201.DrumLevel', unit: '%', minValue: 0, maxValue: 100, currentValue: 68.5 },
      { id: 'FeedPump', name: 'High-Pressure Feedwater Pump P-201', type: 'pump', tagPath: 'SteamPlant.FeedPump.Status', unit: 'ON/OFF', minValue: 0, maxValue: 1, currentValue: 1 },
      { id: 'SteamValve', name: 'Main Steam Throttle MSV-201', type: 'valve', tagPath: 'SteamPlant.MainSteamValve.Position', unit: '%', minValue: 0, maxValue: 100, currentValue: 80 },
      { id: 'SteamPressure', name: 'Steam Header Pressure PT-201', type: 'meter', tagPath: 'SteamPlant.Header.Pressure', unit: 'bar', minValue: 0, maxValue: 40, currentValue: 18.4 },
      { id: 'SteamTemp', name: 'Superheater Temperature TT-201', type: 'meter', tagPath: 'SteamPlant.Header.Temp', unit: '°C', minValue: 0, maxValue: 500, currentValue: 285.0 }
    ]
  },
  chemical_reactor: {
    label: '🧪 Chemical Reactor Skid',
    icon: '🧪',
    desc: 'Continuous batch reactor vessel with agitator pump, reactant dosing, and pH/temperature control',
    items: [
      { id: 'ReactorVessel', name: 'Continuous Reactor Vessel RX-301', type: 'tank', tagPath: 'ChemicalSkid.Reactor301.Level', unit: '%', minValue: 0, maxValue: 100, currentValue: 54.2 },
      { id: 'AgitatorPump', name: 'Reactor Recirculation Pump P-301', type: 'pump', tagPath: 'ChemicalSkid.RecircPump.Status', unit: 'ON/OFF', minValue: 0, maxValue: 1, currentValue: 1 },
      { id: 'ReagentValve', name: 'Reagent Feed Control Valve FV-301', type: 'valve', tagPath: 'ChemicalSkid.FeedValve.Position', unit: '%', minValue: 0, maxValue: 100, currentValue: 45 },
      { id: 'ReagentTank', name: 'Catalyst Reagent Dosing TK-305', type: 'chemical', tagPath: 'ChemicalSkid.Catalyst.Level', unit: '%', minValue: 0, maxValue: 100, currentValue: 62.0 },
      { id: 'ReactorTemp', name: 'Reactor Internal Temperature TT-301', type: 'meter', tagPath: 'ChemicalSkid.Reactor.Temp', unit: '°C', minValue: 0, maxValue: 150, currentValue: 72.4 },
      { id: 'ReactorPH', name: 'Effluent pH Transmitter AT-301', type: 'meter', tagPath: 'ChemicalSkid.Reactor.pH', unit: 'pH', minValue: 0, maxValue: 14, currentValue: 6.8 }
    ]
  },
  blank: {
    label: '✨ Blank Canvas (Custom)',
    icon: '✨',
    desc: 'Build your own custom industrial process synoptic screen from scratch using the equipment palette',
    items: []
  }
};

type ActiveTab = 'synoptic' | 'alarms' | 'tags' | 'symbols' | 'sbo';

export function ScadaView() {
  const [tab, setTab] = useState<ActiveTab>('synoptic');
  const [canvasMode, setCanvasMode] = useState<'monitor' | 'builder'>('monitor');
  const [currentTemplate, setCurrentTemplate] = useState<string>('water_treatment');
  const [canvasEquipment, setCanvasEquipment] = useState<CanvasEquipment[]>(() => {
    const saved = localStorage.getItem('scada_canvas_equipment_v2');
    if (saved) {
      try {
        const parsed = JSON.parse(saved);
        if (Array.isArray(parsed) && parsed.length > 0) return parsed;
      } catch {}
    }
    return TEMPLATES.water_treatment.items;
  });
  const [saveStatus, setSaveStatus] = useState<string>('');

  const [tags, setTags] = useState<ScadaTag[]>([]);
  const [alarms, setAlarms] = useState<ISA182Alarm[]>([]);
  const [kpis, setKpis] = useState<AlarmKPIs | null>(null);
  const [symbols, setSymbols] = useState<ScadaSymbol[]>([]);
  const [controlLogs, setControlLogs] = useState<ControlLogEntry[]>([]);
  const [loading, setLoading] = useState(false);

  // Search & Filters
  const [tagSearch, setTagSearch] = useState('');
  const [symbolCategory, setSymbolCategory] = useState('');

  // SBO State
  const [sboTargetTag, setSboTargetTag] = useState('WaterTreatment.Pump101.Status');
  const [sboDesiredValue, setSboDesiredValue] = useState<number>(1);
  const [activeCommand, setActiveCommand] = useState<ControlCommand | null>(null);
  const [sboTimer, setSboTimer] = useState<number>(0);
  const [operating, setOperating] = useState(false);

  // Simulation inputs
  const [simLevel, setSimLevel] = useState<number>(76.4);
  const [simPressure, setSimPressure] = useState<number>(62.5);
  const [simPH, setSimPH] = useState<number>(7.2);

  // Modals
  const [faceplateItem, setFaceplateItem] = useState<{ id: string; title: string; type: string } | null>(null);
  const [inspectedSymbol, setInspectedSymbol] = useState<ScadaSymbol | null>(null);
  const [showNewTagModal, setShowNewTagModal] = useState(false);
  const [newTagForm, setNewTagForm] = useState({
    path: '',
    data_type: 'ANALOG',
    eng_unit: '',
    deadband: 0.5,
    value: 50.0
  });

  // Symbol interactive preview states (symbolId -> { running, alarm })
  const [symbolStates, setSymbolStates] = useState<Record<string, { running: boolean; alarm: boolean }>>({});

  useEffect(() => {
    loadData();
    const interval = setInterval(loadData, 3000);
    return () => clearInterval(interval);
  }, [symbolCategory]);

  useEffect(() => {
    if (activeCommand?.status === 'SELECTED' && sboTimer > 0) {
      const t = setTimeout(() => setSboTimer(sboTimer - 1), 1000);
      return () => clearTimeout(t);
    }
  }, [activeCommand, sboTimer]);

  const loadData = async () => {
    setLoading(true);
    try {
      const [tagList, alarmList, kpiData, symbolList, logsList] = await Promise.all([
        api.listScadaTags(),
        api.listScadaAlarms(),
        api.getScadaAlarmKPIs().catch(() => null),
        api.listScadaSymbols(symbolCategory).catch(() => []),
        api.listScadaControlLogs(30).catch(() => [])
      ]);

      const safeTags = Array.isArray(tagList) ? tagList : [];
      setTags(safeTags);
      setAlarms(Array.isArray(alarmList) ? alarmList : []);
      if (kpiData) setKpis(kpiData);
      setSymbols(Array.isArray(symbolList) ? symbolList : []);
      setControlLogs(Array.isArray(logsList) ? logsList : []);

      const tankTag = safeTags.find(t => t.path === 'WaterTreatment.Tank101.Level');
      if (tankTag && typeof tankTag.value === 'number') {
        setSimLevel(tankTag.value);
      }
      const pressTag = safeTags.find(t => t.path === 'WaterTreatment.Discharge.Pressure');
      if (pressTag && typeof pressTag.value === 'number') {
        setSimPressure(pressTag.value);
      }
      const pHTag = safeTags.find(t => t.path === 'WaterTreatment.Discharge.pH');
      if (pHTag && typeof pHTag.value === 'number') {
        setSimPH(pHTag.value);
      }
    } catch (e) {
      console.error('SCADA load error:', e);
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

  const handleSelectSBO = async (tagPath = sboTargetTag, desiredVal = sboDesiredValue) => {
    try {
      const cmd = await api.selectScadaControl(tagPath, desiredVal);
      setActiveCommand(cmd);
      setSboTimer(60);
      await loadData();
    } catch (e: any) {
      alert('SBO Select reservation failed: ' + e.message);
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

  const handleShelveAlarm = async (id: string, durationMs = 300000) => {
    try {
      await api.shelveScadaAlarm(id, durationMs);
      await loadData();
    } catch (e: any) {
      alert('Shelve failed: ' + e.message);
    }
  };

  const handleUnshelveAlarm = async (id: string) => {
    try {
      await api.unshelveScadaAlarm(id);
      await loadData();
    } catch (e: any) {
      alert('Unshelve failed: ' + e.message);
    }
  };

  const handleCreateTag = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newTagForm.path) return;
    try {
      await api.updateScadaTag(newTagForm.path, newTagForm.value);
      setShowNewTagModal(false);
      setNewTagForm({ path: '', data_type: 'ANALOG', eng_unit: '', deadband: 0.5, value: 50.0 });
      await loadData();
    } catch (err: any) {
      alert('Failed to register tag: ' + err.message);
    }
  };

  // Studio Builder Handlers
  const handleSelectTemplate = (tplKey: string) => {
    setCurrentTemplate(tplKey);
    const items = TEMPLATES[tplKey]?.items || [];
    setCanvasEquipment([...items]);
    localStorage.setItem('scada_canvas_equipment_v2', JSON.stringify(items));
    setSaveStatus(`Loaded "${TEMPLATES[tplKey]?.label || tplKey}" template`);
    setTimeout(() => setSaveStatus(''), 3000);
  };

  const handleAddEquipment = (type: 'tank' | 'pump' | 'valve' | 'meter' | 'chemical') => {
    const count = canvasEquipment.filter(e => e.type === type).length + 1;
    const newId = `${type.charAt(0).toUpperCase() + type.slice(1)}_${Date.now().toString().slice(-4)}`;
    const defaultNames: Record<string, string> = {
      tank: `Process Vessel TK-${100 + count}`,
      pump: `Centrifugal Pump P-${100 + count}`,
      valve: `Control Valve FCV-${100 + count}`,
      meter: `Transmitter Sensor TT-${100 + count}`,
      chemical: `Dosing Vessel TK-${200 + count}`
    };
    const defaultUnits: Record<string, string> = {
      tank: '%',
      pump: 'ON/OFF',
      valve: '%',
      meter: 'PSI',
      chemical: '%'
    };
    const newItem: CanvasEquipment = {
      id: newId,
      name: defaultNames[type] || `Equipment ${newId}`,
      type,
      tagPath: `Plant.Area1.${newId}.Value`,
      unit: defaultUnits[type] || 'EU',
      minValue: 0,
      maxValue: 100,
      currentValue: type === 'pump' ? 1 : 50
    };
    const updated = [...canvasEquipment, newItem];
    setCanvasEquipment(updated);
    localStorage.setItem('scada_canvas_equipment_v2', JSON.stringify(updated));
    setSaveStatus(`Added ${newItem.name} to canvas`);
    setTimeout(() => setSaveStatus(''), 3000);
  };

  const handleUpdateEquipment = (id: string, patch: Partial<CanvasEquipment>) => {
    const updated = canvasEquipment.map(item => item.id === id ? { ...item, ...patch } : item);
    setCanvasEquipment(updated);
  };

  const handleRemoveEquipment = (id: string) => {
    const updated = canvasEquipment.filter(item => item.id !== id);
    setCanvasEquipment(updated);
    localStorage.setItem('scada_canvas_equipment_v2', JSON.stringify(updated));
    setSaveStatus('Equipment removed from canvas');
    setTimeout(() => setSaveStatus(''), 3000);
  };

  const handleSaveLayout = () => {
    localStorage.setItem('scada_canvas_equipment_v2', JSON.stringify(canvasEquipment));
    setSaveStatus('Canvas layout saved successfully!');
    setTimeout(() => setSaveStatus(''), 3000);
  };

  const handleResetLayout = () => {
    const items = TEMPLATES[currentTemplate]?.items || TEMPLATES.water_treatment.items;
    setCanvasEquipment([...items]);
    localStorage.setItem('scada_canvas_equipment_v2', JSON.stringify(items));
    setSaveStatus('Reset to template defaults');
    setTimeout(() => setSaveStatus(''), 3000);
  };

  const getEquipmentLiveValue = (item: CanvasEquipment): number => {
    const found = tags.find(t => t.path === item.tagPath);
    if (found && typeof found.value === 'number') return found.value;
    if (item.tagPath === 'WaterTreatment.Tank101.Level') return simLevel;
    if (item.tagPath === 'WaterTreatment.Discharge.Pressure') return simPressure;
    if (item.tagPath === 'WaterTreatment.Discharge.pH') return simPH;
    return item.currentValue;
  };

  // Synoptic Tag Resolvers
  const tankLevelTag = tags.find(t => t.path === 'WaterTreatment.Tank101.Level');
  const pumpStatusTag = tags.find(t => t.path === 'WaterTreatment.Pump101.Status');
  const flowTag = tags.find(t => t.path === 'WaterTreatment.Pump101.Flow');
  const pressureTag = tags.find(t => t.path === 'WaterTreatment.Discharge.Pressure');
  const valveTag = tags.find(t => t.path === 'WaterTreatment.Valve101.Position');
  const chemAcidTag = tags.find(t => t.path === 'WaterTreatment.Chemical.AcidLevel');
  const chemSpeedTag = tags.find(t => t.path === 'WaterTreatment.Chemical.DosingSpeed');
  const dischargePHTag = tags.find(t => t.path === 'WaterTreatment.Discharge.pH');

  const tankLevel = typeof tankLevelTag?.value === 'number' ? tankLevelTag.value : simLevel;
  const isPumpRunning = pumpStatusTag?.value === 1 || pumpStatusTag?.value === true;
  const flowVal = typeof flowTag?.value === 'number' ? flowTag.value : 342.8;
  const pressureVal = typeof pressureTag?.value === 'number' ? pressureTag.value : simPressure;
  const valvePosition = typeof valveTag?.value === 'number' ? valveTag.value : 100.0;
  const acidLevel = typeof chemAcidTag?.value === 'number' ? chemAcidTag.value : 48.6;
  const dosingSpeed = typeof chemSpeedTag?.value === 'number' ? chemSpeedTag.value : 1150.0;
  const dischargePH = typeof dischargePHTag?.value === 'number' ? dischargePHTag.value : simPH;

  const activeAlarmsCount = alarms.filter(a => a.state !== 'NORMAL').length;
  const unackAlarmsCount = alarms.filter(a => a.state === 'UNACK_ALARM').length;

  const filteredTags = tags.filter(t => 
    !tagSearch || t.path.toLowerCase().includes(tagSearch.toLowerCase()) || 
    (t.eng_unit && t.eng_unit.toLowerCase().includes(tagSearch.toLowerCase()))
  );

  return (
    <div className="scada-container">
      {/* Header */}
      <div className="page-header">
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <h2 className="page-title">Industrial SCADA & ThingsBoard Studio</h2>
            <span className="badge-tag live" style={{ background: 'rgba(6, 182, 212, 0.2)', color: 'var(--accent-cyan)' }}>
              ThingsBoard 3.8+ Parity
            </span>
          </div>
          <p className="page-subtitle">
            Hierarchical Tag Namespace · ISA-18.2 Process Alarms · SBO Safe Controls · ThingsBoard SVG Symbols
          </p>
        </div>
        <div style={{ display: 'flex', gap: '10px' }}>
          <button className="btn btn-secondary" onClick={loadData} disabled={loading}>
            <RefreshCw className={loading ? 'animate-spin' : ''} size={15} />
            <span>Refresh</span>
          </button>
          <button className="btn btn-primary" onClick={() => setShowNewTagModal(true)}>
            <Plus size={15} />
            <span>New Tag</span>
          </button>
        </div>
      </div>

      {/* 3-Step Guide & SCADA Mode Switcher */}
      <div className="scada-guide-banner">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '10px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <Sparkles size={18} style={{ color: 'var(--accent-cyan)' }} />
            <strong style={{ color: '#fff', fontSize: '0.95rem' }}>How to Build & Run Your Own SCADA in 3 Steps:</strong>
          </div>
          <div className="scada-mode-toggle">
            <button 
              className={canvasMode === 'monitor' ? 'active' : ''} 
              onClick={() => { setCanvasMode('monitor'); setTab('synoptic'); }}
            >
              <Eye size={14} /> Live Monitor
            </button>
            <button 
              className={canvasMode === 'builder' ? 'active' : ''} 
              onClick={() => { setCanvasMode('builder'); setTab('synoptic'); }}
            >
              <Wrench size={14} /> Studio Builder
            </button>
          </div>
        </div>
        <div className="scada-steps-grid">
          <div className="scada-step-card">
            <div className="scada-step-num">1</div>
            <div>
              <div style={{ fontWeight: 700, fontSize: '0.85rem', color: '#fff', marginBottom: '2px' }}>1. Define Tags</div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', lineHeight: 1.4 }}>
                In <strong>Tag Namespace</strong>, register sensors & actuators (or map to device telemetry).
              </div>
            </div>
          </div>
          <div className="scada-step-card">
            <div className="scada-step-num">2</div>
            <div>
              <div style={{ fontWeight: 700, fontSize: '0.85rem', color: '#fff', marginBottom: '2px' }}>2. Assemble Canvas</div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', lineHeight: 1.4 }}>
                In <strong>Studio Builder</strong>, pick templates or drop Tanks, Pumps, Valves & Meters.
              </div>
            </div>
          </div>
          <div className="scada-step-card">
            <div className="scada-step-num">3</div>
            <div>
              <div style={{ fontWeight: 700, fontSize: '0.85rem', color: '#fff', marginBottom: '2px' }}>3. Bind & Monitor</div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', lineHeight: 1.4 }}>
                Bind equipment to tags. In <strong>Live Monitor</strong>, click equipment for faceplates & SBO.
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="scada-tabs">
        <button 
          className={`scada-tab-btn ${tab === 'synoptic' ? 'active' : ''}`}
          onClick={() => setTab('synoptic')}
        >
          <Gauge size={16} />
          <span>Process Synoptic HMI</span>
          <span className="scada-tab-badge">Live</span>
        </button>
        <button 
          className={`scada-tab-btn ${tab === 'alarms' ? 'active' : ''}`}
          onClick={() => setTab('alarms')}
        >
          <ShieldAlert size={16} />
          <span>ISA-18.2 Alarms</span>
          {unackAlarmsCount > 0 ? (
            <span className="scada-tab-badge" style={{ background: 'var(--accent-rose)', color: '#fff' }}>
              {unackAlarmsCount}
            </span>
          ) : (
            <span className="scada-tab-badge">{alarms.length}</span>
          )}
        </button>
        <button 
          className={`scada-tab-btn ${tab === 'tags' ? 'active' : ''}`}
          onClick={() => setTab('tags')}
        >
          <Sliders size={16} />
          <span>Tag Namespace Browser</span>
          <span className="scada-tab-badge">{tags.length}</span>
        </button>
        <button 
          className={`scada-tab-btn ${tab === 'symbols' ? 'active' : ''}`}
          onClick={() => setTab('symbols')}
        >
          <Box size={16} />
          <span>ThingsBoard SVG Symbols</span>
          <span className="scada-tab-badge">{symbols.length}</span>
        </button>
        <button 
          className={`scada-tab-btn ${tab === 'sbo' ? 'active' : ''}`}
          onClick={() => setTab('sbo')}
        >
          <Power size={16} />
          <span>SBO Control & Audit</span>
          <span className="scada-tab-badge">{controlLogs.length}</span>
        </button>
      </div>

      {/* Flood Warning Banner */}
      {kpis?.in_flood && (
        <div style={{
          background: 'rgba(244, 63, 94, 0.15)',
          border: '1px solid var(--accent-rose)',
          borderRadius: 'var(--radius-lg)',
          padding: '12px 18px',
          display: 'flex',
          alignItems: 'center',
          gap: '12px',
          color: '#fff'
        }}>
          <Flame size={20} style={{ color: 'var(--accent-rose)' }} />
          <div>
            <strong>EEMUA 191 Alarm Flood Active:</strong> Alarm rate is exceeding 10 alarms / 10 minutes ({kpis.alarm_rate_10min} triggered). Operator action required.
          </div>
        </div>
      )}

      {/* -------------------------------------------------------------------- */}
      {/* TAB 1: SYNOPTIC PROCESS HMI CANVAS & STUDIO BUILDER                  */}
      {/* -------------------------------------------------------------------- */}
      {tab === 'synoptic' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          
          {/* ================================================================ */}
          {/* STUDIO BUILDER MODE                                              */}
          {/* ================================================================ */}
          {canvasMode === 'builder' && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
              {/* Template Selection Toolbar */}
              <div className="builder-toolbar">
                <div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '4px' }}>
                    <Layers size={16} style={{ color: 'var(--accent-cyan)' }} />
                    <strong style={{ fontSize: '0.95rem', color: '#fff' }}>Industry Process Templates</strong>
                  </div>
                  <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                    Choose a pre-configured synoptic template or build from scratch:
                  </div>
                </div>

                <div className="template-pill-group">
                  {Object.entries(TEMPLATES).map(([key, tpl]) => (
                    <button
                      key={key}
                      className={`template-pill ${currentTemplate === key ? 'active' : ''}`}
                      onClick={() => handleSelectTemplate(key)}
                    >
                      <span>{tpl.icon}</span>
                      <span>{tpl.label.split(' ')[1] || tpl.label}</span>
                    </button>
                  ))}
                </div>

                <div style={{ display: 'flex', gap: '8px' }}>
                  <button className="btn btn-secondary btn-sm" onClick={handleResetLayout} title="Reset to template defaults">
                    <RotateCw size={13} />
                    <span>Reset</span>
                  </button>
                  <button className="btn btn-primary btn-sm" onClick={handleSaveLayout} title="Persist changes to local storage">
                    <Save size={13} />
                    <span>Save Canvas</span>
                  </button>
                  <button className="btn btn-secondary btn-sm" style={{ borderColor: 'var(--accent-cyan)', color: 'var(--accent-cyan)' }} onClick={() => setCanvasMode('monitor')}>
                    <Eye size={13} />
                    <span>Live Monitor</span>
                  </button>
                </div>
              </div>

              {/* Save Status Confirmation */}
              {saveStatus && (
                <div style={{
                  background: 'rgba(6, 182, 212, 0.15)',
                  border: '1px solid var(--accent-cyan)',
                  borderRadius: 'var(--radius-md)',
                  padding: '8px 14px',
                  fontSize: '0.8rem',
                  color: '#fff',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '8px'
                }}>
                  <Sparkles size={15} style={{ color: 'var(--accent-cyan)' }} />
                  <span>{saveStatus}</span>
                </div>
              )}

              {/* Equipment Palette */}
              <div className="card" style={{ padding: '16px' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <Plus size={16} style={{ color: 'var(--accent-cyan)' }} />
                    <strong style={{ fontSize: '0.9rem', color: '#fff' }}>Add Process Equipment to Canvas</strong>
                  </div>
                  <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                    Click an item to drop onto your synoptic screen
                  </span>
                </div>

                <div className="builder-palette-grid">
                  <div className="builder-palette-item" onClick={() => handleAddEquipment('tank')}>
                    <span style={{ fontSize: '1.4rem' }}>🛢️</span>
                    <span>+ Storage Tank</span>
                  </div>
                  <div className="builder-palette-item" onClick={() => handleAddEquipment('pump')}>
                    <span style={{ fontSize: '1.4rem' }}>⚙️</span>
                    <span>+ Centrifugal Pump</span>
                  </div>
                  <div className="builder-palette-item" onClick={() => handleAddEquipment('valve')}>
                    <span style={{ fontSize: '1.4rem' }}>🚰</span>
                    <span>+ Control Valve</span>
                  </div>
                  <div className="builder-palette-item" onClick={() => handleAddEquipment('meter')}>
                    <span style={{ fontSize: '1.4rem' }}>📟</span>
                    <span>+ Sensor / Meter</span>
                  </div>
                  <div className="builder-palette-item" onClick={() => handleAddEquipment('chemical')}>
                    <span style={{ fontSize: '1.4rem' }}>🧪</span>
                    <span>+ Dosing Vessel</span>
                  </div>
                </div>
              </div>

              {/* Tag Datalist */}
              <datalist id="scada-tag-datalist">
                {tags.map(t => (
                  <option key={t.id} value={t.path}>{t.path} ({t.data_type})</option>
                ))}
              </datalist>

              {/* Configured Equipment List */}
              <div className="card" style={{ padding: '16px' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '14px' }}>
                  <strong style={{ fontSize: '0.9rem', color: '#fff' }}>
                    Configured Equipment on Canvas ({canvasEquipment.length})
                  </strong>
                  <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                    Bind each component to a tag or adjust its live test value:
                  </span>
                </div>

                {canvasEquipment.length === 0 ? (
                  <div style={{ padding: '30px', textAlign: 'center', color: 'var(--text-muted)' }}>
                    Your canvas is empty. Select a template above or click any equipment in the palette to start building!
                  </div>
                ) : (
                  <div className="builder-equip-list">
                    {canvasEquipment.map(item => (
                      <div key={item.id} className="builder-equip-card">
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                            <span style={{ fontSize: '1.1rem' }}>
                              {item.type === 'tank' ? '🛢️' : item.type === 'pump' ? '⚙️' : item.type === 'valve' ? '🚰' : item.type === 'chemical' ? '🧪' : '📟'}
                            </span>
                            <span style={{ fontSize: '0.75rem', fontWeight: 700, color: 'var(--accent-cyan)', textTransform: 'uppercase' }}>
                              {item.type}
                            </span>
                            <span style={{ fontSize: '0.65rem', color: 'var(--text-muted)' }}>
                              #{item.id}
                            </span>
                          </div>
                          <button 
                            className="btn btn-secondary btn-sm" 
                            style={{ padding: '3px 6px', color: 'var(--accent-rose)' }}
                            onClick={() => handleRemoveEquipment(item.id)}
                            title="Remove equipment"
                          >
                            <Trash2 size={13} />
                          </button>
                        </div>

                        <div>
                          <label style={{ fontSize: '0.7rem', color: 'var(--text-muted)', display: 'block', marginBottom: '2px' }}>Equipment Name</label>
                          <input 
                            type="text" 
                            className="form-input" 
                            style={{ padding: '4px 8px', fontSize: '0.8rem' }}
                            value={item.name} 
                            onChange={e => handleUpdateEquipment(item.id, { name: e.target.value })} 
                          />
                        </div>

                        <div>
                          <label style={{ fontSize: '0.7rem', color: 'var(--text-muted)', display: 'block', marginBottom: '2px' }}>Bound Tag Path</label>
                          <input 
                            list="scada-tag-datalist" 
                            className="form-input font-mono" 
                            style={{ padding: '4px 8px', fontSize: '0.75rem' }}
                            placeholder="e.g. Plant.Boiler.Pressure"
                            value={item.tagPath} 
                            onChange={e => handleUpdateEquipment(item.id, { tagPath: e.target.value })} 
                          />
                        </div>

                        <div style={{ display: 'grid', gridTemplateColumns: '1fr 2fr', gap: '8px' }}>
                          <div>
                            <label style={{ fontSize: '0.7rem', color: 'var(--text-muted)', display: 'block', marginBottom: '2px' }}>Unit</label>
                            <input 
                              type="text" 
                              className="form-input" 
                              style={{ padding: '4px 8px', fontSize: '0.75rem' }}
                              value={item.unit} 
                              onChange={e => handleUpdateEquipment(item.id, { unit: e.target.value })} 
                            />
                          </div>
                          <div>
                            <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.7rem', color: 'var(--text-muted)', marginBottom: '2px' }}>
                              <span>Test Value:</span>
                              <strong style={{ color: '#fff' }}>{getEquipmentLiveValue(item).toFixed(1)} {item.unit}</strong>
                            </div>
                            <input 
                              type="range" 
                              min={item.minValue} 
                              max={item.maxValue} 
                              step="0.5" 
                              value={getEquipmentLiveValue(item)}
                              onChange={e => {
                                const val = parseFloat(e.target.value);
                                handleUpdateEquipment(item.id, { currentValue: val });
                                handleUpdateTag(item.tagPath, val);
                              }}
                              style={{ width: '100%' }}
                            />
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              {/* Builder Canvas Preview Box */}
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <strong style={{ fontSize: '0.95rem', color: '#fff' }}>Live Synoptic Canvas Preview</strong>
                <button className="btn btn-primary btn-sm" onClick={() => setCanvasMode('monitor')}>
                  <Eye size={13} />
                  <span>Open Full Screen Monitor</span>
                </button>
              </div>
            </div>
          )}

          {/* ================================================================ */}
          {/* LIVE MONITOR CANVAS (Used in Monitor & Preview)                  */}
          {/* ================================================================ */}
          <div className="synoptic-canvas-wrapper">
            <div className="synoptic-header-bar">
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                <Activity size={18} style={{ color: 'var(--accent-cyan)' }} />
                <h3 style={{ margin: 0, fontSize: '1.05rem', fontWeight: 700 }}>
                  {TEMPLATES[currentTemplate]?.label || 'Industrial Process Synoptic'}
                </h3>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                <span className="badge-tag live" style={{ background: isPumpRunning ? 'rgba(16, 185, 129, 0.2)' : 'rgba(244, 63, 94, 0.2)', color: isPumpRunning ? 'var(--accent-emerald)' : 'var(--accent-rose)' }}>
                  {isPumpRunning ? 'PROCESS RUNNING' : 'PROCESS STOPPED'}
                </span>
                <span className="badge-tag" style={{ background: 'rgba(255,255,255,0.06)', color: 'var(--text-secondary)' }}>
                  Click Equipment for Faceplate
                </span>
                {canvasMode === 'monitor' && (
                  <button className="btn btn-secondary btn-sm" onClick={() => setCanvasMode('builder')}>
                    <Wrench size={13} />
                    <span>Studio Builder</span>
                  </button>
                )}
              </div>
            </div>

            {/* Main Interactive SVG Mimic */}
            <div className="synoptic-svg-container">
              <svg width="100%" height="340" viewBox="0 0 960 340" style={{ maxWidth: '960px' }}>
                <defs>
                  <linearGradient id="synWaterGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                    <stop offset="0%" stopColor="#06b6d4" stopOpacity="0.85" />
                    <stop offset="100%" stopColor="#0284c7" stopOpacity="0.95" />
                  </linearGradient>
                  <linearGradient id="chemAcidGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                    <stop offset="0%" stopColor="#c084fc" stopOpacity="0.85" />
                    <stop offset="100%" stopColor="#7e22ce" stopOpacity="0.95" />
                  </linearGradient>
                  <linearGradient id="steamGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                    <stop offset="0%" stopColor="#f59e0b" stopOpacity="0.85" />
                    <stop offset="100%" stopColor="#b45309" stopOpacity="0.95" />
                  </linearGradient>
                  <filter id="synGlow" x="-20%" y="-20%" width="140%" height="140%">
                    <feGaussianBlur stdDeviation="4" result="blur" />
                    <feComposite in="SourceGraphic" in2="blur" operator="over" />
                  </filter>
                </defs>

                {/* TEMPLATE 1: Water Treatment & Chemical Dosing */}
                {currentTemplate === 'water_treatment' && (
                  <g>
                    {/* 1. Raw Water Storage Tank 101 */}
                    <g 
                      className="synoptic-equipment" 
                      onClick={() => setFaceplateItem({ id: 'Tank101', title: 'TK-101 Raw Water Storage', type: 'tank' })}
                    >
                      <rect x="50" y="50" width="160" height="220" rx="12" fill="rgba(30, 41, 59, 0.75)" stroke="#475569" strokeWidth="2.5" />
                      <text x="130" y="38" textAnchor="middle" fill="#cbd5e1" fontSize="12" fontWeight="800">TANK 101</text>
                      <text x="130" y="290" textAnchor="middle" fill="#94a3b8" fontSize="10">Raw Water Buffer</text>

                      {/* Liquid Column */}
                      {(() => {
                        const maxH = 205;
                        const curH = (Math.min(100, Math.max(0, tankLevel)) / 100) * maxH;
                        const y = 265 - curH;
                        return (
                          <g>
                            <rect x="55" y={y} width="150" height={curH} rx="8" fill="url(#synWaterGrad)" />
                            <line x1="50" y1="72" x2="210" y2="72" stroke="#f43f5e" strokeWidth="2.5" strokeDasharray="5 3" />
                            <text x="215" y="75" fill="#f43f5e" fontSize="9" fontWeight="700">90% HI-HI</text>
                            {tankLevel > 90 && (
                              <circle cx="200" cy="62" r="6" fill="#f43f5e" className="scada-pulse-beacon" />
                            )}
                          </g>
                        );
                      })()}

                      <text x="130" y="160" textAnchor="middle" fill="#ffffff" fontSize="22" fontWeight="800" filter="url(#synGlow)">
                        {tankLevel.toFixed(1)} %
                      </text>
                      <text x="130" y="180" textAnchor="middle" fill="#94a3b8" fontSize="11">
                        {(tankLevel * 200).toFixed(0)} L / 20k L
                      </text>
                    </g>

                    {/* 2. Pipe: Tank 101 to Pump 101 */}
                    <path d="M 210 220 L 330 220" stroke="#475569" strokeWidth="14" strokeLinecap="round" />
                    {isPumpRunning && (
                      <path d="M 210 220 L 330 220" stroke="#06b6d4" strokeWidth="4" strokeDasharray="10 6" className="scada-pipe-flow" />
                    )}

                    {/* 3. Centrifugal Process Pump 101 */}
                    <g 
                      className="synoptic-equipment" 
                      transform="translate(360, 220)"
                      onClick={() => setFaceplateItem({ id: 'Pump101', title: 'PUMP 101 Duty Centrifugal', type: 'pump' })}
                    >
                      <circle cx="0" cy="0" r="42" fill="rgba(15, 23, 42, 0.95)" stroke={isPumpRunning ? '#10b981' : '#f43f5e'} strokeWidth="3" filter={isPumpRunning ? 'url(#synGlow)' : undefined} />
                      
                      <g className={isPumpRunning ? 'scada-rotating' : ''}>
                        <circle cx="0" cy="0" r="14" fill="#1e293b" stroke="#cbd5e1" strokeWidth="2" />
                        <line x1="-26" y1="0" x2="26" y2="0" stroke="#f8fafc" strokeWidth="4" strokeLinecap="round" />
                        <line x1="0" y1="-26" x2="0" y2="26" stroke="#f8fafc" strokeWidth="4" strokeLinecap="round" />
                        <circle cx="0" cy="0" r="6" fill="#38bdf8" />
                      </g>

                      <text x="0" y="60" textAnchor="middle" fill="#f8fafc" fontSize="12" fontWeight="700">PUMP 101</text>
                      <text x="0" y="75" textAnchor="middle" fill={isPumpRunning ? '#10b981' : '#f43f5e'} fontSize="10" fontWeight="800">
                        {isPumpRunning ? 'RUNNING (1)' : 'STOPPED (0)'}
                      </text>
                    </g>

                    {/* 4. Pipe: Pump 101 to Modulating Valve FCV-101 */}
                    <path d="M 402 220 L 510 220" stroke="#475569" strokeWidth="14" strokeLinecap="round" />
                    {isPumpRunning && (
                      <path d="M 402 220 L 510 220" stroke="#06b6d4" strokeWidth="4" strokeDasharray="10 6" className="scada-pipe-flow" />
                    )}

                    {/* 5. Modulating Control Valve FCV-101 */}
                    <g 
                      className="synoptic-equipment" 
                      transform="translate(540, 220)"
                      onClick={() => setFaceplateItem({ id: 'Valve101', title: 'FCV-101 Modulating Control Valve', type: 'valve' })}
                    >
                      <path d="M -22 -35 C -22 -48 22 -48 22 -35 Z" fill="#334155" stroke="#64748b" strokeWidth="2" />
                      <line x1="0" y1="-35" x2="0" y2="-15" stroke="#94a3b8" strokeWidth="3" />
                      <polygon points="-25,-18 -25,18 0,0" fill="#0ea5e9" stroke="#38bdf8" strokeWidth="2" />
                      <polygon points="25,-18 25,18 0,0" fill="#0ea5e9" stroke="#38bdf8" strokeWidth="2" />
                      <circle cx="0" cy="0" r="6" fill="#ffffff" />
                      
                      <text x="0" y="32" textAnchor="middle" fill="#cbd5e1" fontSize="11" fontWeight="700">FCV-101</text>
                      <text x="0" y="46" textAnchor="middle" fill="#38bdf8" fontSize="10" fontWeight="800">{valvePosition.toFixed(0)}% OPEN</text>
                    </g>

                    {/* 6. Pipe: Valve 101 to Discharge */}
                    <path d="M 565 220 L 700 220 L 700 150 L 890 150" stroke="#475569" strokeWidth="14" strokeLinecap="round" strokeLinejoin="round" />
                    {isPumpRunning && (
                      <path d="M 565 220 L 700 220 L 700 150 L 890 150" stroke="#06b6d4" strokeWidth="4" strokeDasharray="10 6" strokeLinejoin="round" className="scada-pipe-flow" />
                    )}

                    {/* 7. Chemical Dosing Skid (Tank 202 Acid/Reagent) */}
                    <g 
                      className="synoptic-equipment" 
                      onClick={() => setFaceplateItem({ id: 'ChemSkid', title: 'Chemical Acid Dosing Skid (TK-202)', type: 'chemical' })}
                    >
                      <rect x="580" y="35" width="100" height="120" rx="8" fill="rgba(30, 41, 59, 0.75)" stroke="#7c3aed" strokeWidth="2" />
                      <text x="630" y="24" textAnchor="middle" fill="#c084fc" fontSize="11" fontWeight="700">TK-202 ACID</text>
                      
                      {(() => {
                        const maxH = 105;
                        const curH = (acidLevel / 100) * maxH;
                        return (
                          <rect x="585" y={150 - curH} width="90" height={curH} rx="4" fill="url(#chemAcidGrad)" />
                        );
                      })()}
                      <text x="630" y="95" textAnchor="middle" fill="#fff" fontSize="14" fontWeight="800">{acidLevel.toFixed(1)} %</text>

                      <path d="M 630 155 L 630 220" stroke="#a855f7" strokeWidth="6" strokeDasharray="4 2" />
                      <circle cx="630" cy="220" r="4" fill="#a855f7" />
                      <text x="630" y="175" textAnchor="start" fill="#c084fc" fontSize="9" fontWeight="700">Dosing {dosingSpeed.toFixed(0)} RPM</text>
                    </g>

                    {/* 8. Inline Transmitters */}
                    <g 
                      className="synoptic-equipment" 
                      transform="translate(450, 140)"
                      onClick={() => setFaceplateItem({ id: 'FT101', title: 'FT-101 Electromagnetic Flowmeter', type: 'meter' })}
                    >
                      <line x1="0" y1="20" x2="0" y2="80" stroke="#06b6d4" strokeWidth="2" strokeDasharray="3 2" />
                      <rect x="-42" y="-18" width="84" height="36" rx="6" fill="#0f172a" stroke="#06b6d4" strokeWidth="2" />
                      <text x="0" y="-4" textAnchor="middle" fill="#94a3b8" fontSize="9" fontWeight="700">FLOW (FT-101)</text>
                      <text x="0" y="11" textAnchor="middle" fill="#fff" fontSize="13" fontWeight="800">
                        {isPumpRunning ? flowVal.toFixed(1) : '0.0'} GPM
                      </text>
                    </g>

                    <g 
                      className="synoptic-equipment" 
                      transform="translate(760, 95)"
                      onClick={() => setFaceplateItem({ id: 'PT101', title: 'PT-101 Discharge Pressure Transmitter', type: 'meter' })}
                    >
                      <line x1="0" y1="20" x2="0" y2="55" stroke="#a855f7" strokeWidth="2" strokeDasharray="3 2" />
                      <circle cx="0" cy="-5" r="26" fill="#0f172a" stroke="#a855f7" strokeWidth="2" />
                      <text x="0" y="-12" textAnchor="middle" fill="#94a3b8" fontSize="8" fontWeight="700">DISCHARGE</text>
                      <text x="0" y="4" textAnchor="middle" fill="#fff" fontSize="12" fontWeight="800">
                        {isPumpRunning ? pressureVal.toFixed(1) : '14.7'} PSI
                      </text>
                    </g>

                    <g 
                      className="synoptic-equipment" 
                      transform="translate(850, 95)"
                      onClick={() => setFaceplateItem({ id: 'AT101', title: 'AT-101 Analytical pH Sensor', type: 'meter' })}
                    >
                      <line x1="0" y1="20" x2="0" y2="55" stroke="#10b981" strokeWidth="2" strokeDasharray="3 2" />
                      <circle cx="0" cy="-5" r="26" fill="#0f172a" stroke="#10b981" strokeWidth="2" />
                      <text x="0" y="-12" textAnchor="middle" fill="#94a3b8" fontSize="8" fontWeight="700">EFFLUENT pH</text>
                      <text x="0" y="4" textAnchor="middle" fill="#fff" fontSize="12" fontWeight="800">
                        {dischargePH.toFixed(2)}
                      </text>
                    </g>

                    <text x="910" y="154" fill="#38bdf8" fontSize="11" fontWeight="800">DISCHARGE &gt;&gt;</text>
                  </g>
                )}

                {/* TEMPLATE 2: Steam Boiler Station */}
                {currentTemplate === 'steam_boiler' && (
                  <g>
                    {/* Feedwater Inflow Pipe & Pump */}
                    <path d="M 40 220 L 140 220" stroke="#475569" strokeWidth="12" strokeLinecap="round" />
                    <g 
                      className="synoptic-equipment" 
                      transform="translate(140, 220)"
                      onClick={() => setFaceplateItem({ id: 'FeedPump', title: 'P-201 High-Pressure Feedwater Pump', type: 'pump' })}
                    >
                      <circle cx="0" cy="0" r="38" fill="rgba(15, 23, 42, 0.95)" stroke="#10b981" strokeWidth="3" />
                      <g className="scada-rotating">
                        <circle cx="0" cy="0" r="12" fill="#1e293b" stroke="#cbd5e1" strokeWidth="2" />
                        <line x1="-22" y1="0" x2="22" y2="0" stroke="#f8fafc" strokeWidth="3" />
                        <line x1="0" y1="-22" x2="0" y2="22" stroke="#f8fafc" strokeWidth="3" />
                      </g>
                      <text x="0" y="55" textAnchor="middle" fill="#cbd5e1" fontSize="11" fontWeight="700">FEED PUMP</text>
                    </g>
                    <path d="M 178 220 L 260 220" stroke="#475569" strokeWidth="12" strokeLinecap="round" />

                    {/* Boiler Steam Drum B-201 */}
                    <g 
                      className="synoptic-equipment" 
                      onClick={() => setFaceplateItem({ id: 'BoilerDrum', title: 'Boiler Steam Drum B-201', type: 'tank' })}
                    >
                      <rect x="260" y="60" width="220" height="180" rx="20" fill="rgba(30, 41, 59, 0.85)" stroke="#f59e0b" strokeWidth="3" />
                      <text x="370" y="45" textAnchor="middle" fill="#fcd34d" fontSize="13" fontWeight="800">BOILER DRUM B-201</text>
                      
                      {/* Water Level in Drum */}
                      <rect x="266" y="140" width="208" height="92" rx="10" fill="url(#synWaterGrad)" opacity="0.8" />
                      <rect x="266" y="66" width="208" height="74" rx="10" fill="url(#steamGrad)" opacity="0.4" />
                      
                      <text x="370" y="120" textAnchor="middle" fill="#fff" fontSize="11" fontWeight="700">SATURATED STEAM DOME</text>
                      <text x="370" y="180" textAnchor="middle" fill="#fff" fontSize="22" fontWeight="800" filter="url(#synGlow)">68.5 % LEVEL</text>
                      
                      {/* Burner Combustion Flame beneath drum */}
                      <rect x="300" y="250" width="140" height="24" rx="6" fill="rgba(239, 68, 68, 0.2)" stroke="#ef4444" strokeWidth="1.5" />
                      <text x="370" y="266" textAnchor="middle" fill="#f87171" fontSize="10" fontWeight="700">GAS BURNER FIRING (85%)</text>
                    </g>

                    {/* Steam Header Pipe from drum top */}
                    <path d="M 480 110 L 880 110" stroke="#b45309" strokeWidth="14" strokeLinecap="round" />
                    <path d="M 480 110 L 880 110" stroke="#fbbf24" strokeWidth="4" strokeDasharray="12 6" className="scada-pipe-flow" />

                    {/* Main Steam Throttle Valve MSV-201 */}
                    <g 
                      className="synoptic-equipment" 
                      transform="translate(560, 110)"
                      onClick={() => setFaceplateItem({ id: 'SteamValve', title: 'Main Steam Throttle Valve MSV-201', type: 'valve' })}
                    >
                      <polygon points="-25,-18 -25,18 0,0" fill="#f59e0b" stroke="#fbbf24" strokeWidth="2" />
                      <polygon points="25,-18 25,18 0,0" fill="#f59e0b" stroke="#fbbf24" strokeWidth="2" />
                      <circle cx="0" cy="0" r="6" fill="#fff" />
                      <text x="0" y="32" textAnchor="middle" fill="#fcd34d" fontSize="11" fontWeight="700">MSV-201</text>
                      <text x="0" y="46" textAnchor="middle" fill="#fff" fontSize="10" fontWeight="800">80% OPEN</text>
                    </g>

                    {/* Steam Pressure PT-201 */}
                    <g 
                      className="synoptic-equipment" 
                      transform="translate(700, 180)"
                      onClick={() => setFaceplateItem({ id: 'SteamPressure', title: 'PT-201 Steam Header Pressure', type: 'meter' })}
                    >
                      <circle cx="0" cy="0" r="32" fill="#0f172a" stroke="#f59e0b" strokeWidth="2" />
                      <text x="0" y="-8" textAnchor="middle" fill="#94a3b8" fontSize="8" fontWeight="700">STEAM PRESS</text>
                      <text x="0" y="8" textAnchor="middle" fill="#fff" fontSize="13" fontWeight="800">18.4 bar</text>
                      <line x1="0" y1="-32" x2="0" y2="-70" stroke="#f59e0b" strokeWidth="2" strokeDasharray="3 2" />
                    </g>

                    {/* Superheater Temp TT-201 */}
                    <g 
                      className="synoptic-equipment" 
                      transform="translate(810, 180)"
                      onClick={() => setFaceplateItem({ id: 'SteamTemp', title: 'TT-201 Superheater Temperature', type: 'meter' })}
                    >
                      <circle cx="0" cy="0" r="32" fill="#0f172a" stroke="#ef4444" strokeWidth="2" />
                      <text x="0" y="-8" textAnchor="middle" fill="#94a3b8" fontSize="8" fontWeight="700">STEAM TEMP</text>
                      <text x="0" y="8" textAnchor="middle" fill="#fff" fontSize="13" fontWeight="800">285.0 °C</text>
                      <line x1="0" y1="-32" x2="0" y2="-70" stroke="#ef4444" strokeWidth="2" strokeDasharray="3 2" />
                    </g>

                    <text x="900" y="114" fill="#fbbf24" fontSize="11" fontWeight="800">TURBINE &gt;&gt;</text>
                  </g>
                )}

                {/* TEMPLATE 3: Chemical Reactor Skid */}
                {currentTemplate === 'chemical_reactor' && (
                  <g>
                    {/* Reagent Feed Valve & Inflow */}
                    <path d="M 60 120 L 220 120" stroke="#475569" strokeWidth="12" strokeLinecap="round" />
                    <g 
                      className="synoptic-equipment" 
                      transform="translate(130, 120)"
                      onClick={() => setFaceplateItem({ id: 'ReagentValve', title: 'Reagent Feed Valve FV-301', type: 'valve' })}
                    >
                      <polygon points="-20,-15 -20,15 0,0" fill="#10b981" stroke="#34d399" strokeWidth="2" />
                      <polygon points="20,-15 20,15 0,0" fill="#10b981" stroke="#34d399" strokeWidth="2" />
                      <circle cx="0" cy="0" r="5" fill="#fff" />
                      <text x="0" y="30" textAnchor="middle" fill="#cbd5e1" fontSize="10" fontWeight="700">FV-301 (45%)</text>
                    </g>

                    {/* Continuous Reactor Vessel RX-301 */}
                    <g 
                      className="synoptic-equipment" 
                      onClick={() => setFaceplateItem({ id: 'ReactorVessel', title: 'Continuous Reactor Vessel RX-301', type: 'tank' })}
                    >
                      <rect x="220" y="50" width="220" height="220" rx="30" fill="rgba(30, 41, 59, 0.85)" stroke="#10b981" strokeWidth="3" />
                      <text x="330" y="38" textAnchor="middle" fill="#6ee7b7" fontSize="13" fontWeight="800">REACTOR VESSEL RX-301</text>
                      
                      {/* Reactant Liquid with agitator */}
                      <rect x="226" y="120" width="208" height="142" rx="15" fill="rgba(16, 185, 129, 0.45)" />
                      
                      {/* Spinning Agitator Mixer in Vessel Center */}
                      <line x1="330" y1="50" x2="330" y2="200" stroke="#cbd5e1" strokeWidth="4" />
                      <g transform="translate(330, 200)" className="scada-rotating">
                        <polygon points="-35,-8 35,8 0,0" fill="#38bdf8" />
                        <polygon points="-35,8 35,-8 0,0" fill="#38bdf8" />
                      </g>
                      
                      <text x="330" y="160" textAnchor="middle" fill="#fff" fontSize="22" fontWeight="800" filter="url(#synGlow)">54.2 % VOL</text>
                      <text x="330" y="240" textAnchor="middle" fill="#cbd5e1" fontSize="10" fontWeight="700">AGITATOR 750 RPM</text>
                    </g>

                    {/* Recirculation Loop Pipe & Pump */}
                    <path d="M 330 270 L 330 310 L 520 310 L 520 220" stroke="#475569" strokeWidth="12" strokeLinecap="round" strokeLinejoin="round" />
                    <g 
                      className="synoptic-equipment" 
                      transform="translate(520, 220)"
                      onClick={() => setFaceplateItem({ id: 'AgitatorPump', title: 'Reactor Recirculation Pump P-301', type: 'pump' })}
                    >
                      <circle cx="0" cy="0" r="38" fill="rgba(15, 23, 42, 0.95)" stroke="#10b981" strokeWidth="3" />
                      <g className="scada-rotating">
                        <circle cx="0" cy="0" r="12" fill="#1e293b" stroke="#cbd5e1" strokeWidth="2" />
                        <line x1="-22" y1="0" x2="22" y2="0" stroke="#f8fafc" strokeWidth="3" />
                        <line x1="0" y1="-22" x2="0" y2="22" stroke="#f8fafc" strokeWidth="3" />
                      </g>
                      <text x="0" y="55" textAnchor="middle" fill="#cbd5e1" fontSize="11" fontWeight="700">RECIRC P-301</text>
                    </g>
                    <path d="M 520 182 L 520 120 L 440 120" stroke="#475569" strokeWidth="12" strokeLinecap="round" strokeLinejoin="round" />

                    {/* Catalyst Dosing Vessel TK-305 */}
                    <g 
                      className="synoptic-equipment" 
                      onClick={() => setFaceplateItem({ id: 'ReagentTank', title: 'Catalyst Reagent Dosing TK-305', type: 'chemical' })}
                    >
                      <rect x="580" y="45" width="100" height="110" rx="8" fill="rgba(30, 41, 59, 0.8)" stroke="#a855f7" strokeWidth="2" />
                      <text x="630" y="35" textAnchor="middle" fill="#c084fc" fontSize="10" fontWeight="700">TK-305 CATALYST</text>
                      <rect x="585" y="85" width="90" height="66" rx="4" fill="url(#chemAcidGrad)" />
                      <text x="630" y="125" textAnchor="middle" fill="#fff" fontSize="14" fontWeight="800">62.0 %</text>
                    </g>

                    {/* Analytical Transmitters (Temp & pH) */}
                    <g 
                      className="synoptic-equipment" 
                      transform="translate(750, 120)"
                      onClick={() => setFaceplateItem({ id: 'ReactorTemp', title: 'TT-301 Reactor Temperature', type: 'meter' })}
                    >
                      <circle cx="0" cy="0" r="30" fill="#0f172a" stroke="#f43f5e" strokeWidth="2" />
                      <text x="0" y="-6" textAnchor="middle" fill="#94a3b8" fontSize="8" fontWeight="700">RX TEMP</text>
                      <text x="0" y="8" textAnchor="middle" fill="#fff" fontSize="12" fontWeight="800">72.4 °C</text>
                    </g>

                    <g 
                      className="synoptic-equipment" 
                      transform="translate(840, 120)"
                      onClick={() => setFaceplateItem({ id: 'ReactorPH', title: 'AT-301 Effluent pH Analyzer', type: 'meter' })}
                    >
                      <circle cx="0" cy="0" r="30" fill="#0f172a" stroke="#10b981" strokeWidth="2" />
                      <text x="0" y="-6" textAnchor="middle" fill="#94a3b8" fontSize="8" fontWeight="700">EFFLUENT pH</text>
                      <text x="0" y="8" textAnchor="middle" fill="#fff" fontSize="12" fontWeight="800">6.80</text>
                    </g>

                    <text x="900" y="124" fill="#34d399" fontSize="11" fontWeight="800">PRODUCT &gt;&gt;</text>
                  </g>
                )}

                {/* TEMPLATE 4 / CUSTOM: Dynamic Canvas Rendering */}
                {(currentTemplate === 'blank' || (currentTemplate !== 'water_treatment' && currentTemplate !== 'steam_boiler' && currentTemplate !== 'chemical_reactor')) && (
                  <g>
                    {canvasEquipment.length === 0 ? (
                      <g transform="translate(480, 160)">
                        <text x="0" y="-20" textAnchor="middle" fill="#94a3b8" fontSize="18" fontWeight="700">
                          SCADA Process Canvas is Empty
                        </text>
                        <text x="0" y="10" textAnchor="middle" fill="#64748b" fontSize="13">
                          Click "+ Add Equipment" or choose an Industry Template above to assemble your HMI!
                        </text>
                      </g>
                    ) : (
                      <g>
                        {/* Process Header Pipe spanning canvas */}
                        <line x1="40" y1="180" x2="920" y2="180" stroke="#334155" strokeWidth="14" strokeLinecap="round" />
                        <line x1="40" y1="180" x2="920" y2="180" stroke="#06b6d4" strokeWidth="4" strokeDasharray="10 6" className="scada-pipe-flow" />

                        {canvasEquipment.map((item, idx) => {
                          const count = canvasEquipment.length;
                          const spacing = 840 / Math.max(1, count);
                          const posX = 70 + idx * spacing + (spacing / 2);
                          const curVal = getEquipmentLiveValue(item);

                          if (item.type === 'tank') {
                            const curH = (Math.min(100, Math.max(0, curVal)) / 100) * 120;
                            return (
                              <g 
                                key={item.id} 
                                className="synoptic-equipment" 
                                transform={`translate(${posX - 45}, 70)`}
                                onClick={() => setFaceplateItem({ id: item.id, title: item.name, type: 'tank' })}
                              >
                                <rect x="0" y="0" width="90" height="140" rx="10" fill="rgba(30, 41, 59, 0.85)" stroke="#06b6d4" strokeWidth="2.5" />
                                <rect x="5" y={135 - curH} width="80" height={curH} rx="6" fill="url(#synWaterGrad)" />
                                <text x="45" y="-10" textAnchor="middle" fill="#cbd5e1" fontSize="11" fontWeight="700">{item.name}</text>
                                <text x="45" y="75" textAnchor="middle" fill="#fff" fontSize="16" fontWeight="800">{curVal.toFixed(1)} {item.unit}</text>
                                <text x="45" y="155" textAnchor="middle" fill="#94a3b8" fontSize="9">{item.tagPath}</text>
                              </g>
                            );
                          }

                          if (item.type === 'pump') {
                            const isRunning = curVal > 0;
                            return (
                              <g 
                                key={item.id} 
                                className="synoptic-equipment" 
                                transform={`translate(${posX}, 180)`}
                                onClick={() => setFaceplateItem({ id: item.id, title: item.name, type: 'pump' })}
                              >
                                <circle cx="0" cy="0" r="36" fill="rgba(15, 23, 42, 0.95)" stroke={isRunning ? '#10b981' : '#f43f5e'} strokeWidth="3" />
                                <g className={isRunning ? 'scada-rotating' : ''}>
                                  <circle cx="0" cy="0" r="10" fill="#1e293b" stroke="#cbd5e1" strokeWidth="2" />
                                  <line x1="-22" y1="0" x2="22" y2="0" stroke="#f8fafc" strokeWidth="3" />
                                  <line x1="0" y1="-22" x2="0" y2="22" stroke="#f8fafc" strokeWidth="3" />
                                </g>
                                <text x="0" y="-45" textAnchor="middle" fill="#cbd5e1" fontSize="11" fontWeight="700">{item.name}</text>
                                <text x="0" y="55" textAnchor="middle" fill={isRunning ? '#10b981' : '#f43f5e'} fontSize="10" fontWeight="800">
                                  {isRunning ? 'RUNNING' : 'STOPPED'}
                                </text>
                              </g>
                            );
                          }

                          if (item.type === 'valve') {
                            return (
                              <g 
                                key={item.id} 
                                className="synoptic-equipment" 
                                transform={`translate(${posX}, 180)`}
                                onClick={() => setFaceplateItem({ id: item.id, title: item.name, type: 'valve' })}
                              >
                                <polygon points="-22,-16 -22,16 0,0" fill="#0ea5e9" stroke="#38bdf8" strokeWidth="2" />
                                <polygon points="22,-16 22,16 0,0" fill="#0ea5e9" stroke="#38bdf8" strokeWidth="2" />
                                <circle cx="0" cy="0" r="5" fill="#fff" />
                                <text x="0" y="-28" textAnchor="middle" fill="#cbd5e1" fontSize="11" fontWeight="700">{item.name}</text>
                                <text x="0" y="32" textAnchor="middle" fill="#38bdf8" fontSize="10" fontWeight="800">{curVal.toFixed(0)}% OPEN</text>
                              </g>
                            );
                          }

                          if (item.type === 'chemical') {
                            const curH = (Math.min(100, Math.max(0, curVal)) / 100) * 80;
                            return (
                              <g 
                                key={item.id} 
                                className="synoptic-equipment" 
                                transform={`translate(${posX - 35}, 60)`}
                                onClick={() => setFaceplateItem({ id: item.id, title: item.name, type: 'chemical' })}
                              >
                                <rect x="0" y="0" width="70" height="90" rx="8" fill="rgba(30, 41, 59, 0.85)" stroke="#a855f7" strokeWidth="2" />
                                <rect x="4" y={86 - curH} width="62" height={curH} rx="4" fill="url(#chemAcidGrad)" />
                                <text x="35" y="-8" textAnchor="middle" fill="#c084fc" fontSize="10" fontWeight="700">{item.name}</text>
                                <text x="35" y="50" textAnchor="middle" fill="#fff" fontSize="13" fontWeight="800">{curVal.toFixed(1)}%</text>
                                <line x1="35" y1="90" x2="35" y2="120" stroke="#a855f7" strokeWidth="4" strokeDasharray="3 2" />
                              </g>
                            );
                          }

                          // Default: meter
                          return (
                            <g 
                              key={item.id} 
                              className="synoptic-equipment" 
                              transform={`translate(${posX}, 95)`}
                              onClick={() => setFaceplateItem({ id: item.id, title: item.name, type: 'meter' })}
                            >
                              <line x1="0" y1="20" x2="0" y2="85" stroke="#06b6d4" strokeWidth="2" strokeDasharray="3 2" />
                              <circle cx="0" cy="0" r="28" fill="#0f172a" stroke="#06b6d4" strokeWidth="2" />
                              <text x="0" y="-8" textAnchor="middle" fill="#94a3b8" fontSize="8" fontWeight="700">{item.name}</text>
                              <text x="0" y="8" textAnchor="middle" fill="#fff" fontSize="12" fontWeight="800">{curVal.toFixed(1)}</text>
                              <text x="0" y="20" textAnchor="middle" fill="#38bdf8" fontSize="8" fontWeight="700">{item.unit}</text>
                            </g>
                          );
                        })}
                      </g>
                    )}
                  </g>
                )}
              </svg>
            </div>
          </div>

          {/* Quick Simulation Dock */}
          <div className="card">
            <h3 style={{ fontSize: '1rem', fontWeight: 700, marginBottom: '12px', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Radio size={16} style={{ color: 'var(--accent-cyan)' }} />
              <span>Process Variable Simulator & Deadband Demonstration</span>
            </h3>
            <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '16px' }}>
              Move sliders to inject process telemetry. Values within deadbands are filtered; levels &gt; 90% trigger ISA-18.2 High-High Alarms.
            </p>

            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '20px' }}>
              <div>
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.8rem', marginBottom: '6px' }}>
                  <span>Tank 101 Level (Deadband: ±0.5%):</span>
                  <strong style={{ color: tankLevel > 90 ? 'var(--accent-rose)' : 'var(--accent-cyan)' }}>
                    {simLevel.toFixed(1)} % {tankLevel > 90 ? '(! HI-HI)' : ''}
                  </strong>
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
                  style={{ width: '100%' }}
                />
              </div>

              <div>
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.8rem', marginBottom: '6px' }}>
                  <span>Discharge Pressure (Deadband: ±0.5 PSI):</span>
                  <strong style={{ color: 'var(--accent-purple)' }}>{simPressure.toFixed(1)} PSI</strong>
                </div>
                <input 
                  type="range" 
                  min="0" 
                  max="150" 
                  step="0.5"
                  value={simPressure}
                  onChange={e => {
                    const n = parseFloat(e.target.value);
                    setSimPressure(n);
                    handleUpdateTag('WaterTreatment.Discharge.Pressure', n);
                  }}
                  style={{ width: '100%' }}
                />
              </div>

              <div>
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.8rem', marginBottom: '6px' }}>
                  <span>Effluent pH (Deadband: ±0.1 pH):</span>
                  <strong style={{ color: 'var(--accent-emerald)' }}>{simPH.toFixed(2)} pH</strong>
                </div>
                <input 
                  type="range" 
                  min="0" 
                  max="14" 
                  step="0.1"
                  value={simPH}
                  onChange={e => {
                    const n = parseFloat(e.target.value);
                    setSimPH(n);
                    handleUpdateTag('WaterTreatment.Discharge.pH', n);
                  }}
                  style={{ width: '100%' }}
                />
              </div>
            </div>
          </div>
        </div>
      )}

      {/* -------------------------------------------------------------------- */}
      {/* TAB 2: ISA-18.2 PROCESS ALARM CENTER                                 */}
      {/* -------------------------------------------------------------------- */}
      {tab === 'alarms' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          {/* EEMUA 191 KPI Cards */}
          <div className="scada-kpi-grid">
            <div className="scada-kpi-card">
              <span className="kpi-label">Active Alarms</span>
              <span className="kpi-val" style={{ color: activeAlarmsCount > 0 ? 'var(--accent-rose)' : 'var(--accent-emerald)' }}>
                {activeAlarmsCount}
              </span>
              <span className="kpi-sub">Total ISA-18.2 active</span>
            </div>

            <div className="scada-kpi-card">
              <span className="kpi-label">Unacknowledged</span>
              <span className="kpi-val" style={{ color: unackAlarmsCount > 0 ? 'var(--accent-rose)' : '#fff' }}>
                {unackAlarmsCount}
              </span>
              <span className="kpi-sub">Requires operator ack</span>
            </div>

            <div className="scada-kpi-card">
              <span className="kpi-label">Shelved Alarms</span>
              <span className="kpi-val" style={{ color: 'var(--accent-amber)' }}>
                {kpis?.shelved_alarms ?? alarms.filter(a => a.state === 'SHELVED').length}
              </span>
              <span className="kpi-sub">Temporarily suppressed</span>
            </div>

            <div className="scada-kpi-card">
              <span className="kpi-label">Mean Time to Ack</span>
              <span className="kpi-val" style={{ color: 'var(--accent-cyan)' }}>
                {kpis?.mtta_seconds ? `${kpis.mtta_seconds.toFixed(1)}s` : '0.0s'}
              </span>
              <span className="kpi-sub">Operator MTTA target &lt; 30s</span>
            </div>

            <div className="scada-kpi-card">
              <span className="kpi-label">10-Min Flood Rate</span>
              <span className="kpi-val" style={{ color: (kpis?.alarm_rate_10min ?? 0) > 10 ? 'var(--accent-rose)' : '#fff' }}>
                {kpis?.alarm_rate_10min ?? 0}
              </span>
              <span className="kpi-sub">Max EEMUA 191 threshold: 10</span>
            </div>
          </div>

          {/* Alarms Management Table */}
          <div className="card">
            <h3 style={{ fontSize: '1.05rem', fontWeight: 700, marginBottom: '16px', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <ShieldAlert size={18} style={{ color: 'var(--accent-rose)' }} />
              <span>ISA-18.2 Process Alarm Ledger</span>
            </h3>

            {alarms.length === 0 ? (
              <div style={{ padding: '30px', textAlign: 'center', color: 'var(--text-muted)' }}>
                No active process alarms. System operating within normal thresholds.
              </div>
            ) : (
              <div className="table-container">
                <table className="data-table">
                  <thead>
                    <tr>
                      <th>Tag Path</th>
                      <th>Type & Severity</th>
                      <th>Message</th>
                      <th>ISA State</th>
                      <th>Active Since</th>
                      <th>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {alarms.map(a => (
                      <tr key={a.id}>
                        <td>
                          <div className="font-mono" style={{ fontWeight: 600, color: '#fff', fontSize: '0.85rem' }}>
                            {a.tag_path}
                          </div>
                          <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                            Setpoint: {a.setpoint}
                          </div>
                        </td>
                        <td>
                          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                            <span style={{
                              background: a.severity === 'CRITICAL' ? 'rgba(244, 63, 94, 0.2)' : 'rgba(245, 158, 11, 0.2)',
                              color: a.severity === 'CRITICAL' ? 'var(--accent-rose)' : 'var(--accent-amber)',
                              padding: '2px 8px',
                              borderRadius: '4px',
                              fontWeight: 700,
                              fontSize: '0.75rem'
                            }}>
                              {a.severity}
                            </span>
                            <span style={{ fontWeight: 600, fontSize: '0.85rem' }}>{a.alarm_type}</span>
                          </div>
                        </td>
                        <td style={{ fontSize: '0.85rem', color: '#cbd5e1' }}>
                          {a.message}
                        </td>
                        <td>
                          <span style={{
                            background: a.state === 'UNACK_ALARM' ? 'rgba(244, 63, 94, 0.15)' :
                                        a.state === 'ACKED_ALARM' ? 'rgba(16, 185, 129, 0.15)' :
                                        a.state === 'SHELVED' ? 'rgba(245, 158, 11, 0.15)' : 'rgba(255, 255, 255, 0.1)',
                            color: a.state === 'UNACK_ALARM' ? 'var(--accent-rose)' :
                                   a.state === 'ACKED_ALARM' ? 'var(--accent-emerald)' :
                                   a.state === 'SHELVED' ? 'var(--accent-amber)' : 'var(--text-secondary)',
                            padding: '3px 8px',
                            borderRadius: '4px',
                            fontWeight: 700,
                            fontSize: '0.75rem'
                          }}>
                            {a.state}
                          </span>
                        </td>
                        <td style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                          {new Date(a.active_ts).toLocaleTimeString()}
                        </td>
                        <td>
                          <div style={{ display: 'flex', gap: '6px' }}>
                            {a.state === 'UNACK_ALARM' && (
                              <button className="btn btn-secondary btn-sm" onClick={() => handleAckAlarm(a.id)}>
                                <CheckCircle size={13} style={{ color: 'var(--accent-emerald)' }} />
                                <span>Ack</span>
                              </button>
                            )}
                            {a.state !== 'SHELVED' ? (
                              <button className="btn btn-secondary btn-sm" onClick={() => handleShelveAlarm(a.id, 300000)}>
                                <Clock size={13} style={{ color: 'var(--accent-amber)' }} />
                                <span>Shelve 5m</span>
                              </button>
                            ) : (
                              <button className="btn btn-secondary btn-sm" onClick={() => handleUnshelveAlarm(a.id)}>
                                <RotateCw size={13} style={{ color: 'var(--accent-cyan)' }} />
                                <span>Unshelve</span>
                              </button>
                            )}
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}

      {/* -------------------------------------------------------------------- */}
      {/* TAB 3: TAG NAMESPACE BROWSER                                         */}
      {/* -------------------------------------------------------------------- */}
      {tab === 'tags' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          <div className="card">
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px', flexWrap: 'wrap', gap: '10px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flex: 1, maxWidth: '400px' }}>
                <input 
                  type="text" 
                  className="form-input" 
                  placeholder="Filter by tag path (e.g. WaterTreatment.Pump101)..."
                  value={tagSearch}
                  onChange={e => setTagSearch(e.target.value)}
                />
              </div>
              <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                Showing {filteredTags.length} of {tags.length} process tags
              </span>
            </div>

            <div className="table-container">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Tag Path (Namespace)</th>
                    <th>Data Type</th>
                    <th>Live Value</th>
                    <th>OPC UA Quality</th>
                    <th>Deadband</th>
                    <th>Scan Rate</th>
                    <th>Action</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredTags.map(t => (
                    <tr key={t.id}>
                      <td>
                        <div className="font-mono" style={{ fontWeight: 600, color: '#fff', fontSize: '0.85rem' }}>
                          {t.path}
                        </div>
                      </td>
                      <td>
                        <span className="badge-tag" style={{ fontSize: '0.75rem' }}>
                          {t.data_type}
                        </span>
                      </td>
                      <td>
                        <strong style={{ color: 'var(--accent-cyan)', fontSize: '0.95rem' }}>
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
                      <td style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                        {t.scan_rate_ms ?? 1000} ms
                      </td>
                      <td>
                        <button 
                          className="btn btn-secondary btn-sm"
                          onClick={() => {
                            const val = prompt(`Enter new simulated value for ${t.path}:`, String(t.value));
                            if (val !== null) {
                              const num = parseFloat(val);
                              handleUpdateTag(t.path, isNaN(num) ? val : num);
                            }
                          }}
                        >
                          <Sliders size={13} />
                          <span>Write</span>
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* -------------------------------------------------------------------- */}
      {/* TAB 4: THINGSBOARD SVG SYMBOLS CATALOG                               */}
      {/* -------------------------------------------------------------------- */}
      {tab === 'symbols' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          {/* Category Filter Pills */}
          <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
            {['', 'Pumps', 'Valves', 'Tanks', 'Meters'].map(cat => (
              <button 
                key={cat}
                className={`btn btn-sm ${symbolCategory === cat ? 'btn-primary' : 'btn-secondary'}`}
                onClick={() => setSymbolCategory(cat)}
              >
                {cat === '' ? 'All Categories' : cat}
              </button>
            ))}
          </div>

          {/* Symbol Cards Grid */}
          <div className="symbol-grid">
            {symbols.map(sym => {
              const state = symbolStates[sym.id] || { running: false, alarm: false };
              return (
                <div key={sym.id} className="symbol-card">
                  {/* Card Header */}
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <div style={{ fontWeight: 700, fontSize: '0.9rem', color: '#fff' }}>
                      {sym.name}
                    </div>
                    <span style={{
                      fontSize: '0.7rem',
                      fontWeight: 700,
                      padding: '2px 6px',
                      borderRadius: '4px',
                      background: sym.style === 'isa-101' ? 'rgba(148, 163, 184, 0.15)' : 'rgba(6, 182, 212, 0.15)',
                      color: sym.style === 'isa-101' ? '#94a3b8' : 'var(--accent-cyan)'
                    }}>
                      {sym.style.toUpperCase()}
                    </span>
                  </div>

                  {/* Live SVG Symbol Preview */}
                  <div className="symbol-preview-box" dangerouslySetInnerHTML={{ __html: sym.svg_content }} />

                  {/* Metadata Pills */}
                  <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
                    <span className="symbol-meta-pill">Category: {sym.category}</span>
                    <span className="symbol-meta-pill">{sym.tags?.length ?? 0} Tags</span>
                    <span className="symbol-meta-pill">{sym.properties?.length ?? 0} Props</span>
                    <span className="symbol-meta-pill">{sym.behaviors?.length ?? 0} Behaviors</span>
                  </div>

                  {/* Interactive Preview Toggles */}
                  <div style={{ display: 'flex', gap: '8px', marginTop: 'auto' }}>
                    <button 
                      className={`btn btn-sm ${state.running ? 'btn-primary' : 'btn-secondary'}`}
                      style={{ flex: 1 }}
                      onClick={() => setSymbolStates({
                        ...symbolStates,
                        [sym.id]: { ...state, running: !state.running }
                      })}
                    >
                      {state.running ? 'Running' : 'Stopped'}
                    </button>
                    <button 
                      className="btn btn-secondary btn-sm"
                      onClick={() => setInspectedSymbol(sym)}
                    >
                      <FileCode size={13} />
                      <span>Meta</span>
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* -------------------------------------------------------------------- */}
      {/* TAB 5: SELECT-BEFORE-OPERATE (SBO) & AUDIT JOURNAL                   */}
      {/* -------------------------------------------------------------------- */}
      {tab === 'sbo' && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(420px, 1fr))', gap: '20px' }}>
          {/* SBO Station */}
          <div className="card">
            <h3 style={{ fontSize: '1.05rem', fontWeight: 700, marginBottom: '6px', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Power size={18} style={{ color: 'var(--accent-amber)' }} />
              <span>Select-Before-Operate Safe Control Station</span>
            </h3>
            <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '16px' }}>
              Two-phase reservation & confirm sequence with 60s timeout to avoid unintended industrial command execution.
            </p>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
              <div className="form-group" style={{ margin: 0 }}>
                <label className="form-label">Control Target Tag</label>
                <select 
                  className="form-select font-mono"
                  value={sboTargetTag}
                  onChange={e => setSboTargetTag(e.target.value)}
                >
                  <option value="WaterTreatment.Pump101.Status">WaterTreatment.Pump101.Status</option>
                  <option value="WaterTreatment.Valve101.Position">WaterTreatment.Valve101.Position</option>
                  <option value="WaterTreatment.Chemical.DosingSpeed">WaterTreatment.Chemical.DosingSpeed</option>
                </select>
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
                    <span>START / ACTUATE (1)</span>
                  </button>
                  <button 
                    type="button"
                    className={`btn ${sboDesiredValue === 0 ? 'btn-primary' : 'btn-secondary'}`}
                    style={{ flex: 1 }}
                    onClick={() => setSboDesiredValue(0)}
                  >
                    <StopCircle size={15} />
                    <span>STOP / CLOSE (0)</span>
                  </button>
                </div>
              </div>

              {/* Interlock Check */}
              <div style={{
                background: 'rgba(16, 185, 129, 0.08)',
                border: '1px solid rgba(16, 185, 129, 0.25)',
                padding: '10px 14px',
                borderRadius: 'var(--radius-md)',
                display: 'flex',
                alignItems: 'center',
                gap: '8px',
                fontSize: '0.8rem',
                color: 'var(--accent-emerald)'
              }}>
                <CheckCircle size={15} />
                <span>Interlock Checks Passed: Low Tank Level interlock cleared, permissives OK.</span>
              </div>

              {/* Phase 1 or Phase 2 */}
              {(!activeCommand || activeCommand.status !== 'SELECTED' || sboTimer === 0) ? (
                <button 
                  className="btn btn-secondary" 
                  style={{ padding: '12px', borderColor: 'var(--accent-amber)' }}
                  onClick={() => handleSelectSBO()}
                >
                  <ShieldAlert size={16} style={{ color: 'var(--accent-amber)' }} />
                  <span>Phase 1: Arm & Reserve Control Token</span>
                </button>
              ) : (
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

          {/* SBO Control Audit Journal */}
          <div className="card">
            <h3 style={{ fontSize: '1.05rem', fontWeight: 700, marginBottom: '14px', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <History size={18} style={{ color: 'var(--accent-cyan)' }} />
              <span>SBO Control Audit Trail</span>
            </h3>

            {controlLogs.length === 0 ? (
              <div style={{ padding: '30px', textAlign: 'center', color: 'var(--text-muted)' }}>
                No control operations logged yet.
              </div>
            ) : (
              <div className="table-container" style={{ maxHeight: '380px', overflowY: 'auto' }}>
                <table className="data-table">
                  <thead>
                    <tr>
                      <th>Operator</th>
                      <th>Target Tag</th>
                      <th>Desired Value</th>
                      <th>Status</th>
                      <th>Timestamp</th>
                    </tr>
                  </thead>
                  <tbody>
                    {controlLogs.map(log => (
                      <tr key={log.id}>
                        <td style={{ fontWeight: 600, color: '#fff', fontSize: '0.8rem' }}>
                          {log.operator}
                        </td>
                        <td className="font-mono" style={{ fontSize: '0.8rem' }}>
                          {log.tag_path}
                        </td>
                        <td style={{ fontSize: '0.85rem', color: 'var(--accent-cyan)', fontWeight: 700 }}>
                          {String(log.desired_value)}
                        </td>
                        <td>
                          <span style={{
                            background: log.status === 'OPERATED' ? 'rgba(16, 185, 129, 0.15)' :
                                        log.status === 'SELECTED' ? 'rgba(245, 158, 11, 0.15)' : 'rgba(244, 63, 94, 0.15)',
                            color: log.status === 'OPERATED' ? 'var(--accent-emerald)' :
                                   log.status === 'SELECTED' ? 'var(--accent-amber)' : 'var(--accent-rose)',
                            padding: '2px 6px',
                            borderRadius: '4px',
                            fontSize: '0.75rem',
                            fontWeight: 700
                          }}>
                            {log.status}
                          </span>
                        </td>
                        <td style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                          {new Date(log.selected_at).toLocaleTimeString()}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}

      {/* -------------------------------------------------------------------- */}
      {/* MODAL: EQUIPMENT FACEPLATE                                           */}
      {/* -------------------------------------------------------------------- */}
      {faceplateItem && (
        <div className="faceplate-backdrop" onClick={() => setFaceplateItem(null)}>
          <div className="faceplate-dialog" onClick={e => e.stopPropagation()}>
            <div className="faceplate-header">
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <Gauge size={18} style={{ color: 'var(--accent-cyan)' }} />
                <strong style={{ fontSize: '1rem', color: '#fff' }}>{faceplateItem.title}</strong>
              </div>
              <button className="btn btn-secondary btn-sm" onClick={() => setFaceplateItem(null)}>
                <X size={14} />
              </button>
            </div>

            <div className="faceplate-body">
              {/* Dynamic tag path resolution for faceplate */}
              {(() => {
                const eq = canvasEquipment.find(e => e.id === faceplateItem.id);
                const boundPath = eq ? eq.tagPath : 
                  faceplateItem.id === 'Pump101' ? 'WaterTreatment.Pump101.Status' :
                  faceplateItem.id === 'Valve101' ? 'WaterTreatment.Valve101.Position' :
                  faceplateItem.id === 'Tank101' ? 'WaterTreatment.Tank101.Level' :
                  faceplateItem.id === 'ChemSkid' ? 'WaterTreatment.Chemical.AcidLevel' : '';
                return boundPath ? (
                  <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', background: 'rgba(0,0,0,0.3)', padding: '6px 10px', borderRadius: '4px', fontFamily: 'monospace' }}>
                    Bound Tag: <span style={{ color: 'var(--accent-cyan)' }}>{boundPath}</span>
                  </div>
                ) : null;
              })()}

              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', background: 'rgba(255, 255, 255, 0.03)', padding: '12px', borderRadius: 'var(--radius-md)' }}>
                <div>
                  <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>Control Mode</div>
                  <strong style={{ color: 'var(--accent-cyan)' }}>REMOTE SUPERVISORY (SBO)</strong>
                </div>
                <div style={{ display: 'flex', gap: '6px' }}>
                  <button className="btn btn-secondary btn-sm" style={{ background: 'var(--accent-cyan)', color: '#000' }}>Manual</button>
                  <button className="btn btn-secondary btn-sm">Auto</button>
                </div>
              </div>

              {/* Equipment Specific Faceplate Controls */}
              {faceplateItem.type === 'pump' && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem' }}>
                    <span>Current Status:</span>
                    <strong style={{ color: isPumpRunning ? 'var(--accent-emerald)' : 'var(--accent-rose)' }}>
                      {isPumpRunning ? 'RUNNING (1)' : 'STOPPED (0)'}
                    </strong>
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem' }}>
                    <span>Discharge Flow Rate:</span>
                    <strong style={{ color: '#fff' }}>{flowVal.toFixed(1)} GPM</strong>
                  </div>

                  <div style={{ display: 'flex', gap: '10px', marginTop: '10px' }}>
                    <button 
                      className="btn btn-primary" 
                      style={{ flex: 1 }}
                      onClick={() => {
                        const eq = canvasEquipment.find(e => e.id === faceplateItem.id);
                        const tag = eq ? eq.tagPath : 'WaterTreatment.Pump101.Status';
                        handleSelectSBO(tag, 1);
                        setTab('sbo');
                        setFaceplateItem(null);
                      }}
                    >
                      <PlayCircle size={15} />
                      <span>Start Pump via SBO</span>
                    </button>
                    <button 
                      className="btn btn-secondary" 
                      style={{ flex: 1, borderColor: 'var(--accent-rose)', color: 'var(--accent-rose)' }}
                      onClick={() => {
                        const eq = canvasEquipment.find(e => e.id === faceplateItem.id);
                        const tag = eq ? eq.tagPath : 'WaterTreatment.Pump101.Status';
                        handleSelectSBO(tag, 0);
                        setTab('sbo');
                        setFaceplateItem(null);
                      }}
                    >
                      <StopCircle size={15} />
                      <span>Stop Pump</span>
                    </button>
                  </div>
                </div>
              )}

              {faceplateItem.type === 'valve' && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem' }}>
                    <span>Valve Position Setpoint:</span>
                    <strong style={{ color: 'var(--accent-cyan)' }}>{valvePosition.toFixed(0)} %</strong>
                  </div>
                  <input 
                    type="range" 
                    min="0" 
                    max="100" 
                    value={valvePosition}
                    onChange={e => {
                      const v = parseFloat(e.target.value);
                      const eq = canvasEquipment.find(e => e.id === faceplateItem.id);
                      const tag = eq ? eq.tagPath : 'WaterTreatment.Valve101.Position';
                      handleUpdateTag(tag, v);
                    }}
                    style={{ width: '100%' }}
                  />
                  <div style={{ display: 'flex', gap: '8px' }}>
                    <button className="btn btn-secondary btn-sm" style={{ flex: 1 }} onClick={() => {
                      const eq = canvasEquipment.find(e => e.id === faceplateItem.id);
                      handleUpdateTag(eq ? eq.tagPath : 'WaterTreatment.Valve101.Position', 0);
                    }}>Close (0%)</button>
                    <button className="btn btn-secondary btn-sm" style={{ flex: 1 }} onClick={() => {
                      const eq = canvasEquipment.find(e => e.id === faceplateItem.id);
                      handleUpdateTag(eq ? eq.tagPath : 'WaterTreatment.Valve101.Position', 50);
                    }}>50%</button>
                    <button className="btn btn-secondary btn-sm" style={{ flex: 1 }} onClick={() => {
                      const eq = canvasEquipment.find(e => e.id === faceplateItem.id);
                      handleUpdateTag(eq ? eq.tagPath : 'WaterTreatment.Valve101.Position', 100);
                    }}>Full Open (100%)</button>
                  </div>
                </div>
              )}

              {faceplateItem.type === 'tank' && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem' }}>
                    <span>Liquid Level:</span>
                    <strong style={{ color: tankLevel > 90 ? 'var(--accent-rose)' : 'var(--accent-cyan)' }}>
                      {tankLevel.toFixed(1)} %
                    </strong>
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem' }}>
                    <span>Calculated Volume:</span>
                    <strong style={{ color: '#fff' }}>{(tankLevel * 200).toFixed(0)} L</strong>
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem' }}>
                    <span>Alarm Threshold:</span>
                    <span style={{ color: 'var(--accent-rose)' }}>90.0% HI-HI</span>
                  </div>
                </div>
              )}

              {faceplateItem.type === 'meter' && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem' }}>
                    <span>OPC UA Quality:</span>
                    <span style={{ color: 'var(--accent-emerald)', fontWeight: 700 }}>GOOD (192)</span>
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem' }}>
                    <span>Scan Frequency:</span>
                    <span style={{ color: 'var(--text-secondary)' }}>1000 ms</span>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* -------------------------------------------------------------------- */}
      {/* MODAL: SYMBOL METADATA INSPECTOR                                     */}
      {/* -------------------------------------------------------------------- */}
      {inspectedSymbol && (
        <div className="faceplate-backdrop" onClick={() => setInspectedSymbol(null)}>
          <div className="faceplate-dialog" style={{ maxWidth: '640px' }} onClick={e => e.stopPropagation()}>
            <div className="faceplate-header">
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <FileCode size={18} style={{ color: 'var(--accent-cyan)' }} />
                <strong style={{ fontSize: '1rem', color: '#fff' }}>ThingsBoard Symbol Metadata: {inspectedSymbol.name}</strong>
              </div>
              <button className="btn btn-secondary btn-sm" onClick={() => setInspectedSymbol(null)}>
                <X size={14} />
              </button>
            </div>

            <div className="faceplate-body">
              <pre style={{
                background: '#020617',
                padding: '16px',
                borderRadius: 'var(--radius-md)',
                color: '#38bdf8',
                fontSize: '0.8rem',
                overflowX: 'auto',
                maxHeight: '340px'
              }}>
                {JSON.stringify({
                  title: inspectedSymbol.name,
                  category: inspectedSymbol.category,
                  style: inspectedSymbol.style,
                  tags: inspectedSymbol.tags,
                  properties: inspectedSymbol.properties,
                  behaviors: inspectedSymbol.behaviors,
                  animations: inspectedSymbol.animations
                }, null, 2)}
              </pre>
            </div>
          </div>
        </div>
      )}

      {/* -------------------------------------------------------------------- */}
      {/* MODAL: REGISTER NEW SCADA TAG                                        */}
      {/* -------------------------------------------------------------------- */}
      {showNewTagModal && (
        <div className="faceplate-backdrop" onClick={() => setShowNewTagModal(false)}>
          <div className="faceplate-dialog" onClick={e => e.stopPropagation()}>
            <div className="faceplate-header">
              <strong style={{ fontSize: '1rem', color: '#fff' }}>Register SCADA Process Tag</strong>
              <button className="btn btn-secondary btn-sm" onClick={() => setShowNewTagModal(false)}>
                <X size={14} />
              </button>
            </div>

            <form onSubmit={handleCreateTag} className="faceplate-body">
              <div className="form-group">
                <label className="form-label">Tag Path (Hierarchical)</label>
                <input 
                  type="text" 
                  className="form-input font-mono" 
                  placeholder="e.g. Plant.Area1.Pump102.Speed"
                  value={newTagForm.path}
                  onChange={e => setNewTagForm({ ...newTagForm, path: e.target.value })}
                  required
                />
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                <div className="form-group">
                  <label className="form-label">Data Type</label>
                  <select 
                    className="form-select"
                    value={newTagForm.data_type}
                    onChange={e => setNewTagForm({ ...newTagForm, data_type: e.target.value })}
                  >
                    <option value="ANALOG">ANALOG</option>
                    <option value="DISCRETE">DISCRETE</option>
                    <option value="STRING">STRING</option>
                  </select>
                </div>

                <div className="form-group">
                  <label className="form-label">Engineering Unit</label>
                  <input 
                    type="text" 
                    className="form-input" 
                    placeholder="e.g. GPM, %, PSI, °C"
                    value={newTagForm.eng_unit}
                    onChange={e => setNewTagForm({ ...newTagForm, eng_unit: e.target.value })}
                  />
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                <div className="form-group">
                  <label className="form-label">Deadband</label>
                  <input 
                    type="number" 
                    step="0.1"
                    className="form-input" 
                    value={newTagForm.deadband}
                    onChange={e => setNewTagForm({ ...newTagForm, deadband: parseFloat(e.target.value) })}
                  />
                </div>

                <div className="form-group">
                  <label className="form-label">Initial Value</label>
                  <input 
                    type="number" 
                    step="0.1"
                    className="form-input" 
                    value={newTagForm.value}
                    onChange={e => setNewTagForm({ ...newTagForm, value: parseFloat(e.target.value) })}
                  />
                </div>
              </div>

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '10px' }}>
                <button type="button" className="btn btn-secondary" onClick={() => setShowNewTagModal(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary">
                  Register Tag
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
