import { yamlGenerationConfig } from "./yaml.js";
import { generateRandomYAMLContent } from "../utils/yaml.js";

const DEFAULT_BASE_URL = "http://localhost:8001";
const DEFAULT_DRYRUN_PATH = "/dryrun";
const DEFAULT_THINK_TIME = 0;

export const dryRunConfig = {
  baseURL: __ENV.BASE_URL || DEFAULT_BASE_URL,
  path: __ENV.DRYRUN_PATH || __ENV.VALIDATE_PATH || DEFAULT_DRYRUN_PATH,
  thinkTime: Number(__ENV.THINK_TIME || DEFAULT_THINK_TIME),
  get yamlContent() {
    return (
      __ENV.YAML_CONTENT ||
      generateRandomYAMLContent("k6-dryrun", yamlGenerationConfig)
    );
  },
};
