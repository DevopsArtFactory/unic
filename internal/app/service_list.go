package app

import (
	"fmt"
	"sort"
	"strings"

	"unic/internal/config"
	"unic/internal/domain"
)

var configSetFavoriteServicesFn = config.SetFavoriteServices
var configSetBootSplashEnabledFn = config.SetBootSplashEnabled
var configSetBootSplashSeenVersionFn = config.SetBootSplashSeenVersion

func (m Model) serviceList() []domain.Service {
	if m.filteredServices != nil {
		return m.filteredServices
	}
	return m.services
}

func (m Model) selectedService() (domain.Service, bool) {
	services := m.serviceList()
	if len(services) == 0 {
		return domain.Service{}, false
	}
	return services[clampListIndex(m.svcIdx, len(services))], true
}

func (m *Model) applyServiceListFilter() {
	filtered := applyFilter(m.services, m.filterValue(filterServices))
	m.filteredServices = append([]domain.Service(nil), filtered...)
	sort.SliceStable(m.filteredServices, m.lessService)
	m.svcIdx = clampListIndex(m.svcIdx, len(m.filteredServices))
}

func (m Model) lessService(i, j int) bool {
	left := m.filteredServices[i]
	right := m.filteredServices[j]
	leftFavorite := m.isFavoriteService(left.Name)
	rightFavorite := m.isFavoriteService(right.Name)
	if leftFavorite != rightFavorite {
		return leftFavorite
	}
	return string(left.Name) < string(right.Name)
}

func favoriteServiceSet(names []string) map[domain.AwsService]struct{} {
	favorites := make(map[domain.AwsService]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		favorites[domain.AwsService(name)] = struct{}{}
	}
	return favorites
}

func (m Model) isFavoriteService(name domain.AwsService) bool {
	_, ok := m.favoriteServices[name]
	return ok
}

func (m *Model) toggleFavoriteService(name domain.AwsService) error {
	if m.favoriteServices == nil {
		m.favoriteServices = make(map[domain.AwsService]struct{})
	}
	if m.isFavoriteService(name) {
		delete(m.favoriteServices, name)
	} else {
		m.favoriteServices[name] = struct{}{}
	}
	favorites := m.favoriteServiceNames()
	if m.cfg != nil {
		m.cfg.FavoriteServices = favorites
	}
	if strings.TrimSpace(m.configPath) != "" {
		if err := configSetFavoriteServicesFn(m.configPath, favorites); err != nil {
			return err
		}
	}
	m.applyServiceListFilter()
	m.selectServiceByName(name)
	return nil
}

func (m *Model) toggleBootSplash() error {
	newVal := !m.bootSplash
	if strings.TrimSpace(m.configPath) != "" {
		if err := configSetBootSplashEnabledFn(m.configPath, newVal); err != nil {
			return err
		}
	}
	m.bootSplash = newVal
	if m.cfg != nil {
		m.cfg.BootSplash = newVal
	}
	return nil
}

func (m Model) favoriteServiceNames() []string {
	names := make([]string, 0, len(m.favoriteServices))
	for name := range m.favoriteServices {
		names = append(names, string(name))
	}
	sort.Strings(names)
	return names
}

func (m Model) serviceListSummary() string {
	favoriteCount := len(m.favoriteServices)
	if favoriteCount == 1 {
		return "favorites first, A-Z • 1 favorite"
	}
	return fmt.Sprintf("favorites first, A-Z • %d favorites", favoriteCount)
}

func (m *Model) selectServiceByName(name domain.AwsService) {
	for i, service := range m.filteredServices {
		if service.Name == name {
			m.svcIdx = i
			return
		}
	}
	m.svcIdx = clampListIndex(m.svcIdx, len(m.filteredServices))
}
