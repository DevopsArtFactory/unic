package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsservice "unic/internal/services/aws"
)

type s3Model struct {
	buckets         []awsservice.S3Bucket
	filteredBuckets []awsservice.S3Bucket
	bucketIdx       int
	selectedBucket  *awsservice.S3Bucket
	objects         []awsservice.S3Object
	filteredObjects []awsservice.S3Object
	objectIdx       int
	currentPrefix   string
	prefixStack     []string
	selectedObject  *awsservice.S3ObjectDetail
}

func newS3Model() s3Model {
	return s3Model{}
}

func (s3m *s3Model) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoading(s3m.loadBuckets(*m))
}

func (s3m *s3Model) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case s3BucketsLoadedMsg:
		s3m.buckets = msg.buckets
		s3m.filteredBuckets = msg.buckets
		s3m.bucketIdx = 0
		m.resetFilter(filterS3Buckets)
		m.screen = screenS3BucketList
		return *m, nil, true
	case s3ObjectsLoadedMsg:
		if s3m.selectedBucket == nil || s3m.selectedBucket.Name != msg.bucket {
			return *m, nil, true
		}
		s3m.currentPrefix = msg.prefix
		s3m.objects = flattenS3Objects(msg.objects)
		s3m.filteredObjects = s3m.objects
		s3m.objectIdx = 0
		m.resetFilter(filterS3Objects)
		m.screen = screenS3ObjectList
		return *m, nil, true
	case s3ObjectDetailLoadedMsg:
		s3m.selectedObject = msg.detail
		m.screen = screenS3ObjectDetail
		return *m, nil, true
	}
	return *m, nil, false
}

func (s3m *s3Model) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenS3BucketList:
		newM, cmd := s3m.updateBucketList(m, msg)
		return newM, cmd, true
	case screenS3ObjectList:
		newM, cmd := s3m.updateObjectList(m, msg)
		return newM, cmd, true
	case screenS3ObjectDetail:
		newM, cmd := s3m.updateObjectDetail(m, msg)
		return newM, cmd, true
	default:
		return *m, nil, false
	}
}

func (s3m s3Model) View(m Model) (string, bool) {
	switch m.screen {
	case screenS3BucketList:
		return s3m.viewBucketList(m), true
	case screenS3ObjectList:
		return s3m.viewObjectList(m), true
	case screenS3ObjectDetail:
		return s3m.viewObjectDetail(m), true
	default:
		return "", false
	}
}

func (s3m *s3Model) ApplyFilter(m *Model, target filterTarget) bool {
	switch target {
	case filterS3Buckets:
		s3m.filteredBuckets = applyFilter(s3m.buckets, m.filterValue(target))
		s3m.bucketIdx = 0
		return true
	case filterS3Objects:
		s3m.filteredObjects = applyFilter(s3m.objects, m.filterValue(target))
		s3m.objectIdx = 0
		return true
	default:
		return false
	}
}

func flattenS3Objects(result awsservice.S3ListResult) []awsservice.S3Object {
	items := make([]awsservice.S3Object, 0, len(result.Prefixes)+len(result.Objects))
	items = append(items, result.Prefixes...)
	items = append(items, result.Objects...)
	return items
}

func (s3m s3Model) loadBuckets(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return errMsg{err: err}
		}
		// Note: Cannot assign to m.awsRepo here as m is a value receiver copy
		// The repository will be created fresh on each call which is acceptable

		buckets, err := repo.ListBuckets(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		if len(buckets) == 0 {
			return errMsg{err: fmt.Errorf("no S3 buckets found")}
		}
		return s3BucketsLoadedMsg{buckets: buckets}
	}
}

func (s3m s3Model) loadObjects(m Model, bucketName, prefix string) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return errMsg{err: err}
			}
		}

		objects, err := repo.ListBucketObjects(ctx, bucketName, prefix)
		if err != nil {
			return errMsg{err: err}
		}
		return s3ObjectsLoadedMsg{
			bucket:  bucketName,
			prefix:  awsservice.NormalizeS3Prefix(prefix),
			objects: objects,
		}
	}
}

func (s3m s3Model) loadObjectDetail(m Model, bucketName, key string) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo := m.awsRepo
		if repo == nil {
			var err error
			repo, err = awsservice.NewAwsRepository(ctx, m.cfg)
			if err != nil {
				return errMsg{err: err}
			}
		}

		detail, err := repo.HeadBucketObject(ctx, bucketName, key)
		if err != nil {
			return errMsg{err: err}
		}
		return s3ObjectDetailLoadedMsg{detail: detail}
	}
}

