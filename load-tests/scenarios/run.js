import { sleep } from "k6";
import { Rate } from "k6/metrics";
import { checkSubmitResponse } from "../utils/check.js";
import { parseJSON, postJSON } from "../utils/http.js";
import { runConfig } from "../config/run.js";

const runSuccessRate = new Rate("run_success_rate");

export function runSubmitScenario() {
  const res = postJSON(
    runConfig.baseURL,
    runConfig.path,
    { yaml_content: runConfig.yamlContent },
    "run",
  );
  const body = parseJSON(res);
  const ok = checkSubmitResponse(res, body);

  runSuccessRate.add(ok);

  if (runConfig.thinkTime > 0) {
    sleep(runConfig.thinkTime);
  }
}

export default runSubmitScenario;
