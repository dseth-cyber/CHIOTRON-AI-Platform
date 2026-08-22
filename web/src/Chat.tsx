import { useEffect, useRef, useState } from 'react';
import { ApiError, streamChat, type TokenUsage } from './api';
import { FavoriteButton } from './components/FavoriteButton';
import { useAssistants, useConversation, useFavorites, useRefreshHistory } from './hooks';
import { useBrandIcon } from '@/contexts/BrandContext';
import { useTranslation } from './LanguageContext';
import type { ChatTarget, Navigate } from './navigation';

type Turn = {
  role: 'user' | 'assistant';
  content: string;
  redacted?: boolean;
  usage?: TokenUsage;
  finishReason?: string;
  latencyMs?: number;
};

const PROMPT_SUGGESTIONS = [
  {
    icon: '✍️',
    title: 'สร้างเอกสารและรายงาน',
    desc: 'ร่างรายงานสรุปผลการดำเนินงานประจำไตรมาส',
    prompt: 'ช่วยร่างโครงสร้างรายงานสรุปผลการดำเนินงานและแนวทางพัฒนาองค์กรประจำไตรมาสนี้ให้หน่อย',
  },
  {
    icon: '📊',
    title: 'วิเคราะห์ข้อมูล & SQL',
    desc: 'เขียนคำสั่ง SQL สรุปยอดขายตามกลุ่มลูกค้า',
    prompt: 'ช่วยเขียนคำสั่ง SQL สำหรับสรุปยอดขายและจัดอันดับพฤติกรรมการซื้อตามกลุ่มลูกค้าให้หน่อย',
  },
  {
    icon: '🔍',
    title: 'ค้นหาคลังความรู้ (RAG)',
    desc: 'ค้นหานโยบายและคู่มือปฏิบัติงานล่าสุด',
    prompt: 'ค้นหานโยบายและคู่มือขั้นตอนการปฏิบัติงานล่าสุดจากคลังข้อมูลความรู้ขององค์กรให้หน่อย',
  },
  {
    icon: '💡',
    title: 'ระดมความคิดและวางแผน',
    desc: 'เสนอไอเดียปรับปรุงประสิทธิภาพการทำงาน',
    prompt: 'ช่วยเสนอ 5 ไอเดียสร้างสรรค์ในการปรับปรุงประสิทธิภาพและลดขั้นตอนการทำงานในทีมให้หน่อย',
  },
];

