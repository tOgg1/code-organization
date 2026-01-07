---
id: code-organization-1fi
status: closed
deps: [code-organization-aqu, code-organization-03r]
links: []
created: 2025-12-31T14:47:36.990979+01:00
type: task
priority: 3
parent: code-organization-2j3
---
# Comprehensive integration and end-to-end tests

Integration tests for the complete partials feature:

TEST FILE: internal/partial/integration_test.go

FULL FLOW TESTS:

TestApplyPartialToEmptyDir:
- Create temp partial and target
- Apply with variables
- Verify all files created with correct content
- Verify variables substituted

TestApplyPartialWithConflicts:
- Create target with existing files
- Apply with skip strategy
- Verify existing files unchanged
- Verify new files created

TestApplyPartialWithBackup:
- Create target with existing files  
- Apply with backup strategy
- Verify .bak files created
- Verify new content written

TestApplyPartialDryRun:
- Apply with --dry-run
- Verify no files written
- Verify result shows planned actions

TestApplyPartialWithHooks:
- Create partial with pre/post hooks
- Apply
- Verify hooks executed in order
- Verify hook env vars set correctly

TestApplyPartialWithPrerequisites:
- Create partial with requires
- Test satisfied prerequisites pass
- Test unsatisfied prerequisites fail
- Test --force bypasses

TestPartialMergeStrategies:
- Test gitignore merge
- Test JSON merge
- Test YAML merge
- Verify deep merge behavior

TEST FILE: cmd/co/cmd/partial_test.go

CLI TESTS:

TestPartialListCommand:
- Test default output
- Test --json flag
- Test --tag filter

TestPartialShowCommand:
- Test default output
- Test --json flag
- Test --files flag
- Test not found error

TestPartialApplyCommand:
- Test basic apply
- Test with variables
- Test --dry-run
- Test --conflict flag
- Test error handling

END-TO-END:

TestFullWorkflow:
1. Create custom partial in temp dir
2. List and verify it appears
3. Show and verify details
4. Apply to target
5. Verify all files correct
6. Apply again with conflicts
7. Verify conflict handling


