package app

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/domain"
	awsservice "unic/internal/services/aws"
)

// The command palette is a global fuzzy finder over three item kinds:
// service features (jump to a browser), contexts (switch), and resources
// indexed across services (jump to the owning browser with the filter
// prefilled to the resource). Features and contexts are available instantly;
// resources stream in from an async index built when the palette opens.

type paletteItemKind int

const (
	paletteItemFeature paletteItemKind = iota
	paletteItemContext
	paletteItemResource
)

type paletteItem struct {
	kind         paletteItemKind
	label        string
	filterText   string
	feature      domain.FeatureKind
	service      domain.AwsService
	filterTarget filterTarget
	resourceKey  string
	contextName  string
}

func (i paletteItem) DisplayTitle() string { return i.label }
func (i paletteItem) FilterText() string   { return strings.ToLower(i.filterText) }

type paletteModel struct {
	static     []paletteItem
	resources  []paletteItem
	filtered   []paletteItem
	idx        int
	query      string
	indexing   bool
	indexErrs  []string
	prevScreen screen
	// generation identifies the current palette invocation; index results
	// carry the generation they were started for, so a slow index from an
	// earlier open (possibly under a different context) can never overwrite
	// a newer session's results.
	generation int
}

// openPalette builds the instantly-available items (features, contexts) and
// starts the async resource index for the current context.
func (m Model) openPalette() (tea.Model, tea.Cmd) {
	m.palette.prevScreen = m.screen
	m.palette.query = ""
	m.palette.idx = 0
	m.palette.resources = nil
	m.palette.indexErrs = nil
	m.palette.indexing = true
	m.palette.generation++

	var static []paletteItem
	for _, svc := range domain.Catalog() {
		for _, feat := range svc.Features {
			static = append(static, paletteItem{
				kind:       paletteItemFeature,
				label:      fmt.Sprintf("%-10s %s", svc.Name, feat.Kind),
				filterText: fmt.Sprintf("%s %s %s", svc.Name, feat.Kind, feat.Description),
				feature:    feat.Kind,
				service:    svc.Name,
			})
		}
	}
	for _, ctx := range m.ctxList {
		static = append(static, paletteItem{
			kind:        paletteItemContext,
			label:       fmt.Sprintf("%-10s switch to %s", "Context", ctx.Name),
			filterText:  fmt.Sprintf("context switch %s", ctx.FilterText()),
			contextName: ctx.Name,
		})
	}
	m.palette.static = static
	m.palette.filtered = m.palette.allItems()

	m.screen = screenCommandPalette
	return m, m.indexPaletteResources()
}

func (pm paletteModel) allItems() []paletteItem {
	items := make([]paletteItem, 0, len(pm.static)+len(pm.resources))
	items = append(items, pm.resources...)
	items = append(items, pm.static...)
	return items
}

func (pm *paletteModel) refilter() {
	pm.filtered = applyFilter(pm.allItems(), pm.query)
	if pm.idx >= len(pm.filtered) {
		pm.idx = 0
	}
}

func (m Model) updatePalette(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.palette.prevScreen == screenLoading && m.ssmParams.loading {
			return m.ssmParams.Start(&m)
		}
		m.screen = m.palette.prevScreen
	case "up":
		m.palette.idx = previousListIndex(m.palette.idx, len(m.palette.filtered))
	case "down":
		m.palette.idx = nextListIndex(m.palette.idx, len(m.palette.filtered))
	case "backspace":
		if runes := []rune(m.palette.query); len(runes) > 0 {
			m.palette.query = string(runes[:len(runes)-1])
			m.palette.refilter()
		}
	case "enter":
		if len(m.palette.filtered) == 0 || m.palette.idx >= len(m.palette.filtered) {
			return m, nil
		}
		return m.executePaletteItem(m.palette.filtered[m.palette.idx])
	default:
		if runes := msg.Runes; len(runes) > 0 {
			m.palette.query += string(runes)
			m.palette.refilter()
		}
	}
	return m, nil
}

func (m Model) executePaletteItem(item paletteItem) (tea.Model, tea.Cmd) {
	switch item.kind {
	case paletteItemContext:
		return m.startLoading(m.switchContext(item.contextName))
	case paletteItemFeature, paletteItemResource:
		m.enterServiceForPalette(item)
		if item.kind == paletteItemResource && item.filterTarget != filterNone {
			// Prefill the browser's shared filter so the target resource is
			// selected as soon as the list loads.
			m.storeFilterValue(item.filterTarget, item.resourceKey)
		}
		return m.startFeature(item.feature)
	}
	return m, nil
}

// enterServiceForPalette aligns the service/feature list state with the jump
// target so esc-navigation behaves as if the user drilled in manually.
// svcIdx indexes the rendered (sorted/filtered) service list, so it is
// resolved against filteredServices; activeService carries the identity.
func (m *Model) enterServiceForPalette(item paletteItem) {
	for _, svc := range m.services {
		if svc.Name != item.service {
			continue
		}
		m.activeService = svc.Name
		m.features = svc.Features
		for j, feat := range svc.Features {
			if feat.Kind == item.feature {
				m.featIdx = j
				break
			}
		}
		break
	}
	for i, svc := range m.filteredServices {
		if svc.Name == item.service {
			m.svcIdx = i
			break
		}
	}
}

