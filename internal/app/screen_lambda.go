package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

const lambdaAPITimeout = 30 * time.Second

// lambdaPayloadSource indicates how the invoke payload is provided.
type lambdaPayloadSource int

const (
	lambdaPayloadManual lambdaPayloadSource = iota
	lambdaPayloadFile
)

// handleLambdaMsg routes Lambda messages to the correct screen.
func (m Model) handleLambdaMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case lambdaFunctionsLoadedMsg:
		m.lambdaFunctions = msg.functions
		m.filteredLambdaFunctions = msg.functions
		m.lambdaFunctionIdx = 0
		m.resetFilter(filterLambdaFunctions)
		m.screen = screenLambdaFunctionList
		return m, nil, true
	case lambdaInvokeResultMsg:
		m.lambdaInvokeResult = msg.result
		m.screen = screenLambdaInvokeResult
		return m, nil, true
	}
	return m, nil, false
}

// --- Function List ---

func (m Model) updateLambdaFunctionList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterLambdaFunctions); handled {
		return m, cmd
	}

	switch msg.String() {
	case "q", "esc":
		m.screen = screenFeatureList
	case "up", "k":
		m.lambdaFunctionIdx = previousListIndex(m.lambdaFunctionIdx, len(m.filteredLambdaFunctions))
	case "down", "j":
		m.lambdaFunctionIdx = nextListIndex(m.lambdaFunctionIdx, len(m.filteredLambdaFunctions))
	case "/":
		return m, m.activateFilter(filterLambdaFunctions)
	case "r":
		return m.startLoading(m.loadLambdaFunctions())
	case "d":
		if len(m.filteredLambdaFunctions) > 0 && m.lambdaFunctionIdx < len(m.filteredLambdaFunctions) {
			fn := m.filteredLambdaFunctions[m.lambdaFunctionIdx]
			m.selectedLambdaFunction = &fn
			m.screen = screenLambdaFunctionDetail
		}
	case "l":
		if len(m.filteredLambdaFunctions) > 0 && m.lambdaFunctionIdx < len(m.filteredLambdaFunctions) {
			fn := m.filteredLambdaFunctions[m.lambdaFunctionIdx]
			m.selectedLambdaFunction = &fn
			return m.cwLogs.StartFromLambda(&m, fn.Name)
		}
	case "enter":
		if len(m.filteredLambdaFunctions) > 0 && m.lambdaFunctionIdx < len(m.filteredLambdaFunctions) {
			fn := m.filteredLambdaFunctions[m.lambdaFunctionIdx]
			m.selectedLambdaFunction = &fn
			m.lambdaInvokePayload = ""
			m.lambdaPayloadSource = lambdaPayloadManual
			m.lambdaInvokeStep = 0
			m.screen = screenLambdaInvokeInput
		}
	}
	return m, nil
}

func (m Model) viewLambdaFunctionList() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Lambda Functions"))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterLambdaFunctions))
	b.WriteString("\n\n")

	if len(m.filteredLambdaFunctions) == 0 {
		panel.WriteString(dimStyle.Render("  No functions found"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-10, 5)
		start := 0
		if m.lambdaFunctionIdx >= visibleLines {
			start = m.lambdaFunctionIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredLambdaFunctions))

		for i := start; i < end; i++ {
			fn := m.filteredLambdaFunctions[i]
			cursor := "  "
			style := normalStyle
			if i == m.lambdaFunctionIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterLambdaFunctions, fn.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d functions", len(m.filteredLambdaFunctions), len(m.lambdaFunctions))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • r: refresh • enter: invoke • d: detail • l: logs • esc: back"))
	return b.String()
}

// --- Function Detail ---

func (m Model) updateLambdaFunctionDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenLambdaFunctionList
	case "i":
		m.lambdaInvokePayload = ""
		m.lambdaPayloadSource = lambdaPayloadManual
		m.lambdaInvokeStep = 0
		m.screen = screenLambdaInvokeInput
	case "l":
		if m.selectedLambdaFunction != nil {
			return m.cwLogs.StartFromLambda(&m, m.selectedLambdaFunction.Name)
		}
	}
	return m, nil
}

