package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const bootupFrameCount = 28

func (m Model) viewBootup() string {
	frame := clampListIndex(m.bootFrame, bootupFrameCount)
	logo := bootupLogo(frame)
	lines := []string{
		"",
		dimStyle.Render(bootupBanner(m.currentVersion)),
		"",
		logo,
		"",
		bootupProgress(frame),
	}
	lines = append(lines, bootupDiagnostics(frame)...)
	lines = append(lines,
		"",
		dimStyle.Render("enter/esc/space: skip  q: quit"),
	)

	content := strings.Join(lines, "\n")
	if m.width > 0 {
		content = lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(content)
	}
	return m.centerBootupVertically(content)
}

// bootupBanner builds the BIOS header line, reflecting the running app version
// instead of a hardcoded one. Empty/unknown versions fall back to "dev".
func bootupBanner(version string) string {
	v := strings.TrimSpace(version)
	if v == "" {
		v = "dev"
	}
	if v != "dev" && !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return fmt.Sprintf("UNIC BIOS %s   COPYRIGHT 1986-2026 DEVOPS ART FACTORY", v)
}

func (m Model) centerBootupVertically(content string) string {
	if m.height <= 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) >= m.height {
		return strings.Join(lines, "\n")
	}

	topPadding := (m.height - len(lines)) / 2
	centered := make([]string, 0, topPadding+len(lines))
	for i := 0; i < topPadding; i++ {
		centered = append(centered, "")
	}
	centered = append(centered, lines...)
	return strings.Join(centered, "\n")
}

func bootupLogo(frame int) string {
	logo := []string{
		"██   ██ ███  ██ ██  ██████",
		"██   ██ ████ ██ ██ ██     ",
		"██   ██ ██ ████ ██ ██     ",
		"██   ██ ██  ███ ██ ██     ",
		" █████  ██   ██ ██  ██████",
	}

	progress := float64(frame) / float64(bootupFrameCount-1)
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	rendered := make([]string, 0, len(logo))
	for i, line := range logo {
		rendered = append(rendered, bootupFillLine(line, progress, i))
	}
	return strings.Join(rendered, "\n")
}

func bootupFillLine(line string, progress float64, row int) string {
	runes := []rune(line)
	width := len(runes)
	fillWidth := int(progress*float64(width+6)) - row
	if fillWidth < 0 {
		fillWidth = 0
	}
	if fillWidth > width {
		fillWidth = width
	}

	var b strings.Builder
	for i, r := range runes {
		ch := string(r)
		switch {
		case i < fillWidth:
			b.WriteString(titleStyle.Render(ch))
		case i < fillWidth+2 && r != ' ':
			b.WriteString(normalStyle.Render(ch))
		default:
			b.WriteString(dimStyle.Render(ch))
		}
	}
	return b.String()
}

func bootupProgress(frame int) string {
	total := 24
	filled := frame * total / (bootupFrameCount - 1)
	if filled > total {
		filled = total
	}
	bar := strings.Repeat("#", filled) + strings.Repeat("-", total-filled)
	status := "SEEKING AWS SECTOR"
	if frame > 16 {
		status = "CONTEXT BUS READY"
	}
	if frame >= bootupFrameCount-1 {
		status = "READY"
	}
	return fmt.Sprintf("%s  %s", successStyle.Render("["+bar+"]"), warningStyle.Render(status))
}

func bootupDiagnostics(frame int) []string {
	steps := []string{
		"memcheck: 640K operational intent",
		"context rom: mounted",
		"aws bus: listening",
		"operator console: online",
	}

	lines := make([]string, 0, len(steps))
	for i, step := range steps {
		if frame > 7+i*4 {
			lines = append(lines, "  "+successStyle.Render("OK")+"  "+normalStyle.Render(step))
			continue
		}
		lines = append(lines, "  "+dimStyle.Render("--")+"  "+dimStyle.Render(step))
	}
	return lines
}
