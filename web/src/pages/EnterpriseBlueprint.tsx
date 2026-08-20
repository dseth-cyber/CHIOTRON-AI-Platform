import { useState } from 'react';
import { useTranslation } from '../LanguageContext';
import type { Language } from '../i18n';

interface BlueprintStrings {
  backBtn: string;
  title: string;
  subtitle: string;
  tabs: {
    capabilities: string;
    architecture: string;
    features: string;
    security: string;
    hardware: string;
    summary: string;
  };
  capBadge: string;
  capHeroTitle: string;
  capHeroDesc: string;
  cap1Title: string;
  cap1Desc: string;
  cap1Items: string[];
  cap2Title: string;
  cap2Desc: string;
  cap2Items: string[];
  cap3Title: string;
  cap3Desc: string;
  cap3Items: string[];
  cap4Title: string;
  cap4Desc: string;
  cap4Items: string[];
  archTitle: string;
  archDesc: string;
  archErpTitle: string;
  archErpDesc: string;
  archErpPills: { title: string; desc: string }[];
  archTechTitle: string;
  archTechItems: { title: string; desc: string }[];
  featA: { badge: string; title: string; desc: string; example: string };
  featB: { badge: string; title: string; desc: string; example: string };
  featC: { badge: string; title: string; desc: string; example: string };
  featD: { badge: string; title: string; desc: string; example: string };
  featE: { badge: string; title: string; desc: string };
  featF: { badge: string; title: string; desc: string };
  featG: { badge: string; title: string; desc: string };
  featH: { badge: string; title: string; desc: string };
  secTitle: string;
  secSteps: { num: string; title: string; desc: string }[];
  secPillars: { title: string; desc: string }[];
  hwTitle: string;
  hwHint: string;
  hwHeaders: string[];
  hwRows: string[][];
  hwRecs: { title: string; desc: string }[];
  sumTitle: string;
  sumCards: { icon: string; title: string; quote: string }[];
  budgetTitle: string;
  budgetItems: { title: string; desc: string }[];
}

