import { useEffect, useState } from "react";
import { Navigate, useLocation, useNavigate } from "react-router-dom";
import { getConfig, putConfig } from "../lib/api";
import type { ConfigUpdateRequest, ConfigView } from "../lib/types";
import { useI18n } from "../i18n";
import { LanguageSelect } from "../components/LanguageSelect";
import { SelectMenu } from "../components/SelectMenu";
import { ConfiguredApiKeyField } from "../components/ConfiguredApiKeyField";
import { EmbedDimensionsSelect } from "../components/EmbedDimensionsSelect";
import { Modal } from "../components/Modal";
import { DEFAULT_PIPELINE } from "../lib/pipelineDefaults";
import {
  DEFAULT_EMBED_DIMENSION,
  normalizeEmbedDimension,
  type EmbedDimension,
} from "../lib/embedDimensions";
import { durationToSeconds, secondsToDuration } from "../lib/pipelineDuration";
import { parseSettingsPath, settingsPath, type SettingsSection } from "../lib/settingsRoutes";

function embedSettingsChanged(
  saved: ConfigView["embed"],
  embedBase: string,
  embedModel: string,
  embedDims: EmbedDimension,
): boolean {
  return (
    saved.api_base.trim() !== embedBase.trim() ||
    saved.model.trim() !== embedModel.trim() ||
    normalizeEmbedDimension(saved.dimensions) !== embedDims
  );
}

