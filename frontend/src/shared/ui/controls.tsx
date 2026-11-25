import { useState, type ComponentProps, type ReactNode } from "react";
import { Eye, EyeOff, LockKeyhole, type LucideIcon } from "lucide-react";

export function Logo() {
  return (
    <span className="brand">
      <span className="brand-mark">
        <svg
          width="24"
          height="24"
          viewBox="0 0 24 24"
          fill="none"
          aria-hidden="true"
        >
          <rect
            x="3.5"
            y="4"
            width="17"
            height="16"
            rx="3"
            stroke="currentColor"
            strokeWidth="1.6"
          />
          <path
            d="M9 4v16M12 12.5l2.2 2.2 3.8-4.2"
            stroke="currentColor"
            strokeWidth="1.7"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </span>
      FocusBoard
    </span>
  );
}

export function LoadingIndicator({ children }: { children: ReactNode }) {
  return (
    <span className="loading-indicator" role="status">
      <span aria-hidden="true" />
      {children}
    </span>
  );
}

export function Button({
  variant = "primary",
  loading,
  children,
  disabled,
  ...props
}: ComponentProps<"button"> & {
  variant?: "primary" | "secondary" | "outline" | "danger";
  loading?: string;
}) {
  return (
    <button
      type="button"
      {...props}
      className={`button button--${variant} ${props.className ?? ""}`}
      disabled={disabled || Boolean(loading)}
      aria-busy={Boolean(loading)}
    >
      {loading ?? children}
    </button>
  );
}

export function Input({
  icon: Icon,
  ...props
}: ComponentProps<"input"> & { icon?: LucideIcon }) {
  return (
    <span className="input-wrap">
      {Icon && <Icon className="input-icon" size={18} aria-hidden="true" />}
      <input {...props} />
    </span>
  );
}

export function PasswordInput({
  toggleLabel = "password",
  ...props
}: ComponentProps<"input"> & { toggleLabel?: string }) {
  const [visible, setVisible] = useState(false);
  const Icon = visible ? EyeOff : Eye;
  return (
    <span className="password-wrap">
      <Input
        {...props}
        icon={LockKeyhole}
        type={visible ? "text" : "password"}
      />
      <button
        type="button"
        className="password-toggle"
        disabled={props.disabled}
        onClick={() => setVisible(!visible)}
        aria-label={`${visible ? "Hide" : "Show"} ${toggleLabel}`}
      >
        <Icon size={19} aria-hidden="true" />
      </button>
    </span>
  );
}

export function FormField({
  id,
  label,
  error,
  hint,
  children,
}: {
  id: string;
  label: string;
  error?: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <div className="form-field">
      <label htmlFor={id}>{label}</label>
      {children}
      {hint && (
        <p id={`${id}-hint`} className="field-hint">
          {hint}
        </p>
      )}
      {error && (
        <p id={`${id}-error`} className="field-error">
          {error}
        </p>
      )}
    </div>
  );
}

export function FormError({ children }: { children: ReactNode }) {
  return (
    <div role="alert" className="form-error">
      {children}
    </div>
  );
}
