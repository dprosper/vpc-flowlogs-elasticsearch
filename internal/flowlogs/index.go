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

func validateKey(key string) bool {
	if key == "" {
		return false
	}
	return true
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

	if !validateKey(apiKey) {
		log.Fatalln("cos.apikey or COS_APIKEY not provided ")
	}
	if !validateKey(serviceInstanceID) {
		log.Fatalln("cos.resource_instance_id or COS_RESOURCE_INSTANCE_ID not provided ")
	}
	if !validateKey(authEndpoint) {
		log.Fatalln("ibmcloud.iamUrl or IBMCLOUD_IAMURL not provided ")
	}
	if !validateKey(serviceEndpoint) {
		log.Fatalln("cos.serviceEndpoint or COS_SERVICEENDPOINT not provided ")
	}
	if !validateKey(bucketsLocation) {
		log.Fatalln("cos.bucketsLocation or COS_BUCKETSLOCATION not provided ")
	}
	if !validateKey(sourceBucketName) {
		log.Fatalln("cos.sourceBucketName or COS_SOURCEBUCKETNAME not provided ")
	}
	if !validateKey(indexedBucketName) {
		log.Fatalln("cos.indexedBucketName or COS_INDEXEDBUCKETNAME not provided ")
	}
	if !validateKey(esIndexName) {
		log.Fatalln("elasticsearch.indexName or ELASTICSEARCH_INDEXNAME not provided ")
	}
	if !validateKey(esIndexMapping) {
		log.Fatalln("elasticsearch.indexMapping or ELASTICSEARCH_INDEXMAPPING not provided ")
	}
	if !validateKey(esUsername) {
		log.Fatalln("elasticsearch.username or ELASTICSEARCH_USERNAME not provided ")
	}
	if !validateKey(esPassword) {
		log.Fatalln("elasticsearch.password or ELASTICSEARCH_PASSWORD not provided ")
	}
	if !validateKey(esCert) {
		log.Fatalln("elasticsearch.certificate.certificate_base64 or ELASTICSEARCH_CERTIFICATE_CERTIFICATE_BASE64 not provided ")
	}

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
	logger.SystemLogger.Debug("Client Info", zap.String("Client version:", elasticsearch.Version), zap.String("Server version:", serverVersion.String()))

	res, err = esClient.Indices.Exists([]string{esIndexName})
	if res.Status() != "200 OK" {
		indexMapping, _ := ioutil.ReadFile("config/" + esIndexMapping)

		res, err = esClient.Indices.Create(esIndexName, esClient.Indices.Create.WithBody(bytes.NewReader(indexMapping)))
		if err != nil || res.IsError() {
			logger.ErrorLogger.Error("Cannot create index", zap.String("error: ", err.Error()))
			return fmt.Errorf("esClient.Indices.Create: %v", err)
		}
		logger.SystemLogger.Debug(fmt.Sprintf("Created a new index: %s", esIndexName))

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

		logger.SystemLogger.Info(fmt.Sprintf("Adding 25 or less objects to bulk index from: %s", sourceBucketName))

		for _, object := range objects.Contents {
			key := *object.Key
			sha256DocumentID := fmt.Sprintf("%x", sha256.Sum256([]byte(key)))

			logger.SystemLogger.Debug(fmt.Sprintf("[%s] Read from COS bucket.", sha256DocumentID))

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

			flowlog1 := strings.Replace(string(flowlog), "\"start_time\":\"\"", "\"start_time\":null", -1)
			flowlog2 := strings.Replace(string(flowlog1), "\"end_time\":\"\"", "\"end_time\":null", -1)

			bierr := bi.Add(
				context.Background(),
				esutil.BulkIndexerItem{
					Action:       "index",
					DocumentID:   sha256DocumentID,
					DocumentType: "flowlog",
					Body:         strings.NewReader(flowlog2),

					OnSuccess: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem) {
						atomic.AddUint64(&countSuccessful, 1)
						logger.SystemLogger.Info(fmt.Sprintf("[%s] Successfully added to index.", sha256DocumentID))

						copyObjectInput := s3.CopyObjectInput{
							Bucket:     aws.String(indexedBucketName),
							CopySource: aws.String(sourceBucketName + "/" + key),
							Key:        aws.String(key),
						}
						_, err := cosClient.CopyObject(&copyObjectInput)
						if err != nil {
							logger.ErrorLogger.Error(fmt.Sprintf("[%s] ERROR copying object: %s", sha256DocumentID, err))
						} else {
							logger.SystemLogger.Debug(fmt.Sprintf("[%s] Copied to: %s.", sha256DocumentID, indexedBucketName))

							deleteObjectInput := s3.DeleteObjectInput{
								Bucket: aws.String(sourceBucketName),
								Key:    aws.String(key),
							}
							_, err = cosClient.DeleteObject(&deleteObjectInput)
							if err != nil {
								logger.ErrorLogger.Error(fmt.Sprintf("[%s] ERROR deleting object: %s", sha256DocumentID, err))
							}
							logger.SystemLogger.Debug(fmt.Sprintf("[%s] Deleted from: %s.", sha256DocumentID, sourceBucketName))
						}

					},

					OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem, err error) {
						if err != nil {
							logger.ErrorLogger.Error(fmt.Sprintf("[%s] ERROR: %s", sha256DocumentID, err))
						} else {
							logger.ErrorLogger.Error(fmt.Sprintf("[%s] ERROR: %s: %s", sha256DocumentID, res.Error.Type, res.Error.Reason))
						}
					},
				},
			)

			if bierr != nil {
				logger.ErrorLogger.Error("Unexpected error.", zap.String("error: ", err.Error()))
			}

		}

		logger.SystemLogger.Debug(fmt.Sprintf("Added 25 or less objects to bulk index from: %s", sourceBucketName))

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
