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

type lambdaModel struct {
	functions     []awsservice.LambdaFunction
	filtered      []awsservice.LambdaFunction
	functionIdx   int
	selected      *awsservice.LambdaFunction
	invokePayload string
	invokeResult  *awsservice.LambdaInvokeResult
	payloadSource lambdaPayloadSource
	invokeStep    int // 0=source select, 1=text input
}

func newLambdaModel() lambdaModel {
	return lambdaModel{payloadSource: lambdaPayloadManual}
}

func (lm *lambdaModel) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(lm.loadFunctions(*m))
}

func (lm *lambdaModel) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case lambdaFunctionsLoadedMsg:
		lm.functions = msg.functions
		lm.filtered = msg.functions
		lm.functionIdx = 0
		m.resetFilter(filterLambdaFunctions)
		m.screen = screenLambdaFunctionList
		return *m, nil, true
	case lambdaInvokeResultMsg:
		lm.invokeResult = msg.result
		m.screen = screenLambdaInvokeResult
		return *m, nil, true
	}
	return *m, nil, false
}

func (lm *lambdaModel) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenLambdaFunctionList:
		newM, cmd := lm.updateFunctionList(m, msg)
		return newM, cmd, true
	case screenLambdaFunctionDetail:
		newM, cmd := lm.updateFunctionDetail(m, msg)
		return newM, cmd, true
	case screenLambdaInvokeInput:
		newM, cmd := lm.updateInvokeInput(m, msg)
		return newM, cmd, true
	case screenLambdaInvokeResult:
		newM, cmd := lm.updateInvokeResult(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (lm lambdaModel) View(m Model) (string, bool) {
	switch m.screen {
	case screenLambdaFunctionList:
		return lm.viewFunctionList(m), true
	case screenLambdaFunctionDetail:
		return lm.viewFunctionDetail(m), true
	case screenLambdaInvokeInput:
		return lm.viewInvokeInput(m), true
	case screenLambdaInvokeResult:
		return lm.viewInvokeResult(m), true
	default:
		return "", false
	}
}

func (lm *lambdaModel) ApplyFilter(m *Model, target filterTarget) bool {
	if target != filterLambdaFunctions {
		return false
	}
	lm.filtered = applyFilter(lm.functions, m.filterValue(target))
	lm.functionIdx = 0
	return true
}

// --- Function List ---

func (lm *lambdaModel) updateFunctionList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterLambdaFunctions); handled {
		return *m, cmd
	}

	switch msg.String() {
	case "q", "esc":
		m.screen = screenFeatureList
	case "up", "k":
		lm.functionIdx = previousListIndex(lm.functionIdx, len(lm.filtered))
	case "down", "j":
		lm.functionIdx = nextListIndex(lm.functionIdx, len(lm.filtered))
	case "/":
		return *m, m.activateFilter(filterLambdaFunctions)
	case "r":
		return m.startLoading(lm.loadFunctions(*m))
	case "d":
		if len(lm.filtered) > 0 && lm.functionIdx < len(lm.filtered) {
			fn := lm.filtered[lm.functionIdx]
			lm.selected = &fn
			m.screen = screenLambdaFunctionDetail
		}
	case "l":
		if len(lm.filtered) > 0 && lm.functionIdx < len(lm.filtered) {
			fn := lm.filtered[lm.functionIdx]
			lm.selected = &fn
			return m.cwLogs.StartFromLambda(m, fn.Name)
		}
	case "enter":
		if len(lm.filtered) > 0 && lm.functionIdx < len(lm.filtered) {
			fn := lm.filtered[lm.functionIdx]
			lm.selected = &fn
			lm.invokePayload = ""
			lm.payloadSource = lambdaPayloadManual
			lm.invokeStep = 0
			m.screen = screenLambdaInvokeInput
		}
	}
	return *m, nil
}

