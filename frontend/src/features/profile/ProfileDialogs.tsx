import type { ReactNode } from "react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { X } from "lucide-react";
import { useSession } from "../auth/context";
import {
  currentPasswordSchema,
  nameSchema,
  newPasswordSchema,
} from "../auth/validation";
import type { User } from "../auth/session";
import { ApiError } from "../../shared/api/client";
import {
  Button,
  FormError,
  FormField,
  Input,
  PasswordInput,
} from "../../shared/ui/controls";
import { useModal } from "../../shared/ui/useModal";

function ProfileDialog({
  title,
  onClose,
  busy,
  children,
}: {
  title: string;
  onClose: () => void;
  busy: boolean;
  children: ReactNode;
}) {
  const dialog = useModal("[data-initial-focus]");
  function close() {
    if (!busy) onClose();
  }
  return (
    <dialog
      className="task-dialog profile-dialog"
      ref={dialog}
      aria-labelledby="profile-dialog-title"
      onCancel={(event) => {
        event.preventDefault();
        close();
      }}
      onClick={(event) => {
        if (event.target === dialog.current) close();
      }}
    >
      <div className="task-dialog__panel">
        <header className="task-dialog__header">
          <h2 id="profile-dialog-title">{title}</h2>
          <button
            type="button"
            className="icon-button"
            aria-label="Close dialog"
            onClick={close}
            disabled={busy}
          >
            <X size={20} aria-hidden="true" />
          </button>
        </header>
        {children}
      </div>
    </dialog>
  );
}

function Actions({
  onClose,
  busy,
  submit,
}: {
  onClose: () => void;
  busy: boolean;
  submit: string;
}) {
  return (
    <footer className="task-dialog__footer">
      <Button
        variant="secondary"
        className="task-cancel"
        onClick={onClose}
        disabled={busy}
      >
        Cancel
      </Button>
      <Button type="submit" loading={busy ? "Saving…" : undefined}>
        {submit}
      </Button>
    </footer>
  );
}

const nameForm = z.object({ name: nameSchema });
const passwordForm = z.object({
  currentPassword: currentPasswordSchema,
  newPassword: newPasswordSchema,
});

export function EditProfileDialog({
  user,
  onClose,
}: {
  user: User;
  onClose: () => void;
}) {
  const { controller } = useSession();
  const [error, setRequestError] = useState("");
  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm({
    resolver: zodResolver(nameForm),
    defaultValues: { name: user.name },
  });
  return (
    <ProfileDialog title="Edit profile" busy={isSubmitting} onClose={onClose}>
      <form
        className="task-form"
        noValidate
        onSubmit={handleSubmit(async ({ name }) => {
          setRequestError("");
          try {
            await controller.updateName(name);
            onClose();
          } catch (failure) {
            if (failure instanceof ApiError && failure.fields.name)
              setError("name", { message: failure.fields.name });
            else
              setRequestError(
                failure instanceof ApiError
                  ? failure.message
                  : "Couldn't update your profile.",
              );
          }
        })}
      >
        <FormField
          id="profile-name"
          label="Full name"
          error={errors.name?.message}
        >
          <Input
            id="profile-name"
            autoComplete="name"
            data-initial-focus=""
            aria-invalid={Boolean(errors.name)}
            aria-describedby={errors.name ? "profile-name-error" : undefined}
            {...register("name")}
          />
        </FormField>
        <FormField id="profile-email" label="Email">
          <Input id="profile-email" type="email" value={user.email} readOnly />
        </FormField>
        {error && <FormError>{error}</FormError>}
        <Actions onClose={onClose} busy={isSubmitting} submit="Save" />
      </form>
    </ProfileDialog>
  );
}

export function ChangePasswordDialog({ onClose }: { onClose: () => void }) {
  const { controller } = useSession();
  const [error, setRequestError] = useState("");
  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm({
    resolver: zodResolver(passwordForm),
    defaultValues: { currentPassword: "", newPassword: "" },
  });
  return (
    <ProfileDialog
      title="Change password"
      busy={isSubmitting}
      onClose={onClose}
    >
      <form
        className="task-form profile-password-form"
        noValidate
        onSubmit={handleSubmit(async (passwords) => {
          setRequestError("");
          try {
            await controller.changePassword(passwords);
          } catch (failure) {
            if (failure instanceof ApiError) {
              if (failure.code === "INVALID_CREDENTIALS") {
                setError(
                  "currentPassword",
                  { message: "Invalid current password." },
                  { shouldFocus: true },
                );
                return;
              }
              if (failure.fields.newPassword) {
                setError("newPassword", {
                  message: failure.fields.newPassword,
                });
                return;
              }
            }
            setRequestError(
              failure instanceof ApiError
                ? failure.message
                : "Couldn't change your password.",
            );
          }
        })}
      >
        <FormField
          id="current-password"
          label="Current password"
          error={errors.currentPassword?.message}
        >
          <PasswordInput
            id="current-password"
            autoComplete="current-password"
            toggleLabel="current password"
            data-initial-focus=""
            aria-invalid={Boolean(errors.currentPassword)}
            aria-describedby={
              errors.currentPassword ? "current-password-error" : undefined
            }
            {...register("currentPassword")}
          />
        </FormField>
        <FormField
          id="new-password"
          label="New password"
          hint="At least 8 characters."
          error={errors.newPassword?.message}
        >
          <PasswordInput
            id="new-password"
            autoComplete="new-password"
            toggleLabel="new password"
            aria-invalid={Boolean(errors.newPassword)}
            aria-describedby={
              errors.newPassword
                ? "new-password-hint new-password-error"
                : "new-password-hint"
            }
            {...register("newPassword")}
          />
        </FormField>
        <p className="field-hint">
          You’ll need to sign in again after changing your password.
        </p>
        {error && <FormError>{error}</FormError>}
        <Actions
          onClose={onClose}
          busy={isSubmitting}
          submit="Change password"
        />
      </form>
    </ProfileDialog>
  );
}
