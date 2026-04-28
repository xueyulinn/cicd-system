import { sleep } from "k6";
import { Rate } from "k6/metrics";
import { dryRunConfig } from "../config/dryrun.js";
import { checkDryRunResponse } from "../utils/check.js";
import { parseJSON, postJSON } from "../utils/http.js";

const dryRunSuccessRate = new Rate("dryrun_success_rate");

export function runDryRunScenario() {
  const res = postJSON(
    dryRunConfig.baseURL,
    dryRunConfig.path,
    { yaml_content: dryRunConfig.yamlContent },
    "dryrun",
  );
  const body = parseJSON(res);
  const ok = checkDryRunResponse(res, body);

  dryRunSuccessRate.add(ok);

  if (dryRunConfig.thinkTime > 0) {
    sleep(dryRunConfig.thinkTime);
  }
}

export default runDryRunScenario;
