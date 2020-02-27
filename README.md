## Pinger

Used to deploy a server/client that polls a list of endpoints, and also accepts polls from each of them. This leads to a "mesh" topology whereby each 
relies on a functioning overlay network, and its DNS service. All connections are logged to stderr as well as (optional) Graylog. This makes it
trivial to spot when containers within an overlay network lose their ability to talk to each other.

There are lots of issues which have plagued Docker Swarm overlay networks, especially when using encryption - and this simply helps to pin-point 
where the issue lies. This issue is very hard to diagnose by looking at application logs from other containers.

### syntax

docker-compose up -d

gives:

```
<admin>$ docker-compose up -d
Creating network "pinger_my-test-bridge" with the default driver
Creating pinger_pinger_1 ... done
```
view logs:
```
<admin>$docker logs pinger_pinger_1
time="2020-02-18T15:39:11Z" level=info msg="running with config: {1 10 pinger pinger_1 _ 1s 1s 30s 8111 }"
time="2020-02-18T15:39:11Z" level=info msg="starting clients..."
time="2020-02-18T15:39:11Z" level=info msg="logging to stderr only - no graylog url supplied"
time="2020-02-18T15:39:11Z" level=info msg="http listener on: 0.0.0.0:8111"
time="2020-02-18T15:39:41Z" level=error msg="error:Get http://pinger_2:8111/ping: dial tcp: lookup pinger_2 on 127.0.0.11:53: no such host"
time="2020-02-18T15:39:42Z" level=error msg="error:Get http://pinger_4:8111/ping: net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)"
<admin>$
```
finally, to stop:
```
<admin>$docker-compose down
Stopping pinger_pinger_1 ... done
Removing pinger_pinger_1 ... done
Removing network pinger_my-test-bridge
```
