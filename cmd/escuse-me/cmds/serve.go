package cmds

import (
	"context"
	"fmt"
	es_cmds "github.com/go-go-golems/escuse-me/pkg/cmds"
	es_layers "github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/parka/pkg/glazed/handlers/datatables"
	"github.com/go-go-golems/parka/pkg/handlers"
	command_dir "github.com/go-go-golems/parka/pkg/handlers/command-dir"
	"github.com/go-go-golems/parka/pkg/handlers/config"
	generic_command "github.com/go-go-golems/parka/pkg/handlers/generic-command"
	"github.com/go-go-golems/parka/pkg/handlers/template"
	template_dir "github.com/go-go-golems/parka/pkg/handlers/template-dir"
	"github.com/go-go-golems/parka/pkg/server"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"os"
	"os/signal"
	"path/filepath"
)

type ServeCommand struct {
	*cmds.CommandDescription
	repositories []string
}

var _ cmds.BareCommand = &ServeCommand{}

type ServeSettings struct {
	Dev         bool     `glazed.parameter:"dev"`
	Debug       bool     `glazed.parameter:"debug"`
	ContentDirs []string `glazed.parameter:"content-dirs"`
	ServePort   int      `glazed.parameter:"serve-port"`
	ServeHost   string   `glazed.parameter:"serve-host"`
	ConfigFile  string   `glazed.parameter:"config-file"`
}

func NewServeCommand(
	repositories []string,
	options ...cmds.CommandDescriptionOption,
) (*ServeCommand, error) {
	esParameterLayer, err := es_layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES connection layer")
	}

	options_ := append(options,
		cmds.WithShort("Serve the API"),
		cmds.WithFlags(
			parameters.NewParameterDefinition(
				"serve-port",
				parameters.ParameterTypeInteger,
				parameters.WithShortFlag("p"),
				parameters.WithHelp("Port to serve the API on"),
				parameters.WithDefault(8080),
			),
			parameters.NewParameterDefinition(
				"serve-host",
				parameters.ParameterTypeString,
				parameters.WithHelp("Host to serve the API on"),
				parameters.WithDefault("localhost"),
			),
			parameters.NewParameterDefinition(
				"dev",
				parameters.ParameterTypeBool,
				parameters.WithHelp("Run in development mode"),
				parameters.WithDefault(false),
			),
			parameters.NewParameterDefinition(
				"debug",
				parameters.ParameterTypeBool,
				parameters.WithHelp("Run in debug mode (expose /debug/pprof routes)"),
				parameters.WithDefault(false),
			),
			parameters.NewParameterDefinition(
				"content-dirs",
				parameters.ParameterTypeStringList,
				parameters.WithHelp("Serve static and templated files from these directories"),
				parameters.WithDefault([]string{}),
			),
			parameters.NewParameterDefinition(
				"config-file",
				parameters.ParameterTypeString,
				parameters.WithHelp("Config file to configure the serve functionality"),
			),
		),
		cmds.WithLayersList(esParameterLayer),
	)

	return &ServeCommand{
		CommandDescription: cmds.NewCommandDescription(
			"serve",
			options_...,
		),
		repositories: repositories,
	}, nil
}

func (s *ServeCommand) runWithConfigFile(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	configFilePath string,
	serverOptions []server.ServerOption,
) error {
	ss := &ServeSettings{}
	err := parsedLayers.InitializeStruct(layers.DefaultSlug, ss)
	if err != nil {
		return err
	}

	configData, err := os.ReadFile(configFilePath)
	if err != nil {
		return err
	}

	configFile, err := config.ParseConfig(configData)
	if err != nil {
		return err
	}

	server_, err := server.NewServer(serverOptions...)
	if err != nil {
		return err
	}

	if ss.Debug {
		server_.RegisterDebugRoutes()
	}

	esConnectionLayer, ok := parsedLayers.Get(es_layers.EsConnectionSlug)
	if !ok {
		return errors.Errorf("Could not find layer %s", es_layers.EsConnectionSlug)
	}

	commandDirHandlerOptions := []command_dir.CommandDirHandlerOption{}
	templateDirHandlerOptions := []template_dir.TemplateDirHandlerOption{}

	// TODO(manuel, 2023-06-20): These should be able to be set from the config file itself.
	// See: https://github.com/go-go-golems/parka/issues/51
	devMode := ss.Dev

	// NOTE(manuel, 2023-12-13) Why do we append these to the config file?
	commandDirHandlerOptions = append(
		commandDirHandlerOptions,
		command_dir.WithGenericCommandHandlerOptions(
			generic_command.WithParameterFilterOptions(
				config.WithLayerDefaults(
					esConnectionLayer.Layer.GetSlug(),
					esConnectionLayer.Parameters.ToMap(),
				),
			),
			generic_command.WithDefaultTemplateName("data-tables.tmpl.html"),
			generic_command.WithDefaultIndexTemplateName("commands.tmpl.html"),
		),
		command_dir.WithDevMode(devMode),
	)

	templateDirHandlerOptions = append(
		// pass in the default parka renderer options for being able to render markdown files
		templateDirHandlerOptions,
		template_dir.WithAlwaysReload(devMode),
	)

	templateHandlerOptions := []template.TemplateHandlerOption{
		template.WithAlwaysReload(devMode),
	}

	cfh := handlers.NewConfigFileHandler(
		configFile,
		handlers.WithAppendCommandDirHandlerOptions(commandDirHandlerOptions...),
		handlers.WithAppendTemplateDirHandlerOptions(templateDirHandlerOptions...),
		handlers.WithAppendTemplateHandlerOptions(templateHandlerOptions...),
		handlers.WithRepositoryFactory(es_cmds.NewRepositoryFactory()),
		handlers.WithDevMode(devMode),
	)

	err = runConfigFileHandler(ctx, server_, cfh)
	if err != nil {
		return err
	}

	return nil
}

