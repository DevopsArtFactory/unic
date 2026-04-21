package app

import (
	"sort"

	"unic/internal/domain"
)

type serviceSortMode int

const (
	serviceSortCatalog serviceSortMode = iota
	serviceSortName
)

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
	if m.serviceSort == serviceSortName {
		sort.SliceStable(m.filteredServices, func(i, j int) bool {
			return m.filteredServices[i].Name < m.filteredServices[j].Name
		})
	}
	m.svcIdx = clampListIndex(m.svcIdx, len(m.filteredServices))
}

func (m *Model) toggleServiceSort() {
	if m.serviceSort == serviceSortName {
		m.serviceSort = serviceSortCatalog
	} else {
		m.serviceSort = serviceSortName
	}
	m.applyServiceListFilter()
}

func (m Model) serviceSortLabel() string {
	if m.serviceSort == serviceSortName {
		return "name"
	}
	return "catalog"
}
