package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"unic/internal/domain"
)

const (
	discoverySchemaVersion  = "v1"
	annotationReadOnly      = "unic.dev/read-only"
	annotationDestructive   = "unic.dev/destructive"
	annotationOutputVersion = "unic.dev/output-version"
)

type capabilityDocument struct {
	SchemaVersion string              `json:"schema_version"`
	Services      []capabilityService `json:"services"`
	Commands      []commandContract   `json:"commands"`
}

type capabilityService struct {
	Name     string              `json:"name"`
	Features []capabilityFeature `json:"features"`
}

type capabilityFeature struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type commandContract struct {
	SchemaVersion string            `json:"schema_version"`
	Path          string            `json:"path"`
	Description   string            `json:"description"`
	Arguments     []commandArgument `json:"arguments"`
	Flags         []commandFlag     `json:"flags"`
	ReadOnly      bool              `json:"read_only"`
	Destructive   bool              `json:"destructive"`
	OutputVersion string            `json:"output_version"`
}

type commandArgument struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
}

type commandFlag struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Type      string `json:"type"`
	Default   string `json:"default"`
	Required  bool   `json:"required"`
}

func newCapabilitiesCmd(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "capabilities",
		Short:       "Describe supported AWS features and automation commands",
		Args:        cobra.NoArgs,
		Annotations: discoveryAnnotations(),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return writeJSON(cmd, buildCapabilityDocument(root))
		},
	}
	cmd.Flags().Bool("json", false, "Print the v1 machine-readable contract")
	_ = cmd.MarkFlagRequired("json")
	return cmd
}

func newSchemaCmd(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "schema <command> [subcommand...]",
		Short:       "Describe one automation command contract",
		Args:        cobra.MinimumNArgs(1),
		Annotations: discoveryAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := findCommand(root, args)
			if target == nil || target.Run == nil && target.RunE == nil {
				return fmt.Errorf("command %q not found or is not executable", strings.Join(args, " "))
			}
			return writeJSON(cmd, contractFor(target))
		},
	}
	cmd.Flags().Bool("json", false, "Print the v1 machine-readable contract")
	_ = cmd.MarkFlagRequired("json")
	return cmd
}

func discoveryAnnotations() map[string]string {
	return map[string]string{
		annotationReadOnly:      "true",
		annotationDestructive:   "false",
		annotationOutputVersion: discoverySchemaVersion,
	}
}

func buildCapabilityDocument(root *cobra.Command) capabilityDocument {
	services := make([]capabilityService, 0, len(domain.Catalog()))
	for _, service := range domain.Catalog() {
		features := make([]capabilityFeature, 0, len(service.Features))
		for _, feature := range service.Features {
			features = append(features, capabilityFeature{Name: string(feature.Kind), Description: feature.Description})
		}
		sort.Slice(features, func(i, j int) bool { return features[i].Name < features[j].Name })
		services = append(services, capabilityService{Name: string(service.Name), Features: features})
	}
	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })

	var commands []commandContract
	var walk func(*cobra.Command)
	walk = func(parent *cobra.Command) {
		for _, cmd := range parent.Commands() {
			if cmd.Hidden {
				continue
			}
			if cmd.Run != nil || cmd.RunE != nil {
				commands = append(commands, contractFor(cmd))
			}
			walk(cmd)
		}
	}
	walk(root)
	sort.Slice(commands, func(i, j int) bool { return commands[i].Path < commands[j].Path })

	return capabilityDocument{SchemaVersion: discoverySchemaVersion, Services: services, Commands: commands}
}

func contractFor(cmd *cobra.Command) commandContract {
	outputVersion := cmd.Annotations[annotationOutputVersion]
	if outputVersion == "" {
		outputVersion = "unversioned"
	}
	return commandContract{
		SchemaVersion: discoverySchemaVersion,
		Path:          cmd.CommandPath(),
		Description:   cmd.Short,
		Arguments:     commandArguments(cmd.Use),
		Flags:         commandFlags(cmd),
		ReadOnly:      cmd.Annotations[annotationReadOnly] == "true",
		Destructive:   cmd.Annotations[annotationDestructive] == "true",
		OutputVersion: outputVersion,
	}
}

func commandArguments(use string) []commandArgument {
	fields := strings.Fields(use)
	arguments := make([]commandArgument, 0, max(len(fields)-1, 0))
	for _, field := range fields[1:] {
		optional := strings.HasPrefix(field, "[")
		name := strings.Trim(field, "[]<>")
		name = strings.TrimSuffix(name, "...")
		arguments = append(arguments, commandArgument{Name: name, Required: !optional})
	}
	return arguments
}

func commandFlags(cmd *cobra.Command) []commandFlag {
	byName := make(map[string]commandFlag)
	collect := func(flag *pflag.Flag) {
		if flag.Name == "help" {
			return
		}
		byName[flag.Name] = commandFlag{
			Name: flag.Name, Shorthand: flag.Shorthand, Type: flag.Value.Type(), Default: flag.DefValue,
			Required: len(flag.Annotations[cobra.BashCompOneRequiredFlag]) > 0,
		}
	}
	cmd.Flags().VisitAll(collect)
	cmd.InheritedFlags().VisitAll(collect)
	flags := make([]commandFlag, 0, len(byName))
	for _, flag := range byName {
		flags = append(flags, flag)
	}
	sort.Slice(flags, func(i, j int) bool { return flags[i].Name < flags[j].Name })
	return flags
}

func findCommand(root *cobra.Command, names []string) *cobra.Command {
	current := root
	for _, name := range names {
		var next *cobra.Command
		for _, candidate := range current.Commands() {
			if candidate.Name() == name {
				next = candidate
				break
			}
		}
		if next == nil {
			return nil
		}
		current = next
	}
	return current
}

func writeJSON(cmd *cobra.Command, value any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
