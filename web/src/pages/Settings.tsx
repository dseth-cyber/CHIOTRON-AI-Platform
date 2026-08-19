import { useRef, useState } from 'react';
import { Modal } from '../Modal';
import { Tag } from '../components/EmptyState';
import { SearchableSelect } from '../components/SearchableSelect';
import {
  useComputeHealth,
  useCredential,
  useIdentity,
  useModels,
  usePlatform,
  usePlatformSettings,
  usePromptTemplates,
  useRefreshSettings,
  useScopes,
} from '../hooks';
import { useTheme } from '@/contexts/ThemeContext';
import { useBrandIcon } from '@/contexts/BrandContext';
import { useTranslation } from '../LanguageContext';
import { LANGUAGES, LANGUAGE_NAMES, type Language } from '../i18n';
import { classificationTone, statusTone, toneFor } from '../theme';
import { SCOPE_ADMIN_KEYS, SCOPE_MODELS_READ } from '../Connection';
import { updatePlatformSetting } from '../api';

/** Every scope the platform defines, so a key can be read against the whole list. */
const ALL_SCOPES = [
  'models:read',
  'assistants:read',
  'chat:completions',
  'knowledge:read',
  'knowledge:write',
  'tools:read',
  'agent:run',
  'admin:keys',
  'admin:assistants',
] as const;

