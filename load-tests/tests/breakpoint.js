import { runValidateScenario } from "../scenarios/validate.js";
import { runDryRunScenario } from "../scenarios/dryrun.js";
import { runSubmitScenario } from "../scenarios/run.js";

const TEST_SCENARIO = __ENV.TEST_SCENARIO || "validate";

const SCENARIOS = {
  validate: {
    run: runValidateScenario,
    executor: "ramping-arrival-rate",
    stages: [
		{ duration: "10m", target: 2000 },
	],
  },
  dryrun: {
    run: runDryRunScenario,
    executor: "ramping-arrival-rate",
    stages: [
		{ duration: "2h", target: 2000 },
	],
  },
  runSubmit: {
    run: runSubmitScenario,
    executor: "ramping-arrival-rate",
    stages: [
		{ duration: "2h", target: 2000 },
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
  executor: selectedScenario.executor,
  stages: selectedScenario.stages,
};

export default function () {
  selectedScenario.run();
}
