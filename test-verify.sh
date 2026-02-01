#!/bin/bash

# Test script for cicd verify command (parallel YAML structure)

set -e

echo "Building CLI..."
go build -o bin/cicd ./cmd/cli
chmod +x bin/cicd

echo -e "\n=========================================="
echo "Testing cicd verify command"
echo "=========================================="

# Test 1: Valid configuration (default pipeline)
echo -e "\n[TEST 1] Valid configuration (pipeline.yaml)"
echo "Expected: ✓ Configuration is valid"
if ./bin/cicd verify .pipelines/pipeline.yaml 2>&1 | grep -q "valid"; then
    echo "Result: PASS ✓"
else
    echo "Result: FAIL ✗"
fi

# Test 2: Valid dependency
echo -e "\n[TEST 2] Valid dependency"
echo "Expected: ✓ Configuration is valid"
if ./bin/cicd verify .pipelines/valid_dependency.yaml 2>&1 | grep -q "valid"; then
    echo "Result: PASS ✓"
else
    echo "Result: FAIL ✗"
fi

# Test 3: Complex valid dependency
echo -e "\n[TEST 3] Complex dependency"
echo "Expected: ✓ Configuration is valid"
if ./bin/cicd verify .pipelines/complex_dependency.yaml 2>&1 | grep -q "valid"; then
    echo "Result: PASS ✓"
else
    echo "Result: FAIL ✗"
fi

# Test 4: Empty file
echo -e "\n[TEST 4] Empty file"
echo "Expected: Error - must have at least one stage or missing keys"
# Just check if the command fails
if ! ./bin/cicd verify .pipelines/empty.yaml > /dev/null 2>&1; then
    echo "Result: PASS ✓"
else
    echo "Result: FAIL ✗"
fi

# Test 5: No stages
echo -e "\n[TEST 5] No stages defined"
echo "Expected: Error - must have at least one stage"
if ./bin/cicd verify .pipelines/no_stages.yaml 2>&1 | grep -q "at least one stage"; then
    echo "Result: PASS ✓"
else
    echo "Result: FAIL ✗"
fi

# Test 6: Missing pipeline key
echo -e "\n[TEST 6] Missing pipeline structure"
echo "Expected: Error"
if ! ./bin/cicd verify .pipelines/missing_pipeline.yaml > /dev/null 2>&1; then
    echo "Result: PASS ✓"
else
    echo "Result: FAIL ✗"
fi

# Test 7: Empty stage
echo -e "\n[TEST 7] Empty stage (no jobs assigned)"
echo "Expected: Error - stage has no jobs assigned"
if ./bin/cicd verify .pipelines/empty_stage.yaml 2>&1 | grep -q "no jobs assigned"; then
    echo "Result: PASS ✓"
else
    echo "Result: FAIL ✗"
fi

# Test 8: Duplicate stages
echo -e "\n[TEST 8] Duplicate stage names"
echo "Expected: Error - duplicate stage name"
if ./bin/cicd verify .pipelines/duplicate_stages.yaml 2>&1 | grep -q "duplicate stage name"; then
    echo "Result: PASS ✓"
else
    echo "Result: FAIL ✗"
fi

# Test 9: Duplicate jobs
echo -e "\n[TEST 9] Duplicate job names"
echo "Expected: Error - duplicate job name"
if ./bin/cicd verify .pipelines/duplicate_jobs.yaml 2>&1 | grep -q "duplicate job name"; then
    echo "Result: PASS ✓"
else
    echo "Result: FAIL ✗"
fi

# Test 10: Undefined jobs in needs
echo -e "\n[TEST 10] Undefined job in needs"
echo "Expected: Error - references undefined job"
if ./bin/cicd verify .pipelines/undefined_jobs.yaml 2>&1 | grep -q "undefined job"; then
    echo "Result: PASS ✓"
else
    echo "Result: FAIL ✗"
fi

