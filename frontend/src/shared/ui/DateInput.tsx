import { CalendarDays, ChevronLeft, ChevronRight } from "lucide-react";
import {
  useId,
  useRef,
  useState,
  type ComponentProps,
  type KeyboardEvent,
} from "react";
import { toDisplayDate, toISODate } from "./date-value";
import "./date-input.css";

const monthLabel = new Intl.DateTimeFormat("en-GB", {
  month: "long",
  year: "numeric",
  timeZone: "UTC",
});
const dayLabel = new Intl.DateTimeFormat("en-GB", {
  day: "numeric",
  month: "long",
  year: "numeric",
  timeZone: "UTC",
});

function makeDate(year: number, month: number, day: number) {
  const date = new Date(0);
  date.setUTCFullYear(year, month, day);
  return date;
}

type Props = Omit<ComponentProps<"input">, "value" | "onChange" | "type"> & {
  value: string;
  onChange: (value: string) => void;
};

export function DateInput({ value, onChange, disabled, ...input }: Props) {
  const panelId = useId();
  const trigger = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const [focused, setFocused] = useState<Date | null>(null);
  const selected = toISODate(value);

  function focusDay(date: Date) {
    setFocused(date);
    requestAnimationFrame(() =>
      panel.current
        ?.querySelector<HTMLButtonElement>("[data-current-day]")
        ?.focus(),
    );
  }

  function close() {
    setFocused(null);
    trigger.current?.focus();
  }

  function onCalendarKey(event: KeyboardEvent) {
    if (!focused || event.key !== "Escape") return;
    event.preventDefault();
    event.stopPropagation();
    close();
  }

  const year = focused?.getUTCFullYear() ?? 0;
  const month = focused?.getUTCMonth() ?? 0;
  const offset = (makeDate(year, month, 1).getUTCDay() + 6) % 7;
  const days = makeDate(year, month + 1, 0).getUTCDate();

  function changeMonth(step: number) {
    const next = makeDate(year, month + step, 1);
    if (next.getUTCFullYear() >= 1 && next.getUTCFullYear() <= 9999)
      setFocused(next);
  }

  function navigateDays(event: KeyboardEvent) {
    const step = (
      { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -7, ArrowDown: 7 } as Record<
        string,
        number
      >
    )[event.key];
    if (!focused || !step) return;
    event.preventDefault();
    const date = makeDate(year, month, focused.getUTCDate() + step);
    if (date.getUTCFullYear() >= 1 && date.getUTCFullYear() <= 9999)
      focusDay(date);
  }

  return (
    <div className="date-input" onKeyDown={onCalendarKey}>
      <div className="field-with-icon">
        <input
          {...input}
          disabled={disabled}
          type="text"
          value={value}
          onChange={(event) => onChange(event.currentTarget.value)}
          placeholder="DD/MM/YYYY"
        />
        <button
          ref={trigger}
          type="button"
          className="date-input__trigger"
          disabled={disabled}
          aria-label="Choose due date"
          aria-expanded={Boolean(focused)}
          aria-controls={focused ? panelId : undefined}
          onClick={() => {
            if (focused) return close();
            const today = new Date();
            focusDay(
              selected
                ? new Date(`${selected}T00:00:00Z`)
                : makeDate(
                    today.getFullYear(),
                    today.getMonth(),
                    today.getDate(),
                  ),
            );
          }}
        >
          <CalendarDays
            className="field-end-icon"
            size={17}
            aria-hidden="true"
          />
        </button>
      </div>
      {focused && (
        <div
          ref={panel}
          id={panelId}
          role="group"
          aria-label="Due date calendar"
          className="date-input__calendar"
        >
          <div className="date-input__month">
            <button
              type="button"
              aria-label="Previous month"
              disabled={year === 1 && month === 0}
              onClick={() => changeMonth(-1)}
            >
              <ChevronLeft size={16} />
            </button>
            <span aria-live="polite">{monthLabel.format(focused)}</span>
            <button
              type="button"
              aria-label="Next month"
              disabled={year === 9999 && month === 11}
              onClick={() => changeMonth(1)}
            >
              <ChevronRight size={16} />
            </button>
          </div>
          <div className="date-input__days" onKeyDown={navigateDays}>
            {["Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"].map((day) => (
              <span key={day} aria-hidden="true">
                {day}
              </span>
            ))}
            {Array.from({ length: offset }, (_, index) => (
              <span key={`blank-${index}`} aria-hidden="true" />
            ))}
            {Array.from({ length: days }, (_, index) => {
              const day = index + 1;
              const date = makeDate(year, month, day);
              const iso = date.toISOString().slice(0, 10);
              const current = day === focused.getUTCDate();
              return (
                <button
                  key={day}
                  type="button"
                  aria-label={dayLabel.format(date)}
                  aria-pressed={iso === selected}
                  tabIndex={current ? 0 : -1}
                  data-current-day={current || undefined}
                  onClick={() => {
                    onChange(toDisplayDate(iso));
                    close();
                  }}
                >
                  {day}
                </button>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