export function SettingsPage() {
  const { t, lang, setLang } = useI18n();
  const location = useLocation();
  const navigate = useNavigate();
  const { section: tab } = parseSettingsPath(location.pathname);

  const [config, setConfig] = useState<ConfigView | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [showEmbedConfirm, setShowEmbedConfirm] = useState(false);

  const [addr, setAddr] = useState("");
  const [launchAtLogin, setLaunchAtLogin] = useState(false);
  const [llmBase, setLlmBase] = useState("");
  const [llmKey, setLlmKey] = useState("");
  const [llmModel, setLlmModel] = useState("");
  const [embedBase, setEmbedBase] = useState("");
  const [embedKey, setEmbedKey] = useState("");
  const [embedModel, setEmbedModel] = useState("");
  const [embedDims, setEmbedDims] = useState<EmbedDimension>(DEFAULT_EMBED_DIMENSION);
  const [pipeline, setPipeline] = useState<ConfigView["pipeline"] | null>(null);

  useEffect(() => {
    getConfig()
      .then((c) => {
        setConfig(c);
        setAddr(c.addr);
        setLaunchAtLogin(c.launch_at_login);
        setLlmBase(c.llm.api_base);
        setLlmModel(c.llm.model);
        setEmbedBase(c.embed.api_base);
        setEmbedModel(c.embed.model);
        setEmbedDims(normalizeEmbedDimension(c.embed.dimensions));
        setPipeline(c.pipeline);
      })
      .catch((err: Error) => setError(err.message));
  }, []);

  if (location.pathname === "/settings" || location.pathname === "/settings/") {
    return <Navigate to="/settings/general" replace />;
  }
  if (location.pathname === "/settings/language") {
    return <Navigate to="/settings/general" replace />;
  }

  async function performSave() {
    if (!config || !pipeline) return;
    setSaving(true);
    setError(null);
    setMessage(null);
    const update: ConfigUpdateRequest = {
      addr,
      launch_at_login: launchAtLogin,
      llm: {
        api_base: llmBase,
        model: llmModel,
        ...(llmKey ? { api_key: llmKey } : {}),
      },
      embed: {
        api_base: embedBase,
        model: embedModel,
        dimensions: embedDims,
        ...(embedKey ? { api_key: embedKey } : {}),
      },
      pipeline,
    };
    try {
      const res = await putConfig(update);
      setConfig(res.config);
      setLaunchAtLogin(res.config.launch_at_login);
      setLlmKey("");
      setEmbedKey("");
      setMessage(res.message);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
      setShowEmbedConfirm(false);
    }
  }

  async function handleSave() {
    if (!config || !pipeline) return;
    if (embedSettingsChanged(config.embed, embedBase, embedModel, embedDims)) {
      setShowEmbedConfirm(true);
      return;
    }
    await performSave();
  }

  const tabs: { id: SettingsSection; label: string }[] = [
    { id: "general", label: t.settings.tabs.general },
    { id: "models", label: t.settings.tabs.models },
    { id: "advanced", label: t.settings.tabs.advanced },
  ];

  function selectTab(next: SettingsSection) {
    setMessage(null);
    setError(null);
    navigate(settingsPath(next), { replace: true });
  }

  if (!config || !pipeline) {
    return <p className="text-rmb-gray">{t.common.loading}</p>;
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">{t.settings.title}</h1>
        <p className="mt-1 text-rmb-gray">{t.settings.subtitle}</p>
      </div>

      <div className="flex gap-2 border-b border-rmb-gray/20">
        {tabs.map((item) => (
          <button
            key={item.id}
            type="button"
            onClick={() => selectTab(item.id)}
            className={`border-b-2 px-3 py-2 text-sm font-medium transition ${
              tab === item.id
                ? "border-rmb-accent text-rmb-dark"
                : "border-transparent text-rmb-gray hover:text-rmb-dark"
            }`}
          >
            {item.label}
          </button>
        ))}
      </div>

      <div className="overflow-hidden rounded-xl border border-rmb-gray/35 bg-white shadow-sm">
        <div className="p-6">
        {tab === "general" && config && pipeline && (
          <div className="space-y-4 max-w-xl">
            <Field label={t.settings.general.addr} value={addr} onChange={setAddr} />
            <ReadOnly label={t.settings.general.dbPath} value={config.db_path} />
            <ReadOnly label={t.settings.general.configPath} value={config.config_path} />
            <div>
              <div className="text-sm font-medium text-rmb-gray">{t.settings.general.distillation}</div>
              <p className="mt-1 text-sm text-rmb-gray">
                {config.distillation_enabled
                  ? t.settings.general.distillationOn
                  : t.settings.general.distillationOff}
              </p>
            </div>
            <div className="border-t border-rmb-gray/15 pt-4">
              <label id="settings-launch-at-login-label" className="block text-sm font-medium text-rmb-gray">
                {t.settings.general.launchAtLogin}
              </label>
              <div className="mt-1 max-w-xs">
                <SelectMenu
                  id="settings-launch-at-login"
                  labelId="settings-launch-at-login-label"
                  value={launchAtLogin ? "enabled" : "disabled"}
                  onChange={(next) => setLaunchAtLogin(next === "enabled")}
                  options={[
                    { value: "enabled", label: t.settings.general.launchAtLoginEnabled },
                    { value: "disabled", label: t.settings.general.launchAtLoginDisabled },
                  ]}
                />
              </div>
              <p className="mt-1.5 text-xs text-rmb-gray">{t.settings.general.launchAtLoginHint}</p>
            </div>
            <div className="border-t border-rmb-gray/15 pt-4">
              <label id="settings-language-label" className="block text-sm font-medium text-rmb-gray">
                {t.settings.language.label}
              </label>
              <div className="mt-1 max-w-xs">
                <LanguageSelect
                  id="settings-language"
                  labelId="settings-language-label"
                  value={lang}
                  onChange={setLang}
                />
              </div>
            </div>
          </div>
        )}

        {tab === "models" && config && (
          <div className="max-w-xl space-y-8">
            <p className="text-sm text-rmb-gray">{t.settings.models.intro}</p>
            <section className="space-y-4">
              <h3 className="text-sm font-semibold text-rmb-dark">{t.settings.models.llm}</h3>
              <Field
                label={t.settings.llm.apiBase}
                value={llmBase}
                onChange={setLlmBase}
                placeholder={t.settings.llm.apiBasePlaceholder}
              />
              <Field label={t.settings.llm.model} value={llmModel} onChange={setLlmModel} />
              <ConfiguredApiKeyField
                label={t.settings.llm.apiKey}
                configured={config.llm.api_key_set}
                value={llmKey}
                onChange={setLlmKey}
                emptyPlaceholder={t.settings.llm.apiKeyPlaceholder}
                replacePlaceholder={t.settings.llm.apiKeyReplacePlaceholder}
              />
            </section>

            <section className="space-y-4 border-t border-rmb-gray/15 pt-8">
              <h3 className="text-sm font-semibold text-rmb-dark">{t.settings.models.embed}</h3>
              <p className="text-xs text-rmb-gray">{t.settings.embed.reembedNotice}</p>
              <Field
                label={t.settings.embed.apiBase}
                value={embedBase}
                onChange={setEmbedBase}
                placeholder={t.settings.embed.apiBasePlaceholder}
              />
              <Field label={t.settings.embed.model} value={embedModel} onChange={setEmbedModel} />
              <div>
                <label
                  id="settings-embed-dimensions-label"
                  className="block text-sm font-medium text-rmb-gray"
                >
                  {t.settings.embed.dimensions}
                </label>
                <div className="mt-1 max-w-xs">
                  <EmbedDimensionsSelect
                    id="settings-embed-dimensions"
                    labelId="settings-embed-dimensions-label"
                    value={embedDims}
                    onChange={setEmbedDims}
                  />
                </div>
              </div>
              <ConfiguredApiKeyField
                label={t.settings.embed.apiKey}
                configured={config.embed.api_key_set}
                value={embedKey}
                onChange={setEmbedKey}
                emptyPlaceholder={t.settings.embed.apiKeyPlaceholder}
                replacePlaceholder={t.settings.embed.apiKeyReplacePlaceholder}
              />
            </section>
          </div>
        )}

        {tab === "advanced" && pipeline && (
          <div className="max-w-xl space-y-4">
            <p className="text-sm text-rmb-gray">{t.settings.advanced.pipelineIntro}</p>
            <PipelineSecondsField label={t.settings.pipeline.l1Poll} field="l1_poll_interval" pipeline={pipeline} setPipeline={setPipeline} />
            <PipelineSecondsField label={t.settings.pipeline.l2Poll} field="l2_poll_interval" pipeline={pipeline} setPipeline={setPipeline} />
            <PipelineSecondsField label={t.settings.pipeline.l3Poll} field="l3_poll_interval" pipeline={pipeline} setPipeline={setPipeline} />
            <PipelineSecondsField label={t.settings.pipeline.embedPoll} field="embed_poll_interval" pipeline={pipeline} setPipeline={setPipeline} />
            <PipelineSecondsField label={t.settings.pipeline.l1Idle} field="l1_idle_seconds" pipeline={pipeline} setPipeline={setPipeline} />
            <PipelineSecondsField label={t.settings.pipeline.l2Delay} field="l2_delay_after_l1" pipeline={pipeline} setPipeline={setPipeline} />
            <PipelineNumField label={t.settings.pipeline.l1EveryN} field="l1_every_n" pipeline={pipeline} setPipeline={setPipeline} />
            <PipelineNumField label={t.settings.pipeline.l1MaxTurns} field="l1_max_turns_per_batch" pipeline={pipeline} setPipeline={setPipeline} />
            <PipelineNumField label={t.settings.pipeline.l1MaxChars} field="l1_max_chars_per_batch" pipeline={pipeline} setPipeline={setPipeline} />
            <PipelineNumField label={t.settings.pipeline.l2MaxAtoms} field="l2_max_atoms_per_batch" pipeline={pipeline} setPipeline={setPipeline} />
            <PipelineNumField label={t.settings.pipeline.l3MaxAtoms} field="l3_max_atoms_per_batch" pipeline={pipeline} setPipeline={setPipeline} />
            <PipelineNumField label={t.settings.pipeline.embedBatch} field="embed_batch_size" pipeline={pipeline} setPipeline={setPipeline} />
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={pipeline.l1_warmup}
                onChange={(e) => setPipeline({ ...pipeline, l1_warmup: e.target.checked })}
              />
              {t.settings.pipeline.l1Warmup}
            </label>
            <button
              type="button"
              onClick={() => setPipeline({ ...DEFAULT_PIPELINE })}
              className="rounded-md border border-rmb-gray/20 bg-white px-3 py-1.5 text-sm font-medium text-rmb-dark hover:bg-rmb-light"
            >
              {t.settings.advanced.reset}
            </button>
          </div>
        )}

        </div>

        <div className="flex flex-wrap items-center gap-4 border-t border-rmb-gray/15 bg-rmb-light/30 px-6 py-4">
            <button
              type="button"
              onClick={() => void handleSave()}
              disabled={saving}
              className="rounded-md bg-rmb-accent px-4 py-2 text-sm font-medium text-white hover:bg-rmb-accent/90 disabled:opacity-50"
            >
              {saving ? t.settings.saving : t.settings.save}
            </button>
            {message && <p className="text-sm text-emerald-600">{message}</p>}
            {error && <p className="text-sm text-red-600">{error}</p>}
          </div>
      </div>

      <Modal
        open={showEmbedConfirm}
        onClose={() => setShowEmbedConfirm(false)}
        title={t.settings.embed.reembedConfirmTitle}
      >
        <p className="text-sm text-rmb-gray">{t.settings.embed.reembedConfirmBody}</p>
        <div className="mt-6 flex justify-end gap-3">
          <button
            type="button"
            onClick={() => setShowEmbedConfirm(false)}
            disabled={saving}
            className="rounded-md border border-rmb-gray/20 bg-white px-4 py-2 text-sm font-medium text-rmb-dark hover:bg-rmb-light disabled:opacity-50"
          >
            {t.settings.embed.reembedConfirmCancel}
          </button>
          <button
            type="button"
            onClick={() => void performSave()}
            disabled={saving}
            className="rounded-md bg-rmb-accent px-4 py-2 text-sm font-medium text-white hover:bg-rmb-accent/90 disabled:opacity-50"
          >
            {saving ? t.settings.saving : t.settings.embed.reembedConfirmAction}
          </button>
        </div>
      </Modal>
    </div>
  );
}

