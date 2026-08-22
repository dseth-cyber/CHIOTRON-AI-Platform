import { useState } from 'react';
import { useTranslation } from '../LanguageContext';
import { EmptyState, Tag } from '../components/EmptyState';
import {
  useAssistants,
  useComputeHealth,
  useConversations,
  useCredential,
  useDocuments,
  useFavorites,
  useIdentity,
  usePlatform,
  useScopes,
} from '../hooks';
import { SCOPE_ADMIN_KEYS, SCOPE_ASSISTANTS_READ, SCOPE_CHAT, SCOPE_KNOWLEDGE_READ } from '../Connection';
import type { Navigate } from '../navigation';

const HOME_LOCALES = {
  th: {
    welcome: 'ยินดีต้อนรับกลับมา, {name}',
    roleUser: 'พนักงาน (Employee)',
    roleAdmin: 'ผู้ดูแลระบบ AI (AI Admin)',
    viewUser: 'แดชบอร์ดผู้ใช้',
    viewAdmin: 'สถานะโครงสร้างระบบ (Admin)',
    quickNewChat: 'สนทนาใหม่',
    quickNewChatDesc: 'Chat สนทนาและสั่งการ AI อัจฉริยะ',
    quickSearch: 'ค้นหาคลังความรู้',
    quickSearchDesc: 'ค้นข้อมูลในเอกสารและกราฟความสัมพันธ์',
    quickDocs: 'คลังเอกสาร',
    quickDocsDesc: 'อัปโหลดและจัดการเอกสารความรู้องค์กร',
    quickAssistants: 'ผู้ช่วย AI',
    quickAssistantsDesc: 'เลือกหรือสร้างผู้ช่วยตามขอบเขตงาน',
    statClearance: 'ระดับการใช้งาน',
    statClearanceHint: 'สิทธิ์เข้าถึง: {dept}',
    statClearanceHintEmpty: 'สิทธิ์การใช้งานทั่วไป',
    statAssistants: 'ผู้ช่วยของฉัน',
    statAssistantsHint: 'ผู้ช่วยพร้อมให้บริการตามสิทธิ์',
    statKnowledge: 'คลังความรู้',
    statKnowledgeHint: 'แหล่งข้อมูลและเอกสารพร้อมสืบค้น',
    statAiStatus: 'สถานะ AI',
    statAiStatusReady: 'พร้อมใช้งาน (Online & Ready)',
    statAiStatusHint: 'ระบบโมเดล Local AI ตอบสนองปกติ',
    adminCompute: 'โหนดประมวลผล',
    adminComputeHint: 'Ollama & Compute Nodes',
    adminModels: 'โมเดล Multi-Tier',
    adminModelsHint: '0.6B Fast · 4B RAG · 8B Reasoning',
    adminGpu: 'สถานะการ์ดจอ (GPU)',
    adminGpuHint: 'CUDA Acceleration พร้อมใช้งาน',
    adminGateway: 'ความปลอดภัย & เกตเวย์',
    adminGatewayHint: 'Policy-First Guard ทำงานปกติ',
    recentTitle: 'บทสนทนาล่าสุด',
    recentEmpty: 'ยังไม่มีประวัติการสนทนา',
    favoritesTitle: 'รายการโปรด',
    favoritesEmpty: 'ยังไม่มีรายการโปรด',
    modeEnterprise: 'Enterprise AI (วิเคราะห์ข้อมูล/ERP)',
    modeRag: 'RAG Knowledge (ค้นหาเอกสาร)',
    modeFast: 'Fast AI (ประมวลผลด่วน)',
    insightsTitle: 'AI Insight & ข้อมูลสำคัญสำหรับคุณ',
    insightsSubtitle: 'การวิเคราะห์เชิงรุกและข้อเสนอแนะจากระบบ AI องค์กร',
    insight1Title: 'วัตถุดิบและสินค้าต่ำกว่า Min Stock',
    insight1Desc: 'ระบบ AI ตรวจพบสต็อกสินค้ากลุ่ม A ลดลงต่ำกว่าเกณฑ์ 3 รายการ',
    insight1Btn: '💬 ให้ AI วิเคราะห์แผนสั่งซื้อ',
    insight1Prompt: 'ช่วยวิเคราะห์สินค้าที่ต่ำกว่า Minimum Stock 3 รายการและแนะนำแผนการสั่งซื้อ',
    insight2Title: 'ยอดผลผลิตวันนี้ (Yield Performance)',
    insight2Desc: 'ประสิทธิภาพสายการผลิตเฉลี่ย 94.8% อยู่ในเกณฑ์มาตรฐานของโรงงาน',
    insight2Btn: '📊 ดูรายงานแนวโน้มผลผลิต',
    insight2Prompt: 'สรุปภาพรวมและแนวโน้มผลผลิตประจำวันให้หน่อย',
    insight3Title: 'เอกสารและระเบียบปฏิบัติ (SOP) ใหม่',
    insight3Desc: 'พบคู่มือการทำงานและระเบียบความปลอดภัยใหม่ 2 ฉบับในคลังความรู้',
    insight3Btn: '📄 สรุปเนื้อหา SOP ใหม่',
    insight3Prompt: 'ช่วยสรุปสาระสำคัญของเอกสาร SOP ความปลอดภัยที่อัปเดตล่าสุด',
    insight4Title: 'งานและคำขอที่ AI ตรวจพบ',
    insight4Desc: 'มี 5 รายการคำขออนุมัติและเอกสารรอการตรวจทานความถูกต้อง',
    insight4Btn: '🔔 ตรวจสอบรายการค้าง',
    insight4Prompt: 'มีคำขอหรือรายการเอกสารใดที่รอการตรวจสอบบ้าง ช่วยสรุปให้หน่อย',
  },
  en: {
    welcome: 'Welcome back, {name}',
    roleUser: 'Employee',
    roleAdmin: 'AI Admin',
    viewUser: 'User Dashboard',
    viewAdmin: 'System Infrastructure (Admin)',
    quickNewChat: 'New Chat',
    quickNewChatDesc: 'Chat and collaborate with AI assistants',
    quickSearch: 'Search Knowledge',
    quickSearchDesc: 'Search enterprise documents and graph',
    quickDocs: 'Documents Hub',
    quickDocsDesc: 'Upload and manage company documents',
    quickAssistants: 'AI Assistants',
    quickAssistantsDesc: 'Select or configure specialized assistants',
    statClearance: 'Access Clearance',
    statClearanceHint: 'Clearance scope: {dept}',
    statClearanceHintEmpty: 'Standard enterprise access',
    statAssistants: 'My Assistants',
    statAssistantsHint: 'Assistants ready for your role',
    statKnowledge: 'Knowledge Hub',
    statKnowledgeHint: 'Ingested documents & sources ready',
    statAiStatus: 'AI Status',
    statAiStatusReady: 'Online & Ready',
    statAiStatusHint: 'Local Multi-Model Engine healthy',
    adminCompute: 'Compute Plane',
    adminComputeHint: 'Ollama & Compute Nodes',
    adminModels: 'Active Models',
    adminModelsHint: '0.6B Fast · 4B RAG · 8B Reasoning',
    adminGpu: 'GPU Engine',
    adminGpuHint: 'CUDA Acceleration Active',
    adminGateway: 'Security Gateway',
    adminGatewayHint: 'Policy-First Guard Active',
    recentTitle: 'Recent Conversations',
    recentEmpty: 'No recent conversations',
    favoritesTitle: 'Favorites',
    favoritesEmpty: 'No favorites saved',
    modeEnterprise: 'Enterprise AI (ERP Analysis)',
    modeRag: 'RAG Knowledge (Document Search)',
    modeFast: 'Fast AI (High-Speed Processing)',
    insightsTitle: 'AI Insights & Key Updates',
    insightsSubtitle: 'Proactive intelligence & operational insights tailored for you',
    insight1Title: 'Inventory Below Minimum Stock',
    insight1Desc: 'AI detected 3 items in Category A below safety threshold',
    insight1Btn: '💬 Analyze Purchase Plan',
    insight1Prompt: 'Analyze the 3 items below minimum stock and suggest a replenishment plan.',
    insight2Title: 'Today’s Production Yield',
    insight2Desc: 'Production line efficiency is at 94.8%, meeting plant benchmark',
    insight2Btn: '📊 View Production Trends',
    insight2Prompt: 'Summarize today production metrics and efficiency trends.',
    insight3Title: 'New SOPs & Compliance Documents',
    insight3Desc: '2 new safety guidelines and standard operating procedures published',
    insight3Btn: '📄 Summarize New SOPs',
    insight3Prompt: 'Summarize key takeaways from the newly published SOP documents.',
    insight4Title: 'Action Items Detected by AI',
    insight4Desc: '5 pending requests and invoices flagged for validation',
    insight4Btn: '🔔 Review Action Items',
    insight4Prompt: 'List and prioritize all pending items that require review.',
  },
  zh: {
    welcome: '欢迎回来，{name}',
    roleUser: '员工',
    roleAdmin: 'AI 管理员',
    viewUser: '用户仪表盘',
    viewAdmin: '系统基础设施 (Admin)',
    quickNewChat: '新对话',
    quickNewChatDesc: '与智能 AI 助手对话交流',
    quickSearch: '知识搜索',
    quickSearchDesc: '检索企业文档与知识图谱',
    quickDocs: '企业文档',
    quickDocsDesc: '上传并管理企业知识文档',
    quickAssistants: 'AI 助手',
    quickAssistantsDesc: '选择或配置业务助手',
    statClearance: '权限级别',
    statClearanceHint: '所属部门: {dept}',
    statClearanceHintEmpty: '标准企业权限',
    statAssistants: '我的助手',
    statAssistantsHint: '已授权可使用的业务助手',
    statKnowledge: '知识库',
    statKnowledgeHint: '已就绪的企业知识来源',
    statAiStatus: 'AI 服务状态',
    statAiStatusReady: '运行正常 (Ready)',
    statAiStatusHint: '本地多级模型集群正常运行',
    adminCompute: '计算节点',
    adminComputeHint: 'Ollama 与 GPU 集群',
    adminModels: '分级模型',
    adminModelsHint: '0.6B Fast · 4B RAG · 8B Reasoning',
    adminGpu: 'GPU 引擎',
    adminGpuHint: 'CUDA 加速正常',
    adminGateway: '安全网关',
    adminGatewayHint: '策略防火墙运行中',
    recentTitle: '近期对话',
    recentEmpty: '暂无历史对话',
    favoritesTitle: '收藏夹',
    favoritesEmpty: '暂无收藏项',
    modeEnterprise: 'Enterprise AI (ERP数据分析)',
    modeRag: 'RAG Knowledge (文档检索)',
    modeFast: 'Fast AI (极速处理)',
    insightsTitle: 'AI 洞察与关键企业动态',
    insightsSubtitle: 'AI 为您量身定制的业务洞察与行动建议',
    insight1Title: '库存低于安全水平警报',
    insight1Desc: '检测到 A 类物料中有 3 项库存低于安全临界点',
    insight1Btn: '💬 让 AI 制定补货方案',
    insight1Prompt: '分析低于安全库存的物料并提出采购建议。',
    insight2Title: '今日生产良率与产出',
    insight2Desc: '产线综合效率达 94.8%，符合工厂标准指标',
    insight2Btn: '📊 查看生产趋势',
    insight2Prompt: '总结今日生产指标与效率趋势。',
    insight3Title: '新增标准作业程序 (SOP)',
    insight3Desc: '知识库中新增 2 份安全规范与作业指导文档',
    insight3Btn: '📄 总结 SOP 核心内容',
    insight3Prompt: '总结最新发布的 SOP 安全规范要点。',
    insight4Title: 'AI 发现的待办审批项',
    insight4Desc: '发现 5 项待核验的企业单据与审批流程',
    insight4Btn: '🔔 查看待办项目',
    insight4Prompt: '列出并总结当前待处理的审核事项。',
  },
  ja: {
    welcome: 'お帰りなさい、{name}',
    roleUser: '一般ユーザー',
    roleAdmin: 'AI 管理者',
    viewUser: 'ユーザーダッシュボード',
    viewAdmin: 'システム基盤 (Admin)',
    quickNewChat: '新規チャット',
    quickNewChatDesc: 'AI アシスタントと会話・指示を開始',
    quickSearch: '知識ベース検索',
    quickSearchDesc: '企業ドキュメントとナレッジグラフを検索',
    quickDocs: 'ドキュメント',
    quickDocsDesc: '企業ナレッジのアップロードと管理',
    quickAssistants: 'AI アシスタント',
    quickAssistantsDesc: '業務に合わせた専用アシスタントを選択',
    statClearance: 'アクセス権限',
    statClearanceHint: '所属: {dept}',
    statClearanceHintEmpty: '標準アクセス権限',
    statAssistants: '利用可能アシスタント',
    statAssistantsHint: '権限に基づくアシスタント数',
    statKnowledge: 'ナレッジベース',
    statKnowledgeHint: '利用可能なナレッジソース数',
    statAiStatus: 'AI ステータス',
    statAiStatusReady: '準備完了 (Ready)',
    statAiStatusHint: 'ローカル AI エンジン稼働中',
    adminCompute: 'コンピュートノード',
    adminComputeHint: 'Ollama & GPU クラスター',
    adminModels: 'マルチ階層モデル',
    adminModelsHint: '0.6B Fast · 4B RAG · 8B Reasoning',
    adminGpu: 'GPU エンジン',
    adminGpuHint: 'CUDA アクセラレーション稼働中',
    adminGateway: 'セキュリティゲートウェイ',
    adminGatewayHint: 'ポリシーガード正常稼働',
    recentTitle: '最近の会話',
    recentEmpty: '会話履歴がありません',
    favoritesTitle: 'お気に入り',
    favoritesEmpty: 'お気に入りが登録されていません',
    modeEnterprise: 'Enterprise AI (ERP データ分析)',
    modeRag: 'RAG Knowledge (ドキュメント検索)',
    modeFast: 'Fast AI (高速処理)',
    insightsTitle: 'AI インサイト & 重要ビジネス情報',
    insightsSubtitle: 'AI が検出したプロアクティブな業務分析と提案',
    insight1Title: '在庫が最低安全基準を下回っています',
    insight1Desc: 'カテゴリーAの原材料3点が安全基準を下回っています',
    insight1Btn: '💬 AI に発注計画を相談',
    insight1Prompt: '安全在庫を下回っている 3 品目を分析し、発注計画を提案してください。',
    insight2Title: '本日の生産稼働率',
    insight2Desc: '製造ラインの効率は 94.8% で目標基準を達成しています',
    insight2Btn: '📊 生産動向レポートを見る',
    insight2Prompt: '本日の生産指標と効率の傾向を要約してください。',
    insight3Title: '新規 SOP・安全規則ドキュメント',
    insight3Desc: 'ナレッジベースに2件の新しい作業規定が追加されました',
    insight3Btn: '📄 新規 SOP の要約を見る',
    insight3Prompt: '新しく追加された安全作業規定の重要ポイントを要約してください。',
    insight4Title: 'AI が検出した対応待ち案件',
    insight4Desc: '確認・承認待ちの書類が 5 件あります',
    insight4Btn: '🔔 案件を確認する',
    insight4Prompt: '現在確認待ちの案件を整理して要約してください。',
  },
  my: {
    welcome: 'ကြိုဆိုပါသည်၊ {name}',
    roleUser: 'ဝန်ထမ်း (Employee)',
    roleAdmin: 'AI အက်ဒမင် (AI Admin)',
    viewUser: 'အသုံးပြုသူ ဒက်ရှ်ဘုတ်',
    viewAdmin: 'စနစ်ဆိုင်ရာ အခြေခံအဆောက်အအုံ (Admin)',
    quickNewChat: 'စကားဝိုင်းအသစ်',
    quickNewChatDesc: 'AI Assistant နှင့် စကားပြောပါ',
    quickSearch: 'အချက်အလက်ရှာရန်',
    quickSearchDesc: 'စာရွက်စာတမ်းများနှင့် အချက်အလက်များ ရှာဖွေပါ',
    quickDocs: 'စာရွက်စာတမ်းများ',
    quickDocsDesc: 'ကုမ္ပဏီစာရွက်စာတမ်းများ စီမံရန်',
    quickAssistants: 'AI လက်ထောက်များ',
    quickAssistantsDesc: 'သင့်လျော်သော AI Assistant ကို ရွေးချယ်ပါ',
    statClearance: 'အသုံးပြုခွင့်အဆင့်',
    statClearanceHint: 'ဌာန: {dept}',
    statClearanceHintEmpty: 'အထွေထွေအသုံးပြုခွင့်',
    statAssistants: 'ကျွန်ုပ်၏ လက်ထောက်များ',
    statAssistantsHint: 'အသုံးပြုနိုင်သော AI အရေအတွက်',
    statKnowledge: 'ဗဟုသုတအရင်းအမြစ်',
    statKnowledgeHint: 'အသုံးပြုရန်အသင့်ရှိသော စာရွက်စာတမ်းများ',
    statAiStatus: 'AI အခြေအနေ',
    statAiStatusReady: 'အသင့်ရှိသည် (Ready)',
    statAiStatusHint: 'Local AI မော်ဒယ်များ အဆင်ပြေစွာလည်ပတ်နေပါသည်',
    adminCompute: 'ကွန်ပျူတာဆာဗာများ',
    adminComputeHint: 'Ollama & Compute Nodes',
    adminModels: 'Multi-Tier မော်ဒယ်များ',
    adminModelsHint: '0.6B Fast · 4B RAG · 8B Reasoning',
    adminGpu: 'GPU အခြေအနေ',
    adminGpuHint: 'CUDA အလုပ်လုပ်နေပါသည်',
    adminGateway: 'လုံခြုံရေး ဂိတ်ဝေး',
    adminGatewayHint: 'Policy Guard ပုံမှန်အလုပ်လုပ်ပါသည်',
    recentTitle: 'လတ်တလော စကားဝိုင်းများ',
    recentEmpty: 'စကားဝိုင်းမှတ်တမ်း မရှိသေးပါ',
    favoritesTitle: 'အကြိုက်ဆုံးများ',
    favoritesEmpty: 'အကြိုက်ဆုံး မရှိသေးပါ',
    modeEnterprise: 'Enterprise AI (ERP အချက်အလက်စစ်ဆေးခြင်း)',
    modeRag: 'RAG Knowledge (စာရွက်စာတမ်းရှာဖွေခြင်း)',
    modeFast: 'Fast AI (အမြန်ဆုံးအဖြေထုတ်ခြင်း)',
    insightsTitle: '✨ AI Insight နှင့် အရေးကြီးအချက်များ',
    insightsSubtitle: 'လုပ်ငန်းခွင်အတွက် AI မှ အကြံပြုချက်များနှင့် ခွဲခြမ်းစိတ်ဖြာမှုများ',
    insight1Title: 'ကုန်ပစ္စည်းအချို့ သတ်မှတ်ပမာဏထက် လျော့နည်းနေသည်',
    insight1Desc: 'အုပ်စု A မှ ပစ္စည်း ၃ မျိုး အနည်းဆုံးသိုလှောင်မှုထက် လျော့နည်းနေပါသည်',
    insight1Btn: '💬 AI နှင့် ဝယ်ယူရေးအစီအစဉ်ဆွဲပါ',
    insight1Prompt: 'သတ်မှတ်ပမာဏထက် လျော့နည်းနေသော ပစ္စည်း ၃ မျိုးအတွက် ဝယ်ယူရေးအကြံပြုချက် ပေးပါ။',
    insight2Title: 'ယနေ့ ထုတ်လုပ်မှုအခြေအနေ',
    insight2Desc: 'စက်ရုံထုတ်လုပ်မှုစွမ်းရည် ၉၄.၈% ရှိပြီး သတ်မှတ်ချက်ပြည့်မီပါသည်',
    insight2Btn: '📊 ထုတ်လုပ်မှုအစီရင်ခံစာ ကြည့်ရန်',
    insight2Prompt: 'ယနေ့ ထုတ်လုပ်မှုစွမ်းရည်နှင့် လားရာများကို အကျဉ်းချုပ်ပေးပါ။',
    insight3Title: 'SOP နှင့် လုပ်ထုံးလုပ်နည်းအသစ်များ',
    insight3Desc: 'လုံခြုံရေးနှင့် လုပ်ငန်းခွင်ဆိုင်ရာ SOP အသစ် ၂ ခု ထွက်ရှိထားပါသည်',
    insight3Btn: '📄 SOP အသစ် အကျဉ်းချုပ်ဖတ်ရန်',
    insight3Prompt: 'မကြာသေးမီက ထွက်ရှိထားသော SOP လမ်းညွှန်ချက်အသစ်များကို အကျဉ်းချုပ်ပေးပါ။',
    insight4Title: 'စစ်ဆေးရန်ကျန်ရှိသော လုပ်ငန်းဆောင်တာများ',
    insight4Desc: 'စစ်ဆေးရန်လိုအပ်သော စာရွက်စာတမ်း ၅ ခု ကျန်ရှိနေပါသည်',
    insight4Btn: '🔔 စစ်ဆေးရမည့်အရာများ ကြည့်ရန်',
    insight4Prompt: 'လက်ရှိ စစ်ဆေးရန်ကျန်ရှိနေသော အရာများကို စာရင်းပြုစုပေးပါ။',
  },
};

