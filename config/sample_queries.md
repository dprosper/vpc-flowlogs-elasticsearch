
### https://<elasticsearch_cluster_hostname>:<elasticsearch_cluster_port>/ibm_vpc_flowlogs_v1/_search

#### 25 Top target IP addresses found 
```json
{
	"size": 0,
	"aggregations" : {
		"target_ips" : {
			"terms" : { 
				"field" : "flow_logs.target_ip.keyword",  
				"size" : 25 
				}
		}
	}
}
```

#### 25 Top initiator IP addresses found 

```json
{
	"size": 0,
	"aggregations" : {
		"initiator_ips" : {
			"terms" : { 
				"field" : "flow_logs.initiator_ip.keyword",  
				"size" : 25 
				}
		}
	}
}
```

#### Total Inbound and Outbound connections found (verify)

```json
{
	"size": 0,
	"aggregations" : {
		"directions" : {
			"terms" : { 
				"field" : "flow_logs.direction.keyword",  
				"size" : 2
				}
		}
	}
}
```

#### Total accepted/rejected found
```json
{
	"size": 0,
	"aggregations" : {
		"actions" : {
			"terms" : { 
				"field" : "flow_logs.action.keyword",  
				"size" : 2
				}
		}
	}
}
```


#### 25 Top target ports 

```json
{
	"size": 0,
	"aggregations" : {
		"target_ports" : {
			"terms" : { 
				"field" : "flow_logs.target_port",  
				"size" : 25 
				}
		}
	}
}
```

#### 25 Top transport protocols 

```json
{
	"size": 0,
	"aggregations" : {
		"transport_protocols" : {
			"terms" : { 
				"field" : "flow_logs.transport_protocol",  
				"size" : 25 
				}
		}
	}
}
```

### https://<elasticsearch_cluster_hostname>:<elasticsearch_cluster_port>/ibm_vpc_flowlogs_v1/_search?scroll=1m


#### All logs with 50 page size and scroll token
```json
{
  "size": 50,
  "query": {
    "match_all": {}
  }
}
```