func (s *ServeCommand) Run(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
) error {
	ss := &ServeSettings{}
	err := parsedLayers.InitializeStruct(layers.DefaultSlug, ss)
	if err != nil {
		return err
	}

	serverOptions := []server.ServerOption{
		server.WithPort(uint16(ss.ServePort)),
		server.WithAddress(ss.ServeHost),
		server.WithGzip(),
	}

	if ss.ConfigFile != "" {
		return s.runWithConfigFile(ctx, parsedLayers, ss.ConfigFile, serverOptions)
	}

	configFile := &config.Config{
		Routes: []*config.Route{
			{
				Path: "/",
				CommandDirectory: &config.CommandDir{
					Repositories: s.repositories,
				},
			},
		},
	}

	contentDirs := ss.ContentDirs

	if len(contentDirs) > 1 {
		return fmt.Errorf("only one content directory is supported at the moment")
	}

	if len(contentDirs) == 1 {
		// resolve directory to absolute directory
		dir, err := filepath.Abs(contentDirs[0])
		if err != nil {
			return err
		}
		configFile.Routes = append(configFile.Routes, &config.Route{
			Path: "/",
			TemplateDirectory: &config.TemplateDir{
				LocalDirectory: dir,
			},
		})
	}

	server_, err := server.NewServer(serverOptions...)
	if err != nil {
		return err
	}

	if ss.Dev {
		server_.RegisterDebugRoutes()
	}

	esClientLayer, ok := parsedLayers.Get(es_layers.EsConnectionSlug)
	if !ok {
		return errors.Errorf("Could not find layer %s", es_layers.EsConnectionSlug)
	}

	commandDirHandlerOptions := []command_dir.CommandDirHandlerOption{
		command_dir.WithGenericCommandHandlerOptions(
			generic_command.WithTemplateLookup(datatables.NewDataTablesLookupTemplate()),
			generic_command.WithParameterFilterOptions(
				config.WithLayerDefaults(
					esClientLayer.Layer.GetSlug(),
					esClientLayer.Parameters.ToMap(),
				),
			),
			generic_command.WithDefaultTemplateName("data-tables.tmpl.html"),
			generic_command.WithDefaultIndexTemplateName(""),
		),
		command_dir.WithDevMode(ss.Dev),
	}

	templateDirHandlerOptions := []template_dir.TemplateDirHandlerOption{
		template_dir.WithAlwaysReload(ss.Dev),
	}

	err = configFile.Initialize()
	if err != nil {
		return err
	}

	cfh := handlers.NewConfigFileHandler(
		configFile,
		handlers.WithAppendCommandDirHandlerOptions(commandDirHandlerOptions...),
		handlers.WithAppendTemplateDirHandlerOptions(templateDirHandlerOptions...),
		handlers.WithRepositoryFactory(es_cmds.NewRepositoryFactory()),
		handlers.WithDevMode(ss.Dev),
	)

	err = runConfigFileHandler(ctx, server_, cfh)
	if err != nil {
		return err
	}

	return nil
}

func runConfigFileHandler(
	ctx context.Context,
	server_ *server.Server,
	cfh *handlers.ConfigFileHandler,
) error {
	err := cfh.Serve(server_)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	errGroup, ctx := errgroup.WithContext(ctx)
	errGroup.Go(func() error {
		return cfh.Watch(ctx)
	})
	errGroup.Go(func() error {
		return server_.Run(ctx)
	})

	err = errGroup.Wait()
	if err != nil {
		return err
	}

	return nil
}