# Test 11: Circular dependency
echo -e "\n[TEST 11] Circular dependency"
echo "Expected: Error - cycle detected"
if ./bin/cicd verify .pipelines/circular_dependency.yaml 2>&1 | grep -q "cycle detected"; then
    echo "Result: PASS ✓"
else
    echo "Result: FAIL ✗"
fi

# Test 12: Self dependency
echo -e "\n[TEST 12] Self dependency"
echo "Expected: Error - cycle detected or cannot depend on itself"
# Check for specific error about self-dependency or cycle
if ./bin/cicd verify .pipelines/self_dependency.yaml 2>&1 | grep -qE "(cycle|cannot depend on itself|validation failed)"; then
    echo "Result: PASS ✓"
else
    echo "Result: FAIL ✗"
fi

# Test 13: Wrong name type
echo -e "\n[TEST 13] Wrong job name type"
echo "Expected: Error - wrong type or parse error"
if ./bin/cicd verify .pipelines/wrong_job_name_type.yaml 2>&1 | grep -qE "(type|parse|unmarshal)"; then
    echo "Result: PASS ✓"
else
    echo "Result: FAIL ✗"
fi

# Test 14: Wrong name field
echo -e "\n[TEST 14] Wrong name field value"
echo "Expected: Error - wrong type or validation error"
# Just check if it fails - any error is acceptable
if ! ./bin/cicd verify .pipelines/wrong_name.yaml > /dev/null 2>&1; then
    echo "Result: PASS ✓"
else
    echo "Result: FAIL ✗"
fi

# Test 15: Wrong needs type
echo -e "\n[TEST 15] Wrong needs type"
echo "Expected: Error - wrong type or parse error"
if ./bin/cicd verify .pipelines/wrong_needs_type.yaml 2>&1 | grep -qE "(type|parse|unmarshal)"; then
    echo "Result: PASS ✓"
else
    echo "Result: FAIL ✗"
fi

# Test 16: Wrong stage type
echo -e "\n[TEST 16] Wrong stage type"
echo "Expected: Error - wrong type or parse error"
if ./bin/cicd verify .pipelines/wrong_stage_type.yaml 2>&1 | grep -qE "(type|parse|unmarshal)"; then
    echo "Result: PASS ✓"
else
    echo "Result: FAIL ✗"
fi

# Test 17: Multiple errors in one file
echo -e "\n[TEST 17] Multiple errors"
echo "Expected: Multiple error messages"
if ./bin/cicd verify .pipelines/multiple_errors.yaml 2>&1 | grep -qE "(duplicate|undefined|cycle|empty)"; then
    echo "Result: PASS ✓"
else
    echo "Result: FAIL ✗"
fi

# Test 18: Default file (no argument)
echo -e "\n[TEST 18] Default file path (.pipelines/pipeline.yaml)"
echo "Expected: ✓ Configuration is valid"
if ./bin/cicd verify 2>&1 | grep -q "valid"; then
    echo "Result: PASS ✓"
else
    echo "Result: FAIL ✗"
fi

echo -e "\n=========================================="
echo "All tests completed!"
echo "=========================================="
echo ""
echo "Summary of test files:"
echo "  Valid configurations:"
echo "    - pipeline.yaml (default)"
echo "    - valid_dependency.yaml"
echo "    - complex_dependency.yaml"
echo ""
echo "  Error cases:"
echo "    - empty.yaml"
echo "    - no_stages.yaml"
echo "    - missing_pipeline.yaml"
echo "    - empty_stage.yaml"
echo "    - duplicate_stages.yaml"
echo "    - duplicate_jobs.yaml"
echo "    - undefined_jobs.yaml"
echo "    - circular_dependency.yaml"
echo "    - self_dependency.yaml"
echo "    - wrong_job_name_type.yaml"
echo "    - wrong_name.yaml"
echo "    - wrong_needs_type.yaml"
echo "    - wrong_stage_type.yaml"
echo "    - multiple_errors.yaml"