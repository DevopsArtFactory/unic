package auth

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

var (
	listSSOAccountsFn     = awsservice.ListSSOAccounts
	listSSOAccountRolesFn = awsservice.ListSSOAccountRoles
	buildEnvExportsFn     = BuildEnvExports
	runConsoleLoginFn     = awsservice.RunConsoleLogin
)

// SelectContext interactively selects a context without changing current state.
func SelectContext(configPath string, in io.Reader, errOut io.Writer) (config.ContextInfo, error) {
	reader := bufio.NewReader(in)
	contexts, err := config.Contexts(configPath)
	if err != nil {
		return config.ContextInfo{}, err
	}
	if len(contexts) == 0 {
		return config.ContextInfo{}, fmt.Errorf("no contexts found in %s", configPath)
	}
	return chooseContext(in, reader, errOut, contexts)
}

// SetupContext interactively selects a context, resolves any required SSO account/role,
// sets it as current, and returns shell exports for eval.
func SetupContext(ctx context.Context, configPath string, in io.Reader, errOut io.Writer) (string, error) {
	reader := bufio.NewReader(in)
	contexts, err := config.Contexts(configPath)
	if err != nil {
		return "", err
	}
	if len(contexts) == 0 {
		return "", fmt.Errorf("no contexts found in %s", configPath)
	}

	selected, err := chooseContext(in, reader, errOut, contexts)
	if err != nil {
		return "", err
	}

	finalName, selectedRegion, err := resolveContextSelection(ctx, configPath, selected, in, reader, errOut)
	if err != nil {
		return "", err
	}

	if err := config.SetCurrent(configPath, finalName); err != nil {
		return "", err
	}

	finalCfg, err := config.LoadNamedContext(configPath, finalName)
	if err != nil {
		return "", err
	}
	finalCfg.Region = selectedRegion
	if finalCfg.AuthType == config.AuthTypeConsoleLogin {
		if err := runConsoleLoginFn(finalCfg); err != nil {
			return "", err
		}
	}

	fmt.Fprintf(errOut, "Selected context: %s\n", finalName)
	if len(contextRegions(selected.Region, selected.Regions)) > 1 {
		fmt.Fprintf(errOut, "Selected resource region: %s\n", selectedRegion)
	}
	return buildEnvExportsFn(ctx, finalCfg)
}

func resolveContextSelection(ctx context.Context, configPath string, selected config.ContextInfo, rawIn io.Reader, in *bufio.Reader, errOut io.Writer) (string, string, error) {
	if !IsBaseSSOContext(selected) {
		region, err := chooseResourceRegion(rawIn, in, errOut, selected.Region, selected.Regions)
		return selected.Name, region, err
	}

	fmt.Fprintf(errOut, "Listing AWS accounts for %s ...\n", selected.Name)
	accounts, err := ListSSOContextAccounts(ctx, configPath, selected)
	if err != nil {
		return "", "", err
	}
	if len(accounts) == 0 {
		return "", "", fmt.Errorf("no SSO accounts available for %q", selected.Name)
	}

	account, err := chooseSSOAccount(rawIn, in, errOut, accounts)
	if err != nil {
		return "", "", err
	}

	fmt.Fprintf(errOut, "Listing roles for account %s ...\n", account.ID)
	roles, err := ListSSOContextRoles(ctx, configPath, selected, account.ID)
	if err != nil {
		return "", "", err
	}
	if len(roles) == 0 {
		return "", "", fmt.Errorf("no SSO roles available for account %s", account.ID)
	}

	role, err := chooseSSORole(rawIn, in, errOut, roles)
	if err != nil {
		return "", "", err
	}

	region, err := chooseResourceRegion(rawIn, in, errOut, selected.Region, selected.Regions)
	if err != nil {
		return "", "", err
	}
	name, err := ResolveSSOContextSelection(configPath, selected, account, role)
	return name, region, err
}

func IsBaseSSOContext(selected config.ContextInfo) bool {
	return config.AuthType(selected.AuthType) == config.AuthTypeSSO &&
		(selected.SSOAccountID == "" || selected.SSORoleName == "")
}

func ListSSOContextAccounts(ctx context.Context, configPath string, selected config.ContextInfo) ([]awsservice.SSOAccount, error) {
	if !IsBaseSSOContext(selected) {
		return nil, fmt.Errorf("context %q is not an SSO base context", selected.Name)
	}
	if selected.SSOStartURL == "" {
		return nil, fmt.Errorf("context %q is missing sso_start_url", selected.Name)
	}

	baseCfg, err := config.LoadNamedContext(configPath, selected.Name)
	if err != nil {
		return nil, err
	}
	return listSSOAccountsFn(ctx, baseCfg)
}

