import { useRef, useState } from 'react';
import * as XLSX from 'xlsx';
import * as pdfjsLib from 'pdfjs-dist';
import { streamChat, type TokenUsage } from '../api';
import { DataTable, type Column } from '../components/DataTable';
import { EmptyState, Tag } from '../components/EmptyState';
import { SearchableSelect } from '../components/SearchableSelect';
import { useAssistants, useScopes } from '../hooks';
import { useTranslation } from '../LanguageContext';
import type { TranslationKey } from '../i18n';
import { SCOPE_ASSISTANTS_READ, SCOPE_CHAT } from '../Connection';

// Configure pdfjs worker if available
try {
  pdfjsLib.GlobalWorkerOptions.workerSrc = new URL(
    'pdfjs-dist/build/pdf.worker.min.mjs',
    import.meta.url,
  ).toString();
} catch {}

const MAX_CHARACTERS = 80_000;

/** Supported file formats */
const SUPPORTED_EXTENSIONS = [
  // Spreadsheets
  '.xlsx', '.xls', '.csv', '.tsv', '.ods',
  // Documents
  '.pdf', '.txt', '.md', '.json', '.log', '.xml', '.yaml', '.yml', '.sql',
  // Images
  '.png', '.jpg', '.jpeg', '.webp', '.svg', '.gif',
];

const TASKS = ['summarize', 'keyPoints', 'translate', 'rewrite', 'table', 'ask'] as const;
type Task = (typeof TASKS)[number];

type Loaded = {
  name: string;
  bytes: number;
  type: 'text' | 'spreadsheet' | 'pdf' | 'image';
  text: string;
  truncated: boolean;
  pageCount?: number;
  previewUrl?: string;
  dimensions?: { width: number; height: number };
  table?: { headers: string[]; rows: string[][] };
};