export function ChatWorkspace({
  target,
  onNavigate,
  onStreamingChange,
  onResponseComplete,
  isChatVisible = true,
}: {
  target: ChatTarget | null;
  onConnect: () => void;
  onNavigate: Navigate;
  onStreamingChange?: (isStreaming: boolean) => void;
  onResponseComplete?: (snippet: string) => void;
  isChatVisible?: boolean;
}) {
  const { t, formatNumber, language } = useTranslation();
  const { customIcon } = useBrandIcon();
  const assistants = useAssistants(true);
  const favorites = useFavorites(true);
  const refreshHistory = useRefreshHistory();

  const [assistant, setAssistant] = useState(target?.assistant ?? '');
  const [conversationId, setConversationId] = useState<string | null>(target?.conversationId ?? null);
  const [turns, setTurns] = useState<Turn[]>([]);
  const [prompt, setPrompt] = useState('');
  const [streaming, setStreaming] = useState(false);
  const [error, setError] = useState('');
  const [showToolsMenu, setShowToolsMenu] = useState(false);

  useEffect(() => {
    onStreamingChange?.(streaming);
  }, [streaming, onStreamingChange]);
  const [showModelPicker, setShowModelPicker] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [copiedIndex, setCopiedIndex] = useState<number | null>(null);
  const [speakingIndex, setSpeakingIndex] = useState<number | null>(null);
  const [isListening, setIsListening] = useState(false);

  // Gemini Live Interactive Two-Way Voice Mode
  const [isLiveMode, setIsLiveMode] = useState(false);
  const [liveState, setLiveState] = useState<'idle' | 'listening' | 'thinking' | 'speaking'>('idle');
  const [liveUserText, setLiveUserText] = useState('');
  const [liveAssistantReply, setLiveAssistantReply] = useState('');
  const [thinkingElapsed, setThinkingElapsed] = useState<number>(0);
  const thinkingTimerRef = useRef<number | null>(null);

  useEffect(() => {
    if (streaming) {
      const startTime = Date.now();
      setThinkingElapsed(0);
      thinkingTimerRef.current = window.setInterval(() => {
        setThinkingElapsed((Date.now() - startTime) / 1000);
      }, 100);
    } else {
      if (thinkingTimerRef.current) {
        clearInterval(thinkingTimerRef.current);
        thinkingTimerRef.current = null;
      }
    }
    return () => {
      if (thinkingTimerRef.current) clearInterval(thinkingTimerRef.current);
    };
  }, [streaming]);

  const detail = useConversation(conversationId);
  const abort = useRef<AbortController | null>(null);
  const transcript = useRef<HTMLDivElement>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const heroTextareaRef = useRef<HTMLTextAreaElement>(null);
  const bottomTextareaRef = useRef<HTMLTextAreaElement>(null);
  const recognitionRef = useRef<any>(null);
  const liveRecognitionRef = useRef<any>(null);
  const isLiveActiveRef = useRef(false);
  const isPausedRef = useRef(false);
  const liveSpokenTextRef = useRef('');
  const silenceTimeoutRef = useRef<number | null>(null);
  const hydratedFor = useRef<string | null>(null);

  const lastAppliedTargetRef = useRef<ChatTarget | null | undefined>(undefined);

  isLiveActiveRef.current = isLiveMode;

  // Sync target changes from props ONLY when the target prop actually changes
  useEffect(() => {
    if (lastAppliedTargetRef.current === target) return;
    lastAppliedTargetRef.current = target;

    if (target?.conversationId !== undefined) {
      setConversationId(target.conversationId);
      if (target.conversationId === null) {
        setTurns([]);
        setPrompt('');
        hydratedFor.current = null;
      }
    } else if (target && Object.keys(target).length === 0) {
      setConversationId(null);
      setTurns([]);
      setPrompt('');
      hydratedFor.current = null;
    }
    if (target?.prompt !== undefined) {
      setPrompt(target.prompt);
    }
    if (target?.assistant !== undefined && target.assistant !== '') {
      setAssistant(target.assistant);
    }
  }, [target]);

  // Set default assistant if none is selected
  useEffect(() => {
    if (assistant === '' && assistants.data && assistants.data.length > 0) {
      setAssistant(assistants.data[0]!.slug);
    }
  }, [assistant, assistants.data]);

  // Hydrate conversation messages when opening from history
  useEffect(() => {
    if (conversationId === null || detail.data === undefined) return;
    if (hydratedFor.current === conversationId) return;
    hydratedFor.current = conversationId;
    if (detail.data.conversation.assistantSlug) {
      setAssistant(detail.data.conversation.assistantSlug);
    }
    setTurns(
      detail.data.messages.map((message) => ({
        role: message.role,
        content: message.content,
        redacted: message.redacted,
        usage: message.completionTokens
          ? {
              promptTokens: message.promptTokens ?? 0,
              completionTokens: message.completionTokens,
              totalTokens: (message.promptTokens ?? 0) + message.completionTokens,
            }
          : undefined,
      })),
    );
  }, [conversationId, detail.data]);

  // Robust Auto-scroll to bottom on new turns, streaming chunks, or status updates
  const scrollToBottom = (behavior: ScrollBehavior = 'smooth') => {
    if (messagesEndRef.current) {
      messagesEndRef.current.scrollIntoView({ behavior, block: 'end' });
    }
    window.scrollTo({
      top: document.documentElement.scrollHeight,
      behavior,
    });
  };

  useEffect(() => {
    scrollToBottom('smooth');
  }, [turns, streaming]);

  useEffect(() => {
    return () => {
      abort.current?.abort();
      if ('speechSynthesis' in window) {
        window.speechSynthesis.cancel();
      }
      try {
        recognitionRef.current?.abort?.();
        liveRecognitionRef.current?.abort?.();
      } catch {}
    };
  }, []);

  // Auto-resize both hero and bottom textareas up to ~9 lines
  useEffect(() => {
    const adjustHeight = (el: HTMLTextAreaElement | null) => {
      if (!el) return;
      el.style.height = 'auto';
      const maxHeight = 225; // ~9 lines
      const scrollHeight = el.scrollHeight;
      if (scrollHeight > maxHeight) {
        el.style.height = `${maxHeight}px`;
        el.style.overflowY = 'auto';
      } else {
        el.style.height = `${Math.max(scrollHeight, 24)}px`;
        el.style.overflowY = 'hidden';
      }
    };

    adjustHeight(heroTextareaRef.current);
    adjustHeight(bottomTextareaRef.current);
  }, [prompt]);

  // Speech Recognition (Voice Input / Dictation into Textarea)
  const toggleVoiceInput = async () => {
    const SpeechRecognition =
      (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;

    if (!SpeechRecognition) {
      alert('เบราว์เซอร์ของคุณยังไม่รองรับระบบสั่งงานด้วยเสียง กรุณาเปิดผ่าน Google Chrome หรือ Microsoft Edge');
      return;
    }

    if (isListening) {
      try {
        recognitionRef.current?.stop();
      } catch {}
      setIsListening(false);
      return;
    }

    // Request microphone permission explicitly if available
    if (navigator.mediaDevices && navigator.mediaDevices.getUserMedia) {
      try {
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
        stream.getTracks().forEach((track) => track.stop());
      } catch (err: any) {
        console.warn('Microphone permission request failed:', err);
        if (err?.name === 'NotAllowedError' || err?.name === 'PermissionDeniedError') {
          alert('⚠️ กรุณากด "อนุญาต (Allow)" ให้เบราว์เซอร์เข้าถึงไมโครโฟน โดยคลิกที่ไอคอนรูปแม่กุญแจหรือการตั้งค่าหน้าเว็บข้างช่อง URL');
          setIsListening(false);
          return;
        }
      }
    }

    try {
      const recognition = new SpeechRecognition();
      recognition.lang = 'th-TH';
      recognition.continuous = false;
      recognition.interimResults = true;
      recognition.maxAlternatives = 1;

      recognition.onstart = () => {
        setIsListening(true);
      };

      recognition.onresult = (event: any) => {
        let text = '';
        for (let i = event.resultIndex; i < event.results.length; i++) {
          text += event.results[i][0].transcript;
        }
        if (text) {
          setPrompt((prev) => {
            const trimmed = prev.trim();
            return trimmed ? `${trimmed} ${text}` : text;
          });
        }
      };

      recognition.onerror = (event: any) => {
        console.warn('Speech recognition error:', event?.error);
        setIsListening(false);
        if (event?.error === 'not-allowed') {
          alert('⚠️ เบราว์เซอร์ไม่อนุญาตให้ใช้ไมโครโฟน กรุณาเปิดสิทธิ์ไมโครโฟนในการตั้งค่าเบราว์เซอร์ (หรือตรวจสอบว่าเชื่อมต่อผ่าน HTTPS / localhost หรือไม่)');
        } else if (event?.error === 'network') {
          alert('⚠️ การแปลงเสียงเป็นข้อความของ Google Speech Service ต้องการการเชื่อมต่ออินเทอร์เน็ต');
        } else if (event?.error === 'audio-capture') {
          alert('⚠️ ไม่พบอุปกรณ์ไมโครโฟน หรือไมโครโฟนกำลังถูกโปรแกรมอื่นใช้งานอยู่');
        }
      };

      recognition.onend = () => {
        setIsListening(false);
      };

      recognitionRef.current = recognition;
      recognition.start();
    } catch (err) {
      console.error('Speech recognition start failed:', err);
      setIsListening(false);
      alert('⚠️ ไม่สามารถเริ่มระบบแปลงเสียงได้ กรุณาตรวจสอบสิทธิ์การใช้ไมโครโฟนในเบราว์เซอร์');
    }
  };

  // Text to Speech (Voice Output / Read Aloud)
  const speakText = (text: string, index: number, onComplete?: () => void) => {
    if (!('speechSynthesis' in window)) {
      onComplete?.();
      return;
    }

    if (speakingIndex === index && !isLiveActiveRef.current) {
      window.speechSynthesis.cancel();
      setSpeakingIndex(null);
      return;
    }

    window.speechSynthesis.cancel();
    const cleanText = text.replace(/[*_#`]/g, '');
    const utterance = new SpeechSynthesisUtterance(cleanText);
    utterance.lang = 'th-TH';
    utterance.rate = 1.05;
    utterance.onend = () => {
      setSpeakingIndex(null);
      onComplete?.();
    };
    utterance.onerror = () => {
      setSpeakingIndex(null);
      onComplete?.();
    };

    setSpeakingIndex(index);
    window.speechSynthesis.speak(utterance);
  };

  // ==========================================
  // Gemini Live Interactive Continuous Voice Mode
  // ==========================================
  const startLiveMode = () => {
    const SpeechRecognition =
      (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;

    if (!SpeechRecognition) {
      alert('เบราว์เซอร์ของคุณยังไม่รองรับการสนทนาด้วยเสียง กรุณาใช้ Google Chrome หรือ Edge');
      return;
    }

    isLiveActiveRef.current = true;
    isPausedRef.current = false;
    setIsLiveMode(true);
    setLiveUserText('');
    setLiveAssistantReply('');
    startLiveListening();
  };

  const stopLiveMode = () => {
    isLiveActiveRef.current = false;
    isPausedRef.current = false;
    if (silenceTimeoutRef.current) {
      clearTimeout(silenceTimeoutRef.current);
      silenceTimeoutRef.current = null;
    }
    setIsLiveMode(false);
    setLiveState('idle');
    try {
      liveRecognitionRef.current?.abort?.();
    } catch {}
    if ('speechSynthesis' in window) {
      window.speechSynthesis.cancel();
    }
    setSpeakingIndex(null);
  };

  const toggleLivePause = () => {
    if (liveState === 'listening') {
      isPausedRef.current = true;
      if (silenceTimeoutRef.current) {
        clearTimeout(silenceTimeoutRef.current);
        silenceTimeoutRef.current = null;
      }
      try {
        liveRecognitionRef.current?.abort?.();
      } catch {}
      setLiveState('idle');
    } else if (liveState === 'speaking') {
      isPausedRef.current = true;
      if ('speechSynthesis' in window) {
        window.speechSynthesis.cancel();
      }
      setSpeakingIndex(null);
      setLiveState('idle');
    } else {
      // Resume listening
      isPausedRef.current = false;
      startLiveListening();
    }
  };

  const startLiveListening = () => {
    if (!isLiveActiveRef.current || isPausedRef.current) return;
    const SpeechRecognition =
      (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
    if (!SpeechRecognition) return;

    if (silenceTimeoutRef.current) {
      clearTimeout(silenceTimeoutRef.current);
      silenceTimeoutRef.current = null;
    }

    try {
      liveRecognitionRef.current?.abort?.();
    } catch {}

    setLiveState('listening');
    liveSpokenTextRef.current = '';

    try {
      const recognition = new SpeechRecognition();
      recognition.lang = 'th-TH';
      recognition.continuous = true;
      recognition.interimResults = true;

      recognition.onresult = (event: any) => {
        if (isPausedRef.current || !isLiveActiveRef.current) return;

        let fullTranscript = '';
        for (let i = 0; i < event.results.length; i++) {
          const res = event.results[i];
          if (res && res[0]) {
            fullTranscript += res[0].transcript;
          }
        }

        const trimmed = fullTranscript.trim();
        if (trimmed) {
          liveSpokenTextRef.current = trimmed;
          setLiveUserText(trimmed);

          // Reset silence timer: trigger AI reply after 1.2s of silence
          if (silenceTimeoutRef.current) {
            clearTimeout(silenceTimeoutRef.current);
          }
          silenceTimeoutRef.current = window.setTimeout(() => {
            if (isLiveActiveRef.current && !isPausedRef.current && liveSpokenTextRef.current) {
              const textToSend = liveSpokenTextRef.current;
              liveSpokenTextRef.current = '';
              try {
                recognition.stop();
              } catch {}
              void sendLiveMessage(textToSend);
            }
          }, 1200);
        }
      };

      recognition.onerror = (event: any) => {
        console.warn('Live speech recognition error:', event?.error);
        if (event?.error === 'not-allowed' || event?.error === 'permission-denied') {
          alert('⚠️ กรุณากด "อนุญาต (Allow)" ให้เบราว์เซอร์เข้าถึงไมโครโฟน เพื่อสนทนาด้วยเสียง');
          stopLiveMode();
          return;
        }
        if (isLiveActiveRef.current && !isPausedRef.current && liveState === 'listening') {
          setTimeout(() => {
            if (isLiveActiveRef.current && !isPausedRef.current) {
              startLiveListening();
            }
          }, 800);
        }
      };

      recognition.onend = () => {
        if (isLiveActiveRef.current && !isPausedRef.current && liveState === 'listening') {
          setTimeout(() => {
            if (isLiveActiveRef.current && !isPausedRef.current && liveState === 'listening') {
              startLiveListening();
            }
          }, 300);
        }
      };

      liveRecognitionRef.current = recognition;
      recognition.start();
    } catch {
      setLiveState('idle');
    }
  };

  const sendLiveMessage = async (text: string) => {
    const effectiveAssistant = assistant || assistants.data?.[0]?.slug || 'fast';
    if (!text.trim() || !effectiveAssistant) {
      if (isLiveActiveRef.current && !isPausedRef.current) {
        startLiveListening();
      }
      return;
    }

    if (silenceTimeoutRef.current) {
      clearTimeout(silenceTimeoutRef.current);
      silenceTimeoutRef.current = null;
    }

    try {
      liveRecognitionRef.current?.stop?.();
    } catch {}

    setLiveState('thinking');
    setLiveAssistantReply('');

    const userTurn: Turn = { role: 'user', content: text };
    const assistantTurn: Turn = { role: 'assistant', content: '' };
    setTurns((existing) => [...existing, userTurn, assistantTurn]);

    abort.current?.abort();
    abort.current = new AbortController();

    let fullReply = '';

    try {
      await streamChat(
        {
          assistant: effectiveAssistant,
          conversationId: conversationId ?? undefined,
          message: text,
        },
        {
          onContent: (delta) => {
            fullReply += delta;
            setLiveAssistantReply(fullReply);
            setTurns((existing) => {
              const last = existing[existing.length - 1];
              if (!last || last.role !== 'assistant') return existing;
              return [...existing.slice(0, -1), { ...last, content: fullReply }];
            });
          },
          onConversation: (id) => {
            setConversationId(id);
            hydratedFor.current = id;
            refreshHistory();
          },
          onDone: (finishReason, usage) => {
            setTurns((existing) => {
              const last = existing[existing.length - 1];
              if (!last || last.role !== 'assistant') return existing;
              return [
                ...existing.slice(0, -1),
                {
                  ...last,
                  usage: usage ?? undefined,
                  finishReason,
                },
              ];
            });

            // AI finishes text response -> speak aloud -> then resume listening!
            if (isLiveActiveRef.current && !isPausedRef.current && fullReply.trim()) {
              setLiveState('speaking');
              speakText(fullReply, 9999, () => {
                if (isLiveActiveRef.current && !isPausedRef.current) {
                  startLiveListening();
                }
              });
            } else if (isLiveActiveRef.current && !isPausedRef.current) {
              startLiveListening();
            }
          },
        },
        abort.current.signal,
      );
    } catch (err: any) {
      console.error('Live voice chat error:', err);
      setLiveAssistantReply(`(เกิดข้อผิดพลาดในการประมวลผลคำตอบ: ${err?.message || 'โปรดลองใหม่อีกครั้ง'})`);
      if (isLiveActiveRef.current && !isPausedRef.current) {
        setTimeout(() => {
          if (isLiveActiveRef.current && !isPausedRef.current) {
            startLiveListening();
          }
        }, 2000);
      }
    }
  };

  const startNew = () => {
    abort.current?.abort();
    if ('speechSynthesis' in window) window.speechSynthesis.cancel();
    setSpeakingIndex(null);
    hydratedFor.current = null;
    setConversationId(null);
    setTurns([]);
    setError('');
    setPrompt('');
    if (heroTextareaRef.current) {
      heroTextareaRef.current.style.height = 'auto';
      heroTextareaRef.current.style.overflowY = 'hidden';
    }
    if (bottomTextareaRef.current) {
      bottomTextareaRef.current.style.height = 'auto';
      bottomTextareaRef.current.style.overflowY = 'hidden';
    }
    onNavigate('chat', { conversationId: null, prompt: '' });
  };

  const handleCopy = async (text: string, index: number) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedIndex(index);
      setTimeout(() => setCopiedIndex(null), 2000);
    } catch {}
  };

  const send = async (explicitText?: string) => {
    const question = (explicitText !== undefined ? explicitText : prompt).trim();
    if (question === '' || streaming || assistant === '') return;

    if (isListening) {
      try {
        recognitionRef.current?.stop();
      } catch {}
      setIsListening(false);
    }

    abort.current?.abort();
    abort.current = new AbortController();

    const userTurn: Turn = { role: 'user', content: question };
    const assistantTurn: Turn = { role: 'assistant', content: '' };
    setTurns((existing) => [...existing, userTurn, assistantTurn]);
    setPrompt('');
    if (heroTextareaRef.current) {
      heroTextareaRef.current.style.height = 'auto';
      heroTextareaRef.current.style.overflowY = 'hidden';
    }
    if (bottomTextareaRef.current) {
      bottomTextareaRef.current.style.height = 'auto';
      bottomTextareaRef.current.style.overflowY = 'hidden';
    }
    setStreaming(true);
    setError('');

    if ('Notification' in window && Notification.permission === 'default') {
      void Notification.requestPermission();
    }

    const startedAt = Date.now();
    let fullReply = '';

    try {
      await streamChat(
        {
          assistant,
          conversationId: conversationId ?? undefined,
          message: question,
        },
        {
          onContent: (delta) => {
            fullReply += delta;
            setTurns((existing) => {
              const last = existing[existing.length - 1];
              if (!last || last.role !== 'assistant') return existing;
              return [...existing.slice(0, -1), { ...last, content: fullReply }];
            });
          },
          onConversation: (id) => {
            setConversationId(id);
            hydratedFor.current = id;
            refreshHistory();
          },
          onDone: (finishReason, usage) => {
            setTurns((existing) => {
              const last = existing[existing.length - 1];
              if (!last || last.role !== 'assistant') return existing;
              return [
                ...existing.slice(0, -1),
                {
                  ...last,
                  usage: usage ?? undefined,
                  finishReason,
                  latencyMs: Date.now() - startedAt,
                },
              ];
            });

            const snippet = fullReply.replace(/[*_#`]/g, '').trim().slice(0, 90);
            onResponseComplete?.(snippet);

            // Send notification if user is away on another page or in another tab
            if (!isChatVisible || document.hidden) {
              if ('Notification' in window && Notification.permission === 'granted') {
                try {
                  new Notification('✨ AI ตอบคำถามเสร็จเรียบร้อยแล้ว', {
                    body: snippet ? `${snippet}...` : 'คลิกเพื่อดูคำตอบในหน้าแชท',
                    icon: '/icon-192.png',
                  });
                } catch {}
              }
            }
          },
        },
        abort.current.signal,
      );
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') {
        setTurns((existing) => {
          const last = existing[existing.length - 1];
          if (!last || last.role !== 'assistant') return existing;
          return [...existing.slice(0, -1), { ...last, finishReason: 'cancelled' }];
        });
      } else if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError('เกิดข้อผิดพลาดในการเชื่อมต่อ กรุณาลองใหม่อีกครั้ง');
      }
    } finally {
      setStreaming(false);
      refreshHistory();
    }
  };

  const selected = assistants.data?.find((entry) => entry.slug === assistant);
  const marked = new Set(
    (favorites.data ?? []).filter((mark) => mark.kind === 'conversation').map((mark) => mark.targetId),
  );
  const title = detail.data?.conversation.title ?? '';
  const isHeroMode = turns.length === 0 && conversationId === null && !detail.isLoading;

  return (
    <section className={`chat-page-gemini ${isHeroMode ? 'hero-view' : 'transcript-view'}`}>
      {/* Top Header Bar */}
      <header className="chat-top-header">
        <div className="chat-model-badge" onClick={() => !streaming && setShowModelPicker((v) => !v)}>
          <span className="model-sparkle">✨</span>
          <span className="model-name">{selected ? selected.name : 'เลือกผู้ช่วย'}</span>
          <span className="model-chevron">▾</span>
        </div>

        {/* Model/Assistant Picker Dropdown */}
        {showModelPicker && (
          <div className="gemini-dropdown-panel model-picker-menu">
            <div className="dropdown-title">เลือกโมเดลและผู้ช่วย AI</div>
            <div className="dropdown-list">
              {(assistants.data ?? []).map((entry) => (
                <button
                  key={entry.slug}
                  className={`dropdown-item ${entry.slug === assistant ? 'active' : ''}`}
                  disabled={!entry.enabled}
                  onClick={() => {
                    setAssistant(entry.slug);
                    setShowModelPicker(false);
                  }}
                >
                  <div className="item-main">
                    <b>{entry.name}</b>
                    <small>{entry.description || entry.logicalModel}</small>
                  </div>
                  {entry.slug === assistant && <span className="check-mark">✓</span>}
                </button>
              ))}
            </div>
          </div>
        )}

        <div className="chat-top-actions">
          {conversationId !== null && (
            <FavoriteButton
              kind="conversation"
              targetId={conversationId}
              marked={marked.has(conversationId)}
              label={title || t('chat.untitled')}
            />
          )}
          <button
            className="gemini-icon-btn"
            title="ประวัติการสนทนา"
            onClick={() => onNavigate('history')}
          >
            🕒 ประวัติ
          </button>
          <button
            className="gemini-icon-btn"
            title="เริ่มสนทนาใหม่"
            onClick={startNew}
            disabled={isHeroMode}
          >
            ＋ แชทใหม่
          </button>
          <button
            className={`gemini-icon-btn ${showAdvanced ? 'active' : ''}`}
            title="รายละเอียดเชิงลึกและบริบท"
            onClick={() => setShowAdvanced((v) => !v)}
          >
            ⚙️ บริบท
          </button>
        </div>
      </header>

      {/* Advanced Inspector Drawer */}
      {showAdvanced && selected && (
        <aside className="gemini-advanced-drawer">
          <div className="drawer-header">
            <b>ข้อมูลผู้ช่วยและการประมวลผล</b>
            <button className="close-btn" onClick={() => setShowAdvanced(false)}>✕</button>
          </div>
          <div className="drawer-body">
            <div className="drawer-pair">
              <span>ชื่อผู้ช่วย:</span>
              <strong>{selected.name}</strong>
            </div>
            <div className="drawer-pair">
              <span>โมเดล Logic:</span>
              <code>{selected.logicalModel}</code>
            </div>
            <div className="drawer-pair">
              <span>คำอธิบาย:</span>
              <p>{selected.description || 'ผู้ช่วยอัจฉริยะทั่วไป'}</p>
            </div>
          </div>
        </aside>
      )}

      {/* Main Conversation Container */}
      <div className="chat-main-container">
        {detail.isLoading && conversationId !== null && turns.length === 0 ? (
          <div className="chat-loading-history">
            <span className="loading-spinner">⏳</span>
            <p>กำลังโหลดประวัติการสนทนา...</p>
          </div>
        ) : isHeroMode ? (
          /* Gemini / ChatGPT Style Hero Greeting */
          <div className="gemini-hero">
            <div className={`hero-avatar ${customIcon ? 'has-custom-icon' : ''}`}>
              {customIcon ? (
                <img src={customIcon} alt="Custom Brand Logo" className="hero-custom-icon-anim" />
              ) : (
                <span className="hero-sparkle-anim">✨</span>
              )}
            </div>
            <h1 className="hero-greeting">สวัสดี มีอะไรให้ฉันช่วยหรือ?</h1>
            <p className="hero-subgreeting">
              พิมพ์คำถาม, แตะไมค์พูด, หรือกดปุ่มเสียงเพื่อคุยตอบโต้ต่อเนื่องแบบสดๆ ได้ทันที
            </p>

            {/* Floating Hero Omnibar */}
            <div className="gemini-omnibar-wrapper">
              <div className="gemini-omnibar">
                <button
                  type="button"
                  className={`omnibar-plus-btn ${showToolsMenu ? 'active' : ''}`}
                  title="เครื่องมือและส่วนขยาย"
                  onClick={() => setShowToolsMenu((v) => !v)}
                >
                  ＋
                </button>

                {showToolsMenu && (
                  <div className="gemini-dropdown-panel tools-menu">
                    <button className="dropdown-item" onClick={() => { setShowToolsMenu(false); onNavigate('documents'); }}>
                      <span className="icon">📄</span>
                      <div className="item-main">
                        <b>คลังเอกสาร (Documents)</b>
                        <small>จัดการและอัปโหลดไฟล์ความรู้</small>
                      </div>
                    </button>
                    <button className="dropdown-item" onClick={() => { setShowToolsMenu(false); onNavigate('create'); }}>
                      <span className="icon">✍️</span>
                      <div className="item-main">
                        <b>สร้างงานเอกสาร (Create)</b>
                        <small>ร่างรายงาน อีเมล โครงสไลด์ หรือโค้ด</small>
                      </div>
                    </button>
                    <button className="dropdown-item" onClick={() => { setShowToolsMenu(false); onNavigate('analyze'); }}>
                      <span className="icon">📊</span>
                      <div className="item-main">
                        <b>วิเคราะห์ข้อมูล (Analyze)</b>
                        <small>วิเคราะห์ข้อความหรือสร้าง SQL</small>
                      </div>
                    </button>
                    <button className="dropdown-item" onClick={() => { setShowToolsMenu(false); onNavigate('search'); }}>
                      <span className="icon">🔍</span>
                      <div className="item-main">
                        <b>ค้นหาความรู้ (Search)</b>
                        <small>ค้นหาแบบ Hybrid & GraphRAG</small>
                      </div>
                    </button>
                  </div>
                )}

                <textarea
                  ref={heroTextareaRef}
                  value={prompt}
                  rows={1}
                  placeholder={selected ? `ถาม ${selected.name} หรือกดไมค์พูด...` : 'ถามอะไรก็ได้ หรือกดไมค์พูด...'}
                  onChange={(e) => setPrompt(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && !e.shiftKey) {
                      e.preventDefault();
                      void send();
                    }
                  }}
                />

                <div className="omnibar-right-controls">
                  {/* Microphone Voice Input Button (Icon Line-art) */}
                  <button
                    type="button"
                    className={`omnibar-mic-icon-btn ${isListening ? 'listening' : ''}`}
                    onClick={toggleVoiceInput}
                    title={isListening ? 'กำลังฟัง... (แตะเพื่อหยุด)' : 'พิมพ์ข้อความด้วยเสียง (ไมค์)'}
                  >
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3z"/>
                      <path d="M19 10v2a7 7 0 0 1-14 0v-2"/>
                      <line x1="12" y1="19" x2="12" y2="22"/>
                    </svg>
                  </button>

                  {/* Gemini Live Waveform Interactive Voice Button */}
                  <button
                    type="button"
                    className="omnibar-live-wave-btn"
                    onClick={startLiveMode}
                    title="เริ่มสนทนาโต้ตอบด้วยเสียงแบบสองทิศทาง (Gemini Live)"
                  >
                    <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor">
                      <rect x="4" y="9" width="2.5" height="6" rx="1.25"/>
                      <rect x="8.5" y="4" width="2.5" height="16" rx="1.25"/>
                      <rect x="13" y="7" width="2.5" height="10" rx="1.25"/>
                      <rect x="17.5" y="9" width="2.5" height="6" rx="1.25"/>
                    </svg>
                  </button>

                  {streaming ? (
                    <button
                      type="button"
                      className="omnibar-stop-btn"
                      onClick={() => abort.current?.abort()}
                      title="หยุดการตอบกลับ"
                    >
                      ⏹
                    </button>
                  ) : prompt.trim() ? (
                    <button
                      type="button"
                      className="omnibar-send-btn ready"
                      disabled={streaming || assistant === ''}
                      onClick={() => void send()}
                      title="ส่งข้อความ (Enter)"
                    >
                      ➤
                    </button>
                  ) : null}
                </div>
              </div>
              {isListening && (
                <div className="voice-listening-badge">
                  <span className="pulse-dot" />
                  <span>กำลังฟังเสียงภาษาไทย... พูดสิ่งที่ต้องการได้เลย</span>
                </div>
              )}
            </div>

            {/* Quick Action Prompt Cards */}
            <div className="gemini-prompt-grid">
              {PROMPT_SUGGESTIONS.map((item, idx) => (
                <button
                  key={idx}
                  className="gemini-prompt-card"
                  onClick={() => {
                    setPrompt(item.prompt);
                    void send(item.prompt);
                  }}
                >
                  <span className="card-icon">{item.icon}</span>
                  <div className="card-content">
                    <b>{item.title}</b>
                    <small>{item.desc}</small>
                  </div>
                </button>
              ))}
            </div>
          </div>
        ) : (
          /* Active Chat Transcript */
          <div className="gemini-transcript" ref={transcript}>
            {title && <div className="transcript-title-tag">💬 {title}</div>}

            {turns.map((turn, index) => (
              <div className={`gemini-message-row ${turn.role}`} key={index}>
                <div className="message-avatar">
                  {turn.role === 'user' ? '👤' : '✨'}
                </div>
                <div className="message-bubble">
                  <div className="message-header">
                    <span className="sender-name">
                      {turn.role === 'user' ? 'คุณ (You)' : selected?.name ?? 'ผู้ช่วย AI'}
                    </span>
                  </div>

                  {/* AI Thinking Animation State (When waiting for initial tokens / reasoning) */}
                  {turn.role === 'assistant' && !turn.content && streaming && index === turns.length - 1 ? (
                    <div className="ai-thinking-card">
                      <div className="ai-thinking-head">
                        <div className="ai-thinking-orb">
                          <span className="sparkle-pulse">✨</span>
                        </div>
                        <div className="ai-thinking-text-group">
                          <div className="ai-thinking-status-row">
                            <span className="ai-thinking-status-label">
                              {language === 'en'
                                ? 'Thinking and reasoning...'
                                : language === 'zh'
                                ? '正在深入思考并生成回答...'
                                : language === 'ja'
                                ? '思考中・回答を生成しています...'
                                : language === 'my'
                                ? 'စဉ်းစားတွေးခေါ်၍ အဖြေထုတ်နေပါသည်...'
                                : 'กำลังคิดและประมวลผลคำตอบ...'}
                            </span>
                            <span className="ai-thinking-timer-pill">
                              ⏱ {thinkingElapsed.toFixed(1)}s
                            </span>
                          </div>
                          <p className="ai-thinking-subhint">
                            {language === 'en'
                              ? 'Accessing models, analyzing context & computing optimal response'
                              : language === 'zh'
                              ? '正在分析上下文、组织逻辑并生成结构化回答'
                              : language === 'ja'
                              ? 'コンテキストを分析し、最適な回答を論理的に構成しています'
                              : language === 'my'
                              ? 'အကြောင်းအရာများကို ခွဲခြမ်းစိတ်ဖြာပြီး အကောင်းဆုံး အဖြေကို ပြင်ဆင်နေပါသည်'
                              : 'กำลังวิเคราะห์บริบท สืบค้นข้อมูล และประมวลผลคำตอบ'}
                          </p>
                        </div>
                      </div>

                      {/* Shimmering Animated Wave Bars */}
                      <div className="ai-thinking-bars">
                        <span className="thinking-bar b1" />
                        <span className="thinking-bar b2" />
                        <span className="thinking-bar b3" />
                        <span className="thinking-bar b4" />
                        <span className="thinking-bar b5" />
                        <span className="thinking-bar b6" />
                        <span className="thinking-bar b7" />
                      </div>
                    </div>
                  ) : (
                    <div className="message-content">
                      {turn.redacted ? (
                        <i className="redacted">{t('chat.notStored')}</i>
                      ) : (
                        turn.content
                      )}
                      {streaming && index === turns.length - 1 && <span className="gemini-caret" />}
                    </div>
                  )}

                  {/* Assistant Message Actions & Usage Metrics */}
                  {turn.role === 'assistant' && turn.content && (
                    <div className="message-footer">
                      <button
                        className="copy-btn"
                        onClick={() => handleCopy(turn.content, index)}
                        title="คัดลอกข้อความ"
                      >
                        {copiedIndex === index ? '✓ คัดลอกแล้ว' : '📋 คัดลอก'}
                      </button>

                      {/* Read Aloud Voice Button */}
                      <button
                        className={`speak-btn ${speakingIndex === index ? 'speaking' : ''}`}
                        onClick={() => speakText(turn.content, index)}
                        title={speakingIndex === index ? 'หยุดอ่าน' : 'อ่านออกเสียง'}
                      >
                        {speakingIndex === index ? '⏹ หยุดเสียง' : '🔊 ฟังเสียง'}
                      </button>

                      {turn.usage && (
                        <span className="token-usage">
                          ⚡ {formatNumber(turn.usage.totalTokens)} tokens
                          {turn.latencyMs ? ` · ${(turn.latencyMs / 1000).toFixed(2)}s` : ''}
                        </span>
                      )}
                    </div>
                  )}
                </div>
              </div>
            ))}

            {/* Invisible anchor element with generous clearance for sticky bottom bar */}
            <div ref={messagesEndRef} className="transcript-scroll-anchor" />
          </div>
        )}
      </div>

      {error !== '' && <p className="error-note chat-error-banner">{error}</p>}

      {/* Sticky Bottom Omnibar (When inside active transcript) */}
      {!isHeroMode && (
        <footer className="gemini-bottom-bar">
          <div className="gemini-omnibar">
            <button
              type="button"
              className={`omnibar-plus-btn ${showToolsMenu ? 'active' : ''}`}
              title="เครื่องมือ"
              onClick={() => setShowToolsMenu((v) => !v)}
            >
              ＋
            </button>

            {showToolsMenu && (
              <div className="gemini-dropdown-panel tools-menu bottom-up">
                <button className="dropdown-item" onClick={() => { setShowToolsMenu(false); onNavigate('documents'); }}>
                  <span className="icon">📄</span>
                  <div className="item-main">
                    <b>คลังเอกสาร (Documents)</b>
                    <small>จัดการไฟล์ความรู้</small>
                  </div>
                </button>
                <button className="dropdown-item" onClick={() => { setShowToolsMenu(false); onNavigate('create'); }}>
                  <span className="icon">✍️</span>
                  <div className="item-main">
                    <b>สร้างงานเอกสาร (Create)</b>
                    <small>ร่างรายงานและบทความ</small>
                  </div>
                </button>
                <button className="dropdown-item" onClick={() => { setShowToolsMenu(false); onNavigate('analyze'); }}>
                  <span className="icon">📊</span>
                  <div className="item-main">
                    <b>วิเคราะห์ข้อมูล (Analyze)</b>
                    <small>สืบค้นข้อมูลเชิงลึก</small>
                  </div>
                </button>
              </div>
            )}

            <textarea
              ref={bottomTextareaRef}
              value={prompt}
              rows={1}
              placeholder={selected ? `ถามต่อกับ ${selected.name}...` : 'ถามคำถามเพิ่มเติม หรือกดไมค์พูด...'}
              onChange={(e) => setPrompt(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault();
                  void send();
                }
              }}
            />

            <div className="omnibar-right-controls">
              {/* Microphone Voice Input Button (Icon Line-art) */}
              <button
                type="button"
                className={`omnibar-mic-icon-btn ${isListening ? 'listening' : ''}`}
                onClick={toggleVoiceInput}
                title={isListening ? 'กำลังฟัง... (แตะเพื่อหยุด)' : 'พิมพ์ข้อความด้วยเสียง (ไมค์)'}
              >
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3z"/>
                  <path d="M19 10v2a7 7 0 0 1-14 0v-2"/>
                  <line x1="12" y1="19" x2="12" y2="22"/>
                </svg>
              </button>

              {/* Gemini Live Waveform Interactive Voice Button */}
              <button
                type="button"
                className="omnibar-live-wave-btn"
                onClick={startLiveMode}
                title="เริ่มสนทนาโต้ตอบด้วยเสียงแบบสองทิศทาง (Gemini Live)"
              >
                <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor">
                  <rect x="4" y="9" width="2.5" height="6" rx="1.25"/>
                  <rect x="8.5" y="4" width="2.5" height="16" rx="1.25"/>
                  <rect x="13" y="7" width="2.5" height="10" rx="1.25"/>
                  <rect x="17.5" y="9" width="2.5" height="6" rx="1.25"/>
                </svg>
              </button>

              {streaming ? (
                <button
                  type="button"
                  className="omnibar-stop-btn"
                  onClick={() => abort.current?.abort()}
                  title="หยุดการตอบกลับ"
                >
                  ⏹
                </button>
              ) : prompt.trim() ? (
                <button
                  type="button"
                  className="omnibar-send-btn ready"
                  disabled={streaming || assistant === ''}
                  onClick={() => void send()}
                  title="ส่งข้อความ (Enter)"
                >
                  ➤
                </button>
              ) : null}
            </div>
          </div>
          {isListening && (
            <div className="voice-listening-badge">
              <span className="pulse-dot" />
              <span>กำลังฟังเสียงภาษาไทย... พูดสิ่งที่ต้องการได้เลย</span>
            </div>
          )}
        </footer>
      )}

      {/* ==================================================== */}
      {/* Gemini Live: Fullscreen Two-Way Voice Chat Overlay */}
      {/* ==================================================== */}
      {isLiveMode && (
        <div className="gemini-live-overlay">
          <div className="live-header">
            <div className="live-title">
              <span className="live-sparkle">✨</span>
              <span>สนทนาสดกับ {selected?.name ?? 'AI Assistant'}</span>
            </div>
            <button className="live-close-btn" onClick={stopLiveMode} title="จบการสนทนาเสียง">
              ✕
            </button>
          </div>

          <div className="live-orb-container">
            <div className={`live-orb ${liveState}`}>
              <div className="orb-ring ring-1" />
              <div className="orb-ring ring-2" />
              <div className="orb-ring ring-3" />
              <div className="orb-core">
                <div className="live-wave-bars">
                  <span className="bar bar-1" />
                  <span className="bar bar-2" />
                  <span className="bar bar-3" />
                  <span className="bar bar-4" />
                </div>
              </div>
            </div>

            <div className="live-status-text">
              {liveState === 'listening' && '🎙️ กำลังฟังคุณพูด... พูดคุยได้ต่อเนื่อง'}
              {liveState === 'thinking' && '🧠 กำลังประมวลผลคำตอบ...'}
              {liveState === 'speaking' && '🔊 ผู้ช่วย AI กำลังตอบ...'}
              {liveState === 'idle' && 'พร้อมสำหรับการสนทนา'}
            </div>

            {/* Realtime Live Subtitles / Transcript */}
            <div className="live-subtitles">
              {liveUserText && (
                <p className="live-user-sub">
                  <b>คุณ:</b> {liveUserText}
                </p>
              )}
              {liveAssistantReply && (
                <p className="live-assistant-sub">
                  <b>{selected?.name ?? 'AI'}:</b> {liveAssistantReply}
                </p>
              )}
            </div>
          </div>

          <div className="live-controls">
            <button
              type="button"
              className={`live-action-btn ${liveState === 'listening' ? 'active' : ''}`}
              onClick={toggleLivePause}
            >
              {liveState === 'listening' ? '⏸ พักไมค์' : liveState === 'speaking' ? '⏹ หยุดพูด' : '🎙️ เริ่มพูด'}
            </button>
            <button type="button" className="live-end-btn" onClick={stopLiveMode}>
              ✕ สิ้นสุดการสนทนา
            </button>
          </div>
        </div>
      )}
    </section>
  );
}
