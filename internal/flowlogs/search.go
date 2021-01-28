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
package flowlogs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/dprosper/vpc-flowlogs-elasticsearch/internal/logger"
	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/estransport"
	"github.com/manifoldco/promptui"
	"github.com/spf13/viper"
	"github.com/tidwall/gjson"

	"go.uber.org/zap"
)

type queryResult struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Search function
func Search(queryName string, trace bool) string {
	search(queryName, trace)
	return "done"
}

func search(queryName string, trace bool) (result *map[string]interface{}) {

	result = nil

	var (
		esIndexName = viper.GetString("elasticsearch.indexName")
		esAddresses []string
		esUsername  = viper.GetString("elasticsearch.username")
		esPassword  = viper.GetString("elasticsearch.password")
		esCert      = viper.GetString("elasticsearch.certificate.certificate_base64")
		cfg         elasticsearch.Config
	)

	if !validateKey(esUsername) {
		log.Fatalln("elasticsearch.username or ELASTICSEARCH_USERNAME not provided ")
	}
	if !validateKey(esPassword) {
		log.Fatalln("elasticsearch.password or ELASTICSEARCH_PASSWORD not provided ")
	}
	if !validateKey(esCert) {
		log.Fatalln("elasticsearch.certificate.certificate_base64 or ELASTICSEARCH_CERTIFICATE_CERTIFICATE_BASE64 not provided ")
	}
	if !validateKey(esIndexName) {
		log.Fatalln("elasticsearch.indexName or ELASTICSEARCH_INDEXNAME not provided ")
	}

	cert, err := base64.StdEncoding.DecodeString(esCert)
	if err != nil {
		logger.ErrorLogger.Error("Error decoding certificate for elasticsearch.", zap.String("error: ", err.Error()))
		return
	}

	esAddresses = append(esAddresses, fmt.Sprintf("https://%s:%s", viper.GetString("elasticsearch.hostname"), viper.GetString("elasticsearch.port")))

	if trace {
		cfg = elasticsearch.Config{
			Addresses: esAddresses,
			Username:  esUsername,
			Password:  esPassword,
			CACert:    cert,
			Logger: &estransport.ColorLogger{
				Output:             os.Stdout,
				EnableRequestBody:  true,
				EnableResponseBody: true,
			},
		}
	} else {
		cfg = elasticsearch.Config{
			Addresses: esAddresses,
			Username:  esUsername,
			Password:  esPassword,
			CACert:    cert,
		}
	}

	esClient, err := elasticsearch.NewClient(cfg)
	if err != nil {
		logger.ErrorLogger.Error("Error creating elasticsearch client.", zap.String("error: ", err.Error()))
		return
	}

	res, err := esClient.Info()
	if err != nil {
		logger.ErrorLogger.Error("Error in getting Client Info", zap.String("error: ", err.Error()))
		return
	}

	if res.IsError() {
		logger.ErrorLogger.Error("Error in getting Client Info", zap.String("error: ", res.String()))
	}
	// else {
	// 	body, _ := ioutil.ReadAll(res.Body)
	// 	serverVersion := gjson.GetBytes(body, "version.number")
	// 	logger.SystemLogger.Info("Client Info", zap.String("Client version:", elasticsearch.Version), zap.String("Server version:", serverVersion.String()))
	// }

	queries, _ := ioutil.ReadFile("config/queries.json")

	if queryName == "" {
		queryList := gjson.GetBytes(queries, "queries.#.name")
		var items []string
		for _, name := range queryList.Array() {
			items = append(items, name.String())
		}
		prompt := promptui.Select{
			Label: "Select a query",
			Items: items,
		}
		_, queryName, err = prompt.Run()

		if err != nil {
			logger.ErrorLogger.Error("Error prompting for query", zap.String("error: ", err.Error()))
			return
		}
	}

	query := gjson.GetBytes(queries, "queries.#(name==\""+queryName+"\").command")
	var buf bytes.Buffer

	if err := json.NewEncoder(&buf).Encode(query.Value()); err != nil {
		logger.ErrorLogger.Error("Error encoding query", zap.String("error: ", err.Error()))
		return
	}

	res, err = esClient.Search(
		esClient.Search.WithContext(context.Background()),
		esClient.Search.WithIndex(esIndexName),
		esClient.Search.WithBody(&buf),
		esClient.Search.WithTrackTotalHits(true),
		esClient.Search.WithPretty(),
	)
	if err != nil {
		logger.ErrorLogger.Error("Error getting response from search", zap.String("error: ", err.Error()))
		return
	}
	defer res.Body.Close()

	body, _ := ioutil.ReadAll(res.Body)
	var qr []queryResult

	output := gjson.GetBytes(queries, "queries.#(name==\""+queryName+"\").output")
	var commandResult []byte
	if output.String() != "" {
		output.ForEach(func(key, value gjson.Result) bool {
			name := gjson.Get(value.String(), "name").String()
			valueof := gjson.Get(value.String(), "valueof").String()

			if strings.Contains(valueof, "#") {
				before := valueof[0:strings.Index(valueof, ".#")]
				after := valueof[strings.LastIndex(valueof, "#.")+2 : len(valueof)]

				buckets := gjson.GetBytes(body, before)
				buckets.ForEach(func(key, value gjson.Result) bool {
					r := queryResult{Name: name, Value: gjson.Get(value.String(), after).String()}
					qr = append(qr, r)

					return true
				})

			} else {
				r := queryResult{Name: name, Value: gjson.GetBytes(body, valueof).String()}
				qr = append(qr, r)
			}
			return true // keep iterating
		})

		commandResult, err = json.Marshal(qr)
		if err != nil {
			logger.ErrorLogger.Error("Error in marshalling results", zap.String("error: ", err.Error()))
			return
		}
	} else {
		commandResult = body
	}

	fmt.Println(string(commandResult))

	return

}
