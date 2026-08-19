import React, { createContext, useContext, useEffect, useState } from 'react';

export const DEFAULT_FAVICON = `data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" rx="8" fill="%231bd9b2"/><text x="16" y="23" font-family="Arial, sans-serif" font-size="20" font-weight="900" text-anchor="middle" fill="%2305221f">C</text></svg>`;

type BrandContextType = {
  customIcon: string | null;
  uploadIcon: (file: File) => Promise<void>;
  resetIcon: () => void;
};

const BrandContext = createContext<BrandContextType>({
  customIcon: null,
  uploadIcon: async () => {},
  resetIcon: () => {},
});

const STORAGE_KEY = 'chiotron_brand_icon';

function updateFavicon(url: string) {
  let link = document.querySelector<HTMLLinkElement>("link[rel~='icon']");
  if (!link) {
    link = document.createElement('link');
    link.rel = 'icon';
    document.head.appendChild(link);
  }
  link.href = url;
}

export function BrandProvider({ children }: { children: React.ReactNode }) {
  const [customIcon, setCustomIcon] = useState<string | null>(() => {
    return localStorage.getItem(STORAGE_KEY);
  });

  useEffect(() => {
    if (customIcon) {
      updateFavicon(customIcon);
    } else {
      updateFavicon(DEFAULT_FAVICON);
    }
  }, [customIcon]);

  const uploadIcon = (file: File): Promise<void> => {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = (e) => {
        const result = e.target?.result as string;
        if (result) {
          localStorage.setItem(STORAGE_KEY, result);
          setCustomIcon(result);
          resolve();
        } else {
          reject(new Error('Failed to read image file'));
        }
      };
      reader.onerror = () => reject(new Error('Failed to read file'));
      reader.readAsDataURL(file);
    });
  };

  const resetIcon = () => {
    localStorage.removeItem(STORAGE_KEY);
    setCustomIcon(null);
  };

  return (
    <BrandContext.Provider value={{ customIcon, uploadIcon, resetIcon }}>
      {children}
    </BrandContext.Provider>
  );
}

export function useBrandIcon() {
  return useContext(BrandContext);
}