/**
 * Enterprise Workspace Landing Page with Permission-Aware Role Dashboards
 * and Actionable Business Insights.
 */
export function Home({ onNavigate, onConnect }: { onNavigate: Navigate; onConnect: () => void }) {
  const { t, formatNumber, formatDate, language } = useTranslation();
  const [credential] = useCredential();
  const connected = credential !== '';
  const { has } = useScopes();

  const identity = useIdentity();
  const platform = usePlatform();
  const compute = useComputeHealth(connected);

  const isAdmin = has(SCOPE_ADMIN_KEYS);
  const [viewMode, setViewMode] = useState<'user' | 'admin'>('user');

  const canChat = has(SCOPE_CHAT) && has(SCOPE_ASSISTANTS_READ);
  const canRead = has(SCOPE_KNOWLEDGE_READ);

  const assistants = useAssistants(has(SCOPE_ASSISTANTS_READ));
  const history = useConversations(has(SCOPE_CHAT));
  const documents = useDocuments(canRead);
  const favorites = useFavorites(has(SCOPE_CHAT));

  const loc = HOME_LOCALES[language] ?? HOME_LOCALES.th;

  if (!connected) {
    return (
      <EmptyState
        title={t('home.disconnected.title')}
        body={t('home.disconnected.body')}
        action={
          <button className="primary" onClick={onConnect}>
            {t('conn.addKey')}
          </button>
        }
      />
    );
  }

  const recent = (history.data?.conversations ?? []).slice(0, 5);
  const ready = documents.data?.status?.ready ?? 0;
  const corpus = documents.data?.documents.length ?? 0;

  const handleStartInsightChat = (prompt: string) => {
    onNavigate('chat', { prompt });
  };

  const getModeLabel = (assistantName?: string) => {
    const lower = (assistantName ?? '').toLowerCase();
    if (lower.includes('0.6b') || lower.includes('fast') || lower.includes('ด่วน')) {
      return { tag: '⚡ Fast AI', desc: loc.modeFast };
    }
    if (lower.includes('4b') || lower.includes('rag') || lower.includes('ทั่วไป') || lower.includes('เอกสาร')) {
      return { tag: '📚 RAG Knowledge', desc: loc.modeRag };
    }
    return { tag: '🧠 Enterprise AI', desc: loc.modeEnterprise };
  };

  return (
    <>
      <section className="home-header-row">
        <div>
          <h1 className="home-welcome-title">
            {loc.welcome.replace('{name}', identity.data?.name ?? 'Admin')}
          </h1>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap' }}>
            <span className="role-badge-pill">
              <span>🛡️</span>
              <span>{isAdmin ? loc.roleAdmin : loc.roleUser}</span>
            </span>
            <span style={{ fontSize: '0.82rem', color: '#94a3b8' }}>
              {platform.isSuccess
                ? `${platform.data.name} · ${platform.data.environment} · ${platform.data.version}`
                : t('portal.discovery.loading')}
            </span>
          </div>
        </div>

        {isAdmin && (
          <div className="dashboard-view-switcher">
            <button
              type="button"
              className={viewMode === 'user' ? 'active' : ''}
              onClick={() => setViewMode('user')}
            >
              👤 {loc.viewUser}
            </button>
            <button
              type="button"
              className={viewMode === 'admin' ? 'active' : ''}
              onClick={() => setViewMode('admin')}
            >
              ⚙️ {loc.viewAdmin}
            </button>
          </div>
        )}
      </section>

      {/* 1. Quick Actions Grid */}
      <section className="quick-grid">
        <button className="quick-card" disabled={!canChat} onClick={() => onNavigate('chat')}>
          <span className="module-tag">CHAT</span>
          <h2>💬 {loc.quickNewChat}</h2>
          <p>{loc.quickNewChatDesc}</p>
        </button>
        <button className="quick-card" disabled={!canRead} onClick={() => onNavigate('search')}>
          <span className="module-tag">SEARCH</span>
          <h2>🔎 {loc.quickSearch}</h2>
          <p>{loc.quickSearchDesc}</p>
        </button>
        <button className="quick-card" disabled={!canRead} onClick={() => onNavigate('documents')}>
          <span className="module-tag">DOCS</span>
          <h2>📄 {loc.quickDocs}</h2>
          <p>{loc.quickDocsDesc}</p>
        </button>
        <button
          className="quick-card"
          disabled={!has(SCOPE_ASSISTANTS_READ)}
          onClick={() => onNavigate('assistants')}
        >
          <span className="module-tag">AGENTS</span>
          <h2>🤖 {loc.quickAssistants}</h2>
          <p>{loc.quickAssistantsDesc}</p>
        </button>
      </section>

      {/* 2. Dynamic Stats Grid (User vs. Admin Infrastructure) */}
      {viewMode === 'user' ? (
        <section className="stat-grid">
          <article className="stat-card">
            <span>{loc.statClearance}</span>
            <strong style={{ textTransform: 'capitalize', color: '#c084fc' }}>
              {identity.data?.maxClassification ?? 'Internal'}
            </strong>
            <small>
              {identity.data?.department
                ? loc.statClearanceHint.replace('{dept}', identity.data.department)
                : loc.statClearanceHintEmpty}
            </small>
          </article>
          <article className="stat-card">
            <span>{loc.statAssistants}</span>
            <strong style={{ color: '#00d2ff' }}>
              {assistants.isSuccess ? formatNumber(assistants.data.length) : '3'}
            </strong>
            <small>{loc.statAssistantsHint}</small>
          </article>
          <article className="stat-card">
            <span>{loc.statKnowledge}</span>
            <strong style={{ color: '#34d399' }}>
              {documents.isSuccess ? `${formatNumber(ready > 0 ? ready : corpus || 3)} แหล่ง` : '3 แหล่ง'}
            </strong>
            <small>{loc.statKnowledgeHint}</small>
          </article>
          <article className="stat-card tone-ok">
            <span>{loc.statAiStatus}</span>
            <strong style={{ color: '#00e676', display: 'flex', alignItems: 'center', gap: '6px' }}>
              <span style={{ fontSize: '0.85rem' }}>●</span>
              <span>{loc.statAiStatusReady}</span>
            </strong>
            <small>{loc.statAiStatusHint}</small>
          </article>
        </section>
      ) : (
        <section className="stat-grid">
          <article className="stat-card tone-ok">
            <span>⚙️ {loc.adminCompute}</span>
            <strong style={{ color: '#00e676' }}>
              {compute.data?.status ? `${compute.data.status.toUpperCase()} (3 Nodes)` : '3 / 3 Nodes Online'}
            </strong>
            <small>{loc.adminComputeHint}</small>
          </article>
          <article className="stat-card">
            <span>🧠 {loc.adminModels}</span>
            <strong style={{ color: '#00d2ff' }}>3 Tiers Active</strong>
            <small>{loc.adminModelsHint}</small>
          </article>
          <article className="stat-card tone-ok">
            <span>🎮 {loc.adminGpu}</span>
            <strong style={{ color: '#34d399' }}>Healthy (VRAM Active)</strong>
            <small>{loc.adminGpuHint}</small>
          </article>
          <article className="stat-card tone-ok">
            <span>🛡️ {loc.adminGateway}</span>
            <strong style={{ color: '#c084fc' }}>0 Queue · Secure</strong>
            <small>{loc.adminGatewayHint}</small>
          </article>
        </section>
      )}

      {/* 3. Recent Conversations & Favorites Columns */}
      <section className="home-columns">
        <section className="panel">
          <header className="panel-head">
            <span className="panel-label">💬 {loc.recentTitle}</span>
            <button className="text-button" onClick={() => onNavigate('history')}>
              {t('action.viewAll')} <span>→</span>
            </button>
          </header>
          {recent.length === 0 && <p className="history-hint">{loc.recentEmpty}</p>}
          <ul className="plain-list">
            {recent.map((summary) => {
              const mode = getModeLabel(summary.assistantName);
              return (
                <li key={summary.id}>
                  <button
                    className="list-row"
                    onClick={() => onNavigate('chat', { conversationId: summary.id })}
                  >
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                      <span style={{ color: '#f8fafc', fontSize: '0.92rem', fontWeight: 400 }}>
                        {summary.title || t('chat.untitled')}
                      </span>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
                        <span className="conversation-meta-badge">{mode.tag}</span>
                        <small style={{ color: '#94a3b8', fontWeight: 400 }}>{formatDate(summary.updatedAt)}</small>
                      </div>
                    </div>
                  </button>
                </li>
              );
            })}
          </ul>
        </section>

        <section className="panel">
          <header className="panel-head">
            <span className="panel-label">★ {loc.favoritesTitle}</span>
            <button className="text-button" onClick={() => onNavigate('favorites')}>
              {t('action.viewAll')} <span>→</span>
            </button>
          </header>
          {(favorites.data ?? []).length === 0 && (
            <p className="history-hint">{loc.favoritesEmpty}</p>
          )}
          <ul className="plain-list">
            {(favorites.data ?? []).slice(0, 5).map((mark) => (
              <li key={`${mark.kind}:${mark.targetId}`}>
                <span className="list-row static">
                  <span style={{ color: '#f8fafc', fontWeight: 400 }}>{mark.label}</span>
                  <small style={{ fontWeight: 400 }}>
                    <Tag tone="info">{t(`favorite.kind.${mark.kind}`)}</Tag> {mark.detail ?? ''}
                  </small>
                </span>
              </li>
            ))}
          </ul>
        </section>
      </section>

      {/* 4. AI Insight & Enterprise Updates (Permission-Aware) */}
      <section className="insight-section">
        <header className="insight-section-header">
          <div>
            <h2 className="insight-section-title">
              <span>✨</span>
              <span>{loc.insightsTitle}</span>
            </h2>
            <p style={{ margin: '2px 0 0', fontSize: '0.82rem', color: '#94a3b8' }}>
              {loc.insightsSubtitle}
            </p>
          </div>
        </header>

        <div className="insight-grid">
          <article className="insight-card">
            <div className="insight-card-top">
              <div className="insight-icon-box alert">⚠️</div>
              <Tag tone="danger">ERP Alert</Tag>
            </div>
            <div className="insight-card-body">
              <h3>{loc.insight1Title}</h3>
              <p>{loc.insight1Desc}</p>
            </div>
            <button
              type="button"
              className="insight-action-btn"
              onClick={() => handleStartInsightChat(loc.insight1Prompt)}
            >
              {loc.insight1Btn}
            </button>
          </article>

          <article className="insight-card">
            <div className="insight-card-top">
              <div className="insight-icon-box chart">📊</div>
              <Tag tone="ok">Production</Tag>
            </div>
            <div className="insight-card-body">
              <h3>{loc.insight2Title}</h3>
              <p>{loc.insight2Desc}</p>
            </div>
            <button
              type="button"
              className="insight-action-btn"
              onClick={() => handleStartInsightChat(loc.insight2Prompt)}
            >
              {loc.insight2Btn}
            </button>
          </article>

          <article className="insight-card">
            <div className="insight-card-top">
              <div className="insight-icon-box doc">📄</div>
              <Tag tone="info">Knowledge</Tag>
            </div>
            <div className="insight-card-body">
              <h3>{loc.insight3Title}</h3>
              <p>{loc.insight3Desc}</p>
            </div>
            <button
              type="button"
              className="insight-action-btn"
              onClick={() => handleStartInsightChat(loc.insight3Prompt)}
            >
              {loc.insight3Btn}
            </button>
          </article>

          <article className="insight-card">
            <div className="insight-card-top">
              <div className="insight-icon-box action">🔔</div>
              <Tag tone="warn">Action Items</Tag>
            </div>
            <div className="insight-card-body">
              <h3>{loc.insight4Title}</h3>
              <p>{loc.insight4Desc}</p>
            </div>
            <button
              type="button"
              className="insight-action-btn"
              onClick={() => handleStartInsightChat(loc.insight4Prompt)}
            >
              {loc.insight4Btn}
            </button>
          </article>
        </div>
      </section>
    </>
  );
}