func (m Model) viewLambdaFunctionDetail() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())

	fn := m.selectedLambdaFunction
	if fn == nil {
		return b.String()
	}

	b.WriteString(titleStyle.Render(fmt.Sprintf("Lambda — %s", fn.Name)))
	b.WriteString("\n\n")

	b.WriteString(renderDetailLine("Runtime", fn.Runtime))
	b.WriteString(renderDetailLine("Handler", fn.Handler))
	b.WriteString(renderDetailLine("Memory", fmt.Sprintf("%d MB", fn.MemoryMB)))
	b.WriteString(renderDetailLine("Timeout", fmt.Sprintf("%d seconds", fn.TimeoutSec)))
	b.WriteString(renderDetailLine("Code Size", formatBytes(fn.CodeSize)))
	b.WriteString(renderDetailLine("Last Modified", fn.LastModified))
	b.WriteString(renderDetailLine("ARN", fn.ARN))
	b.WriteString(renderDetailLine("Role", fn.Role))

	if fn.Description != "" {
		b.WriteString(renderDetailLine("Description", fn.Description))
	}
	if len(fn.Layers) > 0 {
		b.WriteString(renderDetailLine("Layers", fmt.Sprintf("%d layer(s)", len(fn.Layers))))
		for _, l := range fn.Layers {
			b.WriteString(dimStyle.Render(fmt.Sprintf("                    %s", l)))
			b.WriteString("\n")
		}
	}
	if len(fn.VPCSubnets) > 0 {
		b.WriteString(renderDetailLine("VPC Subnets", strings.Join(fn.VPCSubnets, ", ")))
	}
	if len(fn.VPCSGs) > 0 {
		b.WriteString(renderDetailLine("VPC SGs", strings.Join(fn.VPCSGs, ", ")))
	}

	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("i: invoke • l: logs • esc: back • H: home"))
	return b.String()
}

// --- Invoke Input ---
// Step 0: select payload source (manual / file)
// Step 1: type payload or file path, then invoke

func (m Model) updateLambdaInvokeInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Step 0: source selection
	if m.lambdaInvokeStep == 0 {
		switch key {
		case "esc":
			m.screen = screenLambdaFunctionList
		case "up", "k":
			m.lambdaPayloadSource = lambdaPayloadSource(previousListIndex(int(m.lambdaPayloadSource), 2))
		case "down", "j":
			m.lambdaPayloadSource = lambdaPayloadSource(nextListIndex(int(m.lambdaPayloadSource), 2))
		case "enter":
			m.lambdaInvokeStep = 1
			m.lambdaInvokePayload = ""
		}
		return m, nil
	}

	// Step 1: text input
	switch key {
	case "esc":
		m.lambdaInvokeStep = 0
	case "enter":
		if m.selectedLambdaFunction != nil {
			return m.startLoading(m.invokeLambdaFunction())
		}
	case "backspace":
		if len(m.lambdaInvokePayload) > 0 {
			m.lambdaInvokePayload = m.lambdaInvokePayload[:len(m.lambdaInvokePayload)-1]
		}
	default:
		if len(key) == 1 || key == " " {
			m.lambdaInvokePayload += key
		}
	}
	return m, nil
}

func (m Model) viewLambdaInvokeInput() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())

	fnName := ""
	if m.selectedLambdaFunction != nil {
		fnName = m.selectedLambdaFunction.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("Invoke — %s", fnName)))
	b.WriteString("\n\n")

	if m.lambdaInvokeStep == 0 {
		b.WriteString(normalStyle.Render("  Select payload source:"))
		b.WriteString("\n\n")
		sources := []struct {
			src   lambdaPayloadSource
			label string
		}{
			{lambdaPayloadManual, "Manual input (type JSON directly)"},
			{lambdaPayloadFile, "File path (load JSON from local file)"},
		}
		for _, s := range sources {
			cursor := "  "
			style := normalStyle
			if m.lambdaPayloadSource == s.src {
				cursor = "> "
				style = selectedStyle
			}
			b.WriteString(style.Render(cursor + s.label))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(m.renderHelpBar("↑/↓: select source • enter: confirm • esc: back"))
	} else {
		label := "Payload (JSON):"
		hint := "(empty — press enter to invoke with no payload)"
		if m.lambdaPayloadSource == lambdaPayloadFile {
			label = "File path:"
			hint = "(enter path to a local JSON file)"
		}
		b.WriteString(normalStyle.Render("  " + label))
		b.WriteString("\n")
		payload := m.lambdaInvokePayload
		if payload == "" {
			payload = dimStyle.Render(hint)
		}
		b.WriteString(normalStyle.Render("  " + payload))
		b.WriteString("\n\n")
		b.WriteString(m.renderHelpBar("type: edit • enter: invoke • esc: back to source"))
	}

	return b.String()
}

