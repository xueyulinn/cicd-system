import http from "k6/http";
import { check, sleep } from "k6";
import { Rate } from "k6/metrics";

const BASE_URL = __ENV.BASE_URL || "http://localhost:8000";
const VALIDATE_PATH = __ENV.VALIDATE_PATH || "/validate";
const THINK_TIME = Number(__ENV.THINK_TIME || "0");

// You can override this with env: YAML_CONTENT='...'
const YAML_CONTENT =
  __ENV.YAML_CONTENT ||
  `pipeline:
  name: k6-validate
stages:
  - build

build-job:
  - stage: build
  - image: alpine:latest
  - script:
    - echo "hello from k6"
`;

const validateSuccessRate = new Rate("validate_success_rate");

export const options = {
  vus: Number(__ENV.VUS || "5"),
  duration: __ENV.DURATION || "30s",
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<500", "p(99)<1000"],
    validate_success_rate: ["rate>0.99"],
  },
};

export default function () {
  const payload = JSON.stringify({ yaml_content: YAML_CONTENT });
  const params = {
    headers: {
      "Content-Type": "application/json",
    },
    tags: { endpoint: "validate" },
  };

  const res = http.post(`${BASE_URL}${VALIDATE_PATH}`, payload, params);

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
    "validate result is valid=true": () => body !== null && body.valid === true,
  });

  validateSuccessRate.add(ok);

  if (THINK_TIME > 0) {
    sleep(THINK_TIME);
  }
}
