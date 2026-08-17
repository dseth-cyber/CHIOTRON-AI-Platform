// User-visible strings and locale-aware formatting.
//
// ARCHITECTURE-v1 section 10 requires every user-visible string to go through
// t(key) for Thai, English, Chinese, Burmese and Japanese, and dates to be
// formatted with the active language.
//
// The catalogue is typed from the English keys, so `tsc` fails the build if any
// locale is missing a string. That is deliberate: a silent fallback to English
// in one language is the failure mode this design exists to prevent.

export const LANGUAGES = ['en', 'th', 'zh', 'ja', 'my'] as const;
export type Language = (typeof LANGUAGES)[number];

/** Endonyms: a language picker is useless to someone who cannot read it. */
export const LANGUAGE_NAMES: Record<Language, string> = {
  en: 'English',
  th: 'ไทย',
  zh: '中文',
  ja: '日本語',
  my: 'မြန်မာ',
};

/** BCP 47 tags for Intl. Thai formatting uses the Buddhist era, as expected locally. */
const LOCALES: Record<Language, string> = {
  en: 'en-GB',
  th: 'th-TH',
  zh: 'zh-CN',
  ja: 'ja-JP',
  my: 'my-MM',
};

const en = {
  'nav.platform': 'AI PLATFORM',
  'nav.developerPortal': 'Developer Portal',
  'nav.roadmap': 'Roadmap',
  'nav.chat': 'Chat',
  'nav.knowledge': 'Knowledge',
  'nav.chatDisabled': 'Needs a key with the chat:completions and assistants:read scopes',
  'nav.notBuilt': 'Not built yet',

  'page.portal.crumb': 'DEVELOPER PORTAL',
  'page.portal.title': 'Developer Portal',
  'page.roadmap.crumb': 'ROADMAP',
  'page.roadmap.title': 'Roadmap',
  'page.chat.crumb': 'WORKSPACE',
  'page.chat.title': 'Chat',
  'page.rules.crumb': 'CORE GOVERNANCE',
  'page.rules.title': 'Rules that must not slip',
  'page.architecture.crumb': 'SYSTEM ARCHITECTURE',
  'page.architecture.title': 'System architecture',

  'action.open': 'Open',
  'action.apiKey': 'API key',
  'action.viewRoadmap': 'View roadmap',
  'action.addPhase': 'Add phase',
  'action.cancel': 'Cancel',
  'action.save': 'Save key',
  'action.saveProgress': 'Save progress',
  'action.disconnect': 'Disconnect',
  'action.close': 'Close dialog',
  'action.send': 'Send',
  'action.stop': 'Stop',
  'action.newChat': 'New chat',
  'action.changeKey': 'Change key',
  'action.updateProgress': 'Update progress',
  'action.backToPortal': 'Back to Developer Portal',
  'action.openRoadmap': 'Open roadmap',

  'lang.label': 'Language',

  'conn.notConnected': 'Not connected',
  'conn.addKey': 'Add API key',
  'conn.checking': 'Checking credential…',
  'conn.rejected': 'Key rejected',
  'conn.unreachable': 'Gateway unreachable',
  'conn.connectedAs': 'Connected as',
  'conn.summary': '{count} scopes · {rate}/min',

  'dialog.connect.title': 'Connect to the AI Gateway',
  'dialog.connect.intro':
    'The portal calls the Control Plane with an API key. Paste one minted with the apikey command or from the admin endpoint. It is kept in this tab only and is cleared when the tab closes.',
  'dialog.connect.caution':
    'Give the portal the narrowest key that does the job. A browser is not a safe place for admin scopes. This is a development bridge until the Identity Service issues JWTs to the portal.',
  'dialog.connect.keyLabel': 'API key',
  'dialog.addPhase.title': 'Add roadmap phase',
  'dialog.addPhase.body':
    'A new planned phase with three delivery milestones will be added to the roadmap.',
  'dialog.progress.title': 'Update {name}',
  'dialog.progress.body':
    'Set delivery progress for this phase. Completed milestones will be recalculated.',

  'scope.models:read': 'Read the model catalogue',
  'scope.assistants:read': 'Read the assistant catalogue',
  'scope.chat:completions': 'Run completions and read own history',
  'scope.admin:keys': 'Manage API keys',
  'scope.admin:assistants': 'Manage assistants',

  'portal.intro': 'Architecture governance, API contracts and platform delivery.',
  'portal.discovery.loading': 'Reading platform discovery…',
  'portal.discovery.error': 'Control Plane unreachable',
  'portal.notice.title': 'Not connected to the Gateway.',
  'portal.notice.body':
    'Platform discovery is public. Models, compute status and the chat workspace need an API key.',

  'governance.rules.title': 'Rules that must not slip',
  'governance.rules.body': 'Platform requirements every feature has to follow.',
  'governance.architecture.title': 'System architecture',
  'governance.architecture.body': 'The stack actually running, and the boundary of each plane.',

  'stat.environment': 'Environment',
  'stat.environment.hint': 'version {version}',
  'stat.environment.hintEmpty': 'from platform discovery',
  'stat.capabilities': 'Capabilities',
  'stat.capabilities.hintEmpty': 'declared by the Control Plane',
  'stat.models': 'Models',
  'stat.models.hint': 'logical routes loaded',
  'stat.compute': 'Compute plane',
  'stat.compute.hint': '{provider} provider',
  'stat.needsKey': 'connect a key to read',

  'progress.label': 'Delivery progress',
  'progress.milestones': '{done}/{total} milestones',
  'progress.completed': 'Completed',
  'progress.inProgress': 'In progress',
  'progress.planned': 'Planned',

  'roadmap.intro': 'Track platform delivery by phase, milestone and operational readiness.',
  'roadmap.overall': 'overall delivery',
  'roadmap.complete': 'milestones complete',
  'roadmap.activePhases': 'active phases',
  'roadmap.phase': 'Phase {number}',
  'status.complete': 'Complete',
  'status.active': 'In progress',
  'status.planned': 'Planned',

  'detail.rules.intro':
    'These are non-negotiable guardrails. A feature that violates one must be redesigned before implementation.',
  'detail.rules.count': '12 mandatory engineering guardrails',
  'detail.architecture.intro':
    'A truthful view of the local development stack and the enterprise services it is designed to integrate with.',
  'detail.architecture.count': 'Development runtime plus production integration target',
  'detail.required': 'Required',
  'detail.sourceLanguage': 'Governance text is kept in English pending translation review.',

  'module.api.title': 'API Registry',
  'module.api.body': 'Versioned gateway contracts, scopes and owners',
  'module.module.title': 'Module Registry',
  'module.module.body': 'Control Plane boundaries, health and dependencies',
  'module.event.title': 'Event Catalogue',
  'module.event.body': 'Kafka topic proposals, groups and retention policy',
  'module.map.title': 'Dependency Map',
  'module.map.body': 'Allowed synchronous and asynchronous service paths',
  'module.flags.title': 'Feature Flags',
  'module.flags.body': 'Controlled rollout without redeployment',
  'module.prompts.title': 'Prompt Library',
  'module.prompts.body': 'Approved assistant instructions and policy versions',

  'chat.history': 'History',
  'chat.historyEmpty': 'Conversations you start are kept here.',
  'chat.promptsOff':
    'Prompt logging is off, so stored turns carry no text. Titles and counts still work.',
  'chat.assistant': 'Assistant',
  'chat.assistantMeta': '{description} · model {model}',
  'chat.transcriptHint':
    'Messages are sent through the AI Gateway. Nothing here reaches a model provider directly.',
  'chat.you': 'You',
  'chat.assistantRole': 'Assistant',
  'chat.placeholder': 'Ask something…',
  'chat.placeholderNamed': 'Ask {name}…',
  'chat.usage': '{total} tokens ({input} in · {output} out)',
  'chat.cancelled': 'cancelled',
  'chat.notStored': 'content not stored',
  'chat.untitled': 'Untitled conversation',
  'chat.summary': '{assistant} · {messages} messages · {tokens} tokens',
  'chat.unknownAssistant': 'unknown assistant',
  'chat.delete': 'Delete {title}',
  'chat.error.scope.title': 'This key cannot read assistants',
  'chat.error.scope.body':
    'The connected key is missing the assistants:read scope. Connect a key that has it.',
  'chat.error.catalogue.title': 'Cannot reach the assistant catalogue',
  'chat.error.deleteFailed': 'could not delete the conversation',
  'chat.error.failed': 'the request failed',
} as const;