// indexPaletteResources lists resources across services concurrently. Per-
// service failures surface as notes without hiding other services' results.
func (m Model) indexPaletteResources() tea.Cmd {
	cfg := m.cfg
	generation := m.palette.generation
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, cfg)
		if err != nil {
			return paletteResourcesIndexedMsg{generation: generation, errs: []string{err.Error()}}
		}

		type source struct {
			name  string
			fetch func() ([]paletteItem, error)
		}
		sources := []source{
			{"EC2", func() ([]paletteItem, error) {
				instances, err := repo.ListEC2Instances(ctx)
				if err != nil {
					return nil, err
				}
				items := make([]paletteItem, 0, len(instances))
				for _, inst := range instances {
					items = append(items, paletteResourceItem("EC2", inst.DisplayTitle(), inst.FilterText(),
						domain.FeatureEC2InstanceBrowser, domain.ServiceEC2, filterEC2BrowserInstances, inst.InstanceID))
				}
				return items, nil
			}},
			{"RDS", func() ([]paletteItem, error) {
				instances, err := repo.ListDBInstances(ctx)
				if err != nil {
					return nil, err
				}
				items := make([]paletteItem, 0, len(instances))
				for _, inst := range instances {
					items = append(items, paletteResourceItem("RDS", inst.DisplayTitle(), inst.FilterText(),
						domain.FeatureRDSBrowser, domain.ServiceRDS, filterRDS, inst.DBInstanceID))
				}
				return items, nil
			}},
			{"Lambda", func() ([]paletteItem, error) {
				functions, err := repo.ListFunctions(ctx)
				if err != nil {
					return nil, err
				}
				items := make([]paletteItem, 0, len(functions))
				for _, fn := range functions {
					items = append(items, paletteResourceItem("Lambda", fn.DisplayTitle(), fn.FilterText(),
						domain.FeatureLambdaBrowser, domain.ServiceLambda, filterLambdaFunctions, fn.Name))
				}
				return items, nil
			}},
			{"S3", func() ([]paletteItem, error) {
				buckets, err := repo.ListBuckets(ctx)
				if err != nil {
					return nil, err
				}
				items := make([]paletteItem, 0, len(buckets))
				for _, bucket := range buckets {
					items = append(items, paletteResourceItem("S3", bucket.DisplayTitle(), bucket.FilterText(),
						domain.FeatureS3Browser, domain.ServiceS3, filterS3Buckets, bucket.Name))
				}
				return items, nil
			}},
			{"ECS", func() ([]paletteItem, error) {
				clusters, err := repo.ListClusters(ctx)
				if err != nil {
					return nil, err
				}
				items := make([]paletteItem, 0, len(clusters))
				for _, cluster := range clusters {
					items = append(items, paletteResourceItem("ECS", cluster.DisplayTitle(), cluster.FilterText(),
						domain.FeatureECSExec, domain.ServiceECS, filterECSClusters, cluster.Name))
				}
				return items, nil
			}},
			{"Route53", func() ([]paletteItem, error) {
				zones, err := repo.ListHostedZones(ctx)
				if err != nil {
					return nil, err
				}
				items := make([]paletteItem, 0, len(zones))
				for _, zone := range zones {
					items = append(items, paletteResourceItem("Route53", zone.DisplayTitle(), zone.FilterText(),
						domain.FeatureRoute53Browser, domain.ServiceRoute53, filterRoute53Zones, zone.Name))
				}
				return items, nil
			}},
		}

		results := make([][]paletteItem, len(sources))
		errs := make([]string, len(sources))
		var wg sync.WaitGroup
		for i, src := range sources {
			wg.Add(1)
			go func(i int, src source) {
				defer wg.Done()
				items, err := src.fetch()
				if err != nil {
					errs[i] = fmt.Sprintf("%s: %v", src.name, err)
					return
				}
				results[i] = items
			}(i, src)
		}
		wg.Wait()

		var items []paletteItem
		var failures []string
		for i := range sources {
			items = append(items, results[i]...)
			if errs[i] != "" {
				failures = append(failures, errs[i])
			}
		}
		sort.SliceStable(items, func(i, j int) bool { return items[i].label < items[j].label })
		return paletteResourcesIndexedMsg{generation: generation, items: items, errs: failures}
	}
}

func paletteResourceItem(typeLabel, title, filterText string, feature domain.FeatureKind, service domain.AwsService, target filterTarget, key string) paletteItem {
	return paletteItem{
		kind:         paletteItemResource,
		label:        fmt.Sprintf("%-10s %s", typeLabel, title),
		filterText:   fmt.Sprintf("%s %s", typeLabel, filterText),
		feature:      feature,
		service:      service,
		filterTarget: target,
		resourceKey:  key,
	}
}

func (m Model) viewPalette() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Command Palette"))
	b.WriteString("\n")
	b.WriteString(filterStyle.Render(fmt.Sprintf("  > %s▏", m.palette.query)))
	b.WriteString("\n")
	if m.palette.indexing {
		b.WriteString(dimStyle.Render("  Indexing resources across services..."))
		b.WriteString("\n")
	}
	for _, indexErr := range m.palette.indexErrs {
		b.WriteString(errorStyle.Render("  " + indexErr))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	if len(m.palette.filtered) == 0 {
		panel.WriteString(dimStyle.Render("  No matches"))
		panel.WriteString("\n")
	} else {
		visibleLines := max(m.height-11, 5)
		start := 0
		if m.palette.idx >= visibleLines {
			start = m.palette.idx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.palette.filtered))
		for i := start; i < end; i++ {
			item := m.palette.filtered[i]
			cursor := "  "
			style := normalStyle
			if i == m.palette.idx {
				cursor = "> "
				style = selectedStyle
			}
			panel.WriteString(style.Render(cursor + item.label))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d items", len(m.palette.filtered), len(m.palette.allItems()))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("type: search • ↑/↓: navigate • enter: jump • esc: close"))
	return b.String()
}