// --- Invoke Result ---

func (m Model) updateLambdaInvokeResult(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenLambdaFunctionList
	case "i":
		m.lambdaInvokePayload = ""
		m.lambdaPayloadSource = lambdaPayloadManual
		m.lambdaInvokeStep = 0
		m.screen = screenLambdaInvokeInput
	case "l":
		if m.selectedLambdaFunction != nil {
			return m.cwLogs.StartFromLambda(&m, m.selectedLambdaFunction.Name)
		}
	}
	return m, nil
}

func (m Model) viewLambdaInvokeResult() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())

	fnName := ""
	if m.selectedLambdaFunction != nil {
		fnName = m.selectedLambdaFunction.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("Invoke Result — %s", fnName)))
	b.WriteString("\n\n")

	r := m.lambdaInvokeResult
	if r == nil {
		return b.String()
	}

	b.WriteString(renderDetailLine("Status Code", fmt.Sprintf("%d", r.StatusCode)))
	if r.FunctionError != "" {
		b.WriteString(renderDetailLine("Error", r.FunctionError))
	}
	b.WriteString("\n")

	b.WriteString(normalStyle.Render("  Response:"))
	b.WriteString("\n")
	if r.Payload != "" {
		for _, line := range strings.Split(r.Payload, "\n") {
			b.WriteString(normalStyle.Render("  " + line))
			b.WriteString("\n")
		}
	} else {
		b.WriteString(dimStyle.Render("  (empty)"))
		b.WriteString("\n")
	}

	if r.LogResult != "" {
		b.WriteString("\n")
		b.WriteString(normalStyle.Render("  Execution Log:"))
		b.WriteString("\n")
		for _, line := range strings.Split(r.LogResult, "\n") {
			b.WriteString(dimStyle.Render("  " + line))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("i: invoke again • l: logs • esc: back to list • H: home"))
	return b.String()
}

// --- Load Commands ---

func (m Model) loadLambdaFunctions() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), lambdaAPITimeout)
		defer cancel()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		functions, err := repo.ListFunctions(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		if len(functions) == 0 {
			return errMsg{err: fmt.Errorf("no Lambda functions found in region %s", m.cfg.Region)}
		}
		return lambdaFunctionsLoadedMsg{functions: functions}
	}
}

func (m Model) invokeLambdaFunction() tea.Cmd {
	if m.selectedLambdaFunction == nil {
		return func() tea.Msg {
			return errMsg{err: fmt.Errorf("no function selected")}
		}
	}
	payloadSource := m.lambdaPayloadSource
	payloadInput := m.lambdaInvokePayload
	fnName := m.selectedLambdaFunction.Name

	return func() tea.Msg {
		payload := payloadInput
		if payloadSource == lambdaPayloadFile && payloadInput != "" {
			path := strings.TrimSpace(payloadInput)
			if strings.HasPrefix(path, "~/") {
				if home, err := os.UserHomeDir(); err == nil {
					path = home + path[1:]
				}
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return errMsg{err: fmt.Errorf("failed to read payload file: %w", err)}
			}
			payload = string(data)
		}

		// Validate JSON format if payload is provided
		if payload != "" {
			var jsonCheck interface{}
			if err := json.Unmarshal([]byte(payload), &jsonCheck); err != nil {
				return errMsg{err: fmt.Errorf("invalid JSON payload: %w", err)}
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), lambdaAPITimeout)
		defer cancel()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		result, err := repo.InvokeFunction(ctx, fnName, payload, false)
		if err != nil {
			return errMsg{err: err}
		}
		return lambdaInvokeResultMsg{result: result}
	}
}

// formatBytes formats a byte count into a human-readable string.
func formatBytes(b int64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
