import { sleep } from "k6";
import { Rate } from "k6/metrics";
import { validateConfig } from "../config/validate.js";
import { checkValidateResponse } from "../utils/check.js";
import { parseJSON, postJSON } from "../utils/http.js";

const validateSuccessRate = new Rate("validate_success_rate");

export function runValidateScenario() {
  const res = postJSON(
    validateConfig.baseURL,
    validateConfig.path,
    { yaml_content: validateConfig.yamlContent },
    "validate",
  );
  const body = parseJSON(res);
  const ok = checkValidateResponse(res, body);

  validateSuccessRate.add(ok);

  if (validateConfig.thinkTime > 0) {
    sleep(validateConfig.thinkTime);
  }
}

export default runValidateScenario;
