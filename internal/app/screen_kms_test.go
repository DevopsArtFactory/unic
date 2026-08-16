package app

import (
	"strings"
	"testing"
	awsservice "unic/internal/services/aws"
)

func TestKMSKeysLoadedRendersPostureAndDetail(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.kms.HandleMessage(&m, kmsKeysLoadedMsg{keys: []awsservice.KMSKey{{ID: "key-1", Aliases: []string{"alias/app"}, State: "Enabled", Manager: "CUSTOMER", RotationEnabled: true}}})
	view, ok := m.kms.View(m)
	if !ok || !strings.Contains(view, "alias/app") || !strings.Contains(view, "true") {
		t.Fatalf("unexpected list: %s", view)
	}
	m.kms.idx = 0
	m.screen = screenKMSKeyDetail
	m.kms.selected = &m.kms.filtered[0]
	view, _ = m.kms.View(m)
	for _, want := range []string{"key-1", "alias/app", "Rotation Enabled", "true"} {
		if !strings.Contains(view, want) {
			t.Fatalf("detail missing %q: %s", want, view)
		}
	}
}
