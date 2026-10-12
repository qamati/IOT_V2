import React, { useState, useEffect, useRef } from 'react';
import {
  GitMerge,
  Search,
  Plus,
  Trash2,
  Settings,
  Check,
  X,
  Bug,
  RotateCcw,
  Maximize2,
  ChevronDown,
  ChevronRight,
  ChevronLeft,
  Filter,
  Layers,
  Cpu,
  Zap,
  Send,
  Workflow,
  LogIn,
  Play,
  Share2
} from 'lucide-react';
import { api } from '../api';
import type {
  RuleChain,
  RuleChainRevision,
  RuleDescriptor,
  ExecutionTrace,
  RuleNode,
  RuleConnection
} from '../api';
import './RuleEngineView.css';

const DEFAULT_CONFIGS: Record<string, any> = {
  'filter.alarm_status': { statuses: ['ACTIVE_UNACK', 'ACTIVE_ACK'] },
  'filter.asset_profile_switch': {},
  'filter.check_fields_presence': { fields: ['temperature', 'humidity'] },
  'filter.check_relation_presence': {},
  'filter.device_profile_switch': {},
  'filter.entity_type_filter': { entityTypes: ['DEVICE'] },
  'filter.entity_type_switch': {},
  'filter.gps_geofencing_filter': { minLat: 32.0, maxLat: 33.0, minLon: 13.0, maxLon: 14.0 },
  'filter.msg_type': { messageTypes: ['POST_TELEMETRY_REQUEST'] },
  'filter.msg_type_switch': {},
  'filter.script': { field: 'temperature', op: '>', value: 50 },
  'filter.switch': {},
  'enrich.originator_attributes': { scope: 'SERVER_SCOPE', target: 'metadata' },
  'action.save_timeseries': { useServerTs: false },
  'action.save_attributes': { scope: 'SERVER_SCOPE' },
  'action.create_alarm': { alarmType: 'Critical Overheat', severity: 'CRITICAL' },
  'action.clear_alarm': { alarmType: 'Critical Overheat' },
  'action.log': { level: 'INFO' },
  'external.rest_call': { method: 'POST', url: 'https://httpbin.org/post', timeoutMs: 5000 },
  'flow.output': { relation: 'Success' },
  'flow.rule_chain': { ruleChainId: '' }
};

const SAMPLE_PAYLOADS = {
  'High Temperature Overheat': {
    msgType: 'POST_TELEMETRY_REQUEST',
    data: { temperature: 78.4, humidity: 42.1, battery: 85 },
  },
  'Normal Operating Telemetry': {
    msgType: 'POST_TELEMETRY_REQUEST',
    data: { temperature: 24.5, humidity: 55.0, battery: 96 },
  },
  'Low Battery Critical': {
    msgType: 'POST_TELEMETRY_REQUEST',
    data: { battery: 11.2, voltage: 3.25 },
  },
};

