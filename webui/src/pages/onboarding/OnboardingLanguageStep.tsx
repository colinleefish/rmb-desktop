import type { Lang } from "../../i18n/translations";
import { LanguageSelect } from "../../components/LanguageSelect";
import { useI18n } from "../../i18n";

export function OnboardingLanguageStep({ onComplete }: { onComplete: () => void }) {
  const { lang, setLang, t } = useI18n();

  function handleLanguageChange(next: Lang) {
    setLang(next);
  }

  return (
    <div className="onboarding-language-step space-y-6">
      <p className="text-sm text-rmb-gray">{t.onboarding.language.subtitle}</p>

      <div className="rounded-lg border border-[#dadce0] bg-rmb-light/50 px-4 py-4">
        <label id="onboarding-language-label" className="block text-sm font-medium text-rmb-dark">
          {t.settings.language.label}
        </label>
        <div className="mt-2">
          <LanguageSelect
            id="onboarding-language"
            labelId="onboarding-language-label"
            value={lang}
            onChange={handleLanguageChange}
          />
        </div>
      </div>

      <div className="border-t border-rmb-gray/15 pt-6">
        <button
          type="button"
          onClick={onComplete}
          className="rounded-md bg-rmb-accent px-4 py-2 text-sm font-medium text-white hover:bg-rmb-accent/90"
        >
          {t.onboarding.language.continue}
        </button>
      </div>
    </div>
  );
}
