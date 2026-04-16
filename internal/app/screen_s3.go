package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsservice "unic/internal/services/aws"
)

func (m Model) handleS3Msg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case s3BucketsLoadedMsg:
		m.s3Buckets = msg.buckets
		m.filteredS3Buckets = msg.buckets
		m.s3BucketIdx = 0
		m.resetFilter(filterS3Buckets)
		m.screen = screenS3BucketList
		return m, nil, true
	case s3ObjectsLoadedMsg:
		if m.selectedS3Bucket == nil || m.selectedS3Bucket.Name != msg.bucket {
			return m, nil, true
		}
		m.s3CurrentPrefix = msg.prefix
		m.s3Objects = flattenS3Objects(msg.objects)
		m.filteredS3Objects = m.s3Objects
		m.s3ObjectIdx = 0
		m.resetFilter(filterS3Objects)
		m.screen = screenS3ObjectList
		return m, nil, true
	case s3ObjectDetailLoadedMsg:
		m.selectedS3Object = msg.detail
		m.screen = screenS3ObjectDetail
		return m, nil, true
	}
	return m, nil, false
}

func flattenS3Objects(result awsservice.S3ListResult) []awsservice.S3Object {
	items := make([]awsservice.S3Object, 0, len(result.Prefixes)+len(result.Objects))
	items = append(items, result.Prefixes...)
	items = append(items, result.Objects...)
	return items
}

func (m Model) loadS3Buckets() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
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

func (m Model) loadS3Objects(bucketName, prefix string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
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

func (m Model) loadS3ObjectDetail(bucketName, key string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
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

func (m Model) updateS3BucketList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterS3Buckets); handled {
		return m, cmd
	}

	switch key {
	case "q", "esc":
		m.screen = screenFeatureList
		m.resetFilter(filterS3Buckets)
	case "up", "k":
		if m.s3BucketIdx > 0 {
			m.s3BucketIdx--
		}
	case "down", "j":
		if m.s3BucketIdx < len(m.filteredS3Buckets)-1 {
			m.s3BucketIdx++
		}
	case "/":
		return m, m.activateFilter(filterS3Buckets)
	case "enter":
		if len(m.filteredS3Buckets) > 0 && m.s3BucketIdx < len(m.filteredS3Buckets) {
			selected := m.filteredS3Buckets[m.s3BucketIdx]
			m.selectedS3Bucket = &selected
			m.s3CurrentPrefix = ""
			m.s3PrefixStack = nil
			return m.startLoading(m.loadS3Objects(selected.Name, ""))
		}
	}
	return m, nil
}

func (m Model) updateS3ObjectList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if cmd, handled := m.updateSharedFilter(msg, filterS3Objects); handled {
		return m, cmd
	}

	switch key {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		if m.selectedS3Bucket == nil {
			m.screen = screenS3BucketList
			return m, nil
		}
		if m.s3CurrentPrefix == "" {
			m.screen = screenS3BucketList
			m.s3ObjectIdx = 0
			return m, nil
		}
		nextPrefix := awsservice.ParentS3Prefix(m.s3CurrentPrefix)
		return m.startLoading(m.loadS3Objects(m.selectedS3Bucket.Name, nextPrefix))
	case "up", "k":
		if m.s3ObjectIdx > 0 {
			m.s3ObjectIdx--
		}
	case "down", "j":
		if m.s3ObjectIdx < len(m.filteredS3Objects)-1 {
			m.s3ObjectIdx++
		}
	case "/":
		return m, m.activateFilter(filterS3Objects)
	case "enter":
		if len(m.filteredS3Objects) == 0 || m.s3ObjectIdx >= len(m.filteredS3Objects) || m.selectedS3Bucket == nil {
			return m, nil
		}
		selected := m.filteredS3Objects[m.s3ObjectIdx]
		if selected.IsPrefix {
			return m.startLoading(m.loadS3Objects(m.selectedS3Bucket.Name, selected.Prefix))
		}
		return m.startLoading(m.loadS3ObjectDetail(m.selectedS3Bucket.Name, selected.Key))
	}
	return m, nil
}

func (m Model) updateS3ObjectDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenS3ObjectList
	}
	return m, nil
}

func (m Model) viewS3BucketList() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("S3 Buckets"))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterS3Buckets))
	b.WriteString("\n\n")

	if len(m.filteredS3Buckets) == 0 {
		panel.WriteString(dimStyle.Render("  No matching buckets"))
		panel.WriteString("\n")
	} else {
		maxName := 6
		for _, bucket := range m.filteredS3Buckets {
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
		if m.s3BucketIdx >= visibleLines {
			start = m.s3BucketIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredS3Buckets))
		for i := start; i < end; i++ {
			bucket := m.filteredS3Buckets[i]
			cursor := "  "
			style := normalStyle
			if i == m.s3BucketIdx {
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
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d buckets", len(m.filteredS3Buckets), len(m.s3Buckets))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: browse • esc: back • H: home"))
	return b.String()
}

func (m Model) viewS3ObjectList() string {
	var b strings.Builder
	var panel strings.Builder
	b.WriteString(m.renderStatusBar())
	bucketName := ""
	if m.selectedS3Bucket != nil {
		bucketName = m.selectedS3Bucket.Name
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("S3 Objects — %s", bucketName)))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("Path: %s", awsservice.S3Breadcrumb(m.s3CurrentPrefix))))
	b.WriteString("\n")

	b.WriteString(m.renderFilterValue(filterS3Objects))
	b.WriteString("\n\n")

	if len(m.filteredS3Objects) == 0 {
		panel.WriteString(dimStyle.Render("  No matching objects or prefixes"))
		panel.WriteString("\n")
	} else {
		maxName := 6
		for _, obj := range m.filteredS3Objects {
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
		if m.s3ObjectIdx >= visibleLines {
			start = m.s3ObjectIdx - visibleLines + 1
		}
		end := min(start+visibleLines, len(m.filteredS3Objects))
		for i := start; i < end; i++ {
			obj := m.filteredS3Objects[i]
			cursor := "  "
			style := normalStyle
			if i == m.s3ObjectIdx {
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
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d items", len(m.filteredS3Objects))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar("↑/↓: navigate • /: filter • enter: open • esc: up/back • H: home"))
	return b.String()
}

func (m Model) viewS3ObjectDetail() string {
	if m.selectedS3Object == nil {
		return ""
	}
	o := m.selectedS3Object
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
