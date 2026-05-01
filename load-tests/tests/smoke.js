import { runValidateScenario } from "../scenarios/validate.js";
import { runDryRunScenario } from "../scenarios/dryrun.js";
import { runSubmitScenario } from "../scenarios/run.js";

const TEST_SCENARIO = __ENV.TEST_SCENARIO || "validate";
const DEFAULT_DURATION = "30s";

const SCENARIOS = {
  validate: {
    run: runValidateScenario,
    defaultVUs: 5,
    thresholds: {
      http_req_failed: ["rate<0.01"],
      validate_success_rate: ["rate>0.99"],
    },
  },
  dryrun: {
    run: runDryRunScenario,
    defaultVUs: 5,
    thresholds: {
      http_req_failed: ["rate<0.01"],
      dryrun_success_rate: ["rate>0.99"],
    },
  },
  runSubmit: {
    run: runSubmitScenario,
    defaultVUs: 5,
    thresholds: {
      http_req_failed: ["rate<0.01"],
      dryrun_success_rate: ["rate>0.99"],
    },
  }
};

const selectedScenario = SCENARIOS[TEST_SCENARIO];

if (!selectedScenario) {
  throw new Error(
    `unsupported TEST_SCENARIO: ${TEST_SCENARIO}. Expected one of: ${Object.keys(SCENARIOS).join(", ")}`,
  );
}

export const options = {
  vus: Number(__ENV.VUS || selectedScenario.defaultVUs),
  duration: __ENV.DURATION || DEFAULT_DURATION,
  thresholds: selectedScenario.thresholds,
};

export default function () {
  selectedScenario.run();
}
