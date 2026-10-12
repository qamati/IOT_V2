import React, { useState, useEffect } from 'react';
import { 
  Cpu, 
  Layers, 
  Upload, 
  Download, 
  Trash2, 
  RefreshCw, 
  Check, 
  Copy, 
  AlertCircle, 
  FileCode, 
  Tag, 
  Activity, 
  Plus, 
  Send,
  X
} from 'lucide-react';
import { api } from '../api';
import type { OtaPackage, DeviceOtaState, Device } from '../api';
import './OtaView.css';

interface OtaViewProps {
  devices: Device[];
  onSelectDevice?: (deviceId: string) => void;
}

export function OtaView({ devices, onSelectDevice }: OtaViewProps) {
  const [packages, setPackages] = useState<OtaPackage[]>([]);
  const [loading, setLoading] = useState(false);
  const [typeFilter, setTypeFilter] = useState<'ALL' | 'FIRMWARE' | 'SOFTWARE'>('ALL');
  const [copiedId, setCopiedId] = useState<string | null>(null);

  // Upload modal state
  const [showUploadModal, setShowUploadModal] = useState(false);
  const [pkgType, setPkgType] = useState<'FIRMWARE' | 'SOFTWARE'>('FIRMWARE');
  const [pkgTitle, setPkgTitle] = useState('');
  const [pkgVersion, setPkgVersion] = useState('');
  const [pkgTag, setPkgTag] = useState('stable');
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState('');

  // Assign modal state
  const [showAssignModal, setShowAssignModal] = useState(false);
  const [targetPackage, setTargetPackage] = useState<OtaPackage | null>(null);
  const [selectedDeviceId, setSelectedDeviceId] = useState('');
  const [assigning, setAssigning] = useState(false);
  const [assignSuccess, setAssignSuccess] = useState('');

  // Rollout fleet state
  const [selectedPackageForFleet, setSelectedPackageForFleet] = useState<OtaPackage | null>(null);
  const [fleetStates, setFleetStates] = useState<DeviceOtaState[]>([]);
  const [fleetLoading, setFleetLoading] = useState(false);

  const loadPackages = async () => {
    setLoading(true);
    try {
      const typeParam = typeFilter === 'ALL' ? undefined : typeFilter;
      const res = await api.listOtaPackages(typeParam);
      setPackages(res.data || []);
      if (res.data && res.data.length > 0 && !selectedPackageForFleet) {
        setSelectedPackageForFleet(res.data[0]);
      }
    } catch (err) {
      console.error('Failed to load OTA packages:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadPackages();
  }, [typeFilter]);

  const loadFleetStates = async (pkg: OtaPackage) => {
    setFleetLoading(true);
    setSelectedPackageForFleet(pkg);
    try {
      const states = await api.listFleetOtaStates(pkg.id);
      setFleetStates(states || []);
    } catch (err) {
      console.error('Failed to fetch fleet states:', err);
    } finally {
      setFleetLoading(false);
    }
  };

  useEffect(() => {
    if (selectedPackageForFleet) {
      loadFleetStates(selectedPackageForFleet);
    }
  }, [selectedPackageForFleet?.id]);

  const handleCopyChecksum = (checksum: string, id: string) => {
    navigator.clipboard.writeText(checksum);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  const handleUploadSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!pkgTitle || !pkgVersion) {
      setUploadError('Title and Version are required');
      return;
    }
    setUploading(true);
    setUploadError('');
    try {
      // 1. Create metadata record
      const created = await api.createOtaPackage({
        type: pkgType,
        title: pkgTitle,
        version: pkgVersion,
        tag: pkgTag,
      });

      // 2. Upload binary if provided
      if (selectedFile) {
        await api.uploadOtaBinary(created.id, selectedFile);
      }

      setShowUploadModal(false);
      setPkgTitle('');
      setPkgVersion('');
      setSelectedFile(null);
      await loadPackages();
    } catch (err: any) {
      setUploadError(err.message || 'Upload failed');
    } finally {
      setUploading(false);
    }
  };

  const handleAssignSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!targetPackage || !selectedDeviceId) return;
    setAssigning(true);
    setAssignSuccess('');
    try {
      await api.assignOtaPackage(selectedDeviceId, targetPackage.id, targetPackage.type);
      setAssignSuccess(`Package assigned successfully to device!`);
      setTimeout(() => {
        setShowAssignModal(false);
        setAssignSuccess('');
        if (selectedPackageForFleet?.id === targetPackage.id) {
          loadFleetStates(targetPackage);
        }
      }, 1200);
    } catch (err: any) {
      alert(`Assignment failed: ${err.message}`);
    } finally {
      setAssigning(false);
    }
  };

  const handleDeletePackage = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    if (!confirm('Are you sure you want to delete this OTA package?')) return;
    try {
      await api.deleteOtaPackage(id);
      await loadPackages();
      if (selectedPackageForFleet?.id === id) {
        setSelectedPackageForFleet(null);
        setFleetStates([]);
      }
    } catch (err: any) {
      alert(`Delete failed: ${err.message}`);
    }
  };

  const formatBytes = (bytes?: number) => {
    if (!bytes || bytes <= 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const firmwareCount = packages.filter(p => p.type === 'FIRMWARE').length;
  const softwareCount = packages.filter(p => p.type === 'SOFTWARE').length;

  return (
    <div className="ota-container">
      {/* Top Header */}
      <div className="ota-header">
        <div className="ota-header-title">
          <h2>
            <Cpu size={26} color="#60a5fa" />
            Firmware & Software OTA Management
          </h2>
          <p>
            Secure binary distribution, SHA-256 integrity verification, chunked OTA endpoints, and fleet rollout tracking.
          </p>
        </div>
        <div className="ota-header-actions">
          <div className="ota-filter-pills">
            <button 
              className={`ota-pill-btn ${typeFilter === 'ALL' ? 'active' : ''}`}
              onClick={() => setTypeFilter('ALL')}
            >
              All Packages ({packages.length})
            </button>
            <button 
              className={`ota-pill-btn ${typeFilter === 'FIRMWARE' ? 'active' : ''}`}
              onClick={() => setTypeFilter('FIRMWARE')}
            >
              Firmware ({firmwareCount})
            </button>
            <button 
              className={`ota-pill-btn ${typeFilter === 'SOFTWARE' ? 'active' : ''}`}
              onClick={() => setTypeFilter('SOFTWARE')}
            >
              Software ({softwareCount})
            </button>
          </div>

          <button 
            className="ota-btn ota-btn-secondary" 
            onClick={loadPackages} 
            disabled={loading}
          >
            <RefreshCw size={15} className={loading ? 'spin' : ''} />
            Refresh
          </button>

          <button 
            className="ota-btn ota-btn-primary"
            onClick={() => setShowUploadModal(true)}
          >
            <Plus size={16} />
            Upload Package
          </button>
        </div>
      </div>

      {/* Stats Summary */}
      <div className="ota-stats-grid">
        <div className="ota-stat-card">
          <div className="ota-stat-icon firmware">
            <Cpu size={22} />
          </div>
          <div className="ota-stat-info">
            <div className="stat-value">{firmwareCount}</div>
            <div className="stat-label">Firmware Images</div>
          </div>
        </div>

        <div className="ota-stat-card">
          <div className="ota-stat-icon software">
            <Layers size={22} />
          </div>
          <div className="ota-stat-info">
            <div className="stat-value">{softwareCount}</div>
            <div className="stat-label">Software Packages</div>
          </div>
        </div>

        <div className="ota-stat-card">
          <div className="ota-stat-icon rollout">
            <Activity size={22} />
          </div>
          <div className="ota-stat-info">
            <div className="stat-value">{fleetStates.length}</div>
            <div className="stat-label">Active Device Rollouts</div>
          </div>
        </div>

        <div className="ota-stat-card">
          <div className="ota-stat-icon active">
            <Tag size={22} />
          </div>
          <div className="ota-stat-info">
            <div className="stat-value">
              {fleetStates.filter(s => s.state === 'UPDATED').length}
            </div>
            <div className="stat-label">Fully Updated Devices</div>
          </div>
        </div>
      </div>

      {/* Packages Grid */}
      <div className="ota-packages-grid">
        {packages.map((pkg) => {
          const isSelected = selectedPackageForFleet?.id === pkg.id;
          return (
            <div 
              key={pkg.id} 
              className="ota-card"
              style={{
                borderColor: isSelected ? 'rgba(59, 130, 246, 0.6)' : undefined,
                background: isSelected ? 'rgba(15, 23, 42, 0.85)' : undefined
              }}
              onClick={() => setSelectedPackageForFleet(pkg)}
            >
              <div className="ota-card-header">
                <div className="ota-card-title-group">
                  <h3>{pkg.title}</h3>
                  <div className="ota-version-tag">
                    <FileCode size={13} />
                    {pkg.version}
                    {pkg.tag && <span style={{ opacity: 0.7 }}>• {pkg.tag}</span>}
                  </div>
                </div>
                <span className={`ota-type-badge ${pkg.type.toLowerCase()}`}>
                  {pkg.type}
                </span>
              </div>

              <div className="ota-card-meta">
                <div className="ota-meta-row">
                  <span className="ota-meta-label">File Name:</span>
                  <span className="ota-meta-value">{pkg.file_name || 'N/A'}</span>
                </div>
                <div className="ota-meta-row">
                  <span className="ota-meta-label">Size:</span>
                  <span className="ota-meta-value">{formatBytes(pkg.data_size)}</span>
                </div>
                <div className="ota-meta-row">
                  <span className="ota-meta-label">Checksum:</span>
                  <span className="ota-meta-value">
                    {pkg.checksum ? (
                      <button 
                        className="ota-checksum-copy"
                        onClick={(e) => {
                          e.stopPropagation();
                          handleCopyChecksum(pkg.checksum!, pkg.id);
                        }}
                        title={pkg.checksum}
                      >
                        {copiedId === pkg.id ? <Check size={12} color="#10b981" /> : <Copy size={12} />}
                        {pkg.checksum.slice(0, 10)}...
                      </button>
                    ) : (
                      'Pending Upload'
                    )}
                  </span>
                </div>
              </div>

              <div className="ota-card-actions">
                <button 
                  className="ota-btn ota-btn-secondary"
                  style={{ fontSize: '0.78rem', padding: '5px 10px' }}
                  onClick={(e) => {
                    e.stopPropagation();
                    setTargetPackage(pkg);
                    setShowAssignModal(true);
                  }}
                >
                  <Send size={13} />
                  Assign
                </button>

                <div style={{ display: 'flex', gap: '0.4rem' }}>
                  {pkg.data_size && pkg.data_size > 0 && (
                    <a
                      href={`/api/v1/otaPackages/${pkg.id}/download`}
                      download
                      className="ota-btn ota-btn-secondary"
                      style={{ fontSize: '0.78rem', padding: '5px 10px', textDecoration: 'none' }}
                      onClick={(e) => e.stopPropagation()}
                    >
                      <Download size={13} />
                      Binary
                    </a>
                  )}

                  <button 
                    className="ota-btn ota-btn-danger"
                    style={{ fontSize: '0.78rem', padding: '5px 10px' }}
                    onClick={(e) => handleDeletePackage(pkg.id, e)}
                    title="Delete package"
                  >
                    <Trash2 size={13} />
                  </button>
                </div>
              </div>
            </div>
          );
        })}

        {packages.length === 0 && !loading && (
          <div style={{ gridColumn: '1 / -1', textAlign: 'center', padding: '3rem', background: 'rgba(15, 23, 42, 0.4)', borderRadius: 12, border: '1px dashed rgba(255, 255, 255, 0.1)' }}>
            <FileCode size={40} color="#64748b" style={{ margin: '0 auto 0.75rem auto' }} />
            <h4 style={{ margin: 0, color: '#f1f5f9' }}>No OTA packages found</h4>
            <p style={{ color: '#94a3b8', fontSize: '0.85rem', marginTop: 4 }}>
              Upload your first firmware or software binary package to initiate over-the-air fleet rollouts.
            </p>
            <button 
              className="ota-btn ota-btn-primary"
              style={{ marginTop: '1rem' }}
              onClick={() => setShowUploadModal(true)}
            >
              <Plus size={16} />
              Upload Package
            </button>
          </div>
        )}
      </div>

      {/* Fleet Rollout Section */}
      {selectedPackageForFleet && (
        <div className="ota-rollout-section">
          <div className="ota-rollout-header">
            <h3>
              <Activity size={18} color="#60a5fa" />
              Live Fleet Rollout: {selectedPackageForFleet.title} ({selectedPackageForFleet.version})
            </h3>
            <button 
              className="ota-btn ota-btn-secondary"
              style={{ fontSize: '0.78rem', padding: '4px 10px' }}
              onClick={() => loadFleetStates(selectedPackageForFleet)}
              disabled={fleetLoading}
            >
              <RefreshCw size={12} className={fleetLoading ? 'spin' : ''} />
              Refresh Status
            </button>
          </div>

          <table className="ota-rollout-table">
            <thead>
              <tr>
                <th>Device Name</th>
                <th>Kind</th>
                <th>Rollout State</th>
                <th>Download Progress</th>
                <th>Last Updated</th>
                <th>Action</th>
              </tr>
            </thead>
            <tbody>
              {fleetStates.map((st) => {
                const dev = devices.find(d => d.id === st.device_id);
                const progressPct = Math.round((st.progress || 0) * 100);
                return (
                  <tr key={st.device_id + st.kind}>
                    <td>
                      <strong>{dev ? dev.name : st.device_id.slice(0, 12)}</strong>
                      {dev?.label && <div style={{ fontSize: '0.75rem', color: '#64748b' }}>{dev.label}</div>}
                    </td>
                    <td>
                      <span className={`ota-type-badge ${st.kind.toLowerCase()}`} style={{ fontSize: '0.7rem' }}>
                        {st.kind}
                      </span>
                    </td>
                    <td>
                      <span className={`ota-state-pill ${st.state.toLowerCase()}`}>
                        {st.state}
                      </span>
                      {st.error && (
                        <div style={{ color: '#f87171', fontSize: '0.72rem', marginTop: 2 }}>
                          {st.error}
                        </div>
                      )}
                    </td>
                    <td>
                      <div className="ota-progress-container">
                        <div 
                          className="ota-progress-bar" 
                          style={{ width: `${progressPct}%` }}
                        />
                      </div>
                      <span style={{ fontSize: '0.78rem', fontWeight: 600 }}>{progressPct}%</span>
                    </td>
                    <td style={{ color: '#94a3b8', fontSize: '0.8rem' }}>
                      {new Date(st.updated_ts).toLocaleTimeString()}
                    </td>
                    <td>
                      {onSelectDevice && (
                        <button 
                          className="ota-btn ota-btn-secondary"
                          style={{ fontSize: '0.72rem', padding: '3px 8px' }}
                          onClick={() => onSelectDevice(st.device_id)}
                        >
                          View Device
                        </button>
                      )}
                    </td>
                  </tr>
                );
              })}

              {fleetStates.length === 0 && (
                <tr>
                  <td colSpan={6} style={{ textAlign: 'center', padding: '2rem', color: '#64748b' }}>
                    No devices have been assigned this package yet. Click "Assign" on the package card to begin rollout.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* Upload Package Modal */}
      {showUploadModal && (
        <div className="ota-modal-overlay" onClick={() => setShowUploadModal(false)}>
          <div className="ota-modal" onClick={e => e.stopPropagation()}>
            <div className="ota-modal-header">
              <h3>Upload OTA Package</h3>
              <button 
                onClick={() => setShowUploadModal(false)}
                style={{ background: 'transparent', border: 'none', color: '#94a3b8', cursor: 'pointer' }}
              >
                <X size={18} />
              </button>
            </div>

            <form onSubmit={handleUploadSubmit}>
              <div className="ota-modal-body">
                {uploadError && (
                  <div style={{ background: 'rgba(239, 68, 68, 0.15)', border: '1px solid #ef4444', color: '#fca5a5', padding: '8px 12px', borderRadius: 6, fontSize: '0.82rem', display: 'flex', alignItems: 'center', gap: 6 }}>
                    <AlertCircle size={15} />
                    {uploadError}
                  </div>
                )}

                <div className="ota-form-group">
                  <label>Package Type</label>
                  <select 
                    className="ota-select"
                    value={pkgType}
                    onChange={e => setPkgType(e.target.value as any)}
                  >
                    <option value="FIRMWARE">Firmware (Microcontroller / Gateway Image)</option>
                    <option value="SOFTWARE">Software (Application / Edge Container)</option>
                  </select>
                </div>

                <div className="ota-form-group">
                  <label>Package Title</label>
                  <input 
                    type="text" 
                    className="ota-input" 
                    placeholder="e.g. ESP32 Weather Sensor Firmware"
                    value={pkgTitle}
                    onChange={e => setPkgTitle(e.target.value)}
                    required
                  />
                </div>

                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem' }}>
                  <div className="ota-form-group">
                    <label>Version</label>
                    <input 
                      type="text" 
                      className="ota-input" 
                      placeholder="e.g. v2.3.0"
                      value={pkgVersion}
                      onChange={e => setPkgVersion(e.target.value)}
                      required
                    />
                  </div>

                  <div className="ota-form-group">
                    <label>Release Tag</label>
                    <input 
                      type="text" 
                      className="ota-input" 
                      placeholder="e.g. stable, beta"
                      value={pkgTag}
                      onChange={e => setPkgTag(e.target.value)}
                    />
                  </div>
                </div>

                <div className="ota-form-group">
                  <label>Binary File (.bin, .hex, .tar.gz)</label>
                  <label className={`ota-dropzone ${selectedFile ? 'has-file' : ''}`}>
                    <input 
                      type="file" 
                      style={{ display: 'none' }}
                      onChange={e => {
                        if (e.target.files && e.target.files[0]) {
                          setSelectedFile(e.target.files[0]);
                        }
                      }}
                    />
                    <Upload size={24} color={selectedFile ? '#10b981' : '#60a5fa'} />
                    {selectedFile ? (
                      <div>
                        <strong style={{ color: '#10b981' }}>{selectedFile.name}</strong>
                        <div style={{ fontSize: '0.75rem', color: '#94a3b8', marginTop: 2 }}>
                          {formatBytes(selectedFile.size)}
                        </div>
                      </div>
                    ) : (
                      <div>
                        <strong>Choose binary file</strong> or drag & drop here
                        <div style={{ fontSize: '0.75rem', color: '#64748b', marginTop: 2 }}>
                          SHA-256 checksum calculated automatically upon upload
                        </div>
                      </div>
                    )}
                  </label>
                </div>
              </div>

              <div className="ota-modal-footer">
                <button 
                  type="button" 
                  className="ota-btn ota-btn-secondary"
                  onClick={() => setShowUploadModal(false)}
                  disabled={uploading}
                >
                  Cancel
                </button>
                <button 
                  type="submit" 
                  className="ota-btn ota-btn-primary"
                  disabled={uploading}
                >
                  {uploading ? (
                    <>
                      <RefreshCw size={14} className="spin" />
                      Uploading & Hashing...
                    </>
                  ) : (
                    'Upload & Publish'
                  )}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Assign Package Modal */}
      {showAssignModal && targetPackage && (
        <div className="ota-modal-overlay" onClick={() => setShowAssignModal(false)}>
          <div className="ota-modal" onClick={e => e.stopPropagation()}>
            <div className="ota-modal-header">
              <h3>Assign OTA Package to Device</h3>
              <button 
                onClick={() => setShowAssignModal(false)}
                style={{ background: 'transparent', border: 'none', color: '#94a3b8', cursor: 'pointer' }}
              >
                <X size={18} />
              </button>
            </div>

            <form onSubmit={handleAssignSubmit}>
              <div className="ota-modal-body">
                {assignSuccess && (
                  <div style={{ background: 'rgba(16, 185, 129, 0.15)', border: '1px solid #10b981', color: '#6ee7b7', padding: '8px 12px', borderRadius: 6, fontSize: '0.82rem', display: 'flex', alignItems: 'center', gap: 6 }}>
                    <Check size={15} />
                    {assignSuccess}
                  </div>
                )}

                <div style={{ background: 'rgba(30, 41, 59, 0.5)', padding: '0.75rem', borderRadius: 8, fontSize: '0.82rem' }}>
                  <div style={{ color: '#94a3b8' }}>Target Package:</div>
                  <strong style={{ color: '#ffffff', fontSize: '0.95rem' }}>{targetPackage.title}</strong>
                  <div style={{ color: '#60a5fa', fontWeight: 600, marginTop: 2 }}>{targetPackage.version} ({targetPackage.type})</div>
                </div>

                <div className="ota-form-group">
                  <label>Select Target Device</label>
                  <select 
                    className="ota-select"
                    value={selectedDeviceId}
                    onChange={e => setSelectedDeviceId(e.target.value)}
                    required
                  >
                    <option value="">-- Choose Device --</option>
                    {devices.map(d => (
                      <option key={d.id} value={d.id}>
                        {d.name} {d.label ? `(${d.label})` : ''}
                      </option>
                    ))}
                  </select>
                </div>

                <div style={{ fontSize: '0.78rem', color: '#94a3b8', lineHeight: 1.4 }}>
                  Assigning will write shared attributes (<code>fw_title</code>, <code>fw_version</code>, <code>fw_checksum</code>, <code>fw_size</code>) to the target device, queue the firmware update, and notify the device over MQTT.
                </div>
              </div>

              <div className="ota-modal-footer">
                <button 
                  type="button" 
                  className="ota-btn ota-btn-secondary"
                  onClick={() => setShowAssignModal(false)}
                  disabled={assigning}
                >
                  Cancel
                </button>
                <button 
                  type="submit" 
                  className="ota-btn ota-btn-primary"
                  disabled={assigning || !selectedDeviceId}
                >
                  {assigning ? (
                    <>
                      <RefreshCw size={14} className="spin" />
                      Assigning...
                    </>
                  ) : (
                    'Confirm & Push Downlink'
                  )}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
