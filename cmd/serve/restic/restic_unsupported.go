// Build for unsupported platforms to stop go complaining
// about "no buildable Go source files "

// +build !go1.9

package restic

import "github.com/spf13/cobra"

// Command definition is nil to show not implemented
var Command *cobra.Command = nil
