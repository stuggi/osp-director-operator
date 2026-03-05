/*
Copyright 2021 Red Hat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package openstackconfigversion

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
)

// TestObjectNotFoundDetection tests both type-based and string-based error detection
// for "object not found" errors that Azure DevOps may return
func TestObjectNotFoundDetection(t *testing.T) {
	tests := []struct {
		name                string
		err                 error
		expectedTypeMatch   bool
		expectedStringMatch bool
		shouldTriggerRetry  bool
		description         string
	}{
		{
			name:                "Proper plumbing.ErrObjectNotFound",
			err:                 plumbing.ErrObjectNotFound,
			expectedTypeMatch:   true,
			expectedStringMatch: true,
			shouldTriggerRetry:  true,
			description:         "Standard go-git error - both methods should detect it",
		},
		{
			name:                "Wrapped plumbing.ErrObjectNotFound",
			err:                 fmt.Errorf("git operation failed: %w", plumbing.ErrObjectNotFound),
			expectedTypeMatch:   true,
			expectedStringMatch: true,
			shouldTriggerRetry:  true,
			description:         "Wrapped error - errors.Is should unwrap it",
		},
		{
			name:                "Plain string error with 'object not found'",
			err:                 errors.New("object not found"),
			expectedTypeMatch:   false,
			expectedStringMatch: true,
			shouldTriggerRetry:  true,
			description:         "Azure DevOps non-standard error - only string matching works",
		},
		{
			name:                "Plain string error with 'Object Not Found' (mixed case)",
			err:                 errors.New("Object Not Found"),
			expectedTypeMatch:   false,
			expectedStringMatch: true,
			shouldTriggerRetry:  true,
			description:         "Case-insensitive string matching should work",
		},
		{
			name:                "Error with 'object not found' in message",
			err:                 errors.New("failed to retrieve: object not found in repository"),
			expectedTypeMatch:   false,
			expectedStringMatch: true,
			shouldTriggerRetry:  true,
			description:         "Substring matching should catch it",
		},
		{
			name:                "Different error entirely",
			err:                 errors.New("network timeout"),
			expectedTypeMatch:   false,
			expectedStringMatch: false,
			shouldTriggerRetry:  false,
			description:         "Unrelated error - should not trigger retry",
		},
		{
			name:                "Nil error",
			err:                 nil,
			expectedTypeMatch:   false,
			expectedStringMatch: false,
			shouldTriggerRetry:  false,
			description:         "No error - should not trigger retry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test type-based detection
			isObjectNotFound := errors.Is(tt.err, plumbing.ErrObjectNotFound)
			if isObjectNotFound != tt.expectedTypeMatch {
				t.Errorf("Type-based detection failed: got %v, want %v\n  Description: %s",
					isObjectNotFound, tt.expectedTypeMatch, tt.description)
			}

			// Test string-based detection
			var isObjectNotFoundByMessage bool
			if tt.err != nil {
				errMsg := strings.ToLower(tt.err.Error())
				isObjectNotFoundByMessage = strings.Contains(errMsg, "object not found")
			}

			if isObjectNotFoundByMessage != tt.expectedStringMatch {
				t.Errorf("String-based detection failed: got %v, want %v\n  Description: %s",
					isObjectNotFoundByMessage, tt.expectedStringMatch, tt.description)
			}

			// Test combined detection (should trigger retry if either method matches)
			shouldRetry := isObjectNotFound || isObjectNotFoundByMessage
			if shouldRetry != tt.shouldTriggerRetry {
				t.Errorf("Combined detection failed: got %v, want %v\n  Description: %s",
					shouldRetry, tt.shouldTriggerRetry, tt.description)
			}

			// Log the detection results for debugging
			if tt.err != nil {
				t.Logf("Error type: %T, message: %q", tt.err, tt.err.Error())
				t.Logf("  Type match: %v, String match: %v, Should retry: %v",
					isObjectNotFound, isObjectNotFoundByMessage, shouldRetry)
			}
		})
	}
}

// TestAzureDevOpsErrorFormats tests various error formats that Azure DevOps might return
func TestAzureDevOpsErrorFormats(t *testing.T) {
	// These are potential error formats we might see from Azure DevOps
	azureErrorFormats := []string{
		"object not found",
		"Object not found",
		"OBJECT NOT FOUND",
		"Object Not Found",
		"git: object not found",
		"error: object not found in pack",
		"failed to find object: object not found",
		"repository object not found",
	}

	for _, errMsg := range azureErrorFormats {
		t.Run(fmt.Sprintf("Format: %q", errMsg), func(t *testing.T) {
			err := errors.New(errMsg)

			// Type-based should fail for plain string errors
			isObjectNotFound := errors.Is(err, plumbing.ErrObjectNotFound)
			if isObjectNotFound {
				t.Errorf("Type-based detection should fail for plain string error: %q", errMsg)
			}

			// String-based should succeed
			errMsgLower := strings.ToLower(err.Error())
			isObjectNotFoundByMessage := strings.Contains(errMsgLower, "object not found")
			if !isObjectNotFoundByMessage {
				t.Errorf("String-based detection should succeed for error: %q", errMsg)
			}

			// Combined should trigger retry
			shouldRetry := isObjectNotFound || isObjectNotFoundByMessage
			if !shouldRetry {
				t.Errorf("Combined detection should trigger retry for error: %q", errMsg)
			}
		})
	}
}