func (s3m *s3Model) updateBucketList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterS3Buckets); handled {
		return *m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterS3Buckets)
	case "up", "k":
		s3m.bucketIdx = previousListIndex(s3m.bucketIdx, len(s3m.filteredBuckets))
	case "down", "j":
		s3m.bucketIdx = nextListIndex(s3m.bucketIdx, len(s3m.filteredBuckets))
	case "/":
		return *m, m.activateFilter(filterS3Buckets)
	case "enter":
		if len(s3m.filteredBuckets) > 0 && s3m.bucketIdx < len(s3m.filteredBuckets) {
			selected := s3m.filteredBuckets[s3m.bucketIdx]
			s3m.selectedBucket = &selected
			s3m.currentPrefix = ""
			s3m.prefixStack = nil
			return m.startLoading(s3m.loadObjects(*m, selected.Name, ""))
		}
	}
	return *m, nil
}

func (s3m *s3Model) updateObjectList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterS3Objects); handled {
		return *m, cmd
	}

	switch key {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		if s3m.selectedBucket == nil {
			m.screen = screenS3BucketList
			return *m, nil
		}
		if s3m.currentPrefix == "" {
			m.screen = screenS3BucketList
			s3m.objectIdx = 0
			return *m, nil
		}
		nextPrefix := awsservice.ParentS3Prefix(s3m.currentPrefix)
		return m.startLoading(s3m.loadObjects(*m, s3m.selectedBucket.Name, nextPrefix))
	case "up", "k":
		s3m.objectIdx = previousListIndex(s3m.objectIdx, len(s3m.filteredObjects))
	case "down", "j":
		s3m.objectIdx = nextListIndex(s3m.objectIdx, len(s3m.filteredObjects))
	case "/":
		return *m, m.activateFilter(filterS3Objects)
	case "enter":
		if len(s3m.filteredObjects) == 0 || s3m.objectIdx >= len(s3m.filteredObjects) || s3m.selectedBucket == nil {
			return *m, nil
		}
		selected := s3m.filteredObjects[s3m.objectIdx]
		if selected.IsPrefix {
			return m.startLoading(s3m.loadObjects(*m, s3m.selectedBucket.Name, selected.Prefix))
		}
		return m.startLoading(s3m.loadObjectDetail(*m, s3m.selectedBucket.Name, selected.Key))
	}
	return *m, nil
}

func (s3m *s3Model) updateObjectDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenS3ObjectList
	}
	return *m, nil
}

func (s3m s3Model) viewBucketList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("S3 Buckets"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterS3Buckets))
	b.WriteString("\n\n")

	if len(s3m.filteredBuckets) == 0 {
		panel.WriteString(dimStyle.Render("  No matching buckets"))
		panel.WriteString("\n")
	} else {
		maxName := 6
		for _, bucket := range s3m.filteredBuckets {
			if len(bucket.Name) > maxName {
				maxName = len(bucket.Name)
			}
		}
		if maxName > 48 {
			maxName = 48
		}
		nameCol := lipgloss.NewStyle().Width(maxName + 2)
		regionCol := lipgloss.NewStyle().Width(18)
		panel.WriteString(dimStyle.Render("  " + nameCol.Render("NAME") + regionCol.Render("REGION") + "CREATED"))
		panel.WriteString("\n")

		visibleLines := max(m.height-11, 5)
		start := 0
		if s3m.bucketIdx >= visibleLines {
			start = s3m.bucketIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(s3m.filteredBuckets))
		for i := start; i < end; i++ {
			bucket := s3m.filteredBuckets[i]
			cursor := "  "
			style := normalStyle
			if i == s3m.bucketIdx {
				cursor = "> "
				style = selectedStyle
			}
			name := bucket.Name
			if len(name) > maxName {
				name = name[:maxName-3] + "..."
			}
			created := "-"
			if !bucket.CreationDate.IsZero() {
				created = bucket.CreationDate.Format(time.DateOnly)
			}
			row := cursor +
				nameCol.Inherit(style).Render(m.renderHighlightedValue(filterS3Buckets, name)) +
				regionCol.Inherit(dimStyle).Render(m.renderHighlightedValue(filterS3Buckets, bucket.Region)) +
				dimStyle.Render(created)
			panel.WriteString(row)
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d buckets", len(s3m.filteredBuckets), len(s3m.buckets))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: browse • esc: back • H: home"))
	return b.String()
}

