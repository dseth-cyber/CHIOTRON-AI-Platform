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
      secondary: 'text-purple-200/90',
    },
    card: 'bg-[#3b1768]/45 backdrop-blur-2xl border border-purple-300/30 shadow-2xl shadow-purple-950/40',
    cardBorder: 'border-purple-300/35',
    navBar: 'bg-[#2a1052]/70 backdrop-blur-2xl border-b border-purple-400/25',
    navBorder: 'border-purple-400/25',
    sidebar: 'bg-[#1c0838]/80 backdrop-blur-2xl border-r border-purple-400/25',
    sidebarBorder: 'border-purple-400/25',
    inputBg: 'bg-purple-800/35 backdrop-blur-md',
    inputBorder: 'border-purple-300/40 focus:border-cyan-400 focus:ring-1 focus:ring-cyan-400/60',
    border: 'border-purple-400/25',
    tableHeader: 'bg-purple-800/40 backdrop-blur-md',
    tableRow: 'hover:bg-purple-700/35 transition-colors',
    tableBorder: 'border-purple-400/25',
    tableDivide: 'divide-purple-400/25',
    primary: 'text-cyan-400',
    primaryHover: 'text-cyan-300',
    buttonGradient: 'bg-gradient-to-r from-cyan-400 via-blue-500 to-indigo-600 shadow-lg shadow-cyan-500/25',
    progressTrack: 'bg-purple-900/60',
    glowColor: 'from-cyan-400 via-purple-300 to-pink-400',
    background: 'bg-[radial-gradient(ellipse_at_top,_var(--tw-gradient-stops))] from-purple-800/80 via-[#2e1065] to-[#1a063b] text-white min-h-screen',
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
    return 'dark'; // Dark theme matches the user's CHIOTRON baseline screenshot
  });

  const setTheme = (newTheme: ThemeMode) => {
    setThemeState(newTheme);
    localStorage.setItem(STORAGE_KEY, newTheme);
  };

  const toggleTheme = () => {
    setThemeState((prev) => {
      const next: ThemeMode =
        prev === 'dark' ? 'glassmorphism' : prev === 'glassmorphism' ? 'light' : 'dark';
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
      theme: 'dark',
      themeConfig: themes.dark,
      setTheme: () => {},
      toggleTheme: () => {},
    };
  }
  return context;
}