export function RuleEngineView() {
  const [chains, setChains] = useState<RuleChain[]>([]);
  const [activeChain, setActiveChain] = useState<RuleChain | null>(null);
  const [activeRevision, setActiveRevision] = useState<RuleChainRevision | null>(null);
  const [descriptors, setDescriptors] = useState<RuleDescriptor[]>([]);
  const [saving, setSaving] = useState(false);
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false);

  // UI state
  const [chainMenuOpen, setChainMenuOpen] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [showTopCallout, setShowTopCallout] = useState(true);
  const [showBottomCallout, setShowBottomCallout] = useState(true);
  const [expandedCategories, setExpandedCategories] = useState<Record<string, boolean>>({
    Filter: true,
    Enrichment: false,
    Transformation: false,
    Action: false,
    External: false,
    Flow: false,
  });

  // Modal & Drawer states
  const [showCreateChainModal, setShowCreateChainModal] = useState(false);
  const [newChainName, setNewChainName] = useState('');
  const [newChainDesc, setNewChainDesc] = useState('');
  const [newChainIsRoot, setNewChainIsRoot] = useState(false);

  const [editingNodeId, setEditingNodeId] = useState<string | null>(null);
  const [editNodeName, setEditNodeName] = useState('');
  const [editNodeConfig, setEditNodeConfig] = useState('');

  // Simulator Drawer state
  const [showSimulator, setShowSimulator] = useState(false);
  const [selectedPreset, setSelectedPreset] = useState<string>('High Temperature Overheat');
  const [testTrace, setTestTrace] = useState<ExecutionTrace | null>(null);
  const [testingMsg, setTestingMsg] = useState(false);

  // Connecting ports state (Port click linking)
  const [connectingFromId, setConnectingFromId] = useState<string | null>(null); // 'INPUT' = Input node

  // Node Dragging state on canvas
  const canvasViewportRef = useRef<HTMLDivElement>(null);
  const [draggingNodeId, setDraggingNodeId] = useState<string | null>(null); // 'INPUT' = Input node
  const [dragOffset, setDragOffset] = useState<{ x: number; y: number }>({ x: 0, y: 0 });
  const [inputNodePos, setInputNodePos] = useState<{ x: number; y: number }>({ x: 80, y: 80 });

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    try {
      const [chainsRes, descs] = await Promise.all([
        api.listRuleChains(),
        api.listRuleDescriptors(),
      ]);
      const chainList = chainsRes?.data || [];
      setChains(chainList);
      setDescriptors(descs || []);

      if (chainList.length > 0) {
        const root = chainList.find((c) => c.is_root) || chainList[0];
        selectChain(root);
      }
    } catch (err) {
      console.error('Failed to load rule chains:', err);
    }
  };

  const selectChain = async (chain: RuleChain) => {
    setActiveChain(chain);
    setChainMenuOpen(false);
    setHasUnsavedChanges(false);
    setConnectingFromId(null);
    try {
      const rev = await api.getRuleChainMetadata(chain.id);
      setActiveRevision(rev);
    } catch (err) {
      console.error('Failed to load revision metadata:', err);
    }
  };

  // Node Drag Handlers on canvas
  const handleNodeMouseDown = (e: React.MouseEvent, nodeId: string) => {
    e.stopPropagation();
    let currentX = 100;
    let currentY = 100;

    if (nodeId === 'INPUT') {
      currentX = inputNodePos.x;
      currentY = inputNodePos.y;
    } else {
      const found = activeRevision?.definition.nodes.find((n) => n.id === nodeId);
      currentX = found?.additionalInfo?.layoutX ?? 200;
      currentY = found?.additionalInfo?.layoutY ?? 100;
    }

    setDraggingNodeId(nodeId);
    setDragOffset({
      x: e.clientX - currentX,
      y: e.clientY - currentY,
    });
  };

  const handleCanvasMouseMove = (e: React.MouseEvent) => {
    if (draggingNodeId === null || !activeRevision) return;

    // Snap to 10px grid
    const newX = Math.max(20, Math.round((e.clientX - dragOffset.x) / 10) * 10);
    const newY = Math.max(20, Math.round((e.clientY - dragOffset.y) / 10) * 10);

    if (draggingNodeId === 'INPUT') {
      setInputNodePos({ x: newX, y: newY });
      return;
    }

    const updatedNodes = activeRevision.definition.nodes.map((node) => {
      if (node.id === draggingNodeId) {
        return {
          ...node,
          additionalInfo: {
            ...node.additionalInfo,
            layoutX: newX,
            layoutY: newY,
          },
        };
      }
      return node;
    });

    setActiveRevision({
      ...activeRevision,
      definition: {
        ...activeRevision.definition,
        nodes: updatedNodes,
      },
    });
    setHasUnsavedChanges(true);
  };

  const handleCanvasMouseUp = () => {
    setDraggingNodeId(null);
  };

  // Drag and drop from sidebar to canvas
  const handleSidebarDragStart = (e: React.DragEvent, desc: RuleDescriptor) => {
    e.dataTransfer.setData('application/json', JSON.stringify(desc));
  };

  const handleCanvasDrop = (e: React.DragEvent) => {
    e.preventDefault();
    if (!activeRevision || !canvasViewportRef.current) return;

    const data = e.dataTransfer.getData('application/json');
    if (!data) return;

    try {
      const desc: RuleDescriptor = JSON.parse(data);
      const rect = canvasViewportRef.current.getBoundingClientRect();
      const dropX = Math.max(40, Math.round((e.clientX - rect.left + canvasViewportRef.current.scrollLeft) / 10) * 10);
      const dropY = Math.max(40, Math.round((e.clientY - rect.top + canvasViewportRef.current.scrollTop) / 10) * 10);

      const newNode: RuleNode = {
        id: `node_${Date.now()}_${Math.random().toString(36).substring(2, 6)}`,
        type: desc.type,
        name: desc.name,
        configuration: DEFAULT_CONFIGS[desc.type] || {},
        additionalInfo: {
          layoutX: dropX,
          layoutY: dropY,
        },
      };

      const updatedNodes = [...activeRevision.definition.nodes, newNode];
      setActiveRevision({
        ...activeRevision,
        definition: {
          ...activeRevision.definition,
          nodes: updatedNodes,
        },
      });
      setHasUnsavedChanges(true);
      setShowTopCallout(false);
    } catch (err) {
      console.error('Failed to parse dropped node:', err);
    }
  };

  // Add node from sidebar via click
  const handleAddNodeToCanvas = (desc: RuleDescriptor) => {
    if (!activeRevision) return;

    const existingCount = activeRevision.definition.nodes.length;
    const posX = 280 + (existingCount % 4) * 200;
    const posY = 80 + Math.floor(existingCount / 4) * 120;

    const newNode: RuleNode = {
      id: `node_${Date.now()}_${Math.random().toString(36).substring(2, 6)}`,
      type: desc.type,
      name: desc.name,
      configuration: DEFAULT_CONFIGS[desc.type] || {},
      additionalInfo: {
        layoutX: posX,
        layoutY: posY,
      },
    };

    const updatedNodes = [...activeRevision.definition.nodes, newNode];
    setActiveRevision({
      ...activeRevision,
      definition: {
        ...activeRevision.definition,
        nodes: updatedNodes,
      },
    });
    setHasUnsavedChanges(true);
    setShowTopCallout(false);
  };

  // Port linking (Click right port -> Click left port)
  const handlePortClick = (e: React.MouseEvent, nodeId: string, portType: 'in' | 'out') => {
    e.stopPropagation();

    if (portType === 'out') {
      setConnectingFromId(nodeId);
    } else if (portType === 'in' && connectingFromId !== null) {
      if (connectingFromId === nodeId) {
        setConnectingFromId(null);
        return;
      }

      if (connectingFromId === 'INPUT') {
        setActiveRevision({
          ...activeRevision!,
          definition: {
            ...activeRevision!.definition,
            firstNodeId: nodeId,
          },
        });
      } else {
        const fromNode = activeRevision?.definition.nodes.find((n) => n.id === connectingFromId);
        const defaultLabel = fromNode?.type.startsWith('filter') ? 'True' : 'Success';

        const newConn: RuleConnection = {
          from: connectingFromId,
          to: nodeId,
          label: defaultLabel,
        };
        const updatedConnections = [...(activeRevision?.definition.connections || []), newConn];
        setActiveRevision({
          ...activeRevision!,
          definition: {
            ...activeRevision!.definition,
            connections: updatedConnections,
          },
        });
      }

      setConnectingFromId(null);
      setHasUnsavedChanges(true);
      setShowBottomCallout(true);
    }
  };

  // Delete node
  const handleDeleteNode = (e: React.MouseEvent, nodeId: string) => {
    e.stopPropagation();
    if (!activeRevision) return;

    const updatedNodes = activeRevision.definition.nodes.filter((n) => n.id !== nodeId);
    const updatedConns = activeRevision.definition.connections.filter(
      (c) => c.from !== nodeId && c.to !== nodeId
    );

    let firstId = activeRevision.definition.firstNodeId;
    if (firstId === nodeId) {
      firstId = updatedNodes.length > 0 ? updatedNodes[0].id : '';
    }

    setActiveRevision({
      ...activeRevision,
      definition: {
        ...activeRevision.definition,
        firstNodeId: firstId,
        nodes: updatedNodes,
        connections: updatedConns,
      },
    });
    setHasUnsavedChanges(true);
  };

  // Delete connection
  const handleDeleteConnection = (e: React.MouseEvent, index: number) => {
    e.stopPropagation();
    if (!activeRevision) return;

    const updatedConns = activeRevision.definition.connections.filter((_, idx) => idx !== index);
    setActiveRevision({
      ...activeRevision,
      definition: {
        ...activeRevision.definition,
        connections: updatedConns,
      },
    });
    setHasUnsavedChanges(true);
  };

  // Apply changes (Save)
  const handleApplyChanges = async () => {
    if (!activeChain || !activeRevision) return;
    setSaving(true);
    try {
      const saved = await api.saveRuleChainMetadata(activeChain.id, activeRevision.definition);
      setActiveRevision(saved);
      setHasUnsavedChanges(false);
      setShowBottomCallout(false);
      alert('Rule chain configuration successfully saved and applied!');
    } catch (err: any) {
      alert(`Failed to save rule chain: ${err.message || 'error'}`);
    } finally {
      setSaving(false);
    }
  };

  // Revert changes
  const handleRevertChanges = async () => {
    if (!activeChain) return;
    try {
      const rev = await api.getRuleChainMetadata(activeChain.id);
      setActiveRevision(rev);
      setHasUnsavedChanges(false);
      setConnectingFromId(null);
    } catch (err) {
      console.error('Failed to revert:', err);
    }
  };

  // Toggle debug mode
  const handleToggleDebug = async () => {
    if (!activeChain) return;
    const isDebug = !activeChain.additional_info?.debug_mode;
    try {
      await api.createRuleChain({
        name: activeChain.name,
        type: activeChain.type,
        is_root: activeChain.is_root,
        additional_info: { ...(activeChain.additional_info || {}), debug_mode: isDebug },
      });
      setActiveChain({
        ...activeChain,
        additional_info: { ...(activeChain.additional_info || {}), debug_mode: isDebug },
      });
    } catch (err) {
      console.error('Failed to toggle debug:', err);
    }
  };

  // Create chain
  const handleCreateChainSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newChainName.trim()) return;

    try {
      const created = await api.createRuleChain({
        name: newChainName.trim(),
        type: 'CORE',
        is_root: newChainIsRoot,
        additional_info: { description: newChainDesc.trim(), debug_mode: false },
      });
      setShowCreateChainModal(false);
      setNewChainName('');
      setNewChainDesc('');
      await loadData();
      selectChain(created);
    } catch (err: any) {
      alert(`Failed to create rule chain: ${err.message || 'error'}`);
    }
  };

  // Run test simulation
  const handleRunTest = async () => {
    if (!activeChain) return;
    setTestingMsg(true);
    try {
      const payload = SAMPLE_PAYLOADS[selectedPreset as keyof typeof SAMPLE_PAYLOADS];
      const trace = await api.testRuleChainById(activeChain.id, {
        msgType: payload.msgType,
        data: payload.data,
      });
      setTestTrace(trace);
    } catch (err: any) {
      alert(`Simulation failed: ${err.message || 'error'}`);
    } finally {
      setTestingMsg(false);
    }
  };

  // Helper: Category to node style
  const getCategoryClass = (category: string) => {
    switch (category?.toUpperCase()) {
      case 'FILTER': return 'tb-node-filter';
      case 'ENRICHMENT': return 'tb-node-enrichment';
      case 'TRANSFORMATION': return 'tb-node-transform';
      case 'ACTION': return 'tb-node-action';
      case 'EXTERNAL': return 'tb-node-external';
      case 'FLOW': return 'tb-node-flow';
      default: return 'tb-node-filter';
    }
  };

  // Helper: Category to icon
  const getCategoryIcon = (category: string) => {
    switch (category?.toUpperCase()) {
      case 'FILTER': return <Filter size={14} />;
      case 'ENRICHMENT': return <Layers size={14} />;
      case 'TRANSFORMATION': return <Cpu size={14} />;
      case 'ACTION': return <Zap size={14} />;
      case 'EXTERNAL': return <Send size={14} />;
      case 'FLOW': return <Workflow size={14} />;
      default: return <Workflow size={14} />;
    }
  };

  // Group descriptors by category
  const categories = ['Filter', 'Enrichment', 'Transformation', 'Action', 'External', 'Flow'];
  const groupedDescriptors = categories.reduce((acc, cat) => {
    acc[cat] = descriptors.filter(
      (d) =>
        d.category.toLowerCase() === cat.toLowerCase() &&
        (d.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
          d.type.toLowerCase().includes(searchQuery.toLowerCase()))
    );
    return acc;
  }, {} as Record<string, RuleDescriptor[]>);

  const nodes = activeRevision?.definition.nodes || [];
  const connections = activeRevision?.definition.connections || [];
  const firstNode = nodes.find((n) => n.id === activeRevision?.definition.firstNodeId) || nodes[0];

  return (
    <div className="tb-studio-container">
      {/* 1. TOP NAVIGATION / BREADCRUMBS BAR (Thingsboard Teal #00695c) */}
      <div className="tb-top-bar">
        <div className="tb-top-left">
          <div className="tb-brand-badge">
            <Share2 size={18} />
            <span>ThingsBoard Edge</span>
          </div>

          <div className="tb-breadcrumbs">
            <div className="tb-breadcrumb-link" onClick={() => setChainMenuOpen(!chainMenuOpen)}>
              <GitMerge size={16} />
              <span>Rule chains</span>
            </div>
            <span className="tb-breadcrumb-sep">&gt;</span>
            <div
              className="tb-chain-dropdown-trigger"
              onClick={() => setChainMenuOpen(!chainMenuOpen)}
            >
              <Workflow size={15} />
              <span>{activeChain ? activeChain.name : 'Select Rule Chain'}</span>
              {activeChain?.is_root && <span className="tb-root-badge">ROOT</span>}
              <ChevronDown size={13} />
            </div>
          </div>
        </div>

        <div className="tb-top-right">
          <div className="tb-user-profile">
            <div className="tb-user-avatar">T</div>
            <div style={{ display: 'flex', flexDirection: 'column', lineHeight: 1.2 }}>
              <span style={{ fontWeight: 600 }}>tenant@thingsboard.org</span>
              <span style={{ fontSize: '0.7rem', opacity: 0.8 }}>Tenant administrator</span>
            </div>
          </div>
        </div>
      </div>

      {/* Popover Chain Selector */}
      {chainMenuOpen && (
        <div className="tb-chain-dropdown-menu">
          <div style={{ padding: '6px 10px', fontSize: '0.72rem', fontWeight: 700, color: '#64748b' }}>
            AVAILABLE RULE CHAINS
          </div>
          {chains.map((c) => (
            <div
              key={c.id}
              className={`tb-dropdown-item ${activeChain?.id === c.id ? 'selected' : ''}`}
              onClick={() => selectChain(c)}
            >
              <span>{c.name}</span>
              {c.is_root && <span className="tb-root-badge">ROOT</span>}
            </div>
          ))}
          <div className="tb-dropdown-divider" />
          <div
            className="tb-dropdown-item"
            style={{ color: '#00695c', fontWeight: 600 }}
            onClick={() => {
              setChainMenuOpen(false);
              setShowCreateChainModal(true);
            }}
          >
            <span style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
              <Plus size={14} /> Create New Rule Chain
            </span>
          </div>
        </div>
      )}

      {/* 2. MAIN WORKSPACE: SIDEBAR & GRID CANVAS */}
      <div className="tb-workspace">
        {/* Left Sidebar: Node Catalog */}
        <div className={`tb-sidebar ${sidebarCollapsed ? 'collapsed' : ''}`}>
          <div className="tb-search-bar">
            <div className="tb-search-input-wrapper">
              <Search className="tb-search-icon" size={15} />
              <input
                type="text"
                placeholder="Search nodes"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="tb-search-input"
              />
            </div>
          </div>

          <div className="tb-catalog-list">
            {categories.map((cat) => {
              const catNodes = groupedDescriptors[cat] || [];
              const isExpanded = expandedCategories[cat] !== false;

              return (
                <div key={cat} className="tb-category-group">
                  <div
                    className="tb-category-header"
                    onClick={() =>
                      setExpandedCategories({
                        ...expandedCategories,
                        [cat]: !isExpanded,
                      })
                    }
                  >
                    <div className="tb-category-title">
                      {getCategoryIcon(cat)}
                      <span>{cat}</span>
                    </div>
                    {isExpanded ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
                  </div>

                  {isExpanded && (
                    <div className="tb-category-nodes">
                      {catNodes.map((desc) => (
                        <div
                          key={desc.type}
                          className={`tb-node-pill ${getCategoryClass(cat)}`}
                          title={`Drag or click to place on canvas\n${desc.name}`}
                          draggable
                          onDragStart={(e) => handleSidebarDragStart(e, desc)}
                          onClick={() => handleAddNodeToCanvas(desc)}
                        >
                          <span className="tb-port-nub tb-port-nub-in" />
                          <span className="tb-node-icon">{getCategoryIcon(cat)}</span>
                          <span className="tb-node-label">{desc.name.toLowerCase()}</span>
                          <span className="tb-port-nub tb-port-nub-out" />
                        </div>
                      ))}
                      {catNodes.length === 0 && (
                        <div style={{ fontSize: '0.72rem', color: '#94a3b8', padding: '4px 6px' }}>
                          No matching nodes
                        </div>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>

          {/* Semicircular Collapse Handle */}
          <div
            className="tb-sidebar-toggle-handle"
            onClick={() => setSidebarCollapsed(!sidebarCollapsed)}
            title={sidebarCollapsed ? 'Expand catalog' : 'Collapse catalog'}
          >
            {sidebarCollapsed ? <ChevronRight size={13} /> : <ChevronLeft size={13} />}
          </div>
        </div>

        {/* Center / Right Canvas Area */}
        <div
          className="tb-canvas-viewport"
          ref={canvasViewportRef}
          onMouseMove={handleCanvasMouseMove}
          onMouseUp={handleCanvasMouseUp}
          onDragOver={(e) => e.preventDefault()}
          onDrop={handleCanvasDrop}
        >
          <div className="tb-canvas-board">
            {/* SVG Connector Wires Layer */}
            <svg className="tb-wires-svg">
              {/* Wire from Input Node to entry node */}
              {firstNode && (
                <path
                  className="tb-wire-path"
                  d={`M ${inputNodePos.x + 95} ${inputNodePos.y + 17} C ${inputNodePos.x + 140} ${
                    inputNodePos.y + 17
                  }, ${(firstNode.additionalInfo?.layoutX ?? 200) - 50} ${
                    (firstNode.additionalInfo?.layoutY ?? 80) + 17
                  }, ${firstNode.additionalInfo?.layoutX ?? 200} ${
                    (firstNode.additionalInfo?.layoutY ?? 80) + 17
                  }`}
                />
              )}

              {/* Node-to-node Connections */}
              {connections.map((conn, idx) => {
                const fromNode = nodes.find((n) => n.id === conn.from);
                const toNode = nodes.find((n) => n.id === conn.to);
                if (!fromNode || !toNode) return null;

                const x1 = (fromNode.additionalInfo?.layoutX ?? 200) + 170;
                const y1 = (fromNode.additionalInfo?.layoutY ?? 80) + 17;
                const x2 = toNode.additionalInfo?.layoutX ?? 400;
                const y2 = (toNode.additionalInfo?.layoutY ?? 80) + 17;
                const dx = Math.max(40, (x2 - x1) / 2);

                return (
                  <path
                    key={idx}
                    className="tb-wire-path"
                    d={`M ${x1} ${y1} C ${x1 + dx} ${y1}, ${x2 - dx} ${y2}, ${x2} ${y2}`}
                  />
                );
              })}
            </svg>

            {/* Wire Badges (Labels with delete 'X' button) */}
            {connections.map((conn, idx) => {
              const fromNode = nodes.find((n) => n.id === conn.from);
              const toNode = nodes.find((n) => n.id === conn.to);
              if (!fromNode || !toNode) return null;

              const x1 = (fromNode.additionalInfo?.layoutX ?? 200) + 170;
              const y1 = (fromNode.additionalInfo?.layoutY ?? 80) + 17;
              const x2 = toNode.additionalInfo?.layoutX ?? 400;
              const y2 = (toNode.additionalInfo?.layoutY ?? 80) + 17;
              const midX = (x1 + x2) / 2;
              const midY = (y1 + y2) / 2;

              return (
                <div
                  key={`badge_${idx}`}
                  className="tb-wire-badge"
                  style={{ left: `${midX}px`, top: `${midY}px` }}
                  title="Click 'x' to delete connection"
                >
                  <span>{conn.label || 'Success'}</span>
                  <span
                    className="tb-wire-del-btn"
                    onClick={(e) => handleDeleteConnection(e, idx)}
                  >
                    <X size={10} />
                  </span>
                </div>
              );
            })}

            {/* Green Input Node (Thingsboard Authentic) */}
            <div
              className="tb-canvas-node"
              style={{
                left: `${inputNodePos.x}px`,
                top: `${inputNodePos.y}px`,
                width: '95px',
                minWidth: '95px',
              }}
              onMouseDown={(e) => handleNodeMouseDown(e, 'INPUT')}
            >
              <div className="tb-node-pill tb-node-input" style={{ width: '100%' }}>
                <span className="tb-node-icon">
                  <LogIn size={14} />
                </span>
                <span className="tb-node-label" style={{ fontWeight: 600 }}>
                  Input
                </span>
                <span
                  className={`tb-port-nub tb-port-nub-out ${connectingFromId === 'INPUT' ? 'active' : ''}`}
                  onClick={(e) => handlePortClick(e, 'INPUT', 'out')}
                  title="Click to connect Input to rule chain entry node"
                />
              </div>
            </div>

            {/* Placed Nodes on Canvas */}
            {nodes.map((node, idx) => {
              const posX = node.additionalInfo?.layoutX ?? (240 + idx * 190);
              const posY = node.additionalInfo?.layoutY ?? (80 + (idx % 2) * 80);
              const desc = descriptors.find((d) => d.type === node.type);
              const category = desc?.category || (node.type.startsWith('filter') ? 'Filter' : 'Action');

              return (
                <div
                  key={node.id}
                  className="tb-canvas-node"
                  style={{ left: `${posX}px`, top: `${posY}px` }}
                  onMouseDown={(e) => handleNodeMouseDown(e, node.id)}
                  onDoubleClick={() => {
                    setEditingNodeId(node.id);
                    setEditNodeName(node.name);
                    setEditNodeConfig(JSON.stringify(node.configuration || {}, null, 2));
                  }}
                >
                  <div className="tb-canvas-node-hover-actions">
                    <button
                      className="tb-node-action-btn"
                      title="Configure node"
                      onClick={(e) => {
                        e.stopPropagation();
                        setEditingNodeId(node.id);
                        setEditNodeName(node.name);
                        setEditNodeConfig(JSON.stringify(node.configuration || {}, null, 2));
                      }}
                    >
                      <Settings size={13} />
                    </button>
                    <button
                      className="tb-node-action-btn delete"
                      title="Delete node"
                      onClick={(e) => handleDeleteNode(e, node.id)}
                    >
                      <Trash2 size={13} />
                    </button>
                  </div>

                  <div className={`tb-node-pill ${getCategoryClass(category)}`}>
                    <span
                      className={`tb-port-nub tb-port-nub-in ${
                        connectingFromId !== null && connectingFromId !== node.id ? 'active' : ''
                      }`}
                      onClick={(e) => handlePortClick(e, node.id, 'in')}
                      title="Input connection port"
                    />

                    <span className="tb-node-icon">{getCategoryIcon(category)}</span>
                    <span className="tb-node-label" title={node.name}>
                      {node.name.toLowerCase()}
                    </span>

                    <span
                      className={`tb-port-nub tb-port-nub-out ${
                        connectingFromId === node.id ? 'active' : ''
                      }`}
                      onClick={(e) => handlePortClick(e, node.id, 'out')}
                      title="Click to draw connection wire"
                    />
                  </div>
                </div>
              );
            })}
          </div>

          {/* Top Callout Guide (as shown in ThingsBoard screenshot) */}
          {showTopCallout && (
            <div className="tb-callout-top">
              <span>Use a search bar to add rule nodes and configure the rule chain</span>
              <button className="tb-callout-close" onClick={() => setShowTopCallout(false)}>
                ✕
              </button>
              {/* Curved arrow pointing to search bar */}
              <svg className="tb-callout-top-arrow" viewBox="0 0 28 28" fill="none">
                <path
                  d="M18 2 C10 6, 4 14, 4 22 M4 22 L2 16 M4 22 L10 20"
                  stroke="#facc15"
                  strokeWidth="2.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            </div>
          )}

          {/* Canvas Top-Right Circular Tools (Revert & Fullscreen) */}
          <div className="tb-canvas-top-tools">
            <button
              className="tb-tool-circle-btn tb-tool-circle-revert"
              title="Revert modifications"
              onClick={handleRevertChanges}
              disabled={!hasUnsavedChanges}
            >
              <RotateCcw size={18} />
            </button>
            <button
              className={`tb-tool-circle-btn tb-tool-circle-fullscreen ${showSimulator ? 'active' : ''}`}
              title="Test trace simulator"
              onClick={() => setShowSimulator(!showSimulator)}
            >
              <Play size={16} />
            </button>
            <button
              className="tb-tool-circle-btn tb-tool-circle-fullscreen"
              title="Toggle fullscreen canvas"
              onClick={() => {
                const el = document.querySelector('.tb-studio-container');
                if (el) {
                  if (!document.fullscreenElement) el.requestFullscreen?.();
                  else document.exitFullscreen?.();
                }
              }}
            >
              <Maximize2 size={18} />
            </button>
          </div>

          {/* Bottom Callout Guide (as shown in ThingsBoard screenshot) */}
          {showBottomCallout && hasUnsavedChanges && (
            <div className="tb-callout-bottom">
              <span>Once the configuration is complete, click the &ldquo;Apply changes&rdquo; button</span>
              <button className="tb-callout-close" onClick={() => setShowBottomCallout(false)}>
                ✕
              </button>
              <div className="tb-callout-bottom-pointer" />
            </div>
          )}

          {/* 3. FLOATING ACTION BUTTONS (BOTTOM RIGHT - Matches screenshot) */}
          <div className="tb-floating-actions">
            <button
              className={`tb-fab ${activeChain?.additional_info?.debug_mode ? 'debug-active' : ''}`}
              title={`Debug Mode: ${activeChain?.additional_info?.debug_mode ? 'Enabled' : 'Disabled'}`}
              onClick={handleToggleDebug}
            >
              <Bug size={18} />
            </button>

            {/* Highlighted Apply Changes button */}
            <div className={`tb-fab-apply-wrapper ${hasUnsavedChanges ? 'highlighted' : ''}`}>
              <button
                className={`tb-fab tb-fab-apply ${hasUnsavedChanges ? 'has-changes' : ''}`}
                title="Apply changes"
                onClick={handleApplyChanges}
                disabled={saving}
              >
                <Check size={20} />
              </button>
            </div>

            <button
              className="tb-fab"
              title="Revert changes"
              onClick={handleRevertChanges}
              disabled={!hasUnsavedChanges}
            >
              <X size={18} />
            </button>
          </div>
        </div>
      </div>

      {/* 4. NODE CONFIGURATION MODAL */}
      {editingNodeId !== null && (
        <div className="tb-modal-backdrop" onClick={() => setEditingNodeId(null)}>
          <div className="tb-modal-card" onClick={(e) => e.stopPropagation()}>
            <div className="tb-modal-header">
              <h3>Configure Node</h3>
              <button
                style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#64748b' }}
                onClick={() => setEditingNodeId(null)}
              >
                <X size={16} />
              </button>
            </div>

            <div className="tb-modal-body">
              <div className="tb-form-group">
                <label className="tb-form-label">Node Title</label>
                <input
                  type="text"
                  value={editNodeName}
                  onChange={(e) => setEditNodeName(e.target.value)}
                  className="tb-form-input"
                />
              </div>

              <div className="tb-form-group">
                <label className="tb-form-label">JSON Configuration</label>
                <textarea
                  rows={8}
                  value={editNodeConfig}
                  onChange={(e) => setEditNodeConfig(e.target.value)}
                  className="tb-form-textarea"
                />
              </div>
            </div>

            <div className="tb-modal-footer">
              <button
                className="tb-btn tb-btn-secondary"
                onClick={() => setEditingNodeId(null)}
              >
                Cancel
              </button>
              <button
                className="tb-btn tb-btn-primary"
                onClick={() => {
                  try {
                    const parsed = JSON.parse(editNodeConfig);
                    const updatedNodes = activeRevision!.definition.nodes.map((n) => {
                      if (n.id === editingNodeId) {
                        return { ...n, name: editNodeName.trim(), configuration: parsed };
                      }
                      return n;
                    });
                    setActiveRevision({
                      ...activeRevision!,
                      definition: {
                        ...activeRevision!.definition,
                        nodes: updatedNodes,
                      },
                    });
                    setHasUnsavedChanges(true);
                    setEditingNodeId(null);
                  } catch (err: any) {
                    alert(`Invalid JSON configuration: ${err.message}`);
                  }
                }}
              >
                Save
              </button>
            </div>
          </div>
        </div>
      )}

      {/* 5. CREATE NEW RULE CHAIN MODAL */}
      {showCreateChainModal && (
        <div className="tb-modal-backdrop" onClick={() => setShowCreateChainModal(false)}>
          <div className="tb-modal-card" onClick={(e) => e.stopPropagation()}>
            <form onSubmit={handleCreateChainSubmit}>
              <div className="tb-modal-header">
                <h3>Create New Rule Chain</h3>
                <button
                  type="button"
                  style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#64748b' }}
                  onClick={() => setShowCreateChainModal(false)}
                >
                  <X size={16} />
                </button>
              </div>

              <div className="tb-modal-body">
                <div className="tb-form-group">
                  <label className="tb-form-label">Name *</label>
                  <input
                    type="text"
                    required
                    placeholder="e.g. Edge Ingestion Pipeline"
                    value={newChainName}
                    onChange={(e) => setNewChainName(e.target.value)}
                    className="tb-form-input"
                  />
                </div>

                <div className="tb-form-group">
                  <label className="tb-form-label">Description</label>
                  <textarea
                    rows={3}
                    placeholder="Brief description of the rule chain purpose..."
                    value={newChainDesc}
                    onChange={(e) => setNewChainDesc(e.target.value)}
                    className="tb-form-textarea"
                  />
                </div>

                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginTop: '6px' }}>
                  <input
                    type="checkbox"
                    id="newChainRoot"
                    checked={newChainIsRoot}
                    onChange={(e) => setNewChainIsRoot(e.target.checked)}
                  />
                  <label htmlFor="newChainRoot" style={{ fontSize: '0.82rem', color: '#334155', cursor: 'pointer' }}>
                    Set as Root Rule Chain for this tenant
                  </label>
                </div>
              </div>

              <div className="tb-modal-footer">
                <button
                  type="button"
                  className="tb-btn tb-btn-secondary"
                  onClick={() => setShowCreateChainModal(false)}
                >
                  Cancel
                </button>
                <button type="submit" className="tb-btn tb-btn-primary">
                  Create
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
      {/* 6. TEST SIMULATOR DRAWER */}
      {showSimulator && (
        <div className="tb-simulator-drawer">
          <div className="tb-sim-header">
            <h4>Rule Chain Simulator</h4>
            <button
              style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#64748b' }}
              onClick={() => setShowSimulator(false)}
            >
              <X size={16} />
            </button>
          </div>
          <div className="tb-sim-body">
            <div className="tb-form-group">
              <label className="tb-form-label">Simulation Preset</label>
              <select
                className="tb-form-input"
                value={selectedPreset}
                onChange={(e) => setSelectedPreset(e.target.value)}
              >
                {Object.keys(SAMPLE_PAYLOADS).map((k) => (
                  <option key={k} value={k}>
                    {k}
                  </option>
                ))}
              </select>
            </div>

            <button
              className="tb-btn tb-btn-primary"
              onClick={handleRunTest}
              disabled={testingMsg}
            >
              {testingMsg ? 'Simulating...' : 'Run Test Message'}
            </button>

            {testTrace && (
              <div className="tb-sim-card">
                <label className="tb-form-label">Execution Trace Results</label>
                <pre>{JSON.stringify(testTrace, null, 2)}</pre>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
