// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"fmt"
	"os/exec"

	"github.com/cli/safeexec"
)

// VerifySignature verifies a binary using cosign if signature and key are provided.
// It returns nil if verification is skipped or successful.
func VerifySignature(binaryPath, signaturePath, publicKeyPath string) error {
	if signaturePath == "" || publicKeyPath == "" {
		return nil
	}
	cosignPath, err := safeexec.LookPath("cosign")
	if err != nil {
		return fmt.Errorf("cosign not found for signature verification")
	}
	cmd := exec.Command(
		cosignPath,
		"verify-blob",
		"--key", publicKeyPath,
		"--signature", signaturePath,
		binaryPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cosign verification failed: %w (%s)", err, string(out))
	}
	return nil
}
