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
)

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

	selected, err := chooseContext(reader, errOut, contexts)
	if err != nil {
		return "", err
	}

	finalName, err := resolveContextSelection(ctx, configPath, selected, reader, errOut)
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

	fmt.Fprintf(errOut, "Selected context: %s\n", finalName)
	return buildEnvExportsFn(ctx, finalCfg)
}

func resolveContextSelection(ctx context.Context, configPath string, selected config.ContextInfo, in *bufio.Reader, errOut io.Writer) (string, error) {
	if config.AuthType(selected.AuthType) != config.AuthTypeSSO || (selected.SSOAccountID != "" && selected.SSORoleName != "") {
		return selected.Name, nil
	}
	if selected.SSOStartURL == "" {
		return "", fmt.Errorf("context %q is missing sso_start_url", selected.Name)
	}

	baseCfg, err := config.LoadNamedContext(configPath, selected.Name)
	if err != nil {
		return "", err
	}

	fmt.Fprintf(errOut, "Listing AWS accounts for %s ...\n", selected.Name)
	accounts, err := listSSOAccountsFn(ctx, baseCfg)
	if err != nil {
		return "", err
	}
	if len(accounts) == 0 {
		return "", fmt.Errorf("no SSO accounts available for %q", selected.Name)
	}

	account, err := chooseSSOAccount(in, errOut, accounts)
	if err != nil {
		return "", err
	}

	fmt.Fprintf(errOut, "Listing roles for account %s ...\n", account.ID)
	roles, err := listSSOAccountRolesFn(ctx, baseCfg, account.ID)
	if err != nil {
		return "", err
	}
	if len(roles) == 0 {
		return "", fmt.Errorf("no SSO roles available for account %s", account.ID)
	}

	role, err := chooseSSORole(in, errOut, roles)
	if err != nil {
		return "", err
	}

	entry, name, err := buildSSOContextEntry(configPath, selected, account, role)
	if err != nil {
		return "", err
	}
	if err := config.UpsertContext(configPath, entry); err != nil {
		return "", err
	}
	return name, nil
}

func chooseContext(in *bufio.Reader, errOut io.Writer, contexts []config.ContextInfo) (config.ContextInfo, error) {
	fmt.Fprintln(errOut, "Available contexts:")
	for i, ctx := range contexts {
		current := ""
		if ctx.Current {
			current = " (current)"
		}
		fmt.Fprintf(errOut, "  %d) %s [%s]%s\n", i+1, ctx.Name, displayAuthType(ctx.AuthType), current)
	}

	index, err := chooseIndex(in, errOut, "Select context", len(contexts))
	if err != nil {
		return config.ContextInfo{}, err
	}
	return contexts[index], nil
}

func chooseSSOAccount(in *bufio.Reader, errOut io.Writer, accounts []awsservice.SSOAccount) (awsservice.SSOAccount, error) {
	fmt.Fprintln(errOut, "Available AWS accounts:")
	for i, account := range accounts {
		label := account.Name
		if label == "" {
			label = account.ID
		}
		if account.Email != "" {
			label = fmt.Sprintf("%s <%s>", label, account.Email)
		}
		fmt.Fprintf(errOut, "  %d) %s (%s)\n", i+1, label, account.ID)
	}

	index, err := chooseIndex(in, errOut, "Select account", len(accounts))
	if err != nil {
		return awsservice.SSOAccount{}, err
	}
	return accounts[index], nil
}

func chooseSSORole(in *bufio.Reader, errOut io.Writer, roles []awsservice.SSORole) (awsservice.SSORole, error) {
	fmt.Fprintln(errOut, "Available AWS roles:")
	for i, role := range roles {
		fmt.Fprintf(errOut, "  %d) %s\n", i+1, role.Name)
	}

	index, err := chooseIndex(in, errOut, "Select role", len(roles))
	if err != nil {
		return awsservice.SSORole{}, err
	}
	return roles[index], nil
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
			ctx.SSOStartURL == base.SSOStartURL &&
			ctx.SSOAccountID == account.ID &&
			ctx.SSORoleName == role.Name {
			return config.ContextEntry{
				Name:         ctx.Name,
				Profile:      ctx.Profile,
				Region:       ctx.Region,
				AuthType:     ctx.AuthType,
				SSOStartURL:  ctx.SSOStartURL,
				SSOAccountID: ctx.SSOAccountID,
				SSORoleName:  ctx.SSORoleName,
			}, ctx.Name, nil
		}
	}

	name := uniqueContextName(existing, fmt.Sprintf("%s-%s-%s", base.Name, account.ID, sanitizeName(role.Name)))
	region := base.Region
	if region == "" {
		region = config.DefaultRegion
	}
	entry := config.ContextEntry{
		Name:         name,
		Profile:      base.Profile,
		Region:       region,
		AuthType:     string(config.AuthTypeSSO),
		SSOStartURL:  base.SSOStartURL,
		SSOAccountID: account.ID,
		SSORoleName:  role.Name,
	}
	return entry, name, nil
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
