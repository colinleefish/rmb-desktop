import type { IntegrationSetupPanelProps } from "../types";
import { HookRecallVerifySetup } from "../shared/HookRecallVerifySetup";
import { RecallRuleCopy } from "../shared/RecallRuleCopy";
import { useI18n } from "../../i18n";
import rulesGuide from "./assets/rules-guide.png";

export function CursorSetupPanel(props: IntegrationSetupPanelProps) {
  const { t } = useI18n();

  return (
    <HookRecallVerifySetup
      {...props}
      sessionSource="cursor"
      renderRecall={({ artifact }) => (
        <RecallRuleCopy
          artifact={artifact}
          manualHint={t.agents.cursorRulesManualHint}
          contentHint={t.agents.cursorRulesCopyHint}
          guideImage={{
            src: rulesGuide,
            alt: t.agents.cursorRulesGuideAlt,
          }}
        />
      )}
    />
  );
}
