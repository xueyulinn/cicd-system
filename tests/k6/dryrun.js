import http from "k6/http";
import { check, sleep } from "k6";
import { Rate } from "k6/metrics";

const BASE_URL = __ENV.BASE_URL || "http://localhost:8001";
const DRYRUN_PATH = __ENV.DRYRUN_PATH || __ENV.VALIDATE_PATH || "/dryrun";
const THINK_TIME = Number(__ENV.THINK_TIME || "0");

// You can override this with env: YAML_CONTENT='...'
const YAML_CONTENT =
  __ENV.YAML_CONTENT ||
  `pipeline:
  name: k6-dryrun
stages:
  - build

build-job:
  - stage: build
  - image: alpine:latest
  - script:
    - echo "hello from k6"
`;

const dryrunSuccessRate = new Rate("dryrun_success_rate");

export const options = {
  vus: Number(__ENV.VUS || "10"),
  duration: __ENV.DURATION || "30s",
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<500", "p(99)<1000"],
    dryrun_success_rate: ["rate>0.99"],
  },
};

export default function () {
  const payload = JSON.stringify({ yaml_content: YAML_CONTENT });
  const params = {
    headers: {
      "Content-Type": "application/json",
    },
    tags: { endpoint: "dryrun" },
  };

  const res = http.post(`${BASE_URL}${DRYRUN_PATH}`, payload, params);

  let body = null;
  try {
    body = res.json();
  } catch (_) {
    body = null;
  }

  const ok = check(res, {
    "status is 200": (r) => r.status === 200,
    "response has valid(boolean)": () =>
      body !== null && typeof body.valid === "boolean",
    "dryrun result is valid=true": () => body !== null && body.valid === true,
  });

  dryrunSuccessRate.add(ok);

  if (THINK_TIME > 0) {
    sleep(THINK_TIME);
  }
}