export type TranslationKey = keyof typeof en;

/** Every locale must supply every key; a missing one is a compile error. */
type Catalogue = Record<TranslationKey, string>;

const th: Catalogue = {
  'nav.platform': 'แพลตฟอร์ม AI',
  'nav.developerPortal': 'พอร์ทัลนักพัฒนา',
  'nav.roadmap': 'แผนงาน',
  'nav.chat': 'สนทนา',
  'nav.knowledge': 'คลังความรู้',
  'nav.chatDisabled': 'ต้องใช้คีย์ที่มีสิทธิ์ chat:completions และ assistants:read',
  'nav.notBuilt': 'ยังไม่ได้พัฒนา',

  'page.portal.crumb': 'พอร์ทัลนักพัฒนา',
  'page.portal.title': 'พอร์ทัลนักพัฒนา',
  'page.roadmap.crumb': 'แผนงาน',
  'page.roadmap.title': 'แผนงาน',
  'page.chat.crumb': 'พื้นที่ทำงาน',
  'page.chat.title': 'สนทนา',
  'page.rules.crumb': 'กฎหลักของระบบ',
  'page.rules.title': 'กฎหลักที่ต้องไม่หลุด',
  'page.architecture.crumb': 'สถาปัตยกรรมระบบ',
  'page.architecture.title': 'สถาปัตยกรรมระบบ',

  'action.open': 'เปิด',
  'action.apiKey': 'คีย์ API',
  'action.viewRoadmap': 'ดูแผนงาน',
  'action.addPhase': 'เพิ่มเฟส',
  'action.cancel': 'ยกเลิก',
  'action.save': 'บันทึกคีย์',
  'action.saveProgress': 'บันทึกความคืบหน้า',
  'action.disconnect': 'ตัดการเชื่อมต่อ',
  'action.close': 'ปิดหน้าต่าง',
  'action.send': 'ส่ง',
  'action.stop': 'หยุด',
  'action.newChat': 'สนทนาใหม่',
  'action.changeKey': 'เปลี่ยนคีย์',
  'action.updateProgress': 'อัปเดตความคืบหน้า',
  'action.backToPortal': 'กลับไปพอร์ทัลนักพัฒนา',
  'action.openRoadmap': 'เปิดแผนงาน',

  'lang.label': 'ภาษา',

  'conn.notConnected': 'ยังไม่เชื่อมต่อ',
  'conn.addKey': 'เพิ่มคีย์ API',
  'conn.checking': 'กำลังตรวจสอบคีย์…',
  'conn.rejected': 'คีย์ถูกปฏิเสธ',
  'conn.unreachable': 'ติดต่อเกตเวย์ไม่ได้',
  'conn.connectedAs': 'เชื่อมต่อในชื่อ',
  'conn.summary': '{count} สิทธิ์ · {rate}/นาที',

  'dialog.connect.title': 'เชื่อมต่อกับ AI Gateway',
  'dialog.connect.intro':
    'พอร์ทัลเรียก Control Plane ด้วยคีย์ API วางคีย์ที่สร้างจากคำสั่ง apikey หรือจาก endpoint ฝ่ายผู้ดูแล คีย์จะถูกเก็บไว้ในแท็บนี้เท่านั้น และถูกลบเมื่อปิดแท็บ',
  'dialog.connect.caution':
    'ให้พอร์ทัลใช้คีย์ที่มีสิทธิ์น้อยที่สุดเท่าที่จำเป็น เบราว์เซอร์ไม่ใช่ที่ปลอดภัยสำหรับสิทธิ์ระดับผู้ดูแล นี่เป็นทางเชื่อมชั่วคราวจนกว่า Identity Service จะออก JWT ให้พอร์ทัล',
  'dialog.connect.keyLabel': 'คีย์ API',
  'dialog.addPhase.title': 'เพิ่มเฟสในแผนงาน',
  'dialog.addPhase.body': 'ระบบจะเพิ่มเฟสใหม่ที่มีสามหมุดหมายส่งมอบเข้าไปในแผนงาน',
  'dialog.progress.title': 'อัปเดต {name}',
  'dialog.progress.body': 'กำหนดความคืบหน้าของเฟสนี้ ระบบจะคำนวณหมุดหมายที่เสร็จแล้วใหม่',

  'scope.models:read': 'อ่านรายการโมเดล',
  'scope.assistants:read': 'อ่านรายการผู้ช่วย',
  'scope.chat:completions': 'เรียกใช้โมเดลและอ่านประวัติของตนเอง',
  'scope.admin:keys': 'จัดการคีย์ API',
  'scope.admin:assistants': 'จัดการผู้ช่วย',

  'portal.intro': 'การกำกับสถาปัตยกรรม สัญญา API และการส่งมอบแพลตฟอร์ม',
  'portal.discovery.loading': 'กำลังอ่านข้อมูลแพลตฟอร์ม…',
  'portal.discovery.error': 'ติดต่อ Control Plane ไม่ได้',
  'portal.notice.title': 'ยังไม่ได้เชื่อมต่อกับเกตเวย์',
  'portal.notice.body':
    'ข้อมูลแพลตฟอร์มเปิดให้เข้าถึงได้ แต่รายการโมเดล สถานะ Compute และพื้นที่สนทนาต้องใช้คีย์ API',

  'governance.rules.title': 'กฎหลักที่ต้องไม่หลุด',
  'governance.rules.body': 'ข้อบังคับของระบบ - ทุกฟีเจอร์ต้องยึดตาม',
  'governance.architecture.title': 'สถาปัตยกรรมระบบ',
  'governance.architecture.body': 'Stack จริงที่กำลังรันอยู่ และ boundary ของแต่ละ plane',

  'stat.environment': 'สภาพแวดล้อม',
  'stat.environment.hint': 'เวอร์ชัน {version}',
  'stat.environment.hintEmpty': 'จากข้อมูลแพลตฟอร์ม',
  'stat.capabilities': 'ความสามารถ',
  'stat.capabilities.hintEmpty': 'ประกาศโดย Control Plane',
  'stat.models': 'โมเดล',
  'stat.models.hint': 'เส้นทางเชิงตรรกะที่โหลดแล้ว',
  'stat.compute': 'Compute Plane',
  'stat.compute.hint': 'ผู้ให้บริการ {provider}',
  'stat.needsKey': 'เชื่อมต่อคีย์เพื่ออ่านข้อมูล',

  'progress.label': 'ความคืบหน้าการส่งมอบ',
  'progress.milestones': '{done}/{total} หมุดหมาย',
  'progress.completed': 'เสร็จแล้ว',
  'progress.inProgress': 'กำลังดำเนินการ',
  'progress.planned': 'วางแผนไว้',

  'roadmap.intro': 'ติดตามการส่งมอบแพลตฟอร์มตามเฟส หมุดหมาย และความพร้อมด้านปฏิบัติการ',
  'roadmap.overall': 'ความคืบหน้ารวม',
  'roadmap.complete': 'หมุดหมายที่เสร็จแล้ว',
  'roadmap.activePhases': 'เฟสที่กำลังทำ',
  'roadmap.phase': 'เฟส {number}',
  'status.complete': 'เสร็จสิ้น',
  'status.active': 'กำลังดำเนินการ',
  'status.planned': 'วางแผนไว้',

  'detail.rules.intro':
    'นี่คือข้อกำหนดที่ต่อรองไม่ได้ ฟีเจอร์ใดที่ขัดกับข้อใดข้อหนึ่งต้องออกแบบใหม่ก่อนลงมือพัฒนา',
  'detail.rules.count': 'ข้อบังคับทางวิศวกรรม 12 ข้อ',
  'detail.architecture.intro':
    'ภาพจริงของ stack ที่ใช้พัฒนา และบริการระดับองค์กรที่ออกแบบไว้ให้เชื่อมต่อด้วย',
  'detail.architecture.count': 'สภาพแวดล้อมพัฒนา และเป้าหมายการเชื่อมต่อในระบบจริง',
  'detail.required': 'บังคับ',
  'detail.sourceLanguage': 'เนื้อหาการกำกับดูแลยังเป็นภาษาอังกฤษ รอการตรวจทานคำแปล',

  'module.api.title': 'ทะเบียน API',
  'module.api.body': 'สัญญาเกตเวย์ตามเวอร์ชัน สิทธิ์ และผู้รับผิดชอบ',
  'module.module.title': 'ทะเบียนโมดูล',
  'module.module.body': 'ขอบเขต Control Plane สถานะ และการพึ่งพากัน',
  'module.event.title': 'ทะเบียนอีเวนต์',
  'module.event.body': 'ข้อเสนอ Kafka topic กลุ่มผู้บริโภค และนโยบายการเก็บข้อมูล',
  'module.map.title': 'แผนผังการพึ่งพา',
  'module.map.body': 'เส้นทางการเรียกบริการแบบซิงโครนัสและอะซิงโครนัสที่อนุญาต',
  'module.flags.title': 'ฟีเจอร์แฟล็ก',
  'module.flags.body': 'ทยอยเปิดใช้งานได้โดยไม่ต้องดีพลอยใหม่',
  'module.prompts.title': 'คลังพรอมป์ต',
  'module.prompts.body': 'คำสั่งผู้ช่วยที่อนุมัติแล้วและเวอร์ชันของนโยบาย',

  'chat.history': 'ประวัติ',
  'chat.historyEmpty': 'บทสนทนาที่คุณเริ่มจะถูกเก็บไว้ที่นี่',
  'chat.promptsOff':
    'การบันทึกพรอมป์ตถูกปิดอยู่ บทสนทนาที่เก็บไว้จึงไม่มีข้อความ แต่ชื่อเรื่องและจำนวนยังใช้ได้',
  'chat.assistant': 'ผู้ช่วย',
  'chat.assistantMeta': '{description} · โมเดล {model}',
  'chat.transcriptHint':
    'ข้อความถูกส่งผ่าน AI Gateway ไม่มีอะไรในหน้านี้ที่ติดต่อผู้ให้บริการโมเดลโดยตรง',
  'chat.you': 'คุณ',
  'chat.assistantRole': 'ผู้ช่วย',
  'chat.placeholder': 'พิมพ์คำถาม…',
  'chat.placeholderNamed': 'ถาม {name}…',
  'chat.usage': '{total} โทเคน (เข้า {input} · ออก {output})',
  'chat.cancelled': 'ยกเลิกแล้ว',
  'chat.notStored': 'ไม่ได้เก็บเนื้อหา',
  'chat.untitled': 'บทสนทนาไม่มีชื่อ',
  'chat.summary': '{assistant} · {messages} ข้อความ · {tokens} โทเคน',
  'chat.unknownAssistant': 'ไม่ทราบผู้ช่วย',
  'chat.delete': 'ลบ {title}',
  'chat.error.scope.title': 'คีย์นี้อ่านรายการผู้ช่วยไม่ได้',
  'chat.error.scope.body': 'คีย์ที่เชื่อมต่ออยู่ไม่มีสิทธิ์ assistants:read กรุณาใช้คีย์ที่มีสิทธิ์นี้',
  'chat.error.catalogue.title': 'ติดต่อรายการผู้ช่วยไม่ได้',
  'chat.error.deleteFailed': 'ลบบทสนทนาไม่สำเร็จ',
  'chat.error.failed': 'คำขอไม่สำเร็จ',
};

