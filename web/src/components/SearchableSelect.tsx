import { useEffect, useId, useMemo, useRef, useState } from 'react';
import { useTranslation } from '../LanguageContext';
import { useTheme } from '@/contexts/ThemeContext';

export type SelectOption = {
  value: string;
  label: string;
  detail?: string;
  disabled?: boolean;
};

/**
 * A select that can be filtered by typing.
 *
 * ARCHITECTURE-v1 section 10 names this component because a plain select stops
 * being usable once a catalogue has more than a handful of entries, and every
 * page that lists assistants, models or documents hits that eventually.
 *
 * It is a listbox rather than a text input with suggestions: the value is always
 * one of the options, so a caller cannot submit something the platform will then
 * have to reject.
 */
export function SearchableSelect({
  label,
  labelHidden = false,
  value,
  options,
  onChange,
  disabled = false,
  placeholder,
}: {
  label: string;
  /** Keeps the label for screen readers where the surrounding UI already names the control. */
  labelHidden?: boolean;
  value: string;
  options: SelectOption[];
  onChange: (value: string) => void;
  disabled?: boolean;
  placeholder?: string;
}) {
  const { t } = useTranslation();
  const { themeConfig } = useTheme();
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState('');
  const [active, setActive] = useState(0);
  const container = useRef<HTMLDivElement>(null);
  const listId = useId();

  const selected = options.find((option) => option.value === value);
  const visible = useMemo(() => {
    const needle = filter.trim().toLowerCase();
    if (needle === '') return options;
    return options.filter(
      (option) =>
        option.label.toLowerCase().includes(needle) ||
        (option.detail ?? '').toLowerCase().includes(needle),
    );
  }, [filter, options]);

  // Closing on an outside click is what makes this behave like a select rather
  // than a panel that has to be dismissed deliberately.
  useEffect(() => {
    if (!open) return;
    const close = (event: MouseEvent) => {
      if (!container.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', close);
    return () => document.removeEventListener('mousedown', close);
  }, [open]);

  useEffect(() => {
    if (!open) {
      setFilter('');
      setActive(0);
    }
  }, [open]);

  const choose = (option: SelectOption) => {
    if (option.disabled) return;
    onChange(option.value);
    setOpen(false);
  };

  const onKeyDown = (event: React.KeyboardEvent) => {
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault();
        setOpen(true);
        setActive((current) => Math.min(current + 1, visible.length - 1));
        break;
      case 'ArrowUp':
        event.preventDefault();
        setActive((current) => Math.max(current - 1, 0));
        break;
      case 'Enter':
        if (open && visible[active]) {
          event.preventDefault();
          choose(visible[active]);
        }
        break;
      case 'Escape':
        setOpen(false);
        break;
    }
  };

  return (
    <div className="relative w-full" ref={container}>
      <span className={labelHidden ? 'sr-only' : `block text-xs font-medium mb-1.5 ${themeConfig.text.secondary}`}>
        {label}
      </span>
      <button
        type="button"
        className={`w-full px-4 py-2.5 rounded-xl text-sm border flex items-center justify-between transition-all ${
          themeConfig.inputBorder
        } ${themeConfig.inputBg} ${themeConfig.text.primary} focus:outline-none focus:ring-1 focus:ring-cyan-500/50 ${
          disabled ? 'opacity-50 cursor-not-allowed' : ''
        }`}
        disabled={disabled}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={listId}
        onClick={() => setOpen((current) => !current)}
        onKeyDown={onKeyDown}
      >
        <span className={selected ? themeConfig.text.primary : themeConfig.text.secondary}>
          {selected?.label ?? placeholder ?? t('select.none')}
        </span>
        <span className={`text-xs ${themeConfig.text.secondary}`}>{open ? '▴' : '▾'}</span>
      </button>

      {open && (
        <div className={`absolute z-50 mt-1.5 w-full rounded-xl border p-2 shadow-2xl ${themeConfig.cardBorder} ${themeConfig.card}`}>
          <input
            autoFocus
            className={`w-full px-3 py-2 mb-2 rounded-lg text-xs border ${themeConfig.inputBorder} ${themeConfig.inputBg} ${themeConfig.text.primary} focus:outline-none`}
            value={filter}
            placeholder={t('select.filter')}
            aria-label={t('select.filter')}
            onChange={(event) => {
              setFilter(event.target.value);
              setActive(0);
            }}
            onKeyDown={onKeyDown}
          />
          <ul className={`max-h-60 overflow-y-auto space-y-1 divide-y ${themeConfig.tableDivide}`} role="listbox" id={listId}>
            {visible.length === 0 && (
              <li className={`p-3 text-xs text-center ${themeConfig.text.secondary}`}>{t('select.noMatch')}</li>
            )}
            {visible.map((option, index) => (
              <li key={option.value} className="pt-1 first:pt-0">
                <button
                  type="button"
                  role="option"
                  aria-selected={option.value === value}
                  disabled={option.disabled}
                  className={`w-full p-2.5 rounded-lg text-left text-xs transition-colors flex flex-col gap-0.5 ${
                    index === active ? `${themeConfig.tableRow} ${themeConfig.primary}` : `${themeConfig.text.primary}`
                  } ${option.disabled ? 'opacity-40 cursor-not-allowed' : 'cursor-pointer'}`}
                  onMouseEnter={() => setActive(index)}
                  onClick={() => choose(option)}
                >
                  <span className="font-semibold">{option.label}</span>
                  {option.detail && <span className={`text-[11px] ${themeConfig.text.secondary}`}>{option.detail}</span>}
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
