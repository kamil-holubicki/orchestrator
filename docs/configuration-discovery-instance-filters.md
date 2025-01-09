# Configuration: instance filters

Sometimes it is desirable to exclude particular hosts from discovery.
It may be the case for large, complex environments where auxiliary nodes like gh-ost, Debezium, Tungsten, etc., are present.

A node can be excluded by specifying its host's name regex pattern. When the Orchestrator detects such a node and its host name matches one of the given patterns, the node is skipped during the discovery process:

```json
{
  "DiscoveryIgnoreHostnameFilters": [
    "a_host_i_want_to_ignore[.]example[.]com:5000",
    ".*[.]example2.com:5000",
    "192.168.0.1:6000"
  ]
}
```

Regexp filters to apply to prevent auto-discovering new replicas. Usage: unreachable servers due to firewalls, applications which trigger binlog dumps:

```json
{
  "DiscoveryIgnoreReplicaHostnameFilters": [
    "a_host_i_want_to_ignore[.]example[.]com",
    ".*[.]ignore_all_hosts_from_this_domain[.]example[.]com",
    "a_host_with_extra_port_i_want_to_ignore[.]example[.]com:3307"
  ]
}
```

Regexp filters to apply to prevent auto-discovering a master. Usage: pointing your master temporarily to replicate some data from external host:

```json
{
  "DiscoveryIgnoreMasterHostnameFilters": [
    "a_host_i_want_to_ignore[.]example[.]com:5000",
    ".*[.]example2.com:5000",
    "192.168.0.1:6000"
  ]
}
```

It is also possible to specify regexp filters used to check the replication user name used by the replica. When Orchestrator detects that a given instance uses the replication user matching one of the patterns, such instance is skipped during the discovery:

```json
{
    "DiscoveryIgnoreReplicationUsernameFilters": [
            "replication_user",
            "rpl_[0-9].*"
    ]
}
```
For the Orchestrator to discover the replication topology it is enough to point it to any node. The Orchestrator will detect possible replication sources of the given node and its replicas. Then it will continue with the discovery using the detected source/replicas and build a picture of the whole replication graph.

Let's assume that we have the following replication channel:

node_1 -> node_2(repl_user_2) -> node_3(repl_user_3)

node_1 : source\
node_2 : intermediate source that uses `repl_user_2` to replicate from node_1\
node_3 : replica that uses `repl_user_3` to replicate from node_2

and
```json
{
    "DiscoveryIgnoreReplicationUsernameFilters": [
            "repl_user_2"
    ]
}
```

Let's consider following discovery cases:

## node_1 used for discovery
Orchestrator learns that node_2 is the replica during the examination of node_1. If the replication user used by node_2 is known at this point, node_2 will be filtered out immediately and never queried.
1. In the case of `DiscoverByShowSlaveHosts=true`, node_1 knows about node_2 replication user only if node_1 is started with `--show-replica-auth-info=1` and node_2 is configured with `report_user=<replication_user>`. If that's not the case, Orchestrator cannot filter out the replica, and node_2 will be scheduled for discovery.\
Once the discovery for node_2 is started, Orchestrator checks its replication user name (but this needs querying of node_2), and if it matches the filter, node_2 is skipped.\
To avoid this overhead, configure node_1 and node_2 properly.
2. In the case of `DiscoverByShowSlaveHosts=false`, `information_schema.processlist` table is used. It contains the replica's replication user name, so node_2 will be skipped immediately and never queried.

As a result, only node_1 is discovered.

## node_2 used for discovery
While examining node_2, Orchestrator detects that its replication user name matches the given pattern and immediately reports the node is excluded from discovery.

Nothing will be discovered.

## node_3 used for discovery
The Orchestrator learns about node_2 being the source during the examination of node_3. At this point Orchestrator doesn't know which replication user is used by node_2, so node_2 is scheduled for discovery. Then the Orchestrator discovers node_2, it detects that its replication user matches the filter pattern and skips the node. As it is skipped, node_1 is never examined. \
Please note that during the discovery of node_3, it is not possible to know the replication user of node_2, so the Orchestrator needs to query node_2 for this information periodically.

As a result, only node_3 is discovered.

## Logs
Orchestrator logs such discovery skips in its error log. Logging is enabled by default and controlled by the following setting:

```json
{
    "EnableDiscoveryFiltersLogs": false
}
```
As for now, logging control applies only to `DiscoveryIgnoreReplicationUsernameFilters`.