function randomToken() {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

function buildStageNames(token, stageCount) {
  return Array.from(
    { length: stageCount },
    (_, index) => `stage-${index + 1}-${token}`,
  );
}

function buildScriptLines(token, stageIndex, jobIndex, scriptSteps) {
  return Array.from({ length: scriptSteps }, (_, index) => {
    return `    - echo "${token}-s${stageIndex + 1}-j${jobIndex + 1}-step${index + 1}"`;
  });
}

export function generateRandomYAMLContent(namePrefix, complexity = {}) {
  const stageCount = complexity.stageCount || 1;
  const jobsPerStage = complexity.jobsPerStage || 1;
  const scriptSteps = complexity.scriptSteps || 1;
  const token = randomToken();
  const pipelineName = `${namePrefix}-${token}`;
  const stageNames = buildStageNames(token, stageCount);
  const lines = [
    "pipeline:",
    `  name: ${pipelineName}`,
    "stages:",
    ...stageNames.map((stageName) => `  - ${stageName}`),
    "",
  ];

  stageNames.forEach((stageName, stageIndex) => {
    for (let jobIndex = 0; jobIndex < jobsPerStage; jobIndex += 1) {
      const jobName = `job-${stageIndex + 1}-${jobIndex + 1}-${token}`;
      lines.push(
        `${jobName}:`,
        `  - stage: ${stageName}`,
        "  - image: alpine:latest",
        "  - script:",
        ...buildScriptLines(token, stageIndex, jobIndex, scriptSteps),
        "",
      );
    }
  });

  return `${lines.join("\n").trimEnd()}\n`;
}
