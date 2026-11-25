import { useState } from "react";
import { useSearchParams } from "react-router";
import { AuthForm } from "../../features/auth/AuthForm";
import { Button } from "../../shared/ui/controls";
import "./auth.css";

export function AuthPage() {
  const [params, setParams] = useSearchParams();
  const mode = params.get("mode") === "register" ? "register" : "login";
  const [email, updateEmail] = useState("");
  const [fullName, updateName] = useState("");
  const [busy, onBusy] = useState(false);
  const switchMode = () =>
    setParams(mode === "login" ? { mode: "register" } : {});
  const props = {
    email,
    fullName,
    updateEmail,
    updateName,
    switchMode,
    onBusy,
    busy,
  };

  return (
    <main className="auth-screen">
      <div className="auth-layout" data-mode={mode}>
        <AuthForm mode="login" active={mode === "login"} {...props} />
        <AuthForm mode="register" active={mode === "register"} {...props} />
        <aside className="auth-invitation">
          <div
            className="invitation-copy"
            inert={mode !== "login"}
            aria-hidden={mode !== "login"}
          >
            <h2>New here?</h2>
            <p>Create your personal board.</p>
            <Button variant="outline" disabled={busy} onClick={switchMode}>
              Create account
            </Button>
          </div>
          <div
            className="invitation-copy"
            inert={mode !== "register"}
            aria-hidden={mode !== "register"}
          >
            <h2>Already have an account?</h2>
            <Button variant="outline" disabled={busy} onClick={switchMode}>
              Sign in
            </Button>
          </div>
        </aside>
      </div>
    </main>
  );
}