export function Settings({ onConnect }: { onConnect: () => void }) {
  const { t, language, setLanguage, formatNumber } = useTranslation();
  const { theme, setTheme } = useTheme();
  const { customIcon, uploadIcon, resetIcon } = useBrandIcon();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [iconBusy, setIconBusy] = useState(false);
  const [iconMsg, setIconMsg] = useState<{ type: 'ok' | 'err'; text: string } | null>(null);

  const [credential] = useCredential();
  const identity = useIdentity();
  const platform = usePlatform();
  const { has } = useScopes();
  const canAdmin = has(SCOPE_ADMIN_KEYS);
  const models = useModels(has(SCOPE_MODELS_READ));
  const compute = useComputeHealth(credential !== '');
  const settingsQuery = usePlatformSettings(canAdmin);
  const promptsQuery = usePromptTemplates(credential !== '');
  const refreshSettings = useRefreshSettings();

  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [editValue, setEditValue] = useState<string>('');
  const [editDesc, setEditDesc] = useState<string>('');
  const [saveBusy, setSaveBusy] = useState<boolean>(false);

  // Add Setting Modal
  const [showAddModal, setShowAddModal] = useState<boolean>(false);
  const [newKey, setNewKey] = useState<string>('');
  const [newValue, setNewValue] = useState<string>('true');
  const [newDesc, setNewDesc] = useState<string>('');

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (!file.type.startsWith('image/')) {
      setIconMsg({ type: 'err', text: 'กรุณาเลือกไฟล์รูปภาพที่ถูกต้อง (PNG, JPG, SVG, WebP, ICO)' });
      return;
    }
    setIconBusy(true);
    setIconMsg(null);
    try {
      await uploadIcon(file);
      setIconMsg({ type: 'ok', text: 'อัปเดตไอคอนระบบและ Favicon เรียบร้อยแล้ว!' });
    } catch {
      setIconMsg({ type: 'err', text: 'เกิดข้อผิดพลาดในการอัปโหลดรูปภาพ' });
    } finally {
      setIconBusy(false);
    }
  };

  const handleStartEdit = (key: string, currentValue: string, currentDesc: string) => {
    setEditingKey(key);
    setEditValue(currentValue);
    setEditDesc(currentDesc);
  };

  const handleSaveSetting = async (key: string, description: string, overrideValue?: any) => {
    setSaveBusy(true);
    try {
      let parsedValue: any = overrideValue !== undefined ? overrideValue : editValue;
      if (typeof parsedValue === 'string') {
        try {
          parsedValue = JSON.parse(parsedValue);
        } catch {
          // If not valid JSON, pass as string
        }
      }
      await updatePlatformSetting(key, parsedValue, description);
      refreshSettings();
      setEditingKey(null);
    } catch (err) {
      console.error('Failed to update setting', err);
    } finally {
      setSaveBusy(false);
    }
  };

  const handleAddNewSetting = async () => {
    if (!newKey.trim()) return;
    setSaveBusy(true);
    try {
      let parsedValue: any = newValue;
      try {
        parsedValue = JSON.parse(newValue);
      } catch {}
      await updatePlatformSetting(newKey.trim(), parsedValue, newDesc.trim());
      refreshSettings();
      setShowAddModal(false);
      setNewKey('');
      setNewValue('true');
      setNewDesc('');
    } catch (err) {
      console.error('Failed to add setting', err);
    } finally {
      setSaveBusy(false);
    }
  };

  const isBooleanSetting = (val: string) => val === 'true' || val === 'false';

  return (
    <>
      <section className="page-intro">
        <p>{t('page.settings.title')}</p>
        <span>ศูนย์กลางการปรับแต่งธีม ไอคอนองค์กร การจัดการผู้ดูแล และนโยบายแพลตฟอร์ม</span>
      </section>

      <section className="settings-grid">
        <section className="panel" style={{ gridColumn: '1 / -1' }}>
          <span className="panel-label">Theme / ธีมระบบ</span>
          <p className="history-hint">เลือกรูปแบบการแสดงผลของหน้าจอ (Modern Glassmorphism, Dark, Light) เพื่อประสบการณ์การใช้งานที่ดีที่สุด</p>
          <div className="theme-card-grid">
            <button
              type="button"
              className={`theme-select-card ${theme === 'glassmorphism' ? 'active' : ''}`}
              onClick={() => setTheme('glassmorphism')}
            >
              <span className="theme-select-icon">🔮</span>
              <div>
                <b>Glassmorphism Modern</b>
                <small>มิติกระจกใส แสงออร่าระดับพรีเมียม สไตล์ Gemini 3.6</small>
              </div>
            </button>
            <button
              type="button"
              className={`theme-select-card ${theme === 'dark' ? 'active' : ''}`}
              onClick={() => setTheme('dark')}
            >
              <span className="theme-select-icon">🌙</span>
              <div>
                <b>Dark Classic</b>
                <small>โทนสีเข้ม คมชัด ถนอมสายตาสำหรับการทำงานระดับโปร</small>
              </div>
            </button>
            <button
              type="button"
              className={`theme-select-card ${theme === 'light' ? 'active' : ''}`}
              onClick={() => setTheme('light')}
            >
              <span className="theme-select-icon">☀️</span>
              <div>
                <b>Clean Light</b>
                <small>โทนสว่าง สะอาด สดใส สบายตา อ่านง่ายทุกมุมมอง</small>
              </div>
            </button>
          </div>
        </section>

        {/* Brand Custom Logo / Icon Customizer */}
        <section className="panel" style={{ gridColumn: '1 / -1' }}>
          <span className="panel-label">Brand & Custom Icon / กำหนดไอคอนและโลโก้ระบบ</span>
          <p className="history-hint">
            คุณสามารถอัปโหลดรูปภาพหรือโลโก้องค์กร เพื่อใช้เป็นไอคอนประจำแถบเมนูด้านข้าง (Sidebar Logo) และ Favicon ของระบบได้ทันที
          </p>

          <div className="brand-customizer-wrap">
            <div className="brand-preview-box">
              <div className="brand-preview-header">
                <span>ตัวอย่างการแสดงผลในแถบเมนู (Sidebar Preview)</span>
              </div>
              <div className="brand-preview-content">
                <div className="brand-sample-sidebar">
                  <div className="brand-sample-logo">
                    <div className="brand-icon-wrapper" style={{ width: '28px', height: '28px', borderRadius: '8px', overflow: 'hidden', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'rgba(255,255,255,0.05)', border: '1px solid rgba(255,255,255,0.1)' }}>
                      {customIcon ? (
                        <img src={customIcon} alt="Custom Brand Logo" style={{ width: '100%', height: '100%', objectFit: 'contain' }} />
                      ) : (
                        <span style={{ display: 'inline-grid', placeItems: 'center', width: '16px', height: '16px', background: '#1bd9b2', color: '#05221f', borderRadius: '3px', fontSize: '11px', fontWeight: 900 }}>C</span>
                      )}
                    </div>
                    <span>CHIOTRON AI Platform</span>
                  </div>
                </div>
              </div>
            </div>

            <div className="brand-upload-actions">
              <input
                type="file"
                ref={fileInputRef}
                style={{ display: 'none' }}
                accept="image/png,image/jpeg,image/svg+xml,image/webp,image/x-icon"
                onChange={handleFileUpload}
              />
              <button
                type="button"
                className="primary"
                onClick={() => fileInputRef.current?.click()}
                disabled={iconBusy}
                style={{ display: 'inline-flex', alignItems: 'center', gap: '8px' }}
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                  <polyline points="17 8 12 3 7 8" />
                  <line x1="12" y1="3" x2="12" y2="15" />
                </svg>
                {iconBusy ? 'กำลังอัปโหลด...' : 'อัปโหลดรูปไอคอนใหม่'}
              </button>

              {customIcon && (
                <button
                  type="button"
                  className="secondary"
                  onClick={() => {
                    resetIcon();
                    setIconMsg({ type: 'ok', text: 'คืนค่าไอคอนเป็นค่าเริ่มต้นแล้ว' });
                  }}
                  style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}
                >
                  ↺ คืนค่าเริ่มต้น (Reset to Default)
                </button>
              )}

              <p className="upload-hint-text">
                💡 รองรับไฟล์ <code>PNG</code>, <code>JPG</code>, <code>SVG</code>, <code>WebP</code>, <code>ICO</code> (แนะนำขนาดสี่เหลี่ยมจัตุรัส 512×512px)
              </p>

              {iconMsg && (
                <div style={{ marginTop: '8px', fontSize: '0.85rem', color: iconMsg.type === 'ok' ? '#2de1ba' : '#f87171' }}>
                  {iconMsg.type === 'ok' ? '✓ ' : '✕ '} {iconMsg.text}
                </div>
              )}
            </div>
          </div>
        </section>

        <section className="panel">
          <span className="panel-label">ภาษา / Language</span>
          <p className="history-hint">เลือกภาษาที่ต้องการให้ระบบแสดงผล</p>
          <SearchableSelect
            label={t('lang.label')}
            value={language}
            options={LANGUAGES.map((option) => ({
              value: option,
              label: LANGUAGE_NAMES[option],
              detail: option,
            }))}
            onChange={(next) => setLanguage(next as Language)}
          />
        </section>

        <section className="panel">
          <span className="panel-label">คีย์การเชื่อมต่อ / API Credential</span>
          {credential === '' ? (
            <p className="history-hint">{t('conn.notConnected')}</p>
          ) : (
            <dl className="detail-pairs">
              <dt>ชื่อคีย์ (Name)</dt>
              <dd>{identity.data?.name ?? '—'}</dd>
              <dt>ระดับความลับสูงสุด (Max Clearance)</dt>
              <dd>
                <Tag tone={toneFor(classificationTone, identity.data?.maxClassification)}>
                  {identity.data?.maxClassification ?? '—'}
                </Tag>
              </dd>
              <dt>โควตาการเรียกใช้งาน (Rate Limit)</dt>
              <dd>
                {identity.data
                  ? `${formatNumber(identity.data.rateLimitPerMinute)} คำขอ/นาที`
                  : '—'}
              </dd>
            </dl>
          )}
          <button className="primary" onClick={onConnect}>
            {credential === '' ? t('conn.addKey') : t('action.changeKey')}
          </button>
        </section>

        <section className="panel">
          <span className="panel-label">ข้อมูลแพลตฟอร์ม / Platform Info</span>
          <dl className="detail-pairs">
            <dt>ชื่อแพลตฟอร์ม</dt>
            <dd>{platform.data?.name ?? '—'}</dd>
            <dt>Control Plane</dt>
            <dd>{platform.data?.plane ?? '—'}</dd>
            <dt>เวอร์ชัน (Version)</dt>
            <dd>{platform.data?.version ?? '—'}</dd>
            <dt>สภาพแวดล้อม (Environment)</dt>
            <dd>{platform.data?.environment ?? '—'}</dd>
            <dt>ผู้ให้บริการประมวลผลหลัก</dt>
            <dd>{platform.data?.computeProvider ?? '—'}</dd>
          </dl>
        </section>

        <section className="panel">
          <span className="panel-label">สถานะการทำงาน / Runtime Health</span>
          <dl className="detail-pairs">
            <dt>สถานะระบบ Compute</dt>
            <dd>
              <Tag tone={toneFor(statusTone, compute.data?.status)}>{compute.data?.status ?? '—'}</Tag>
            </dd>
            <dt>โมเดลที่พร้อมใช้งาน</dt>
            <dd>
              {models.isSuccess
                ? `${formatNumber(models.data.models.filter((entry) => entry.available).length)}/${formatNumber(models.data.models.length)}`
                : '—'}
            </dd>
          </dl>
        </section>
      </section>

      {canAdmin && (
        <section className="panel">
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px', flexWrap: 'wrap', gap: '8px' }}>
            <div>
              <span className="panel-label">{t('settings.platformSettings')}</span>
              <p className="history-hint" style={{ margin: '4px 0 0' }}>
                กำหนดค่าพารามิเตอร์การทำงานและนโยบายความเป็นส่วนตัวของระบบ AI (เช่น การบันทึกประวัติแชท, อุณหภูมิโมเดล, ขนาดเอกสาร)
              </p>
            </div>
            <button
              type="button"
              className="primary"
              onClick={() => setShowAddModal(true)}
              style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}
            >
              ＋ เพิ่มการตั้งค่า
            </button>
          </div>

          {settingsQuery.isLoading ? (
            <p className="history-hint">{t('table.loading')}</p>
          ) : (settingsQuery.data ?? []).length === 0 ? (
            <p className="history-hint">{t('settings.empty')}</p>
          ) : (
            <div className="table-wrap">
              <table className="data-table">
                <thead>
                  <tr>
                    <th style={{ width: '220px' }}>{t('settings.column.key')}</th>
                    <th style={{ width: '220px' }}>{t('settings.column.value')}</th>
                    <th>{t('settings.column.description')}</th>
                    <th style={{ width: '130px', textAlign: 'right' }}>การกระทำ</th>
                  </tr>
                </thead>
                <tbody>
                  {(settingsQuery.data ?? []).map((setting) => {
                    const isBool = isBooleanSetting(setting.value);
                    const boolVal = setting.value === 'true';

                    return (
                      <tr key={setting.key}>
                        <td><code>{setting.key}</code></td>
                        <td>
                          {editingKey === setting.key ? (
                            <input
                              type="text"
                              className="input-text"
                              value={editValue}
                              onChange={(e) => setEditValue(e.target.value)}
                              disabled={saveBusy}
                            />
                          ) : isBool ? (
                            <button
                              type="button"
                              role="switch"
                              aria-checked={boolVal}
                              disabled={saveBusy}
                              className={`ui-toggle-switch ${boolVal ? 'active' : ''}`}
                              onClick={() => handleSaveSetting(setting.key, setting.description, !boolVal)}
                              title={`คลิกเพื่อสลับเป็น ${boolVal ? 'ปิดใช้งาน (false)' : 'เปิดใช้งาน (true)'}`}
                            >
                              <span className="ui-toggle-track">
                                <span className="ui-toggle-thumb" />
                              </span>
                              <span className="ui-toggle-label">
                                {boolVal ? 'เปิด (ON)' : 'ปิด (OFF)'}
                              </span>
                            </button>
                          ) : (
                            <code>{setting.value}</code>
                          )}
                        </td>
                        <td>
                          {editingKey === setting.key ? (
                            <input
                              type="text"
                              className="input-text"
                              value={editDesc}
                              onChange={(e) => setEditDesc(e.target.value)}
                              disabled={saveBusy}
                            />
                          ) : (
                            <small>{setting.description}</small>
                          )}
                        </td>
                        <td style={{ textAlign: 'right' }}>
                          {editingKey === setting.key ? (
                            <div style={{ display: 'flex', gap: '6px', justifyContent: 'flex-end' }}>
                              <button
                                className="primary"
                                onClick={() => handleSaveSetting(setting.key, editDesc)}
                                disabled={saveBusy}
                              >
                                {t('settings.save')}
                              </button>
                              <button
                                className="secondary"
                                onClick={() => setEditingKey(null)}
                                disabled={saveBusy}
                              >
                                {t('action.cancel')}
                              </button>
                            </div>
                          ) : (
                            <button
                              className="secondary"
                              onClick={() => handleStartEdit(setting.key, setting.value, setting.description)}
                            >
                              {t('settings.edit')}
                            </button>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </section>
      )}

      {/* Add New Setting Modal Dialog */}
      {showAddModal && (
        <Modal title="เพิ่มการตั้งค่าแพลตฟอร์ม (Add Platform Setting)" onClose={() => setShowAddModal(false)}>
          <div style={{ display: 'grid', gap: '16px' }}>
            <label className="field">
              <span>ชื่อคีย์ (Setting Key)</span>
              <input
                type="text"
                className="input-text"
                placeholder="เช่น persist_prompts, history_turn_limit"
                value={newKey}
                onChange={(e) => setNewKey(e.target.value)}
              />
            </label>

            <label className="field">
              <span>ประเภทค่า (Value Type)</span>
              <div style={{ display: 'flex', gap: '8px', marginTop: '4px' }}>
                <button
                  type="button"
                  className={`theme-select-card ${newValue === 'true' || newValue === 'false' ? 'active' : ''}`}
                  style={{ padding: '8px 12px', flex: 1, minHeight: 'auto' }}
                  onClick={() => setNewValue('true')}
                >
                  <b>🔘 สวิตช์ เปิด/ปิด (Boolean)</b>
                </button>
                <button
                  type="button"
                  className={`theme-select-card ${newValue !== 'true' && newValue !== 'false' ? 'active' : ''}`}
                  style={{ padding: '8px 12px', flex: 1, minHeight: 'auto' }}
                  onClick={() => setNewValue('20')}
                >
                  <b>🔢 ตัวเลข / ข้อความ</b>
                </button>
              </div>
            </label>

            {newValue === 'true' || newValue === 'false' ? (
              <label className="field">
                <span>สถานะเปิด/ปิด</span>
                <div style={{ paddingTop: '6px' }}>
                  <button
                    type="button"
                    role="switch"
                    aria-checked={newValue === 'true'}
                    className={`ui-toggle-switch ${newValue === 'true' ? 'active' : ''}`}
                    onClick={() => setNewValue(newValue === 'true' ? 'false' : 'true')}
                  >
                    <span className="ui-toggle-track">
                      <span className="ui-toggle-thumb" />
                    </span>
                    <span className="ui-toggle-label">
                      {newValue === 'true' ? 'เปิดใช้งาน (ON)' : 'ปิดใช้งาน (OFF)'}
                    </span>
                  </button>
                </div>
              </label>
            ) : (
              <label className="field">
                <span>ค่าที่ตั้ง (Setting Value)</span>
                <input
                  type="text"
                  className="input-text"
                  placeholder="เช่น 20, 0.7 หรือข้อความ"
                  value={newValue}
                  onChange={(e) => setNewValue(e.target.value)}
                />
              </label>
            )}

            <label className="field">
              <span>คำอธิบาย (Description)</span>
              <input
                type="text"
                className="input-text"
                placeholder="อธิบายวัตถุประสงค์และการทำงานของการตั้งค่านี้"
                value={newDesc}
                onChange={(e) => setNewDesc(e.target.value)}
              />
            </label>

            <div className="modal-actions">
              <button className="secondary" onClick={() => setShowAddModal(false)} disabled={saveBusy}>
                {t('action.cancel')}
              </button>
              <button className="primary" onClick={handleAddNewSetting} disabled={saveBusy || !newKey.trim()}>
                {saveBusy ? 'กำลังบันทึก...' : t('settings.save')}
              </button>
            </div>
          </div>
        </Modal>
      )}

      {credential !== '' && (
        <section className="panel">
          <span className="panel-label">{t('settings.promptTemplates')}</span>
          <p className="history-hint">{t('settings.promptTemplates.body')}</p>
          {promptsQuery.isLoading ? (
            <p className="history-hint">{t('table.loading')}</p>
          ) : (promptsQuery.data ?? []).length === 0 ? (
            <p className="history-hint">{t('settings.emptyPrompts')}</p>
          ) : (
            <div className="table-wrap">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>{t('settings.column.name')}</th>
                    <th>{t('settings.column.slug')}</th>
                    <th>{t('settings.column.template')}</th>
                  </tr>
                </thead>
                <tbody>
                  {(promptsQuery.data ?? []).map((tpl) => (
                    <tr key={tpl.id}>
                      <td>
                        <span className="cell-stack">
                          <b>{tpl.name}</b>
                          <small>{tpl.description}</small>
                        </span>
                      </td>
                      <td><code>{tpl.slug}</code></td>
                      <td><small style={{ whiteSpace: 'pre-wrap' }}>{tpl.template}</small></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      )}

      <section className="panel">
        <span className="panel-label">{t('settings.scopes')}</span>
        <p className="history-hint">สิทธิ์การเข้าถึงและการทำงานทั้งหมดที่คีย์นี้ได้รับอนุญาต</p>
        <ul className="scope-table">
          {ALL_SCOPES.map((scope) => (
            <li key={scope} className={has(scope) ? 'granted' : 'withheld'}>
              <span>{has(scope) ? '✓' : '·'}</span>
              <code>{scope}</code>
              <small>{scope}</small>
            </li>
          ))}
        </ul>
      </section>
    </>
  );
}
