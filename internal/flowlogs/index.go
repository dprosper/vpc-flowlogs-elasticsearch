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
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials/ibmiam"
	"github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3"
	"github.com/dprosper/vpc-flowlogs-elasticsearch/internal/logger"
	"github.com/dustin/go-humanize"
	"github.com/elastic/go-elasticsearch/v6"
	"github.com/elastic/go-elasticsearch/v6/estransport"
	"github.com/elastic/go-elasticsearch/v6/esutil"
	"github.com/spf13/viper"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// Index function
func Index(trace bool) string {
	bulkIndex(trace)
	return "done"
}

// bulkIndex function
func bulkIndex(trace bool) error {

	var (
		countSuccessful   uint64
		apiKey            = viper.GetString("cos.apikey")
		serviceInstanceID = viper.GetString("cos.resource_instance_id")
		authEndpoint      = viper.GetString("ibmcloud.iamUrl")
		serviceEndpoint   = viper.GetString("cos.serviceEndpoint")
		bucketsLocation   = viper.GetString("cos.bucketsLocation")
		sourceBucketName  = viper.GetString("cos.sourceBucketName")
		indexedBucketName = viper.GetString("cos.indexedBucketName")
		esAddresses       []string
		esIndexName       = viper.GetString("elasticsearch.indexName")
		esIndexMapping    = viper.GetString("elasticsearch.indexMapping")
		esUsername        = viper.GetString("elasticsearch.username")
		esPassword        = viper.GetString("elasticsearch.password")
		esCert            = viper.GetString("elasticsearch.certificate.certificate_base64")
		cfg               elasticsearch.Config
	)

	cert, err := base64.StdEncoding.DecodeString(esCert)
	if err != nil {
		logger.ErrorLogger.Error("Error decoding certificate for elasticsearch.", zap.String("error: ", err.Error()))
		return fmt.Errorf("base64.StdEncoding.DecodeString: %v", err)
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
			Logger: &estransport.ColorLogger{
				Output: os.Stdout,
			},
		}
	}

	esClient, err := elasticsearch.NewClient(cfg)
	if err != nil {
		logger.ErrorLogger.Error("Error creating elasticsearch client.", zap.String("error: ", err.Error()))
		return fmt.Errorf("elasticsearch.NewClient: %v", err)
	}

	res, err := esClient.Info()
	if err != nil || res.IsError() {
		logger.ErrorLogger.Error("Error in getting Client Info", zap.String("error: ", err.Error()))
		return fmt.Errorf("esClient.Info: %v", err)
	}

	body, _ := ioutil.ReadAll(res.Body)
	serverVersion := gjson.GetBytes(body, "version.number")
	logger.SystemLogger.Info("Client Info", zap.String("Client version:", elasticsearch.Version), zap.String("Server version:", serverVersion.String()))

	res, err = esClient.Indices.Exists([]string{esIndexName})
	if res.Status() != "200 OK" {
		indexMapping, _ := ioutil.ReadFile("config/" + esIndexMapping)

		res, err = esClient.Indices.Delete([]string{esIndexName}, esClient.Indices.Delete.WithIgnoreUnavailable(true))
		if err != nil || res.IsError() {
			logger.ErrorLogger.Error("Cannot delete index", zap.String("error: ", err.Error()))
			return fmt.Errorf("esClient.Indices.Delete: %v", err)
		}
		res.Body.Close()

		res, err = esClient.Indices.Create(esIndexName, esClient.Indices.Create.WithBody(bytes.NewReader(indexMapping)))
		if err != nil || res.IsError() {
			logger.ErrorLogger.Error("Cannot create index", zap.String("error: ", err.Error()))
			return fmt.Errorf("esClient.Indices.Create: %v", err)
		}
		res.Body.Close()
	}

	bi, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Index:         esIndexName,
		Client:        esClient,
		NumWorkers:    runtime.NumCPU(),
		FlushBytes:    int(5e+6),
		FlushInterval: 30 * time.Second,
	})

	if err != nil {
		logger.ErrorLogger.Error("Error creating the indexer.", zap.String("error: ", err.Error()))
		return fmt.Errorf("esutil.NewBulkIndexer: %v", err)
	}

	conf := aws.NewConfig().
		WithRegion(bucketsLocation).
		WithEndpoint(serviceEndpoint).
		WithCredentials(ibmiam.NewStaticCredentials(aws.NewConfig(), authEndpoint, apiKey, serviceInstanceID)).
		WithS3ForcePathStyle(true)

	sess := session.Must(session.NewSession(&aws.Config{
		MaxRetries: aws.Int(3),
	}))

	cosClient := s3.New(sess, conf)

	continuationToken := ""
	previousKey := ""

	start := time.Now().UTC()
	for {
		listInput := &s3.ListObjectsV2Input{
			Bucket:            aws.String(sourceBucketName),
			MaxKeys:           aws.Int64(25),
			ContinuationToken: aws.String(continuationToken),
			StartAfter:        aws.String(previousKey),
		}

		objects, err := cosClient.ListObjectsV2(listInput)
		if err != nil {
			logger.ErrorLogger.Error("Error in getting bucket objects.", zap.String("error: ", err.Error()))
			return fmt.Errorf("cosClient.ListObjectsV2: %v", err)
		}

		logger.SystemLogger.Info(fmt.Sprintf("Bulk indexing 25 or less objects from: %s", sourceBucketName))

		for _, object := range objects.Contents {

			key := *object.Key
			sha256DocumentID := fmt.Sprintf("%x", sha256.Sum256([]byte(key)))

			objectInput := s3.GetObjectInput{
				Bucket: aws.String(sourceBucketName),
				Key:    aws.String(key),
			}

			res, err := cosClient.GetObject(&objectInput)
			if err != nil {
				logger.ErrorLogger.Error(fmt.Sprintf("[%s] ERROR: %s", *aws.String(key), err))
				continue
			}

			flowlog, _ := ioutil.ReadAll(res.Body)

			bierr := bi.Add(
				context.Background(),
				esutil.BulkIndexerItem{
					Action:       "index",
					DocumentID:   sha256DocumentID,
					DocumentType: "flowlog",
					Body:         strings.NewReader(string(flowlog)),

					OnSuccess: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem) {
						atomic.AddUint64(&countSuccessful, 1)
						// logger.SystemLogger.Debug(fmt.Sprintf("[%s] Indexed.", sha256DocumentID))

						copyObjectInput := s3.CopyObjectInput{
							Bucket:     aws.String(indexedBucketName),
							CopySource: aws.String(sourceBucketName + "/" + key),
							Key:        aws.String(key),
						}
						_, err := cosClient.CopyObject(&copyObjectInput)
						if err != nil {
							logger.ErrorLogger.Error(fmt.Sprintf("[%s] ERROR copying object: %s", sha256DocumentID, err))
						} else {
							deleteObjectInput := s3.DeleteObjectInput{
								Bucket: aws.String(sourceBucketName),
								Key:    aws.String(key),
							}
							_, err = cosClient.DeleteObject(&deleteObjectInput)
							if err != nil {
								logger.ErrorLogger.Error(fmt.Sprintf("[%s] ERROR deleting object: %s", sha256DocumentID, err))
							}
						}

					},

					OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem, err error) {
						if err != nil {
							logger.ErrorLogger.Error(fmt.Sprintf("[%s] ERROR: %s", sha256DocumentID, err))
						} else {
							log.Printf("ERROR: %s: %s", res.Error.Type, res.Error.Reason)
							logger.ErrorLogger.Error(fmt.Sprintf("[%s] ERROR: %s: %s", sha256DocumentID, res.Error.Type, res.Error.Reason))
						}
					},
				},
			)

			if bierr != nil {
				logger.ErrorLogger.Error("Unexpected error.", zap.String("error: ", err.Error()))
			}

		}

		// logger.SystemLogger.Info(fmt.Sprintf("Completed bulk indexing of 25 or less objects from: %s", sourceBucketName))

		if *objects.IsTruncated {
			continuationToken = *objects.NextContinuationToken
		} else {
			break
		}
	}

	if err := bi.Close(context.Background()); err != nil {
		logger.ErrorLogger.Error("Unexpected error.", zap.String("error: ", err.Error()))
	}

	biStats := bi.Stats()

	duration := time.Since(start)
	logger.SystemLogger.Info(fmt.Sprintf("Indexed [%s] documents with [%s] errors in %s (%s docs/sec)",
		humanize.Comma(int64(biStats.NumFlushed)),
		humanize.Comma(int64(biStats.NumFailed)),
		duration.Truncate(time.Millisecond),
		humanize.Comma(int64(1000.0/float64(duration/time.Millisecond)*float64(biStats.NumFlushed)))))

	return nil
}
