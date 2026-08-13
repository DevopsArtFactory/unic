package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
)

func TestECRLoginResolvedMsgOpensHelperScreen(t *testing.T) {
	m := Model{
		cfg:    &config.Config{Region: "ap-northeast-2"},
		screen: screenLoading,
	}

	msg := ecrLoginResolvedMsg{
		registryURI:   "123456789012.dkr.ecr.ap-northeast-2.amazonaws.com",
		dockerCommand: "aws ecr get-login-password --region ap-northeast-2 | docker login ...",
		podmanCommand: "aws ecr get-login-password --region ap-northeast-2 | podman login ...",
	}
	_, _, handled := m.ecr.HandleMessage(&m, msg)
	if !handled {
		t.Fatal("expected login-resolved message to be handled")
	}
	if m.screen != screenECRLoginHelper {
		t.Fatalf("expected login helper screen, got %v", m.screen)
	}

	view, ok := m.ecr.View(m)
	if !ok {
		t.Fatal("expected login helper view to render")
	}
	for _, want := range []string{"ECR Login Helper", msg.registryURI, "docker login", "podman login"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got:\n%s", want, view)
		}
	}
}

func TestECRLoginHelperEscReturnsToFeatureList(t *testing.T) {
	m := Model{
		cfg:    &config.Config{Region: "ap-northeast-2"},
		screen: screenECRLoginHelper,
	}
	m.ecr.copyMsg = "stale"

	_, _, handled := m.ecr.HandleKey(&m, tea.KeyMsg{Type: tea.KeyEsc})
	if !handled {
		t.Fatal("expected key to be handled on the login helper screen")
	}
	if m.screen != screenFeatureList {
		t.Fatalf("expected feature list screen, got %v", m.screen)
	}
	if m.ecr.copyMsg != "" {
		t.Fatalf("expected copy message to be cleared, got %q", m.ecr.copyMsg)
	}
}
