import { yamlGenerationConfig } from "./yaml.js";
import { generateRandomYAMLContent } from "../utils/yaml.js";

const DEFAULT_BASE_URL = "http://localhost:8000";
const DEFAULT_VALIDATE_PATH = "/validate";
const DEFAULT_THINK_TIME = 0;

export const validateConfig = {
  baseURL: __ENV.BASE_URL || DEFAULT_BASE_URL,
  path: __ENV.VALIDATE_PATH || DEFAULT_VALIDATE_PATH,
  thinkTime: Number(__ENV.THINK_TIME || DEFAULT_THINK_TIME),
  get yamlContent() {
    return (
      __ENV.YAML_CONTENT ||
      generateRandomYAMLContent("k6-validate", yamlGenerationConfig)
    );
  },
};