const zh: Catalogue = {
  'nav.platform': 'AI 平台',
  'nav.developerPortal': '开发者门户',
  'nav.roadmap': '路线图',
  'nav.chat': '对话',
  'nav.knowledge': '知识库',
  'nav.chatDisabled': '需要具备 chat:completions 和 assistants:read 权限的密钥',
  'nav.notBuilt': '尚未开发',

  'page.portal.crumb': '开发者门户',
  'page.portal.title': '开发者门户',
  'page.roadmap.crumb': '路线图',
  'page.roadmap.title': '路线图',
  'page.chat.crumb': '工作区',
  'page.chat.title': '对话',
  'page.rules.crumb': '核心治理',
  'page.rules.title': '不可违背的规则',
  'page.architecture.crumb': '系统架构',
  'page.architecture.title': '系统架构',

  'action.open': '打开',
  'action.apiKey': 'API 密钥',
  'action.viewRoadmap': '查看路线图',
  'action.addPhase': '新增阶段',
  'action.cancel': '取消',
  'action.save': '保存密钥',
  'action.saveProgress': '保存进度',
  'action.disconnect': '断开连接',
  'action.close': '关闭对话框',
  'action.send': '发送',
  'action.stop': '停止',
  'action.newChat': '新建对话',
  'action.changeKey': '更换密钥',
  'action.updateProgress': '更新进度',
  'action.backToPortal': '返回开发者门户',
  'action.openRoadmap': '打开路线图',

  'lang.label': '语言',

  'conn.notConnected': '未连接',
  'conn.addKey': '添加 API 密钥',
  'conn.checking': '正在验证密钥…',
  'conn.rejected': '密钥被拒绝',
  'conn.unreachable': '无法连接网关',
  'conn.connectedAs': '已连接为',
  'conn.summary': '{count} 项权限 · {rate}/分钟',

  'dialog.connect.title': '连接到 AI 网关',
  'dialog.connect.intro':
    '门户使用 API 密钥调用控制平面。请粘贴通过 apikey 命令或管理端点生成的密钥。密钥仅保存在当前标签页，关闭标签页后即被清除。',
  'dialog.connect.caution':
    '请为门户使用权限最小的密钥。浏览器不适合存放管理级权限。这只是在身份服务向门户签发 JWT 之前的临时方案。',
  'dialog.connect.keyLabel': 'API 密钥',
  'dialog.addPhase.title': '新增路线图阶段',
  'dialog.addPhase.body': '将向路线图新增一个包含三个交付里程碑的计划阶段。',
  'dialog.progress.title': '更新 {name}',
  'dialog.progress.body': '设置此阶段的交付进度，系统将重新计算已完成的里程碑。',

  'scope.models:read': '读取模型目录',
  'scope.assistants:read': '读取助手目录',
  'scope.chat:completions': '调用模型并读取自己的历史',
  'scope.admin:keys': '管理 API 密钥',
  'scope.admin:assistants': '管理助手',

  'portal.intro': '架构治理、API 契约与平台交付。',
  'portal.discovery.loading': '正在读取平台信息…',
  'portal.discovery.error': '无法连接控制平面',
  'portal.notice.title': '尚未连接到网关。',
  'portal.notice.body': '平台信息公开可读。模型、计算状态与对话工作区需要 API 密钥。',

  'governance.rules.title': '不可违背的规则',
  'governance.rules.body': '平台的强制要求 - 每个功能都必须遵守。',
  'governance.architecture.title': '系统架构',
  'governance.architecture.body': '实际运行的技术栈，以及各平面的边界。',

  'stat.environment': '环境',
  'stat.environment.hint': '版本 {version}',
  'stat.environment.hintEmpty': '来自平台信息',
  'stat.capabilities': '能力',
  'stat.capabilities.hintEmpty': '由控制平面声明',
  'stat.models': '模型',
  'stat.models.hint': '已加载的逻辑路由',
  'stat.compute': '计算平面',
  'stat.compute.hint': '{provider} 提供方',
  'stat.needsKey': '连接密钥后可读取',

  'progress.label': '交付进度',
  'progress.milestones': '{done}/{total} 个里程碑',
  'progress.completed': '已完成',
  'progress.inProgress': '进行中',
  'progress.planned': '已计划',

  'roadmap.intro': '按阶段、里程碑和运维就绪度跟踪平台交付。',
  'roadmap.overall': '整体交付',
  'roadmap.complete': '个里程碑已完成',
  'roadmap.activePhases': '个进行中的阶段',
  'roadmap.phase': '第 {number} 阶段',
  'status.complete': '已完成',
  'status.active': '进行中',
  'status.planned': '已计划',

  'detail.rules.intro': '这些是不可协商的底线。违反任何一条的功能必须在实现前重新设计。',
  'detail.rules.count': '12 条强制工程准则',
  'detail.architecture.intro': '本地开发技术栈的真实情况，以及它设计要对接的企业服务。',
  'detail.architecture.count': '开发运行环境与生产集成目标',
  'detail.required': '必须遵守',
  'detail.sourceLanguage': '治理内容暂以英文呈现，等待译文审校。',

  'module.api.title': 'API 注册表',
  'module.api.body': '带版本的网关契约、权限与责任人',
  'module.module.title': '模块注册表',
  'module.module.body': '控制平面边界、健康状况与依赖关系',
  'module.event.title': '事件目录',
  'module.event.body': 'Kafka 主题提案、消费组与保留策略',
  'module.map.title': '依赖关系图',
  'module.map.body': '允许的同步与异步服务调用路径',
  'module.flags.title': '功能开关',
  'module.flags.body': '无需重新部署即可逐步放量',
  'module.prompts.title': '提示词库',
  'module.prompts.body': '已批准的助手指令与策略版本',

  'chat.history': '历史记录',
  'chat.historyEmpty': '你发起的对话会保存在这里。',
  'chat.promptsOff': '提示词记录已关闭，保存的对话不含文本，但标题和统计仍然可用。',
  'chat.assistant': '助手',
  'chat.assistantMeta': '{description} · 模型 {model}',
  'chat.transcriptHint': '消息通过 AI 网关发送。此处没有任何内容直接触达模型提供方。',
  'chat.you': '你',
  'chat.assistantRole': '助手',
  'chat.placeholder': '输入问题…',
  'chat.placeholderNamed': '向 {name} 提问…',
  'chat.usage': '{total} 个 token（输入 {input} · 输出 {output}）',
  'chat.cancelled': '已取消',
  'chat.notStored': '未保存内容',
  'chat.untitled': '未命名对话',
  'chat.summary': '{assistant} · {messages} 条消息 · {tokens} 个 token',
  'chat.unknownAssistant': '未知助手',
  'chat.delete': '删除 {title}',
  'chat.error.scope.title': '此密钥无法读取助手目录',
  'chat.error.scope.body': '当前密钥缺少 assistants:read 权限。请改用具备该权限的密钥。',
  'chat.error.catalogue.title': '无法访问助手目录',
  'chat.error.deleteFailed': '删除对话失败',
  'chat.error.failed': '请求失败',
};

