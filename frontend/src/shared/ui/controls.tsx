import { useState, type ComponentProps, type ReactNode } from "react";
import { Eye, EyeOff, LockKeyhole, type LucideIcon } from "lucide-react";

import logo from "../../assets/focusboard-logo.svg";

export function Logo() {
  return (
    <span className="brand">
      <img className="brand-mark" src={logo} alt="" width={36} height={36} />
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
