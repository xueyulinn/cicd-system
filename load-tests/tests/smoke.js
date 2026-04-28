import { runValidateScenario } from "../scenarios/validate.js";

const DEFAULT_VUS = 5;
const DEFAULT_DURATION = "30s";

export const options = {
  vus: Number(__ENV.VUS || DEFAULT_VUS),
  duration: __ENV.DURATION || DEFAULT_DURATION,
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<100", "p(99)<300"],
    validate_success_rate: ["rate>0.99"],
  },
};

export default function () {
  runValidateScenario();
}
