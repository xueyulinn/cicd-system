package verifier

import (
	"github.com/CS7580-SEA-SP26/e-team/internal/models"
	"gopkg.in/yaml.v3"
)

// PipelineVerifier validates pipeline configurations
type PipelineVerifier struct {
	filePath         string
	pipeline         *models.Pipeline
	rootNode         *yaml.Node
	legacyJobsCached bool
	legacyJobNodes   []legacyJobNode
}

// legacyJobNode represents a job defined in format
type legacyJobNode struct {
	name  string
	key   *yaml.Node
	value *yaml.Node
}

// NewPipelineVerifier creates a new verifier
func NewPipelineVerifier(filePath string, pipeline *models.Pipeline, rootNode *yaml.Node) *PipelineVerifier {
	return &PipelineVerifier{
		filePath: filePath,
		pipeline: pipeline,
		rootNode: rootNode,
	}
}

// Verify runs all validation checks
func (v *PipelineVerifier) Verify() []error {
	var errors []error

	// Populate pipeline data from legacy format if needed
	v.populateLegacyPipelineData()

	// Check 0: Validate YAML types and structure
	typeErrors := v.checkYAMLTypes()
	if len(typeErrors) > 0 {
		errors = append(errors, typeErrors...)
		// Only return early if there are critical parsing errors
		// that would prevent other checks from working
		if v.hasCriticalErrors(typeErrors) {
			return errors
		}
	}

	// Check 1: At least one stage defined
	if err := v.checkAtLeastOneStage(); err != nil {
		errors = append(errors, err)
	}

	// Check 2: Stage names are unique
	errors = append(errors, v.checkUniqueStageNames()...)

	// Check 3: At least one job defined
	if err := v.checkAtLeastOneJob(); err != nil {
		errors = append(errors, err)
	}

	// Check 4: Job names are unique
	errors = append(errors, v.checkUniqueJobNames()...)

	// Check 5: All job stages exist
	errors = append(errors, v.checkJobStagesExist()...)

	// Check 6: No empty stages
	errors = append(errors, v.checkNoEmptyStages()...)

	// Check 7: All needs references are valid
	errors = append(errors, v.checkNeedsReferences()...)

	// Check 8: No cycles in dependencies
	if err := v.checkNoCycles(); err != nil {
		errors = append(errors, err)
	}

	return errors
}
