import { useState } from "react";
import { useTranslation } from "react-i18next";

interface PasswordInputModalProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: (password: string, confirmPassword: string) => void;
}

export function PasswordInputModal({
  isOpen,
  onClose,
  onConfirm,
}: PasswordInputModalProps) {
  const { t } = useTranslation();
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);

  if (!isOpen)
    return null;

  const handleSubmit = () => {
    if (!password) {
      return;
    }
    if (password !== confirmPassword) {
      return;
    }
    onConfirm(password, confirmPassword);
    onClose();
    setPassword("");
    setConfirmPassword("");
    setShowPassword(false);
  };

  const handleClose = () => {
    onClose();
    setPassword("");
    setConfirmPassword("");
    setShowPassword(false);
  };

  const passwordsMatch = password === confirmPassword;
  const canSubmit = password.length >= 6 && passwordsMatch;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
      <div className="w-full max-w-md rounded-xl bg-white p-6 shadow-xl dark:bg-brand-800 border border-brand-200 dark:border-brand-700">
        <div className="flex items-start gap-4 mb-6">
          <div className="p-2 rounded-full bg-warning-100 text-warning-600 dark:bg-warning-900/30 dark:text-warning-400">
            <div className="i-mdi-lock-plus text-2xl" />
          </div>
          <div className="flex-1">
            <h3 className="text-xl font-bold text-brand-900 dark:text-white mb-2">{t("settings.passwordModal.title")}</h3>
            <div className="text-brand-600 dark:text-brand-400 text-sm leading-relaxed space-y-2">
              <p>{t("settings.passwordModal.desc1")}</p>
              <p className="font-semibold text-warning-600 dark:text-warning-400">
                {t("settings.passwordModal.desc2")}
              </p>
              <p>{t("settings.passwordModal.desc3")}</p>
            </div>
          </div>
        </div>

        <div className="space-y-4 mb-6">
          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-2">
              {t("settings.passwordModal.passwordLabel")}
            </label>
            <div className="relative">
              <input
                type={showPassword ? "text" : "password"}
                value={password}
                onChange={e => setPassword(e.target.value)}
                placeholder={t("settings.passwordModal.passwordPlaceholder")}
                className="w-full px-3 py-2 pr-10 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white"
                autoFocus
              />
              <button
                type="button"
                onClick={() => setShowPassword(!showPassword)}
                className="absolute right-2 top-1/2 -translate-y-1/2 text-brand-500 hover:text-brand-700 dark:text-brand-400 dark:hover:text-brand-200"
              >
                <span className={showPassword ? "i-mdi-eye-off text-xl" : "i-mdi-eye text-xl"} />
              </button>
            </div>
            {password && password.length < 6 && (
              <p className="text-xs text-error-600 dark:text-error-400 mt-1">
                {t("settings.passwordModal.passwordTooShort")}
              </p>
            )}
          </div>

          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-2">
              {t("settings.passwordModal.confirmLabel")}
            </label>
            <input
              type={showPassword ? "text" : "password"}
              value={confirmPassword}
              onChange={e => setConfirmPassword(e.target.value)}
              placeholder={t("settings.passwordModal.confirmPlaceholder")}
              className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white"
            />
            {confirmPassword && !passwordsMatch && (
              <p className="text-xs text-error-600 dark:text-error-400 mt-1">
                {t("settings.passwordModal.passwordMismatch")}
              </p>
            )}
          </div>
        </div>

        <div className="flex justify-end gap-3">
          <button
            type="button"
            onClick={handleClose}
            className="px-4 py-2 text-sm font-medium text-brand-700 hover:bg-brand-100 rounded-lg dark:text-brand-300 dark:hover:bg-brand-700 transition-colors"
          >
            {t("common.cancel")}
          </button>
          <button
            type="button"
            onClick={handleSubmit}
            disabled={!canSubmit}
            className="px-4 py-2 text-sm font-medium text-white rounded-lg transition-colors bg-brand-600 hover:bg-brand-700 shadow-sm shadow-brand-200 dark:shadow-none disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {t("common.confirm")}
          </button>
        </div>
      </div>
    </div>
  );
}
