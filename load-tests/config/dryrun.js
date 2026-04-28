const DEFAULT_BASE_URL = "http://localhost:8001";
const DEFAULT_DRYRUN_PATH = "/dryrun";
const DEFAULT_THINK_TIME = 0;
const DEFAULT_YAML_CONTENT = `pipeline:
  name: k6-dryrun
stages:
  - build

build-job:
  - stage: build
  - image: alpine:latest
  - script:
    - echo "hello from k6"
`;

export const dryRunConfig = {
  baseURL: __ENV.BASE_URL || DEFAULT_BASE_URL,
  path: __ENV.DRYRUN_PATH || __ENV.VALIDATE_PATH || DEFAULT_DRYRUN_PATH,
  thinkTime: Number(__ENV.THINK_TIME || DEFAULT_THINK_TIME),
  yamlContent: __ENV.YAML_CONTENT || DEFAULT_YAML_CONTENT,
};