function Field({
  label,
  value,
  onChange,
  type = "text",
  min,
  placeholder,
  compact,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  type?: "text" | "number";
  min?: number;
  placeholder?: string;
  compact?: boolean;
}) {
  return (
    <div>
      <label className="block text-sm font-medium text-rmb-gray">{label}</label>
      <input
        type={type}
        min={min}
        inputMode={type === "number" ? "numeric" : undefined}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className={`mt-1 rounded-md border border-rmb-gray/20 px-3 py-2 text-sm placeholder:text-rmb-gray/45 ${
          compact ? "w-28" : "w-full"
        } ${
          type === "number"
            ? "[appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
            : ""
        }`}
      />
    </div>
  );
}

function ReadOnly({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-sm font-medium text-rmb-gray">{label}</div>
      <p className="mt-1 break-all font-mono text-xs text-rmb-gray">{value}</p>
    </div>
  );
}

function PipelineSecondsField({
  label,
  field,
  pipeline,
  setPipeline,
}: {
  label: string;
  field: keyof ConfigView["pipeline"];
  pipeline: ConfigView["pipeline"];
  setPipeline: (p: ConfigView["pipeline"]) => void;
}) {
  const raw = pipeline[field];
  if (typeof raw !== "string") return null;
  const seconds = durationToSeconds(raw);
  return (
    <Field
      label={label}
      value={String(seconds)}
      onChange={(v) =>
        setPipeline({ ...pipeline, [field]: secondsToDuration(Number(v) || 0) })
      }
      type="number"
      min={0}
      compact
    />
  );
}

function PipelineNumField({
  label,
  field,
  pipeline,
  setPipeline,
}: {
  label: string;
  field: keyof ConfigView["pipeline"];
  pipeline: ConfigView["pipeline"];
  setPipeline: (p: ConfigView["pipeline"]) => void;
}) {
  const raw = pipeline[field];
  if (typeof raw !== "number") return null;
  return (
    <Field
      label={label}
      value={String(raw)}
      onChange={(v) => setPipeline({ ...pipeline, [field]: Number(v) || 0 })}
      type="number"
      min={0}
      compact
    />
  );
}