const BLUEPRINT_I18N: Record<Language, BlueprintStrings> = {
  th: {
    backBtn: '← กลับไปพอร์ทัลนักพัฒนา',
    title: '🏛️ CHIOTRON Enterprise AI Platform Blueprint',
    subtitle: 'สถาปัตยกรรมและขุมพลังปัญญาประดิษฐ์ระดับองค์กร เชื่อมต่อ ERP + Knowledge + Data + Workflow + Tools พร้อมระบบ Governance และ Security ครบวงจร',
    tabs: {
      capabilities: '⚡ 04 ขุมพลังหลักของระบบ',
      architecture: '🏛️ สถาปัตยกรรม & AI Layer',
      features: '🧠 ฟังก์ชัน AI ระดับองค์กร',
      security: '🛡️ ความปลอดภัย & เสถียรภาพ',
      hardware: '💻 ฮาร์ดแวร์ & การขยายระบบ',
      summary: '📊 คุณค่า & สรุปผู้บริหาร',
    },
    capBadge: 'CORE CAPABILITIES',
    capHeroTitle: '⚡ 4 ขุมพลังหลักในการขับเคลื่อนแพลตฟอร์ม AI ระดับองค์กร',
    capHeroDesc: 'CHIOTRON AI ไม่ใช่แค่ Local LLM Server ทั่วไป แต่ถูกออกแบบด้วยสถาปัตยกรรม 4 เสาหลัก (Four Pillars) ที่ทำให้องค์กรสามารถควบคุม ใช้งาน และต่อยอด AI ได้อย่างปลอดภัยและมีประสิทธิภาพสูงสุด',
    cap1Title: 'AI Gateway & Ingress Core',
    cap1Desc: 'ประตูทางเข้าเดี่ยวของทั้งองค์กร (Single Secure Ingress) ทำหน้าที่จัดการความปลอดภัย การเข้ารหัส และควบคุมการเข้าถึงโมเดล AI ทั้งหมด',
    cap1Items: [
      'Authentication & Scope: ตรวจสอบสิทธิ์ผ่าน API Key, JWT Tokens และ Company Scopes',
      'Rate Limiting & Quotas: กำหนดเพดานคำขอ (Request Limit) และ Token Quota รายบุคคล/แผนก',
      'Data Loss Prevention (DLP): ตรวจจับและทำ Redaction ข้อมูลส่วนบุคคล (PII) แบบเรียลไทม์',
      'Streaming & SSE Engine: รองรับการสตรีมคำตอบแบบ Server-Sent Events ไร้รอยต่อ',
    ],
    cap2Title: 'AI Orchestration & Agents',
    cap2Desc: 'มันสมองในการคิดวิเคราะห์และประสานงาน (Reasoning & Dispatching) เลือกว่าคำถามใดควรใช้ LLM, RAG, Text-to-SQL หรือ Agent Tools',
    cap2Items: [
      'ReAct Agent Loop: วงรอบการคิด-วางแผน-ลงมือทำ (Reasoning + Action) หลายขั้นตอน',
      'Hybrid Search & RAG: ผสาน Vector Embedding (HNSW) และ Full-Text Search อย่างแม่นยำ',
      'Text-to-SQL Engine: แปลงคำถามภาษาธรรมชาติเป็นคำสั่ง SQL วิเคราะห์ข้อมูล ERP อัตโนมัติ',
      'MCP & Tool Calling: เชื่อมต่อเครื่องมือและ API ภายนอกขององค์กรภายใต้ Sandbox ที่ปลอดภัย',
    ],
    cap3Title: 'Audit, Logging & Governance',
    cap3Desc: 'ระบบธรรมาภิบาลและการกำกับดูแลความโปร่งใส บันทึกและตรวจสอบประวัติการใช้งาน AI ทุกมิติเพื่อความปลอดภัยระดับ Enterprise Compliance',
    cap3Items: [
      'Turn-by-turn Auditing: บันทึก Log ทุกคำถาม-คำตอบ, โมเดลที่เรียกใช้, สิทธิ์ที่อนุมัติ และเวลาประมวลผล',
      'Token Usage Analytics: ติดตาม Prompt Tokens, Completion Tokens และประเมินต้นทุนแม่นยำ',
      'Multi-Tenant Data Isolation: กรองข้อมูลตาม Company Filter และ User Permission ข้อมูลไม่รั่วไหลข้ามฝ่าย',
      'Observability & Metrics: ส่งข้อมูล Prometheus Metrics, Grafana Dashboards และ OpenTelemetry Traces',
    ],
    cap4Title: 'Intent & 3-Tier Model Routing',
    cap4Desc: 'ระบบวิเคราะห์ความตั้งใจและจัดสรรโมเดลอัจฉริยะ (Intent Router & Model Dispatcher) พร้อม 3-Tier Multi-Model Routing สำหรับงานองค์กร',
    cap4Items: [
      '3-Tier Local GPU Routing: ⚡ Qwen3 0.6B (งานง่าย/ด่วน) ➔ 📚 Qwen3 4B (งานเอกสาร/RAG) ➔ 🧠 Qwen3 8B (ERP/SQL/Agent)',
      'Policy-First Authorization: ตรวจสอบสิทธิ์ RBAC / Department Filter ที่ Gateway ก่อนส่งคำขอไปยังโมเดล',
      'Zero Cognitive Load (Auto Mode): ผู้ใช้พิมพ์คำถามตามปกติ ระบบ Gateway ตัดสินใจเลือก Tool + RAG + Model + GPU Node อัตโนมัติ',
      'Dynamic Compute Balancing: กระจายโหลดคำขออัตโนมัติ สลับระหว่าง Local On-Premise GPU และ Cloud Adapters ไร้รอยต่อ',
    ],
    archTitle: '🏛️ 1. ภาพรวมสถาปัตยกรรม CHIOTRON Enterprise AI Platform',
    archDesc: 'พนักงานคุยกับ AI ตัวเดียว แต่เบื้องหลัง AI สามารถเข้าถึง "สิ่งที่เขามีสิทธิ์เข้าถึง" ในระบบบริษัทได้อย่างครบถ้วน',
    archErpTitle: '🔗 14. AI Layer เหนือ ERP (Zero Disruption Integration)',
    archErpDesc: 'ระบบทำหน้าที่เป็น AI Intelligence Layer ซ้อนทับอยู่เหนือระบบ ERP เดิมขององค์กร ทำให้สามารถเพิ่มความสามารถของ AI ได้ทันทีโดยไม่ต้องแก้ Core ERP',
    archErpPills: [
      { title: 'ไม่ต้องแก้ Core ERP', desc: 'เชื่อมต่อผ่าน ERP API, Read-Only Database View หรือ MCP Tools' },
      { title: 'สิทธิ์ตรงกับ ERP', desc: 'User A เห็นเฉพาะข้อมูลบริษัท A และแผนกที่ได้รับอนุญาตเท่านั้น' },
      { title: 'ลดเวลาทำรายงาน', desc: 'ถามสรุป วิเคราะห์แนวโน้ม และสร้างกราฟผู้บริหารได้ในเสี้ยววินาที' },
    ],
    archTechTitle: '🚀 15. ความทันสมัย & Tech Stack ชั้นนำ',
    archTechItems: [
      { title: 'Local LLM Engine:', desc: 'ทำงานบน On-Premise GPU ปลอดภัย 100% ไม่ส่งข้อมูลออกภายนอก' },
      { title: 'AI Gateway & Router:', desc: 'กำหนดเส้นทางโมเดลและจัดการความปลอดภัยแบบรวมศูนย์' },
      { title: 'Hybrid Search & GraphRAG:', desc: 'ค้นหาเอกสารด้วยความหมายเชิงลึกและโครงสร้างความสัมพันธ์' },
      { title: 'Text-to-SQL & Tool Sandbox:', desc: 'สืบค้นฐานข้อมูลและสั่งงานเครื่องมืออัตโนมัติ' },
      { title: 'Isolated Compute Plane:', desc: 'แยก Control Plane (VM4) และ Compute Plane (VM5) ชัดเจน' },
    ],
    featA: {
      badge: 'A. GENERAL AI',
      title: '✨ ผู้ช่วยอัจฉริยะทั่วไปครบทุกมิติ',
      desc: 'Chat, Q&A, สรุปใจความ, แปลภาษา, Brainstorm, วิเคราะห์ PDF/Word/Excel/CSV, OCR ถอดข้อความจากภาพ, สร้างกราฟ, เขียนอีเมล, สร้างโค้ด และ Voice AI พูดคุยสด',
      example: '💬 "สรุปไฟล์ Excel นี้ให้หน่อย และทำรายงานผู้บริหารพร้อมกราฟเปรียบเทียบ"',
    },
    featB: {
      badge: '3. KNOWLEDGE AI',
      title: '📚 ค้นหาความรู้ขององค์กร (Enterprise Knowledge)',
      desc: 'ค้นหาข้าม PDF, SOP, นโยบายบริษัท, คู่มือปฏิบัติงาน, NAS, Database โดยคำตอบจะมาจากการสืบค้น Knowledge Base ➔ Hybrid Search ➔ Permission Filter ➔ RAG ➔ LLM ➔ คำตอบ + แหล่งอ้างอิง (Source)',
      example: '💬 "ขั้นตอนการตรวจสอบคุณภาพสินค้าตัวนี้ตามคู่มือ SOP ล่าสุดต้องทำอย่างไร?"',
    },
    featC: {
      badge: '4. ERP AI',
      title: '💼 ถามข้อมูล ERP ด้วยภาษาธรรมชาติ',
      desc: 'พนักงานและผู้บริหารสามารถถามยอดขาย, สต็อกสินค้า, จำนวนการผลิต, ลูกหนี้ค้างชำระ โดยระบบจะตัดสินใจอัตโนมัติว่าจะใช้ ERP API, SQL, RAG หรือ Agent Tool เพื่อนำข้อมูลจริงมาตอบ',
      example: '💬 "ยอดขายเดือนนี้เทียบกับเดือนก่อนเป็นอย่างไร และสินค้าตัวไหนใกล้หมดสต็อก?"',
    },
    featD: {
      badge: '5. TEXT-TO-SQL',
      title: '📊 แปลงภาษาเป็นคำสั่ง SQL (Text-to-SQL & BI)',
      desc: 'แปลงคำถามภาษาธรรมชาติเป็นคำสั่ง SQL ที่ปลอดภัย (Read-Only) ดึงข้อมูลจากฐานข้อมูลจริงมาสรุปเป็นตารางและกราฟได้ทันที',
      example: '💬 "แสดงยอดขาย 10 อันดับแรกของปีนี้ แยกตามลูกค้า พร้อมคำนวณ % สัดส่วน"',
    },
    featE: {
      badge: '6. AGENTIC RAG',
      title: '🔍 วิเคราะห์ปัญหาข้ามระบบ (Root Cause Analysis)',
      desc: 'เมื่อถามว่า "ทำไมยอดผลิตสัปดาห์นี้ตก" AI Agent จะวิ่งไปดึงข้อมูลจากฝ่ายผลิต + คลังสินค้า + เครื่องจักร + การซ่อมบำรุง + QC มาวิเคราะห์เปรียบเทียบหาสาเหตุที่แท้จริง',
    },
    featF: {
      badge: '7. GRAPHRAG',
      title: '🕸️ โครงสร้างความสัมพันธ์ (Enterprise Knowledge Graph)',
      desc: 'เข้าใจความเชื่อมโยงซับซ้อน: ลูกค้า ➔ คำสั่งซื้อ ➔ สินค้า ➔ การผลิต ➔ วัตถุดิบ ➔ เครื่องจักร ➔ QC ➔ ปัญหาที่เกิดขึ้น',
    },
    featG: {
      badge: '8. AI AGENTS',
      title: '🤖 ผู้ช่วยเฉพาะทางรายตำแหน่ง (Specialized Agents)',
      desc: 'Finance Agent (ตรวจงบ, กระแสเงินสด, ลูกหนี้), HR Agent (ถามวันลา, นโยบาย, สวัสดิการ), Production Agent (เช็คยอดผลิต, OEE, เครื่องจักร), Inventory Agent (เช็คสต็อก, วัตถุดิบขาด)',
    },
    featH: {
      badge: '9. MCP TOOLS',
      title: '🛠️ เชื่อมต่อเครื่องมือภายนอก (Model Context Protocol)',
      desc: 'ให้ AI สามารถเรียกใช้ Tools และ APIs ขององค์กรได้อย่างปลอดภัย ภายใต้การควบคุมสิทธิ์, Rate Limit, Audit และ Scope',
    },
    secTitle: '🔒 16. ระบบความปลอดภัยหลายชั้น (Multi-Layer Security)',
    secSteps: [
      { num: '1', title: 'ยืนยันตัวตน', desc: 'เชื่อมต่อ SSO และ ERP Identity' },
      { num: '2', title: 'ตรวจสอบสิทธิ์', desc: 'กำหนด Scope และ Role ผ่าน JWT Tokens' },
      { num: '3', title: 'AI Gateway', desc: 'ตรวจจับและ Redact ข้อมูลส่วนบุคคล (PII)' },
      { num: '4', title: 'Company Filter', desc: 'แยกฐานข้อมูล Multi-Tenant ชัดเจน' },
      { num: '5', title: 'Read-Only DB', desc: 'สืบค้นข้อมูลผ่าน Sandbox ที่ปลอดภัย' },
    ],
    secPillars: [
      { title: '🛡️ 17. การแยกส่วนระบบ (Fault Isolation)', desc: 'แยก Control Plane (VM4) และ Compute Plane (VM5) ออกจากกัน ชัดเจน หาก AI ล่ม ระบบ ERP ยังทำงานได้ปกติ 100%' },
      { title: '🎯 18. ป้องกันข้อมูลมั่ว (Zero Hallucination)', desc: 'บังคับให้ AI ตอบอิงจากข้อมูลจริงใน ERP และ Knowledge Base พร้อมแสดงเอกสารอ้างอิง (Source Citations)' },
      { title: '⚡ 19. ความเร็ว & ประสิทธิภาพ (Performance)', desc: 'มีระบบ Caching คำถามซ้ำ (Redis Semantic Cache) และการสตรีมคำตอบ (Streaming SSE) ลดภาระ GPU' },
      { title: '💰 20. การควบคุมต้นทุน (Cost & Budget Governance)', desc: 'กำหนดเพดานการใช้งาน Token รายบุคคล/รายแผนก พร้อม Dashboard ตรวจสอบค่าใช้จ่ายเรียลไทม์' },
    ],
    hwTitle: '💻 22. ตารางประมาณการขนาดฮาร์ดแวร์ (Hardware Sizing Guide)',
    hwHint: '*ประเมินบนสมมติฐานการใช้งาน Local LLM 7B–32B, RAG, Text-to-SQL, Agent และการสร้างรายงานเป็นหลัก',
    hwHeaders: ['จำนวนผู้ใช้', 'Concurrent Load', 'GPU ที่แนะนำ', 'RAM', 'Storage NVMe', 'งบ AI Infra ประมาณการ'],
    hwRows: [
      ['10 คน', '2–4 คน', 'RTX 5090 (32GB)', '64 GB', '2 TB NVMe', '250k – 400k บาท'],
      ['30 คน', '4–8 คน', 'RTX 5090 (32GB)', '128 GB', '4 TB NVMe', '400k – 600k บาท'],
      ['50 คน', '8–12 คน', '2× RTX 5090 หรือ 48GB', '128–192 GB', '4–8 TB NVMe', '600k – 1.0M บาท'],
      ['100 คน', '12–25 คน', '2× RTX 5090 / RTX 6000 Ada', '192–256 GB', '8 TB NVMe', '900k – 1.5M บาท'],
      ['200 คน', '25–50 คน', '2–4× 48–96GB Blackwell', '256–512 GB', '8–16 TB NVMe', '1.5M – 3.0M บาท'],
      ['300 คน+', '40–75+ คน', '4× 48–96GB / Multi-Node', '512 GB+', '16 TB+ NVMe', '2.5M – 5.0M+ บาท'],
    ],
    hwRecs: [
      { title: '💡 23. คำแนะนำการลงทุนแบบคุ้มค่า', desc: 'เริ่มจาก 10–30 คนก่อนด้วย Node 4 (1 GPU) แล้ววัด Concurrent Usage และ Tokens/sec จริง 1–3 เดือน ค่อย Scale Out ตามจริง' },
      { title: '🔀 28. กลยุทธ์ 3-Tier Multi-Model Routing', desc: '⚡ Qwen3 0.6B (งานง่าย/ประมวลผลด่วน 0.5s), 📚 Qwen3 4B (งานทั่วไป/RAG คลังความรู้), 🧠 Qwen3 8B (ERP/SQL/Agent วิเคราะห์เชิงลึก) พร้อมโหมด 🤖 Auto อัจฉริยะ' },
    ],
    sumTitle: '📊 31. สรุปคุณค่าของ CHIOTRON Enterprise AI Platform ต่อแต่ละกลุ่ม',
    sumCards: [
      { icon: '🏢', title: 'สำหรับองค์กร', quote: '"มี AI Platform ของบริษัทเอง ความรู้ไม่สูญหายไปกับพนักงานที่ลาออก และข้อมูลไม่รั่วไหลออกนอกองค์กร"' },
      { icon: '👨‍💼', title: 'สำหรับพนักงาน', quote: '"มี Digital Assistant ส่วนตัวที่รู้จักงานและข้อมูลบริษัท ช่วยลดขั้นตอนค้นหาเอกสารและทำรายงาน"' },
      { icon: '📊', title: 'สำหรับผู้บริหาร', quote: '"ถามข้อมูลธุรกิจด้วยภาษาคน และได้ Insight พร้อมตารางและกราฟประกอบการตัดสินใจได้ทันที"' },
      { icon: '👨‍💻', title: 'สำหรับฝ่าย IT', quote: '"มีศูนย์กลางบริหารจัดการ AI (Central Governance) ควบคุม User, Token, Model, GPU และ Security ได้ในที่เดียว"' },
    ],
    budgetTitle: '💼 โครงสร้างการจัดสรรงบประมาณ 4 ส่วน (Recommended AI Budget Structure)',
    budgetItems: [
      { title: '1. AI Platform Software', desc: 'Control Plane, User Portal, AI Gateway, RAG Engine, Agent Framework, Tool Sandbox' },
      { title: '2. AI Compute Infrastructure', desc: 'GPU Acceleration, Enterprise Server, RAM, NVMe High-Speed Storage' },
      { title: '3. Enterprise Integration', desc: 'การเชื่อมต่อ ERP Database, Company Knowledge Ingestion, Multi-Layer Security & RBAC' },
      { title: '4. Operations & Maintenance', desc: 'Observability Monitoring, Backup, Token Governance, Model Versioning & Updates' },
    ],
  },
  en: {
    backBtn: '← Back to Developer Portal',
    title: '🏛️ CHIOTRON Enterprise AI Platform Blueprint',
    subtitle: 'Enterprise AI Architecture connecting ERP + Company Knowledge + Documents + Data + Workflows + Tools with End-to-End Governance and Security.',
    tabs: {
      capabilities: '⚡ 04 Core Capabilities',
      architecture: '🏛️ Architecture & AI Layer',
      features: '🧠 Enterprise AI Features',
      security: '🛡️ Security & Reliability',
      hardware: '💻 Hardware & Sizing',
      summary: '📊 Value & Executive Summary',
    },
    capBadge: 'CORE CAPABILITIES',
    capHeroTitle: '⚡ Four Core Pillars Driving the Enterprise AI Platform',
    capHeroDesc: 'CHIOTRON AI is not merely a local LLM server. It is engineered with a four-pillar architecture enabling safe governance, granular access control, and scalable AI adoption.',
    cap1Title: 'AI Gateway & Ingress Core',
    cap1Desc: 'Single secure ingress for the entire enterprise, handling authentication, encryption, token rate limits, and compliance policy enforcement.',
    cap1Items: [
      'Authentication & Scope: Verified via API Keys, JWT Tokens, and Company Scope isolation.',
      'Rate Limiting & Quotas: Enforced request ceilings and token budgets per user and department.',
      'Data Loss Prevention (DLP): Real-time scanning and PII redaction prior to model inference.',
      'Streaming & SSE Engine: High-performance Server-Sent Events for real-time token output.',
    ],
    cap2Title: 'AI Orchestration & Agents',
    cap2Desc: 'Reasoning and dispatch brain that determines whether a query requires pure LLM, Vector RAG, Text-to-SQL, or multi-step Agent Tooling.',
    cap2Items: [
      'ReAct Agent Loop: Multi-turn Reasoning + Action execution across company tools.',
      'Hybrid Search & RAG: Precision fusion of HNSW Vector Embeddings and Full-Text Search.',
      'Text-to-SQL Engine: Converts natural language inquiries into validated SQL for ERP analytics.',
      'MCP & Tool Calling: Secure Model Context Protocol sandboxing for enterprise APIs.',
    ],
    cap3Title: 'Audit, Logging & Governance',
    cap3Desc: 'Comprehensive transparency and audit framework recording every interaction for regulatory and enterprise compliance.',
    cap3Items: [
      'Turn-by-turn Auditing: Complete logging of prompts, completions, routing, scopes, and latency.',
      'Token Usage Analytics: Accurate breakdown of prompt vs completion tokens and cost tracking.',
      'Multi-Tenant Data Isolation: Strict company and department boundary enforcement.',
      'Observability & Metrics: Prometheus metrics, Grafana dashboards, and OpenTelemetry tracing.',
    ],
    cap4Title: 'Intent & 3-Tier Model Routing',
    cap4Desc: 'Intelligent intent analysis and model dispatching engine with 3-tier multi-model routing for enterprise workloads.',
    cap4Items: [
      '3-Tier Local GPU Routing: ⚡ Qwen3 0.6B (Fast/Simple) ➔ 📚 Qwen3 4B (General/RAG) ➔ 🧠 Qwen3 8B (ERP/SQL/Agent)',
      'Policy-First Authorization: Enforce RBAC and department isolation at the Gateway before dispatching to models',
      'Zero Cognitive Load (Auto Mode): Users chat naturally while the system dynamically selects Tools, RAG, and Models',
      'Dynamic Compute Balancing: Automated load balancing and seamless fallback between Local On-Premise GPUs and Cloud adapters',
    ],
    archTitle: '🏛️ 1. CHIOTRON Enterprise AI Platform Architecture',
    archDesc: 'Employees interact with a single unified AI assistant, while behind the scenes AI accesses only what each employee is authorized to see.',
    archErpTitle: '🔗 14. AI Layer Above ERP (Zero Disruption Integration)',
    archErpDesc: 'CHIOTRON operates as an intelligent overlay above existing ERP systems, unlocking AI superpowers without modifying core ERP codebase.',
    archErpPills: [
      { title: 'Zero ERP Code Changes', desc: 'Integrates via ERP APIs, Read-Only Database Views, or MCP tools.' },
      { title: 'RBAC Permission Match', desc: 'User A only views Company A data according to ERP role grants.' },
      { title: 'Automated BI Reporting', desc: 'Summarizes sales trends, anomalies, and graphs in seconds.' },
    ],
    archTechTitle: '🚀 15. Modern Enterprise Tech Stack',
    archTechItems: [
      { title: 'Local LLM Engine:', desc: 'Runs on On-Premise GPUs with 100% data residency and zero leakage.' },
      { title: 'AI Gateway & Router:', desc: 'Centralized model dispatching, security enforcement, and rate limiting.' },
      { title: 'Hybrid Search & GraphRAG:', desc: 'Deep semantic retrieval fused with knowledge graph relationships.' },
      { title: 'Text-to-SQL & Tool Sandbox:', desc: 'Safe natural language querying against read-only replicas.' },
      { title: 'Isolated Compute Plane:', desc: 'Strict decoupling between Control Plane (VM4) and Compute Plane (VM5).' },
    ],
    featA: {
      badge: 'A. GENERAL AI',
      title: '✨ All-in-One Digital Workplace Assistant',
      desc: 'Chat, Q&A, summarization, translation, brainstorming, PDF/Excel/CSV document analysis, OCR, code generation, and interactive Voice AI.',
      example: '💬 "Summarize this quarterly Excel sheet and prepare executive graphs comparing last month."',
    },
    featB: {
      badge: '3. KNOWLEDGE AI',
      title: '📚 Enterprise Knowledge Retrieval',
      desc: 'Search across PDFs, SOPs, manuals, company policies, and NAS databases with verified Source Citations and Permission Filters.',
      example: '💬 "What is the standard SOP quality inspection procedure for product batch A-102?"',
    },
    featC: {
      badge: '4. ERP AI',
      title: '💼 Natural Language ERP Querying',
      desc: 'Query live revenue, stock balances, production counts, and accounts receivable with automatic tool selection.',
      example: '💬 "What is this month’s gross revenue compared to last month and which items are low in stock?"',
    },
    featD: {
      badge: '5. TEXT-TO-SQL',
      title: '📊 Text-to-SQL & Business Intelligence',
      desc: 'Translates natural language into sanitized SQL queries against read-only replicas, rendering instant summary tables and charts.',
      example: '💬 "List top 10 customers by revenue this year with their percentage contribution."',
    },
    featE: {
      badge: '6. AGENTIC RAG',
      title: '🔍 Multi-System Root-Cause Analysis',
      desc: 'Analyzes complex questions across Production + Inventory + Downtime + Maintenance to uncover actual business bottlenecks.',
    },
    featF: {
      badge: '7. GRAPHRAG',
      title: '🕸️ GraphRAG & Knowledge Graphs',
      desc: 'Maps interconnected entities: Customer ➔ Order ➔ Product ➔ Production ➔ Lot ➔ Machine ➔ Quality ➔ Rejection.',
    },
    featG: {
      badge: '8. AI AGENTS',
      title: '🤖 Specialized Departmental Agents',
      desc: 'Finance Agent (Revenue, Cash Flow, AR/AP), HR Agent (Leave Policies, Benefits), Production Agent (WIP, OEE), Inventory Agent (Reorder Points).',
    },
    featH: {
      badge: '9. MCP TOOLS',
      title: '🛠️ Model Context Protocol & Tool Sandbox',
      desc: 'Enables AI to trigger enterprise tools under strict Authorization, Rate Limit, Audit, and Scope governance.',
    },
    secTitle: '🔒 16. Multi-Layer Enterprise Security Pipeline',
    secSteps: [
      { num: '1', title: 'User Identity', desc: 'SSO & Enterprise Identity verification' },
      { num: '2', title: 'JWT & Scopes', desc: 'Granular role-based access token grants' },
      { num: '3', title: 'AI Gateway', desc: 'DLP scanning & PII redaction engine' },
      { num: '4', title: 'Company Filter', desc: 'Strict multi-tenant boundary isolation' },
      { num: '5', title: 'Read-Only DB', desc: 'Secure execution sandbox' },
    ],
    secPillars: [
      { title: '🛡️ 17. Fault Isolation', desc: 'Decoupled Control Plane and Compute Plane ensure ERP uptime even if GPU crashes.' },
      { title: '🎯 18. Zero Hallucination Grounding', desc: 'Forces answers to be grounded in verified ERP and knowledge facts with Source Citations.' },
      { title: '⚡ 19. High Performance & Caching', desc: 'Redis Semantic Caching, Embedding Caching, and Streaming output reduce GPU overhead.' },
      { title: '💰 20. Budget & Cost Governance', desc: 'Enforces department-level token quotas with real-time analytics.' },
    ],
    hwTitle: '💻 22. Hardware Sizing & Capacity Planning Guide',
    hwHint: '*Estimated based on Local LLM (7B–32B), RAG, Text-to-SQL, Agent, and Report workloads.',
    hwHeaders: ['Users', 'Concurrent Load', 'Recommended GPU', 'RAM', 'Storage NVMe', 'Estimated AI Infra Budget'],
    hwRows: [
      ['10 Users', '2–4 concurrent', 'RTX 5090 (32GB)', '64 GB', '2 TB NVMe', '250k – 400k THB'],
      ['30 Users', '4–8 concurrent', 'RTX 5090 (32GB)', '128 GB', '4 TB NVMe', '400k – 600k THB'],
      ['50 Users', '8–12 concurrent', '2× RTX 5090 or 48GB', '128–192 GB', '4–8 TB NVMe', '600k – 1.0M THB'],
      ['100 Users', '12–25 concurrent', '2× RTX 5090 / RTX 6000 Ada', '192–256 GB', '8 TB NVMe', '900k – 1.5M THB'],
      ['200 Users', '25–50 concurrent', '2–4× 48–96GB Blackwell', '256–512 GB', '8–16 TB NVMe', '1.5M – 3.0M THB'],
      ['300+ Users', '40–75+ concurrent', '4× 48–96GB / Multi-Node', '512 GB+', '16 TB+ NVMe', '2.5M – 5.0M+ THB'],
    ],
    hwRecs: [
      { title: '💡 23. Phased Investment Strategy', desc: 'Begin with 10–30 users on Node 4 (1 GPU), observe concurrent token load for 1–3 months, then scale out compute nodes.' },
      { title: '🔀 28. Model Routing Strategy', desc: 'Small LLMs (3B-8B) for classification, Medium LLMs (14B-32B) for general RAG/SQL, Large LLMs (70B+) for executive reasoning.' },
    ],
    sumTitle: '📊 31. Stakeholder Value Proposition',
    sumCards: [
      { icon: '🏢', title: 'For Enterprise', quote: '"Retains company knowledge permanently and ensures 100% confidential data sovereignty."' },
      { icon: '👨‍💼', title: 'For Employees', quote: '"A personalized AI assistant that understands company data, eliminating hours of manual report writing."' },
      { icon: '📊', title: 'For Executives', quote: '"Query business KPIs in natural language and receive immediate actionable insights and charts."' },
      { icon: '👨‍💻', title: 'For IT Teams', quote: '"A centralized AI management cockpit governing users, tokens, models, GPUs, and security."' },
    ],
    budgetTitle: '💼 Recommended 4-Pillar AI Budget Framework',
    budgetItems: [
      { title: '1. AI Platform Software', desc: 'Control Plane, User Portal, AI Gateway, RAG Engine, Agent Framework, Tool Sandbox.' },
      { title: '2. AI Compute Infrastructure', desc: 'GPU Acceleration, Enterprise Servers, High-Speed RAM, and NVMe Storage.' },
      { title: '3. Enterprise Integration', desc: 'ERP database connectors, knowledge ingestion pipelines, and RBAC security.' },
      { title: '4. Operations & Governance', desc: 'Observability monitoring, automated backups, token quota governance, model lifecycles.' },
    ],
  },
  zh: {
    backBtn: '← 返回开发者门户',
    title: '🏛️ CHIOTRON 企业级 AI 平台蓝图',
    subtitle: '连接 ERP + 企业知识库 + 文档 + 数据 + 工作流 + 工具的企业级 AI 架构，具备全方位的安全监管与权限控制。',
    tabs: {
      capabilities: '⚡ 04 核心能力',
      architecture: '🏛️ 架构与 AI 层',
      features: '🧠 企业级 AI 功能',
      security: '🛡️ 安全与稳定性',
      hardware: '💻 硬件配置与扩展',
      summary: '📊 价值与高管总结',
    },
    capBadge: 'CORE CAPABILITIES',
    capHeroTitle: '⚡ 驱动企业级 AI 平台的四大核心支柱',
    capHeroDesc: 'CHIOTRON AI 不仅仅是本地 LLM 服务器，而是专为企业设计的四支柱架构，确保数据主权、权限隔离与高可用扩展。',
    cap1Title: 'AI 网关与接入核心 (AI Gateway)',
    cap1Desc: '全企业统一安全接入入口，负责身份验证、加密传输、速率限制及合规策略执行。',
    cap1Items: [
      '身份验证与作用域：通过 API Key、JWT Token 及公司作用域进行严格隔离。',
      '速率限制与配额：按个人和部门设定请求上限及 Token 预算配额。',
      '数据防泄漏 (DLP)：在模型推理前实时检测并脱敏个人隐私 (PII) 数据。',
      '流式输出引擎：基于 Server-Sent Events (SSE) 实现极速流式打字效果。',
    ],
    cap2Title: 'AI 编排与智能体 (AI Orchestration)',
    cap2Desc: '智能决策大脑，自动判断问题应调用通用 LLM、知识库 RAG、Text-to-SQL 还是 Agent 业务工具。',
    cap2Items: [
      'ReAct 智能体循环：具备思考-规划-行动的多轮复杂任务执行能力。',
      '混合检索 (Hybrid RAG)：结合 HNSW 向量检索与全文检索，提升召回精度。',
      'Text-to-SQL 引擎：将自然语言问题自动转换为安全 SQL 查询 ERP 数据库。',
      'MCP 工具调用：在安全沙箱内集成企业内部 API 与第三方工具。',
    ],
    cap3Title: '审计、日志与治理 (Audit & Governance)',
    cap3Desc: '全方位的透明监管框架，记录所有 AI 交互细节，满足企业合规审计要求。',
    cap3Items: [
      '逐轮交互审计：记录 Prompt、回答、调用模型、耗时及权限审批日志。',
      'Token 使用量分析：精确统计 Prompt/Completion Tokens，评估运营成本。',
      '多租户数据隔离：基于 Company Filter 确保跨部门数据绝对不互通。',
      '可观测性指标：集成 Prometheus 监控、Grafana 仪表盘与 OpenTelemetry 链路。',
    ],
    cap4Title: '意图识别与三层模型路由 (Intent & 3-Tier Model Routing)',
    cap4Desc: '智能意图分析与模型调度引擎，具备企业级三层多模型路由架构 (3-Tier Multi-Model Routing)。',
    cap4Items: [
      '三层本地 GPU 路由: ⚡ Qwen3 0.6B (极速/简单 0.5s) ➔ 📚 Qwen3 4B (通用/RAG 知识库) ➔ 🧠 Qwen3 8B (ERP/SQL/Agent 深度推理)',
      '安全策略优先 (Policy-First): 在网关层先校验 RBAC 权限与部门隔离，未授权指令直接拦截，不消耗模型 Token',
      '零认知负担 (Auto 模式): 用户自然对话，系统自动决策匹配最佳 Tool、RAG、模型与 GPU 节点',
      '动态算力负载均衡: 自动化负载分流，支持本地私有 GPU 与云端适配器之间无缝切换与容灾',
    ],
    archTitle: '🏛️ 1. CHIOTRON 企业级 AI 平台整体架构',
    archDesc: '员工与统一 AI 交互，后台 AI 仅访问该员工拥有合法查看权限的企业数据。',
    archErpTitle: '🔗 14. 叠加在 ERP 之上的 AI 智能层 (零代码破坏集成)',
    archErpDesc: '无需重构或修改现有 ERP 系统核心代码，即可为企业 ERP 赋予前沿 AI 智能分析能力。',
    archErpPills: [
      { title: '无需改动 ERP', desc: '通过 ERP API、只读数据库视图或 MCP 工具安全连接。' },
      { title: '严格继承 ERP 权限', desc: '员工 A 仅能查询其所属公司和部门的授权数据。' },
      { title: '告别繁琐报表导出', desc: '用自然语言即时生成销售对比趋势与高管图表。' },
    ],
    archTechTitle: '🚀 15. 前沿企业级技术栈',
    archTechItems: [
      { title: '本地 LLM 引擎：', desc: '私有化部署，数据 100% 不离境、不外传。' },
      { title: 'AI 网关与路由器：', desc: '集中式安全调度与访问流量控制。' },
      { title: '混合检索与 GraphRAG：', desc: '深度语义分析结合知识图谱实体关系。' },
      { title: 'Text-to-SQL 与工具沙箱：', desc: '安全只读数据库交互，防止注入风险。' },
      { title: '隔离计算平面：', desc: '控制面 (VM4) 与计算面 (VM5) 物理隔离。' },
    ],
    featA: {
      badge: 'A. GENERAL AI',
      title: '✨ 全方位数字化办公助手',
      desc: '对话问答、文档摘要、语言翻译、头脑风暴、PDF/Word/Excel/CSV 深度分析、OCR 文字提取、代码生成与实时语音 AI。',
      example: '💬 "请帮我总结这份 Excel 销售表，并生成上月对比的高管图表"',
    },
    featB: {
      badge: '3. KNOWLEDGE AI',
      title: '📚 企业私有知识库检索 (Enterprise RAG)',
      desc: '检索跨 PDF、SOP 流程、公司政策、NAS 文档与数据库，答案均带有原文引用来源 (Source Citations)。',
      example: '💬 "根据最新的 SOP 操作手册，该产品的质检步骤有哪些？"',
    },
    featC: {
      badge: '4. ERP AI',
      title: '💼 自然语言 ERP 智能查询',
      desc: '员工与主管可用自然语言查询销售额、库存余额、生产进度与应收账款，系统自动选择最佳数据源。',
      example: '💬 "本月销售额较上月如何？有哪些热销品即将断货？"',
    },
    featD: {
      badge: '5. TEXT-TO-SQL',
      title: '📊 Text-to-SQL 与商业智能 (BI)',
      desc: '将自然语言指令转化为经过安全校验的 SQL，在只读数据库中秒级生成结构化表格和图表。',
      example: '💬 "列出今年销售额前 10 名的客户及其销售额占比"',
    },
    featE: {
      badge: '6. AGENTIC RAG',
      title: '🔍 跨系统多维度归因分析',
      desc: '当询问“为何本周产量下降”时，Agent 自动调取生产、库存、停机、报废与维修多维数据综合诊断原因。',
    },
    featF: {
      badge: '7. GRAPHRAG',
      title: '🕸️ GraphRAG 与企业知识图谱',
      desc: '理解网状业务关联：客户 ➔ 订单 ➔ 产品 ➔ 生产 ➔ 批次 ➔ 设备 ➔ 质检 ➔ 不良率。',
    },
    featG: {
      badge: '8. AI AGENTS',
      title: '🤖 岗位专业 AI 智能体',
      desc: '财务 Agent (营收、账期、现金流)、HR Agent (假期、福利)、生产 Agent (WIP、OEE)、仓储 Agent (安全库存预警)。',
    },
    featH: {
      badge: '9. MCP TOOLS',
      title: '🛠️ 模型上下文协议 (MCP) 与工具沙箱',
      desc: '在严格的鉴权、限流、审计及 Scope 约束下让 AI 安全调用企业内部系统工具。',
    },
    secTitle: '🔒 16. 多层企业级安全管道',
    secSteps: [
      { num: '1', title: '用户身份验证', desc: '对接企业统一身份认证' },
      { num: '2', title: 'JWT 权限分配', desc: '根据角色授予细粒度访问权限' },
      { num: '3', title: 'AI 安全网关', desc: 'DLP 实时检测与敏感信息脱敏' },
      { num: '4', title: '公司租户过滤', desc: '多租户数据边界物理隔离' },
      { num: '5', title: '只读沙箱执行', desc: '安全受控的数据查询执行' },
    ],
    secPillars: [
      { title: '🛡️ 17. 故障物理隔离', desc: '控制面与计算面彻底解耦，GPU 或模型异常绝不影响现有 ERP 系统运行。' },
      { title: '🎯 18. 零幻觉数据锚定 (Grounding)', desc: '强制基于 ERP 与知识库真实数据回答，杜绝大模型凭空臆造。' },
      { title: '⚡ 19. 极致性能与多级缓存', desc: 'Redis 语义缓存与向量缓存大幅节省 GPU 算力消耗。' },
      { title: '💰 20. 预算与成本精细化管控', desc: '按部门和用户精准控制 Token 消耗限额，透明可控。' },
    ],
    hwTitle: '💻 22. 硬件配置与容量规划推荐表',
    hwHint: '*基于 Local LLM (7B–32B)、RAG、Text-to-SQL、Agent 和报表生成等主流企业负载测算。',
    hwHeaders: ['用户规模', '预估并发数', '推荐 GPU 规格', '系统内存', 'NVMe 存储', 'AI 基础设施预估预算'],
    hwRows: [
      ['10 人', '2–4 并发', 'RTX 5090 (32GB)', '64 GB', '2 TB NVMe', '250k – 400k THB'],
      ['30 人', '4–8 并发', 'RTX 5090 (32GB)', '128 GB', '4 TB NVMe', '400k – 600k THB'],
      ['50 人', '8–12 并发', '2× RTX 5090 或 48GB', '128–192 GB', '4–8 TB NVMe', '600k – 1.0M THB'],
      ['100 人', '12–25 并发', '2× RTX 5090 / RTX 6000 Ada', '192–256 GB', '8 TB NVMe', '900k – 1.5M THB'],
      ['200 人', '25–50 并发', '2–4× 48–96GB Blackwell', '256–512 GB', '8–16 TB NVMe', '1.5M – 3.0M THB'],
      ['300 人+', '40–75+ 并发', '4× 48–96GB / 多节点集群', '512 GB+', '16 TB+ NVMe', '2.5M – 5.0M+ THB'],
    ],
    hwRecs: [
      { title: '💡 23. 循序渐进的投资策略', desc: '建议首期 10–30 人先以 Node 4 (单卡 GPU) 起步，运行 1–3 个月收集实际并发与 Token 吞吐数据后按需扩展。' },
      { title: '🔀 28. 3-Tier 多模型路由搭配策略', desc: '⚡ Qwen3 0.6B (极速/简单任务 ~0.5s)、📚 Qwen3 4B (通用/RAG 知识库)、🧠 Qwen3 8B (ERP/SQL/Agent 深度推理) 与智能 🤖 Auto 模式。' },
    ],
    sumTitle: '📊 31. 利益相关者核心价值总结',
    sumCards: [
      { icon: '🏢', title: '对企业', quote: '"建立私有企业 AI 资产，核心业务知识不随员工离职流失，数据 100% 安全自主。"' },
      { icon: '👨‍💼', title: '对员工', quote: '"拥有熟知公司业务的专属数字助理，免去繁杂的文档查阅与报表制作时间。"' },
      { icon: '📊', title: '对管理者', quote: '"用普通话/自然语言即时提问，秒级获取精准商业洞察与决策图表。"' },
      { icon: '👨‍💻', title: '对 IT 部门', quote: '"统一的 AI 治理中台，集中掌控用户、Token、模型、显卡算力与安全合规。"' },
    ],
    budgetTitle: '💼 推荐的企业 AI 四块预算结构',
    budgetItems: [
      { title: '1. AI 平台软件层', desc: '控制面、用户门户、AI 网关、RAG 引擎、Agent 框架与工具沙箱。' },
      { title: '2. AI 算力硬件设施', desc: 'GPU 加速卡、企业级服务器、高速内存与 NVMe 存储。' },
      { title: '3. 企业系统集成', desc: 'ERP 数据库对接、知识库数据管道与 RBAC 权限体系。' },
      { title: '4. 运维与生命周期管理', desc: '监控大屏、定期备份、Token 治理与模型版本迭代。' },
    ],
  },
  ja: {
    backBtn: '← 開発者ポータルに戻る',
    title: '🏛️ CHIOTRON エンタープライズ AI プラットフォーム設計図',
    subtitle: 'ERP + 社内ナレッジ + ドキュメント + データ + ワークフロー + ツールを統合するエンタープライズAIアーキテクチャ。完全なセキュリティとガバナンスを備えています。',
    tabs: {
      capabilities: '⚡ 04 コア機能',
      architecture: '🏛️ アーキテクチャ & AI層',
      features: '🧠 エンタープライズAI機能',
      security: '🛡️ セキュリティ & 安定性',
      hardware: '💻 ハードウェア & 拡張性',
      summary: '📊 価値 & エグゼクティブ要約',
    },
    capBadge: 'CORE CAPABILITIES',
    capHeroTitle: '⚡ エンタープライズ AI を支える 4 つの柱',
    capHeroDesc: 'CHIOTRON AI は単なるローカル LLM サーバーではありません。データ主権、権限制御、高可用性を保証する 4 本の柱からなるエンタープライズ基盤です。',
    cap1Title: 'AI Gateway & Ingress Core',
    cap1Desc: '全社統一のセキュアなエントリポイント。認証、暗号化、レート制限、データ保護ポリシーを一元管理します。',
    cap1Items: [
      '認証 & スコープ：API キー、JWT トークン、会社別スコープによる厳格なアクセス制御。',
      'レート制限 & 予算：ユーザーおよび部門ごとのリクエスト上限とトークン枠設定。',
      'データ漏洩防止 (DLP)：推論前に個人情報 (PII) をリアルタイムで検知・マスキング。',
      'ストリーミング出力：Server-Sent Events (SSE) による高速リアルタイム回答。',
    ],
    cap2Title: 'AI Orchestration & Agents',
    cap2Desc: '質問に応じて汎用 LLM、ナレッジ RAG、Text-to-SQL、業務ツールを自律的に判断・統合する頭脳です。',
    cap2Items: [
      'ReAct エージェント：思考・計画・実行を繰り返す自律型マルチステップ処理。',
      'ハイブリッド検索 (RAG)：HNSW ベクトル検索と全文検索を融合した高精度検索。',
      'Text-to-SQL：自然言語の質問を自動的に安全な SQL に変換し ERP を分析。',
      'MCP ツール連携：セキュアなサンドボックス環境下で社内 API を呼び出し。',
    ],
    cap3Title: 'Audit, Logging & Governance',
    cap3Desc: 'すべての AI 利用履歴を記録・可視化し、企業のコンプライアンスと透明性を確保します。',
    cap3Items: [
      'ターンごとの監査ログ：プロンプト、回答、使用モデル、所要時間、権限を完全記録。',
      'トークン利用分析：Prompt / Completion トークンの詳細とコストを正確に追跡。',
      'マルチテナント分離：会社・部門フィルターにより部署間の情報漏洩を完全防止。',
      '可観測性：Prometheus メトリクス、Grafana ダッシュボード、OpenTelemetry 連携。',
    ],
    cap4Title: 'インテント認識と3層モデルルーティング (Intent & 3-Tier Model Routing)',
    cap4Desc: '企業向け3層マルチモデルルーティングを備えたインテリジェントな意図分析およびモデルディスパッチエンジン。',
    cap4Items: [
      '3層ローカルGPUルーティング: ⚡ Qwen3 0.6B (高速/簡易 0.5s) ➔ 📚 Qwen3 4B (一般/RAG ナレッジ) ➔ 🧠 Qwen3 8B (ERP/SQL/Agent 思考推論)',
      'ポリシー優先アクセス制御: Gateway層でRBAC権限と部門分離を検証した後にモデルへ送信 (未許可クエリは事前遮断)',
      'ゼロ認知負荷 (Auto モード): ユーザーは自然に対話し、システムが最適なTool、RAG、モデル、GPUノードを自動選択',
      '動的コンピュート負荷分散: リクエストの自動分散とローカルGPU/クラウド間のシームレスなフェイルオーバー',
    ],
    archTitle: '🏛️ 1. CHIOTRON エンタープライズ AI 全体アーキテクチャ',
    archDesc: '社員は単一の AI と対話し、AI は裏側で各社員が閲覧権限を持つデータのみを取得して回答します。',
    archErpTitle: '🔗 14. 既存 ERP の上に重なる AI インテリジェンス層',
    archErpDesc: '既存の ERP システムを改修することなく、安全に AI 機能をアドオンして即日運用を開始できます。',
    archErpPills: [
      { title: 'ERP コード改修不要', desc: 'ERP API、読み取り専用 DB ビュー、MCP ツール経由で安全接続。' },
      { title: 'ERP 権限をそのまま継承', desc: '社員 A は自身に許可された会社のデータのみを閲覧可能。' },
      { title: 'レポート作成を自動化', desc: '売上推移や異常値分析を数秒でグラフ・表形式で即座に要約。' },
    ],
    archTechTitle: '🚀 15. 最新鋭のエンタープライズ技術スタック',
    archTechItems: [
      { title: 'ローカル LLM エンジン：', desc: 'オンプレミス GPU 稼働、データ外部送信ゼロで完全機密保持。' },
      { title: 'AI ゲートウェイ & ルーター：', desc: '全社アクセスの一元管理と負荷分散。' },
      { title: 'ハイブリッド検索 & GraphRAG：', desc: '意味検索と知識グラフによる多層的な情報理解。' },
      { title: 'Text-to-SQL サンドボックス：', desc: '安全な読み取り専用 DB クエリによるデータ抽出。' },
      { title: '分離された Compute Plane：', desc: 'Control Plane (VM4) と Compute Plane (VM5) の完全分離。' },
    ],
    featA: {
      badge: 'A. GENERAL AI',
      title: '✨ オールインワンのデジタル業務アシスタント',
      desc: '対話、要約、翻訳、アイデア出し、PDF/Word/Excel/CSV 解析、OCR 文字認識、コード生成、リアルタイム音声対話 AI。',
      example: '💬 「この Excel ファイルを要約し、前月比較グラフ付きの役員向けレポートを作って」',
    },
    featB: {
      badge: '3. KNOWLEDGE AI',
      title: '📚 社内ナレッジ検索 (Enterprise RAG)',
      desc: 'PDF、SOP、社内規程、NAS、DB から横断検索し、必ず出典元の引用 (Source Citations) を付けて回答。',
      example: '💬 「最新の SOP 手順書に基づき、この製品の品質検査手順を教えて」',
    },
    featC: {
      badge: '4. ERP AI',
      title: '💼 自然言語による ERP データ照会',
      desc: '売上、在庫、生産数、売掛金などを自然言語で質問可能。システムが最適な取得経路を自動判定。',
      example: '💬 「今月の売上は前月比でどうですか？在庫切れに近い品目はどれですか？」',
    },
    featD: {
      badge: '5. TEXT-TO-SQL',
      title: '📊 Text-to-SQL & ビジネスインテリジェンス',
      desc: '日本語の質問を安全な SQL に自動変換し、読み取り専用 DB から瞬時に表やグラフを生成。',
      example: '💬 「今年最も購入額の多い上位 10 社の顧客とその構成比を表示して」',
    },
    featE: {
      badge: '6. AGENTIC RAG',
      title: '🔍 複数システムにまたがる根本原因分析',
      desc: '「今週の生産量が減少した理由」などに対し、生産・在庫・停止時間・不良率・保守データを統合診断。',
    },
    featF: {
      badge: '7. GRAPHRAG',
      title: '🕸️ GraphRAG & ナレッジグラフ',
      desc: '顧客 ➔ 受注 ➔ 製品 ➔ 製造 ➔ ロット ➔ 設備 ➔ 品質 ➔ 不良率の網羅的な関係性を深く理解。',
    },
    featG: {
      badge: '8. AI AGENTS',
      title: '🤖 業務特化型 AI エージェント群',
      desc: '財務エージェント（売上・売掛金・キャッシュフロー）、人事エージェント（休暇規程・福利厚生）、生産エージェント（WIP・OEE・稼働率）。',
    },
    featH: {
      badge: '9. MCP TOOLS',
      title: '🛠️ Model Context Protocol & ツール連携',
      desc: '厳格な認可、レート制限、監査ログのもとで社内システムツールを安全に自律呼び出し。',
    },
    secTitle: '🔒 16. 多層エンタープライズ・セキュリティパイプライン',
    secSteps: [
      { num: '1', title: 'ユーザー認証', desc: '社内 SSO による身元確認' },
      { num: '2', title: 'JWT 権限付与', desc: 'ロールに応じたアクセススコープ' },
      { num: '3', title: 'AI ゲートウェイ', desc: 'DLP 機密情報マスキング' },
      { num: '4', title: '会社フィルター', desc: 'マルチテナント境界の隔離' },
      { num: '5', title: '読み取り専用 DB', desc: '安全なサンドボックス実行' },
    ],
    secPillars: [
      { title: '🛡️ 17. 障害の物理分離', desc: 'Control Plane と Compute Plane が独立しているため、GPU 障害時も ERP は正常稼働。' },
      { title: '🎯 18. ハルシネーション根絶 (Grounding)', desc: '社内実データに基づく回答を徹底し、出所不明な AI の作り話を防止。' },
      { title: '⚡ 19. 超高速 & キャッシュ', desc: 'Redis セマンティックキャッシュとベクトルキャッシュにより GPU 負荷を大幅削減。' },
      { title: '💰 20. コスト & 予算管理', desc: '部門別・個人別のトークン枠設定により、予算超過を確実に防止。' },
    ],
    hwTitle: '💻 22. ユーザー規模別ハードウェア構成ガイド',
    hwHint: '*Local LLM (7B–32B)、RAG、Text-to-SQL、Agent、レポート作成負荷を想定。',
    hwHeaders: ['ユーザー数', '想定同時接続数', '推奨 GPU', 'RAM', 'Storage NVMe', 'AI 基盤概算予算'],
    hwRows: [
      ['10 名', '2–4 名', 'RTX 5090 (32GB)', '64 GB', '2 TB NVMe', '250k – 400k THB'],
      ['30 名', '4–8 名', 'RTX 5090 (32GB)', '128 GB', '4 TB NVMe', '400k – 600k THB'],
      ['50 名', '8–12 名', '2× RTX 5090 / 48GB', '128–192 GB', '4–8 TB NVMe', '600k – 1.0M THB'],
      ['100 名', '12–25 名', '2× RTX 5090 / RTX 6000 Ada', '192–256 GB', '8 TB NVMe', '900k – 1.5M THB'],
      ['200 名', '25–50 名', '2–4× 48–96GB Blackwell', '256–512 GB', '8–16 TB NVMe', '1.5M – 3.0M THB'],
      ['300 名+', '40–75+ 名', '4× 48–96GB / 複数ノード', '512 GB+', '16 TB+ NVMe', '2.5M – 5.0M+ THB'],
    ],
    hwRecs: [
      { title: '💡 23. 段階的な投資アプローチ', desc: 'まずは 10〜30 名（Node 4 + GPU 1基）で開始し、1〜3 ヶ月間の実負荷を計測した上で拡張することを推奨。' },
      { title: '🔀 28. 3-Tier マルチモデルルーティング戦略', desc: '⚡ Qwen3 0.6B（高速/簡易タスク ~0.5s）、📚 Qwen3 4B（一般/RAG ナレッジ）、🧠 Qwen3 8B（ERP/SQL/Agent 思考推論）とスマートな 🤖 Auto モード。' },
    ],
    sumTitle: '📊 31. ステークホルダー別の提供価値要約',
    sumCards: [
      { icon: '🏢', title: '企業にとって', quote: '「社内ナレッジが属人化せず資産として残り、100% 安全に機密データを保護できる」' },
      { icon: '👨‍💼', title: '社員にとって', quote: '「自社の業務を熟知した専属アシスタントにより、情報探索と資料作成の時間を大幅削減」' },
      { icon: '📊', title: '経営陣にとって', quote: '「自然言語で即座に経営数値を問いかけ、意思決定に必要なインサイトを瞬時に獲得」' },
      { icon: '👨‍💻', title: 'IT部門にとって', quote: '「ユーザー、トークン、モデル、GPU、セキュリティを一元的に統制できる AI コックピット」' },
    ],
    budgetTitle: '💼 推奨される 4 つの AI 予算枠構造',
    budgetItems: [
      { title: '1. AI プラットフォームソフトウェア', desc: 'Control Plane、ユーザーポータル、AI Gateway、RAG エンジン、Agent サンドボックス。' },
      { title: '2. AI コンピュートインフラ', desc: 'GPU アクセラレータ、エンタープライズサーバー、大容量 RAM、NVMe ストレージ。' },
      { title: '3. 企業システム統合', desc: 'ERP データベース連携、社内ナレッジ取り込みパイプライン、RBAC 権限制御。' },
      { title: '4. 運用・保守・ガバナンス', desc: '監視ダッシュボード、定期バックアップ、トークン枠管理、モデルバージョン更新。' },
    ],
  },
  my: {
    backBtn: '← Developer Portal သို့ ပြန်သွားရန်',
    title: '🏛️ CHIOTRON Enterprise AI Platform ပုံစံကြမ်း (Blueprint)',
    subtitle: 'ERP + ကုမ္ပဏီအသိပညာ + စာရွက်စာတမ်းများ + Data + Workflow + Tools များကို ချိတ်ဆက်ထားသော Enterprise AI ဗိသုကာစနစ်။',
    tabs: {
      capabilities: '⚡ အဓိကစွမ်းဆောင်ရည် ၄ ရပ်',
      architecture: '🏛️ ဗိသုကာနှင့် AI အလွှာ',
      features: '🧠 လုပ်ငန်းသုံး AI လုပ်ဆောင်ချက်များ',
      security: '🛡️ လုံခြုံရေးနှင့် တည်ငြိမ်မှု',
      hardware: '💻 Hardware နှင့် စနစ်တိုးချဲ့မှု',
      summary: '📊 တန်ဖိုးနှင့် အမှုဆောင်အနှစ်ချုပ်',
    },
    capBadge: 'CORE CAPABILITIES',
    capHeroTitle: '⚡ Enterprise AI Platform ကို မောင်းနှင်သည့် အဓိကစွမ်းဆောင်ရည် ၄ ရပ်',
    capHeroDesc: 'CHIOTRON AI သည် ရိုးရိုး Local LLM Server မဟုတ်ဘဲ လုပ်ငန်းလုံခြုံရေး၊ ခွင့်ပြုချက်ထိန်းချုပ်မှုနှင့် စနစ်တိုးချဲ့မှုကို အပြည့်အဝထောက်ပံ့ပေးသော စနစ်ဖြစ်သည်။',
    cap1Title: 'AI Gateway & Ingress Core',
    cap1Desc: 'လုပ်ငန်းတစ်ခုလုံးအတွက် လုံခြုံသော ဝင်ပေါက်တစ်ခုဖြစ်ပြီး Authentication၊ Encryption၊ Rate Limit နှင့် လုံခြုံရေးမူဝါဒများကို ထိန်းချုပ်သည်။',
    cap1Items: [
      'Authentication & Scope: API Key၊ JWT Tokens နှင့် Company Scopes များဖြင့် စစ်ဆေးခွင့်ပြုခြင်း။',
      'Rate Limiting & Quotas: အသုံးပြုသူနှင့် ဌာနအလိုက် Token ဘတ်ဂျက်နှင့် အသုံးပြုမှုကန့်သတ်ချက်များ သတ်မှတ်ခြင်း။',
      'Data Loss Prevention (DLP): သီးသန့်အချက်အလက်များ (PII) မပေါက်ကြားစေရန် အချိန်နှင့်တပြေးညီ ဖျက်ထုတ်စစ်ဆေးခြင်း။',
      'Streaming & SSE Engine: Server-Sent Events ဖြင့် စာသားများကို ချက်ချင်း မြန်ဆန်စွာ ပြသပေးခြင်း။',
    ],
    cap2Title: 'AI Orchestration & Agents',
    cap2Desc: 'မေးခွန်းများကို ခွဲခြမ်းစိတ်ဖြာပြီး LLM၊ RAG၊ Text-to-SQL သို့မဟုတ် Agent Tools များဆီသို့ အလိုအလျောက် ပို့ဆောင်ပေးသည့် ဉာဏ်ရည်စနစ်။',
    cap2Items: [
      'ReAct Agent Loop: အဆင့်ဆင့် စဉ်းစားတွေးခေါ်ပြီး လုပ်ငန်းဆောင်တာများကို ပြီးမြောက်အောင် လုပ်ဆောင်ခြင်း။',
      'Hybrid Search & RAG: HNSW Vector ရှာဖွေမှုနှင့် Full-Text Search ကို ပေါင်းစပ်၍ တိကျစွာ ရှာဖွေခြင်း။',
      'Text-to-SQL Engine: သာမန်မေးခွန်းများကို SQL အဖြစ် ပြောင်းလဲပြီး ERP ဒေတာများကို ခွဲခြမ်းစိတ်ဖြာခြင်း။',
      'MCP & Tool Calling: လုံခြုံသော Sandbox အောက်တွင် ကုမ္ပဏီ၏ API များနှင့် ချိတ်ဆက်လုပ်ဆောင်ခြင်း။',
    ],
    cap3Title: 'Audit, Logging & Governance',
    cap3Desc: 'AI အသုံးပြုမှုမှတ်တမ်းအားလုံးကို ပွင့်လင်းမြင်သာစွာ မှတ်တမ်းတင်စစ်ဆေးနိုင်သော စနစ်။',
    cap3Items: [
      'Turn-by-turn Auditing: မေးခွန်း၊ အဖြေ၊ အသုံးပြုသော Model၊ ခွင့်ပြုချက်နှင့် အချိန်တို့ကို အပြည့်အစုံ မှတ်တမ်းတင်ခြင်း။',
      'Token Usage Analytics: Prompt နှင့် Completion Token အသုံးပြုမှုနှင့် ကုန်ကျစရိတ်များကို တိကျစွာ တွက်ချက်ခြင်း။',
      'Multi-Tenant Isolation: Company Filter ဖြင့် ဌာနအချင်းချင်း ဒေတာမရောနှောစေရန် တင်းကြပ်စွာ ကာကွယ်ခြင်း။',
      'Observability: Prometheus Metrics၊ Grafana Dashboards နှင့် OpenTelemetry ဖြင့် စောင့်ကြည့်ခြင်း။',
    ],
    cap4Title: 'Intent & 3-Tier Model Routing',
    cap4Desc: 'လုပ်ငန်းသုံး 3-Tier Multi-Model Routing ပါဝင်သော စမတ်ရည်ရွယ်ချက် ခွဲခြမ်းစိတ်ဖြာမှုနှင့် Model Dispatcher စနစ်။',
    cap4Items: [
      '3-Tier Local GPU Routing: ⚡ Qwen3 0.6B (ရိုးရှင်း/အမြန် 0.5s) ➔ 📚 Qwen3 4B (အထွေထွေ/RAG နည်းပညာ) ➔ 🧠 Qwen3 8B (ERP/SQL/Agent တွေးခေါ်မှု)',
      'Policy-First Authorization: Model သို့မပို့မီ Gateway တွင် RBAC နှင့် ဌာနခွင့်ပြုချက်များကို စစ်ဆေးပါသည် (ခွင့်မရှိပါက ချက်ချင်းပိတ်ပင်သည်)',
      'Zero Cognitive Load (Auto Mode): အသုံးပြုသူသည် ပုံမှန်အတိုင်း မေးမြန်းနိုင်ပြီး စနစ်မှ Tool၊ RAG နှင့် Model ကို အလိုအလျောက် ရွေးချယ်ပေးပါသည်',
      'Dynamic Compute Balancing: Local GPU နှင့် Cloud Adapters များအကြား အလိုအလျောက် ချိန်ညှိမှု စနစ်',
    ],
    archTitle: '🏛️ ၁။ CHIOTRON Enterprise AI Platform ဗိသုကာအကျဉ်းချုပ်',
    archDesc: 'ဝန်ထမ်းများသည် AI တစ်ခုတည်းနှင့်သာ စကားပြောရသော်လည်း AI သည် ၎င်းဝန်ထမ်းကြည့်ရှုခွင့်ရှိသော ဒေတာများကိုသာ ရယူဖြေကြားပေးသည်။',
    archErpTitle: '🔗 ၁၄။ ERP အပေါ်ထပ်ရှိ AI Intelligence Layer (စနစ်မထိခိုက်ဘဲ ချိတ်ဆက်ခြင်း)',
    archErpDesc: 'လက်ရှိ ERP စနစ်ကို ပြင်ဆင်ရန်မလိုဘဲ အပေါ်မှ AI Layer အဖြစ် ချိတ်ဆက်ကာ အစွမ်းထက်သော AI စွမ်းရည်များကို ချက်ချင်း အသုံးပြုနိုင်သည်။',
    archErpPills: [
      { title: 'ERP စနစ်ပြင်ရန်မလို', desc: 'ERP API၊ Read-Only DB View သို့မဟုတ် MCP Tools များဖြင့် ချိတ်ဆက်သည်။' },
      { title: 'ERP ခွင့်ပြုချက်အတိုင်းရရှိ', desc: 'ဝန်ထမ်း A သည် ခွင့်ပြုထားသော သက်ဆိုင်ရာ ကုမ္ပဏီဒေတာကိုသာ မြင်တွေ့ရသည်။' },
      { title: 'အချိန်ကုန်သက်သာ', desc: 'အရောင်းစာရင်းနှင့် အစီရင်ခံစာများကို စက္ကန့်ပိုင်းအတွင်း အကျဉ်းချုပ် ရယူနိုင်သည်။' },
    ],
    archTechTitle: '🚀 ၁၅။ ခေတ်မီ Enterprise နည်းပညာများ',
    archTechItems: [
      { title: 'Local LLM Engine:', desc: 'On-Premise GPU ပေါ်တွင် လည်ပတ်ပြီး ဒေတာလုံခြုံမှု ၁၀၀% အပြည့်ရှိသည်။' },
      { title: 'AI Gateway & Router:', desc: 'ဗဟိုမှ လုံခြုံရေးနှင့် မော်ဒယ်လမ်းကြောင်းများကို ထိန်းချုပ်သည်။' },
      { title: 'Hybrid Search & GraphRAG:', desc: 'စာရွက်စာတမ်းများနှင့် ဒေတာဆက်စပ်မှုများကို နက်ရှိုင်းစွာ နားလည်သည်။' },
      { title: 'Text-to-SQL & Tool Sandbox:', desc: 'Read-only ဒေတာဘေ့စ်ကို သာမန်ဘာသာစကားဖြင့် မေးမြန်းရှာဖွေနိုင်သည်။' },
      { title: 'Isolated Compute Plane:', desc: 'Control Plane (VM4) နှင့် Compute Plane (VM5) ကို သီးခြားစီ ခွဲခြားထားသည်။' },
    ],
    featA: {
      badge: 'A. GENERAL AI',
      title: '✨ ဘက်စုံသုံး ဒစ်ဂျစ်တယ်လက်ထောက်',
      desc: 'စကားပြောခြင်း၊ မေးမြန်းခြင်း၊ အကျဉ်းချုပ်ခြင်း၊ ဘာသာပြန်ခြင်း၊ PDF/Excel/CSV ဖိုင်များကို ခွဲခြမ်းစိတ်ဖြာခြင်း၊ OCR ဖြင့် စာသားထုတ်ယူခြင်းနှင့် အသံဖြင့် တိုက်ရိုက်ပြောဆိုခြင်း။',
      example: '💬 "ဒီ Excel ဖိုင်ကို အကျဉ်းချုပ်ပြီး ပြီးခဲ့တဲ့လနဲ့ နှိုင်းယှဉ်ချက် ဂရပ်ဖ် ပြုလုပ်ပေးပါ"',
    },
    featB: {
      badge: '3. KNOWLEDGE AI',
      title: '📚 ကုမ္ပဏီတွင်း အသိပညာရှာဖွေခြင်း (Enterprise RAG)',
      desc: 'PDF၊ SOP စည်းမျဉ်းများ၊ မူဝါဒများနှင့် စာရွက်စာတမ်းများကို မူရင်းလင့်ခ် (Source Citations) နှင့်တကွ တိကျစွာ ရှာဖွေဖြေကြားပေးသည်။',
      example: '💬 "နောက်ဆုံးထုတ် SOP စည်းမျဉ်းအရ ဒီပစ္စည်းရဲ့ အရည်အသွေးစစ်ဆေးနည်းက ဘာလဲ?"',
    },
    featC: {
      badge: '4. ERP AI',
      title: '💼 ERP ဒေတာများကို သာမန်ဘာသာစကားဖြင့် မေးမြန်းခြင်း',
      desc: 'အရောင်းစာရင်း၊ ကုန်ပစ္စည်းလက်ကျန်၊ ထုတ်လုပ်မှုအရေအတွက်နှင့် ကြွေးကျန်များကို သာမန်ဘာသာစကားဖြင့် မေးမြန်းနိုင်သည်။',
      example: '💬 "ဒီလ အရောင်းက ပြီးခဲ့တဲ့လနဲ့ ယှဉ်ရင် ဘယ်လိုရှိလဲ? ဘယ်ပစ္စည်းတွေ လက်ကျန်နည်းနေလဲ?"',
    },
    featD: {
      badge: '5. TEXT-TO-SQL',
      title: '📊 Text-to-SQL နှင့် စီးပွားရေးအချက်အလက် (BI)',
      desc: 'မေးခွန်းများကို SQL အဖြစ် အလိုအလျောက် ပြောင်းလဲကာ ဇယားနှင့် ဂရပ်ဖ်များဖြင့် ချက်ချင်း ဖော်ပြပေးသည်။',
      example: '💬 "ယခုနှစ်အတွင်း အရောင်းအများဆုံး ဝယ်သူ ၁၀ ဦးနှင့် ရာခိုင်နှုန်းကို ပြပါ"',
    },
    featE: {
      badge: '6. AGENTIC RAG',
      title: '🔍 စနစ်ပေါင်းစုံ အကြောင်းရင်းရှာဖွေခြင်း',
      desc: 'ထုတ်လုပ်မှု ကျဆင်းရသည့် အကြောင်းရင်းကို ထုတ်လုပ်မှု၊ ကုန်ပစ္စည်းလက်ကျန်၊ စက်ရပ်ချိန်နှင့် ပြုပြင်ထိန်းသိမ်းမှု ဒေတာများ ပေါင်းစပ်ခွဲခြမ်းစိတ်ဖြာသည်။',
    },
    featF: {
      badge: '7. GRAPHRAG',
      title: '🕸️ GraphRAG နှင့် Knowledge Graph',
      desc: 'ဝယ်သူ ➔ အော်ဒါ ➔ ကုန်ပစ္စည်း ➔ ထုတ်လုပ်မှု ➔ အသုတ် ➔ စက်ပစ္စည်း ➔ အရည်အသွေး ဆက်စပ်မှုများကို နားလည်သည်။',
    },
    featG: {
      badge: '8. AI AGENTS',
      title: '🤖 ဌာနအလိုက် အထူးပြု AI Agent များ',
      desc: 'ဘဏ္ဍာရေး Agent (အရောင်း၊ ကြွေးကျန်၊ ငွေစီးဆင်းမှု)၊ လူ့စွမ်းအား Agent (ခွင့်ရက်၊ ခံစားခွင့်)၊ ထုတ်လုပ်မှု Agent (WIP၊ OEE စက်စွမ်းရည်)။',
    },
    featH: {
      badge: '9. MCP TOOLS',
      title: '🛠️ Model Context Protocol & Tool Sandbox',
      desc: 'တင်းကြပ်သော ခွင့်ပြုချက်နှင့် စောင့်ကြည့်မှုအောက်တွင် ကုမ္ပဏီသုံး ကိရိယာများကို AI မှ လုံခြုံစွာ ခေါ်ယူအသုံးပြုနိုင်သည်။',
    },
    secTitle: '🔒 ၁၆။ အဆင့်ဆင့် လုံခြုံရေးစနစ် (Multi-Layer Security)',
    secSteps: [
      { num: '၁', title: 'User Identity', desc: 'ကုမ္ပဏီအကောင့်ဖြင့် စစ်ဆေးခြင်း' },
      { num: '၂', title: 'JWT & Scopes', desc: 'ရာထူးအလိုက် ခွင့်ပြုချက် သတ်မှတ်ခြင်း' },
      { num: '၃', title: 'AI Gateway', desc: 'DLP ဖြင့် လျှို့ဝှက်ချက်များ စစ်ထုတ်ခြင်း' },
      { num: '၄', title: 'Company Filter', desc: 'ကုမ္ပဏီအလိုက် ဒေတာပိုင်းခြားခြင်း' },
      { num: '၅', title: 'Read-Only DB', desc: 'လုံခြုံသော Sandbox အောက်တွင် အလုပ်လုပ်ခြင်း' },
    ],
    secPillars: [
      { title: '🛡️ ၁၇။ တည်ငြိမ်မှု (Fault Isolation)', desc: 'Control Plane နှင့် Compute Plane သီးခြားစီဖြစ်၍ GPU ပြဿနာဖြစ်သော်လည်း ERP ကို မထိခိုက်ပါ။' },
      { title: '🎯 ၁၈။ တိကျမှန်ကန်မှု (Grounding)', desc: 'AI မှ စိတ်ကူးယဉ်ဖြေကြားခြင်းကို တားဆီးပြီး ERP ဒေတာအစစ်အမှန်ပေါ် အခြေခံဖြေကြားသည်။' },
      { title: '⚡ ၁၉။ မြန်ဆန်မှုနှင့် Caching', desc: 'Redis Caching ဖြင့် မကြာခဏမေးသော မေးခွန်းများကို GPU အားမကုန်ဘဲ ချက်ချင်း ဖြေကြားသည်။' },
      { title: '💰 ၂၀။ ကုန်ကျစရိတ် ထိန်းချုပ်မှု', desc: 'ဌာနအလိုက် Token သတ်မှတ်ချက်များနှင့် ဘတ်ဂျက်ကို တိကျစွာ ထိန်းချုပ်နိုင်သည်။' },
    ],
    hwTitle: '💻 ၂၂။ အသုံးပြုသူအရေအတွက်အလိုက် Hardware အကြံပြုချက်ဇယား',
    hwHint: '*Local LLM (7B–32B), RAG, Text-to-SQL နှင့် အစီရင်ခံစာထုတ်လုပ်မှု အခြေခံတွက်ချက်ထားသည်။',
    hwHeaders: ['အသုံးပြုသူ', 'တပြိုင်နက်အသုံးပြုမှု', 'အကြံပြု GPU', 'RAM', 'Storage NVMe', 'ခန့်မှန်းခြေ AI ဘတ်ဂျက်'],
    hwRows: [
      ['၁၀ ဦး', '၂–၄ ဦး', 'RTX 5090 (32GB)', '64 GB', '2 TB NVMe', '250k – 400k THB'],
      ['၃၀ ဦး', '၄–၈ ဦး', 'RTX 5090 (32GB)', '128 GB', '4 TB NVMe', '400k – 600k THB'],
      ['၅၀ ဦး', '၈–၁၂ ဦး', '2× RTX 5090 သို့မဟုတ် 48GB', '128–192 GB', '4–8 TB NVMe', '600k – 1.0M THB'],
      ['၁၀၀ ဦး', '၁၂–၂၅ ဦး', '2× RTX 5090 / RTX 6000 Ada', '192–256 GB', '8 TB NVMe', '900k – 1.5M THB'],
      ['၂၀၀ ဦး', '၂၅–၅၀ ဦး', '2–4× 48–96GB Blackwell', '256–512 GB', '8–16 TB NVMe', '1.5M – 3.0M THB'],
      ['၃၀၀ ဦး+', '၄၀–၇၅+ ဦး', '4× 48–96GB / Multi-Node', '512 GB+', '16 TB+ NVMe', '2.5M – 5.0M+ THB'],
    ],
    hwRecs: [
      { title: '💡 ၂၃။ အကျိုးရှိသော ရင်းနှီးမြှုပ်နှံမှု အကြံပြုချက်', desc: '၁၀–၃၀ ဦးဖြင့် Node 4 (GPU ၁ ခု) စတင်အသုံးပြုပြီး ၁–၃ လ အမှန်တကယ် အသုံးပြုမှု တိုင်းတာပြီးမှ စနစ်တိုးချဲ့ရန် အကြံပြုသည်။' },
      { title: '🔀 ၂၈။ 3-Tier Multi-Model Routing ဗျူဟာ', desc: '⚡ Qwen3 0.6B (ရိုးရှင်း/အမြန် ~0.5s)၊ 📚 Qwen3 4B (အထွေထွေ/RAG ဒေတာ)၊ 🧠 Qwen3 8B (ERP/SQL/Agent တွေးခေါ်မှု) နှင့် စမတ် 🤖 Auto Mode။' },
    ],
    sumTitle: '📊 ၃၁။ ကဏ္ဍအလိုက် ရရှိမည့် အဓိကတန်ဖိုးများ',
    sumCards: [
      { icon: '🏢', title: 'ကုမ္ပဏီအတွက်', quote: '"ကုမ္ပဏီပိုင် AI စနစ်ရှိလာပြီး ဝန်ထမ်းထွက်သွားသော်လည်း အသိပညာများ မပျောက်ပျက်ဘဲ ဒေတာလုံခြုံမှု အပြည့်ရှိသည်။"' },
      { icon: '👨‍💼', title: 'ဝန်ထမ်းများအတွက်', quote: '"ကုမ္ပဏီအလုပ်ကို နားလည်သော ကိုယ်ပိုင် AI လက်ထောက်ရှိလာပြီး အစီရင်ခံစာလုပ်ရသည့် အချိန်များ သက်သာစေသည်။"' },
      { icon: '📊', title: 'အမှုဆောင်များအတွက်', quote: '"သာမန်ဘာသာစကားဖြင့် မေးမြန်းရုံဖြင့် လုပ်ငန်းအချက်အလက်နှင့် ဆုံးဖြတ်ချက်ဆိုင်ရာ ဂရပ်ဖ်များကို ချက်ချင်း ရရှိနိုင်သည်။"' },
      { icon: '👨‍💻', title: 'IT ဌာနအတွက်', quote: '"User၊ Token၊ Model၊ GPU နှင့် လုံခြုံရေးအားလုံးကို တစ်နေရာတည်းမှ ဗဟိုချုပ်ကိုင် ထိန်းချုပ်နိုင်သည်။"' },
    ],
    budgetTitle: '💼 အကြံပြုထားသော AI ဘတ်ဂျက် ၄ ပိုင်း ဖွဲ့စည်းပုံ',
    budgetItems: [
      { title: '၁။ AI Platform Software', desc: 'Control Plane, User Portal, AI Gateway, RAG Engine, Agent Framework, Tool Sandbox.' },
      { title: '၂။ AI Compute Infrastructure', desc: 'GPU Acceleration, Enterprise Server, RAM, High-Speed NVMe Storage.' },
      { title: '၃။ Enterprise Integration', desc: 'ERP ဒေတာဘေ့စ် ချိတ်ဆက်မှု၊ အသိပညာဒေတာ စုဆောင်းမှုနှင့် RBAC လုံခြုံရေးစနစ်။' },
      { title: '၄။ Operations & Maintenance', desc: 'စောင့်ကြည့်စစ်ဆေးမှု၊ Backup၊ Token ဘတ်ဂျက် ထိန်းချုပ်မှုနှင့် Model မွမ်းမံခြင်း။' },
    ],
  },
};

