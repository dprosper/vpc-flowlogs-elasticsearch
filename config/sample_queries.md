### https://<elasticsearch_cluster_hostname>:<elasticsearch_cluster_port>/ibm_vpc_flowlogs_v1/\_search

#### 25 Top target IP addresses found

```json
{
  "size": 0,
  "aggregations": {
    "target_ips": {
      "terms": {
        "field": "flow_logs.target_ip.keyword",
        "size": 25
      }
    }
  }
}
```

#### 25 Top initiator IP addresses found

```json
{
  "size": 0,
  "aggregations": {
    "initiator_ips": {
      "terms": {
        "field": "flow_logs.initiator_ip.keyword",
        "size": 25
      }
    }
  }
}
```

#### Total Inbound and Outbound connections found (verify)

```json
{
  "size": 0,
  "aggregations": {
    "directions": {
      "terms": {
        "field": "flow_logs.direction.keyword",
        "size": 2
      }
    }
  }
}
```

#### Total accepted/rejected found

```json
{
  "size": 0,
  "aggregations": {
    "actions": {
      "terms": {
        "field": "flow_logs.action.keyword",
        "size": 2
      }
    }
  }
}
```

#### 25 Top target ports

```json
{
  "size": 0,
  "aggregations": {
    "target_ports": {
      "terms": {
        "field": "flow_logs.target_port",
        "size": 25
      }
    }
  }
}
```

#### 25 Top transport protocols

```json
{
  "size": 0,
  "aggregations": {
    "transport_protocols": {
      "terms": {
        "field": "flow_logs.transport_protocol",
        "size": 25
      }
    }
  }
}
```

### https://<elasticsearch_cluster_hostname>:<elasticsearch_cluster_port>/ibm_vpc_flowlogs_v1/\_search?scroll=1m

> **Endpoint with scroll token to paginate through results**

#### All logs with 50 page size

```json
{
  "size": 50,
  "query": {
    "match_all": {}
  }
}
```

#### All logs between yesterday and today with 50 page size

```json
{
  "size": 50,
  "query": {
    "range": {
      "capture_start_time": {
        "gte": "now-1d/d",
        "lt": "now/d"
      }
    }
  }
}
```

#### All logs from a specific VPC instance, targetting a specific IP and port number within the last 14 days with 50 page size

```json
{
  "query": {
    "bool": {
      "must": [
        {
          "match": {
            "instance_crn": "crn:v1:bluemix:public:is:us-east-1:a/00bbecaae6a8c4b4fdc16531663a1aec::instance:0757_80497c1b-3530-4b0f-8dd8-2cf721449655"
          }
        },
        {
          "match": {
            "flow_logs.target_ip": "10.241.0.1"
          }
        },
        {
          "match": {
            "flow_logs.target_port": 22
          }
        }
      ],
      "filter": [
        {
          "range": {
            "capture_start_time": {
              "gte": "now-14d/d",
              "lt": "now/d"
            }
          }
        }
      ]
    }
  }
}
```

#### All logs from a specific VPC instance, that have been rejected by Network ACL and/or Security Group policies within the last 14 days with 50 page size

```json
{
  "query": {
    "bool": {
      "must": [
        {
          "match": {
            "instance_crn": "crn:v1:bluemix:public:is:us-east-1:a/00bbecaae6a8c4b4fdc16531663a1aec::instance:0757_ad5c7bb3-4efa-4633-83c8-8c6464494683"
          }
        },
        {
          "match": {
            "flow_logs.action": "rejected"
          }
        }
      ],
      "filter": [
        {
          "range": {
            "capture_start_time": {
              "gte": "now-14d/d",
              "lt": "now/d"
            }
          }
        }
      ]
    }
  }
}
```

#### All logs from a specific VPC instance, that have been rejected by Network ACL and/or Security Group policies within the last 14 days with aggregation of the top 25 initiator IP addresses

```json
{
  "query": {
    "bool": {
      "must": [
        {
          "match": {
            "instance_crn": "crn:v1:bluemix:public:is:us-east-1:a/00bbecaae6a8c4b4fdc16531663a1aec::instance:0757_ad5c7bb3-4efa-4633-83c8-8c6464494683"
          }
        },
        {
          "match": {
            "flow_logs.action": "rejected"
          }
        }
      ],
      "filter": [
        {
          "range": {
            "capture_start_time": {
              "gte": "now-14d/d",
              "lt": "now/d"
            }
          }
        }
      ]
    }
  },
  "aggregations": {
    "initiator_ips": {
      "terms": {
        "field": "flow_logs.initiator_ip.keyword",
        "size": 25
      }
    }
  }
}
```

#### All logs from a specific VPC instance, that have been rejected by Network ACL and/or Security Group policies within the last 14 days with aggregation of the top 25 target IP addresses

```json
{
  "query": {
    "bool": {
      "must": [
        {
          "match": {
            "flow_logs.action": "rejected"
          }
        }
      ],
      "filter": [
        {
          "range": {
            "capture_start_time": {
              "gte": "now-14d/d",
              "lt": "now/d"
            }
          }
        }
      ]
    }
  },
  "aggregations": {
    "target_ips": {
      "terms": {
        "field": "flow_logs.target_ip.keyword",
        "size": 25
      }
    }
  }
}
```
