# Loki Query Debug Commands (Bash)

```bash
BASE='http://127.0.0.1:13104/loki/api/v1/query_range'
START='1774913623355000000'
END='1774924423355000000'
```

## 1) Baseline: execution-service raw logs

```bash
curl -sG "$BASE" \
  --data-urlencode 'query={namespace="e-team", container="execution-service"}' \
  --data-urlencode "start=$START" \
  --data-urlencode "end=$END" \
  --data-urlencode 'limit=5'
```

## 2) Parse nested JSON and print key fields (to verify `event` existence)

```bash
curl -sG "$BASE" \
  --data-urlencode 'query={namespace="e-team", container="execution-service"} | json | line_format "{{.log}}" | json | line_format "event={{.event}} msg={{.msg}} pipeline={{.pipeline}} run_no={{.run_no}} status={{.status}} err={{.__error__}}"' \
  --data-urlencode "start=$START" \
  --data-urlencode "end=$END" \
  --data-urlencode 'limit=20'
```

## 3) Compare without/with `event="pipeline-run"` filter

```bash
# Without event filter (should return data)
curl -sG "$BASE" \
  --data-urlencode 'query={namespace="e-team", container="execution-service"} | json | line_format "{{.log}}" | json | pipeline="Calctl Public Pipeline" | status=~"success|failed"' \
  --data-urlencode "start=$START" \
  --data-urlencode "end=$END" \
  --data-urlencode 'limit=20'
```

```bash
# With event filter (expected empty in your current data)
curl -sG "$BASE" \
  --data-urlencode 'query={namespace="e-team", container="execution-service"} | json | line_format "{{.log}}" | json | event="pipeline-run" | pipeline="Calctl Public Pipeline" | status=~"success|failed"' \
  --data-urlencode "start=$START" \
  --data-urlencode "end=$END" \
  --data-urlencode 'limit=20'
```

## 4) Validate run number filter

```bash
# run_no=12 (expected empty in tested window)
curl -sG "$BASE" \
  --data-urlencode 'query={namespace="e-team", container="execution-service"} | json | line_format "{{.log}}" | json | pipeline="Calctl Public Pipeline" | run_no="12"' \
  --data-urlencode "start=$START" \
  --data-urlencode "end=$END" \
  --data-urlencode 'limit=20'
```

```bash
# run_no=2 (should return data in tested window)
curl -sG "$BASE" \
  --data-urlencode 'query={namespace="e-team", container="execution-service"} | json | line_format "{{.log}}" | json | pipeline="Calctl Public Pipeline" | run_no="2"' \
  --data-urlencode "start=$START" \
  --data-urlencode "end=$END" \
  --data-urlencode 'limit=20'
```

## 5) Candidate fixed query for pipeline-overview (replace `event` with `msg`)

```bash
curl -sG "$BASE" \
  --data-urlencode 'query=sum by (status) (count_over_time({namespace="e-team", container="execution-service"} | json | line_format "{{.log}}" | json | msg="pipeline run completed" | status=~"success|failed" | pipeline=~"Calctl Public Pipeline" [1h]))' \
  --data-urlencode "start=$START" \
  --data-urlencode "end=$END"
```
