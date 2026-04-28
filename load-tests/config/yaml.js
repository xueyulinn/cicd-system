const DEFAULT_STAGE_COUNT = 4;
const DEFAULT_JOBS_PER_STAGE = 2;
const DEFAULT_SCRIPT_STEPS = 2;

function loadPositiveInt(value, fallback) {
  const parsed = Number.parseInt(value || "", 10);
  if (!Number.isFinite(parsed) || parsed < 1) {
    return fallback;
  }

  return parsed;
}

export const yamlGenerationConfig = {
  stageCount: loadPositiveInt(__ENV.YAML_STAGE_COUNT, DEFAULT_STAGE_COUNT),
  jobsPerStage: loadPositiveInt(
    __ENV.YAML_JOBS_PER_STAGE,
    DEFAULT_JOBS_PER_STAGE,
  ),
  scriptSteps: loadPositiveInt(__ENV.YAML_SCRIPT_STEPS, DEFAULT_SCRIPT_STEPS),
};
