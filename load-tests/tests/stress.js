import { runValidateScenario } from "../scenarios/validate.js";
import { runDryRunScenario } from "../scenarios/dryrun.js";

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
        { duration: '2m', target: 150 }, // traffic ramp-up from 1 to 100 users over 5 minutes.
        { duration: '10m', target: 150 }, // stay at 100 users for 30 minutes
        { duration: '2m', target: 0 }, // ramp-down to 0 users
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
        { duration: '2m', target: 150 }, // traffic ramp-up from 1 to 100 users over 5 minutes.
        { duration: '15m', target: 150 }, // stay at 100 users for 30 minutes
        { duration: '2m', target: 0 }, // ramp-down to 0 users
    ],
  },
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
