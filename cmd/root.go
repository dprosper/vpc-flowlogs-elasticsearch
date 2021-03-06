/*
Copyright © 2020 Dimitri Prosper <dimitri_prosper@us.ibm.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/dprosper/vpc-flowlogs-elasticsearch/internal/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var trace bool
var recreateIndex bool
var query string

var rootCmd = &cobra.Command{
	Use:   "vpc-flowlogs-elasticsearch",
	Short: "Indexer for VPC Flowlogs in IBM Cloud",
	Long:  `.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "configuation file (default is $HOME/.flowlogs.json)")

}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.SetConfigType("json")
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath("$HOME")
		viper.SetConfigName("flowlogs")
	}

	viper.AutomaticEnv()
	logger.InitLogger()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	err := viper.ReadInConfig()
	if err != nil {
		log.Println("warning: configuration file not found, expecting environment variables to be set.")
	}

	if _, err := os.Stat("config/queries.json"); os.IsNotExist(err) {
		log.Println("warning: unable to find config/queries.json, make sure the directory exist and is located at the same level as the binary you are running.")
		os.Exit(1)
	}

	if _, err := os.Stat("config/flowlogs-v1.json"); os.IsNotExist(err) {
		log.Println("warning: unable to find config/flowlogs-v1.json, make sure the directory exist and is located at the same level as the binary you are running.")
		os.Exit(1)
	}

}