const ja: Catalogue = {
  'nav.platform': 'AI プラットフォーム',
  'nav.developerPortal': '開発者ポータル',
  'nav.roadmap': 'ロードマップ',
  'nav.chat': 'チャット',
  'nav.knowledge': 'ナレッジ',
  'nav.chatDisabled': 'chat:completions と assistants:read の権限を持つキーが必要です',
  'nav.notBuilt': '未実装',

  'page.portal.crumb': '開発者ポータル',
  'page.portal.title': '開発者ポータル',
  'page.roadmap.crumb': 'ロードマップ',
  'page.roadmap.title': 'ロードマップ',
  'page.chat.crumb': 'ワークスペース',
  'page.chat.title': 'チャット',
  'page.rules.crumb': '中核ガバナンス',
  'page.rules.title': '守り抜くべきルール',
  'page.architecture.crumb': 'システムアーキテクチャ',
  'page.architecture.title': 'システムアーキテクチャ',

  'action.open': '開く',
  'action.apiKey': 'API キー',
  'action.viewRoadmap': 'ロードマップを見る',
  'action.addPhase': 'フェーズを追加',
  'action.cancel': 'キャンセル',
  'action.save': 'キーを保存',
  'action.saveProgress': '進捗を保存',
  'action.disconnect': '接続を解除',
  'action.close': 'ダイアログを閉じる',
  'action.send': '送信',
  'action.stop': '停止',
  'action.newChat': '新しいチャット',
  'action.changeKey': 'キーを変更',
  'action.updateProgress': '進捗を更新',
  'action.backToPortal': '開発者ポータルに戻る',
  'action.openRoadmap': 'ロードマップを開く',

  'lang.label': '言語',

  'conn.notConnected': '未接続',
  'conn.addKey': 'API キーを追加',
  'conn.checking': '資格情報を確認中…',
  'conn.rejected': 'キーが拒否されました',
  'conn.unreachable': 'ゲートウェイに接続できません',
  'conn.connectedAs': '接続中',
  'conn.summary': '権限 {count} 件 · {rate}/分',

  'dialog.connect.title': 'AI ゲートウェイに接続',
  'dialog.connect.intro':
    'ポータルは API キーでコントロールプレーンを呼び出します。apikey コマンドまたは管理エンドポイントで発行したキーを貼り付けてください。キーはこのタブにのみ保存され、タブを閉じると消去されます。',
  'dialog.connect.caution':
    'ポータルには必要最小限の権限のキーを渡してください。ブラウザは管理権限を置く場所ではありません。これは Identity Service がポータルへ JWT を発行するまでの暫定手段です。',
  'dialog.connect.keyLabel': 'API キー',
  'dialog.addPhase.title': 'ロードマップにフェーズを追加',
  'dialog.addPhase.body': '3 つの提供マイルストーンを持つ計画フェーズをロードマップに追加します。',
  'dialog.progress.title': '{name} を更新',
  'dialog.progress.body': 'このフェーズの進捗を設定します。完了マイルストーンは再計算されます。',

  'scope.models:read': 'モデルカタログの参照',
  'scope.assistants:read': 'アシスタントカタログの参照',
  'scope.chat:completions': 'モデル呼び出しと自身の履歴の参照',
  'scope.admin:keys': 'API キーの管理',
  'scope.admin:assistants': 'アシスタントの管理',

  'portal.intro': 'アーキテクチャガバナンス、API 契約、プラットフォームの提供状況。',
  'portal.discovery.loading': 'プラットフォーム情報を読み込み中…',
  'portal.discovery.error': 'コントロールプレーンに接続できません',
  'portal.notice.title': 'ゲートウェイに接続していません。',
  'portal.notice.body':
    'プラットフォーム情報は公開されています。モデル、コンピュート状態、チャットには API キーが必要です。',

  'governance.rules.title': '守り抜くべきルール',
  'governance.rules.body': 'プラットフォームの必須要件 - すべての機能が従う必要があります。',
  'governance.architecture.title': 'システムアーキテクチャ',
  'governance.architecture.body': '実際に稼働しているスタックと各プレーンの境界。',

  'stat.environment': '環境',
  'stat.environment.hint': 'バージョン {version}',
  'stat.environment.hintEmpty': 'プラットフォーム情報より',
  'stat.capabilities': '機能',
  'stat.capabilities.hintEmpty': 'コントロールプレーンの申告',
  'stat.models': 'モデル',
  'stat.models.hint': '読み込み済みの論理ルート',
  'stat.compute': 'コンピュートプレーン',
  'stat.compute.hint': '{provider} プロバイダー',
  'stat.needsKey': 'キーを接続すると表示されます',

  'progress.label': '提供進捗',
  'progress.milestones': '{done}/{total} マイルストーン',
  'progress.completed': '完了',
  'progress.inProgress': '進行中',
  'progress.planned': '計画中',

  'roadmap.intro': 'フェーズ、マイルストーン、運用準備状況で提供状況を追跡します。',
  'roadmap.overall': '全体の進捗',
  'roadmap.complete': 'マイルストーン完了',
  'roadmap.activePhases': '進行中のフェーズ',
  'roadmap.phase': 'フェーズ {number}',
  'status.complete': '完了',
  'status.active': '進行中',
  'status.planned': '計画中',

  'detail.rules.intro':
    'これらは譲れない前提条件です。いずれかに反する機能は、実装前に再設計しなければなりません。',
  'detail.rules.count': '必須のエンジニアリング指針 12 項目',
  'detail.architecture.intro':
    'ローカル開発スタックの実状と、連携を前提に設計された企業サービス。',
  'detail.architecture.count': '開発ランタイムと本番連携の目標',
  'detail.required': '必須',
  'detail.sourceLanguage': 'ガバナンス本文は翻訳レビュー待ちのため英語のままです。',

  'module.api.title': 'API レジストリ',
  'module.api.body': 'バージョン管理されたゲートウェイ契約、権限、責任者',
  'module.module.title': 'モジュールレジストリ',
  'module.module.body': 'コントロールプレーンの境界、健全性、依存関係',
  'module.event.title': 'イベントカタログ',
  'module.event.body': 'Kafka トピック案、コンシューマーグループ、保持ポリシー',
  'module.map.title': '依存関係マップ',
  'module.map.body': '許可された同期・非同期のサービス経路',
  'module.flags.title': 'フィーチャーフラグ',
  'module.flags.body': '再デプロイなしで段階的に公開',
  'module.prompts.title': 'プロンプトライブラリ',
  'module.prompts.body': '承認済みのアシスタント指示とポリシーのバージョン',

  'chat.history': '履歴',
  'chat.historyEmpty': '開始した会話はここに保存されます。',
  'chat.promptsOff':
    'プロンプト記録が無効です。保存された会話に本文はありませんが、タイトルと件数は利用できます。',
  'chat.assistant': 'アシスタント',
  'chat.assistantMeta': '{description} · モデル {model}',
  'chat.transcriptHint':
    'メッセージは AI ゲートウェイ経由で送信されます。ここから直接モデルプロバイダーに届くものはありません。',
  'chat.you': 'あなた',
  'chat.assistantRole': 'アシスタント',
  'chat.placeholder': '質問を入力…',
  'chat.placeholderNamed': '{name} に質問…',
  'chat.usage': '{total} トークン（入力 {input} · 出力 {output}）',
  'chat.cancelled': 'キャンセルしました',
  'chat.notStored': '本文は保存されていません',
  'chat.untitled': '無題の会話',
  'chat.summary': '{assistant} · {messages} 件のメッセージ · {tokens} トークン',
  'chat.unknownAssistant': '不明なアシスタント',
  'chat.delete': '{title} を削除',
  'chat.error.scope.title': 'このキーではアシスタントを参照できません',
  'chat.error.scope.body':
    '接続中のキーに assistants:read 権限がありません。権限のあるキーで接続してください。',
  'chat.error.catalogue.title': 'アシスタントカタログに接続できません',
  'chat.error.deleteFailed': '会話を削除できませんでした',
  'chat.error.failed': 'リクエストが失敗しました',
};

