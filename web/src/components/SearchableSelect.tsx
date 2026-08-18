import { useEffect, useId, useMemo, useRef, useState } from 'react';
import { useTranslation } from '../LanguageContext';

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
    <div className="field searchable" ref={container}>
      <span className={labelHidden ? 'visually-hidden' : undefined}>{label}</span>
      <button
        type="button"
        className="searchable-trigger"
        disabled={disabled}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={listId}
        onClick={() => setOpen((current) => !current)}
        onKeyDown={onKeyDown}
      >
        <span className={selected ? '' : 'placeholder'}>
          {selected?.label ?? placeholder ?? t('select.none')}
        </span>
        <i aria-hidden="true">{open ? '▴' : '▾'}</i>
      </button>

      {open && (
        <div className="searchable-panel">
          <input
            autoFocus
            className="searchable-filter"
            value={filter}
            placeholder={t('select.filter')}
            aria-label={t('select.filter')}
            onChange={(event) => {
              setFilter(event.target.value);
              setActive(0);
            }}
            onKeyDown={onKeyDown}
          />
          <ul className="searchable-list" role="listbox" id={listId}>
            {visible.length === 0 && <li className="searchable-empty">{t('select.noMatch')}</li>}
            {visible.map((option, index) => (
              <li key={option.value}>
                <button
                  type="button"
                  role="option"
                  aria-selected={option.value === value}
                  disabled={option.disabled}
                  className={index === active ? 'active' : ''}
                  onMouseEnter={() => setActive(index)}
                  onClick={() => choose(option)}
                >
                  <b>{option.label}</b>
                  {option.detail && <small>{option.detail}</small>}
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
