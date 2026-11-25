import { useEffect, useRef } from "react";
import { Mail, UserRound } from "lucide-react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { ApiError } from "../../shared/api/client";
import {
  Button,
  FormError,
  FormField,
  Input,
  Logo,
  PasswordInput,
} from "../../shared/ui/controls";
import { useSession } from "./context";
import { registerSchema, signInSchema, type AuthFields } from "./validation";

export function AuthForm({
  mode,
  active,
  email,
  fullName,
  updateEmail,
  updateName,
  switchMode,
  onBusy,
  busy,
}: {
  mode: "login" | "register";
  active: boolean;
  email: string;
  fullName: string;
  updateEmail: (email: string) => void;
  updateName: (name: string) => void;
  switchMode: () => void;
  onBusy: (busy: boolean) => void;
  busy: boolean;
}) {
  const { controller } = useSession();
  const creating = mode === "register";
  const title = creating ? "Create account" : "Sign in";
  const heading = useRef<HTMLHeadingElement>(null);
  const submitted = useRef(false);
  const previous = useRef(active);
  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors },
  } = useForm<AuthFields>({
    resolver: zodResolver(creating ? registerSchema : signInSchema),
    defaultValues: { name: fullName, email, password: "" },
  });

  useEffect(() => {
    if (previous.current === active) return;
    previous.current = active;
    reset({ name: fullName, email, password: "" });
    if (active) heading.current?.focus({ preventScroll: true });
  }, [active, email, fullName, reset]);

  const submit = async (values: AuthFields) => {
    if (submitted.current || !active || busy) return;
    submitted.current = true;
    onBusy(true);
    try {
      const { name, ...credentials } = values;
      await controller.authenticate(
        creating ? "/auth/register" : "/auth/login",
        creating ? { ...credentials, name } : credentials,
      );
    } catch (error) {
      if (!heading.current?.closest("[inert]")) {
        const messages = error instanceof ApiError ? error.fields : {};
        const keys = ["name", "email", "password"] as const;
        let mapped = false;
        for (const key of keys) {
          if (messages[key] && (key !== "name" || creating)) {
            setError(key, { message: messages[key] }, { shouldFocus: !mapped });
            mapped = true;
          }
        }
        if (!mapped) {
          const duplicate =
            error instanceof ApiError && error.code === "EMAIL_ALREADY_EXISTS";
          setError("root", {
            type: duplicate ? "duplicate" : "server",
            message:
              error instanceof Error
                ? error.message
                : "Something went wrong. Try again.",
          });
        }
      }
    } finally {
      submitted.current = false;
      onBusy(false);
    }
  };

  const fieldProps = (field: keyof AuthFields, hint = false) => {
    const id = `${mode}-${field}`;
    return {
      id,
      "aria-invalid": Boolean(errors[field]),
      "aria-describedby":
        [hint && `${id}-hint`, errors[field] && `${id}-error`]
          .filter(Boolean)
          .join(" ") || undefined,
    };
  };

  return (
    <section
      className={`auth-form auth-form--${mode}`}
      inert={!active}
      aria-hidden={!active}
    >
      <div className="auth-form-content">
        <Logo />
        <h1 ref={heading} tabIndex={-1}>
          {title}
        </h1>
        <form noValidate onSubmit={handleSubmit(submit)} aria-label={title}>
          <div className="auth-fields">
            {creating && (
              <FormField
                id={`${mode}-name`}
                label="Full name"
                error={errors.name?.message}
              >
                <Input
                  icon={UserRound}
                  autoComplete="name"
                  {...fieldProps("name")}
                  {...register("name", {
                    onChange: (event) => updateName(event.target.value),
                  })}
                />
              </FormField>
            )}
            <FormField
              id={`${mode}-email`}
              label="Email"
              error={errors.email?.message}
            >
              <Input
                icon={Mail}
                type="email"
                autoComplete={creating ? "email" : "username"}
                {...fieldProps("email")}
                {...register("email", {
                  onChange: (event) => updateEmail(event.target.value),
                })}
              />
            </FormField>
            <FormField
              id={`${mode}-password`}
              label="Password"
              error={errors.password?.message}
              hint={creating ? "At least 8 characters." : undefined}
            >
              <PasswordInput
                key={String(active)}
                autoComplete={creating ? "new-password" : "current-password"}
                {...fieldProps("password", creating)}
                {...register("password")}
              />
            </FormField>
          </div>
          {errors.root?.message && (
            <FormError>
              {errors.root.message}
              {errors.root.type === "duplicate" && (
                <Button variant="secondary" onClick={switchMode}>
                  Sign in instead
                </Button>
              )}
            </FormError>
          )}
          <Button
            type="submit"
            className="auth-submit"
            disabled={busy}
            loading={
              busy && active
                ? creating
                  ? "Creating account…"
                  : "Signing in…"
                : undefined
            }
          >
            {title}
          </Button>
        </form>
      </div>
    </section>
  );
}