func (lm lambdaModel) viewFunctionList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Lambda Functions"))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterLambdaFunctions))
	b.WriteString("\n\n")

	if len(lm.filtered) == 0 {
		panel.WriteString(dimStyle.Render("  No functions found"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-10, 5)
		start := 0
		if lm.functionIdx >= visibleLines {
			start = lm.functionIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(lm.filtered))

		for i := start; i < end; i++ {
			fn := lm.filtered[i]
			cursor := "  "
			style := normalStyle
			if i == lm.functionIdx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterLambdaFunctions, fn.DisplayTitle())))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d functions", len(lm.filtered), len(lm.functions))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • r: refresh • enter: invoke • d: detail • l: logs • esc: back"))
	return b.String()
}

// --- Function Detail ---

func (lm *lambdaModel) updateFunctionDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenLambdaFunctionList
	case "i":
		lm.invokePayload = ""
		lm.payloadSource = lambdaPayloadManual
		lm.invokeStep = 0
		m.screen = screenLambdaInvokeInput
	case "l":
		if lm.selected != nil {
			return m.cwLogs.StartFromLambda(m, lm.selected.Name)
		}
	}
	return *m, nil
}

func (lm lambdaModel) viewFunctionDetail(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())

	fn := lm.selected
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

func (lm *lambdaModel) updateInvokeInput(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Step 0: source selection
	if lm.invokeStep == 0 {
		switch key {
		case "esc":
			m.screen = screenLambdaFunctionList
		case "up", "k":
			lm.payloadSource = lambdaPayloadSource(previousListIndex(int(lm.payloadSource), 2))
		case "down", "j":
			lm.payloadSource = lambdaPayloadSource(nextListIndex(int(lm.payloadSource), 2))
		case "enter":
			lm.invokeStep = 1
			lm.invokePayload = ""
		}
		return *m, nil
	}

	// Step 1: text input
	switch key {
	case "esc":
		lm.invokeStep = 0
	case "enter":
		if lm.selected != nil {
			return m.startLoading(lm.invokeFunction(*m))
		}
	case "backspace":
		lm.invokePayload = trimLastRune(lm.invokePayload)
	default:
		if len(key) == 1 || key == " " {
			lm.invokePayload += key
		}
	}
	return *m, nil
}

func (lm lambdaModel) viewInvokeInput(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())

	fnName := ""
	if lm.selected != nil {
		fnName = lm.selected.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("Invoke — %s", fnName)))
	b.WriteString("\n\n")

	if lm.invokeStep == 0 {
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
			if lm.payloadSource == s.src {
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
		if lm.payloadSource == lambdaPayloadFile {
			label = "File path:"
			hint = "(enter path to a local JSON file)"
		}
		b.WriteString(normalStyle.Render("  " + label))
		b.WriteString("\n")
		payload := lm.invokePayload
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

func (lm *lambdaModel) updateInvokeResult(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenLambdaFunctionList
	case "i":
		lm.invokePayload = ""
		lm.payloadSource = lambdaPayloadManual
		lm.invokeStep = 0
		m.screen = screenLambdaInvokeInput
	case "l":
		if lm.selected != nil {
			return m.cwLogs.StartFromLambda(m, lm.selected.Name)
		}
	}
	return *m, nil
}

func (lm lambdaModel) viewInvokeResult(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())

	fnName := ""
	if lm.selected != nil {
		fnName = lm.selected.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("Invoke Result — %s", fnName)))
	b.WriteString("\n\n")

	r := lm.invokeResult
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

func (lm lambdaModel) loadFunctions(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.commandContext(), lambdaAPITimeout)
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

func (lm lambdaModel) invokeFunction(m Model) tea.Cmd {
	if lm.selected == nil {
		return func() tea.Msg {
			return errMsg{err: fmt.Errorf("no function selected")}
		}
	}
	payloadSource := lm.payloadSource
	payloadInput := lm.invokePayload
	fnName := lm.selected.Name

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

		ctx, cancel := context.WithTimeout(m.commandContext(), lambdaAPITimeout)
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
