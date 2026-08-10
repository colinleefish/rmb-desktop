package integrations_test

import (
	"strings"
	"testing"

	"github.com/colinleefish/rmb-desktop/internal/setup/integrations"
)

func TestRenderOpenCodePlugin_injectsBinaryPath(t *testing.T) {
	const bin = "/Users/jane/.rmb/bin/rmb"
	got, err := integrations.RenderOpenCodePlugin(bin)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "__RMB_HOOK_BIN_JSON__") {
		t.Fatal("placeholder was not replaced")
	}
	if !strings.Contains(got, `"/Users/jane/.rmb/bin/rmb"`) {
		t.Fatalf("expected injected path in output: %s", got)
	}
	if strings.Contains(got, "colinleefish/rmb") {
		t.Fatal("must not contain dev repo path")
	}
	if strings.Contains(got, "make build") {
		t.Fatal("must not contain make build fallback")
	}
}

func TestIsRMBOpenCodePlugin(t *testing.T) {
	if !integrations.IsRMBOpenCodePlugin(integrations.OpenCodePluginTemplate()) {
		t.Fatal("expected embedded template to match plugin marker")
	}
	if integrations.IsRMBOpenCodePlugin("random file") {
		t.Fatal("unexpected match for unrelated content")
	}
}

func TestRenderPiExtension_injectsBinaryPath(t *testing.T) {
	const bin = "/Users/jane/.rmb/bin/rmb"
	got, err := integrations.RenderPiExtension(bin)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "__RMB_HOOK_BIN_JSON__") {
		t.Fatal("placeholder was not replaced")
	}
	if !strings.Contains(got, `"/Users/jane/.rmb/bin/rmb"`) {
		t.Fatalf("expected injected path in output: %s", got)
	}
	if !integrations.IsRMBPiExtension(got) {
		t.Fatal("rendered extension missing rmb markers")
	}
}

func TestIsRMBPiExtension(t *testing.T) {
	if !integrations.IsRMBPiExtension(integrations.PiExtensionTemplate()) {
		t.Fatal("expected embedded template to match extension marker")
	}
	if integrations.IsRMBPiExtension("random file") {
		t.Fatal("unexpected match for unrelated content")
	}
}
