// VulcanizeDB
// Copyright © 2019 Vulcanize

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cmd

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/vulcanize/ipld-eth-state-snapshot/pkg/prom"
	"github.com/vulcanize/ipld-eth-state-snapshot/pkg/snapshot"
)

var (
	cfgFile        string
	subCommand     string
	logWithCommand log.Entry
)

var rootCmd = &cobra.Command{
	Use:              "ipld-eth-state-snapshot",
	PersistentPreRun: initFuncs,
}

// Execute executes root Command.
func Execute() {
	log.Info("----- Starting vDB -----")
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func initFuncs(cmd *cobra.Command, args []string) {
	logfile := viper.GetString(snapshot.LOGRUS_FILE_TOML)
	if logfile != "" {
		file, err := os.OpenFile(logfile,
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			log.Infof("Directing output to %s", logfile)
			log.SetOutput(file)
		} else {
			log.SetOutput(os.Stdout)
			log.Info("Failed to log to file, using default stdout")
		}
	} else {
		log.SetOutput(os.Stdout)
	}
	if err := logLevel(); err != nil {
		log.Fatal("Could not set log level: ", err)
	}

	if viper.GetBool(snapshot.PROM_METRICS_TOML) {
		log.Info("initializing prometheus metrics")
		prom.Init()
	}

	if viper.GetBool(snapshot.PROM_HTTP_TOML) {
		addr := fmt.Sprintf(
			"%s:%s",
			viper.GetString(snapshot.PROM_HTTP_ADDR_TOML),
			viper.GetString(snapshot.PROM_HTTP_PORT_TOML),
		)
		log.Info("starting prometheus server")
		prom.Serve(addr)
	}
}

func logLevel() error {
	lvl, err := log.ParseLevel(viper.GetString(snapshot.LOGRUS_LEVEL_TOML))
	if err != nil {
		return err
	}
	log.SetLevel(lvl)
	if lvl > log.InfoLevel {
		log.SetReportCaller(true)
	}
	log.Info("Log level set to ", lvl.String())

	return nil
}

func init() {
	cobra.OnInitialize(initConfig)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file location")
	rootCmd.PersistentFlags().String(snapshot.LOGRUS_FILE_CLI, "", "file path for logging")
	rootCmd.PersistentFlags().String(snapshot.DATABASE_NAME_CLI, "vulcanize_public", "database name")
	rootCmd.PersistentFlags().Int(snapshot.DATABASE_PORT_CLI, 5432, "database port")
	rootCmd.PersistentFlags().String(snapshot.DATABASE_HOSTNAME_CLI, "localhost", "database hostname")
	rootCmd.PersistentFlags().String(snapshot.DATABASE_USER_CLI, "", "database user")
	rootCmd.PersistentFlags().String(snapshot.DATABASE_PASSWORD_CLI, "", "database password")
	rootCmd.PersistentFlags().String(snapshot.LOGRUS_LEVEL_CLI, log.InfoLevel.String(), "log level (trace, debug, info, warn, error, fatal, panic)")

	rootCmd.PersistentFlags().Bool(snapshot.PROM_METRICS_CLI, false, "enable prometheus metrics")
	rootCmd.PersistentFlags().Bool(snapshot.PROM_HTTP_CLI, false, "enable prometheus http service")
	rootCmd.PersistentFlags().String(snapshot.PROM_HTTP_ADDR_CLI, "127.0.0.1", "prometheus http host")
	rootCmd.PersistentFlags().String(snapshot.PROM_HTTP_PORT_CLI, "8086", "prometheus http port")
	rootCmd.PersistentFlags().Bool(snapshot.PROM_DB_STATS_CLI, false, "enables prometheus db stats")

	viper.BindPFlag(snapshot.LOGRUS_FILE_TOML, rootCmd.PersistentFlags().Lookup(snapshot.LOGRUS_FILE_CLI))
	viper.BindPFlag(snapshot.DATABASE_NAME_TOML, rootCmd.PersistentFlags().Lookup(snapshot.DATABASE_NAME_CLI))
	viper.BindPFlag(snapshot.DATABASE_PORT_TOML, rootCmd.PersistentFlags().Lookup(snapshot.DATABASE_PORT_CLI))
	viper.BindPFlag(snapshot.DATABASE_HOSTNAME_TOML, rootCmd.PersistentFlags().Lookup(snapshot.DATABASE_HOSTNAME_CLI))
	viper.BindPFlag(snapshot.DATABASE_USER_TOML, rootCmd.PersistentFlags().Lookup(snapshot.DATABASE_USER_CLI))
	viper.BindPFlag(snapshot.DATABASE_PASSWORD_TOML, rootCmd.PersistentFlags().Lookup(snapshot.DATABASE_PASSWORD_CLI))
	viper.BindPFlag(snapshot.LOGRUS_LEVEL_TOML, rootCmd.PersistentFlags().Lookup(snapshot.LOGRUS_LEVEL_CLI))

	viper.BindPFlag(snapshot.PROM_METRICS_TOML, rootCmd.PersistentFlags().Lookup(snapshot.PROM_METRICS_CLI))
	viper.BindPFlag(snapshot.PROM_HTTP_TOML, rootCmd.PersistentFlags().Lookup(snapshot.PROM_HTTP_CLI))
	viper.BindPFlag(snapshot.PROM_HTTP_ADDR_TOML, rootCmd.PersistentFlags().Lookup(snapshot.PROM_HTTP_ADDR_CLI))
	viper.BindPFlag(snapshot.PROM_HTTP_PORT_TOML, rootCmd.PersistentFlags().Lookup(snapshot.PROM_HTTP_PORT_CLI))
	viper.BindPFlag(snapshot.PROM_DB_STATS_TOML, rootCmd.PersistentFlags().Lookup(snapshot.PROM_DB_STATS_CLI))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		if err := viper.ReadInConfig(); err == nil {
			log.Printf("Using config file: %s", viper.ConfigFileUsed())
		} else {
			log.Fatal(fmt.Sprintf("Couldn't read config file: %s", err.Error()))
		}
	} else {
		log.Warn("No config file passed with --config flag")
	}
}
