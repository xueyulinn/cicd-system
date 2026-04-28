import { yamlGenerationConfig } from "./yaml.js";
import { generateRandomYAMLContent } from "../utils/yaml.js";

const DEFAULT_BASE_URL = "http://localhost:8000";
const DEFAULT_RUN_PATH = "/run";
const DEFAULT_THINK_TIME = 0;

export const runConfig = {
  baseURL: __ENV.BASE_URL || DEFAULT_BASE_URL,
  path: __ENV.RUN_PATH || DEFAULT_RUN_PATH,
  thinkTime: Number(__ENV.THINK_TIME || DEFAULT_THINK_TIME),
  get yamlContent() {
    return (
      __ENV.YAML_CONTENT ||
      generateRandomYAMLContent("k6-validate", yamlGenerationConfig)
    );
  },
};