const my: Catalogue = {
  'nav.platform': 'AI ပလက်ဖောင်း',
  'nav.developerPortal': 'ဆော့ဖ်ဝဲရေးသူ ပေါ်တယ်',
  'nav.roadmap': 'လုပ်ငန်းစဉ်အစီအစဉ်',
  'nav.chat': 'စကားပြော',
  'nav.knowledge': 'အသိပညာစုစည်းမှု',
  'nav.chatDisabled': 'chat:completions နှင့် assistants:read အခွင့်အရေးပါသော key လိုအပ်သည်',
  'nav.notBuilt': 'မတည်ဆောက်ရသေးပါ',

  'page.portal.crumb': 'ဆော့ဖ်ဝဲရေးသူ ပေါ်တယ်',
  'page.portal.title': 'ဆော့ဖ်ဝဲရေးသူ ပေါ်တယ်',
  'page.roadmap.crumb': 'လုပ်ငန်းစဉ်အစီအစဉ်',
  'page.roadmap.title': 'လုပ်ငန်းစဉ်အစီအစဉ်',
  'page.chat.crumb': 'အလုပ်ခန်း',
  'page.chat.title': 'စကားပြော',
  'page.rules.crumb': 'အဓိက စီမံအုပ်ချုပ်မှု',
  'page.rules.title': 'မလွတ်သွားရမည့် စည်းမျဉ်းများ',
  'page.architecture.crumb': 'စနစ် တည်ဆောက်ပုံ',
  'page.architecture.title': 'စနစ် တည်ဆောက်ပုံ',

  'action.open': 'ဖွင့်',
  'action.apiKey': 'API key',
  'action.viewRoadmap': 'အစီအစဉ်ကို ကြည့်',
  'action.addPhase': 'အဆင့် ထည့်',
  'action.cancel': 'ပယ်ဖျက်',
  'action.save': 'key သိမ်း',
  'action.saveProgress': 'တိုးတက်မှု သိမ်း',
  'action.disconnect': 'ချိတ်ဆက်မှု ဖြုတ်',
  'action.close': 'ပိတ်',
  'action.send': 'ပို့',
  'action.stop': 'ရပ်',
  'action.newChat': 'စကားပြောအသစ်',
  'action.changeKey': 'key ပြောင်း',
  'action.updateProgress': 'တိုးတက်မှု မွမ်းမံ',
  'action.backToPortal': 'ပေါ်တယ်ဆီ ပြန်သွား',
  'action.openRoadmap': 'အစီအစဉ် ဖွင့်',

  'lang.label': 'ဘာသာစကား',

  'conn.notConnected': 'ချိတ်ဆက်ထားခြင်း မရှိပါ',
  'conn.addKey': 'API key ထည့်',
  'conn.checking': 'key ကို စစ်ဆေးနေသည်…',
  'conn.rejected': 'key ကို ငြင်းပယ်ခဲ့သည်',
  'conn.unreachable': 'gateway ကို ရောက်မရပါ',
  'conn.connectedAs': 'ချိတ်ဆက်ထားသည်',
  'conn.summary': 'အခွင့်အရေး {count} · {rate}/မိနစ်',

  'dialog.connect.title': 'AI Gateway နှင့် ချိတ်ဆက်ပါ',
  'dialog.connect.intro':
    'ပေါ်တယ်သည် API key ဖြင့် Control Plane ကို ဆက်သွယ်သည်။ apikey command သို့မဟုတ် admin endpoint မှ ထုတ်ထားသော key ကို ကူးထည့်ပါ။ key သည် ဤ tab တွင်သာ ရှိပြီး tab ပိတ်လိုက်သည်နှင့် ဖျက်သိမ်းသည်။',
  'dialog.connect.caution':
    'ပေါ်တယ်အတွက် လိုအပ်သည့် အနည်းဆုံး အခွင့်အရေးသာ ပါသော key ကို အသုံးပြုပါ။ browser သည် admin အခွင့်အရေးထားရန် လုံခြုံသော နေရာ မဟုတ်ပါ။ ဤနည်းလမ်းသည် Identity Service မှ ပေါ်တယ်အတွက် JWT ထုတ်ပေးသည်အထိ ကြားခံ ဖြေရှင်းချက်သာ ဖြစ်သည်။',
  'dialog.connect.keyLabel': 'API key',
  'dialog.addPhase.title': 'အစီအစဉ်တွင် အဆင့် ထည့်ပါ',
  'dialog.addPhase.body': 'ပေးအပ်မှု မှတ်တိုင် သုံးခုပါသော အဆင့်အသစ်ကို အစီအစဉ်တွင် ထည့်ပါမည်။',
  'dialog.progress.title': '{name} ကို မွမ်းမံ',
  'dialog.progress.body':
    'ဤအဆင့်၏ တိုးတက်မှုကို သတ်မှတ်ပါ။ ပြီးစီးသော မှတ်တိုင်များကို ပြန်တွက်ပါမည်။',

  'scope.models:read': 'model စာရင်းကို ဖတ်ရန်',
  'scope.assistants:read': 'assistant စာရင်းကို ဖတ်ရန်',
  'scope.chat:completions': 'model ကို အသုံးပြုပြီး မိမိမှတ်တမ်းကို ဖတ်ရန်',
  'scope.admin:keys': 'API key များ စီမံရန်',
  'scope.admin:assistants': 'assistant များ စီမံရန်',

  'portal.intro': 'တည်ဆောက်ပုံ စီမံအုပ်ချုပ်မှု၊ API သဘောတူညီချက်များနှင့် ပလက်ဖောင်း ပေးအပ်မှု။',
  'portal.discovery.loading': 'ပလက်ဖောင်း အချက်အလက် ဖတ်နေသည်…',
  'portal.discovery.error': 'Control Plane ကို ရောက်မရပါ',
  'portal.notice.title': 'Gateway နှင့် ချိတ်ဆက်ထားခြင်း မရှိပါ။',
  'portal.notice.body':
    'ပလက်ဖောင်း အချက်အလက်ကို အားလုံး ဖတ်နိုင်သည်။ model များ၊ compute အခြေအနေနှင့် စကားပြောခန်းအတွက် API key လိုအပ်သည်။',

  'governance.rules.title': 'မလွတ်သွားရမည့် စည်းမျဉ်းများ',
  'governance.rules.body': 'စနစ်၏ မဖြစ်မနေ လိုက်နာရမည့် သတ်မှတ်ချက်များ။',
  'governance.architecture.title': 'စနစ် တည်ဆောက်ပုံ',
  'governance.architecture.body': 'အမှန်တကယ် အလုပ်လုပ်နေသော stack နှင့် plane တစ်ခုစီ၏ အနယ်နိမိတ်။',

  'stat.environment': 'ပတ်ဝန်းကျင်',
  'stat.environment.hint': 'ဗားရှင်း {version}',
  'stat.environment.hintEmpty': 'ပလက်ဖောင်း အချက်အလက်မှ',
  'stat.capabilities': 'စွမ်းရည်များ',
  'stat.capabilities.hintEmpty': 'Control Plane မှ ကြေညာသည်',
  'stat.models': 'Model များ',
  'stat.models.hint': 'တင်ထားသော logical route များ',
  'stat.compute': 'Compute Plane',
  'stat.compute.hint': '{provider} ပံ့ပိုးသူ',
  'stat.needsKey': 'ဖတ်ရန် key ချိတ်ဆက်ပါ',

  'progress.label': 'ပေးအပ်မှု တိုးတက်မှု',
  'progress.milestones': 'မှတ်တိုင် {done}/{total}',
  'progress.completed': 'ပြီးစီး',
  'progress.inProgress': 'ဆောင်ရွက်နေ',
  'progress.planned': 'စီစဉ်ထား',

  'roadmap.intro': 'အဆင့်၊ မှတ်တိုင်နှင့် လုပ်ငန်းလည်ပတ်ရန် အသင့်ဖြစ်မှုအလိုက် ပေးအပ်မှုကို လိုက်ကြည့်ပါ။',
  'roadmap.overall': 'စုစုပေါင်း ပေးအပ်မှု',
  'roadmap.complete': 'မှတ်တိုင် ပြီးစီး',
  'roadmap.activePhases': 'ဆောင်ရွက်နေသော အဆင့်',
  'roadmap.phase': 'အဆင့် {number}',
  'status.complete': 'ပြီးစီး',
  'status.active': 'ဆောင်ရွက်နေ',
  'status.planned': 'စီစဉ်ထား',

  'detail.rules.intro':
    'ဤအချက်များသည် ညှိနှိုင်းလို့မရသော အနယ်နိမိတ်များ ဖြစ်သည်။ တစ်ချက်ကို ဆန့်ကျင်သော feature ကို အကောင်အထည်မဖော်မီ ပြန်ဒီဇိုင်းရမည်။',
  'detail.rules.count': 'မဖြစ်မနေ လိုက်နာရမည့် အင်ဂျင်နီယာ စည်းမျဉ်း ၁၂ ချက်',
  'detail.architecture.intro':
    'ဆော့ဖ်ဝဲရေးရာ stack ၏ အမှန်တကယ် အခြေအနေနှင့် ချိတ်ဆက်ရန် ဒီဇိုင်းထားသော လုပ်ငန်းဆိုင်ရာ ဝန်ဆောင်မှုများ။',
  'detail.architecture.count': 'ဆော့ဖ်ဝဲရေးရာ runtime နှင့် ထုတ်လုပ်မှု ချိတ်ဆက်ရေး ပန်းတိုင်',
  'detail.required': 'မဖြစ်မနေ',
  'detail.sourceLanguage':
    'စီမံအုပ်ချုပ်မှု စာသားကို ဘာသာပြန် စစ်ဆေးမှု စောင့်ဆိုင်းစဉ် အင်္ဂလိပ်ဘာသာဖြင့် ထားရှိသည်။',

  'module.api.title': 'API မှတ်ပုံတင်',
  'module.api.body': 'ဗားရှင်းခွဲထားသော gateway သဘောတူညီချက်၊ အခွင့်အရေးနှင့် တာဝန်ရှိသူများ',
  'module.module.title': 'Module မှတ်ပုံတင်',
  'module.module.body': 'Control Plane အနယ်နိမိတ်၊ ကျန်းမာမှုနှင့် မှီခိုမှုများ',
  'module.event.title': 'Event စာရင်း',
  'module.event.body': 'Kafka topic အဆိုပြုချက်၊ အုပ်စုများနှင့် သိမ်းဆည်းမှု ပေါ်လစီ',
  'module.map.title': 'မှီခိုမှု ပြဆိပ်',
  'module.map.body': 'ခွင့်ပြုထားသော synchronous နှင့် asynchronous ဝန်ဆောင်မှု လမ်းကြောင်းများ',
  'module.flags.title': 'Feature Flag များ',
  'module.flags.body': 'ပြန်တင်ခြင်း မလိုဘဲ အဆင့်လိုက် ဖွင့်ပေးရန်',
  'module.prompts.title': 'Prompt စာကြည့်တိုက်',
  'module.prompts.body': 'အတည်ပြုထားသော assistant လမ်းညွှန်နှင့် ပေါ်လစီ ဗားရှင်းများ',

  'chat.history': 'မှတ်တမ်း',
  'chat.historyEmpty': 'သင်စတင်သော စကားပြောများကို ဤနေရာတွင် သိမ်းထားပါမည်။',
  'chat.promptsOff':
    'Prompt မှတ်တမ်းတင်ခြင်း ပိတ်ထားသည်။ သိမ်းထားသော စကားပြောများတွင် စာသား မပါဝင်သော်လည်း ခေါင်းစဉ်နှင့် အရေအတွက်များ ရရှိနိုင်သည်။',
  'chat.assistant': 'အကူ',
  'chat.assistantMeta': '{description} · model {model}',
  'chat.transcriptHint':
    'မက်ဆေ့ဂျ်များကို AI Gateway မှတဆင့် ပို့သည်။ ဤနေရာမှ model ပံ့ပိုးသူဆီ တိုက်ရိုက် မရောက်ပါ။',
  'chat.you': 'သင်',
  'chat.assistantRole': 'အကူ',
  'chat.placeholder': 'မေးခွန်း ရေးပါ…',
  'chat.placeholderNamed': '{name} ကို မေးပါ…',
  'chat.usage': 'token {total} (ဝင် {input} · ထွက် {output})',
  'chat.cancelled': 'ပယ်ဖျက်ပြီး',
  'chat.notStored': 'အကြောင်းအရာ မသိမ်းထားပါ',
  'chat.untitled': 'ခေါင်းစဉ်မရှိသော စကားပြော',
  'chat.summary': '{assistant} · မက်ဆေ့ဂျ် {messages} · token {tokens}',
  'chat.unknownAssistant': 'အကူ မသိရပါ',
  'chat.delete': '{title} ကို ဖျက်',
  'chat.error.scope.title': 'ဤ key ဖြင့် assistant များကို ဖတ်မရပါ',
  'chat.error.scope.body':
    'ချိတ်ဆက်ထားသော key တွင် assistants:read အခွင့်အရေး မပါပါ။ ပါသော key ဖြင့် ချိတ်ဆက်ပါ။',
  'chat.error.catalogue.title': 'assistant စာရင်းကို ရောက်မရပါ',
  'chat.error.deleteFailed': 'စကားပြောကို ဖျက်မရပါ',
  'chat.error.failed': 'တောင်းဆိုမှု မအောင်မြင်ပါ',
};

