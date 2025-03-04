package providers

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/awslabs/goformation/v4"

	"github.com/infracost/infracost/internal/config"
	"github.com/infracost/infracost/internal/hcl"
	"github.com/infracost/infracost/internal/logging"
	"github.com/infracost/infracost/internal/providers/cloudformation"
	"github.com/infracost/infracost/internal/providers/terraform"
	"github.com/infracost/infracost/internal/schema"
)

// Detect returns a list of providers for the given path. Multiple returned
// providers are because of auto-detected root modules residing under the
// original path.
func Detect(ctx *config.RunContext, project *config.Project, includePastResources bool) ([]schema.Provider, error) {
	path := project.Path

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("No such file or directory %s", path)
	}

	forceCLI := project.TerraformForceCLI
	projectType := DetectProjectType(path, forceCLI)
	projectContext := config.NewProjectContext(ctx, project, nil)
	if projectType != ProjectTypeAutodetect {
		projectContext.ContextValues.SetValue("project_type", projectType)
	}

	switch projectType {
	case ProjectTypeTerraformPlanJSON:
		return []schema.Provider{terraform.NewPlanJSONProvider(projectContext, includePastResources)}, nil
	case ProjectTypeTerraformPlanBinary:
		return []schema.Provider{terraform.NewPlanProvider(projectContext, includePastResources)}, nil
	case ProjectTypeTerraformCLI:
		return []schema.Provider{terraform.NewDirProvider(projectContext, includePastResources)}, nil
	case ProjectTypeTerragruntCLI:
		return []schema.Provider{terraform.NewTerragruntProvider(projectContext, includePastResources)}, nil
	case ProjectTypeTerraformStateJSON:
		return []schema.Provider{terraform.NewStateJSONProvider(projectContext, includePastResources)}, nil
	case ProjectTypeCloudFormation:
		return []schema.Provider{cloudformation.NewTemplateProvider(projectContext, includePastResources)}, nil
	}

	pathOverrides := make([]hcl.PathOverrideConfig, len(ctx.Config.Autodetect.PathOverrides))
	for i, override := range ctx.Config.Autodetect.PathOverrides {
		pathOverrides[i] = hcl.PathOverrideConfig{
			Path:    override.Path,
			Only:    override.Only,
			Exclude: override.Exclude,
		}
	}

	locatorConfig := &hcl.ProjectLocatorConfig{
		ExcludedDirs:           append(project.ExcludePaths, ctx.Config.Autodetect.ExcludeDirs...),
		IncludedDirs:           ctx.Config.Autodetect.IncludeDirs,
		PathOverrides:          pathOverrides,
		EnvNames:               ctx.Config.Autodetect.EnvNames,
		ChangedObjects:         ctx.VCSMetadata.Commit.ChangedObjects,
		UseAllPaths:            project.IncludeAllPaths,
		SkipAutoDetection:      project.SkipAutodetect,
		FallbackToIncludePaths: ctx.IsAutoDetect(),
	}
	pl := hcl.NewProjectLocator(logging.Logger, locatorConfig)
	rootPaths := pl.FindRootModules(project.Path)
	if len(rootPaths) == 0 {
		return nil, fmt.Errorf("could not detect path type for '%s'", path)
	}

	var autoProviders []schema.Provider
	for _, rootPath := range rootPaths {
		projectContext := config.NewProjectContext(ctx, project, nil)
		if rootPath.IsTerragrunt {
			projectContext.ContextValues.SetValue("project_type", "terragrunt_dir")
			autoProviders = append(autoProviders, terraform.NewTerragruntHCLProvider(rootPath, projectContext))
		} else {
			options := []hcl.Option{hcl.OptionWithSpinner(ctx.NewSpinner)}
			projectContext.ContextValues.SetValue("project_type", "terraform_dir")
			if ctx.Config.ConfigFilePath == "" && len(project.TerraformVarFiles) == 0 {
				autoProviders = append(autoProviders, autodetectedRootToProviders(pl, projectContext, rootPath, options...)...)
			} else {
				autoProviders = append(autoProviders, configFileRootToProvider(rootPath, options, projectContext, pl))
			}

		}
	}

	return autoProviders, nil
}

// configFileRootToProvider returns a provider for the given root path which is
// assumed to be a root module defined with a config file. In this case the
// terraform var files should not be grouped/reordered as the user has specified
// these manually.
func configFileRootToProvider(rootPath hcl.RootPath, options []hcl.Option, projectContext *config.ProjectContext, pl *hcl.ProjectLocator) *terraform.HCLProvider {
	var autoVarFiles []string
	for _, varFile := range rootPath.TerraformVarFiles {
		if hcl.IsAutoVarFile(varFile.RelPath) && (filepath.Dir(varFile.RelPath) == rootPath.Path || filepath.Dir(varFile.RelPath) == ".") {
			autoVarFiles = append(autoVarFiles, varFile.RelPath)
		}
	}

	if len(autoVarFiles) > 0 {
		options = append(options, hcl.OptionWithTFVarsPaths(autoVarFiles, false))
	}

	h, providerErr := terraform.NewHCLProvider(
		projectContext,
		rootPath,
		nil,
		options...,
	)
	if providerErr != nil {
		logging.Logger.Warn().Err(providerErr).Msgf("could not initialize provider for path %q", rootPath.Path)
	}
	return h
}

