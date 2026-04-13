import http from 'k6/http';
import { check } from 'k6';

const PIPELINE_FILE = '.pipelines/parallel-local.yaml';
const EXEC_BASE_URL = 'http://localhost:8002';
const EXEC_RUN_PATH = '/run';
const RUN_BRANCH = 'main';
const RUN_COMMIT = 'k6-test';
const RUN_REPO_URL = '';
const RUN_WORKSPACE_PATH = '';
const K6_RATE = 20;
const K6_TIME_UNIT = '1s';
const K6_DURATION = '1m';
const K6_PRE_ALLOCATED_VUS = 20;
const K6_MAX_VUS = 200;
const K6_HTTP_TIMEOUT = '120s';
const K6_ERR_RATE_MAX = '0.01';
const K6_P95_MS_MAX = '500';
const K6_P99_MS_MAX = '1200';

const pipelineYAML = open(PIPELINE_FILE);

export const options = {
  scenarios: {
    run_pipeline: {
      executor: 'constant-arrival-rate',
      rate: K6_RATE,
      timeUnit: K6_TIME_UNIT,
      duration: K6_DURATION,
      preAllocatedVUs: K6_PRE_ALLOCATED_VUS,
      maxVUs: K6_MAX_VUS,
    },
  },
  thresholds: {
    http_req_failed: [`rate<${K6_ERR_RATE_MAX}`],
    http_req_duration: [`p(95)<${K6_P95_MS_MAX}`, `p(99)<${K6_P99_MS_MAX}`],
  },
};

export default function () {
  const runNonce = `k6-${__VU}-${__ITER}-${Date.now()}-${Math.floor(Math.random() * 1e9)}`;
  const payload = {
    yaml_content: `${pipelineYAML}\n# k6-run-nonce: ${runNonce}\n`,
    branch: RUN_BRANCH,
    commit: RUN_COMMIT,
    repo_url: RUN_REPO_URL,
    workspace_path: RUN_WORKSPACE_PATH,
  };

  const res = http.post(`${EXEC_BASE_URL}${EXEC_RUN_PATH}`, JSON.stringify(payload), {
    headers: { 'Content-Type': 'application/json' },
    timeout: K6_HTTP_TIMEOUT,
  });

  check(res, {
    'status 200': (r) => r.status === 200,
    'accepted': (r) => {
      try {
        const body = JSON.parse(r.body || '{}');
        return ['queued', 'running', 'success'].includes(body.status);
      } catch (e) {
        return false;
      }
    },
  });
}
