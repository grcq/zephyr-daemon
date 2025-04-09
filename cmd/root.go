package cmd

import (
	"daemon/config"
	"daemon/router"
	"daemon/server"
	"daemon/testing"
	"daemon/utils"
	"github.com/apex/log"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"strconv"
)

var rootCmd = &cobra.Command{
	Use: "nuxion",
	PreRun: func(cmd *cobra.Command, args []string) {
		runByApp := os.Getenv("ZEPHYR_DAEMON_IGNITION")
		if runByApp != "true" {
			return
		}

		var path = config.DefaultPath
		if cmd.Flags().Changed("config") {
			path, _ = cmd.Flags().GetString("config")
		}

		c, err := config.Load(path)
		if err != nil {
			log.WithField("path.go", path).Info("config not found or invalid, creating default")
			conf, err := config.Set(config.DefaultConfig(path))
			if err != nil {
				log.WithError(err).Fatal("failed to set default config")
			}

			if err := conf.Save(); err != nil {
				log.WithError(err).Fatal("failed to save default config")
			}

			c = conf
		}

		if debug, _ := cmd.Flags().GetBool("debug"); debug {
			c.Debug = true
			log.SetLevel(log.DebugLevel)
			log.Debug("running in debug mode")
		}

		initFiles(c)
	},
	Run: mainRunCmd,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringP("config", "c", config.DefaultPath, "config file path.go")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "enable debug mode")
	rootCmd.PersistentFlags().BoolP("test", "t", false, "enable testing mode")
}

func mainRunCmd(cmd *cobra.Command, args []string) {
	runByApp := os.Getenv("ZEPHYR_DAEMON_IGNITION")
	if runByApp != "true" {
		log.Errorf("this binary should be run by the app itself or in development mode")
		return
	}

	log.Info("running main command")

	c := config.Get()
	log.WithField("config", c).Info("loaded config")

	if t, _ := cmd.Flags().GetBool("test"); t {
		log.Info("running in testing mode")
		c.DataPath = "test/data"
		c.VolumesPath = "test/volumes"

		testing.RunTests()
	}

	load(c)

	s := http.Server{
		Addr:    c.Server.Bind + ":" + strconv.Itoa(c.Server.Port),
		Handler: router.Configure(),
	}

	log.WithField("addr", s.Addr).Info("server started on " + s.Addr)
	if c.Server.TLS.Enabled {
		if err := s.ListenAndServeTLS(c.Server.TLS.Cert, c.Server.TLS.Key); err != nil {
			log.WithError(err).Fatal("failed to start server")
		}
	} else {
		if err := s.ListenAndServe(); err != nil {
			log.WithError(err).Fatal("failed to start server")
		}
	}
}

func load(c *config.Config) {
	server.Load(c)
}

func initFiles(c *config.Config) {
	log.Info("initializing files")
	dataPath := utils.Normalize(c.DataPath)
	volumesPath := utils.Normalize(c.VolumesPath)

	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		log.Debugf("data path.go %s does not exist, creating", dataPath)
		if err := os.MkdirAll(dataPath, 0755); err != nil {
			log.WithError(err).Fatal("failed to create data path.go")
		}
	}

	if _, err := os.Stat(volumesPath); os.IsNotExist(err) {
		log.Debugf("volumes path.go %s does not exist, creating", volumesPath)
		if err := os.MkdirAll(volumesPath, 0755); err != nil {
			log.WithError(err).Fatal("failed to create volumes path.go")
		}
	}

	templatesFolder := dataPath + "/templates"
	if _, err := os.Stat(templatesFolder); os.IsNotExist(err) {
		log.Debugf("templates path.go %s does not exist, creating", templatesFolder)
		if err := os.MkdirAll(templatesFolder, 0755); err != nil {
			log.WithError(err).Fatal("failed to create templates path.go")
		}

		// todo: download default templates
	}
}