// autodetectedRootToProviders returns a list of providers for the given root
// path. These providers are generated by autodetected environments defined in
// the root module. These are defined by var file naming conventions.
func autodetectedRootToProviders(pl *hcl.ProjectLocator, projectContext *config.ProjectContext, rootPath hcl.RootPath, options ...hcl.Option) []schema.Provider {
	var providers []schema.Provider
	autoVarFiles := rootPath.AutoFiles()
	autoVarFiles = append(autoVarFiles, rootPath.GlobalFiles()...)
	varFileGrouping := rootPath.EnvGroupings()

	if len(varFileGrouping) > 0 {
		for _, env := range varFileGrouping {
			provider, err := terraform.NewHCLProvider(
				projectContext,
				rootPath,
				nil,
				append(
					options,
					hcl.OptionWithTFVarsPaths(append(autoVarFiles.ToPaths(), env.TerraformVarFiles.ToPaths()...), true),
					hcl.OptionWithModuleSuffix(env.Name),
				)...)
			if err != nil {
				logging.Logger.Warn().Err(err).Msgf("could not initialize provider for path %q", rootPath.Path)
				continue
			}

			providers = append(providers, provider)
		}

		return providers
	}

	varFiles := rootPath.EnvFiles()
	providerOptions := options
	if len(autoVarFiles) > 0 {
		providerOptions = append(providerOptions, hcl.OptionWithTFVarsPaths(append(varFiles.ToPaths(), autoVarFiles.ToPaths()...), true))
	}

	provider, err := terraform.NewHCLProvider(
		projectContext,
		rootPath,
		nil,
		providerOptions...,
	)
	if err != nil {
		logging.Logger.Warn().Err(err).Msgf("could not initialize provider for path %q", rootPath.Path)
		return nil
	}

	return []schema.Provider{provider}
}

type ProjectType string

var (
	ProjectTypeTerraformPlanJSON   ProjectType = "terraform_plan_json"
	ProjectTypeTerraformPlanBinary ProjectType = "terraform_plan_binary"
	ProjectTypeTerraformCLI        ProjectType = "terraform_cli"
	ProjectTypeTerragruntCLI       ProjectType = "terragrunt_cli"
	ProjectTypeTerraformStateJSON  ProjectType = "terraform_state_json"
	ProjectTypeCloudFormation      ProjectType = "cloudformation"
	ProjectTypeAutodetect          ProjectType = "autodetect"
)

func DetectProjectType(path string, forceCLI bool) ProjectType {
	if isCloudFormationTemplate(path) {
		return ProjectTypeCloudFormation
	}

	if isTerraformPlanJSON(path) {
		return ProjectTypeTerraformPlanJSON
	}

	if isTerraformStateJSON(path) {
		return ProjectTypeTerraformStateJSON
	}

	if isTerraformPlan(path) {
		return ProjectTypeTerraformPlanBinary
	}

	if forceCLI {
		if isTerragruntNestedDir(path, 5) {
			return ProjectTypeTerragruntCLI
		}

		return ProjectTypeTerraformCLI
	}

	return ProjectTypeAutodetect
}

func isTerraformPlanJSON(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	var jsonFormat struct {
		FormatVersion string      `json:"format_version"`
		PlannedValues interface{} `json:"planned_values"`
	}

	b, hasWrapper := terraform.StripSetupTerraformWrapper(b)
	if hasWrapper {
		logging.Logger.Info().Msgf("Stripped wrapper output from %s (to make it a valid JSON file) since setup-terraform GitHub Action was used without terraform_wrapper: false", path)
	}

	err = json.Unmarshal(b, &jsonFormat)
	if err != nil {
		return false
	}

	return jsonFormat.FormatVersion != "" && jsonFormat.PlannedValues != nil
}

func isTerraformStateJSON(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	var jsonFormat struct {
		FormatVersion string      `json:"format_version"`
		Values        interface{} `json:"values"`
	}

	b, hasWrapper := terraform.StripSetupTerraformWrapper(b)
	if hasWrapper {
		logging.Logger.Debug().Msgf("Stripped setup-terraform wrapper output from %s", path)
	}

	err = json.Unmarshal(b, &jsonFormat)
	if err != nil {
		return false
	}

	return jsonFormat.FormatVersion != "" && jsonFormat.Values != nil
}

func isTerraformPlan(path string) bool {
	r, err := zip.OpenReader(path)
	if err != nil {
		return false
	}
	defer r.Close()

	var planFile *zip.File
	for _, file := range r.File {
		if file.Name == "tfplan" {
			planFile = file
			break
		}
	}

	return planFile != nil
}

func isTerragruntDir(path string) bool {
	if val, ok := os.LookupEnv("TERRAGRUNT_CONFIG"); ok {
		if filepath.IsAbs(val) {
			return config.FileExists(val)
		}
		return config.FileExists(filepath.Join(path, val))
	}

	return config.FileExists(filepath.Join(path, "terragrunt.hcl")) || config.FileExists(filepath.Join(path, "terragrunt.hcl.json"))
}

func isTerragruntNestedDir(path string, maxDepth int) bool {
	if isTerragruntDir(path) {
		return true
	}

	if maxDepth > 0 {
		entries, err := os.ReadDir(path)
		if err == nil {
			for _, entry := range entries {
				name := entry.Name()
				if entry.IsDir() && name != config.InfracostDir && name != ".terraform" {
					if isTerragruntNestedDir(filepath.Join(path, name), maxDepth-1) {
						return true
					}
				}
			}
		}
	}
	return false
}

// goformation lib is not threadsafe, so we run this check synchronously
// See: https://github.com/awslabs/goformation/issues/363
var cfMux = &sync.Mutex{}

func isCloudFormationTemplate(path string) bool {
	cfMux.Lock()
	defer cfMux.Unlock()

	template, err := goformation.Open(path)
	if err != nil {
		return false
	}

	if len(template.Resources) > 0 {
		return true
	}

	return false
}