func ListSSOContextRoles(ctx context.Context, configPath string, selected config.ContextInfo, accountID string) ([]awsservice.SSORole, error) {
	if !IsBaseSSOContext(selected) {
		return nil, fmt.Errorf("context %q is not an SSO base context", selected.Name)
	}

	baseCfg, err := config.LoadNamedContext(configPath, selected.Name)
	if err != nil {
		return nil, err
	}
	return listSSOAccountRolesFn(ctx, baseCfg, accountID)
}

func ResolveSSOContextSelection(configPath string, selected config.ContextInfo, account awsservice.SSOAccount, role awsservice.SSORole) (string, error) {
	entry, name, err := buildSSOContextEntry(configPath, selected, account, role)
	if err != nil {
		return "", err
	}
	if err := config.UpsertContext(configPath, entry); err != nil {
		return "", err
	}
	return name, nil
}

func chooseContext(rawIn io.Reader, in *bufio.Reader, errOut io.Writer, contexts []config.ContextInfo) (config.ContextInfo, error) {
	index, err := chooseFilteredIndex(rawIn, in, errOut, "contexts", contexts, renderContextChoice, func(ctx config.ContextInfo, query string) bool {
		return strings.Contains(strings.ToLower(ctx.FilterText()), strings.ToLower(strings.TrimSpace(query)))
	})
	if err != nil {
		return config.ContextInfo{}, err
	}
	return contexts[index], nil
}

func chooseSSOAccount(rawIn io.Reader, in *bufio.Reader, errOut io.Writer, accounts []awsservice.SSOAccount) (awsservice.SSOAccount, error) {
	index, err := chooseFilteredIndex(rawIn, in, errOut, "AWS accounts", accounts, renderAccountChoice, func(account awsservice.SSOAccount, query string) bool {
		label := account.Name
		if label == "" {
			label = account.ID
		}
		return containsFold(label, query) || containsFold(account.ID, query) || containsFold(account.Email, query)
	})
	if err != nil {
		return awsservice.SSOAccount{}, err
	}
	return accounts[index], nil
}

func chooseSSORole(rawIn io.Reader, in *bufio.Reader, errOut io.Writer, roles []awsservice.SSORole) (awsservice.SSORole, error) {
	index, err := chooseFilteredIndex(rawIn, in, errOut, "AWS roles", roles, renderRoleChoice, func(role awsservice.SSORole, query string) bool {
		return containsFold(role.Name, query)
	})
	if err != nil {
		return awsservice.SSORole{}, err
	}
	return roles[index], nil
}

func chooseResourceRegion(rawIn io.Reader, in *bufio.Reader, errOut io.Writer, defaultRegion string, regions []string) (string, error) {
	regions = contextRegions(defaultRegion, regions)
	if len(regions) == 0 {
		return config.DefaultRegion, nil
	}
	if len(regions) == 1 {
		return regions[0], nil
	}
	index, err := chooseFilteredIndex(rawIn, in, errOut, "resource regions", regions, func(region string) string {
		if region == defaultRegion {
			return region + " (default)"
		}
		return region
	}, func(region, query string) bool {
		return containsFold(region, query)
	})
	if err != nil {
		return "", err
	}
	return regions[index], nil
}

func chooseIndex(in *bufio.Reader, errOut io.Writer, label string, count int) (int, error) {
	for {
		fmt.Fprintf(errOut, "%s [1-%d]: ", label, count)
		raw, err := in.ReadString('\n')
		if err != nil {
			return 0, err
		}
		raw = strings.TrimSpace(raw)
		index := 0
		if _, err := fmt.Sscanf(raw, "%d", &index); err != nil || index < 1 || index > count {
			fmt.Fprintf(errOut, "Invalid selection %q\n", raw)
			continue
		}
		return index - 1, nil
	}
}

func chooseFilteredIndex[T any](
	rawIn io.Reader,
	in *bufio.Reader,
	errOut io.Writer,
	label string,
	items []T,
	render func(T) string,
	matches func(T, string) bool,
) (int, error) {
	if ttyIn, ttyOut, ok := interactiveChoiceIO(rawIn, errOut); ok {
		return chooseInteractiveIndex(ttyIn, ttyOut, label, items, render, matches)
	}
	return chooseLineFilteredIndex(in, errOut, label, items, render, matches)
}

func chooseLineFilteredIndex[T any](
	in *bufio.Reader,
	errOut io.Writer,
	label string,
	items []T,
	render func(T) string,
	matches func(T, string) bool,
) (int, error) {
	filtered := make([]int, len(items))
	for i := range items {
		filtered[i] = i
	}
	query := ""

	for {
		fmt.Fprintf(errOut, "Available %s", label)
		if query != "" {
			fmt.Fprintf(errOut, " (filtered by %q)", query)
		}
		fmt.Fprintln(errOut, ":")
		for i, originalIdx := range filtered {
			fmt.Fprintf(errOut, "  %d) %s\n", i+1, render(items[originalIdx]))
		}

		fmt.Fprintf(errOut, "Select %s [1-%d, search text, / to reset]: ", strings.TrimSuffix(label, "s"), len(filtered))
		raw, err := in.ReadString('\n')
		if err != nil {
			return 0, err
		}
		raw = strings.TrimSpace(raw)
		if raw == "/" {
			filtered = filtered[:0]
			for i := range items {
				filtered = append(filtered, i)
			}
			query = ""
			continue
		}

		index := 0
		if _, err := fmt.Sscanf(raw, "%d", &index); err == nil && index >= 1 && index <= len(filtered) {
			return filtered[index-1], nil
		}

		query = raw
		filtered = filtered[:0]
		for i, item := range items {
			if matches(item, query) {
				filtered = append(filtered, i)
			}
		}
		if len(filtered) == 0 {
			fmt.Fprintf(errOut, "No %s match %q\n", label, query)
			filtered = make([]int, len(items))
			for i := range items {
				filtered[i] = i
			}
			query = ""
		}
	}
}

