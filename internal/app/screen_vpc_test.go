package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

func TestVPCListSharedFilterUsesFuzzyMatching(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 80
	m.height = 20

	updated, _, handled := m.vpc.HandleMessage(&m, vpcsLoadedMsg{vpcs: []awsservice.VPC{
		{VPCID: "vpc-111", Name: "dev-core", CIDR: "10.0.0.0/16"},
		{VPCID: "vpc-222", Name: "prod-core", CIDR: "10.1.0.0/16"},
	}})
	if !handled {
		t.Fatal("expected VPC load message to be handled")
	}
	model := updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model = updated.(Model)
	for _, ch := range []rune("prd") {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		model = updated.(Model)
	}

	if !model.isFiltering(filterVPCs) {
		t.Fatal("expected VPC filter to be active")
	}
	if got := len(model.vpc.filteredVPCs); got != 1 {
		t.Fatalf("expected 1 filtered VPC, got %d", got)
	}
	if got := model.vpc.filteredVPCs[0].Name; got != "prod-core" {
		t.Fatalf("expected filtered VPC prod-core, got %q", got)
	}

	view := model.vpc.viewVPCList(model)
	if !strings.Contains(stripANSI(view), "Filter: prd") {
		t.Fatalf("expected view to show VPC filter value, got %q", stripANSI(view))
	}
	if view == stripANSI(view) {
		t.Fatalf("expected highlighted VPC row output, got %q", view)
	}
}

func TestSubnetListSharedFilterDrivesSelection(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 80
	m.height = 20
	m.vpc.selectedVPC = &awsservice.VPC{Name: "prod-core", VPCID: "vpc-222"}

	updated, _, handled := m.vpc.HandleMessage(&m, subnetsLoadedMsg{subnets: []awsservice.Subnet{
		{SubnetID: "subnet-111", Name: "private-a", CIDR: "10.1.1.0/24", AvailabilityZone: "us-west-2a"},
		{SubnetID: "subnet-222", Name: "public-b", CIDR: "10.1.2.0/24", AvailabilityZone: "us-west-2b"},
	}})
	if !handled {
		t.Fatal("expected subnet load message to be handled")
	}
	model := updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model = updated.(Model)
	for _, ch := range []rune("pb") {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		model = updated.(Model)
	}

	if got := len(model.vpc.filteredSubnets); got != 1 {
		t.Fatalf("expected 1 filtered subnet, got %d", got)
	}
	if got := model.vpc.filteredSubnets[0].SubnetID; got != "subnet-222" {
		t.Fatalf("expected subnet-222 after filtering, got %q", got)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.isFiltering(filterSubnets) {
		t.Fatal("expected enter to close subnet filter mode before selection")
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected subnet detail load command on enter")
	}
	if model.vpc.selectedSubnet == nil || model.vpc.selectedSubnet.SubnetID != "subnet-222" {
		t.Fatalf("expected enter to select filtered subnet, got %+v", model.vpc.selectedSubnet)
	}
	if model.screen != screenLoading {
		t.Fatalf("expected loading screen after selecting subnet, got %v", model.screen)
	}
}
