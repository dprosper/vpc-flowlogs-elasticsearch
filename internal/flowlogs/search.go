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
	"os"

	"github.com/dprosper/vpc-flowlogs-elasticsearch/internal/logger"
	"github.com/elastic/go-elasticsearch/v6"
	"github.com/elastic/go-elasticsearch/v6/estransport"
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
		esIndexName = `ibm_vpc_flowlogs_v1`
		esAddresses []string
		esUsername  = viper.GetString("elasticsearch.username")
		esPassword  = viper.GetString("elasticsearch.password")
		esCert      = viper.GetString("elasticsearch.certificate.certificate_base64")
		cfg         elasticsearch.Config
	)

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
	} else {
		body, _ := ioutil.ReadAll(res.Body)
		serverVersion := gjson.GetBytes(body, "version.number")
		logger.SystemLogger.Info("Client Info", zap.String("Client version:", elasticsearch.Version), zap.String("Server version:", serverVersion.String()))
	}

	queries, _ := ioutil.ReadFile("config/queries.json")
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

	output.ForEach(func(key, value gjson.Result) bool {
		name := gjson.Get(value.String(), "name").String()
		valueof := gjson.Get(value.String(), "valueof").String()
		r := queryResult{Name: name, Value: gjson.GetBytes(body, valueof).String()}
		qr = append(qr, r)

		return true // keep iterating
	})

	commandResult, err := json.Marshal(qr)
	if err != nil {
		logger.ErrorLogger.Error("Error in marshalling results", zap.String("error: ", err.Error()))
		return
	}
	println(string(commandResult))

	return

}