func renderContextChoice(ctx config.ContextInfo) string {
	current := ""
	if ctx.Current {
		current = " (current)"
	}
	return fmt.Sprintf("%s [%s]%s", ctx.Name, displayAuthType(ctx.AuthType), current)
}

func renderAccountChoice(account awsservice.SSOAccount) string {
	label := account.Name
	if label == "" {
		label = account.ID
	}
	if account.Email != "" {
		label = fmt.Sprintf("%s <%s>", label, account.Email)
	}
	return fmt.Sprintf("%s (%s)", label, account.ID)
}

func renderRoleChoice(role awsservice.SSORole) string {
	return role.Name
}

func containsFold(value, query string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(strings.TrimSpace(query)))
}

func buildSSOContextEntry(configPath string, base config.ContextInfo, account awsservice.SSOAccount, role awsservice.SSORole) (config.ContextEntry, string, error) {
	existing, err := config.Contexts(configPath)
	if err != nil {
		return config.ContextEntry{}, "", err
	}

	for _, ctx := range existing {
		if ctx.AuthType != string(config.AuthTypeSSO) {
			continue
		}
		if ctx.Profile == base.Profile &&
			ctx.Region == base.Region &&
			equalContextRegions(ctx.Region, ctx.Regions, base.Region, base.Regions) &&
			ctx.SSORegion == base.SSORegion &&
			ctx.SSOStartURL == base.SSOStartURL &&
			ctx.SSOAccountID == account.ID &&
			ctx.SSORoleName == role.Name {
			return config.ContextEntry{
				Name:         ctx.Name,
				Profile:      ctx.Profile,
				Region:       ctx.Region,
				AuthType:     ctx.AuthType,
				SSOStartURL:  ctx.SSOStartURL,
				SSORegion:    ctx.SSORegion,
				SSOAccountID: ctx.SSOAccountID,
				SSORoleName:  ctx.SSORoleName,
				Regions:      ctx.Regions,
			}, ctx.Name, nil
		}
	}

	name := uniqueContextName(existing, fmt.Sprintf("%s-%s-%s", base.Name, account.ID, sanitizeName(role.Name)))
	region := base.Region
	if region == "" {
		region = config.DefaultRegion
	}
	resourceRegions := contextRegions(region, base.Regions)
	entry := config.ContextEntry{
		Name: name,
		Auth: &config.ContextAuth{
			Type:         string(config.AuthTypeSSO),
			Profile:      base.Profile,
			SSOStartURL:  base.SSOStartURL,
			SSORegion:    base.SSORegion,
			SSOAccountID: account.ID,
			SSORoleName:  role.Name,
		},
		Resources: &config.ContextResources{
			DefaultRegion: region,
			Regions:       resourceRegions[1:],
		},
	}
	return entry, name, nil
}

func equalContextRegions(leftDefault string, left []string, rightDefault string, right []string) bool {
	left = contextRegions(leftDefault, left)
	right = contextRegions(rightDefault, right)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func contextRegions(defaultRegion string, regions []string) []string {
	result := make([]string, 0, len(regions)+1)
	seen := make(map[string]struct{}, len(regions)+1)
	for _, region := range append([]string{defaultRegion}, regions...) {
		region = strings.TrimSpace(region)
		if region == "" {
			continue
		}
		if _, ok := seen[region]; ok {
			continue
		}
		seen[region] = struct{}{}
		result = append(result, region)
	}
	return result
}

func uniqueContextName(existing []config.ContextInfo, base string) string {
	used := map[string]struct{}{}
	for _, ctx := range existing {
		used[ctx.Name] = struct{}{}
	}
	if _, ok := used[base]; !ok {
		return base
	}
	for i := 2; ; i++ {
		name := fmt.Sprintf("%s-%d", base, i)
		if _, ok := used[name]; !ok {
			return name
		}
	}
}

var invalidNameChars = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func sanitizeName(value string) string {
	value = invalidNameChars.ReplaceAllString(strings.TrimSpace(value), "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "role"
	}
	return strings.ToLower(value)
}

func displayAuthType(value string) string {
	if value == "" {
		return "default"
	}
	return value
}