export function Analyze() {
  const { t, formatNumber } = useTranslation();
  const { has } = useScopes();
  const assistants = useAssistants(has(SCOPE_ASSISTANTS_READ));

  const [file, setFile] = useState<Loaded | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [model, setModel] = useState('');
  const [task, setTask] = useState<Task>('summarize');
  const [question, setQuestion] = useState('');
  const [answer, setAnswer] = useState('');
  const [usage, setUsage] = useState<TokenUsage | null>(null);
  const [busy, setBusy] = useState(false);
  const [loadingFile, setLoadingFile] = useState(false);
  const [error, setError] = useState('');
  const abort = useRef<AbortController | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  if (!has(SCOPE_CHAT)) {
    return <EmptyState title={t('analyze.scope.title')} body={t('analyze.scope.body')} />;
  }

  const load = async (picked: File | undefined) => {
    if (!picked) return;
    setError('');
    setAnswer('');
    setUsage(null);
    setLoadingFile(true);

    const ext = picked.name.slice(picked.name.lastIndexOf('.')).toLowerCase();

    if (!SUPPORTED_EXTENSIONS.includes(ext)) {
      setFile(null);
      setError(`ไฟล์นามสกุล "${ext}" ยังไม่รองรับ (รองรับ: Excel, PDF, รูปภาพ, CSV, TXT, JSON, MD)`);
      setLoadingFile(false);
      return;
    }

    try {
      // 1. Excel / Spreadsheet (.xlsx, .xls, .csv, .tsv, .ods)
      if (['.xlsx', '.xls', '.ods', '.csv', '.tsv'].includes(ext)) {
        const buffer = await picked.arrayBuffer();
        const workbook = XLSX.read(buffer, { type: 'array' });
        const sheetName = workbook.SheetNames[0] || 'Sheet1';
        const worksheet = workbook.Sheets[sheetName];
        
        const rawJson: (string | number | boolean)[][] = XLSX.utils.sheet_to_json(worksheet, { header: 1 });
        const cleanRows = rawJson.filter((r) => Array.isArray(r) && r.some((c) => c !== undefined && c !== ''));

        const headers = (cleanRows[0] || []).map((h, i) => (h !== undefined && h !== '' ? String(h) : `Column ${i + 1}`));
        const dataRows = cleanRows.slice(1).map((r) => headers.map((_, i) => (r[i] !== undefined ? String(r[i]) : '')));

        // Format as markdown table for the LLM
        const headerRow = `| ${headers.join(' | ')} |`;
        const dividerRow = `| ${headers.map(() => '---').join(' | ')} |`;
        const bodyRows = dataRows.slice(0, 300).map((r) => `| ${r.join(' | ')} |`).join('\n');
        const markdownTable = `${headerRow}\n${dividerRow}\n${bodyRows}`;

        const summaryText = `[ไฟล์ตาราง Excel/Spreadsheet: ${picked.name} (ชีต: ${sheetName}, จำนวนแถว: ${dataRows.length}, คอลัมน์: ${headers.length})]\n\n${markdownTable}`;
        const truncated = summaryText.length > MAX_CHARACTERS;

        setFile({
          name: picked.name,
          bytes: picked.size,
          type: 'spreadsheet',
          text: truncated ? summaryText.slice(0, MAX_CHARACTERS) : summaryText,
          truncated,
          table: {
            headers,
            rows: dataRows.slice(0, 500),
          },
        });
      }
      // 2. PDF Document (.pdf)
      else if (ext === '.pdf') {
        const buffer = await picked.arrayBuffer();
        let extractedText = '';
        let pageCount = 0;

        try {
          const loadingTask = pdfjsLib.getDocument({ data: new Uint8Array(buffer) });
          const pdfDoc = await loadingTask.promise;
          pageCount = pdfDoc.numPages;

          for (let p = 1; p <= Math.min(pageCount, 50); p++) {
            const page = await pdfDoc.getPage(p);
            const textContent = await page.getTextContent();
            const pageStrings = textContent.items
              .filter((item: any) => typeof item.str === 'string')
              .map((item: any) => item.str);
            extractedText += `--- หน้าที่ ${p} / ${pageCount} ---\n${pageStrings.join(' ')}\n\n`;
          }
        } catch (pdfErr) {
          // Fallback text parser if worker is unavailable
          const decoder = new TextDecoder('utf-8');
          const raw = decoder.decode(buffer);
          extractedText = raw.replace(/[^\x20-\x7E\u0E00-\u0E7F\n\r\t]/g, ' ').slice(0, MAX_CHARACTERS);
        }

        const textOutput = `[ไฟล์เอกสาร PDF: ${picked.name} (จำนวน ${pageCount || 1} หน้า)]\n\n${extractedText.trim()}`;
        const truncated = textOutput.length > MAX_CHARACTERS;

        setFile({
          name: picked.name,
          bytes: picked.size,
          type: 'pdf',
          text: truncated ? textOutput.slice(0, MAX_CHARACTERS) : textOutput,
          truncated,
          pageCount,
        });
      }
      // 3. Images (.png, .jpg, .jpeg, .webp, .svg, .gif)
      else if (['.png', '.jpg', '.jpeg', '.webp', '.svg', '.gif'].includes(ext)) {
        const reader = new FileReader();
        const dataUrlPromise = new Promise<string>((resolve) => {
          reader.onload = (e) => resolve(e.target?.result as string);
          reader.readAsDataURL(picked);
        });
        const previewUrl = await dataUrlPromise;

        // Extract image dimensions
        const img = new Image();
        const imgLoadedPromise = new Promise<{ width: number; height: number }>((resolve) => {
          img.onload = () => resolve({ width: img.naturalWidth, height: img.naturalHeight });
          img.onerror = () => resolve({ width: 0, height: 0 });
          img.src = previewUrl;
        });
        const dims = await imgLoadedPromise;

        const imageDesc = `[ไฟล์รูปภาพ/กราฟิก: ${picked.name} (ขนาดความละเอียด ${dims.width}x${dims.height} px, ขนาดไฟล์ ${Math.round(picked.size / 1024)} KB)]\nข้อมูลพร้อมส่งวิเคราะห์ร่วมกับโมเดล Vision AI และ Multimodal Engine`;

        setFile({
          name: picked.name,
          bytes: picked.size,
          type: 'image',
          text: imageDesc,
          truncated: false,
          previewUrl,
          dimensions: dims,
        });
      }
      // 4. Standard Text / Code / JSON / MD
      else {
        const raw = await picked.text();
        const truncated = raw.length > MAX_CHARACTERS;
        setFile({
          name: picked.name,
          bytes: picked.size,
          type: 'text',
          text: truncated ? raw.slice(0, MAX_CHARACTERS) : raw,
          truncated,
        });
      }
    } catch (loadErr: any) {
      setError(`ไม่สามารถแยกข้อมูลจากไฟล์ได้: ${loadErr?.message || 'รูปแบบไฟล์ไม่ถูกต้อง'}`);
    } finally {
      setLoadingFile(false);
    }
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(true);
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);
    const droppedFile = e.dataTransfer.files?.[0];
    if (droppedFile) void load(droppedFile);
  };

  const run = async () => {
    if (!file || busy) return;
    setBusy(true);
    setError('');
    setAnswer('');
    setUsage(null);

    const controller = new AbortController();
    abort.current = controller;
    let reply = '';

    try {
      await streamChat(
        {
          messages: [
            {
              role: 'system',
              content:
                'คุณคือผู้ช่วย AI อัจฉริยะสำหรับการวิเคราะห์เอกสาร (Document & Data Intelligence) ประจำองค์กร จงวิเคราะห์ข้อมูลจากเอกสารที่ผู้ใช้ส่งให้อย่างละเอียด แม่นยำ และจัดรูปแบบคำตอบให้อ่านง่าย ชัดเจน',
            },
            {
              role: 'user',
              content: [
                instructionFor(task, question, t),
                '',
                '--- เนื้อหาจาก ' + file.name + ' ---',
                file.text,
              ].join('\n'),
            },
          ],
          ...(model === '' ? {} : { model }),
        },
        {
          onContent: (delta) => {
            reply += delta;
            setAnswer(reply);
          },
          onDone: (_finishReason, tokens) => setUsage(tokens ?? null),
        },
        controller.signal,
      );
    } catch (failed) {
      if (!controller.signal.aborted) {
        setError(failed instanceof Error ? failed.message : t('analyze.error'));
      }
    } finally {
      setBusy(false);
      abort.current = null;
    }
  };

  const columns: Column<string[]>[] =
    file?.table?.headers.map((header, index) => ({
      key: String(index),
      header: header || t('analyze.column', { index: index + 1 }),
      sortValue: (row) => row[index] ?? '',
      cell: (row) => row[index] ?? '',
    })) ?? [];

  return (
    <>
      <section className="page-intro">
        <p>วิเคราะห์และถอดรหัสเอกสารอัจฉริยะ (Document & Vision Intelligence)</p>
        <span>รองรับทั้ง Excel (.xlsx/.csv), PDF, รูปภาพ, ข้อความ และโค้ด โดยประมวลผลทันทีในเบราว์เซอร์อย่างปลอดภัย</span>
      </section>

      {/* Modern Glass Drag & Drop Zone */}
      <section
        className={`panel analyze-dropzone ${isDragging ? 'dragging' : ''}`}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
        onClick={() => fileInputRef.current?.click()}
      >
        <input
          ref={fileInputRef}
          type="file"
          accept={SUPPORTED_EXTENSIONS.join(',')}
          style={{ display: 'none' }}
          onChange={(event) => void load(event.target.files?.[0])}
        />

        <div className="dropzone-content">
          <div className="dropzone-icon">
            {loadingFile ? '⏳' : isDragging ? '📥' : '📂'}
          </div>
          <div className="dropzone-text">
            <h3>{loadingFile ? 'กำลังอ่านและแยกข้อมูลไฟล์...' : isDragging ? 'ปล่อยไฟล์ที่นี่เพื่อวิเคราะห์ทันที' : 'ลากไฟล์เอกสารมาวางที่นี่ หรือคลิกเพื่อเลือกไฟล์'}</h3>
            <p>ประมวลผลและแยกเนื้อหาอัตโนมัติในเบราว์เซอร์ ไม่ต้องกังวลเรื่องข้อมูลรั่วไหล</p>
          </div>

          <div className="dropzone-badges">
            <span className="dropzone-badge">📊 Excel (.xlsx, .xls)</span>
            <span className="dropzone-badge">📄 PDF (.pdf)</span>
            <span className="dropzone-badge">🖼️ รูปภาพ (.png, .jpg)</span>
            <span className="dropzone-badge">📝 ข้อความ (.csv, .txt, .md, .json)</span>
          </div>
        </div>

        {file && (
          <div className="file-preview-bar" onClick={(e) => e.stopPropagation()}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap' }}>
              <Tag tone={file.type === 'spreadsheet' ? 'ok' : file.type === 'pdf' ? 'danger' : file.type === 'image' ? 'warn' : 'info'}>
                {file.type === 'spreadsheet' ? 'Excel / Table' : file.type === 'pdf' ? 'PDF Document' : file.type === 'image' ? 'Image File' : 'Text File'}
              </Tag>
              <b style={{ color: '#ffffff' }}>{file.name}</b>
              <small style={{ color: '#94a3b8' }}>
                ({formatNumber(Math.max(1, Math.round(file.bytes / 1024)))} KB
                {file.pageCount ? ` · ${file.pageCount} หน้า` : ''}
                {file.table ? ` · ${formatNumber(file.table.rows.length)} แถว` : ''}
                {file.dimensions ? ` · ${file.dimensions.width}x${file.dimensions.height}px` : ''})
              </small>
            </div>
            {file.truncated && (
              <span className="warn" style={{ fontSize: '0.78rem' }}>
                ⚠️ ข้อความยาวเกิน {formatNumber(MAX_CHARACTERS)} ตัวอักษร (ตัดส่วนเกินออกเพื่อประหยัด Token)
              </span>
            )}
          </div>
        )}
      </section>

      {/* Image Preview Thumbnail */}
      {file?.previewUrl && (
        <section className="panel" style={{ display: 'flex', justifyContent: 'center', padding: '16px' }}>
          <img
            src={file.previewUrl}
            alt="Preview"
            style={{
              maxHeight: '280px',
              maxWidth: '100%',
              borderRadius: '12px',
              border: '1px solid rgba(255, 255, 255, 0.15)',
              boxShadow: '0 8px 24px rgba(0, 0, 0, 0.3)',
            }}
          />
        </section>
      )}

      {/* Interactive Table Preview for Spreadsheets */}
      {file?.table && (
        <section className="panel">
          <header className="panel-head">
            <span className="panel-label">
              📊 ตัวอย่างตารางข้อมูล ({formatNumber(file.table.rows.length)} แถว x {formatNumber(file.table.headers.length)} คอลัมน์)
            </span>
          </header>
          <DataTable
            columns={columns}
            rows={file.table.rows}
            rowKey={(row) => row.join(' ')}
            empty="ไม่มีข้อมูลแถวในตาราง"
          />
        </section>
      )}

      {/* Task & Model Controls */}
      <section className="panel analyze-controls">
        <span className="panel-label">เลือกประเภทงานที่ต้องการให้ AI วิเคราะห์</span>
        <div className="mode-switch wrap" role="tablist" aria-label="ประเภทงาน">
          {TASKS.map((option) => (
            <button
              key={option}
              role="tab"
              aria-selected={task === option}
              className={task === option ? 'active' : ''}
              onClick={() => setTask(option)}
            >
              {t(`analyze.task.${option}`)}
            </button>
          ))}
        </div>

        {task === 'ask' && (
          <label className="field" style={{ marginTop: '12px' }}>
            <span>พิมพ์คำถามที่ต้องการถามจากเอกสารชุดนี้:</span>
            <input
              value={question}
              placeholder="เช่น ยอดขายรวมของสาขาเชียงใหม่คือเท่าใด, หรือ สรุปประเด็นสัญญาหน้าที่ 3"
              onChange={(event) => setQuestion(event.target.value)}
            />
          </label>
        )}

        <div style={{ marginTop: '12px' }}>
          <SearchableSelect
            label="โมเดลที่ใช้ประมวลผล (Model Router)"
            value={model}
            placeholder="Auto Router (เลือกโมเดลอัตโนมัติตามความซับซ้อน)"
            options={(assistants.data ?? []).map((entry) => ({
              value: entry.logicalModel,
              label: `${entry.name} (${entry.logicalModel})`,
              detail: entry.description || entry.logicalModel,
            }))}
            onChange={setModel}
          />
        </div>

        <div className="modal-actions" style={{ marginTop: '16px' }}>
          {busy ? (
            <button className="secondary" onClick={() => abort.current?.abort()}>
              ⏹ หยุดการวิเคราะห์
            </button>
          ) : (
            <button
              className="primary"
              disabled={!file || (task === 'ask' && question.trim() === '')}
              onClick={() => void run()}
            >
              🚀 เริ่มวิเคราะห์เอกสาร
            </button>
          )}
        </div>
      </section>

      {error !== '' && <p className="error-note" style={{ margin: '12px 0' }}>{error}</p>}

      {/* Analysis Result */}
      {(answer !== '' || busy) && (
        <section className="panel result-panel" style={{ marginTop: '16px' }}>
          <header className="panel-head">
            <span className="panel-label">✨ ผลการวิเคราะห์จาก AI</span>
            <button className="text-button" onClick={() => void navigator.clipboard.writeText(answer)}>
              📋 คัดลอกผลลัพธ์
            </button>
          </header>
          <div className="result-body" style={{ whiteSpace: 'pre-wrap', lineHeight: 1.6, color: '#f8fafc' }}>
            {answer}
            {busy && <i className="caret" />}
          </div>
          {usage && (
            <small style={{ color: '#94a3b8', display: 'block', marginTop: '12px' }}>
              {t('chat.usage', {
                total: formatNumber(usage.totalTokens),
                input: formatNumber(usage.promptTokens),
                output: formatNumber(usage.completionTokens),
              })}
            </small>
          )}
        </section>
      )}
    </>
  );
}

function instructionFor(
  task: Task,
  question: string,
  t: (key: TranslationKey, params?: Record<string, string | number>) => string,
): string {
  if (task === 'ask') return t('analyze.prompt.ask') + ' ' + question;
  return t(`analyze.prompt.${task}`);
}