export function EnterpriseBlueprint({ onBack }: { onBack: () => void }) {
  const { language } = useTranslation();
  const [activeTab, setActiveTab] = useState<'capabilities' | 'architecture' | 'features' | 'security' | 'hardware' | 'summary'>('capabilities');

  const loc: BlueprintStrings = BLUEPRINT_I18N[language] ?? BLUEPRINT_I18N.th;

  return (
    <div className="enterprise-blueprint-view">
      {/* Header & Breadcrumb */}
      <div className="blueprint-header">
        <div>
          <button className="text-button" onClick={onBack} style={{ marginBottom: '8px' }}>
            {loc.backBtn}
          </button>
          <h1 className="blueprint-title">
            {loc.title}
          </h1>
          <p className="blueprint-subtitle">
            {loc.subtitle}
          </p>
        </div>
      </div>

      {/* Interactive Navigation Tabs */}
      <div className="blueprint-tabs">
        <button
          className={`blueprint-tab-btn ${activeTab === 'capabilities' ? 'active' : ''}`}
          onClick={() => setActiveTab('capabilities')}
        >
          {loc.tabs.capabilities}
        </button>
        <button
          className={`blueprint-tab-btn ${activeTab === 'architecture' ? 'active' : ''}`}
          onClick={() => setActiveTab('architecture')}
        >
          {loc.tabs.architecture}
        </button>
        <button
          className={`blueprint-tab-btn ${activeTab === 'features' ? 'active' : ''}`}
          onClick={() => setActiveTab('features')}
        >
          {loc.tabs.features}
        </button>
        <button
          className={`blueprint-tab-btn ${activeTab === 'security' ? 'active' : ''}`}
          onClick={() => setActiveTab('security')}
        >
          {loc.tabs.security}
        </button>
        <button
          className={`blueprint-tab-btn ${activeTab === 'hardware' ? 'active' : ''}`}
          onClick={() => setActiveTab('hardware')}
        >
          {loc.tabs.hardware}
        </button>
        <button
          className={`blueprint-tab-btn ${activeTab === 'summary' ? 'active' : ''}`}
          onClick={() => setActiveTab('summary')}
        >
          {loc.tabs.summary}
        </button>
      </div>

      {/* Tab Content Panels */}
      <div className="blueprint-content">
        {/* TAB 1: 04 CORE CAPABILITIES */}
        {activeTab === 'capabilities' && (
          <div className="blueprint-section">
            <div className="blueprint-hero-card">
              <span className="badge-core">{loc.capBadge}</span>
              <h2>{loc.capHeroTitle}</h2>
              <p>{loc.capHeroDesc}</p>
            </div>

            <div className="capabilities-four-grid">
              {/* 1. Gateway */}
              <div className="cap-card cap-gateway">
                <div className="cap-card-header">
                  <span className="cap-icon">🚪</span>
                  <div>
                    <span className="cap-num">01 / CAPABILITY</span>
                    <h3>{loc.cap1Title}</h3>
                  </div>
                </div>
                <p className="cap-desc">{loc.cap1Desc}</p>
                <ul className="cap-list">
                  {loc.cap1Items.map((item, idx) => (
                    <li key={idx}><b>{item.split(':')[0]}:</b>{item.split(':')[1]}</li>
                  ))}
                </ul>
              </div>

              {/* 2. Orchestration */}
              <div className="cap-card cap-orch">
                <div className="cap-card-header">
                  <span className="cap-icon">🎼</span>
                  <div>
                    <span className="cap-num">02 / CAPABILITY</span>
                    <h3>{loc.cap2Title}</h3>
                  </div>
                </div>
                <p className="cap-desc">{loc.cap2Desc}</p>
                <ul className="cap-list">
                  {loc.cap2Items.map((item, idx) => (
                    <li key={idx}><b>{item.split(':')[0]}:</b>{item.split(':')[1]}</li>
                  ))}
                </ul>
              </div>

              {/* 3. Audit */}
              <div className="cap-card cap-audit">
                <div className="cap-card-header">
                  <span className="cap-icon">📋</span>
                  <div>
                    <span className="cap-num">03 / CAPABILITY</span>
                    <h3>{loc.cap3Title}</h3>
                  </div>
                </div>
                <p className="cap-desc">{loc.cap3Desc}</p>
                <ul className="cap-list">
                  {loc.cap3Items.map((item, idx) => (
                    <li key={idx}><b>{item.split(':')[0]}:</b>{item.split(':')[1]}</li>
                  ))}
                </ul>
              </div>

              {/* 4. Model Provider Routing */}
              <div className="cap-card cap-routing">
                <div className="cap-card-header">
                  <span className="cap-icon">🔀</span>
                  <div>
                    <span className="cap-num">04 / CAPABILITY</span>
                    <h3>{loc.cap4Title}</h3>
                  </div>
                </div>
                <p className="cap-desc">{loc.cap4Desc}</p>
                <ul className="cap-list">
                  {loc.cap4Items.map((item, idx) => (
                    <li key={idx}><b>{item.split(':')[0]}:</b>{item.split(':')[1]}</li>
                  ))}
                </ul>
              </div>
            </div>
          </div>
        )}

        {/* TAB 2: ARCHITECTURE & AI LAYER */}
        {activeTab === 'architecture' && (
          <div className="blueprint-section">
            <div className="blueprint-card">
              <h2>{loc.archTitle}</h2>
              <p>{loc.archDesc}</p>
              
              <div className="ascii-architecture-box">
                <pre>{`
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                                CHIOTRON ENTERPRISE AI                                  │
├────────────────────────────┬────────────────────────────┬───────────────────────────────┤
│         GENERAL AI         │       ENTERPRISE AI        │           AI AGENTS           │
│   Chat / Create / Vision   │     ERP / Data / RAG       │      Automation / Workflows   │
├────────────────────────────┴────────────────────────────┴───────────────────────────────┤
│                                     AI ORCHESTRATOR                                     │
│            ┌──────────────────────────────┬──────────────────────────────┐              │
│            ↓                              ↓                              ↓              │
│       LLM Engine                   RAG / GraphRAG                   Text-to-SQL         │
├─────────────────────────────────────────────────────────────────────────────────────────┤
│                                  ENTERPRISE CONNECTORS                                  │
│            ┌──────────────────────────────┼──────────────────────────────┐              │
│            ↓                              ↓                              ↓              │
│        ERP System                 Company Knowledge                Tools / MCP Sandbox  │
└─────────────────────────────────────────────────────────────────────────────────────────┘
                                            ↓
                                  🏢 ENTERPRISE DATA
                (PostgreSQL + pgvector / Redis / NAS Documents / ERP Database)
                `}</pre>
              </div>
            </div>

            <div className="blueprint-card" style={{ marginTop: '20px' }}>
              <h2>{loc.archErpTitle}</h2>
              <p>{loc.archErpDesc}</p>
              <div className="feature-pill-grid">
                {loc.archErpPills.map((pill, idx) => (
                  <div className="feature-pill-card" key={idx}>
                    <b>{pill.title}</b>
                    <span>{pill.desc}</span>
                  </div>
                ))}
              </div>
            </div>

            <div className="blueprint-card" style={{ marginTop: '20px' }}>
              <h2>{loc.archTechTitle}</h2>
              <ul className="modern-tech-list">
                {loc.archTechItems.map((item, idx) => (
                  <li key={idx}><b>{item.title}</b> {item.desc}</li>
                ))}
              </ul>
            </div>
          </div>
        )}

        {/* TAB 3: ENTERPRISE FEATURES */}
        {activeTab === 'features' && (
          <div className="blueprint-section">
            <div className="features-showcase-grid">
              {/* Feature A */}
              <div className="feature-showcase-card">
                <div className="fsc-badge">{loc.featA.badge}</div>
                <h3>{loc.featA.title}</h3>
                <p>{loc.featA.desc}</p>
                <div className="example-bubble">{loc.featA.example}</div>
              </div>

              {/* Feature B */}
              <div className="feature-showcase-card">
                <div className="fsc-badge">{loc.featB.badge}</div>
                <h3>{loc.featB.title}</h3>
                <p>{loc.featB.desc}</p>
                <div className="example-bubble">{loc.featB.example}</div>
              </div>

              {/* Feature C */}
              <div className="feature-showcase-card">
                <div className="fsc-badge">{loc.featC.badge}</div>
                <h3>{loc.featC.title}</h3>
                <p>{loc.featC.desc}</p>
                <div className="example-bubble">{loc.featC.example}</div>
              </div>

              {/* Feature D */}
              <div className="feature-showcase-card">
                <div className="fsc-badge">{loc.featD.badge}</div>
                <h3>{loc.featD.title}</h3>
                <p>{loc.featD.desc}</p>
                <div className="example-bubble">{loc.featD.example}</div>
              </div>

              {/* Feature E */}
              <div className="feature-showcase-card">
                <div className="fsc-badge">{loc.featE.badge}</div>
                <h3>{loc.featE.title}</h3>
                <p>{loc.featE.desc}</p>
              </div>

              {/* Feature F */}
              <div className="feature-showcase-card">
                <div className="fsc-badge">{loc.featF.badge}</div>
                <h3>{loc.featF.title}</h3>
                <p>{loc.featF.desc}</p>
              </div>

              {/* Feature G */}
              <div className="feature-showcase-card">
                <div className="fsc-badge">{loc.featG.badge}</div>
                <h3>{loc.featG.title}</h3>
                <p>{loc.featG.desc}</p>
              </div>

              {/* Feature H */}
              <div className="feature-showcase-card">
                <div className="fsc-badge">{loc.featH.badge}</div>
                <h3>{loc.featH.title}</h3>
                <p>{loc.featH.desc}</p>
              </div>
            </div>
          </div>
        )}

        {/* TAB 4: SECURITY & STABILITY */}
        {activeTab === 'security' && (
          <div className="blueprint-section">
            <div className="blueprint-card">
              <h2>{loc.secTitle}</h2>
              <div className="security-pipeline-flow">
                {loc.secSteps.map((step, idx) => (
                  <div key={idx} style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <div className="sec-step">
                      <span className="step-num">{step.num}</span>
                      <b>{step.title}</b>
                      <p>{step.desc}</p>
                    </div>
                    {idx < loc.secSteps.length - 1 && <span className="sec-arrow">➔</span>}
                  </div>
                ))}
              </div>
            </div>

            <div className="capabilities-four-grid" style={{ marginTop: '20px' }}>
              {loc.secPillars.map((pillar, idx) => (
                <div className="blueprint-card" key={idx}>
                  <h3>{pillar.title}</h3>
                  <p>{pillar.desc}</p>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* TAB 5: HARDWARE & SIZING */}
        {activeTab === 'hardware' && (
          <div className="blueprint-section">
            <div className="blueprint-card">
              <h2>{loc.hwTitle}</h2>
              <p className="history-hint">{loc.hwHint}</p>

              <div className="table-wrap" style={{ marginTop: '14px' }}>
                <table className="data-table">
                  <thead>
                    <tr>
                      {loc.hwHeaders.map((header, idx) => (
                        <th key={idx}>{header}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {loc.hwRows.map((row, rIdx) => (
                      <tr key={rIdx}>
                        <td><b>{row[0]}</b></td>
                        <td>{row[1]}</td>
                        <td>{row[2]}</td>
                        <td>{row[3]}</td>
                        <td>{row[4]}</td>
                        <td>{row[5]}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>

            <div className="capabilities-four-grid" style={{ marginTop: '20px' }}>
              {loc.hwRecs.map((rec, idx) => (
                <div className="blueprint-card" key={idx}>
                  <h3>{rec.title}</h3>
                  <p>{rec.desc}</p>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* TAB 6: EXECUTIVE SUMMARY */}
        {activeTab === 'summary' && (
          <div className="blueprint-section">
            <div className="blueprint-hero-card">
              <span className="badge-core">EXECUTIVE SUMMARY</span>
              <h2>{loc.sumTitle}</h2>
            </div>

            <div className="summary-cards-grid">
              {loc.sumCards.map((card, idx) => (
                <div className="summary-card" key={idx}>
                  <span className="summary-icon">{card.icon}</span>
                  <h4>{card.title}</h4>
                  <p className="summary-quote">{card.quote}</p>
                </div>
              ))}
            </div>

            <div className="blueprint-card" style={{ marginTop: '20px' }}>
              <h2>{loc.budgetTitle}</h2>
              <div className="budget-four-grid">
                {loc.budgetItems.map((bItem, idx) => (
                  <div className="budget-item" key={idx}>
                    <b>{bItem.title}</b>
                    <p>{bItem.desc}</p>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
