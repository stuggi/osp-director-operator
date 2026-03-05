// +build ignore

/*
Standalone reproducer for Azure DevOps compatibility issue

This reproducer demonstrates the fix for the issue where:
1. Clone works with Azure DevOps
2. But CommitObject fails with "Object not found"

The fix detects the error and retries with transport.UnsupportedCapabilities
set to only [ThinPack].

Usage:
  go run reproducer_azure_devops.go

Set environment variables:
  export TEST_GIT_URL="git@ssh.dev.azure.com:v3/org/project/repo"
  export TEST_SSH_KEY_PATH="/path/to/ssh/key"
*/

package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	crypto_ssh "golang.org/x/crypto/ssh"
)

func main() {
	// Get git URL from environment or use default
	gitURL := os.Getenv("TEST_GIT_URL")
	if gitURL == "" {
		gitURL = "git@github.com:zerodayz/dns-gather-tool.git"
	}

	// Get SSH key path from environment or use default
	privateKeyPath := os.Getenv("TEST_SSH_KEY_PATH")
	if privateKeyPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		privateKeyPath = homeDir + "/.ssh/id_rsa"
	}

	log.Printf("Testing Azure DevOps compatibility with: %s", gitURL)

	pkey, err := os.ReadFile(privateKeyPath)
	if err != nil {
		log.Fatal(err)
	}

	endpoint, err := transport.NewEndpoint(gitURL)
	if err != nil {
		log.Fatalf("parse git url failed: %s", err)
	}

	publicKeys, err := ssh.NewPublicKeys(endpoint.User, pkey, "")
	if err != nil {
		log.Fatalf("generate publickeys failed: %s", err)
	}
	publicKeys.HostKeyCallback = crypto_ssh.InsecureIgnoreHostKey()

	// Attempt git operations (may fail with Azure DevOps)
	repo, refs, err := performGitOperations(gitURL, publicKeys)
	if err != nil {
		log.Fatal(err)
	}

	// Try to get commit objects from branches
	needsRetry := false
	for _, ref := range refs {
		if !ref.Name().IsBranch() || ref.Name() == "HEAD" {
			continue
		}

		// Skip default branches for testing (or test them if no other branches exist)
		isDefault := ref.Name() == "refs/heads/master" || ref.Name() == "refs/heads/main"

		log.Printf("Testing branch: %s", ref.Name())

		_, err := repo.CommitObject(ref.Hash())
		if err != nil {
			// Check if we've already applied the fix
			azureDevOpsFixApplied := len(transport.UnsupportedCapabilities) == 1 &&
				transport.UnsupportedCapabilities[0] == capability.ThinPack

			// Detect "object not found" using two methods:
			// 1. Type-based: errors.Is(err, plumbing.ErrObjectNotFound) - most reliable
			// 2. String-based: Check error message for "object not found" - fallback for non-standard error wrapping
			isObjectNotFound := errors.Is(err, plumbing.ErrObjectNotFound)
			isObjectNotFoundByMessage := strings.Contains(strings.ToLower(err.Error()), "object not found")

			if (isObjectNotFound || isObjectNotFoundByMessage) && !azureDevOpsFixApplied {
				log.Println("✗ CommitObject failed with object not found error")
				if isObjectNotFound {
					log.Println("  Detected via errors.Is() type matching")
				} else {
					log.Println("  Detected via string matching (Azure DevOps non-standard error)")
				}
				log.Printf("  Error: %v (type: %T)", err, err)
				log.Println("  Applying Azure DevOps fix: setting UnsupportedCapabilities = [ThinPack]")
				transport.UnsupportedCapabilities = []capability.Capability{
					capability.ThinPack,
				}
				needsRetry = true
				break
			} else if err != nil {
				log.Fatalf("✗ Unexpected error: %v (type: %T)", err, err)
			}
		}

		log.Printf("✓ CommitObject succeeded")

		// Only test one branch for default branches
		if isDefault {
			break
		}
	}

	// Retry if needed
	if needsRetry {
		log.Println("\nRetrying git operations with ThinPack fix...")
		repo, refs, err = performGitOperations(gitURL, publicKeys)
		if err != nil {
			log.Fatal(err)
		}

		// Test again
		for _, ref := range refs {
			if !ref.Name().IsBranch() || ref.Name() == "HEAD" {
				continue
			}

			isDefault := ref.Name() == "refs/heads/master" || ref.Name() == "refs/heads/main"

			log.Printf("Testing branch (retry): %s", ref.Name())

			_, err := repo.CommitObject(ref.Hash())
			if err != nil {
				log.Fatalf("✗ CommitObject failed after retry: %v", err)
			}

			log.Printf("✓ CommitObject succeeded after retry")

			if isDefault {
				break
			}
		}
	}

	log.Println("\n✓ All operations completed successfully!")
	log.Println("\nThe Azure DevOps compatibility fix works by:")
	log.Println("1. Detecting ErrObjectNotFound when Clone succeeded but objects are incomplete")
	log.Println("2. Setting transport.UnsupportedCapabilities = [ThinPack]")
	log.Println("3. Retrying the entire git operation with the fix applied")
}

func performGitOperations(gitURL string, publicKeys *ssh.PublicKeys) (*git.Repository, []*plumbing.Reference, error) {
	// Clone repository
	log.Println("Cloning repository...")
	repo, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:  gitURL,
		Auth: publicKeys,
	})

	// Handle ErrEmptyUploadPackRequest (Azure DevOps Clone failure)
	if err != nil && err == transport.ErrEmptyUploadPackRequest {
		log.Println("Clone failed with ErrEmptyUploadPackRequest")
		log.Println("Applying ThinPack fix and retrying...")
		transport.UnsupportedCapabilities = []capability.Capability{
			capability.ThinPack,
		}

		repo, err = git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
			URL:  gitURL,
			Auth: publicKeys,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("clone failed after ThinPack workaround: %w", err)
		}
	} else if err != nil {
		return nil, nil, fmt.Errorf("clone failed: %w", err)
	}

	log.Println("✓ Clone successful")

	// List refs
	rem := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{gitURL},
	})

	refs, err := rem.List(&git.ListOptions{Auth: publicKeys})
	if err != nil {
		return nil, nil, fmt.Errorf("list refs failed: %w", err)
	}

	log.Printf("✓ Found %d refs", len(refs))
	return repo, refs, nil
}
