package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"unic/internal/domain"
	awsservice "unic/internal/services/aws"
)

type firstUseGuide struct {
	previous screen
	loading  bool
	identity *awsservice.CallerIdentity
	err      string
	scroll   int
}

type firstUseIdentityMsg struct {
	identity *awsservice.CallerIdentity
	err      error
}

var firstUseRepository = awsservice.NewAwsRepository

func (m Model) openFirstUseGuide() (tea.Model, tea.Cmd) {
	m.helpVisible = false
	m.guide = firstUseGuide{previous: m.screen}
	m.screen = screenFirstUse
	return m, nil
}

func (m Model) updateFirstUseGuide(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		if m.guide.loading && m.commands != nil {
			m.commands.CancelAll()
		}
		m.guide.loading = false
		m.screen = m.guide.previous
	case "a":
		return m.updateContextPicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	case "r":
		if m.cfg == nil {
			return m, nil
		}
		m.guide.loading = true
		m.guide.identity = nil
		m.guide.err = ""
		cfg := *m.cfg
		generation := 0
		if m.commands != nil {
			generation = m.commands.Renew()
		}
		baseCtx := m.commandContext()
		cmd := func() tea.Msg {
			ctx, cancel := context.WithTimeout(baseCtx, 10*time.Second)
			defer cancel()
			repo, err := firstUseRepository(ctx, &cfg)
			if err != nil {
				return firstUseIdentityMsg{err: err}
			}
			identity, err := repo.GetCallerIdentity(ctx)
			return firstUseIdentityMsg{identity: identity, err: err}
		}
		if m.commands != nil {
			cmd = m.commands.BindCmd(generation, cmd)
		}
		return m, cmd
	case "e":
		if m.guide.identity == nil || m.guide.loading {
			return m, nil
		}
		m.callerIdentity = m.guide.identity
		m.enterServiceForPalette(paletteItem{service: domain.ServiceECS, feature: domain.FeatureECSExec})
		return m.startFeature(domain.FeatureECSExec)
	case "up", "k":
		m.guide.scroll = max(m.guide.scroll-1, 0)
	case "down", "j":
		m.guide.scroll++
	case "pgup":
		m.guide.scroll = max(m.guide.scroll-max(m.height-7, 5), 0)
	case "pgdown":
		m.guide.scroll += max(m.height-7, 5)
	}
	return m, nil
}

func recoveryGuidance(message string) string {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "accessdenied") || strings.Contains(lower, "access denied") || strings.Contains(lower, "not authorized"):
		return "Access denied: ask your administrator for the permission named in the error. Do not treat denial as an empty resource list."
	case strings.Contains(lower, "sso") || strings.Contains(lower, "expired") || strings.Contains(lower, "token"):
		return "Authentication expired/unavailable: sign in again with aws sso login --profile <your-profile> (for SSO), then reselect the context with C."
	default:
		return "Check configuration and credentials with unic doctor --profile <your-profile> --region <resource-region>. Select a context with C, or add one with a in the picker."
	}
}

func (m Model) firstUseLines() []string {
	lines := []string{"1. Choose your AWS scope", "Press C to select an existing context; press a to reuse the Add Context wizard.",
		"Credential contexts reuse an existing AWS profile; directly configured SSO uses the existing login flow.",
		"For an SSO profile, sign in with aws sso login --profile <your-profile> first.",
		"CLI-selected profile/region and existing authentication settings are retained; no new config is saved here.", "",
		"2. Resolve and review your live identity", "Press r to check the configured credentials using STS (10-second limit)."}
	if m.cfg != nil {
		lines = append(lines, renderDetailLine("Context", zeroFallback(m.cfg.ContextName, "one-off/default")), renderDetailLine("Profile", zeroFallback(m.cfg.Profile, "default credential chain")), renderDetailLine("Resource region", zeroFallback(m.cfg.Region, "not configured")))
	}
	if m.guide.loading {
		lines = append(lines, "Resolving identity... Esc cancels.")
	}
	if m.guide.identity != nil {
		lines = append(lines, renderDetailLine("Resolved account", m.guide.identity.Account), renderDetailLine("Resolved principal/role", m.guide.identity.Arn), "Review this account/principal and region before pressing e. Credentials are never shown.")
	}
	if m.guide.err != "" {
		lines = append(lines, errorStyle.Render(m.guide.err), recoveryGuidance(m.guide.err))
	}
	lines = append(lines, "", "3. Find your first resource", "After identity verification, press e to open ECS clusters, then select a cluster and service with Enter.",
		"Use / to filter. A successful empty list is different from access denied: verify account, region, and list permissions.",
		"Use R to select a configured resource region. Use P for other browsers.", "",
		"No AWS access? Quit with Ctrl+C and run unic demo for a synthetic failed ECS rollout.",
		"Skip with Esc/q and continue your normal workflow. Reopen with G from the context picker, catalog, feature list, or error screen.")
	return lines
}

func (m Model) viewFirstUseGuide() string {
	var lines []string
	width := max(m.width-4, 20)
	for _, line := range m.firstUseLines() {
		lines = append(lines, strings.Split(ansi.Hardwrap(line, width, true), "\n")...)
	}
	visibleLines := max(m.height-7, 5)
	start := min(m.guide.scroll, max(len(lines)-visibleLines, 0))
	end := min(start+visibleLines, len(lines))
	return titleStyle.Render("First useful AWS result") + "\n\n" + strings.Join(lines[start:end], "\n") + fmt.Sprintf("\n\n%d-%d/%d\n", start+1, end, len(lines)) + m.renderHelpBar("↑/↓: scroll • r: verify • e: ECS • a: add context • C: contexts • esc/q: skip")
}
