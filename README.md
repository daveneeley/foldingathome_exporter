# Folding@home Exporter for Prometheus

A [Folding@home](https://foldingathome.org/) exporter for Prometheus.

Uses [MakotoE/go-fahapi](https://github.com/MakotoE/go-fahapi), based on [prometheus/memcached_exporter](https://github.com/prometheus/memcached_exporter).

## Collectors

The exporter collects a number of statistics from the Folding@home client's telnet API:

```
# HELP foldingathome_slot_status The status of the slot, encoded numerically: 0 => uknown, 1 => ready, 2 => download, 3 => running, 4 => upload, 5 => finishing, 6 => stopping, 7 => paused.
# TYPE foldingathome_slot_status gauge
# HELP foldingathome_slot_attempts Number of attempts to download a work unit.
# TYPE foldingathome_slot_attempts gauge
# HELP foldingathome_slot_next_attempt_seconds Seconds until the next attempt to download a work unit.
# TYPE foldingathome_slot_next_attempt_seconds gauge
# HELP foldingathome_slot_estimated_points_per_day Estimated number of points the slot can produce in a day.
# TYPE foldingathome_slot_estimated_points_per_day gauge
# HELP foldingathome_work_unit_steps_completed_percent Work unit completion percentage.
# TYPE foldingathome_work_unit_steps_completed_percent gauge
# HELP foldingathome_work_unit_credit_estimate_points Estimated number of points that will be credited for the work unit.
# TYPE foldingathome_work_unit_credit_estimate_points gauge
# HELP foldingathome_work_unit_estimated_completion_seconds Estimated seconds until the work unit is completed.
# TYPE foldingathome_work_unit_estimated_completion_seconds gauge
# HELP foldingathome_work_unit_time_remaining_seconds Seconds until the work unit's deadline, after which the work unit is expired and will be discarded by the client.
# TYPE foldingathome_work_unit_time_remaining_seconds gauge
# HELP foldingathome_time_seconds Current UNIX time according to the FAHClient.
# TYPE foldingathome_time_seconds gauge
# HELP foldingathome_up Could the FAHClient be reached.
# TYPE foldingathome_up gauge
# HELP foldingathome_uptime_seconds Number of seconds since the FAHClient started.
# TYPE foldingathome_uptime_seconds gauge
# HELP foldingathome_version The version of this FAHClient.
# TYPE foldingathome_version gauge
```