func (s3m s3Model) viewObjectList(m Model) string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	bucketName := ""
	if s3m.selectedBucket != nil {
		bucketName = s3m.selectedBucket.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("S3 Objects — %s", bucketName)))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("Path: %s", awsservice.S3Breadcrumb(s3m.currentPrefix))))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterS3Objects))
	b.WriteString("\n\n")

	if len(s3m.filteredObjects) == 0 {
		panel.WriteString(dimStyle.Render("  No matching objects or prefixes"))
		panel.WriteString("\n")
	} else {
		maxName := 6
		for _, obj := range s3m.filteredObjects {
			if len(obj.Name) > maxName {
				maxName = len(obj.Name)
			}
		}
		if maxName > 48 {
			maxName = 48
		}
		nameCol := lipgloss.NewStyle().Width(maxName + 2)
		sizeCol := lipgloss.NewStyle().Width(10)
		modCol := lipgloss.NewStyle().Width(18)
		panel.WriteString(dimStyle.Render("  " + nameCol.Render("NAME") + sizeCol.Render("SIZE") + modCol.Render("MODIFIED") + "TYPE"))
		panel.WriteString("\n")

		visibleLines := max(m.height-12, 5)
		start := 0
		if s3m.objectIdx >= visibleLines {
			start = s3m.objectIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(s3m.filteredObjects))
		for i := start; i < end; i++ {
			obj := s3m.filteredObjects[i]
			cursor := "  "
			style := normalStyle
			if i == s3m.objectIdx {
				cursor = "> "
				style = selectedStyle
			}
			name := obj.Name
			if obj.IsPrefix {
				name += "/"
			}
			if len(name) > maxName {
				name = name[:maxName-3] + "..."
			}
			sizeText := "-"
			modified := "-"
			typeText := "prefix"
			if !obj.IsPrefix {
				sizeText = awsservice.FormatBytes(obj.Size)
				if !obj.LastModified.IsZero() {
					modified = obj.LastModified.Format("2006-01-02 15:04")
				}
				typeText = obj.StorageClass
				if typeText == "" {
					typeText = "-"
				}
			}
			row := cursor +
				nameCol.Inherit(style).Render(m.renderHighlightedValue(filterS3Objects, name)) +
				sizeCol.Inherit(dimStyle).Render(sizeText) +
				modCol.Inherit(dimStyle).Render(modified) +
				dimStyle.Render(m.renderHighlightedValue(filterS3Objects, typeText))
			panel.WriteString(row)
			panel.WriteString("\n")
		}

		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d items", len(s3m.filteredObjects))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: open • esc: up/back • H: home"))
	return b.String()
}

func (s3m s3Model) viewObjectDetail(m Model) string {
	if s3m.selectedObject == nil {
		return ""
	}
	o := s3m.selectedObject
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("S3 Object Detail"))
	b.WriteString("\n\n")

	b.WriteString(renderDetailLine("Bucket", normalStyle.Render(o.Bucket)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Key", normalStyle.Render(o.Key)))
	b.WriteString("\n")
	b.WriteString(renderDetailLine("Size", normalStyle.Render(awsservice.FormatBytes(o.Size))))
	b.WriteString("\n")
	modified := "-"
	if !o.LastModified.IsZero() {
		modified = o.LastModified.Format("2006-01-02 15:04:05")
	}
	b.WriteString(renderDetailLine("Last Modified", normalStyle.Render(modified)))
	b.WriteString("\n")
	storageClass := o.StorageClass
	if storageClass == "" {
		storageClass = "-"
	}
	b.WriteString(renderDetailLine("Storage Class", normalStyle.Render(storageClass)))
	b.WriteString("\n")
	contentType := o.ContentType
	if contentType == "" {
		contentType = "-"
	}
	b.WriteString(renderDetailLine("Content Type", normalStyle.Render(contentType)))
	b.WriteString("\n")
	etag := o.ETag
	if etag == "" {
		etag = "-"
	}
	b.WriteString(renderDetailLine("ETag", normalStyle.Render(etag)))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("esc: back • H: home"))
	return b.String()
}
