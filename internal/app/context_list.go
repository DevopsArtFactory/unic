package app

import (
	"sort"
	"strings"

	"unic/internal/config"
)

var configSetFavoriteContextsFn = config.SetFavoriteContexts

func favoriteContextSet(names []string) map[string]struct{} {
	favorites := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		favorites[name] = struct{}{}
	}
	return favorites
}

func (m Model) isFavoriteContext(name string) bool {
	_, ok := m.favoriteContexts[name]
	return ok
}

func (m *Model) contextsWithFavoriteState(contexts []config.ContextInfo) []config.ContextInfo {
	updated := append([]config.ContextInfo(nil), contexts...)
	if m.favoriteContexts == nil {
		m.favoriteContexts = make(map[string]struct{})
	}
	for i := range updated {
		if updated[i].Favorite {
			m.favoriteContexts[updated[i].Name] = struct{}{}
		}
		updated[i].Favorite = m.isFavoriteContext(updated[i].Name)
	}
	return updated
}

func (m *Model) sortFavoriteContextsFirst(contexts []config.ContextInfo) {
	sort.SliceStable(contexts, func(i, j int) bool {
		return contexts[i].Favorite && !contexts[j].Favorite
	})
}

func (m *Model) applyContextListFilter() {
	m.filteredCtxList = applyFilter(m.ctxList, m.filterValue(filterContexts))
	m.sortFavoriteContextsFirst(m.filteredCtxList)
	m.ctxIdx = clampListIndex(m.ctxIdx, len(m.filteredCtxList))
	m.syncContextTable()
}

func (m *Model) toggleFavoriteContext(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if m.favoriteContexts == nil {
		m.favoriteContexts = make(map[string]struct{})
	}
	wasFavorite := m.isFavoriteContext(name)
	neighborName := adjacentContextName(m.filteredCtxList, name)
	nextFavorites := copyFavoriteContextSet(m.favoriteContexts)
	if wasFavorite {
		delete(nextFavorites, name)
	} else {
		nextFavorites[name] = struct{}{}
	}

	favorites := favoriteContextNames(nextFavorites)
	if strings.TrimSpace(m.configPath) != "" {
		if err := configSetFavoriteContextsFn(m.configPath, favorites); err != nil {
			return err
		}
	}

	m.favoriteContexts = nextFavorites
	if m.cfg != nil {
		m.cfg.FavoriteContexts = favorites
	}
	for i := range m.ctxList {
		m.ctxList[i].Favorite = m.isFavoriteContext(m.ctxList[i].Name)
	}
	m.applyContextListFilter()
	if !wasFavorite && neighborName != "" {
		m.selectContextByName(neighborName)
		return nil
	}
	m.selectContextByName(name)
	return nil
}

func adjacentContextName(contexts []config.ContextInfo, name string) string {
	for i, ctx := range contexts {
		if ctx.Name != name {
			continue
		}
		if i+1 < len(contexts) {
			return contexts[i+1].Name
		}
		if i > 0 {
			return contexts[i-1].Name
		}
		return ""
	}
	return ""
}

func copyFavoriteContextSet(favorites map[string]struct{}) map[string]struct{} {
	copied := make(map[string]struct{}, len(favorites))
	for name := range favorites {
		copied[name] = struct{}{}
	}
	return copied
}

func (m Model) favoriteContextNames() []string {
	return favoriteContextNames(m.favoriteContexts)
}

func favoriteContextNames(favorites map[string]struct{}) []string {
	names := make([]string, 0, len(favorites))
	for name := range favorites {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (m *Model) selectContextByName(name string) {
	for i, ctx := range m.filteredCtxList {
		if ctx.Name == name {
			m.ctxIdx = i
			m.syncContextTable()
			return
		}
	}
	m.ctxIdx = clampListIndex(m.ctxIdx, len(m.filteredCtxList))
	m.syncContextTable()
}
