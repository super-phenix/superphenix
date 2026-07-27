package instance

import (
	"encoding/json"
	"strings"
	"testing"
)

func boolPtr(b bool) *bool { return &b }

func advancedBody() *InstanceAdvancedOptionsBody {
	adv := &InstanceAdvancedOptionsBody{}
	adv.Devices.Tpm.Enabled = boolPtr(false) // remove the TPM
	adv.Devices.Tpm.Persistent = boolPtr(false)
	adv.Firmware.Bootloader.Efi.SecureBoot = boolPtr(true)
	return adv
}

// toUpdateAzControllerBody must forward the optional Advanced block verbatim.
func TestToUpdateAzControllerBody_ForwardsAdvanced(t *testing.T) {
	t.Run("nil stays nil", func(t *testing.T) {
		body := UpdateInstanceBody{Advanced: nil}
		got := toUpdateAzControllerBody(body, nil)
		if got.Advanced != nil {
			t.Fatalf("expected nil Advanced, got %+v", got.Advanced)
		}
	})

	t.Run("block is forwarded", func(t *testing.T) {
		body := UpdateInstanceBody{Advanced: advancedBody()}
		got := toUpdateAzControllerBody(body, nil)
		if got.Advanced == nil {
			t.Fatal("expected Advanced to be forwarded, got nil")
		}
		if got.Advanced.Devices.Tpm.Enabled == nil || *got.Advanced.Devices.Tpm.Enabled {
			t.Fatalf("expected tpm.enabled=false forwarded, got %+v", got.Advanced.Devices.Tpm.Enabled)
		}
	})
}

// A non-nil pointer to false must survive JSON marshaling despite omitempty
// (omitempty on a pointer only drops nil), so "disable" intents reach the wire.
func TestInstanceAdvancedOptions_FalseLeafSurvivesMarshal(t *testing.T) {
	b, err := json.Marshal(CreateInstanceSpxControllerBody{Advanced: advancedBody()})
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	out := string(b)
	if !strings.Contains(out, `"enabled":false`) {
		t.Fatalf(`expected "enabled":false in payload, got %s`, out)
	}
	if !strings.Contains(out, `"secureBoot":true`) {
		t.Fatalf(`expected "secureBoot":true in payload, got %s`, out)
	}
}

// When the block is omitted, "advanced" must not appear in the payload at all.
func TestInstanceAdvancedOptions_NilOmitted(t *testing.T) {
	b, err := json.Marshal(CreateInstanceSpxControllerBody{Advanced: nil})
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if strings.Contains(string(b), "advanced") {
		t.Fatalf("expected no advanced key when nil, got %s", string(b))
	}
}