export const CATALOGUES: Record<Language, Catalogue> = { en, th, zh, ja, my };

const STORAGE_KEY = 'ceap.language';

function isLanguage(value: string | null): value is Language {
  return value !== null && (LANGUAGES as readonly string[]).includes(value);
}

/** Reads a stored choice, otherwise the closest browser preference. */
export function detectLanguage(): Language {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (isLanguage(stored)) return stored;

  for (const preferred of navigator.languages ?? [navigator.language]) {
    // Match on the base tag so `zh-Hant` and `en-US` still resolve.
    const base = preferred.toLowerCase().split('-')[0];
    if (base === 'zh' || base === 'ja' || base === 'th' || base === 'en') return base;
    if (base === 'my' || base === 'mya' || base === 'bur') return 'my';
  }
  return 'en';
}

export function storeLanguage(language: Language): void {
  localStorage.setItem(STORAGE_KEY, language);
}

/**
 * Translates a key, substituting {name} placeholders.
 *
 * A missing key cannot happen for a known language because the catalogue type
 * demands every key, so the key itself is only ever returned if a caller reaches
 * this with an untyped string.
 */
export function translate(
  language: Language,
  key: TranslationKey,
  params?: Record<string, string | number>,
): string {
  const template = CATALOGUES[language][key] ?? CATALOGUES.en[key] ?? key;
  if (params === undefined) return template;
  return template.replace(/\{(\w+)\}/g, (match, name: string) =>
    name in params ? String(params[name]) : match,
  );
}

export function formatDate(date: Date | string, language: Language): string {
  const value = typeof date === 'string' ? new Date(date) : date;
  if (Number.isNaN(value.getTime())) return '';
  return new Intl.DateTimeFormat(LOCALES[language], {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(value);
}

export function formatNumber(value: number, language: Language): string {
  return new Intl.NumberFormat(LOCALES[language]).format(value);
}
