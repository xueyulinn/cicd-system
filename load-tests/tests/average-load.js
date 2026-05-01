import { runValidateScenario } from "../scenarios/validate.js";
import { runDryRunScenario } from "../scenarios/dryrun.js";
import { runSubmitScenario } from "../scenarios/run.js";

const TEST_SCENARIO = __ENV.TEST_SCENARIO || "validate";

const SCENARIOS = {
  validate: {
    run: runValidateScenario,
    thresholds: {
      http_req_failed: ["rate<0.01"],
      http_req_duration: ["p(95)<100", "p(99)<300"],
      validate_success_rate: ["rate>0.99"],
    },
    stages: [
        { duration: '2m', target: 50 },
        { duration: '15m', target: 50 },
        { duration: '2m', target: 0 },
    ],
  },
  dryrun: {
    run: runDryRunScenario,
    thresholds: {
      http_req_failed: ["rate<0.01"],
      http_req_duration: ["p(95)<100", "p(99)<300"],
      dryrun_success_rate: ["rate>0.99"],
    },
    stages: [
        { duration: '2m', target: 50 },
        { duration: '15m', target: 50 },
        { duration: '2m', target: 0 },
    ],
  },
  runSubmit: {
    run: runSubmitScenario,
    thresholds: {
      http_req_failed: ["rate<0.01"],
      http_req_duration: ["p(95)<100", "p(99)<300"],
      dryrun_success_rate: ["rate>0.99"],
    },
    stages: [
      { duration: '2m', target: 50 },
      { duration: '15m', target: 50 },
      { duration: '2m', target: 0 },
    ],
  }
};

const selectedScenario = SCENARIOS[TEST_SCENARIO];

if (!selectedScenario) {
  throw new Error(
    `unsupported TEST_SCENARIO: ${TEST_SCENARIO}. Expected one of: ${Object.keys(SCENARIOS).join(", ")}`,
  );
}

export const options = {
  thresholds: selectedScenario.thresholds,
  stages: selectedScenario.stages,
};

export default function () {
  selectedScenario.run();
}
