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
	"github.com/dprosper/vpc-flowlogs-elasticsearch/internal/flowlogs"
	"github.com/spf13/cobra"
)

// searchCmd represents the serve command
var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Performs a search in Elasticsearch.",
	Run: func(cmd *cobra.Command, args []string) {
		flowlogs.Search(query, trace)
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)

	searchCmd.Flags().StringVar(&query, "query", "", "name of the query to run")
	searchCmd.MarkFlagRequired("query")

	searchCmd.Flags().BoolVar(&trace, "trace", false, "When set will add elasticsearch request and response body to the output")
}
