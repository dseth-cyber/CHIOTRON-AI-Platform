import { useEffect, useId, useMemo, useRef, useState } from 'react';
import { useTranslation } from '../LanguageContext';

export type SelectOption = {
  value: string;
  label: string;
  detail?: string;
  disabled?: boolean;
};

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
  labelHidden?: boolean;
  value: string;
  options: SelectOption[];
  onChange: (value: string) => void;
  disabled?: boolean;
  placeholder?: string;
}) {
  const { t } = useTranslation();
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
    <div className="searchable-wrap" ref={container}>
      {!labelHidden && <label className="searchable-label">{label}</label>}
      <button
        type="button"
        className="searchable-btn"
        disabled={disabled}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={listId}
        onClick={() => setOpen((current) => !current)}
        onKeyDown={onKeyDown}
      >
        <span>{selected?.label ?? placeholder ?? t('select.none')}</span>
        <span style={{ fontSize: '0.75rem', opacity: 0.8 }}>{open ? '▲' : '▼'}</span>
      </button>

      {open && (
        <div className="searchable-dropdown">
          <input
            autoFocus
            className="searchable-filter-input"
            value={filter}
            placeholder={t('select.filter')}
            aria-label={t('select.filter')}
            onChange={(event) => {
              setFilter(event.target.value);
              setActive(0);
            }}
            onKeyDown={onKeyDown}
          />
          <ul className="searchable-option-list" role="listbox" id={listId}>
            {visible.length === 0 && (
              <li style={{ padding: 10, textAlign: 'center', fontSize: '0.75rem', opacity: 0.6 }}>
                {t('select.noMatch')}
              </li>
            )}
            {visible.map((option, index) => {
              const isChosen = option.value === value || index === active;
              return (
                <li key={option.value}>
                  <button
                    type="button"
                    role="option"
                    aria-selected={option.value === value}
                    disabled={option.disabled}
                    className={isChosen ? 'searchable-option-btn active' : 'searchable-option-btn'}
                    onMouseEnter={() => setActive(index)}
                    onClick={() => choose(option)}
                  >
                    <span>{option.label}</span>
                    {option.detail && <span className="opt-detail">{option.detail}</span>}
                  </button>
                </li>
              );
            })}
          </ul>
        </div>
      )}
    </div>
  );
}
