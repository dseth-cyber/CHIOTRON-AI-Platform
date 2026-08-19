import { createContext, useContext, useEffect, useState, useMemo, type ReactNode } from 'react';

export type ThemeMode = 'glassmorphism' | 'dark' | 'light';

export interface ThemeConfig {
  mode: ThemeMode;
  text: {
    primary: string;
    secondary: string;
  };
  card: string;
  cardBorder: string;
  navBar: string;
  navBorder: string;
  sidebar: string;
  sidebarBorder: string;
  inputBg: string;
  inputBorder: string;
  border: string;
  tableHeader: string;
  tableRow: string;
  tableBorder: string;
  tableDivide: string;
  primary: string;
  primaryHover: string;
  buttonGradient: string;
  progressTrack: string;
  glowColor: string;
  background: string;
}

export const themes: Record<ThemeMode, ThemeConfig> = {
  glassmorphism: {
    mode: 'glassmorphism',
    text: {
      primary: 'text-white',
      secondary: 'text-slate-300',
    },
    card: 'bg-white/[0.04] backdrop-blur-2xl border border-white/[0.09] shadow-2xl shadow-black/30',
    cardBorder: 'border-white/[0.09]',
    navBar: 'bg-[#120a24]/60 backdrop-blur-2xl border-b border-white/[0.08]',
    navBorder: 'border-white/[0.08]',
    sidebar: 'bg-[#120a24]/70 backdrop-blur-2xl border-r border-white/[0.08]',
    sidebarBorder: 'border-white/[0.08]',
    inputBg: 'bg-white/[0.06] backdrop-blur-md',
    inputBorder: 'border-white/[0.12] focus:border-cyan-400 focus:ring-1 focus:ring-cyan-400/50',
    border: 'border-white/[0.08]',
    tableHeader: 'bg-white/[0.06] backdrop-blur-md',
    tableRow: 'hover:bg-white/[0.06] transition-colors',
    tableBorder: 'border-white/[0.08]',
    tableDivide: 'divide-white/[0.08]',
    primary: 'text-cyan-400',
    primaryHover: 'text-cyan-300',
    buttonGradient: 'bg-gradient-to-r from-cyan-400 to-blue-600 shadow-lg shadow-cyan-500/30',
    progressTrack: 'bg-white/[0.08]',
    glowColor: 'from-cyan-400 via-sky-300 to-blue-500',
    background: 'bg-[radial-gradient(ellipse_at_top,_var(--tw-gradient-stops))] from-purple-900/80 via-[#1a0b36] to-[#0d041c] text-white min-h-screen',
  },
  dark: {
    mode: 'dark',
    text: {
      primary: 'text-[#f0f5f7]',
      secondary: 'text-[#9bb0be]',
    },
    card: 'bg-[#14283b] border border-[#34516a] shadow-lg',
    cardBorder: 'border-[#34516a]',
    navBar: 'bg-[#0b1728] border-b border-[#294257]',
    navBorder: 'border-[#294257]',
    sidebar: 'bg-[#0b1728] border-r border-[#294056]',
    sidebarBorder: 'border-[#294056]',
    inputBg: 'bg-[#0e2132]',
    inputBorder: 'border-[#3d6079] focus:border-[#2fc9ad]',
    border: 'border-[#34516a]',
    tableHeader: 'bg-[#0e2132]',
    tableRow: 'hover:bg-[#14304a] transition-colors',
    tableBorder: 'border-[#22394d]',
    tableDivide: 'divide-[#22394d]',
    primary: 'text-[#2de1ba]',
    primaryHover: 'text-[#38e4c1]',
    buttonGradient: 'bg-[#1ad7b0] text-[#07251f] hover:bg-[#38e4c1]',
    progressTrack: 'bg-[#244357]',
    glowColor: 'from-[#1ad7b0] via-[#31b8e3] to-[#58e0c1]',
    background: 'bg-[#101b2d] text-[#f0f5f7] min-h-screen',
  },
  light: {
    mode: 'light',
    text: {
      primary: 'text-gray-900',
      secondary: 'text-gray-600',
    },
    card: 'bg-white border border-gray-200 shadow-md',
    cardBorder: 'border-gray-200',
    navBar: 'bg-white border-b border-gray-200',
    navBorder: 'border-gray-200',
    sidebar: 'bg-gray-50 border-r border-gray-200',
    sidebarBorder: 'border-gray-200',
    inputBg: 'bg-gray-50',
    inputBorder: 'border-gray-300 focus:border-blue-500',
    border: 'border-gray-200',
    tableHeader: 'bg-gray-100',
    tableRow: 'hover:bg-gray-50 transition-colors',
    tableBorder: 'border-gray-200',
    tableDivide: 'divide-gray-200',
    primary: 'text-blue-600',
    primaryHover: 'text-blue-700',
    buttonGradient: 'bg-gradient-to-r from-blue-600 to-cyan-600',
    progressTrack: 'bg-gray-200',
    glowColor: 'from-blue-600 via-indigo-600 to-cyan-600',
    background: 'bg-gray-100 text-gray-900 min-h-screen',
  },
};

interface ThemeContextType {
  theme: ThemeMode;
  themeConfig: ThemeConfig;
  setTheme: (theme: ThemeMode) => void;
  toggleTheme: () => void;
}

const STORAGE_KEY = 'ceap_theme';

const ThemeContext = createContext<ThemeContextType | undefined>(undefined);

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<ThemeMode>(() => {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved === 'glassmorphism' || saved === 'dark' || saved === 'light') {
      return saved;
    }
    return 'glassmorphism'; // Modern Glassmorphism is default
  });

  const setTheme = (newTheme: ThemeMode) => {
    setThemeState(newTheme);
    localStorage.setItem(STORAGE_KEY, newTheme);
  };

  const toggleTheme = () => {
    setThemeState((prev) => {
      const next: ThemeMode =
        prev === 'glassmorphism' ? 'dark' : prev === 'dark' ? 'light' : 'glassmorphism';
      localStorage.setItem(STORAGE_KEY, next);
      return next;
    });
  };

  useEffect(() => {
    const root = document.documentElement;
    root.classList.remove('theme-glassmorphism', 'theme-dark', 'theme-light', 'dark', 'light');
    root.classList.add(`theme-${theme}`);
    if (theme === 'dark' || theme === 'glassmorphism') {
      root.classList.add('dark');
    } else {
      root.classList.add('light');
    }
  }, [theme]);

  const themeConfig = useMemo(() => themes[theme], [theme]);

  return (
    <ThemeContext.Provider value={{ theme, themeConfig, setTheme, toggleTheme }}>
      <div className={themeConfig.background}>{children}</div>
    </ThemeContext.Provider>
  );
}

export function useTheme(): ThemeContextType {
  const context = useContext(ThemeContext);
  if (!context) {
    return {
      theme: 'glassmorphism',
      themeConfig: themes.glassmorphism,
      setTheme: () => {},
      toggleTheme: () => {},
    };
  }
  return context;
}
